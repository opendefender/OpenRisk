// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entitlements

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
)

type errOrg struct{}

func (errOrg) PlanAndRegion(context.Context, uuid.UUID) (string, string, error) {
	return "", "", errors.New("organization not found")
}

// TestSelfHosted_Success: a self-hosted deployment resolves every tenant to the
// self-hosted set, whatever the organisation row or a subscription says.
func TestSelfHosted_Success(t *testing.T) {
	canceled := &domain.Subscription{Plan: "pro", Status: domain.SubCanceled}
	s := NewService(fakeOrg{plan: "", region: "africa"}).
		WithSubscriptions(fakeSubs{sub: canceled}).
		WithUsage(fakeUsage{ent.LimitUsers: 500, ent.LimitRisks: 10000}).
		WithDeployment(ent.DeploymentSelfHosted)
	tenant := uuid.New()

	p, r, trial, err := s.EffectivePlan(context.Background(), tenant)
	if err != nil || p != ent.PlanSelfHosted || r != ent.RegionAfrica || trial != nil {
		t.Fatalf("got %v/%v/%v err=%v, want self_hosted/africa/no trial", p, r, trial, err)
	}

	for _, k := range ent.AllLimits {
		allowed, limit, _, _, err := s.Capacity(context.Background(), tenant, k)
		if err != nil || !allowed || limit != ent.Unlimited {
			t.Errorf("Capacity(%s) = %v limit %d err %v, want allowed and unlimited", k, allowed, limit, err)
		}
	}

	snap, err := s.Resolve(context.Background(), tenant)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Plan != string(ent.PlanSelfHosted) {
		t.Fatalf("snapshot plan = %s", snap.Plan)
	}
	for k, l := range snap.Limits {
		if l.Limit != ent.Unlimited {
			t.Errorf("snapshot limit %s = %d, want -1", k, l.Limit)
		}
	}
	if !snap.Features[string(ent.FeatAIAdvisor)].Enabled || !snap.Features[string(ent.FeatAutomation)].Enabled {
		t.Error("AI advisor and automation must be on for self-hosted")
	}
}

// TestSelfHosted_NotFound: an organisation the resolver cannot read surfaces the
// error and grants nothing, self-hosted or not.
func TestSelfHosted_NotFound(t *testing.T) {
	s := NewService(errOrg{}).WithDeployment(ent.DeploymentSelfHosted)
	p, _, _, err := s.EffectivePlan(context.Background(), uuid.New())
	if err == nil || p != ent.PlanFree {
		t.Fatalf("got %s err=%v, want free with the error", p, err)
	}
}

// TestSelfHosted_Unauthorized is criterion 4 of #612: with the flag off, a Free
// tenant is still Free, and no stored or subscription value reaches the
// self-hosted set.
func TestSelfHosted_Unauthorized(t *testing.T) {
	ctx := context.Background()

	for _, d := range []ent.Deployment{ent.DeploymentHosted, ""} {
		s := NewService(fakeOrg{plan: "free"}).WithUsage(fakeUsage{ent.LimitUsers: 2}).WithDeployment(d)
		if p, _, _, _ := s.EffectivePlan(ctx, uuid.New()); p != ent.PlanFree {
			t.Fatalf("deployment %q: Free tenant resolved to %s", d, p)
		}
		if ok, _, _, _, _ := s.Capacity(ctx, uuid.New(), ent.LimitUsers); ok {
			t.Fatalf("deployment %q: Free tenant at 2 users was allowed a third", d)
		}
	}

	stored := NewService(fakeOrg{plan: "self_hosted"}).WithDeployment(ent.DeploymentHosted)
	if p, _, _, _ := stored.EffectivePlan(ctx, uuid.New()); p != ent.PlanFree {
		t.Fatalf("stored plan self_hosted on hosted resolved to %s, want free", p)
	}

	sub := &domain.Subscription{Plan: "self_hosted", Status: domain.SubActive}
	billed := NewService(fakeOrg{plan: "pro"}).WithSubscriptions(fakeSubs{sub: sub}).WithDeployment(ent.DeploymentHosted)
	if p, _, _, _ := billed.EffectivePlan(ctx, uuid.New()); p != ent.PlanFree {
		t.Fatalf("subscription plan self_hosted on hosted resolved to %s, want free", p)
	}
}
