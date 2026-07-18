// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package automation

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// ChannelInput is the save payload for the tenant alert-channel config. Webhook
// URLs are write-only: an empty string preserves the stored value.
type ChannelInput struct {
	SlackEnabled    bool
	SlackWebhookURL string
	TeamsEnabled    bool
	TeamsWebhookURL string
	EmailEnabled    bool
	DefaultEmail    string
}

// ChannelService manages the tenant's outbound alert-channel configuration
// ("configurer un nouveau canal d'alerte").
type ChannelService struct {
	repo domain.AutomationChannelRepository
}

// NewChannelService builds the channel config service.
func NewChannelService(repo domain.AutomationChannelRepository) *ChannelService {
	return &ChannelService{repo: repo}
}

// Get returns the tenant channel config (a default when none is set).
func (s *ChannelService) Get(ctx context.Context, tenantID uuid.UUID) (*domain.AutomationChannelConfig, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("missing tenant")
	}
	return s.repo.Get(ctx, tenantID)
}

// Save upserts the tenant channel config.
func (s *ChannelService) Save(ctx context.Context, tenantID uuid.UUID, in ChannelInput) (*domain.AutomationChannelConfig, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("missing tenant")
	}
	cfg := &domain.AutomationChannelConfig{
		TenantID:        tenantID,
		SlackEnabled:    in.SlackEnabled,
		SlackWebhookURL: in.SlackWebhookURL,
		TeamsEnabled:    in.TeamsEnabled,
		TeamsWebhookURL: in.TeamsWebhookURL,
		EmailEnabled:    in.EmailEnabled,
		DefaultEmail:    in.DefaultEmail,
	}
	if err := s.repo.Upsert(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
