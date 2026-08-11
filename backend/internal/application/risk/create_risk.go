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

// CreateRiskInput represents the input for creating a risk.
type CreateRiskInput struct {
	Title       string
	Description string
	Impact      float64 // ERD numeric(5,1) — bounds [0,10]
	Probability float64 // ERD numeric(5,3) — bounds [0,1]
	Status      domain.RiskStatus
	Tags        []string
	Frameworks  []string
	Owner       string
	// Ownership assigns the responsable / exécutant / validateur at creation.
	// When no owner is supplied the creator becomes the owner: a risk nobody
	// answers for is how a register rots, and the creator is the only defensible
	// default at this point.
	Ownership  domain.OwnershipPatch
	Source     string // parsed into domain.RiskSource in Execute()
	ExternalID string
	CreatedBy  uuid.UUID // the authenticated user creating the risk
	SLEXAF     *float64  // CRQ: single loss expectancy (XAF), optional
	ARO        *float64  // CRQ: annualized rate of occurrence, optional
	// Full financial-quantification drivers (spec §9). All optional XAF amounts.
	DowntimeHours           *float64
	HourlyDowntimeCostXAF   *float64
	DataLossCostXAF         *float64
	FinesXAF                *float64
	OtherDirectCostXAF      *float64
	RemediationCostXAF      *float64
	MitigationEffectiveness *float64 // [0,1]
}

// ActivationRecorder notes product milestones so activation state is derived from
// SERVER events instead of client guesses. Narrow and satisfied structurally by
// application/activation.Recorder, so this package does not import it.
//
// It returns nothing on purpose: noting "this was their first risk" must never be
// able to fail the creation of the risk.
type ActivationRecorder interface {
	Record(ctx context.Context, tenantID uuid.UUID, key string, payload map[string]interface{})
}

// CreateRiskUseCase handles the creation of a new risk.
type CreateRiskUseCase struct {
	riskRepo   domain.RiskRepository
	activation ActivationRecorder
	ownership  OwnershipManager
}

// NewCreateRiskUseCase creates a new CreateRiskUseCase.
func NewCreateRiskUseCase(riskRepo domain.RiskRepository) *CreateRiskUseCase {
	return &CreateRiskUseCase{riskRepo: riskRepo}
}

// WithActivation attaches the optional activation recorder. Nil-safe.
func (uc *CreateRiskUseCase) WithActivation(rec ActivationRecorder) *CreateRiskUseCase {
	uc.activation = rec
	return uc
}

// WithOwnership attaches the optional ownership manager (membership validation
// + assignment notifications). Nil-safe.
func (uc *CreateRiskUseCase) WithOwnership(m OwnershipManager) *CreateRiskUseCase {
	uc.ownership = m
	return uc
}

// Execute creates a new risk within the specified organization.
func (uc *CreateRiskUseCase) Execute(ctx context.Context, orgID uuid.UUID, input CreateRiskInput) (*domain.Risk, error) {
	// 1. Validate input
	if err := uc.validate(input); err != nil {
		return nil, err
	}

	// Convert the raw source string into the typed domain.RiskSource
	// (empty defaults to SourceManual; anything else must be a known value).
	source, err := domain.ParseRiskSource(input.Source)
	if err != nil {
		return nil, err
	}

	// 2. Build domain entity
	// TenantID is the canonical (not-null) field; OrganizationID is kept as
	// its legacy alias. Risk.BeforeSave() also syncs the two on real GORM
	// writes, but setting both explicitly here avoids depending solely on
	// that hook (e.g. against mocked repositories in tests).
	risk := &domain.Risk{
		ID:             uuid.New(),
		Title:          input.Title,
		Description:    input.Description,
		Impact:         input.Impact,
		Probability:    input.Probability,
		Tags:           input.Tags,
		Frameworks:     input.Frameworks,
		Owner:          input.Owner,
		Source:         source,
		ExternalID:     input.ExternalID,
		TenantID:       orgID,
		OrganizationID: orgID,
		CreatedBy:      input.CreatedBy,
		SLEXAF:         input.SLEXAF,
		ARO:            input.ARO,

		DowntimeHours:           input.DowntimeHours,
		HourlyDowntimeCostXAF:   input.HourlyDowntimeCostXAF,
		DataLossCostXAF:         input.DataLossCostXAF,
		FinesXAF:                input.FinesXAF,
		OtherDirectCostXAF:      input.OtherDirectCostXAF,
		RemediationCostXAF:      input.RemediationCostXAF,
		MitigationEffectiveness: input.MitigationEffectiveness,
	}

	// Set status (default to DRAFT)
	if input.Status != "" {
		risk.Status = input.Status
	} else {
		risk.Status = domain.StatusDraft
	}

	// A newly created risk enters the lifecycle at "Identifier" (ISO 31000).
	risk.LifecyclePhase = domain.PhaseIdentified

	// Ownership: apply what the caller asked for, then guarantee an owner. The
	// creator is the fallback — a risk with no responsable is unactionable, and
	// leaving the slot empty is how registers accumulate orphans.
	ownershipChanges, err := applyOwnership(ctx, uc.ownership, orgID, &risk.Ownership, input.Ownership, input.CreatedBy)
	if err != nil {
		return nil, err
	}
	if risk.OwnerID == nil && input.CreatedBy != uuid.Nil {
		owner := input.CreatedBy
		risk.OwnerID = &owner
	}
	// Keep the legacy assigned_to column in step so pre-0044 readers and the
	// existing facet do not go blind on newly created rows.
	if risk.AssigneeID != nil {
		risk.AssignedTo = risk.AssigneeID
	}

	// 3. Compute score (Claude.md formula: P × I, score engine can override later)
	risk.Score = risk.Impact * risk.Probability

	// 4. Persist
	if err := uc.riskRepo.Create(ctx, risk); err != nil {
		return nil, domain.NewInternalError(fmt.Sprintf("failed to create risk: %v", err))
	}

	// 5. Note the activation milestone. Every creation records an event; only the
	// FIRST one ticks the checklist (the read model takes MIN(occurred_at)), so
	// no counting or de-duplication is needed here.
	if uc.activation != nil {
		uc.activation.Record(ctx, orgID, string(domain.ActivationRiskCreated), map[string]interface{}{
			"risk_id": risk.ID.String(),
			"source":  string(risk.Source),
		})
	}

	// 6. Announce assignments made at creation (never to the creator themselves).
	if uc.ownership != nil && len(ownershipChanges) > 0 {
		uc.ownership.Notify(ctx, orgID, ownershipChanges, domain.OwnershipSubject{
			ResourceType: "risk",
			ResourceID:   risk.ID,
			Title:        risk.Title,
		})
	}

	return risk, nil
}

func (uc *CreateRiskUseCase) validate(input CreateRiskInput) error {
	if input.Title == "" {
		return domain.NewValidationError("title is required")
	}
	if len(input.Title) > 255 {
		return domain.NewValidationError("title must be 255 characters or less")
	}
	if input.Impact < 0 || input.Impact > 10 {
		return domain.NewValidationError("impact must be between 0 and 10")
	}
	if input.Probability < 0 || input.Probability > 1 {
		return domain.NewValidationError("probability must be between 0 and 1")
	}
	if input.MitigationEffectiveness != nil && (*input.MitigationEffectiveness < 0 || *input.MitigationEffectiveness > 1) {
		return domain.NewValidationError("mitigation_effectiveness must be between 0 and 1")
	}
	return nil
}
