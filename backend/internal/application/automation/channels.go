// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package automation

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/netguard"
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

	WebhookEnabled bool
	WebhookURL     string
	WebhookSecret  string

	SMSEnabled     bool
	SMSGatewayURL  string
	SMSAPIKey      string
	SMSSender      string
	SMSRecipients  string
	SMSToField     string
	SMSTextField   string
	SMSSenderField string
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
	if err := checkChannelURLs(in); err != nil {
		return nil, err
	}
	cfg := &domain.AutomationChannelConfig{
		TenantID:        tenantID,
		SlackEnabled:    in.SlackEnabled,
		SlackWebhookURL: in.SlackWebhookURL,
		TeamsEnabled:    in.TeamsEnabled,
		TeamsWebhookURL: in.TeamsWebhookURL,
		EmailEnabled:    in.EmailEnabled,
		DefaultEmail:    in.DefaultEmail,
		WebhookEnabled:  in.WebhookEnabled,
		WebhookURL:      in.WebhookURL,
		WebhookSecret:   in.WebhookSecret,
		SMSEnabled:      in.SMSEnabled,
		SMSGatewayURL:   in.SMSGatewayURL,
		SMSAPIKey:       in.SMSAPIKey,
		SMSSender:       in.SMSSender,
		SMSRecipients:   in.SMSRecipients,
		SMSToField:      in.SMSToField,
		SMSTextField:    in.SMSTextField,
		SMSSenderField:  in.SMSSenderField,
	}
	if err := s.repo.Upsert(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// checkChannelURLs refuses, at save time, a webhook or gateway URL the server
// must not call (#1315): anything but https, or a host that is a loopback,
// link-local (cloud metadata) or private address. Empty keeps the stored value
// and is not checked here; the send path re-checks every URL it is given.
func checkChannelURLs(in ChannelInput) error {
	for _, f := range []struct{ name, url string }{
		{"slack_webhook_url", in.SlackWebhookURL},
		{"teams_webhook_url", in.TeamsWebhookURL},
		{"webhook_url", in.WebhookURL},
		{"sms_gateway_url", in.SMSGatewayURL},
	} {
		if strings.TrimSpace(f.url) == "" {
			continue
		}
		if err := netguard.ValidateURL(f.url); err != nil {
			return domain.NewOutboundDeniedError(f.name, err)
		}
	}
	return nil
}
