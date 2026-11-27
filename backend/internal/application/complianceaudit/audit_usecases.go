// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package complianceaudit holds the use cases for compliance audits (plan,
// execute, archive) and remediation plans (create, assign, track). Each use case
// is a small injectable struct; CRUD for one aggregate is grouped per file for
// readability. Every method is tenant-scoped and returns typed domain errors.
package complianceaudit

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// -------------------- Create --------------------

// CreateAuditInput is the payload to schedule a new audit.
type CreateAuditInput struct {
	Title          string
	FrameworkID    *uuid.UUID
	Type           string
	Auditor        string
	Scope          string
	ScheduledStart *time.Time
	ScheduledEnd   *time.Time
}

// FrameworkLookup is the narrow, tenant-scoped port that proves a framework
// belongs to the caller's organisation. GetFrameworkByID returns (nil, nil) when
// the framework is absent or owned by another tenant (#1520).
type FrameworkLookup interface {
	GetFrameworkByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.ComplianceFramework, error)
}

type CreateAuditUseCase struct {
	repo       domain.ComplianceAuditRepository
	frameworks FrameworkLookup
}

func NewCreateAuditUseCase(repo domain.ComplianceAuditRepository, frameworks FrameworkLookup) *CreateAuditUseCase {
	return &CreateAuditUseCase{repo: repo, frameworks: frameworks}
}

func (uc *CreateAuditUseCase) Execute(ctx context.Context, tenantID, createdBy uuid.UUID, in CreateAuditInput) (*domain.ComplianceAudit, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no tenant")
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, domain.NewValidationError("title is required")
	}
	auditType, err := domain.ParseAuditType(in.Type)
	if err != nil {
		return nil, err
	}

	// #1520: the framework must be this tenant's. A foreign id and an unknown id
	// answer the same not-found, so the reply says nothing about whether the
	// framework exists elsewhere. Checked before anything is written.
	if in.FrameworkID != nil {
		fw, err := uc.frameworks.GetFrameworkByID(ctx, *in.FrameworkID, tenantID)
		if err != nil {
			return nil, err
		}
		if fw == nil {
			return nil, domain.NewNotFoundError("framework", in.FrameworkID.String())
		}
	}

	audit := &domain.ComplianceAudit{
		TenantID:       tenantID,
		Title:          title,
		FrameworkID:    in.FrameworkID,
		Type:           auditType,
		Status:         domain.AuditStatusPlanned,
		Auditor:        strings.TrimSpace(in.Auditor),
		Scope:          in.Scope,
		ScheduledStart: in.ScheduledStart,
		ScheduledEnd:   in.ScheduledEnd,
	}
	if createdBy != uuid.Nil {
		audit.CreatedBy = &createdBy
	}
	if err := uc.repo.CreateAudit(ctx, audit); err != nil {
		return nil, err
	}
	return audit, nil
}

// -------------------- List --------------------

type ListAuditsUseCase struct {
	repo domain.ComplianceAuditRepository
}

func NewListAuditsUseCase(repo domain.ComplianceAuditRepository) *ListAuditsUseCase {
	return &ListAuditsUseCase{repo: repo}
}

func (uc *ListAuditsUseCase) Execute(ctx context.Context, tenantID uuid.UUID) ([]domain.ComplianceAudit, error) {
	return uc.repo.ListAudits(ctx, tenantID)
}

// -------------------- Get --------------------

type GetAuditUseCase struct {
	repo domain.ComplianceAuditRepository
}

func NewGetAuditUseCase(repo domain.ComplianceAuditRepository) *GetAuditUseCase {
	return &GetAuditUseCase{repo: repo}
}

func (uc *GetAuditUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID) (*domain.ComplianceAudit, error) {
	audit, err := uc.repo.GetAuditByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if audit == nil {
		return nil, domain.NewNotFoundError("audit", id)
	}
	return audit, nil
}

// -------------------- Update --------------------

// UpdateAuditInput carries the editable fields. Nil pointers leave a field
// unchanged; Status is applied as-is (empty keeps the current status).
type UpdateAuditInput struct {
	Title           *string
	FrameworkID     *uuid.UUID
	ClearFramework  bool // set FrameworkID back to nil (program-wide)
	Type            *string
	Status          *string
	Auditor         *string
	Scope           *string
	Summary         *string
	ComplianceScore *float64
	ScheduledStart  *time.Time
	ScheduledEnd    *time.Time
}

type UpdateAuditUseCase struct {
	repo       domain.ComplianceAuditRepository
	frameworks FrameworkLookup
}

func NewUpdateAuditUseCase(repo domain.ComplianceAuditRepository, frameworks FrameworkLookup) *UpdateAuditUseCase {
	return &UpdateAuditUseCase{repo: repo, frameworks: frameworks}
}

func (uc *UpdateAuditUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID, in UpdateAuditInput) (*domain.ComplianceAudit, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no tenant")
	}
	audit, err := uc.repo.GetAuditByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if audit == nil {
		return nil, domain.NewNotFoundError("audit", id)
	}

	// #1520: a framework_id from the body must be this tenant's, same answer as
	// the create path for a foreign or unknown id. Checked before any field is
	// assigned, so a refusal writes nothing.
	if !in.ClearFramework && in.FrameworkID != nil {
		fw, err := uc.frameworks.GetFrameworkByID(ctx, *in.FrameworkID, tenantID)
		if err != nil {
			return nil, err
		}
		if fw == nil {
			return nil, domain.NewNotFoundError("framework", in.FrameworkID.String())
		}
	}

	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if t == "" {
			return nil, domain.NewValidationError("title cannot be empty")
		}
		audit.Title = t
	}
	if in.ClearFramework {
		audit.FrameworkID = nil
	} else if in.FrameworkID != nil {
		audit.FrameworkID = in.FrameworkID
	}
	if in.Type != nil {
		at, err := domain.ParseAuditType(*in.Type)
		if err != nil {
			return nil, err
		}
		audit.Type = at
	}
	if in.Status != nil {
		st, err := domain.ParseAuditStatus(*in.Status)
		if err != nil {
			return nil, err
		}
		audit.Status = st
		// Completing the audit stamps CompletedAt once; reopening clears it.
		if st == domain.AuditStatusCompleted && audit.CompletedAt == nil {
			now := time.Now().UTC()
			audit.CompletedAt = &now
		}
		if st != domain.AuditStatusCompleted {
			audit.CompletedAt = nil
		}
	}
	if in.Auditor != nil {
		audit.Auditor = strings.TrimSpace(*in.Auditor)
	}
	if in.Scope != nil {
		audit.Scope = *in.Scope
	}
	if in.Summary != nil {
		audit.Summary = *in.Summary
	}
	if in.ComplianceScore != nil {
		audit.ComplianceScore = *in.ComplianceScore
	}
	if in.ScheduledStart != nil {
		audit.ScheduledStart = in.ScheduledStart
	}
	if in.ScheduledEnd != nil {
		audit.ScheduledEnd = in.ScheduledEnd
	}

	if err := uc.repo.UpdateAudit(ctx, audit); err != nil {
		return nil, err
	}
	return audit, nil
}

// -------------------- Delete --------------------

type DeleteAuditUseCase struct {
	repo domain.ComplianceAuditRepository
}

func NewDeleteAuditUseCase(repo domain.ComplianceAuditRepository) *DeleteAuditUseCase {
	return &DeleteAuditUseCase{repo: repo}
}

func (uc *DeleteAuditUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID) error {
	return uc.repo.DeleteAudit(ctx, id, tenantID)
}
