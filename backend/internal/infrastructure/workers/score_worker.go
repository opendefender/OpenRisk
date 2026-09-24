// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	redisclient "github.com/opendefender/openrisk/internal/infrastructure/redis"
	"github.com/opendefender/openrisk/pkg/events"
	"github.com/opendefender/openrisk/pkg/scoring"
	"github.com/rs/zerolog"
)

// ScoreWorker écoute les events Redis et déclenche le recalcul des scores.
//
// RÈGLE ABSOLUE: Le Score Engine n'est JAMAIS appelé directement
// depuis un handler. Toujours via cet event Redis.
//
// Flux exact:
//  1. Subscribe sur risk.updated ET asset.criticality_changed
//  2. Pour risk.updated:
//     a. Désérialiser RiskUpdatedEvent
//     b. Appeler scoring.Engine.Breakdown(probability, impact, criticality)
//     c. Mettre à jour le risque en DB via RiskRepository.UpdateScore()
//     d. Publier risk.score_updated avec RiskScoreUpdatedEvent
//  3. Pour asset.criticality_changed:
//     a. Récupérer tous les risques liés à cet asset (via repository)
//     b. Pour chacun → republier risk.updated (déclenche le flux du 2)
//  4. Graceful shutdown via context.Done()
//  5. Retry: si erreur DB → retry 3× avec backoff 100ms/500ms/2s
//     Si toujours en échec → logger l'erreur + continuer (ne pas panic)
type ScoreWorker struct {
	redis    *redisclient.Client
	engine   scoring.Engine
	riskRepo RiskRepository
	logger   zerolog.Logger
	// audit journals every score the engine changes (#486). Optional: nil
	// keeps the worker running without a trail, as before.
	audit ScoreAuditAppender
}

// ScoreAuditAppender is the chained audit store (GormAuditChainRepository).
type ScoreAuditAppender interface {
	Append(ctx context.Context, e *domain.AuditEvent) error
}

// WithAudit attaches the audit trail. The engine rewrites a risk's score outside
// any request, so without this the score an auditor sees has no recorded origin.
func (w *ScoreWorker) WithAudit(a ScoreAuditAppender) *ScoreWorker {
	w.audit = a
	return w
}

// RiskRepository est l'interface minimale requise par le worker.
// Signatures alignées sur GormRiskRepository (uuid.UUID, domain.RiskForScoring) -
// les events Redis restent en string (JSON), parsés en uuid.UUID à la frontière
// du worker avant tout appel au repository.
type RiskRepository interface {
	// UpdateScore met à jour score + criticality d'un risque.
	// OBLIGATOIRE: filtre par tenant_id ET id (isolation stricte).
	// Si aucune ligne affectée → retourner une erreur (risque introuvable).
	UpdateScore(ctx context.Context, riskID, tenantID uuid.UUID, score float64, criticality string) error

	// GetRisksByAssetID retourne tous les risques liés à un asset.
	GetRisksByAssetID(ctx context.Context, assetID, tenantID uuid.UUID) ([]domain.RiskForScoring, error)

	// GetRiskScore retourne le score actuel d'un risque (pour les events).
	GetRiskScore(ctx context.Context, riskID, tenantID uuid.UUID) (float64, error)
}

// NewScoreWorker crée une nouvelle instance.
func NewScoreWorker(
	redis *redisclient.Client,
	engine scoring.Engine,
	riskRepo RiskRepository,
	logger zerolog.Logger,
) *ScoreWorker {
	return &ScoreWorker{
		redis:    redis,
		engine:   engine,
		riskRepo: riskRepo,
		logger:   logger,
	}
}

// Start démarre l'écoute des events. Bloquant jusqu'à ctx.Done().
func (w *ScoreWorker) Start(ctx context.Context) {
	pubsub := w.redis.Subscribe(ctx, events.RiskUpdated, events.AssetCriticalityChanged)
	defer pubsub.Close()

	ch := pubsub.Channel()

	w.logger.Info().Msg("score worker started, listening for events")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info().Msg("score worker shutting down")
			return

		case msg := <-ch:
			if msg == nil {
				// Channel closed
				return
			}

			switch msg.Channel {
			case events.RiskUpdated:
				w.handleRiskUpdatedEvent(ctx, msg.Payload)

			case events.AssetCriticalityChanged:
				w.handleAssetCriticalityChangedEvent(ctx, msg.Payload)
			}
		}
	}
}

// handleRiskUpdatedEvent traite un événement risk.updated.
func (w *ScoreWorker) handleRiskUpdatedEvent(ctx context.Context, payload string) {
	startTime := time.Now()

	var event events.RiskUpdatedEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		w.logger.Error().
			Err(err).
			Str("payload", payload).
			Msg("failed to unmarshal RiskUpdatedEvent")
		return
	}

	riskID, err := uuid.Parse(event.RiskID)
	if err != nil {
		w.logger.Warn().
			Err(err).
			Str("risk_id", event.RiskID).
			Msg("risk.updated event has malformed risk_id, skipping")
		return
	}
	tenantID, err := uuid.Parse(event.TenantID)
	if err != nil {
		w.logger.Warn().
			Err(err).
			Str("tenant_id", event.TenantID).
			Msg("risk.updated event has malformed tenant_id, skipping")
		return
	}

	w.logger.Debug().
		Str("risk_id", event.RiskID).
		Str("tenant_id", event.TenantID).
		Float64("probability", event.Probability).
		Float64("impact", event.Impact).
		Float64("asset_criticality", event.AssetCriticality).
		Msg("processing risk.updated event")

	// Calculate score using the Score Engine
	breakdown, err := w.engine.Breakdown(
		event.Probability,
		event.Impact,
		event.AssetCriticality,
		nil,
	)
	if err != nil {
		w.logger.Error().
			Err(err).
			Str("risk_id", event.RiskID).
			Msg("score calculation failed")
		return
	}

	// Read the score being replaced BEFORE replacing it. It used to be read
	// after the write, so the published old_score was always the new score and
	// every delta was zero.
	oldScore, oldErr := w.riskRepo.GetRiskScore(ctx, riskID, tenantID)
	if oldErr != nil {
		w.logger.Warn().
			Err(oldErr).
			Str("risk_id", event.RiskID).
			Msg("failed to retrieve old risk score")
		oldScore = 0
	}

	// Update database with retry logic
	if err := w.retryUpdateScore(ctx, riskID, tenantID, breakdown); err != nil {
		w.logger.Error().
			Err(err).
			Str("risk_id", event.RiskID).
			Str("tenant_id", event.TenantID).
			Msg("failed to update risk score after retries")
		return
	}

	if oldErr == nil {
		w.journalScoreChange(ctx, riskID, tenantID, oldScore, event, breakdown)
	}

	// Publish risk.score_updated event
	scoreUpdatedEvent := events.RiskScoreUpdatedEvent{
		RiskID:       event.RiskID,
		TenantID:     event.TenantID,
		NewScore:     breakdown.Score,
		OldScore:     oldScore,
		Delta:        breakdown.Score - oldScore,
		Criticality:  string(breakdown.Criticality),
		CalculatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if err := w.redis.Publish(ctx, events.RiskScoreUpdated, scoreUpdatedEvent); err != nil {
		w.logger.Error().
			Err(err).
			Str("risk_id", event.RiskID).
			Msg("failed to publish risk.score_updated event")
		// Don't fail — the score is already in DB
	}

	durationMs := time.Since(startTime).Milliseconds()
	w.logger.Info().
		Str("risk_id", event.RiskID).
		Float64("new_score", breakdown.Score).
		Int64("duration_ms", durationMs).
		Msg("risk score calculated and updated")
}

// handleAssetCriticalityChangedEvent traite un événement asset.criticality_changed.
func (w *ScoreWorker) handleAssetCriticalityChangedEvent(ctx context.Context, payload string) {
	var event events.AssetCriticalityChangedEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		w.logger.Error().
			Err(err).
			Str("payload", payload).
			Msg("failed to unmarshal AssetCriticalityChangedEvent")
		return
	}

	w.logger.Debug().
		Str("asset_id", event.AssetID).
		Str("old_criticality", event.OldCriticality).
		Str("new_criticality", event.NewCriticality).
		Msg("processing asset.criticality_changed event")

	assetID, err := uuid.Parse(event.AssetID)
	if err != nil {
		w.logger.Warn().
			Err(err).
			Str("asset_id", event.AssetID).
			Msg("asset.criticality_changed event has malformed asset_id, skipping")
		return
	}
	tenantID, err := uuid.Parse(event.TenantID)
	if err != nil {
		w.logger.Warn().
			Err(err).
			Str("tenant_id", event.TenantID).
			Msg("asset.criticality_changed event has malformed tenant_id, skipping")
		return
	}

	// Get all risks linked to this asset
	risks, err := w.riskRepo.GetRisksByAssetID(ctx, assetID, tenantID)
	if err != nil {
		w.logger.Error().
			Err(err).
			Str("asset_id", event.AssetID).
			Msg("failed to get risks for asset")
		return
	}

	w.logger.Debug().
		Str("asset_id", event.AssetID).
		Int("affected_risks", len(risks)).
		Msg("republishing risk.updated for affected risks")

	// Republish risk.updated for each affected risk
	for _, risk := range risks {
		riskEvent := events.RiskUpdatedEvent{
			RiskID:           risk.ID.String(),
			TenantID:         risk.TenantID.String(),
			Probability:      risk.Probability,
			Impact:           risk.Impact,
			AssetCriticality: risk.AssetCriticality,
			TriggeredBy:      "system", // System-triggered recalculation
		}

		if err := w.redis.Publish(ctx, events.RiskUpdated, riskEvent); err != nil {
			w.logger.Error().
				Err(err).
				Str("risk_id", risk.ID.String()).
				Msg("failed to republish risk.updated event")
		}
	}

	w.logger.Info().
		Str("asset_id", event.AssetID).
		Int("affected_risks", len(risks)).
		Msg("asset criticality change triggered risk recalculations")
}

// scoreEpsilon is half the stored precision (numeric(8,3)): below it the
// score did not change as far as anyone reading it can tell.
const scoreEpsilon = 0.0005

// journalScoreChange appends one chained audit entry for a score the engine
// actually moved, attributed to the job by name and carrying the exact terms
// it multiplied — so the entry alone lets a reader redo the arithmetic.
// Best-effort, like every audit write: the score is already stored.
func (w *ScoreWorker) journalScoreChange(
	ctx context.Context,
	riskID, tenantID uuid.UUID,
	oldScore float64,
	event events.RiskUpdatedEvent,
	breakdown scoring.ScoreBreakdown,
) {
	if w.audit == nil {
		return
	}
	if d := breakdown.Score - oldScore; d < scoreEpsilon && d > -scoreEpsilon {
		return
	}
	ev := domain.ScoreEngineAuditEvent(tenantID, riskID, oldScore, domain.ScoreEngineTerms{
		Probability:      breakdown.Probability,
		Impact:           breakdown.Impact,
		AssetCriticality: breakdown.AssetCriticality,
		Score:            breakdown.Score,
		Criticality:      string(breakdown.Criticality),
		Explanation:      breakdown.Explanation,
		TriggeredBy:      event.TriggeredBy,
	})
	if err := w.audit.Append(ctx, ev); err != nil {
		w.logger.Error().
			Err(err).
			Str("risk_id", riskID.String()).
			Msg("failed to journal score change")
	}
}

// retryUpdateScore met à jour le risque avec logique de retry.
// Retry 3× avec backoff: 100ms → 500ms → 2s
// Si toujours en échec → logger l'erreur (ne pas panic)
func (w *ScoreWorker) retryUpdateScore(
	ctx context.Context,
	riskID, tenantID uuid.UUID,
	breakdown scoring.ScoreBreakdown,
) error {
	retries := 3
	backoffs := []time.Duration{
		100 * time.Millisecond,
		500 * time.Millisecond,
		2 * time.Second,
	}

	var lastErr error
	for attempt := 0; attempt < retries; attempt++ {
		err := w.riskRepo.UpdateScore(
			ctx,
			riskID,
			tenantID,
			breakdown.Score,
			string(breakdown.Criticality),
		)
		if err == nil {
			return nil
		}

		lastErr = err
		if attempt < retries-1 {
			w.logger.Warn().
				Err(err).
				Str("risk_id", riskID.String()).
				Int("attempt", attempt+1).
				Int("max_retries", retries).
				Msg("UpdateScore failed, retrying...")

			time.Sleep(backoffs[attempt])
		}
	}

	return fmt.Errorf("UpdateScore failed after %d retries: %w", retries, lastErr)
}
