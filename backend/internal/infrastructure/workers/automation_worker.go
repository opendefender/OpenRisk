// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package workers

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	appauto "github.com/opendefender/openrisk/internal/application/automation"
	"github.com/opendefender/openrisk/internal/domain"
	redisclient "github.com/opendefender/openrisk/internal/infrastructure/redis"
	"github.com/opendefender/openrisk/pkg/events"
	"github.com/rs/zerolog"
)

// AutomationWorker is the event-driven front of the SOAR engine. It subscribes
// to the platform's Redis event channels and, for each event, builds a
// normalised TriggerContext and hands it to the Engine, which matches the
// tenant's rules and runs their action chains.
//
// Channels consumed:
//   - vulnerability.detected → trigger vulnerability_detected (headline scenario)
//   - risk.score_updated     → trigger risk_score_updated
//
// Graceful shutdown on ctx.Done(). Malformed payloads are logged and skipped —
// one bad message never stops the loop.
type AutomationWorker struct {
	redis  *redisclient.Client
	engine *appauto.Engine
	logger zerolog.Logger
}

// NewAutomationWorker builds the worker.
func NewAutomationWorker(redis *redisclient.Client, engine *appauto.Engine, logger zerolog.Logger) *AutomationWorker {
	return &AutomationWorker{redis: redis, engine: engine, logger: logger}
}

// Start blocks listening for events until ctx is cancelled.
func (w *AutomationWorker) Start(ctx context.Context) {
	pubsub := w.redis.Subscribe(ctx, events.VulnerabilityDetected, events.RiskScoreUpdated)
	defer pubsub.Close()
	ch := pubsub.Channel()
	w.logger.Info().Msg("automation worker started (SOAR engine, listening for triggers)")
	for {
		select {
		case <-ctx.Done():
			w.logger.Info().Msg("automation worker shutting down")
			return
		case msg := <-ch:
			if msg == nil {
				return
			}
			switch msg.Channel {
			case events.VulnerabilityDetected:
				w.handleVulnerabilityDetected(ctx, msg.Payload)
			case events.RiskScoreUpdated:
				w.handleRiskScoreUpdated(ctx, msg.Payload)
			}
		}
	}
}

func (w *AutomationWorker) handleVulnerabilityDetected(ctx context.Context, payload string) {
	var evt events.VulnerabilityDetectedEvent
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
		w.logger.Warn().Err(err).Msg("automation: bad vulnerability.detected payload")
		return
	}
	tenantID, err := uuid.Parse(evt.TenantID)
	if err != nil {
		return
	}
	tc := appauto.TriggerContext{
		TenantID:     tenantID,
		Ref:          refFor("cve", evt.CVEID, evt.VulnerabilityID),
		Subject:      firstNonEmptyStr(evt.Title, evt.CVEID),
		Title:        firstNonEmptyStr(evt.Title, evt.CVEID),
		Severity:     evt.Severity,
		CVSS:         evt.CVSS,
		KEV:          evt.KEV,
		PriorityTier: evt.PriorityTier,
		CVEID:        evt.CVEID,
		AssetName:    evt.AssetName,
	}
	if id, err := uuid.Parse(evt.AssetID); err == nil && id != uuid.Nil {
		tc.AssetID = &id
	}
	if id, err := uuid.Parse(evt.TriggeredBy); err == nil {
		tc.TriggeredBy = id
	}
	w.engine.HandleTrigger(ctx, domain.TriggerVulnerabilityDetected, tc)
}

func (w *AutomationWorker) handleRiskScoreUpdated(ctx context.Context, payload string) {
	var evt events.RiskScoreUpdatedEvent
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
		w.logger.Warn().Err(err).Msg("automation: bad risk.score_updated payload")
		return
	}
	tenantID, err := uuid.Parse(evt.TenantID)
	if err != nil {
		return
	}
	riskID, err := uuid.Parse(evt.RiskID)
	if err != nil {
		return
	}
	tc := appauto.TriggerContext{
		TenantID: tenantID,
		Ref:      refFor("risk", evt.RiskID, ""),
		Subject:  "Risk score updated",
		Severity: evt.Criticality,
		RiskID:   &riskID,
	}
	w.engine.HandleTrigger(ctx, domain.TriggerRiskScoreUpdated, tc)
}

func refFor(kind, primary, fallback string) string {
	v := primary
	if v == "" {
		v = fallback
	}
	if v == "" {
		return kind
	}
	return kind + ":" + v
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
