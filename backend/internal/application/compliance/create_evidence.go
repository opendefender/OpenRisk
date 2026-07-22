// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import (
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/storage"
)

// CreateEvidenceInput carries the uploaded file content alongside metadata.
// UploadedBy must come from the authenticated request context (middleware),
// never trusted from client-supplied body fields.
type CreateEvidenceInput struct {
	ControlID   uuid.UUID
	Filename    string
	Description string
	Content     io.Reader
	UploadedBy  uuid.UUID
}

// CreateEvidenceUseCase attaches an uploaded file as evidence to a control.
type CreateEvidenceUseCase struct {
	repo    domain.ComplianceRepository
	storage storage.Storage
}

func NewCreateEvidenceUseCase(repo domain.ComplianceRepository, store storage.Storage) *CreateEvidenceUseCase {
	return &CreateEvidenceUseCase{repo: repo, storage: store}
}

func (uc *CreateEvidenceUseCase) Execute(ctx context.Context, tenantID uuid.UUID, input CreateEvidenceInput) (*domain.ControlEvidence, error) {
	if input.Filename == "" {
		return nil, domain.NewValidationError("filename is required")
	}
	if input.Content == nil {
		return nil, domain.NewValidationError("file content is required")
	}

	// Verify the control exists and belongs to this tenant BEFORE touching
	// storage — never let a tenant attach evidence to a control it can't
	// see (which would also mean it can't be reached again to delete it).
	control, err := uc.repo.GetControlByID(ctx, input.ControlID, tenantID)
	if err != nil {
		return nil, err
	}
	if control == nil {
		return nil, domain.NewNotFoundError("control", input.ControlID)
	}

	key, err := uc.storage.Save(ctx, tenantID, input.Filename, input.Content)
	if err != nil {
		return nil, domain.NewInternalError("failed to store evidence file: " + err.Error())
	}

	evidence := &domain.ControlEvidence{
		ID:          uuid.New(),
		TenantID:    tenantID,
		ControlID:   input.ControlID,
		Filename:    input.Filename,
		URL:         key,
		Description: input.Description,
		UploadedBy:  &input.UploadedBy,
	}

	if err := uc.repo.CreateEvidence(ctx, evidence); err != nil {
		// Don't leave an orphaned file behind if the DB write fails.
		_ = uc.storage.Delete(ctx, key)
		return nil, err
	}
	return evidence, nil
}
