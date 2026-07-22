// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

// Package automation is the Security Automation / SOAR engine (spec §10
// « Automatisation »). It binds platform events (triggers) to ordered action
// chains, records executions for audit, and drives SLA countdowns + escalation.
package automation

import (
	"context"

	"github.com/google/uuid"
)

// TriggerContext is the normalised description of what fired a rule. The engine
// builds it from a decoded Redis event; action ports read from it and enrich it
// as the chain runs (e.g. create_risk sets RiskID so later actions can use it).
type TriggerContext struct {
	TenantID     uuid.UUID
	Ref          string // stable reference, e.g. "cve:CVE-2021-44228"
	Subject      string // human summary
	Title        string
	Severity     string // low|medium|high|critical
	CVSS         float64
	KEV          bool
	PriorityTier string // P1..P4
	CVEID        string
	AssetID      *uuid.UUID
	AssetName    string
	AssetTags    []string
	RiskID       *uuid.UUID
	TicketRef    string
	OwnerID      *uuid.UUID
	TriggeredBy  uuid.UUID
}

// Fact is a labelled value shown in an alert.
type Fact struct {
	Label string
	Value string
}

// NotifyRequest is a cross-channel alert. The Notifier resolves recipients for
// the user-addressed channels (in_app/email) from OwnerID + TargetRole, and
// posts to the tenant Slack/Teams webhooks for the chat channels.
type NotifyRequest struct {
	TenantID     uuid.UUID
	Channels     []string // in_app|email|slack|teams (empty = all configured)
	Severity     string
	Subject      string
	Message      string
	TargetRole   string     // admin|manager — resolves extra recipients
	OwnerID      *uuid.UUID // explicit recipient (e.g. risk owner)
	Facts        []Fact
	LinkURL      string
	ResourceID   *uuid.UUID
	ResourceType string
}

// TicketRequest opens an ITSM ticket for the triggering item.
type TicketRequest struct {
	TenantID    uuid.UUID
	Provider    string // jira|servicenow (empty = tenant default)
	Summary     string
	Description string
	Severity    string
	Labels      []string
}

// TicketResult is a successfully opened ticket.
type TicketResult struct {
	Provider string
	Key      string
	URL      string
}

// RiskRequest opens (or reuses) a risk for a triggering vulnerability.
type RiskRequest struct {
	TenantID  uuid.UUID
	Title     string
	CVEID     string
	Severity  string
	AssetID   *uuid.UUID
	CreatedBy uuid.UUID
}

// RiskResult identifies the ensured risk.
type RiskResult struct {
	RiskID  uuid.UUID
	Created bool
}

// ---------------------------------------------------------------------------
// Action ports — every one is OPTIONAL. A nil port makes its action degrade to
// a recorded "skipped (not configured)" step; the chain never hard-fails on a
// missing capability. Wired in the composition root (main.go).
// ---------------------------------------------------------------------------

// Notifier sends alerts across channels; returns the channels that delivered.
type Notifier interface {
	Notify(ctx context.Context, req NotifyRequest) (delivered []string, err error)
}

// Ticketer opens an ITSM ticket.
type Ticketer interface {
	OpenTicket(ctx context.Context, req TicketRequest) (TicketResult, error)
}

// RiskCreator opens (or finds) a risk for a triggering vulnerability.
type RiskCreator interface {
	EnsureRisk(ctx context.Context, req RiskRequest) (RiskResult, error)
}

// RiskAssigner assigns a risk to an owner resolved from a role or user hint.
type RiskAssigner interface {
	Assign(ctx context.Context, tenantID, riskID uuid.UUID, target string) (assignedTo uuid.UUID, err error)
}

// AssetScanner triggers a targeted re-scan of an asset. Returns a job reference.
type AssetScanner interface {
	ScanAsset(ctx context.Context, tenantID, assetID uuid.UUID) (jobRef string, err error)
}

// RiskResolver marks a risk resolved (auto-close on confirmed remediation).
type RiskResolver interface {
	Resolve(ctx context.Context, tenantID, riskID uuid.UUID) error
}
