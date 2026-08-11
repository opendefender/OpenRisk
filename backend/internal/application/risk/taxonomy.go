// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// Categories — the controlled vocabulary an admin curates
// ---------------------------------------------------------------------------

// CategoryUseCase is the CRUD of the tenant's classification vocabulary.
type CategoryUseCase struct {
	repo domain.RiskCategoryRepository
}

func NewCategoryUseCase(repo domain.RiskCategoryRepository) *CategoryUseCase {
	return &CategoryUseCase{repo: repo}
}

// List returns the tenant's categories, seeding the defaults for a tenant that
// has never had any. Seeding on first READ (not at signup) means existing
// tenants get the vocabulary too, without a backfill that would guess at what
// they want.
func (uc *CategoryUseCase) List(ctx context.Context, tenantID uuid.UUID, includeInactive, withCounts bool) ([]domain.RiskCategory, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewValidationError("tenant is required")
	}
	if err := uc.repo.SeedDefaults(ctx, tenantID); err != nil {
		// Seeding is a convenience, not a precondition: a tenant with no
		// categories still gets an (empty) list rather than an error.
		_ = err
	}
	rows, err := uc.repo.List(ctx, tenantID, includeInactive)
	if err != nil {
		return nil, err
	}
	if withCounts {
		counts, err := uc.repo.CountRisks(ctx, tenantID)
		if err == nil { // degrade to no counts rather than failing the list
			for i := range rows {
				rows[i].RiskCount = counts[rows[i].ID]
			}
		}
	}
	return rows, nil
}

// CategoryInput is the create/update payload.
type CategoryInput struct {
	Name        string
	Description string
	Color       string
	SortOrder   int
	Active      *bool // nil on create → true
}

func (uc *CategoryUseCase) Create(ctx context.Context, tenantID uuid.UUID, in CategoryInput) (*domain.RiskCategory, error) {
	c := &domain.RiskCategory{
		TenantID:    tenantID,
		Name:        strings.TrimSpace(in.Name),
		Description: in.Description,
		Color:       defaultString(in.Color, "neutral"),
		SortOrder:   in.SortOrder,
		Active:      in.Active == nil || *in.Active,
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	// The vocabulary is CONTROLLED: two entries with the same key would let the
	// same concept be counted twice in a dashboard, which is exactly what tags
	// are for and categories are not.
	exists, err := uc.repo.ExistsBySlug(ctx, tenantID, c.Slug, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.NewConflictError("risk category", "name")
	}
	if err := uc.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (uc *CategoryUseCase) Update(ctx context.Context, tenantID, id uuid.UUID, in CategoryInput) (*domain.RiskCategory, error) {
	c, err := uc.repo.GetByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, domain.NewNotFoundError("risk category", id)
	}
	if strings.TrimSpace(in.Name) != "" {
		c.Name = strings.TrimSpace(in.Name)
		// Slug is NOT recomputed on rename: saved filters and links point at it.
	}
	if in.Description != "" {
		c.Description = in.Description
	}
	if in.Color != "" {
		c.Color = in.Color
	}
	if in.SortOrder != 0 {
		c.SortOrder = in.SortOrder
	}
	if in.Active != nil {
		c.Active = *in.Active
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if err := uc.repo.Update(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (uc *CategoryUseCase) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return uc.repo.Delete(ctx, id, tenantID)
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

// ---------------------------------------------------------------------------
// Control mappings — the "Référentiel" column
// ---------------------------------------------------------------------------

// ControlResolver checks that a framework/control pair really exists in THIS
// tenant before a mapping is written. Narrow port, satisfied structurally by the
// compliance repository — a mapping must never be able to point at another
// organisation's framework.
type ControlResolver interface {
	GetFrameworkByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.ComplianceFramework, error)
	GetControlByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.ComplianceControl, error)
}

// ControlMappingUseCase manages a risk's references to compliance controls.
type ControlMappingUseCase struct {
	mappings domain.RiskControlMappingRepository
	risks    domain.RiskRepository
	controls ControlResolver
}

func NewControlMappingUseCase(mappings domain.RiskControlMappingRepository, risks domain.RiskRepository) *ControlMappingUseCase {
	return &ControlMappingUseCase{mappings: mappings, risks: risks}
}

// WithControls attaches the compliance resolver. Optional and nil-safe, but
// without it a mapping is written unverified — so main.go always wires it.
func (uc *ControlMappingUseCase) WithControls(c ControlResolver) *ControlMappingUseCase {
	uc.controls = c
	return uc
}

// CreateMappingInput links a risk to a framework, optionally narrowed to one
// control.
type CreateMappingInput struct {
	RiskID      uuid.UUID
	FrameworkID uuid.UUID
	ControlID   *uuid.UUID
	Note        string
	CreatedBy   uuid.UUID
}

func (uc *ControlMappingUseCase) Create(ctx context.Context, tenantID uuid.UUID, in CreateMappingInput) (*domain.RiskControlMapping, error) {
	if in.RiskID == uuid.Nil {
		return nil, domain.NewValidationError("risk_id is required")
	}
	if in.FrameworkID == uuid.Nil && in.ControlID == nil {
		return nil, domain.NewValidationError("framework_id or control_id is required")
	}

	// The risk must be ours. Cross-tenant reads as not found.
	r, err := uc.risks.GetByID(ctx, in.RiskID, tenantID)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, domain.NewNotFoundError("risk", in.RiskID)
	}

	frameworkID := in.FrameworkID
	if uc.controls != nil {
		// A control narrows — and DEFINES — the framework: taking framework_id
		// from the body when a control is named would let the two disagree.
		if in.ControlID != nil {
			ctrl, err := uc.controls.GetControlByID(ctx, *in.ControlID, tenantID)
			if err != nil {
				return nil, err
			}
			if ctrl == nil {
				return nil, domain.NewNotFoundError("compliance control", *in.ControlID)
			}
			frameworkID = ctrl.FrameworkID
		} else {
			fw, err := uc.controls.GetFrameworkByID(ctx, frameworkID, tenantID)
			if err != nil {
				return nil, err
			}
			if fw == nil {
				return nil, domain.NewNotFoundError("compliance framework", frameworkID)
			}
		}
	}

	exists, err := uc.mappings.Exists(ctx, tenantID, in.RiskID, frameworkID, in.ControlID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.NewConflictError("risk control mapping", "control_id")
	}

	m := &domain.RiskControlMapping{
		TenantID:    tenantID,
		RiskID:      in.RiskID,
		FrameworkID: frameworkID,
		ControlID:   in.ControlID,
		Note:        in.Note,
		Source:      domain.SourceManual,
	}
	if in.CreatedBy != uuid.Nil {
		author := in.CreatedBy
		m.CreatedBy = &author
	}
	if err := uc.mappings.Create(ctx, m); err != nil {
		return nil, err
	}

	// Return it enriched, so the caller can render the badge immediately.
	enriched, err := uc.mappings.ListByRisk(ctx, tenantID, in.RiskID)
	if err == nil {
		for i := range enriched {
			if enriched[i].ID == m.ID {
				return &enriched[i], nil
			}
		}
	}
	return m, nil
}

func (uc *ControlMappingUseCase) ListForRisk(ctx context.Context, tenantID, riskID uuid.UUID) ([]domain.RiskControlMapping, error) {
	return uc.mappings.ListByRisk(ctx, tenantID, riskID)
}

func (uc *ControlMappingUseCase) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return uc.mappings.Delete(ctx, id, tenantID)
}

// UnmappedRisk is one row of /risks/unmapped.
type UnmappedRisk struct {
	ID          uuid.UUID               `json:"id"`
	Title       string                  `json:"title"`
	Score       float64                 `json:"score"`
	Criticality domain.CriticalityLevel `json:"criticality"`
	State       domain.RiskState        `json:"lifecycle_state"`
	CreatedAt   string                  `json:"created_at"`
}

// ListUnmapped backs the /risks/unmapped screen: the risks nobody has mapped to
// a control yet, worst first.
//
// The screen exists because mapping is OPTIONAL at creation — forcing it there
// would only teach people to pick the first framework in the list. Making the
// backlog visible afterwards is the honest version of the same goal.
func (uc *ControlMappingUseCase) ListUnmapped(ctx context.Context, tenantID uuid.UUID) ([]UnmappedRisk, error) {
	ids, err := uc.mappings.UnmappedRiskIDs(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]UnmappedRisk, 0, len(ids))
	for _, id := range ids {
		r, err := uc.risks.GetByID(ctx, id, tenantID)
		if err != nil || r == nil {
			continue
		}
		out = append(out, UnmappedRisk{
			ID:          r.ID,
			Title:       r.Title,
			Score:       r.Score,
			Criticality: r.Criticality,
			State:       r.State(),
			CreatedAt:   r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return out, nil
}
