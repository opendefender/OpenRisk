// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package scanner

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// SSEChannel returns the Redis pub/sub channel a tenant's clients subscribe to
// for live scan events (preview ready, scan failed). The agent SSE stream and
// the InfrastructurePage both listen here.
func SSEChannel(tenantID uuid.UUID) string {
	return fmt.Sprintf("sse:scan:%s", tenantID)
}

// AgentJobChannel returns the Redis pub/sub channel a tenant's on-prem Agents
// subscribe to (over the SSE stream) to receive queued jobs. All the tenant's
// agents see the job; the first to claim it (Redis lock) runs it.
func AgentJobChannel(tenantID uuid.UUID) string {
	return fmt.Sprintf("sse:scan:agent:%s", tenantID)
}

// AgentJobDispatch is the payload pushed to Agents when a job is queued for an
// agent-based (nmap/agent) config.
type AgentJobDispatch struct {
	Type     string         `json:"type"` // scan.job | agent.revoked
	JobID    uuid.UUID      `json:"job_id"`
	ConfigID uuid.UUID      `json:"config_id"`
	TenantID uuid.UUID      `json:"tenant_id"`
	Provider string         `json:"provider"`
	Targets  []string       `json:"targets"`
	AgentIDs []string       `json:"agent_ids,omitempty"` // restrict to these agents (empty = any)
	Options  map[string]any `json:"options,omitempty"`
}

// ScanEvent is the payload published on SSEChannel and delivered to the browser.
type ScanEvent struct {
	Type        string    `json:"type"` // scan.completed | scan.failed | scan.started
	JobID       uuid.UUID `json:"job_id"`
	ConfigID    uuid.UUID `json:"config_id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	Provider    string    `json:"provider"`
	AgentName   string    `json:"agent_name,omitempty"`
	Assets      int       `json:"assets"`
	Findings    int       `json:"findings"`
	Mitigations int       `json:"mitigations"`
	Error       string    `json:"error,omitempty"`
}

// Notifier is told when a scan finishes so it can fan out to SSE / e-mail /
// in-app. The pipeline never blocks on it: notification is best-effort.
type Notifier interface {
	ScanCompleted(ctx context.Context, p *ScanPreview)
	ScanFailed(ctx context.Context, tenantID, jobID, configID uuid.UUID, provider, agentName, reason string)
}

// InAppSink is the optional hook the RedisNotifier calls to also raise a durable
// in-app notification (and, downstream, an e-mail) for the user who triggered the
// scan. It is deliberately tiny so the scanner package doesn't depend on the
// notification use case directly — the wiring in main.go adapts it to a
// domain.Notification + e-mail. A Nil userID means "no specific recipient" and
// the sink may skip.
type InAppSink func(ctx context.Context, tenantID, userID uuid.UUID, title, message string)

// RedisNotifier publishes ScanEvents on the tenant's SSE channel and, when an
// InAppSink is provided, also raises an in-app notification. SSE is the live
// signal; the in-app entry is the durable one.
type RedisNotifier struct {
	kv    KV
	inApp InAppSink // may be nil
}

func NewRedisNotifier(kv KV, inApp InAppSink) *RedisNotifier {
	return &RedisNotifier{kv: kv, inApp: inApp}
}

func (n *RedisNotifier) ScanCompleted(ctx context.Context, p *ScanPreview) {
	if n == nil || n.kv == nil {
		return
	}
	_ = n.kv.Publish(ctx, SSEChannel(p.TenantID), ScanEvent{
		Type:        "scan.completed",
		JobID:       p.JobID,
		ConfigID:    p.ConfigID,
		TenantID:    p.TenantID,
		Provider:    string(p.Provider),
		AgentName:   p.AgentName,
		Assets:      len(p.Assets),
		Findings:    len(p.Findings),
		Mitigations: len(p.Mitigations),
	})
	if n.inApp != nil {
		src := string(p.Provider)
		if p.AgentName != "" {
			src = fmt.Sprintf("Agent %s", p.AgentName)
		}
		n.inApp(ctx, p.TenantID, p.TriggeredBy, "Scan complete — review pending",
			fmt.Sprintf("%s found %d assets and %d findings. Review and import from the Scan Preview (expires in 48h).",
				src, len(p.Assets), len(p.Findings)))
	}
}

func (n *RedisNotifier) ScanFailed(ctx context.Context, tenantID, jobID, configID uuid.UUID, provider, agentName, reason string) {
	if n == nil || n.kv == nil {
		return
	}
	_ = n.kv.Publish(ctx, SSEChannel(tenantID), ScanEvent{
		Type:      "scan.failed",
		JobID:     jobID,
		ConfigID:  configID,
		TenantID:  tenantID,
		Provider:  provider,
		AgentName: agentName,
		Error:     reason,
	})
	if n.inApp != nil {
		// Recipient is not known at this layer for failures; broadcast to the
		// tenant with a Nil user — the sink decides how to fan out.
		n.inApp(ctx, tenantID, uuid.Nil, "Scan failed", reason)
	}
}

// NoopNotifier satisfies Notifier for tests / wiring where notification is off.
type NoopNotifier struct{}

func (NoopNotifier) ScanCompleted(context.Context, *ScanPreview) {}
func (NoopNotifier) ScanFailed(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, string) {
}
