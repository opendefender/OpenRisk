// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// CreateControlMappingInput is the payload for a new cross-framework mapping.
type CreateControlMappingInput struct {
	SourceControlID uuid.UUID
	TargetControlID uuid.UUID
	Relation        string
	Note            string
}

// CreateControlMappingUseCase links two of the tenant's controls (normally in
// different frameworks). It validates BOTH controls belong to the tenant — a
// double cross-tenant guard — and refuses self-links and duplicates (either
// direction).
type CreateControlMappingUseCase struct {
	repo     domain.ControlMappingRepository
	compRepo domain.ComplianceRepository
}

func NewCreateControlMappingUseCase(repo domain.ControlMappingRepository, compRepo domain.ComplianceRepository) *CreateControlMappingUseCase {
	return &CreateControlMappingUseCase{repo: repo, compRepo: compRepo}
}

func (uc *CreateControlMappingUseCase) Execute(ctx context.Context, tenantID, createdBy uuid.UUID, in CreateControlMappingInput) (*domain.ControlMapping, error) {
	if in.SourceControlID == in.TargetControlID {
		return nil, domain.NewValidationError("a control cannot be mapped to itself")
	}
	relation, err := domain.ParseMappingRelation(in.Relation)
	if err != nil {
		return nil, err
	}

	// Both controls must belong to THIS tenant (GetControlByID returns nil for
	// another tenant's control, so a forged id can never map across tenants).
	src, err := uc.compRepo.GetControlByID(ctx, in.SourceControlID, tenantID)
	if err != nil {
		return nil, err
	}
	if src == nil {
		return nil, domain.NewValidationError("source control not found")
	}
	tgt, err := uc.compRepo.GetControlByID(ctx, in.TargetControlID, tenantID)
	if err != nil {
		return nil, err
	}
	if tgt == nil {
		return nil, domain.NewValidationError("target control not found")
	}

	exists, err := uc.repo.Exists(ctx, tenantID, in.SourceControlID, in.TargetControlID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.NewConflictError("control mapping", "control pair")
	}

	m := &domain.ControlMapping{
		TenantID:        tenantID,
		SourceControlID: in.SourceControlID,
		TargetControlID: in.TargetControlID,
		Relation:        relation,
		Note:            in.Note,
	}
	if createdBy != uuid.Nil {
		m.CreatedBy = &createdBy
	}
	if err := uc.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	// Enrich the response so the UI can render both sides immediately.
	uc.enrichOne(ctx, tenantID, m, map[uuid.UUID]*domain.ComplianceControl{src.ID: src, tgt.ID: tgt}, map[uuid.UUID]*domain.ComplianceFramework{})
	return m, nil
}

// ListControlMappingsUseCase returns the tenant's crosswalks, optionally scoped
// to one control, enriched with each side's code/name/framework.
type ListControlMappingsUseCase struct {
	repo     domain.ControlMappingRepository
	compRepo domain.ComplianceRepository
}

func NewListControlMappingsUseCase(repo domain.ControlMappingRepository, compRepo domain.ComplianceRepository) *ListControlMappingsUseCase {
	return &ListControlMappingsUseCase{repo: repo, compRepo: compRepo}
}

func (uc *ListControlMappingsUseCase) Execute(ctx context.Context, tenantID uuid.UUID, controlID *uuid.UUID) ([]domain.ControlMapping, error) {
	mappings, err := uc.repo.List(ctx, tenantID, controlID)
	if err != nil {
		return nil, err
	}
	ctrlCache := map[uuid.UUID]*domain.ComplianceControl{}
	fwCache := map[uuid.UUID]*domain.ComplianceFramework{}
	for i := range mappings {
		if err := uc.enrichOne(ctx, tenantID, &mappings[i], ctrlCache, fwCache); err != nil {
			return nil, err
		}
	}
	return mappings, nil
}

// enrichOne fills the computed source/target code/name/framework fields, using
// caches to avoid re-querying shared controls/frameworks.
func (uc *ListControlMappingsUseCase) enrichOne(ctx context.Context, tenantID uuid.UUID, m *domain.ControlMapping, ctrlCache map[uuid.UUID]*domain.ComplianceControl, fwCache map[uuid.UUID]*domain.ComplianceFramework) error {
	src, err := uc.lookupControl(ctx, tenantID, m.SourceControlID, ctrlCache)
	if err != nil {
		return err
	}
	tgt, err := uc.lookupControl(ctx, tenantID, m.TargetControlID, ctrlCache)
	if err != nil {
		return err
	}
	if src != nil {
		m.SourceCode, m.SourceName, m.SourceFrameworkID = src.ReferenceCode, src.Name, src.FrameworkID.String()
		if fw := uc.lookupFramework(ctx, tenantID, src.FrameworkID, fwCache); fw != nil {
			m.SourceFrameworkName = fw.Name
		}
	}
	if tgt != nil {
		m.TargetCode, m.TargetName, m.TargetFrameworkID = tgt.ReferenceCode, tgt.Name, tgt.FrameworkID.String()
		if fw := uc.lookupFramework(ctx, tenantID, tgt.FrameworkID, fwCache); fw != nil {
			m.TargetFrameworkName = fw.Name
		}
	}
	return nil
}

func (uc *ListControlMappingsUseCase) lookupControl(ctx context.Context, tenantID, id uuid.UUID, cache map[uuid.UUID]*domain.ComplianceControl) (*domain.ComplianceControl, error) {
	if c, ok := cache[id]; ok {
		return c, nil
	}
	c, err := uc.compRepo.GetControlByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	cache[id] = c
	return c, nil
}

func (uc *ListControlMappingsUseCase) lookupFramework(ctx context.Context, tenantID, id uuid.UUID, cache map[uuid.UUID]*domain.ComplianceFramework) *domain.ComplianceFramework {
	if fw, ok := cache[id]; ok {
		return fw
	}
	fw, err := uc.compRepo.GetFrameworkByID(ctx, id, tenantID)
	if err != nil {
		fw = nil
	}
	cache[id] = fw
	return fw
}

// enrichOne on the create use case reuses the list use case's logic via a small
// shim so both paths return the same shape.
func (uc *CreateControlMappingUseCase) enrichOne(ctx context.Context, tenantID uuid.UUID, m *domain.ControlMapping, ctrlCache map[uuid.UUID]*domain.ComplianceControl, fwCache map[uuid.UUID]*domain.ComplianceFramework) {
	lister := &ListControlMappingsUseCase{repo: uc.repo, compRepo: uc.compRepo}
	_ = lister.enrichOne(ctx, tenantID, m, ctrlCache, fwCache)
}

// DeleteControlMappingUseCase removes a crosswalk (tenant-scoped).
type DeleteControlMappingUseCase struct {
	repo domain.ControlMappingRepository
}

func NewDeleteControlMappingUseCase(repo domain.ControlMappingRepository) *DeleteControlMappingUseCase {
	return &DeleteControlMappingUseCase{repo: repo}
}

func (uc *DeleteControlMappingUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID) error {
	return uc.repo.Delete(ctx, id, tenantID)
}
