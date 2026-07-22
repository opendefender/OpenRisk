// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package scanner

import (
	"context"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// ListScanConfigsUseCase returns a tenant's scan configurations, with encrypted
// credentials stripped.
type ListScanConfigsUseCase struct {
	repo domain.ScanConfigRepository
}

func NewListScanConfigsUseCase(repo domain.ScanConfigRepository) *ListScanConfigsUseCase {
	return &ListScanConfigsUseCase{repo: repo}
}

func (uc *ListScanConfigsUseCase) Execute(ctx context.Context, tenantID uuid.UUID) ([]domain.ScanConfig, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("missing tenant")
	}
	cfgs, err := uc.repo.List(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	for i := range cfgs {
		cfgs[i].EncryptedCredentials = "" // defence in depth; json:"-" already hides it
	}
	return cfgs, nil
}

// GetScanConfigUseCase returns a single scan config (credentials stripped).
type GetScanConfigUseCase struct {
	repo domain.ScanConfigRepository
}

func NewGetScanConfigUseCase(repo domain.ScanConfigRepository) *GetScanConfigUseCase {
	return &GetScanConfigUseCase{repo: repo}
}

func (uc *GetScanConfigUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID) (*domain.ScanConfig, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("missing tenant")
	}
	cfg, err := uc.repo.GetByID(ctx, id, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if cfg == nil {
		return nil, domain.NewNotFoundError("scan config", id)
	}
	cfg.EncryptedCredentials = ""
	return cfg, nil
}

// DeleteScanConfigUseCase removes a tenant's scan configuration.
type DeleteScanConfigUseCase struct {
	repo domain.ScanConfigRepository
}

func NewDeleteScanConfigUseCase(repo domain.ScanConfigRepository) *DeleteScanConfigUseCase {
	return &DeleteScanConfigUseCase{repo: repo}
}

func (uc *DeleteScanConfigUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID) error {
	if tenantID == uuid.Nil {
		return domain.NewUnauthorizedError("missing tenant")
	}
	existing, err := uc.repo.GetByID(ctx, id, tenantID)
	if err != nil {
		return domain.NewInternalError(err.Error())
	}
	if existing == nil {
		return domain.NewNotFoundError("scan config", id)
	}
	if err := uc.repo.Delete(ctx, id, tenantID); err != nil {
		return domain.NewInternalError(err.Error())
	}
	return nil
}
