// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package automation

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// Rule lifecycle and live state.
//
// A rule has four things an operator needs at a glance: is it on, did it just
// work, is it about to be paused for good reason, and can I put it back. This
// file holds those verbs plus the state snapshot the UI polls.
// ---------------------------------------------------------------------------

// Rule health states, derived rather than stored so they cannot go stale.
const (
	RuleHealthOK        = "ok"        // last run succeeded (or never ran)
	RuleHealthDegraded  = "degraded"  // last run was partial
	RuleHealthFailing   = "failing"   // last run failed
	RuleHealthSuspended = "suspended" // deliberately paused
	RuleHealthIdle      = "idle"      // enabled but never fired
)

// Enable resumes a suspended rule.
func (s *RuleService) Enable(ctx context.Context, tenantID, id, actorID uuid.UUID) (*domain.AutomationRule, error) {
	if err := s.repo.SetEnabled(ctx, id, tenantID, true, actorID, "", time.Now().UTC()); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id, tenantID)
}

// Suspend pauses a rule. A reason is required: a paused automation with no
// explanation is the thing that gets silently re-enabled by the next person and
// re-breaks whatever it was paused for.
func (s *RuleService) Suspend(ctx context.Context, tenantID, id, actorID uuid.UUID, reason string) (*domain.AutomationRule, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, domain.NewValidationError("a reason is required to suspend a rule — the next person needs to know why before re-enabling it")
	}
	if err := s.repo.SetEnabled(ctx, id, tenantID, false, actorID, reason, time.Now().UTC()); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id, tenantID)
}

// CreateFromTemplate adopts a ready-made playbook. The template is copied, not
// referenced: editing the rule afterwards must never be blocked by, or leak
// back into, the shared template.
func (s *RuleService) CreateFromTemplate(ctx context.Context, tenantID, createdBy uuid.UUID, key, nameOverride string) (*domain.AutomationRule, error) {
	tpl, ok := domain.FindAutomationTemplate(key)
	if !ok {
		return nil, domain.NewValidationError("unknown automation template: " + key)
	}
	name := strings.TrimSpace(nameOverride)
	if name == "" {
		name = tpl.Name
	}
	rule := &domain.AutomationRule{
		TenantID:    tenantID,
		Name:        name,
		Description: tpl.Description,
		// Adopted rules start suspended on purpose: a template that begins firing
		// against production the second it is clicked is exactly the surprise this
		// module is meant to remove. Dry-run it, then enable it.
		Enabled:         false,
		SuspendedReason: "created from a template — dry-run it, then enable it",
		Trigger:         tpl.Trigger,
		Conditions:      tpl.Conditions,
		Actions:         tpl.Actions,
		SLA:             tpl.SLA,
		Priority:        tpl.Priority,
		TemplateKey:     tpl.Key,
		CreatedBy:       createdBy,
	}
	now := time.Now().UTC()
	rule.SuspendedAt = &now
	if createdBy != uuid.Nil {
		a := createdBy
		rule.SuspendedBy = &a
	}
	if err := rule.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// RuleState is the live indicator the UI polls for one rule.
type RuleState struct {
	RuleID        uuid.UUID  `json:"rule_id"`
	Name          string     `json:"name"`
	Sentence      string     `json:"sentence"`
	Trigger       string     `json:"trigger"`
	Enabled       bool       `json:"enabled"`
	Health        string     `json:"health"`
	HealthDetail  string     `json:"health_detail"`
	LastStatus    string     `json:"last_status,omitempty"`
	LastRunAt     *time.Time `json:"last_run_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	FailureStreak int        `json:"failure_streak"`
	TriggerCount  int        `json:"trigger_count"`
	SuspendedAt   *time.Time `json:"suspended_at,omitempty"`
	SuspendReason string     `json:"suspended_reason,omitempty"`
	TemplateKey   string     `json:"template_key,omitempty"`
}

// AutomationState is the whole module's live state: every rule plus the counts
// a status strip shows.
type AutomationState struct {
	Rules      []RuleState `json:"rules"`
	Active     int         `json:"active"`
	Suspended  int         `json:"suspended"`
	Failing    int         `json:"failing"`
	Degraded   int         `json:"degraded"`
	Idle       int         `json:"idle"`
	ObservedAt time.Time   `json:"observed_at"`
}

// State builds the live indicator for every rule of a tenant.
func (s *RuleService) State(ctx context.Context, tenantID uuid.UUID, locale string) (*AutomationState, error) {
	rules, err := s.repo.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := &AutomationState{Rules: make([]RuleState, 0, len(rules)), ObservedAt: time.Now().UTC()}
	for i := range rules {
		r := rules[i]
		st := RuleState{
			RuleID:        r.ID,
			Name:          r.Name,
			Sentence:      r.Describe(locale),
			Trigger:       string(r.Trigger),
			Enabled:       r.Enabled,
			LastStatus:    r.LastStatus,
			LastRunAt:     r.LastExecutedAt,
			LastError:     r.LastError,
			FailureStreak: r.FailureStreak,
			TriggerCount:  r.TriggerCount,
			SuspendedAt:   r.SuspendedAt,
			SuspendReason: r.SuspendedReason,
			TemplateKey:   r.TemplateKey,
		}
		st.Health, st.HealthDetail = ruleHealth(&r)
		switch st.Health {
		case RuleHealthSuspended:
			out.Suspended++
		case RuleHealthFailing:
			out.Failing++
		case RuleHealthDegraded:
			out.Degraded++
		case RuleHealthIdle:
			out.Idle++
		default:
			out.Active++
		}
		out.Rules = append(out.Rules, st)
	}
	return out, nil
}

// ruleHealth derives a rule's state and a sentence explaining it. Derived, not
// stored, so a health badge can never disagree with the underlying columns.
func ruleHealth(r *domain.AutomationRule) (string, string) {
	if !r.Enabled {
		detail := "paused"
		if r.SuspendedReason != "" {
			detail = "paused — " + r.SuspendedReason
		}
		return RuleHealthSuspended, detail
	}
	switch r.LastStatus {
	case string(domain.ExecutionFailed):
		detail := "the last run failed"
		if r.LastError != "" {
			detail += ": " + r.LastError
		}
		if r.FailureStreak > 1 {
			detail += " (" + itoa(r.FailureStreak) + " consecutive failures)"
		}
		return RuleHealthFailing, detail
	case string(domain.ExecutionPartial):
		detail := "the last run completed with at least one failed step"
		if r.LastError != "" {
			detail += ": " + r.LastError
		}
		return RuleHealthDegraded, detail
	case string(domain.ExecutionSuccess):
		return RuleHealthOK, "the last run succeeded"
	}
	if r.TriggerCount == 0 {
		return RuleHealthIdle, "enabled, but no matching event has fired yet"
	}
	return RuleHealthOK, "enabled"
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		b[p] = '-'
	}
	return string(b[p:])
}

// ExecutionHistory is one row of the run history, enriched for display.
type ExecutionHistory struct {
	domain.AutomationExecution
	// StepSummary is "3 ok · 1 skipped · 1 failed" — the one line that lets a
	// reader triage a list of runs without expanding each one.
	StepSummary string `json:"step_summary"`
}

// Summarise builds the per-run step tally.
func Summarise(e domain.AutomationExecution) ExecutionHistory {
	ok, skipped, failed := 0, 0, 0
	for _, s := range e.Steps {
		switch s.Status {
		case "success":
			ok++
		case "skipped":
			skipped++
		case "failed":
			failed++
		}
	}
	parts := []string{}
	if ok > 0 {
		parts = append(parts, itoa(ok)+" ok")
	}
	if skipped > 0 {
		parts = append(parts, itoa(skipped)+" skipped")
	}
	if failed > 0 {
		parts = append(parts, itoa(failed)+" failed")
	}
	if len(parts) == 0 {
		parts = append(parts, "no steps")
	}
	return ExecutionHistory{AutomationExecution: e, StepSummary: strings.Join(parts, " · ")}
}
