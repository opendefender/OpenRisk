// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/storage"
)

// DeleteEvidenceUseCase soft-deletes an evidence record and best-effort
// removes its underlying file. Callers are restricted to admins (see RBAC
// wiring in main.go) — whoever can attach evidence cannot also remove it,
// preserving audit-trail integrity.
type DeleteEvidenceUseCase struct {
	repo    domain.ComplianceRepository
	storage storage.Storage
}

func NewDeleteEvidenceUseCase(repo domain.ComplianceRepository, store storage.Storage) *DeleteEvidenceUseCase {
	return &DeleteEvidenceUseCase{repo: repo, storage: store}
}

func (uc *DeleteEvidenceUseCase) Execute(ctx context.Context, tenantID, evidenceID uuid.UUID) error {
	evidence, err := uc.repo.GetEvidenceByID(ctx, evidenceID, tenantID)
	if err != nil {
		return err
	}
	if evidence == nil {
		return domain.NewNotFoundError("evidence", evidenceID)
	}

	if err := uc.repo.DeleteEvidence(ctx, evidenceID, tenantID); err != nil {
		return err
	}

	// Best-effort: the DB record is already gone at this point, so a
	// storage failure here is an orphaned file, not an inconsistent state —
	// not worth failing the whole delete over.
	_ = uc.storage.Delete(ctx, evidence.URL)
	return nil
}
