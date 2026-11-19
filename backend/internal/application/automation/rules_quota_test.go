// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package automation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	entapp "github.com/opendefender/openrisk/internal/application/entitlements"
	"github.com/opendefender/openrisk/internal/domain"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
)

// countingQuota caps enabled rules at limit, counting them in the repository
// the service writes to, so the test sees what the real counter would.
type countingQuota struct {
	repo  *mockRuleRepo
	limit int
}

func (q countingQuota) Capacity(ctx context.Context, tenant uuid.UUID, _ ent.LimitKey) (bool, int, int, ent.Plan, error) {
	rules, _ := q.repo.List(ctx, tenant)
	used := 0
	for _, r := range rules {
		if r.Enabled {
			used++
		}
	}
	return used < q.limit, q.limit, used, ent.PlanFree, nil
}

func quotaRule(enabled bool) RuleInput {
	return RuleInput{
		Name: "KEV", Trigger: string(domain.TriggerVulnerabilityDetected), Enabled: &enabled,
		Actions: domain.AutomationActionList{{Type: domain.ActionNotify, Channels: []string{"email"}}},
	}
}

// TestRuleQuota_Success: Free's two enabled rules fit, a disabled third is a
// free draft, and editing a rule that is already on never counts.
func TestRuleQuota_Success(t *testing.T) {
	repo := newMockRuleRepo()
	svc := NewRuleService(repo).WithQuota(countingQuota{repo: repo, limit: 2})
	ctx, tenant := context.Background(), uuid.New()

	a, err := svc.Create(ctx, tenant, uuid.New(), quotaRule(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, tenant, uuid.New(), quotaRule(true)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, tenant, uuid.New(), quotaRule(false)); err != nil {
		t.Fatalf("a disabled draft must not count against the cap: %v", err)
	}
	in := quotaRule(true)
	in.Name = "KEV renamed"
	if _, err := svc.Update(ctx, tenant, a.ID, in); err != nil {
		t.Fatalf("editing a rule that is already on must pass at the cap: %v", err)
	}
}

// TestRuleQuota_Unauthorized: at the cap, every way to turn a rule on is
// refused with the numbers — create enabled, update to enabled, resume.
func TestRuleQuota_Unauthorized(t *testing.T) {
	repo := newMockRuleRepo()
	svc := NewRuleService(repo).WithQuota(countingQuota{repo: repo, limit: 2})
	ctx, tenant := context.Background(), uuid.New()
	for i := 0; i < 2; i++ {
		if _, err := svc.Create(ctx, tenant, uuid.New(), quotaRule(true)); err != nil {
			t.Fatal(err)
		}
	}
	draft, err := svc.Create(ctx, tenant, uuid.New(), quotaRule(false))
	if err != nil {
		t.Fatal(err)
	}

	assertLimit := func(what string, err error) {
		t.Helper()
		var lim *domain.LimitReachedError
		if !errors.As(err, &lim) || lim.Key != string(ent.LimitAutomationRules) || lim.Limit != 2 || lim.Used != 2 {
			t.Fatalf("%s at 2/2: want LimitReachedError automation_rules 2/2, got %v", what, err)
		}
		if !errors.Is(err, domain.ErrForbidden) || domain.HTTPStatusFromError(err) != 402 {
			t.Fatalf("%s: a reached cap is forbidden and maps to 402", what)
		}
	}
	_, err = svc.Create(ctx, tenant, uuid.New(), quotaRule(true))
	assertLimit("create enabled", err)
	_, err = svc.Update(ctx, tenant, draft.ID, quotaRule(true))
	assertLimit("update to enabled", err)
	_, err = svc.Enable(ctx, tenant, draft.ID, uuid.New())
	assertLimit("enable", err)
}

// TestRuleQuota_NotFound: resuming an unknown rule is NotFound, not a cap
// refusal — the tenant learns the right thing.
func TestRuleQuota_NotFound(t *testing.T) {
	repo := newMockRuleRepo()
	svc := NewRuleService(repo).WithQuota(countingQuota{repo: repo, limit: 2})
	_, err := svc.Enable(context.Background(), uuid.New(), uuid.New(), uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

type freeOrg struct{}

func (freeOrg) PlanAndRegion(context.Context, uuid.UUID) (string, string, error) {
	return "free", "eu", nil
}

type ruleUsage int

func (u ruleUsage) Count(context.Context, uuid.UUID, ent.LimitKey, time.Time) (int, error) {
	return int(u), nil
}

// TestRuleQuota_SelfHosted_Success (#783 criterion 8): through the real
// entitlements service, a self-hosted install enables rules past every hosted
// cap on each path, while the same stored Free plan, hosted, is refused.
func TestRuleQuota_SelfHosted_Success(t *testing.T) {
	ctx, tenant := context.Background(), uuid.New()
	self := entapp.NewService(freeOrg{}).WithUsage(ruleUsage(10000)).WithDeployment(ent.DeploymentSelfHosted)
	repo := newMockRuleRepo()
	svc := NewRuleService(repo).WithQuota(self)

	draft, err := svc.Create(ctx, tenant, uuid.New(), quotaRule(false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, tenant, uuid.New(), quotaRule(true)); err != nil {
		t.Fatalf("self-hosted create enabled: %v", err)
	}
	if _, err := svc.Update(ctx, tenant, draft.ID, quotaRule(true)); err != nil {
		t.Fatalf("self-hosted update to enabled: %v", err)
	}

	hosted := NewRuleService(repo).WithQuota(entapp.NewService(freeOrg{}).WithUsage(ruleUsage(2)))
	var lim *domain.LimitReachedError
	if _, err := hosted.Create(ctx, tenant, uuid.New(), quotaRule(true)); !errors.As(err, &lim) {
		t.Fatalf("hosted Free at 2 must refuse, got %v", err)
	}
}
