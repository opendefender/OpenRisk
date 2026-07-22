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
// Governance (spec §15 « Gouvernance »).
//
// Three tenant-scoped building blocks, all in this file so the model reads as one
// story — enforce RULE #2 (filter by tenant_id on every query) in the repos:
//
//  1. AuditEvent         — the immutable, append-only audit trail (Who / What /
//                          When / IP / Before → After). Written automatically by
//                          the pkg/audittrail GORM plugin on every mutation of an
//                          Auditable model, and explicitly by the Recorder for
//                          high-value governance actions (approvals, delegations).
//  2. Delegation         — a time-boxed grant of one user's rights to another
//                          (vacation / absence cover), with start & end dates.
//  3. ApprovalWorkflow / — a configurable Maker-Checker engine: a workflow binds an
//     ApprovalRequest      (entity_type, action) to an ordered chain of approval
//                          steps; a request walks that chain as a state machine.
// ---------------------------------------------------------------------------

// =============================================================================
// JSONMap — a jsonb column holding an arbitrary object (before/after snapshots,
// approval payloads). Implements driver.Valuer / sql.Scanner like the automation
// jsonb types so GORM persists it as jsonb.
// =============================================================================

type JSONMap map[string]interface{}

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

func (m *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*m = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		if len(v) == 0 {
			*m = nil
			return nil
		}
		return json.Unmarshal(v, m)
	case string:
		if v == "" {
			*m = nil
			return nil
		}
		return json.Unmarshal([]byte(v), m)
	default:
		return fmt.Errorf("unsupported type for JSONMap: %T", value)
	}
}

// =============================================================================
// 1. Immutable audit trail
// =============================================================================

// AuditAction is the verb of an audited event.
type AuditAction string

const (
	AuditActionCreate   AuditAction = "create"
	AuditActionUpdate   AuditAction = "update"
	AuditActionDelete   AuditAction = "delete"
	AuditActionSubmit   AuditAction = "submit"   // approval request submitted
	AuditActionApprove  AuditAction = "approve"  // approval step approved
	AuditActionReject   AuditAction = "reject"   // approval request rejected
	AuditActionDelegate AuditAction = "delegate" // rights delegated
	AuditActionRevoke   AuditAction = "revoke"   // delegation revoked / access revoked
	AuditActionLogin    AuditAction = "login"
	AuditActionExport   AuditAction = "export"
)

// AuditEvent is one immutable row in the audit trail. There is intentionally no
// UpdatedAt and no soft-delete: the repository only ever appends. ActorID is nil
// for automatic/system events (e.g. a background worker mutation).
type AuditEvent struct {
	ID       uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID  `gorm:"type:uuid;index;not null" json:"tenant_id"`
	ActorID  *uuid.UUID `gorm:"type:uuid;index" json:"actor_id,omitempty"`
	// ActorEmail is resolved on read (never persisted) so the journal shows a
	// human name instead of a bare UUID.
	ActorEmail    string      `gorm:"-" json:"actor_email,omitempty"`
	Action        AuditAction `gorm:"type:varchar(24);index" json:"action"`
	EntityType    string      `gorm:"type:varchar(64);index" json:"entity_type"`
	EntityID      string      `gorm:"type:varchar(128);index" json:"entity_id"`
	Summary       string      `gorm:"type:text" json:"summary"`
	Before        JSONMap     `gorm:"type:jsonb" json:"before,omitempty"`
	After         JSONMap     `gorm:"type:jsonb" json:"after,omitempty"`
	ChangedFields StringList  `gorm:"type:jsonb" json:"changed_fields,omitempty"`
	IPAddress     string      `gorm:"type:varchar(64)" json:"ip_address,omitempty"`
	UserAgent     string      `gorm:"type:text" json:"user_agent,omitempty"`
	RequestID     string      `gorm:"type:varchar(64)" json:"request_id,omitempty"`
	CreatedAt     time.Time   `gorm:"index;autoCreateTime" json:"created_at"`
}

func (AuditEvent) TableName() string { return "audit_events" }

// AuditEventFilter narrows an audit-trail query. Zero-value fields are ignored.
type AuditEventFilter struct {
	EntityType string
	EntityID   string
	Action     string
	ActorID    *uuid.UUID
	From       *time.Time
	To         *time.Time
	Search     string // free-text over summary / entity_type / entity_id
	Limit      int
	Offset     int
}

// AuditEventRepository is the append-only store for the audit trail.
type AuditEventRepository interface {
	Append(ctx context.Context, e *AuditEvent) error
	List(ctx context.Context, tenantID uuid.UUID, f AuditEventFilter) ([]AuditEvent, int64, error)
}

// Auditable marks a GORM model that the audittrail plugin should capture. The
// method returns a stable, human-readable entity type name (e.g. "risk"). The
// plugin reflects the primary key and tenant_id from the record itself.
type Auditable interface {
	AuditEntityType() string
}

// =============================================================================
// 2. Delegations
// =============================================================================

// DelegationStatus is the lifecycle of a delegation.
type DelegationStatus string

const (
	DelegationActive  DelegationStatus = "active"
	DelegationRevoked DelegationStatus = "revoked"
)

// Delegation is a time-boxed grant of the delegator's rights (a permission
// subset, or "*") to another user. Effective only while active and within
// [StartsAt, EndsAt]. Resolved live by the ResolveEffectivePermissions use case.
type Delegation struct {
	ID             uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID       uuid.UUID        `gorm:"type:uuid;index;not null" json:"tenant_id"`
	DelegatorID    uuid.UUID        `gorm:"type:uuid;index;not null" json:"delegator_id"`
	DelegatorEmail string           `gorm:"-" json:"delegator_email,omitempty"`
	DelegateID     uuid.UUID        `gorm:"type:uuid;index;not null" json:"delegate_id"`
	DelegateEmail  string           `gorm:"-" json:"delegate_email,omitempty"`
	Reason         string           `gorm:"type:text" json:"reason,omitempty"`
	Permissions    StringList       `gorm:"type:jsonb" json:"permissions"`
	Status         DelegationStatus `gorm:"type:varchar(16);index;default:'active'" json:"status"`
	StartsAt       time.Time        `gorm:"index" json:"starts_at"`
	EndsAt         time.Time        `gorm:"index" json:"ends_at"`
	RevokedAt      *time.Time       `json:"revoked_at,omitempty"`
	CreatedBy      uuid.UUID        `gorm:"type:uuid" json:"created_by"`
	CreatedAt      time.Time        `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time        `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Delegation) TableName() string { return "delegations" }

// IsActiveAt reports whether the delegation confers rights at instant t.
func (d *Delegation) IsActiveAt(t time.Time) bool {
	return d.Status == DelegationActive && !t.Before(d.StartsAt) && !t.After(d.EndsAt)
}

// DelegationFilter narrows a delegation query. Zero-value fields are ignored.
type DelegationFilter struct {
	DelegatorID *uuid.UUID
	DelegateID  *uuid.UUID
	ActiveOnly  bool
}

// DelegationRepository is the tenant-scoped store for delegations.
type DelegationRepository interface {
	Create(ctx context.Context, d *Delegation) error
	GetByID(ctx context.Context, id, tenantID uuid.UUID) (*Delegation, error)
	List(ctx context.Context, tenantID uuid.UUID, f DelegationFilter) ([]Delegation, error)
	Update(ctx context.Context, d *Delegation) error
}

// =============================================================================
// 3. Approval workflows (Maker-Checker)
// =============================================================================

// ApprovalStatus is the state of an approval request.
type ApprovalStatus string

const (
	ApprovalPending   ApprovalStatus = "pending"
	ApprovalApproved  ApprovalStatus = "approved"
	ApprovalRejected  ApprovalStatus = "rejected"
	ApprovalCancelled ApprovalStatus = "cancelled"
)

// WorkflowStep is one gate in an approval chain. ApproverRole names the role
// that may sign this step ("" or "any" = any tenant member). MinApprovals is how
// many distinct approvers of that role must sign before the chain advances.
type WorkflowStep struct {
	Order        int    `json:"order"`
	Name         string `json:"name"`
	ApproverRole string `json:"approver_role"`
	MinApprovals int    `json:"min_approvals"`
}

// WorkflowStepList is a jsonb array of steps.
type WorkflowStepList []WorkflowStep

func (l WorkflowStepList) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	return json.Marshal(l)
}

func (l *WorkflowStepList) Scan(value interface{}) error {
	if value == nil {
		*l = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		if len(v) == 0 {
			*l = nil
			return nil
		}
		return json.Unmarshal(v, l)
	case string:
		if v == "" {
			*l = nil
			return nil
		}
		return json.Unmarshal([]byte(v), l)
	default:
		return fmt.Errorf("unsupported type for WorkflowStepList: %T", value)
	}
}

// ApprovalWorkflow binds an (EntityType, Action) pair to an ordered approval
// chain. One workflow per (tenant, entity_type, action) is enforced by the
// application layer. Soft-deletable so history keeps referencing it.
type ApprovalWorkflow struct {
	ID          uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID    uuid.UUID        `gorm:"type:uuid;index;not null" json:"tenant_id"`
	Name        string           `gorm:"type:varchar(160);not null" json:"name"`
	Description string           `gorm:"type:text" json:"description,omitempty"`
	EntityType  string           `gorm:"type:varchar(64);index" json:"entity_type"`
	Action      string           `gorm:"type:varchar(64)" json:"action"`
	Enabled     bool             `gorm:"index;default:true" json:"enabled"`
	Steps       WorkflowStepList `gorm:"type:jsonb" json:"steps"`
	CreatedBy   uuid.UUID        `gorm:"type:uuid" json:"created_by"`
	CreatedAt   time.Time        `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time        `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt   `gorm:"index" json:"-"`
}

func (ApprovalWorkflow) TableName() string { return "approval_workflows" }

// Validate checks that a workflow has at least one well-formed step.
func (w *ApprovalWorkflow) Validate() error {
	if strings.TrimSpace(w.Name) == "" {
		return NewValidationError("workflow name is required")
	}
	if strings.TrimSpace(w.EntityType) == "" {
		return NewValidationError("entity_type is required")
	}
	if len(w.Steps) == 0 {
		return NewValidationError("workflow needs at least one approval step")
	}
	return nil
}

// ApprovalDecision is one approver's sign-off on a step, embedded in the request.
type ApprovalDecision struct {
	StepOrder     int       `json:"step_order"`
	ApproverID    string    `json:"approver_id"`
	ApproverEmail string    `json:"approver_email,omitempty"`
	Decision      string    `json:"decision"` // "approve" | "reject"
	Comment       string    `json:"comment,omitempty"`
	DecidedAt     time.Time `json:"decided_at"`
}

// ApprovalDecisionList is a jsonb array of decisions.
type ApprovalDecisionList []ApprovalDecision

func (l ApprovalDecisionList) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	return json.Marshal(l)
}

func (l *ApprovalDecisionList) Scan(value interface{}) error {
	if value == nil {
		*l = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		if len(v) == 0 {
			*l = nil
			return nil
		}
		return json.Unmarshal(v, l)
	case string:
		if v == "" {
			*l = nil
			return nil
		}
		return json.Unmarshal([]byte(v), l)
	default:
		return fmt.Errorf("unsupported type for ApprovalDecisionList: %T", value)
	}
}

// ApprovalRequest is a live run of an approval workflow — the state machine. It
// snapshots the workflow's steps at submit time so later edits to the workflow
// never rewrite in-flight requests.
type ApprovalRequest struct {
	ID               uuid.UUID            `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID         uuid.UUID            `gorm:"type:uuid;index;not null" json:"tenant_id"`
	WorkflowID       *uuid.UUID           `gorm:"type:uuid;index" json:"workflow_id,omitempty"`
	WorkflowName     string               `gorm:"type:varchar(160)" json:"workflow_name,omitempty"`
	EntityType       string               `gorm:"type:varchar(64);index" json:"entity_type"`
	EntityID         string               `gorm:"type:varchar(128);index" json:"entity_id,omitempty"`
	Action           string               `gorm:"type:varchar(64)" json:"action,omitempty"`
	Title            string               `gorm:"type:varchar(255)" json:"title"`
	Description      string               `gorm:"type:text" json:"description,omitempty"`
	Payload          JSONMap              `gorm:"type:jsonb" json:"payload,omitempty"`
	Status           ApprovalStatus       `gorm:"type:varchar(16);index;default:'pending'" json:"status"`
	CurrentStep      int                  `gorm:"default:0" json:"current_step"`
	Steps            WorkflowStepList     `gorm:"type:jsonb" json:"steps"`
	Decisions        ApprovalDecisionList `gorm:"type:jsonb" json:"decisions"`
	RequestedBy      uuid.UUID            `gorm:"type:uuid;index" json:"requested_by"`
	RequestedByEmail string               `gorm:"-" json:"requested_by_email,omitempty"`
	ResolvedAt       *time.Time           `json:"resolved_at,omitempty"`
	CreatedAt        time.Time            `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time            `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ApprovalRequest) TableName() string { return "approval_requests" }

// CurrentStepDef returns the step the request is waiting on, or nil if resolved
// or out of range.
func (r *ApprovalRequest) CurrentStepDef() *WorkflowStep {
	if r.CurrentStep < 0 || r.CurrentStep >= len(r.Steps) {
		return nil
	}
	return &r.Steps[r.CurrentStep]
}

// ApprovalsAtStep counts distinct approvers who approved a given step.
func (r *ApprovalRequest) ApprovalsAtStep(step int) int {
	seen := map[string]bool{}
	for _, d := range r.Decisions {
		if d.StepOrder == step && d.Decision == "approve" {
			seen[d.ApproverID] = true
		}
	}
	return len(seen)
}

// ApprovalRequestFilter narrows a request query. Zero-value fields are ignored.
type ApprovalRequestFilter struct {
	Status      string
	EntityType  string
	RequestedBy *uuid.UUID
}

// ApprovalWorkflowRepository is the tenant-scoped store for workflow configs.
type ApprovalWorkflowRepository interface {
	CreateWorkflow(ctx context.Context, w *ApprovalWorkflow) error
	GetWorkflowByID(ctx context.Context, id, tenantID uuid.UUID) (*ApprovalWorkflow, error)
	ListWorkflows(ctx context.Context, tenantID uuid.UUID) ([]ApprovalWorkflow, error)
	FindWorkflow(ctx context.Context, tenantID uuid.UUID, entityType, action string) (*ApprovalWorkflow, error)
	UpdateWorkflow(ctx context.Context, w *ApprovalWorkflow) error
	DeleteWorkflow(ctx context.Context, id, tenantID uuid.UUID) error
}

// ApprovalRequestRepository is the tenant-scoped store for live requests.
type ApprovalRequestRepository interface {
	CreateRequest(ctx context.Context, r *ApprovalRequest) error
	GetRequestByID(ctx context.Context, id, tenantID uuid.UUID) (*ApprovalRequest, error)
	ListRequests(ctx context.Context, tenantID uuid.UUID, f ApprovalRequestFilter) ([]ApprovalRequest, error)
	UpdateRequest(ctx context.Context, r *ApprovalRequest) error
}
