// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// The onboarding tunnel's write adapter (#438 step 2, D-012).
//
// It lives HERE, on the risk use case, rather than in application/activation,
// for one reason: risk creation has invariants — the lifecycle state machine,
// the ownership fallback, the synchronous banding — and a second writer that
// reached the repository directly would drift from them silently. The tunnel
// writes real rows into a customer's register; they must be indistinguishable
// from rows a person created, apart from their declared provenance.
//
// `application/activation` never imports this package: it declares
// `domain.StarterRiskWriter` and this satisfies it structurally.
// ---------------------------------------------------------------------------

// Compile-time proof of the port. Without it, a signature drift would surface
// only when main.go is wired.
var _ domain.StarterRiskWriter = (*CreateRiskUseCase)(nil)

// HasStarterRisks reports whether the tenant already adopted starter statements.
//
// This is what makes adoption idempotent. The tunnel is resumable and
// back-navigable by design, so a user WILL return to step 2; without this, every
// return visit would double the register with rows they did not ask for twice.
func (uc *CreateRiskUseCase) HasStarterRisks(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	if tenantID == uuid.Nil {
		return false, domain.NewForbiddenError("a tenant is required to read starter risks")
	}
	if uc.riskRepo == nil {
		return false, nil
	}

	// Through the repository's existing tenant-scoped List rather than a new
	// port method: RiskQuery already filters on `source`, the column is indexed,
	// and adding a method to RiskRepository would ripple into every mock in the
	// tree to answer a question the query language already answers.
	//
	// Limit 1: the answer is a boolean, and paging a register to learn "at least
	// one" is how a fast page becomes a slow one.
	page, err := uc.riskRepo.List(ctx, tenantID, domain.RiskQuery{
		Source: []string{string(domain.SourceStarter)},
		Page:   1,
		Limit:  1,
	})
	if err != nil {
		return false, err
	}
	if page == nil {
		return false, nil
	}
	return page.Total > 0 || len(page.Data) > 0, nil
}

// CreateStarterRisk materialises one adopted statement.
//
// The draft's text has already been re-read from the catalogue by the caller —
// nothing here comes from a request body. Provenance is recorded twice on
// purpose: `Source` says THAT the tunnel wrote it (indexed, queryable), and
// `ExternalID` says WHICH statement it was, so a human reading the row can trace
// it back to the catalogue entry.
func (uc *CreateRiskUseCase) CreateStarterRisk(ctx context.Context, tenantID uuid.UUID, draft domain.StarterRiskDraft) (*domain.Risk, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("a tenant is required to write a starter risk")
	}
	if draft.StarterKey == "" || draft.Title == "" {
		return nil, domain.NewValidationError("a starter risk needs a catalogue key and a title")
	}

	return uc.Execute(ctx, tenantID, CreateRiskInput{
		Title:       draft.Title,
		Description: draft.Description,
		Probability: draft.Probability,
		Impact:      draft.Impact,
		Tags:        draft.Tags,
		Source:      string(domain.SourceStarter),
		ExternalID:  fmt.Sprintf("%s%s", domain.StarterExternalIDPrefix, draft.StarterKey),
		CreatedBy:   draft.CreatedBy,
		// No explicit status: the risk enters the lifecycle at DRAFT like any
		// other, because a statement the user merely recognised is not an
		// assessed risk. Step 4 of the tunnel is where they score one.
	})
}

// ---------------------------------------------------------------------------
// The tunnel's SCORING adapter (#643, step 4).
//
// Same reasoning as the write adapter above, one step later in the tunnel:
// scoring a risk recomputes the score and its band, and a second path that did
// its own arithmetic would be a second definition of the frozen formula. This
// delegates to UpdateRiskUseCase so there is exactly one.
// ---------------------------------------------------------------------------

// StarterScorer satisfies domain.StarterRiskScorer.
type StarterScorer struct {
	riskRepo domain.RiskRepository
	update   *UpdateRiskUseCase
}

// Compile-time proof of the port.
var _ domain.StarterRiskScorer = (*StarterScorer)(nil)

func NewStarterScorer(riskRepo domain.RiskRepository, update *UpdateRiskUseCase) *StarterScorer {
	return &StarterScorer{riskRepo: riskRepo, update: update}
}

// FirstStarterRisk returns the earliest starter risk this tenant adopted.
//
// EARLIEST, not highest-scoring. Step 5 names the risk step 4 scored, and
// ordering by score would let the user's own scoring change which risk that is —
// so the two screens could name different risks precisely because the tunnel
// worked.
//
// (nil, nil) when the tenant adopted none: that is a normal state, not an error.
// Adoption sits on step 2 and is skippable, and the caller has to be able to
// render "nothing to score here" rather than fail the step.
func (s *StarterScorer) FirstStarterRisk(ctx context.Context, tenantID uuid.UUID) (*domain.Risk, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("a tenant is required to read starter risks")
	}
	if s.riskRepo == nil {
		return nil, nil
	}

	page, err := s.riskRepo.List(ctx, tenantID, domain.RiskQuery{
		Source:    []string{string(domain.SourceStarter)},
		SortBy:    "created_at",
		SortOrder: "asc",
		Page:      1,
		Limit:     1,
	})
	if err != nil {
		return nil, err
	}
	if page == nil || len(page.Data) == 0 {
		return nil, nil
	}
	return &page.Data[0], nil
}

// ScoreStarterRisk applies the tunnel's likelihood and impact to one risk.
//
// It re-reads the row under the tenant before writing, so a risk id that belongs
// to another tenant is a NotFound rather than a cross-tenant write — the update
// use case scopes its own read too, and this keeps the refusal explicit here
// rather than relying on a second layer to catch it.
func (s *StarterScorer) ScoreStarterRisk(ctx context.Context, tenantID, riskID uuid.UUID, probability, impact float64) (*domain.Risk, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("a tenant is required to score a starter risk")
	}
	if riskID == uuid.Nil {
		return nil, domain.NewValidationError("a risk id is required to score a starter risk")
	}
	if s.update == nil {
		return nil, domain.NewInternalError("starter risk scorer is not wired")
	}
	// The Score Engine's own bounds. Rejecting here rather than clamping: a
	// slider cannot produce these, so a value outside them means the caller is
	// not the slider, and silently clamping would hide that.
	if probability < 0 || probability > 1 {
		return nil, domain.NewValidationError("probability must be between 0 and 1")
	}
	if impact < 0 || impact > 10 {
		return nil, domain.NewValidationError("impact must be between 0 and 10")
	}

	return s.update.Execute(ctx, tenantID, riskID, UpdateRiskInput{
		Probability: &probability,
		Impact:      &impact,
	})
}
