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

// DownloadEvidenceUseCase streams back a previously uploaded evidence file.
// There is deliberately no public/static route for evidence files — this
// use case is the only path to file content, and it re-verifies tenant
// ownership via the repository on every call.
type DownloadEvidenceUseCase struct {
	repo    domain.ComplianceRepository
	storage storage.Storage
}

func NewDownloadEvidenceUseCase(repo domain.ComplianceRepository, store storage.Storage) *DownloadEvidenceUseCase {
	return &DownloadEvidenceUseCase{repo: repo, storage: store}
}

func (uc *DownloadEvidenceUseCase) Execute(ctx context.Context, tenantID, evidenceID uuid.UUID) (*domain.ControlEvidence, io.ReadCloser, error) {
	evidence, err := uc.repo.GetEvidenceByID(ctx, evidenceID, tenantID)
	if err != nil {
		return nil, nil, err
	}
	if evidence == nil {
		return nil, nil, domain.NewNotFoundError("evidence", evidenceID)
	}

	content, err := uc.storage.Open(ctx, evidence.URL)
	if err != nil {
		// The DB record exists but the file is missing — a data-integrity
		// issue, not a "this evidence doesn't exist" 404.
		return nil, nil, domain.NewInternalError("evidence file is missing from storage: " + err.Error())
	}
	return evidence, content, nil
}
