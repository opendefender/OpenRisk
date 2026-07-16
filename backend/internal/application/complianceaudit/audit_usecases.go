// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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

type CreateAuditUseCase struct{ repo domain.ComplianceAuditRepository }

func NewCreateAuditUseCase(repo domain.ComplianceAuditRepository) *CreateAuditUseCase {
	return &CreateAuditUseCase{repo: repo}
}

func (uc *CreateAuditUseCase) Execute(ctx context.Context, tenantID, createdBy uuid.UUID, in CreateAuditInput) (*domain.ComplianceAudit, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, domain.NewValidationError("title is required")
	}
	auditType, err := domain.ParseAuditType(in.Type)
	if err != nil {
		return nil, err
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

type ListAuditsUseCase struct{ repo domain.ComplianceAuditRepository }

func NewListAuditsUseCase(repo domain.ComplianceAuditRepository) *ListAuditsUseCase {
	return &ListAuditsUseCase{repo: repo}
}

func (uc *ListAuditsUseCase) Execute(ctx context.Context, tenantID uuid.UUID) ([]domain.ComplianceAudit, error) {
	return uc.repo.ListAudits(ctx, tenantID)
}

// -------------------- Get --------------------

type GetAuditUseCase struct{ repo domain.ComplianceAuditRepository }

func NewGetAuditUseCase(repo domain.ComplianceAuditRepository) *GetAuditUseCase {
	return &GetAuditUseCase{repo: repo}
}

func (uc *GetAuditUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID) (*domain.ComplianceAudit, error) {
	audit, err := uc.repo.GetAuditByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if audit == nil {
		return nil, domain.ErrNotFound
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

type UpdateAuditUseCase struct{ repo domain.ComplianceAuditRepository }

func NewUpdateAuditUseCase(repo domain.ComplianceAuditRepository) *UpdateAuditUseCase {
	return &UpdateAuditUseCase{repo: repo}
}

func (uc *UpdateAuditUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID, in UpdateAuditInput) (*domain.ComplianceAudit, error) {
	audit, err := uc.repo.GetAuditByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if audit == nil {
		return nil, domain.ErrNotFound
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

type DeleteAuditUseCase struct{ repo domain.ComplianceAuditRepository }

func NewDeleteAuditUseCase(repo domain.ComplianceAuditRepository) *DeleteAuditUseCase {
	return &DeleteAuditUseCase{repo: repo}
}

func (uc *DeleteAuditUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID) error {
	return uc.repo.DeleteAudit(ctx, id, tenantID)
}
