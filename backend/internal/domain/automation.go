// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package domain

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Security Automation / SOAR (spec §10 « Automatisation »).
//
// An AutomationRule binds a TRIGGER (an event that happened in the platform)
// to an ordered chain of ACTIONS (scan, create risk, assign, ticket, notify,
// start an SLA countdown…). The AutomationEngine (an event-driven worker)
// evaluates a rule's conditions when its trigger fires and runs the chain,
// recording an AutomationExecution for audit. Actions that start an SLA create
// an SLATracker; the SLAMonitor (a background scheduler) escalates breached
// trackers and closes them when the underlying risk is resolved.
//
// Everything here is tenant-scoped — RULE #2 (filter by tenant_id on every
// query) is enforced in the repositories.
// ---------------------------------------------------------------------------

// AutomationTrigger is the kind of platform event that can start a workflow.
type AutomationTrigger string

const (
	// TriggerVulnerabilityDetected fires when a new vulnerability is ingested
	// (scanner webhook / live-pull / CTI). Payload carries CVE, severity, CVSS,
	// KEV, priority tier and the affected asset.
	TriggerVulnerabilityDetected AutomationTrigger = "vulnerability_detected"
	// TriggerRiskCreated fires when a risk is created (manual or automated).
	TriggerRiskCreated AutomationTrigger = "risk_created"
	// TriggerRiskScoreUpdated fires when the Score Engine recomputes a risk score.
	TriggerRiskScoreUpdated AutomationTrigger = "risk_score_updated"
	// TriggerIncidentCreated fires when an incident is opened.
	TriggerIncidentCreated AutomationTrigger = "incident_created"
	// TriggerManual is a rule only ever run on explicit user request (test/dry-run).
	TriggerManual AutomationTrigger = "manual"
)

// ParseAutomationTrigger validates a trigger string.
func ParseAutomationTrigger(s string) (AutomationTrigger, error) {
	switch AutomationTrigger(s) {
	case TriggerVulnerabilityDetected, TriggerRiskCreated, TriggerRiskScoreUpdated,
		TriggerIncidentCreated, TriggerManual:
		return AutomationTrigger(s), nil
	default:
		return "", NewValidationError("invalid automation trigger: " + s)
	}
}

// AutomationActionType is a single step in a rule's action chain.
type AutomationActionType string

const (
	ActionScanAsset    AutomationActionType = "scan_asset"    // targeted re-scan of the affected asset
	ActionCreateRisk   AutomationActionType = "create_risk"   // open a risk in the GRC register
	ActionAssignOwner  AutomationActionType = "assign_owner"  // assign the risk to owner/team
	ActionCreateTicket AutomationActionType = "create_ticket" // open a Jira/ServiceNow ticket
	ActionNotify       AutomationActionType = "notify"        // send alerts (in-app/email/Slack/Teams)
	ActionStartSLA     AutomationActionType = "start_sla"     // start an SLA countdown + escalation
	ActionResolveRisk  AutomationActionType = "resolve_risk"  // mark the risk resolved (auto-close)
	ActionCloseTicket  AutomationActionType = "close_ticket"  // close the linked ticket (auto-close)
)

// ParseAutomationActionType validates an action type string.
func ParseAutomationActionType(s string) (AutomationActionType, error) {
	switch AutomationActionType(s) {
	case ActionScanAsset, ActionCreateRisk, ActionAssignOwner, ActionCreateTicket,
		ActionNotify, ActionStartSLA, ActionResolveRisk, ActionCloseTicket:
		return AutomationActionType(s), nil
	default:
		return "", NewValidationError("invalid automation action: " + s)
	}
}

// AutomationConditions are the guard clauses a triggering event must satisfy
// before the action chain runs. A zero value matches everything.
type AutomationConditions struct {
	// MinSeverity gates on the severity of the triggering object
	// (low<medium<high<critical). Empty = no gate.
	MinSeverity string `json:"min_severity,omitempty"`
	// MinCVSS gates on the CVSS base score (0–10). 0 = no gate.
	MinCVSS float64 `json:"min_cvss,omitempty"`
	// KEVOnly requires the vulnerability to be CISA Known-Exploited.
	KEVOnly bool `json:"kev_only,omitempty"`
	// MinPriorityTier gates on the vuln priority tier (P1 strongest … P4). Empty = no gate.
	MinPriorityTier string `json:"min_priority_tier,omitempty"`
	// AssetTags requires the affected asset to carry at least one of these tags.
	AssetTags []string `json:"asset_tags,omitempty"`
}

// Value/Scan let GORM persist conditions as a jsonb column.
func (c AutomationConditions) Value() (driver.Value, error) { return json.Marshal(c) }

func (c *AutomationConditions) Scan(value interface{}) error {
	if value == nil {
		*c = AutomationConditions{}
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		if s, ok := value.(string); ok {
			b = []byte(s)
		} else {
			return fmt.Errorf("automation conditions: unsupported scan type %T", value)
		}
	}
	if len(b) == 0 {
		*c = AutomationConditions{}
		return nil
	}
	return json.Unmarshal(b, c)
}

// AutomationAction is one step of the chain. Only the fields relevant to Type
// are read; the rest stay zero.
type AutomationAction struct {
	Type AutomationActionType `json:"type"`

	// notify — one or more of: in_app, email, slack, teams.
	Channels []string `json:"channels,omitempty"`
	// notify recipient hint / assign_owner target: a role (admin, manager) or a
	// user email/id. Empty on notify falls back to the risk owner + admins.
	Target string `json:"target,omitempty"`
	// notify custom message (templated with {{title}}, {{severity}}, {{cve}}…).
	Message string `json:"message,omitempty"`

	// create_ticket provider override (jira|servicenow). Empty uses the tenant default.
	TicketProvider string `json:"ticket_provider,omitempty"`
}

// AutomationActionList is an ordered list of actions stored as jsonb.
type AutomationActionList []AutomationAction

func (l AutomationActionList) Value() (driver.Value, error) { return json.Marshal(l) }

func (l *AutomationActionList) Scan(value interface{}) error {
	if value == nil {
		*l = AutomationActionList{}
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		if s, ok := value.(string); ok {
			b = []byte(s)
		} else {
			return fmt.Errorf("automation actions: unsupported scan type %T", value)
		}
	}
	if len(b) == 0 {
		*l = AutomationActionList{}
		return nil
	}
	return json.Unmarshal(b, l)
}

// AutomationSLAConfig defines the resolution deadline (by severity) and the
// escalation policy a start_sla action applies.
type AutomationSLAConfig struct {
	// Resolution budget in minutes per severity. 0 = fall back to the next
	// higher-severity value, else no SLA for that severity.
	CriticalMinutes int `json:"critical_minutes,omitempty"`
	HighMinutes     int `json:"high_minutes,omitempty"`
	MediumMinutes   int `json:"medium_minutes,omitempty"`
	LowMinutes      int `json:"low_minutes,omitempty"`

	// EscalateAfterMinutes is how long AFTER the deadline breach to escalate.
	// 0 escalates immediately on breach.
	EscalateAfterMinutes int `json:"escalate_after_minutes,omitempty"`
	// EscalateToRole is the role notified on escalation (admin|manager). Empty = admin.
	EscalateToRole string `json:"escalate_to_role,omitempty"`
	// EscalateChannels are the channels used for the escalation alert.
	EscalateChannels []string `json:"escalate_channels,omitempty"`
}

// MinutesFor returns the resolution budget for a severity, falling back to the
// next higher tier when a tier is left at 0. Returns 0 when no budget applies.
func (c AutomationSLAConfig) MinutesFor(severity string) int {
	switch strings.ToLower(severity) {
	case "critical":
		return c.CriticalMinutes
	case "high":
		if c.HighMinutes > 0 {
			return c.HighMinutes
		}
		return c.CriticalMinutes
	case "medium":
		if c.MediumMinutes > 0 {
			return c.MediumMinutes
		}
		if c.HighMinutes > 0 {
			return c.HighMinutes
		}
		return c.CriticalMinutes
	case "low":
		if c.LowMinutes > 0 {
			return c.LowMinutes
		}
		if c.MediumMinutes > 0 {
			return c.MediumMinutes
		}
		if c.HighMinutes > 0 {
			return c.HighMinutes
		}
		return c.CriticalMinutes
	default:
		return c.MediumMinutes
	}
}

func (c AutomationSLAConfig) Value() (driver.Value, error) { return json.Marshal(c) }

func (c *AutomationSLAConfig) Scan(value interface{}) error {
	if value == nil {
		*c = AutomationSLAConfig{}
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		if s, ok := value.(string); ok {
			b = []byte(s)
		} else {
			return fmt.Errorf("automation sla config: unsupported scan type %T", value)
		}
	}
	if len(b) == 0 {
		*c = AutomationSLAConfig{}
		return nil
	}
	return json.Unmarshal(b, c)
}

// AutomationRule is a tenant-scoped SOAR playbook: one trigger, a set of
// conditions, an ordered action chain, and an optional SLA policy.
type AutomationRule struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`

	Name        string `gorm:"size:160;not null" json:"name"`
	Description string `gorm:"type:text" json:"description"`
	Enabled     bool   `gorm:"default:true;index" json:"enabled"`

	Trigger    AutomationTrigger    `gorm:"type:varchar(40);not null;index" json:"trigger"`
	Conditions AutomationConditions `gorm:"type:jsonb" json:"conditions"`
	Actions    AutomationActionList `gorm:"type:jsonb" json:"actions"`
	SLA        AutomationSLAConfig  `gorm:"type:jsonb" json:"sla"`

	// Priority orders rules that share a trigger (lower runs first).
	Priority int `gorm:"default:100" json:"priority"`

	// Bookkeeping.
	LastTriggeredAt *time.Time `json:"last_triggered_at,omitempty"`
	TriggerCount    int        `gorm:"default:0" json:"trigger_count"`

	CreatedBy uuid.UUID      `gorm:"type:uuid" json:"created_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName pins the table name.
func (AutomationRule) TableName() string { return "automation_rules" }

// Validate checks a rule is well-formed before persistence.
func (r *AutomationRule) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return NewValidationError("automation rule name is required")
	}
	if _, err := ParseAutomationTrigger(string(r.Trigger)); err != nil {
		return err
	}
	if len(r.Actions) == 0 {
		return NewValidationError("automation rule needs at least one action")
	}
	hasStartSLA := false
	for _, a := range r.Actions {
		if _, err := ParseAutomationActionType(string(a.Type)); err != nil {
			return err
		}
		if a.Type == ActionStartSLA {
			hasStartSLA = true
		}
	}
	if hasStartSLA && r.SLA.MinutesFor("critical") == 0 && r.SLA.MinutesFor("high") == 0 &&
		r.SLA.MinutesFor("medium") == 0 && r.SLA.MinutesFor("low") == 0 {
		return NewValidationError("start_sla action requires at least one SLA budget")
	}
	return nil
}

// AutomationExecutionStatus is the outcome of running a rule once.
type AutomationExecutionStatus string

const (
	ExecutionPending AutomationExecutionStatus = "pending"
	ExecutionRunning AutomationExecutionStatus = "running"
	ExecutionSuccess AutomationExecutionStatus = "success"
	ExecutionPartial AutomationExecutionStatus = "partial" // some steps failed
	ExecutionFailed  AutomationExecutionStatus = "failed"
	ExecutionSkipped AutomationExecutionStatus = "skipped" // conditions not met
)

// ExecutionStep records the outcome of a single action within an execution.
type ExecutionStep struct {
	Action string    `json:"action"`
	Status string    `json:"status"` // success|failed|skipped
	Detail string    `json:"detail"`
	At     time.Time `json:"at"`
}

// ExecutionStepList is stored as jsonb.
type ExecutionStepList []ExecutionStep

func (l ExecutionStepList) Value() (driver.Value, error) { return json.Marshal(l) }

func (l *ExecutionStepList) Scan(value interface{}) error {
	if value == nil {
		*l = ExecutionStepList{}
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		if s, ok := value.(string); ok {
			b = []byte(s)
		} else {
			return fmt.Errorf("execution steps: unsupported scan type %T", value)
		}
	}
	if len(b) == 0 {
		*l = ExecutionStepList{}
		return nil
	}
	return json.Unmarshal(b, l)
}

// AutomationExecution is the audit record of one rule firing.
type AutomationExecution struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`

	RuleID   uuid.UUID         `gorm:"type:uuid;index" json:"rule_id"`
	RuleName string            `gorm:"size:160" json:"rule_name"`
	Trigger  AutomationTrigger `gorm:"type:varchar(40);index" json:"trigger"`

	// TriggerRef is a stable reference to what fired the rule (e.g. "cve:CVE-2021-44228",
	// "risk:<uuid>", "vuln:<uuid>"). Subject is a human summary.
	TriggerRef string `gorm:"size:128;index" json:"trigger_ref"`
	Subject    string `gorm:"size:255" json:"subject"`
	Severity   string `gorm:"size:16" json:"severity"`

	Status AutomationExecutionStatus `gorm:"type:varchar(16);index" json:"status"`
	Steps  ExecutionStepList         `gorm:"type:jsonb" json:"steps"`
	Error  string                    `gorm:"type:text" json:"error,omitempty"`

	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// TableName pins the table name.
func (AutomationExecution) TableName() string { return "automation_executions" }

// SLAStatus is the lifecycle of an SLA countdown.
type SLAStatus string

const (
	SLAOpen      SLAStatus = "open"      // ticking, within budget
	SLABreached  SLAStatus = "breached"  // past due, awaiting escalation window
	SLAEscalated SLAStatus = "escalated" // escalation raised
	SLAMet       SLAStatus = "met"       // resolved before the deadline
	SLAClosed    SLAStatus = "closed"    // resolved (possibly after breach)
)

// SLATracker is a live resolution countdown for one remediation item, created by
// a start_sla action and advanced by the SLAMonitor.
type SLATracker struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`

	RuleID      uuid.UUID  `gorm:"type:uuid;index" json:"rule_id"`
	ExecutionID *uuid.UUID `gorm:"type:uuid;index" json:"execution_id,omitempty"`

	// What the SLA is tracking. RiskID (when set) lets the monitor auto-close the
	// tracker once the risk is resolved.
	SubjectType string     `gorm:"size:16" json:"subject_type"` // risk|ticket|vulnerability
	SubjectID   string     `gorm:"size:128;index" json:"subject_id"`
	RiskID      *uuid.UUID `gorm:"type:uuid;index" json:"risk_id,omitempty"`
	Title       string     `gorm:"size:255" json:"title"`
	Severity    string     `gorm:"size:16;index" json:"severity"`
	TicketRef   string     `gorm:"size:128" json:"ticket_ref,omitempty"`

	Status SLAStatus `gorm:"type:varchar(16);index" json:"status"`
	DueAt  time.Time `gorm:"index" json:"due_at"`

	// Escalation policy snapshot (frozen from the rule at creation time).
	EscalateAt       *time.Time `gorm:"index" json:"escalate_at,omitempty"`
	EscalateToRole   string     `gorm:"size:24" json:"escalate_to_role,omitempty"`
	EscalateChannels StringList `gorm:"type:jsonb" json:"escalate_channels,omitempty"`
	EscalationLevel  int        `gorm:"default:0" json:"escalation_level"`
	EscalatedAt      *time.Time `json:"escalated_at,omitempty"`

	OwnerID  *uuid.UUID `gorm:"type:uuid" json:"owner_id,omitempty"`
	ClosedAt *time.Time `json:"closed_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName pins the table name.
func (SLATracker) TableName() string { return "sla_trackers" }

// RemainingMinutes is minutes until (positive) or past (negative) the deadline.
func (t *SLATracker) RemainingMinutes(now time.Time) int {
	return int(t.DueAt.Sub(now).Minutes())
}

// IsBreached reports whether the deadline has passed for a still-open tracker.
func (t *SLATracker) IsBreached(now time.Time) bool {
	return (t.Status == SLAOpen || t.Status == SLABreached) && now.After(t.DueAt)
}

// StringList is a []string stored as jsonb (used for escalation channels).
type StringList []string

func (l StringList) Value() (driver.Value, error) { return json.Marshal(l) }

func (l *StringList) Scan(value interface{}) error {
	if value == nil {
		*l = StringList{}
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		if s, ok := value.(string); ok {
			b = []byte(s)
		} else {
			return fmt.Errorf("string list: unsupported scan type %T", value)
		}
	}
	if len(b) == 0 {
		*l = StringList{}
		return nil
	}
	return json.Unmarshal(b, l)
}

// ---------------------------------------------------------------------------
// Repository ports — ABSOLUTE RULE #2: every method filters by tenant_id
// (except SLA escalation sweeps, which run cross-tenant on the scheduler and
// carry the tenant on each row).
// ---------------------------------------------------------------------------

// AutomationRuleRepository persists SOAR rules.
type AutomationRuleRepository interface {
	Create(ctx context.Context, r *AutomationRule) error
	Update(ctx context.Context, r *AutomationRule) error
	GetByID(ctx context.Context, id, tenantID uuid.UUID) (*AutomationRule, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]AutomationRule, error)
	// ListEnabledByTrigger returns enabled rules for a tenant + trigger, ordered
	// by Priority ascending. Used by the engine on each event.
	ListEnabledByTrigger(ctx context.Context, tenantID uuid.UUID, trigger AutomationTrigger) ([]AutomationRule, error)
	Delete(ctx context.Context, id, tenantID uuid.UUID) error
	// RecordTriggered bumps TriggerCount + LastTriggeredAt (best-effort, no history spam).
	RecordTriggered(ctx context.Context, id, tenantID uuid.UUID, at time.Time) error
}

// AutomationExecutionRepository persists execution audit records.
type AutomationExecutionRepository interface {
	Create(ctx context.Context, e *AutomationExecution) error
	Update(ctx context.Context, e *AutomationExecution) error
	GetByID(ctx context.Context, id, tenantID uuid.UUID) (*AutomationExecution, error)
	List(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]AutomationExecution, error)
	ListByRule(ctx context.Context, ruleID, tenantID uuid.UUID, limit int) ([]AutomationExecution, error)
}

// SLATrackerRepository persists SLA countdowns.
type SLATrackerRepository interface {
	Create(ctx context.Context, t *SLATracker) error
	Update(ctx context.Context, t *SLATracker) error
	GetByID(ctx context.Context, id, tenantID uuid.UUID) (*SLATracker, error)
	ListOpen(ctx context.Context, tenantID uuid.UUID) ([]SLATracker, error)
	// ListBreaching returns still-open trackers whose escalate_at has elapsed,
	// across ALL tenants (each row carries its tenant). Drives the SLAMonitor.
	ListBreaching(ctx context.Context, now time.Time) ([]SLATracker, error)
	// ListOpenByRisk finds open trackers for a resolved risk (for auto-close).
	ListOpenByRisk(ctx context.Context, tenantID, riskID uuid.UUID) ([]SLATracker, error)
	// ListOpenLinkedToRisk returns every still-open tracker that is bound to a
	// risk, across ALL tenants. Drives the SLAMonitor's auto-close sweep.
	ListOpenLinkedToRisk(ctx context.Context) ([]SLATracker, error)
	// Stats returns counts by status for the tenant's SLA dashboard.
	Stats(ctx context.Context, tenantID uuid.UUID) (SLAStats, error)
}

// AutomationChannelConfig is the tenant-level configuration of the outbound
// alert channels the automation engine can use ("configurer un nouveau canal
// d'alerte"). One row per tenant. Webhook URLs are write-only: they are never
// returned to the API (only HasSlack/HasTeams booleans are).
type AutomationChannelConfig struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"tenant_id"`

	SlackEnabled    bool   `gorm:"default:false" json:"slack_enabled"`
	SlackWebhookURL string `gorm:"size:512" json:"-"`

	TeamsEnabled    bool   `gorm:"default:false" json:"teams_enabled"`
	TeamsWebhookURL string `gorm:"size:512" json:"-"`

	// EmailEnabled turns on e-mail delivery for automation alerts. DefaultEmail is
	// a fallback recipient used when a role-based alert resolves to nobody.
	EmailEnabled bool   `gorm:"default:true" json:"email_enabled"`
	DefaultEmail string `gorm:"size:255" json:"default_email"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Computed, NOT persisted — surfaced instead of the raw webhook URLs.
	HasSlack bool `gorm:"-" json:"has_slack"`
	HasTeams bool `gorm:"-" json:"has_teams"`
}

// TableName pins the table name.
func (AutomationChannelConfig) TableName() string { return "automation_channels" }

// AutomationChannelRepository persists the tenant channel configuration.
type AutomationChannelRepository interface {
	Upsert(ctx context.Context, c *AutomationChannelConfig) error
	Get(ctx context.Context, tenantID uuid.UUID) (*AutomationChannelConfig, error)
}

// SLAStats is the tenant SLA dashboard summary.
type SLAStats struct {
	Open      int64 `json:"open"`
	Breached  int64 `json:"breached"`
	Escalated int64 `json:"escalated"`
	Met       int64 `json:"met"`
	Closed    int64 `json:"closed"`
	// AtRisk is open trackers within 25% of their remaining budget (computed by
	// the use case, not the repo).
	AtRisk int64 `json:"at_risk"`
}
