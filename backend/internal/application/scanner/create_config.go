// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package scanner

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	scanpkg "github.com/opendefender/openrisk/internal/scanner"
)

// CreateScanConfigInput is the request to register a new scan configuration.
// Credentials are plaintext here (from the API body) and encrypted before they
// ever touch the DB — they are never returned or logged.
type CreateScanConfigInput struct {
	Name        string
	Provider    domain.ScannerProvider
	Credentials map[string]string // cloud only
	Regions     []string          // cloud only
	Targets     []string          // agent/nmap only
	AgentIDs    []uuid.UUID       // agent/nmap only
	Options     map[string]any
	// ScheduleMinutes > 0 makes the scan recurring (0 = manual only).
	ScheduleMinutes int
}

// CreateScanConfigUseCase validates and persists a ScanConfig for a tenant.
type CreateScanConfigUseCase struct {
	repo     domain.ScanConfigRepository
	registry *scanpkg.Registry
	cipher   *CredentialCipher
}

func NewCreateScanConfigUseCase(repo domain.ScanConfigRepository, registry *scanpkg.Registry, cipher *CredentialCipher) *CreateScanConfigUseCase {
	return &CreateScanConfigUseCase{repo: repo, registry: registry, cipher: cipher}
}

func (uc *CreateScanConfigUseCase) Execute(ctx context.Context, tenantID, createdBy uuid.UUID, in CreateScanConfigInput) (*domain.ScanConfig, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("missing tenant")
	}
	if in.Name == "" {
		return nil, domain.NewValidationError("name is required")
	}
	if !in.Provider.Valid() {
		return nil, domain.NewValidationError("invalid provider (expected aws|azure|gcp|nmap|agent|kubernetes|docker|vmware|active_directory|m365|github|gitlab)")
	}

	// Validate via the provider's Scanner (cred presence for cloud, target scope
	// for agent/nmap) against a runtime config carrying the plaintext creds.
	s, ok := uc.registry.Get(in.Provider)
	if !ok {
		return nil, domain.NewValidationError("provider not supported on this deployment")
	}
	runtime := scanpkg.ScanConfig{
		TenantID:    tenantID,
		Provider:    in.Provider,
		Credentials: in.Credentials,
		Regions:     in.Regions,
		Targets:     in.Targets,
		Options:     in.Options,
	}
	if err := s.Validate(ctx, runtime); err != nil {
		return nil, err
	}

	encCreds, err := uc.cipher.EncryptCredentials(in.Credentials)
	if err != nil {
		return nil, err
	}

	var optionsJSON []byte
	if len(in.Options) > 0 {
		if optionsJSON, err = json.Marshal(in.Options); err != nil {
			return nil, domain.NewValidationError("invalid options: " + err.Error())
		}
	}

	agentIDs := make([]string, 0, len(in.AgentIDs))
	for _, id := range in.AgentIDs {
		agentIDs = append(agentIDs, id.String())
	}

	cfg := &domain.ScanConfig{
		ID:                   uuid.New(),
		TenantID:             tenantID,
		Name:                 in.Name,
		Provider:             in.Provider,
		Enabled:              true,
		EncryptedCredentials: encCreds,
		Regions:              in.Regions,
		Targets:              in.Targets,
		AgentIDs:             agentIDs,
		Options:              optionsJSON,
		ScheduleMinutes:      in.ScheduleMinutes,
		CreatedBy:            createdBy,
	}
	if in.ScheduleMinutes > 0 {
		next := time.Now().Add(time.Duration(in.ScheduleMinutes) * time.Minute)
		cfg.NextRunAt = &next
	}
	if err := uc.repo.Create(ctx, cfg); err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	// Never leak credentials back out.
	cfg.EncryptedCredentials = ""
	return cfg, nil
}
