// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AssetType classifies a discovered asset. It is a string so that it stays
// assignment-compatible with domain.Asset.Type (a plain string). The scanner
// pipeline populates AssetDiscovery.Type with one of these; the importer copies
// it verbatim into Asset.Type when the user promotes a discovery to inventory.
type AssetType string

const (
	AssetTypeServer      AssetType = "Server"
	AssetTypeVM          AssetType = "VM"
	AssetTypeContainer   AssetType = "Container"
	AssetTypeDatabase    AssetType = "Database"
	AssetTypeStorage     AssetType = "Storage"
	AssetTypeFunction    AssetType = "Function"
	AssetTypeNetwork     AssetType = "Network"
	AssetTypeIdentity    AssetType = "Identity"
	AssetTypeWorkstation AssetType = "Workstation"
	AssetTypeSaaS        AssetType = "SaaS"
	AssetTypeUnknown     AssetType = "Unknown"
)

// ScannerProvider enumerates the supported scan backends.
//
// ABSOLUTE RULE (Master Prompt V5): only "nmap" and "agent" are agent-based;
// nmap/osquery are NEVER executed inside the SaaS backend — an on-prem Agent
// runs them and pushes results. The cloud providers (aws/azure/gcp) run inside
// the SaaS worker using the official SDKs.
type ScannerProvider string

const (
	ProviderAWS   ScannerProvider = "aws"
	ProviderAzure ScannerProvider = "azure"
	ProviderGCP   ScannerProvider = "gcp"
	ProviderNmap  ScannerProvider = "nmap"
	ProviderAgent ScannerProvider = "agent"
)

// IsAgentBased reports whether the provider is executed by an on-prem Agent
// rather than by the SaaS backend itself.
func (p ScannerProvider) IsAgentBased() bool {
	return p == ProviderNmap || p == ProviderAgent
}

// IsCloud reports whether the provider is a cloud SDK scanner run in the SaaS.
func (p ScannerProvider) IsCloud() bool {
	return p == ProviderAWS || p == ProviderAzure || p == ProviderGCP
}

// Valid reports whether the provider string is one of the known constants.
func (p ScannerProvider) Valid() bool {
	switch p {
	case ProviderAWS, ProviderAzure, ProviderGCP, ProviderNmap, ProviderAgent:
		return true
	default:
		return false
	}
}

// ScanConfig is a persisted, tenant-scoped scan configuration.
//
// For cloud providers it carries AES-256-GCM-encrypted credentials
// (EncryptedCredentials) that are decrypted ONLY at scan time inside the worker
// — never logged, never returned to the client. For agent/nmap providers it
// carries the target CIDR/hosts and the list of Agent IDs authorised to run it.
type ScanConfig struct {
	ID       uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID       `gorm:"type:uuid;not null;index" json:"tenant_id"`
	Name     string          `gorm:"size:255;not null" json:"name"`
	Provider ScannerProvider `gorm:"size:20;not null;index" json:"provider"`
	Enabled  bool            `gorm:"default:true" json:"enabled"`

	// Cloud credentials: base64(AES-256-GCM(nonce+ciphertext)). Decrypted only at
	// scan time. NEVER serialised to JSON — the `json:"-"` tag keeps it out of API
	// responses and logs.
	EncryptedCredentials string `gorm:"type:text" json:"-"`

	// Cloud scoping. Regions to enumerate (empty = provider default / all).
	Regions pq.StringArray `gorm:"type:text[];default:'{}'" json:"regions"`

	// Agent/nmap scoping. Targets are CIDRs or host lists. AgentIDs lists the
	// Agents allowed to pick up this config's jobs (empty = any tenant agent).
	// AgentIDs is a text[] of UUID strings (lib/pq here predates pq.UUIDArray) —
	// use AgentUUIDs() for the typed slice.
	Targets  pq.StringArray `gorm:"type:text[];default:'{}'" json:"targets"`
	AgentIDs pq.StringArray `gorm:"type:text[];default:'{}'" json:"agent_ids"`

	// Options is a free-form provider-specific option bag (nmap flags, severity
	// floor, feature toggles). Kept as JSONB so new options never require a
	// migration.
	Options datatypes.JSON `gorm:"type:jsonb" json:"options,omitempty"`

	// Recurring schedule. ScheduleMinutes = 0 means manual-only; > 0 makes the
	// ScanScheduler trigger this config every N minutes. NextRunAt is when it is
	// next due; LastRunAt is the last scheduled trigger.
	ScheduleMinutes int        `gorm:"default:0" json:"schedule_minutes"`
	LastRunAt       *time.Time `json:"last_run_at,omitempty"`
	NextRunAt       *time.Time `gorm:"index" json:"next_run_at,omitempty"`

	CreatedBy uuid.UUID      `gorm:"type:uuid;index" json:"created_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides the default GORM table name.
func (ScanConfig) TableName() string { return "scan_configs" }

// AgentUUIDs parses the stored AgentIDs (text[] of UUID strings) into a typed
// slice, silently skipping any malformed entry.
func (c *ScanConfig) AgentUUIDs() []uuid.UUID {
	out := make([]uuid.UUID, 0, len(c.AgentIDs))
	for _, s := range c.AgentIDs {
		if id, err := uuid.Parse(s); err == nil {
			out = append(out, id)
		}
	}
	return out
}

// AgentStatus is the connectivity/activity state of a registered Agent.
type AgentStatus string

const (
	AgentOnline   AgentStatus = "online"
	AgentOffline  AgentStatus = "offline"
	AgentScanning AgentStatus = "scanning"
	AgentError    AgentStatus = "error"
	AgentRevoked  AgentStatus = "revoked"
)

// ScannerAgent is an on-prem OpenRisk Scanner Agent registered by a tenant.
// It is stateless on the client side; the SaaS tracks its identity, health and
// the SHA-256 hash of its scoped "scanner" token (never the token itself).
type ScannerAgent struct {
	ID            uuid.UUID   `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID      uuid.UUID   `gorm:"type:uuid;not null;index" json:"tenant_id"`
	Name          string      `gorm:"size:255;not null" json:"name"`
	Version       string      `gorm:"size:50" json:"version"`
	Status        AgentStatus `gorm:"size:20;default:'offline';index" json:"status"`
	LastHeartbeat time.Time   `json:"last_heartbeat"`
	IP            string      `gorm:"size:64" json:"ip"`
	Hostname      string      `gorm:"size:255" json:"hostname"`
	OS            string      `gorm:"size:64" json:"os"`
	RegisteredAt  time.Time   `json:"registered_at"`

	// TokenHash is the SHA-256 (hex) of the Agent's current scoped token. Used
	// to authenticate pushes and to support instant revocation + 7-day rotation.
	TokenHash string `gorm:"size:64;index" json:"-"`

	// PushSecretEnc is AES-256-GCM(base64) of the per-agent HMAC secret handed to
	// the Agent at registration. Pushes are signed HMAC-SHA256 with it (second
	// factor on top of the bearer JWT). Decrypted only to verify a push signature;
	// never returned to the client.
	PushSecretEnc string `gorm:"type:text" json:"-"`

	// RegistrationConfigID ties the Agent to the ScanConfig whose registration
	// token was used to enrol it.
	RegistrationConfigID *uuid.UUID `gorm:"type:uuid;index" json:"registration_config_id,omitempty"`

	LastScanJobID  *uuid.UUID     `gorm:"type:uuid" json:"last_scan_job_id,omitempty"`
	TokenRotatedAt time.Time      `json:"token_rotated_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides the default GORM table name.
func (ScannerAgent) TableName() string { return "scanner_agents" }

// ScanJobStatus is the lifecycle state of a scan run.
type ScanJobStatus string

const (
	ScanQueued    ScanJobStatus = "queued"    // created, awaiting a runner
	ScanClaimed   ScanJobStatus = "claimed"   // an agent/worker holds the Redis lock
	ScanRunning   ScanJobStatus = "running"   // enumeration in progress
	ScanCompleted ScanJobStatus = "completed" // preview stored in Redis
	ScanFailed    ScanJobStatus = "failed"    // errored out
	ScanTimeout   ScanJobStatus = "timeout"   // exceeded the hard deadline
)

// ScanJob is a single scan run for a ScanConfig. It is the coordination record
// between the SaaS and the runner (cloud worker or on-prem agent). The job NEVER
// persists assets/risks itself — on completion it points at a Redis preview key.
type ScanJob struct {
	ID       uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID       `gorm:"type:uuid;not null;index" json:"tenant_id"`
	ConfigID uuid.UUID       `gorm:"type:uuid;not null;index" json:"config_id"`
	Provider ScannerProvider `gorm:"size:20;not null" json:"provider"`
	Status   ScanJobStatus   `gorm:"size:20;not null;default:'queued';index" json:"status"`

	// Targets is a frozen snapshot of the config's targets at trigger time, so an
	// agent runs exactly what was queued even if the config later changes.
	Targets pq.StringArray `gorm:"type:text[];default:'{}'" json:"targets"`

	// ClaimedByAgent is set once an agent wins the Redis lock (nil for cloud jobs).
	ClaimedByAgent *uuid.UUID `gorm:"type:uuid;index" json:"claimed_by_agent,omitempty"`

	// PreviewKey is the Redis key holding the full ScanPreview (TTL 48h) once the
	// job completes. The user imports/ignores from that preview.
	PreviewKey string `gorm:"size:255" json:"preview_key,omitempty"`

	AssetsFound   int    `gorm:"default:0" json:"assets_found"`
	FindingsFound int    `gorm:"default:0" json:"findings_found"`
	Error         string `gorm:"type:text" json:"error,omitempty"`

	TriggeredBy uuid.UUID  `gorm:"type:uuid" json:"triggered_by"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TableName overrides the default GORM table name.
func (ScanJob) TableName() string { return "scan_jobs" }

// --- Repository ports (implemented in infrastructure) ---------------------
//
// ABSOLUTE RULE: every method filters by tenant_id in the repository, never in
// the handler. A row belonging to another tenant is returned as (nil, nil).

// ScanConfigRepository persists scan configurations.
type ScanConfigRepository interface {
	Create(ctx context.Context, cfg *ScanConfig) error
	GetByID(ctx context.Context, id, tenantID uuid.UUID) (*ScanConfig, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]ScanConfig, error)
	Update(ctx context.Context, cfg *ScanConfig) error
	Delete(ctx context.Context, id, tenantID uuid.UUID) error
	// ListDueScheduled returns enabled recurring configs whose NextRunAt is due
	// (or unset). It is intentionally NOT tenant-scoped: the ScanScheduler worker
	// runs across all tenants and re-derives the tenant from each config.
	ListDueScheduled(ctx context.Context, now time.Time) ([]ScanConfig, error)
	// UpdateNextRun advances a config's schedule bookkeeping after a run.
	UpdateNextRun(ctx context.Context, id, tenantID uuid.UUID, lastRun, nextRun time.Time) error
}

// ScannerAgentRepository persists registered agents.
type ScannerAgentRepository interface {
	Create(ctx context.Context, agent *ScannerAgent) error
	GetByID(ctx context.Context, id, tenantID uuid.UUID) (*ScannerAgent, error)
	// GetByTokenHash resolves an agent from the hash of its scoped token. Used to
	// authenticate agent pushes/streams. Returns (nil, nil) if unknown/revoked.
	GetByTokenHash(ctx context.Context, tokenHash string) (*ScannerAgent, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]ScannerAgent, error)
	Update(ctx context.Context, agent *ScannerAgent) error
	Delete(ctx context.Context, id, tenantID uuid.UUID) error
}

// ScanJobRepository persists scan jobs.
type ScanJobRepository interface {
	Create(ctx context.Context, job *ScanJob) error
	GetByID(ctx context.Context, id, tenantID uuid.UUID) (*ScanJob, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]ScanJob, error)
	// ListByStatus returns a tenant's jobs in a given status (e.g. queued agent
	// jobs an agent may claim).
	ListByStatus(ctx context.Context, tenantID uuid.UUID, status ScanJobStatus) ([]ScanJob, error)
	// CountActiveByTenant counts jobs currently occupying a runner (claimed or
	// running) — used to enforce the max-3-concurrent-scans-per-tenant rule.
	CountActiveByTenant(ctx context.Context, tenantID uuid.UUID) (int64, error)
	Update(ctx context.Context, job *ScanJob) error
}
