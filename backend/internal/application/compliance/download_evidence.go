// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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
