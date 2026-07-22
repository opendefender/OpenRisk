// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package scanner is the OpenRisk scan engine. It defines the Scanner port,
// the discovery DTOs pushed by cloud SDKs and on-prem Agents, and the pipeline
// that turns raw discoveries into a Redis preview.
//
// ABSOLUTE RULES (Master Prompt V5):
//   - nmap/osquery are NEVER executed inside this backend — an on-prem Agent
//     runs them and pushes results. Only cloud providers run in-process here.
//   - The pipeline NEVER creates Assets or Risks in the database. Everything
//     stays in a Redis preview (48h TTL) until the user imports/ignores it.
//   - Zero secrets in logs. Cloud credentials are decrypted only at scan time.
package scanner

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// Severity levels for findings (aligned with CTI + Security Hub vocabularies).
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
	SeverityInfo     = "info"
)

// severityRank orders severities so callers can filter (e.g. ≥ medium).
var severityRank = map[string]int{
	SeverityInfo:     0,
	SeverityLow:      1,
	SeverityMedium:   2,
	SeverityHigh:     3,
	SeverityCritical: 4,
}

// SeverityAtLeast reports whether sev is >= floor (both normalised lower-case).
func SeverityAtLeast(sev, floor string) bool {
	return severityRank[normalizeSeverity(sev)] >= severityRank[normalizeSeverity(floor)]
}

// Scanner is the port every scan backend implements. Cloud scanners run inside
// the SaaS worker; agent-based scanners (nmap/agent) only Validate here — the
// actual Scan happens on the on-prem Agent, which pushes discoveries back.
type Scanner interface {
	// Name is a human label, e.g. "AWS Cloud Scanner".
	Name() string
	// Provider is the provider key: "aws"|"azure"|"gcp"|"nmap"|"agent".
	Provider() string
	// Scan streams discoveries. The three channels are all closed when the scan
	// finishes; errors are non-fatal per-item unless a fatal error is the last
	// value sent on the error channel before it closes.
	Scan(ctx context.Context, cfg ScanConfig) (<-chan AssetDiscovery, <-chan FindingDiscovery, <-chan error)
	// Validate checks the config is runnable (creds present/decryptable, targets
	// in scope, etc.) without performing a scan.
	Validate(ctx context.Context, cfg ScanConfig) error
	// IsAgentBased is true only for "nmap" and "agent".
	IsAgentBased() bool
}

// ScanConfig is the runtime configuration handed to a Scanner. It is distinct
// from domain.ScanConfig (the persisted entity): credentials here are ALREADY
// DECRYPTED, and it carries the concrete job/agent context. It is never
// persisted and never logged.
type ScanConfig struct {
	ConfigID  uuid.UUID
	TenantID  uuid.UUID
	ScanJobID uuid.UUID
	Provider  domain.ScannerProvider

	// Cloud: decrypted credential material (e.g. access_key_id / secret_access_key
	// / session_token for AWS; tenant_id/client_id/client_secret/subscription for
	// Azure; service_account_json for GCP).
	Credentials map[string]string
	Regions     []string

	// Agent/nmap: the targets to sweep and the specific Agent that owns the job.
	Targets []string
	AgentID *uuid.UUID

	// Options is a provider-specific option bag (nmap flags, severity floor…).
	Options map[string]any
}

// AssetDiscovery is a single asset found by a scan. It is a preview DTO — it is
// NOT a domain.Asset and is never written to the DB by the pipeline.
type AssetDiscovery struct {
	ExternalID  string           `json:"external_id"`
	Name        string           `json:"name"`
	Type        domain.AssetType `json:"type"`
	IP          *string          `json:"ip,omitempty"`
	Hostname    *string          `json:"hostname,omitempty"`
	OS          *string          `json:"os,omitempty"`
	OSVersion   *string          `json:"os_version,omitempty"`
	CPE         []string         `json:"cpe"`         // critical for CTI matching + auto mitigation
	Criticality float64          `json:"criticality"` // inferred [0.1, 3.0]
	Environment string           `json:"environment"` // prod|staging|dev|...
	Tags        []string         `json:"tags"`
	Location    *string          `json:"location,omitempty"`
	RawMetadata map[string]any   `json:"raw_metadata,omitempty"`

	ScanJobID uuid.UUID  `json:"scan_job_id"`
	AgentID   *uuid.UUID `json:"agent_id,omitempty"` // populated only for agent-based scans
}

// FindingDiscovery is a single vulnerability/misconfiguration found by a scan.
type FindingDiscovery struct {
	CVE             *string        `json:"cve,omitempty"`
	Title           string         `json:"title"`
	Description     string         `json:"description"`
	Severity        string         `json:"severity"` // critical|high|medium|low
	AffectedCPE     []string       `json:"affected_cpe"`
	Evidence        string         `json:"evidence"`         // proof: open port, service, vulnerable software…
	RemediationHint string         `json:"remediation_hint"` //nolint:tagliatelle
	Source          string         `json:"source"`           // security-hub|defender|nmap|osquery|agent
	RawFinding      map[string]any `json:"raw_finding,omitempty"`

	// AssetExternalID links a finding back to the discovered asset it belongs to
	// (its ExternalID). Lets the importer attach findings/risks to the right asset.
	AssetExternalID string `json:"asset_external_id,omitempty"`

	ScanJobID uuid.UUID  `json:"scan_job_id"`
	AgentID   *uuid.UUID `json:"agent_id,omitempty"`
}

// AutoMitigation is a finding that was present in the previous scan of the same
// config but is absent now — i.e. it appears to have been remediated. Surfaced
// on the "Auto-detected Mitigations" tab of the preview for the user to confirm.
type AutoMitigation struct {
	AssetExternalID string    `json:"asset_external_id"`
	CVE             *string   `json:"cve,omitempty"`
	Title           string    `json:"title"`
	Severity        string    `json:"severity"`
	Evidence        string    `json:"evidence"` // why we think it's fixed
	DetectedAt      time.Time `json:"detected_at"`
}

// ScanPreview is the full, immutable snapshot of a scan run, held in Redis with
// a 48h TTL. The user imports or ignores from this — nothing is in the DB yet.
type ScanPreview struct {
	JobID     uuid.UUID              `json:"job_id"`
	ConfigID  uuid.UUID              `json:"config_id"`
	TenantID  uuid.UUID              `json:"tenant_id"`
	Provider  domain.ScannerProvider `json:"provider"`
	AgentID   *uuid.UUID             `json:"agent_id,omitempty"`
	AgentName string                 `json:"agent_name,omitempty"`
	// TriggeredBy is the user who launched the scan — used to target the
	// completion notification (in-app + email).
	TriggeredBy uuid.UUID `json:"triggered_by,omitempty"`

	Assets      []AssetDiscovery   `json:"assets"`
	Findings    []FindingDiscovery `json:"findings"`
	Mitigations []AutoMitigation   `json:"mitigations"`

	Errors    []string  `json:"errors,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
