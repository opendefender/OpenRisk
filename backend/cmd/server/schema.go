// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/cti"
)

// schemaModels is every model GORM AutoMigrate builds at startup, in order.
//
// It lives outside main() so it can be tested (#706). With the SQL migration
// layer skipped unless DATABASE_URL is set (#611), this list IS the schema of a
// default deployment: a model a repository reads but that is missing here has
// no table, and every route that touches it answers 500.
func schemaModels() []interface{} {
	return []interface{}{
		&domain.User{},
		&domain.Organization{},
		&domain.OrganizationMember{},
		// Organization invitations (W0-04, migration 0058). The previous
		// invitations model was never in AutoMigrate, so the table never existed
		// and the invitation code that referenced it could not have run.
		&domain.Invitation{},
		&coreauth.RefreshToken{},
		// L2/L4/L5/L7 auth tables. Previously absent from AutoMigrate, so the whole
		// feature set (MFA setup/challenge, PAT auth, SSO account linking, and the
		// full-fidelity auth audit trail) errored on non-existent tables.
		&domain.MFASecret{},
		&domain.MFABackupCode{},
		// OR26-03 — how long a privileged member of this tenant may defer
		// enrolment. One row per tenant; an absent row means the 7-day default.
		&domain.MFAPolicy{},
		&domain.PersonalAccessToken{},
		&domain.OAuthProvider{},
		&domain.AuthAuditLog{},
		// Password reset (migration 0042). Rows are written for every request,
		// including ones for addresses with no account — see the migration for why
		// that is what keeps the rate limiter from leaking account existence.
		&domain.PasswordResetToken{},
		&domain.Risk{},
		// Risk taxonomy (spec §3): the tenant's CONTROLLED category vocabulary and
		// the risk↔compliance-control references. tags need no table — they are a
		// free array on the risk, which is the whole point of the distinction.
		&domain.RiskCategory{},
		&domain.RiskControlMapping{},
		// Smart Risk Calculation (spec §8) — per-tenant configurable weights for the
		// eight-factor multifactor score. The smart-score columns themselves live on
		// domain.Risk above. Additive; the classic Score Engine is untouched.
		&domain.RiskScoringWeights{},
		&domain.Mitigation{},
		&domain.Asset{},
		&domain.AssetSnapshot{},
		// Typed attributes by asset category (Attack Surface §1). One row per
		// (tenant, category) holding the tenant-editable schema that the asset
		// form is generated from and every asset write is validated against.
		&domain.AssetTypeSchema{},
		// The tenant's vulnerability→risk rule (Attack Surface §4). One row per
		// tenant; disabled until they configure it, because automatic risk
		// creation writes to somebody's register.
		&domain.VulnRiskRule{},
		// Directed edges of the asset dependency graph ("cartographie des
		// dépendances"). Tenant-scoped; both endpoints reference assets.
		&domain.AssetDependency{},
		// Cross-framework control crosswalks: how much of a newly imported
		// framework a tenant's existing proof already answers (migration 0053).
		// Tenant-scoped undirected links between two compliance controls.
		&domain.ControlCrosswalk{},
		&domain.RiskHistory{},
		&domain.CustomField{},
		&domain.CustomFieldTemplate{},
		&domain.BulkOperation{},
		&domain.BulkOperationLog{},
		&domain.Team{},
		&domain.TeamMember{},
		// domain.Connector / MarketplaceApp / ConnectorUpdate / MarketplaceLog are intentionally
		// excluded: they carry only `json:` tags (no `gorm:` tags, no primary key), so AutoMigrate
		// fatally errors on them ("unsupported data type"). Marketplace is a pre-existing partial
		// module (see ROADMAP.md) — needs real GORM tagging before it can be added back here.
		&domain.AdminAuditEvent{},
		// M4 (second half) — monthly board-of-directors report (draft → approved),
		// with a per-tenant posture snapshot and an editable AI/template narrative.
		&domain.BoardReport{},
		&domain.ReportJob{},
		// The reporting engine (migration 0054): asynchronous generation, three
		// formats, versioned templates, an integrity hash and an editorial
		// lifecycle. ReportJob above is the earlier synchronous compliance-only
		// job, kept until its route is removed.
		&domain.Report{},
		&domain.ReportComment{},
		// RBAC + audit + multi-tenant tables. These back the Settings admin tabs
		// (Roles / Organizations / Audit log). RoleEnhanced maps onto the existing
		// "roles" table and only ADDS columns (tenant_id/level/is_predefined/...);
		// it never drops the legacy Role columns. Seeded by SeedRBAC() below.
		&domain.PermissionDB{},
		&domain.RoleEnhanced{},
		&domain.RolePermission{},
		&domain.Tenant{},
		&domain.UserTenant{},
		&domain.AuditLog{},
		// M5 (Incident Management) — the incident register + its timeline and
		// mitigation actions. Previously missing here, so every /incidents route
		// 500'd on a non-existent table; War Room stayed a fixture-only preview.
		&domain.Incident{},
		&domain.IncidentTimeline{},
		&domain.IncidentAction{},
		// Scanner engine — tenant-scoped scan configs, on-prem Agents, and scan
		// jobs. The pipeline never writes Assets/Risks itself: results land in a
		// Redis preview (48h TTL) and the user imports/ignores from there.
		&domain.ScanConfig{},
		&domain.ScannerAgent{},
		&domain.ScanJob{},
		// CTI / Intel Threat — vulnerabilities pulled from NVD + CISA KEV, enriched
		// with MITRE ATT&CK. Matched against asset CPEs to auto-create risks.
		&cti.CTIVulnerability{},
		// Vulnerability Management (Module 3) — the tenant-scoped vulnerability
		// register: findings normalised from Nessus/OpenVAS/Qualys/Defender/
		// Inspector/Azure Defender/CrowdStrike and risk-based prioritised.
		&domain.Vulnerability{},
		// Vulnerability integrations — per-source connector config (encrypted API
		// credentials, live-pull schedule, inbound webhook token, automation
		// toggles) + tenant ITSM/ticketing config for auto-ticketing.
		&domain.VulnIntegration{},
		&domain.VulnTicketingConfig{},
		// Notifications — the in-app centre + delivery preferences. Previously
		// missing from AutoMigrate, so every /notifications route errored on a
		// non-existent table (and the scan-completion in-app notification had
		// nowhere to land). Metadata is jsonb; it stays NULL unless a producer
		// sets it (a bare map[string]interface{} has no driver.Valuer).
		&domain.Notification{},
		&domain.NotificationPreference{},
		// The compliance register itself. Created by migration 0028 and, until
		// now, absent from AutoMigrate — so a column added to the model never
		// reached a database where migrations are blocked, and every write failed
		// with "column does not exist". Listed here so model and schema stay in
		// step, which is how every other module in this file works.
		&domain.ComplianceFramework{},
		&domain.ComplianceControl{},
		// Compliance audits ("Audits" — plan/execute/history) and remediation
		// plans ("Plans de remédiation" — close a gap, assign, track). Tenant-scoped.
		&domain.ComplianceAudit{},
		&domain.RemediationPlan{},
		// Evidence library (migration 0052): one proof artifact answering N
		// controls, with a collection date and an expiry. Replaces the read path
		// of control_evidences, which stays in place, backfilled from, for a
		// release.
		&domain.Evidence{},
		&domain.EvidenceControlLink{},
		// Security Automation / SOAR (spec §10 « Automatisation »): tenant-scoped
		// playbooks (trigger + conditions + action chain + SLA policy), their
		// execution audit trail, and the live SLA countdowns the monitor escalates.
		&domain.AutomationRule{},
		&domain.AutomationExecution{},
		&domain.SLATracker{},
		&domain.AutomationChannelConfig{},
		// Governance (spec §15 « Gouvernance »): the immutable audit trail
		// (append-only who/what/when/before→after), time-boxed delegations, and
		// the configurable Maker-Checker approval engine (workflows + requests).
		&domain.IncidentPostMortem{},
		&domain.AuditEvent{},
		&domain.AuditChainSeal{},
		&domain.AuditRetentionPolicy{},
		&domain.Delegation{},
		&domain.ApprovalWorkflow{},
		&domain.ApprovalRequest{},
		// Activation & onboarding (migration 0043). The server-side source of
		// truth for the newcomer journey: an append-only event log written by the
		// domain use cases, a per-user celebration ledger that makes the burst
		// idempotent, and the resumable signup wizard state the route guard reads.
		&domain.ActivationEvent{},
		&domain.ActivationCelebration{},
		&domain.OnboardingProgress{},
		// Open-core commercialisation: the per-tenant subscription + mirrored
		// invoices behind the plan gates, instance-level opt-in telemetry consent,
		// and the danger-zone organization-erasure record (30-day cancelable grace).
		&domain.Subscription{},
		&domain.Invoice{},
		&domain.TelemetryConfig{},
		&domain.OrgDeletionRequest{},
		// Realtime event hub (W0-07, migration 0059): the durable, per-tenant
		// ordered log behind GET /realtime/events. It is what lets a client that
		// reconnects replay what it missed instead of refetching everything, and
		// it is where the ordering guarantee lives (unique tenant_id+sequence).
		&domain.RealtimeEvent{},
		// Saved table views (#580): the named filter/sort/column combinations a
		// user keeps for a register and may share with their tenant. Purely
		// additive — a new table, no column dropped or renamed — which is why it
		// arrives through AutoMigrate alone and carries no .sql file (see
		// internal/migrations/migrator.go:64-66 on which layer owns the schema).
		&domain.SavedView{},
		// TPRM v1 (#670, ADR 0004 D3/D4): questionnaire templates, vendor
		// assessments with their snapshotted items, and the hash-only public link
		// tokens. Additive tables, so AutoMigrate owns them; migration 0062 adds
		// the one constraint AutoMigrate cannot express (one active token per
		// assessment, a partial unique index).
		&domain.VendorQuestionnaireTemplate{},
		&domain.VendorQuestionnaireQuestion{},
		&domain.VendorAssessment{},
		&domain.VendorAssessmentItem{},
		&domain.VendorAssessmentToken{},
	}
}
