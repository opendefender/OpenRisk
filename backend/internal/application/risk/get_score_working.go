// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	pkgscoring "github.com/opendefender/openrisk/pkg/scoring"
)

// scoreWorkingEventWindow bounds how far back the trail is read for one entity.
// A risk edited more often than this still gets its latest source for every
// term in practice; if not, the term says "no recorded source" rather than
// scanning an unbounded history on every drawer open.
const scoreWorkingEventWindow = 500

// scoreWorkingMaxAssets caps the per-asset source lookups (one query each).
const scoreWorkingMaxAssets = 25

// ScoreUserLookup resolves actor ids to emails for display.
type ScoreUserLookup interface {
	EmailsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// GetScoreWorkingUseCase shows a risk's score with its working (#486): the
// frozen formula's terms, the result, and the audit entry behind each term.
type GetScoreWorkingUseCase struct {
	riskRepo domain.RiskRepository
	audit    domain.AuditEventRepository
	engine   pkgscoring.Engine
	lookup   ScoreUserLookup
}

func NewGetScoreWorkingUseCase(
	riskRepo domain.RiskRepository,
	audit domain.AuditEventRepository,
	engine pkgscoring.Engine,
) *GetScoreWorkingUseCase {
	return &GetScoreWorkingUseCase{riskRepo: riskRepo, audit: audit, engine: engine}
}

// WithUserLookup shows actor emails instead of bare ids. Optional.
func (uc *GetScoreWorkingUseCase) WithUserLookup(l ScoreUserLookup) *GetScoreWorkingUseCase {
	uc.lookup = l
	return uc
}

// Execute builds the working for one risk of the caller's tenant.
// canReadAudit gates the provenance: without it the arithmetic is returned and
// every source is withheld (SourcesVisible=false).
func (uc *GetScoreWorkingUseCase) Execute(ctx context.Context, tenantID, riskID uuid.UUID, canReadAudit bool) (*domain.ScoreWorking, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("no tenant in context")
	}
	r, err := uc.riskRepo.GetByID(ctx, riskID, tenantID)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, domain.NewNotFoundError("risk", riskID)
	}

	w := &domain.ScoreWorking{
		RiskID:            r.ID.String(),
		Formula:           domain.ScoreWorkingFormula,
		Assets:            []domain.ScoreWorkingAsset{},
		Stored:            r.Score,
		StoredCriticality: string(r.Criticality),
		SourcesVisible:    canReadAudit && uc.audit != nil,
	}

	// Asset criticality: the average factor over every linked asset, medium when
	// none — the same derivation the Score Engine is fed (GetScoreBreakdown,
	// GormRiskRepository.GetRisksByAssetID).
	ac := domain.CriticalityMedium.ScoreFactor()
	if len(r.Assets) > 0 {
		var sum float64
		for _, a := range r.Assets {
			f := a.Criticality.ScoreFactor()
			sum += f
			w.Assets = append(w.Assets, domain.ScoreWorkingAsset{
				ID:          a.ID.String(),
				Name:        a.Name,
				Criticality: string(a.Criticality),
				Factor:      f,
			})
		}
		ac = sum / float64(len(r.Assets))
	} else {
		w.AssetCriticalityDefaulted = true
	}

	b, err := uc.engine.Breakdown(r.Probability, r.Impact, ac, nil)
	if err != nil {
		return nil, domain.NewInternalError(fmt.Sprintf("score engine rejected the stored terms: %v", err))
	}
	w.Computed = b.Score
	w.Criticality = string(b.Criticality)
	w.Explanation = b.Explanation
	// Stored at numeric(8,3): equal means equal at that precision.
	w.Consistent = math.Abs(b.Score-r.Score) < 0.0005

	w.Terms = []domain.ScoreWorkingTerm{
		{Key: "probability", Value: r.Probability, Min: 0, Max: 1},
		{Key: "impact", Value: r.Impact, Min: 0, Max: 10},
		{Key: "asset_criticality", Value: ac, Min: 0.1, Max: 3},
	}

	if !w.SourcesVisible {
		return w, nil
	}

	riskEvents, _, err := uc.audit.List(ctx, tenantID, domain.AuditEventFilter{
		EntityTypes: []string{"risk"},
		EntityID:    r.ID.String(),
		Limit:       scoreWorkingEventWindow,
	})
	if err != nil {
		return nil, err
	}
	var cited []*domain.AuditEvent
	cite := func(e *domain.AuditEvent) *domain.AuditEvent {
		if e != nil {
			cited = append(cited, e)
		}
		return e
	}
	probSrc := cite(domain.LatestFieldSource(riskEvents, "probability"))
	impactSrc := cite(domain.LatestFieldSource(riskEvents, "impact"))
	scoreSrc := cite(domain.LatestFieldSource(riskEvents, "score"))

	assetSrc := make([]*domain.AuditEvent, len(w.Assets))
	for i := range w.Assets {
		if i >= scoreWorkingMaxAssets {
			break
		}
		evs, _, err := uc.audit.List(ctx, tenantID, domain.AuditEventFilter{
			EntityTypes: []string{"asset"},
			EntityID:    w.Assets[i].ID,
			Limit:       scoreWorkingEventWindow,
		})
		if err != nil {
			return nil, err
		}
		assetSrc[i] = cite(domain.LatestFieldSource(evs, "criticality"))
	}

	uc.resolveEmails(ctx, cited)

	w.Terms[0].Source = domain.ScoreWorkingSourceOf(probSrc, "probability")
	w.Terms[1].Source = domain.ScoreWorkingSourceOf(impactSrc, "impact")
	w.ScoreSource = domain.ScoreWorkingSourceOf(scoreSrc, "score")
	for i := range w.Assets {
		w.Assets[i].Source = domain.ScoreWorkingSourceOf(assetSrc[i], "criticality")
	}
	// asset_criticality is an average, not a stored field: its origin is the
	// per-asset sources above. With exactly one asset that source IS the term's.
	if len(w.Assets) == 1 {
		w.Terms[2].Source = w.Assets[0].Source
	}
	return w, nil
}

func (uc *GetScoreWorkingUseCase) resolveEmails(ctx context.Context, events []*domain.AuditEvent) {
	if uc.lookup == nil || len(events) == 0 {
		return
	}
	set := map[uuid.UUID]struct{}{}
	for _, e := range events {
		if e.ActorID != nil && *e.ActorID != uuid.Nil {
			set[*e.ActorID] = struct{}{}
		}
	}
	if len(set) == 0 {
		return
	}
	ids := make([]uuid.UUID, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	emails, err := uc.lookup.EmailsByIDs(ctx, ids)
	if err != nil {
		return
	}
	for _, e := range events {
		if e.ActorID != nil {
			e.ActorEmail = emails[*e.ActorID]
		}
	}
}
