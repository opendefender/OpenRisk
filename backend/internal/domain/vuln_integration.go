// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// VulnIntegration is a tenant-scoped configuration for one vulnerability scanner
// connector (Nessus, OpenVAS, Qualys, MS Defender, AWS Inspector, Azure Defender,
// CrowdStrike). It carries the connection details (base URL + encrypted API
// credentials), the live-pull schedule, the inbound webhook token, and the
// automation toggles (auto-create risk / auto-create ticket).
//
// One integration per (tenant, source). Credentials are stored AES-256-GCM
// encrypted and are NEVER returned to the API — only HasCredentials is exposed.
type VulnIntegration struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index:idx_vuln_integration_tenant_source,unique,priority:1" json:"tenant_id"`

	Source  VulnSource `gorm:"type:varchar(24);not null;index:idx_vuln_integration_tenant_source,unique,priority:2" json:"source"`
	Name    string     `gorm:"size:128" json:"name"`
	Enabled bool       `gorm:"default:true" json:"enabled"`

	// Connection — BaseURL is the tool's API endpoint (Qualys pod, Nessus host,
	// ServiceNow instance…). EncryptedCredentials is the AES-256-GCM JSON of the
	// per-source credential map (api keys / secrets / tenant ids).
	BaseURL              string `gorm:"size:512" json:"base_url"`
	EncryptedCredentials string `gorm:"type:text" json:"-"`

	// Live pull — periodic API polling. ScheduleMinutes == 0 means manual only.
	LivePullEnabled bool       `gorm:"default:false" json:"live_pull_enabled"`
	ScheduleMinutes int        `gorm:"default:0" json:"schedule_minutes"`
	LastPullAt      *time.Time `json:"last_pull_at,omitempty"`
	LastPullStatus  string     `gorm:"size:16;default:'never'" json:"last_pull_status"` // never|ok|error
	LastPullError   string     `gorm:"type:text" json:"last_pull_error,omitempty"`
	LastPullCount   int        `gorm:"default:0" json:"last_pull_count"`

	// Inbound webhook — the scanner POSTs findings to
	// /api/v1/vulnerabilities/webhook/:source with this opaque token. Unique so the
	// token alone identifies the integration + tenant.
	WebhookEnabled bool   `gorm:"default:false" json:"webhook_enabled"`
	WebhookToken   string `gorm:"size:80;uniqueIndex" json:"webhook_token,omitempty"`

	// Automation — cross-module wiring, opt-in per integration.
	AutoCreateRisk   bool `gorm:"default:false" json:"auto_create_risk"`   // P1/KEV → Risk Register
	AutoCreateTicket bool `gorm:"default:false" json:"auto_create_ticket"` // P1 → Jira/ServiceNow ticket

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Computed, NOT persisted.
	HasCredentials bool `gorm:"-" json:"has_credentials"`
}

// TableName pins the table name.
func (VulnIntegration) TableName() string { return "vuln_integrations" }

// VulnTicketProvider is the ITSM backend used for auto-ticketing.
type VulnTicketProvider string

const (
	TicketProviderNone       VulnTicketProvider = ""
	TicketProviderJira       VulnTicketProvider = "jira"
	TicketProviderServiceNow VulnTicketProvider = "servicenow"
)

// ParseTicketProvider validates a provider string (empty → none).
func ParseTicketProvider(s string) (VulnTicketProvider, error) {
	switch VulnTicketProvider(s) {
	case TicketProviderNone, TicketProviderJira, TicketProviderServiceNow:
		return VulnTicketProvider(s), nil
	default:
		return "", NewValidationError("invalid ticket provider: " + s)
	}
}

// VulnTicketingConfig is the tenant-level ITSM configuration used to open tickets
// from vulnerabilities. One row per tenant. Credentials are encrypted at rest and
// never returned.
type VulnTicketingConfig struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"tenant_id"`

	Provider VulnTicketProvider `gorm:"type:varchar(16)" json:"provider"`
	Enabled  bool               `gorm:"default:false" json:"enabled"`

	// Connection — BaseURL is the Jira/ServiceNow instance URL. ProjectOrTable is
	// the Jira project key or the ServiceNow table (default: incident).
	BaseURL              string `gorm:"size:512" json:"base_url"`
	ProjectOrTable       string `gorm:"size:128" json:"project_or_table"`
	DefaultIssueType     string `gorm:"size:64;default:'Bug'" json:"default_issue_type"`
	EncryptedCredentials string `gorm:"type:text" json:"-"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Computed, NOT persisted.
	HasCredentials bool `gorm:"-" json:"has_credentials"`
}

// TableName pins the table name.
func (VulnTicketingConfig) TableName() string { return "vuln_ticketing_configs" }

// VulnIntegrationRepository is the persistence port for connector + ticketing
// configuration. ABSOLUTE RULE: every method filters by tenant_id.
type VulnIntegrationRepository interface {
	// Scanner connectors
	UpsertIntegration(ctx context.Context, in *VulnIntegration) error
	GetIntegration(ctx context.Context, id, tenantID uuid.UUID) (*VulnIntegration, error)
	GetIntegrationBySource(ctx context.Context, tenantID uuid.UUID, source VulnSource) (*VulnIntegration, error)
	// GetIntegrationByWebhookToken resolves an integration from its opaque webhook
	// token WITHOUT a tenant filter — the token itself is the tenant credential.
	GetIntegrationByWebhookToken(ctx context.Context, token string) (*VulnIntegration, error)
	ListIntegrations(ctx context.Context, tenantID uuid.UUID) ([]VulnIntegration, error)
	// ListDueForPull returns enabled, live-pull integrations whose schedule elapsed.
	ListDueForPull(ctx context.Context, now time.Time) ([]VulnIntegration, error)
	DeleteIntegration(ctx context.Context, id, tenantID uuid.UUID) error

	// Ticketing (one per tenant)
	UpsertTicketing(ctx context.Context, in *VulnTicketingConfig) error
	GetTicketing(ctx context.Context, tenantID uuid.UUID) (*VulnTicketingConfig, error)
	DeleteTicketing(ctx context.Context, tenantID uuid.UUID) error
}
