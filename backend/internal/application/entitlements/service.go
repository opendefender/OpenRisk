// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package entitlements (application) resolves the EFFECTIVE plan for a tenant and
// enforces the open-core matrix. It composes tenant-scoped readers behind narrow,
// nil-safe ports (same contract as the rest of the codebase) and caches the
// effective plan briefly so the enforcement middleware stays cheap.
package entitlements

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
)

// OrgReader returns an organization's stored plan and region default.
type OrgReader interface {
	PlanAndRegion(ctx context.Context, tenant uuid.UUID) (plan, region string, err error)
}

// SubscriptionReader returns the tenant's subscription, or (nil, nil) if none.
// Optional (nil-safe): without it, the effective plan is the org's stored plan.
type SubscriptionReader interface {
	GetByTenant(ctx context.Context, tenant uuid.UUID) (*domain.Subscription, error)
}

// UsageCounter counts the current usage of a capped resource for a tenant.
// Optional (nil-safe): without it, usage reads as 0 and limits are advisory.
type UsageCounter interface {
	Count(ctx context.Context, tenant uuid.UUID, key ent.LimitKey) (int, error)
}

// Service resolves and enforces entitlements.
type Service struct {
	orgs  OrgReader
	subs  SubscriptionReader
	usage UsageCounter

	now func() time.Time
	ttl time.Duration
	mu  sync.Mutex
	c   map[uuid.UUID]cached
}

type cached struct {
	plan   ent.Plan
	region ent.Region
	trial  *TrialInfo
	exp    time.Time
}

// NewService builds a resolver. subs/usage may be attached via With*.
func NewService(orgs OrgReader) *Service {
	return &Service{
		orgs: orgs,
		now:  time.Now,
		ttl:  30 * time.Second,
		c:    make(map[uuid.UUID]cached),
	}
}

// WithSubscriptions attaches the subscription reader (makes plans billing-driven).
func (s *Service) WithSubscriptions(r SubscriptionReader) *Service { s.subs = r; return s }

// WithUsage attaches the usage counter (makes limits enforceable, not advisory).
func (s *Service) WithUsage(u UsageCounter) *Service { s.usage = u; return s }

// TrialInfo describes an active no-card trial.
type TrialInfo struct {
	Active   bool       `json:"active"`
	EndsAt   *time.Time `json:"ends_at,omitempty"`
	DaysLeft int        `json:"days_left"`
}

// EffectivePlan resolves the plan a tenant actually gets right now, plus its
// region and any active trial. Cached for the service TTL.
func (s *Service) EffectivePlan(ctx context.Context, tenant uuid.UUID) (ent.Plan, ent.Region, *TrialInfo, error) {
	now := s.now()
	s.mu.Lock()
	if hit, ok := s.c[tenant]; ok && hit.exp.After(now) {
		s.mu.Unlock()
		return hit.plan, hit.region, hit.trial, nil
	}
	s.mu.Unlock()

	planStr, regionStr, err := s.orgs.PlanAndRegion(ctx, tenant)
	if err != nil {
		return ent.PlanFree, ent.RegionEU, nil, err
	}
	plan := ent.ParsePlan(planStr)
	region := ent.ParseRegion(regionStr)
	var trial *TrialInfo

	// A subscription, when present, is the source of truth: entitled → its plan;
	// present-but-not-entitled (canceled/past-due) → fall back to Free. No
	// subscription at all → the org's stored plan (dev/seeded/manual orgs).
	if s.subs != nil {
		if sub, sErr := s.subs.GetByTenant(ctx, tenant); sErr == nil && sub != nil {
			region = ent.ParseRegion(sub.Region)
			if sub.Entitled(now) {
				plan = ent.ParsePlan(sub.Plan)
			} else {
				plan = ent.PlanFree
			}
			if sub.TrialActive(now) {
				days := int(sub.TrialEndsAt.Sub(now).Hours()/24) + 1
				trial = &TrialInfo{Active: true, EndsAt: sub.TrialEndsAt, DaysLeft: days}
			}
		}
	}

	s.mu.Lock()
	s.c[tenant] = cached{plan: plan, region: region, trial: trial, exp: now.Add(s.ttl)}
	s.mu.Unlock()
	return plan, region, trial, nil
}

// Invalidate drops the cached plan for a tenant (call after a plan change).
func (s *Service) Invalidate(tenant uuid.UUID) {
	s.mu.Lock()
	delete(s.c, tenant)
	s.mu.Unlock()
}

// Allowed reports whether a tenant's effective plan grants a feature, and the
// least plan that would (for upgrade copy).
func (s *Service) Allowed(ctx context.Context, tenant uuid.UUID, f ent.Feature) (bool, ent.Plan, ent.Plan, error) {
	plan, _, _, err := s.EffectivePlan(ctx, tenant)
	if err != nil {
		return false, plan, ent.MinPlanFor(f), err
	}
	return ent.Has(plan, f), plan, ent.MinPlanFor(f), nil
}

// Capacity reports whether a tenant may create one more of a capped resource.
// Unlimited caps short-circuit without counting.
func (s *Service) Capacity(ctx context.Context, tenant uuid.UUID, key ent.LimitKey) (allowed bool, limit, used int, plan ent.Plan, err error) {
	plan, _, _, err = s.EffectivePlan(ctx, tenant)
	if err != nil {
		return false, 0, 0, plan, err
	}
	limit = ent.LimitOf(plan, key)
	if limit == ent.Unlimited {
		return true, ent.Unlimited, -1, plan, nil
	}
	if s.usage != nil {
		if used, err = s.usage.Count(ctx, tenant, key); err != nil {
			// Fail OPEN on a counting error: refusing a legitimate write because a
			// COUNT query hiccuped is worse than briefly allowing one over.
			return true, limit, -1, plan, nil
		}
	}
	return used < limit, limit, used, plan, nil
}
