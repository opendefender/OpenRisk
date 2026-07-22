// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package automation

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

func TestRuleService_CreateValidation(t *testing.T) {
	svc := NewRuleService(newMockRuleRepo())
	tenant := uuid.New()
	ctx := context.Background()

	// Invalid trigger.
	if _, err := svc.Create(ctx, tenant, uuid.Nil, RuleInput{
		Name: "x", Trigger: "nope", Actions: domain.AutomationActionList{{Type: domain.ActionNotify}},
	}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error for bad trigger, got %v", err)
	}

	// No actions.
	if _, err := svc.Create(ctx, tenant, uuid.Nil, RuleInput{
		Name: "x", Trigger: string(domain.TriggerVulnerabilityDetected),
	}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error for no actions, got %v", err)
	}

	// start_sla without any budget.
	if _, err := svc.Create(ctx, tenant, uuid.Nil, RuleInput{
		Name: "x", Trigger: string(domain.TriggerVulnerabilityDetected),
		Actions: domain.AutomationActionList{{Type: domain.ActionStartSLA}},
	}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error for start_sla without budget, got %v", err)
	}

	// Valid.
	rule, err := svc.Create(ctx, tenant, uuid.New(), RuleInput{
		Name: "Critical KEV", Trigger: string(domain.TriggerVulnerabilityDetected),
		Conditions: domain.AutomationConditions{KEVOnly: true},
		Actions:    domain.AutomationActionList{{Type: domain.ActionNotify, Channels: []string{"teams"}}},
	})
	if err != nil {
		t.Fatalf("valid rule create failed: %v", err)
	}
	if rule.ID == uuid.Nil || !rule.Enabled || rule.Priority != 100 {
		t.Fatalf("unexpected rule defaults: %+v", rule)
	}
}

func TestRuleService_UpdateAndDeleteTenantScoped(t *testing.T) {
	repo := newMockRuleRepo()
	svc := NewRuleService(repo)
	tenant := uuid.New()
	other := uuid.New()
	ctx := context.Background()

	rule, err := svc.Create(ctx, tenant, uuid.New(), RuleInput{
		Name: "R", Trigger: string(domain.TriggerRiskScoreUpdated),
		Actions: domain.AutomationActionList{{Type: domain.ActionNotify}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// A different tenant cannot fetch/update/delete it.
	if _, err := svc.Get(ctx, other, rule.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant get should be not found, got %v", err)
	}
	if err := svc.Delete(ctx, other, rule.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant delete should be not found, got %v", err)
	}

	// Owner can disable it.
	disabled := false
	updated, err := svc.Update(ctx, tenant, rule.ID, RuleInput{
		Name: "R2", Trigger: string(domain.TriggerRiskScoreUpdated), Enabled: &disabled,
		Actions: domain.AutomationActionList{{Type: domain.ActionNotify}},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Enabled || updated.Name != "R2" {
		t.Fatalf("update not applied: %+v", updated)
	}
}
