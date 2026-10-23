// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package entitlements

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
)

type fakeOrg struct{ plan, region string }

func (f fakeOrg) PlanAndRegion(context.Context, uuid.UUID) (string, string, error) {
	return f.plan, f.region, nil
}

type fakeSubs struct{ sub *domain.Subscription }

func (f fakeSubs) GetByTenant(context.Context, uuid.UUID) (*domain.Subscription, error) {
	return f.sub, nil
}

type fakeUsage map[ent.LimitKey]int

func (f fakeUsage) Count(_ context.Context, _ uuid.UUID, k ent.LimitKey) (int, error) {
	return f[k], nil
}

func TestEffectivePlan_OrgFallback(t *testing.T) {
	s := NewService(fakeOrg{plan: "free", region: "eu"})
	p, r, trial, err := s.EffectivePlan(context.Background(), uuid.New())
	if err != nil || p != ent.PlanFree || r != ent.RegionEU || trial != nil {
		t.Fatalf("got %v/%v/%v err=%v", p, r, trial, err)
	}
}

func TestEffectivePlan_SubscriptionOverridesAndTrial(t *testing.T) {
	end := time.Now().Add(72 * time.Hour)
	sub := &domain.Subscription{Plan: "pro", Region: "africa", Status: domain.SubTrialing, TrialEndsAt: &end}
	s := NewService(fakeOrg{plan: "free", region: "eu"}).WithSubscriptions(fakeSubs{sub: sub})

	p, r, trial, err := s.EffectivePlan(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if p != ent.PlanPro {
		t.Fatalf("trialing pro sub should grant pro, got %s", p)
	}
	if r != ent.RegionAfrica {
		t.Fatalf("region should follow subscription, got %s", r)
	}
	if trial == nil || !trial.Active || trial.DaysLeft < 2 {
		t.Fatalf("trial should be active with days left, got %+v", trial)
	}
}

func TestEffectivePlan_CanceledSubFallsBackToFree(t *testing.T) {
	sub := &domain.Subscription{Plan: "business", Status: domain.SubCanceled}
	s := NewService(fakeOrg{plan: "business", region: "eu"}).WithSubscriptions(fakeSubs{sub: sub})
	p, _, _, _ := s.EffectivePlan(context.Background(), uuid.New())
	if p != ent.PlanFree {
		t.Fatalf("canceled sub must fall back to Free, got %s", p)
	}
}

func TestAllowed_FeatureGate(t *testing.T) {
	// Threat intel keeps its Business gate under D-078.
	s := NewService(fakeOrg{plan: "free"})
	ok, _, req, _ := s.Allowed(context.Background(), uuid.New(), ent.FeatCTI)
	if ok {
		t.Fatal("Free must not be allowed threat intel")
	}
	if req != ent.PlanBusiness {
		t.Fatalf("required plan should be business, got %s", req)
	}

	s2 := NewService(fakeOrg{plan: "business"})
	ok2, _, _, _ := s2.Allowed(context.Background(), uuid.New(), ent.FeatCTI)
	if !ok2 {
		t.Fatal("Business must be allowed threat intel")
	}
}

func TestCapacity(t *testing.T) {
	s := NewService(fakeOrg{plan: "free"}).WithUsage(fakeUsage{ent.LimitRisks: 30})
	ok, limit, used, _, _ := s.Capacity(context.Background(), uuid.New(), ent.LimitRisks)
	if ok || limit != 30 || used != 30 {
		t.Fatalf("at 30/30 must refuse; got ok=%v limit=%d used=%d", ok, limit, used)
	}

	s2 := NewService(fakeOrg{plan: "free"}).WithUsage(fakeUsage{ent.LimitRisks: 10})
	ok2, _, _, _, _ := s2.Capacity(context.Background(), uuid.New(), ent.LimitRisks)
	if !ok2 {
		t.Fatal("10/30 must allow")
	}

	// Unlimited short-circuits without counting.
	s3 := NewService(fakeOrg{plan: "business"})
	ok3, limit3, _, _, _ := s3.Capacity(context.Background(), uuid.New(), ent.LimitRisks)
	if !ok3 || limit3 != ent.Unlimited {
		t.Fatalf("business risks unlimited; got ok=%v limit=%d", ok3, limit3)
	}
}

func TestResolve_ListsAllFeaturesAndPrices(t *testing.T) {
	s := NewService(fakeOrg{plan: "pro", region: "africa"}).WithUsage(fakeUsage{ent.LimitRisks: 12})
	snap, err := s.Resolve(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Features) != len(ent.AllFeatures) {
		t.Fatalf("expected all %d features, got %d", len(ent.AllFeatures), len(snap.Features))
	}
	if !snap.Features[string(ent.FeatFinancialQuant)].Enabled {
		t.Fatal("pro should enable financial quant")
	}
	if snap.Features[string(ent.FeatCTI)].Enabled {
		t.Fatal("pro should NOT enable threat intel")
	}
	if snap.Features[string(ent.FeatCTI)].RequiredPlan != string(ent.PlanBusiness) {
		t.Fatal("threat intel required plan should be business")
	}
	if got := snap.Limits[string(ent.LimitFrameworks)]; got.Limit != 3 {
		t.Fatalf("pro frameworks capped at 3 expected, got %+v", got)
	}
	if snap.Limits[string(ent.LimitRisks)].Used != 12 || snap.Limits[string(ent.LimitRisks)].Limit != 500 {
		t.Fatalf("pro risks 12/500 expected, got %+v", snap.Limits[string(ent.LimitRisks)])
	}
	if snap.Prices[string(ent.PlanPro)].Currency != "XAF" {
		t.Fatal("africa region should price in XAF")
	}
}

type countingOrg struct {
	plan  string
	calls *int
	err   error
}

func (f countingOrg) PlanAndRegion(context.Context, uuid.UUID) (string, string, error) {
	*f.calls++
	return f.plan, "eu", f.err
}

func TestLevel_Success(t *testing.T) {
	calls := 0
	s := NewService(countingOrg{plan: "pro", calls: &calls})
	tenant := uuid.New()

	lvl, plan, err := s.Level(context.Background(), tenant, ent.FeatAutomation)
	if err != nil || lvl != ent.LevelOn || plan != ent.PlanPro {
		t.Fatalf("pro automation: got %q/%s err=%v, want on/pro", lvl, plan, err)
	}
	lvl, _, _ = s.Level(context.Background(), tenant, ent.FeatFinancialQuant)
	if lvl != ent.LevelOn {
		t.Fatalf("pro financial: got %q, want on", lvl)
	}
	// Both reads share the cached plan: one org lookup, like Allowed.
	if calls != 1 {
		t.Fatalf("org reader called %d times, want 1 (cached)", calls)
	}
	// The cache is per tenant: another tenant is its own lookup.
	_, _, _ = s.Level(context.Background(), uuid.New(), ent.FeatCompliance)
	if calls != 2 {
		t.Fatalf("a second tenant must not reuse the first's plan; calls = %d", calls)
	}
}

func TestLevel_NotFound(t *testing.T) {
	// An unknown tenant (the org lookup fails) reports no depth at all, with
	// the error, so the middleware can apply its fail-open posture knowingly.
	calls := 0
	s := NewService(countingOrg{calls: &calls, err: domain.ErrNotFound})
	lvl, _, err := s.Level(context.Background(), uuid.New(), ent.FeatCompliance)
	if err == nil || lvl != ent.LevelOff {
		t.Fatalf("got %q err=%v, want off + error", lvl, err)
	}
}

func TestLevel_Unauthorized(t *testing.T) {
	// A feature the plan does not grant is LevelOff — below every depth.
	s := NewService(fakeOrg{plan: "free"})
	lvl, plan, err := s.Level(context.Background(), uuid.New(), ent.FeatCTI)
	if err != nil || lvl != ent.LevelOff || plan != ent.PlanFree {
		t.Fatalf("free threat intel: got %q/%s err=%v, want off/free", lvl, plan, err)
	}
	if lvl.AtLeast(ent.LevelBasic) {
		t.Fatal("off must not reach basic")
	}
}

// TestCapacity_FrameworkCapPerPlan pins D-076 through the resolver the
// middleware calls: each hosted plan admits its last framework and refuses the
// one after, Enterprise never refuses.
func TestCapacity_FrameworkCapPerPlan(t *testing.T) {
	for _, tc := range []struct {
		plan string
		cap  int
	}{{"free", 1}, {"pro", 3}, {"business", 10}} {
		under := NewService(fakeOrg{plan: tc.plan}).WithUsage(fakeUsage{ent.LimitFrameworks: tc.cap - 1})
		ok, limit, _, _, err := under.Capacity(context.Background(), uuid.New(), ent.LimitFrameworks)
		if err != nil || !ok || limit != tc.cap {
			t.Errorf("%s at %d/%d must admit one more; got ok=%v limit=%d err=%v", tc.plan, tc.cap-1, tc.cap, ok, limit, err)
		}
		at := NewService(fakeOrg{plan: tc.plan}).WithUsage(fakeUsage{ent.LimitFrameworks: tc.cap})
		ok, limit, used, _, err := at.Capacity(context.Background(), uuid.New(), ent.LimitFrameworks)
		if err != nil || ok || limit != tc.cap || used != tc.cap {
			t.Errorf("%s at %d/%d must refuse; got ok=%v limit=%d used=%d err=%v", tc.plan, tc.cap, tc.cap, ok, limit, used, err)
		}
	}
	ent500 := NewService(fakeOrg{plan: "enterprise"}).WithUsage(fakeUsage{ent.LimitFrameworks: 500})
	if ok, limit, _, _, _ := ent500.Capacity(context.Background(), uuid.New(), ent.LimitFrameworks); !ok || limit != ent.Unlimited {
		t.Errorf("enterprise is uncapped; got ok=%v limit=%d", ok, limit)
	}
}
