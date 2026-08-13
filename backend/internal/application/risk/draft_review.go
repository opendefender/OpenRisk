// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// DraftReviewUseCase lists and dispositions the DRAFT risks that automation
// proposed.
//
// Automation may only propose (see internal/infrastructure/vulnrisk): every
// machine-created risk lands in DRAFT. That is safe, but it is only useful if
// the drafts are reviewable in bulk — a queue of fifty proposals that must each
// be opened individually is a queue that gets ignored, which quietly turns the
// whole vuln→risk feature off.
type DraftReviewUseCase struct {
	repo domain.RiskRepository
}

func NewDraftReviewUseCase(repo domain.RiskRepository) *DraftReviewUseCase {
	return &DraftReviewUseCase{repo: repo}
}

// List returns the tenant's proposed drafts, newest first.
func (uc *DraftReviewUseCase) List(ctx context.Context, tenantID uuid.UUID, page, limit int) (*domain.PaginatedResult[domain.Risk], error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("missing tenant context")
	}
	q := domain.NewRiskQuery()
	q.Page, q.Limit = page, limit
	q.Sanitize()
	q.LifecycleState = []string{string(domain.StateDraft)}
	// Only MACHINE-proposed drafts. A human's own draft is their work in
	// progress, not something for someone else to accept or reject in bulk.
	q.Source = []string{string(domain.SourceScanAuto), string(domain.SourceCTIAuto)}

	res, err := uc.repo.List(ctx, tenantID, q)
	if err != nil {
		return nil, domain.NewInternalError("failed to list draft risks: " + err.Error())
	}
	return res, nil
}

// BulkDecision is what to do with a batch of drafts.
type BulkDecision string

const (
	// DecisionAccept promotes a draft into the register (DRAFT → IDENTIFIED).
	DecisionAccept BulkDecision = "accept"
	// DecisionDismiss removes it. A rejected proposal is not a closed risk —
	// closing it would leave a permanent "we had this risk and resolved it"
	// record for something the reviewer said was never a risk at all, and would
	// count toward every posture number computed from closed risks.
	DecisionDismiss BulkDecision = "dismiss"
)

// BulkReviewResult reports what happened, per id.
type BulkReviewResult struct {
	Accepted  []uuid.UUID       `json:"accepted"`
	Dismissed []uuid.UUID       `json:"dismissed"`
	Failed    map[string]string `json:"failed,omitempty"` // risk id → why
}

// BulkReview applies one decision to many drafts.
//
// Partial success is reported rather than rolled back: with fifty drafts, one
// bad id must not discard the reviewer's forty-nine good decisions. Every
// failure is named so nothing disappears silently.
func (uc *DraftReviewUseCase) BulkReview(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID, decision BulkDecision) (*BulkReviewResult, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("missing tenant context")
	}
	if len(ids) == 0 {
		return nil, domain.NewValidationError("no risks selected")
	}
	if decision != DecisionAccept && decision != DecisionDismiss {
		return nil, domain.NewValidationError("decision must be \"accept\" or \"dismiss\"")
	}

	out := &BulkReviewResult{Failed: map[string]string{}}
	for _, id := range ids {
		r, err := uc.repo.GetByID(ctx, id, tenantID)
		if err != nil {
			out.Failed[id.String()] = "lecture impossible"
			continue
		}
		if r == nil {
			out.Failed[id.String()] = "risque introuvable"
			continue
		}
		// Only drafts are reviewable here. Accepting an already-live risk would
		// be a no-op the caller could not distinguish from a real acceptance.
		if r.State() != domain.StateDraft {
			out.Failed[id.String()] = "ce risque n'est plus un brouillon"
			continue
		}

		if decision == DecisionDismiss {
			if err := uc.repo.Delete(ctx, id, tenantID); err != nil {
				out.Failed[id.String()] = "suppression impossible"
				continue
			}
			out.Dismissed = append(out.Dismissed, id)
			continue
		}

		// SetState is the single writer that keeps status and lifecycle_phase
		// derived from the state — assigning Status directly is what let the
		// three disagree before the lifecycle work.
		r.SetState(domain.StateIdentified)
		r.UpdatedAt = time.Now()
		if err := uc.repo.Update(ctx, r); err != nil {
			out.Failed[id.String()] = "mise à jour impossible"
			continue
		}
		out.Accepted = append(out.Accepted, id)
	}
	if len(out.Failed) == 0 {
		out.Failed = nil
	}
	return out, nil
}
