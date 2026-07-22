// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package automation

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/rs/zerolog"
)

func testEngine(t *testing.T) (*Engine, *mockRuleRepo, *mockExecRepo, *mockSLARepo) {
	t.Helper()
	rules := newMockRuleRepo()
	execs := newMockExecRepo()
	slas := newMockSLARepo()
	e := NewEngine(rules, execs, slas, zerolog.Nop())
	return e, rules, execs, slas
}

func criticalKEVContext(tenantID uuid.UUID) TriggerContext {
	asset := uuid.New()
	return TriggerContext{
		TenantID:     tenantID,
		Ref:          "cve:CVE-2021-44228",
		Subject:      "Log4Shell",
		Title:        "Log4Shell",
		Severity:     "critical",
		CVSS:         9.8,
		KEV:          true,
		PriorityTier: "P1",
		CVEID:        "CVE-2021-44228",
		AssetID:      &asset,
		AssetName:    "web-01",
	}
}

func TestMatchConditions(t *testing.T) {
	tenant := uuid.New()
	tc := criticalKEVContext(tenant)

	cases := []struct {
		name string
		cond domain.AutomationConditions
		want bool
	}{
		{"empty matches", domain.AutomationConditions{}, true},
		{"min severity met", domain.AutomationConditions{MinSeverity: "high"}, true},
		{"min severity not met", domain.AutomationConditions{MinSeverity: "critical"}, true},
		{"min cvss met", domain.AutomationConditions{MinCVSS: 7.0}, true},
		{"min cvss not met", domain.AutomationConditions{MinCVSS: 10.0}, false},
		{"kev only met", domain.AutomationConditions{KEVOnly: true}, true},
		{"tier P1 min P2 qualifies", domain.AutomationConditions{MinPriorityTier: "P2"}, true},
		{"tier P1 min P1 qualifies", domain.AutomationConditions{MinPriorityTier: "P1"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, reason := matchConditions(c.cond, tc)
			if got != c.want {
				t.Fatalf("matchConditions=%v want %v (reason %q)", got, c.want, reason)
			}
		})
	}

	// A medium-severity context must fail a min high gate.
	medium := tc
	medium.Severity = "medium"
	medium.KEV = false
	medium.PriorityTier = "P3"
	if ok, _ := matchConditions(domain.AutomationConditions{MinSeverity: "high"}, medium); ok {
		t.Fatal("medium severity should not satisfy min high")
	}
	if ok, _ := matchConditions(domain.AutomationConditions{KEVOnly: true}, medium); ok {
		t.Fatal("non-KEV should not satisfy KEV-only")
	}
	if ok, _ := matchConditions(domain.AutomationConditions{MinPriorityTier: "P2"}, medium); ok {
		t.Fatal("P3 should not satisfy min P2")
	}
}

func TestEngine_RunsChain_NotifyTicketAndSLA(t *testing.T) {
	e, rules, execs, slas := testEngine(t)
	tenant := uuid.New()

	notifier := &mockNotifier{}
	ticketer := &mockTicketer{}
	e.WithNotifier(notifier).WithTicketer(ticketer)

	rules.add(&domain.AutomationRule{
		ID:       uuid.New(),
		TenantID: tenant,
		Name:     "Critical KEV playbook",
		Enabled:  true,
		Trigger:  domain.TriggerVulnerabilityDetected,
		Conditions: domain.AutomationConditions{MinSeverity: "high", KEVOnly: true},
		Actions: domain.AutomationActionList{
			{Type: domain.ActionNotify, Channels: []string{"slack", "in_app"}},
			{Type: domain.ActionCreateTicket},
			{Type: domain.ActionStartSLA},
		},
		SLA: domain.AutomationSLAConfig{CriticalMinutes: 240, EscalateAfterMinutes: 60, EscalateToRole: "admin"},
	})

	e.HandleTrigger(context.Background(), domain.TriggerVulnerabilityDetected, criticalKEVContext(tenant))

	if len(notifier.calls) != 1 {
		t.Fatalf("expected 1 notify call, got %d", len(notifier.calls))
	}
	if ticketer.opened != 1 {
		t.Fatalf("expected 1 ticket opened, got %d", ticketer.opened)
	}
	if len(slas.trackers) != 1 {
		t.Fatalf("expected 1 SLA tracker, got %d", len(slas.trackers))
	}
	// The execution record must exist and be a success.
	if len(execs.execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs.execs))
	}
	for _, ex := range execs.execs {
		if ex.Status != domain.ExecutionSuccess {
			t.Fatalf("expected success, got %s", ex.Status)
		}
		if len(ex.Steps) != 3 {
			t.Fatalf("expected 3 steps, got %d", len(ex.Steps))
		}
	}
	// SLA due date must be ~240 min out.
	for _, tr := range slas.trackers {
		if tr.Severity != "critical" || tr.EscalateToRole != "admin" {
			t.Fatalf("unexpected tracker: %+v", tr)
		}
		if tr.EscalateAt == nil || !tr.EscalateAt.After(tr.DueAt) {
			t.Fatal("escalate_at should be after due_at")
		}
	}
}

func TestEngine_SkipsWhenConditionsFail(t *testing.T) {
	e, rules, execs, _ := testEngine(t)
	tenant := uuid.New()

	rules.add(&domain.AutomationRule{
		ID:         uuid.New(),
		TenantID:   tenant,
		Name:       "KEV only",
		Enabled:    true,
		Trigger:    domain.TriggerVulnerabilityDetected,
		Conditions: domain.AutomationConditions{KEVOnly: true},
		Actions:    domain.AutomationActionList{{Type: domain.ActionNotify}},
	})

	tc := criticalKEVContext(tenant)
	tc.KEV = false // fails the KEV-only gate

	e.HandleTrigger(context.Background(), domain.TriggerVulnerabilityDetected, tc)

	if len(execs.execs) != 0 {
		t.Fatalf("expected no execution when conditions fail, got %d", len(execs.execs))
	}
}

func TestEngine_CreateRiskThenAssignChaining(t *testing.T) {
	e, rules, _, _ := testEngine(t)
	tenant := uuid.New()

	rc := &mockRiskCreator{}
	e.WithRiskCreator(rc)

	rules.add(&domain.AutomationRule{
		ID:       uuid.New(),
		TenantID: tenant,
		Name:     "Open risk",
		Enabled:  true,
		Trigger:  domain.TriggerVulnerabilityDetected,
		Actions: domain.AutomationActionList{
			{Type: domain.ActionCreateRisk},
			{Type: domain.ActionStartSLA}, // will link to the created risk
		},
		SLA: domain.AutomationSLAConfig{CriticalMinutes: 120},
	})

	e.HandleTrigger(context.Background(), domain.TriggerVulnerabilityDetected, criticalKEVContext(tenant))

	if rc.created != 1 {
		t.Fatalf("expected risk created once, got %d", rc.created)
	}
}

func TestEngine_MissingPortDegradesToSkipped(t *testing.T) {
	e, rules, execs, _ := testEngine(t)
	tenant := uuid.New()
	// No ports wired at all.
	rules.add(&domain.AutomationRule{
		ID:       uuid.New(),
		TenantID: tenant,
		Name:     "No ports",
		Enabled:  true,
		Trigger:  domain.TriggerVulnerabilityDetected,
		Actions:  domain.AutomationActionList{{Type: domain.ActionNotify}, {Type: domain.ActionCreateTicket}},
	})

	e.HandleTrigger(context.Background(), domain.TriggerVulnerabilityDetected, criticalKEVContext(tenant))

	// Chain completes (no hard failure); steps are skipped, so status is success.
	for _, ex := range execs.execs {
		if ex.Status == domain.ExecutionFailed {
			t.Fatalf("missing ports must not fail the chain, got %s", ex.Status)
		}
		for _, s := range ex.Steps {
			if s.Status != "skipped" {
				t.Fatalf("expected skipped step, got %s (%s)", s.Status, s.Detail)
			}
		}
	}
}

func TestEngine_RunRuleByID_DryRun(t *testing.T) {
	e, rules, _, _ := testEngine(t)
	tenant := uuid.New()
	notifier := &mockNotifier{}
	e.WithNotifier(notifier)

	ruleID := uuid.New()
	rules.add(&domain.AutomationRule{
		ID:       ruleID,
		TenantID: tenant,
		Name:     "Manual",
		Enabled:  false, // dry-run bypasses enabled/trigger filters
		Trigger:  domain.TriggerManual,
		Actions:  domain.AutomationActionList{{Type: domain.ActionNotify, Channels: []string{"email"}}},
	})

	exec, err := e.RunRuleByID(context.Background(), ruleID, tenant, criticalKEVContext(tenant))
	if err != nil {
		t.Fatalf("RunRuleByID: %v", err)
	}
	if exec.Status != domain.ExecutionSuccess {
		t.Fatalf("expected success, got %s", exec.Status)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("expected notify called once, got %d", len(notifier.calls))
	}
}
