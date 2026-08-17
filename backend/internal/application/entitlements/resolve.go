// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entitlements

import (
	"context"

	"github.com/google/uuid"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
)

// Snapshot is the full entitlements payload served at GET /api/entitlements. It
// lists EVERY feature (including the ones off on this plan, so the UI can grey and
// explain them), each limit with current usage, the trial state, and the region's
// price table for the paywall/billing page.
type Snapshot struct {
	Plan      string                `json:"plan"`
	Region    string                `json:"region"`
	Features  map[string]FeatureDTO `json:"features"`
	Limits    map[string]LimitDTO   `json:"limits"`
	Trial     *TrialInfo            `json:"trial,omitempty"`
	Prices    map[string]ent.Price  `json:"prices"`
	TrialDays int                   `json:"trial_days"`
}

// FeatureDTO carries the resolved level and, for disabled features, the least
// plan that unlocks it (so the frontend can say "available from the Pro plan").
type FeatureDTO struct {
	Enabled      bool   `json:"enabled"`
	Level        string `json:"level"`
	RequiredPlan string `json:"required_plan"`
}

// LimitDTO carries a cap and its current usage. Limit == -1 means unlimited.
type LimitDTO struct {
	Limit int `json:"limit"`
	Used  int `json:"used"`
}

// Resolve builds the full entitlements snapshot for a tenant.
func (s *Service) Resolve(ctx context.Context, tenant uuid.UUID) (*Snapshot, error) {
	plan, region, trial, err := s.EffectivePlan(ctx, tenant)
	if err != nil {
		return nil, err
	}

	features := make(map[string]FeatureDTO, len(ent.AllFeatures))
	for _, f := range ent.AllFeatures {
		lvl := ent.LevelOf(plan, f)
		features[string(f)] = FeatureDTO{
			Enabled:      lvl.Enabled(),
			Level:        string(lvl),
			RequiredPlan: string(ent.MinPlanFor(f)),
		}
	}

	limits := make(map[string]LimitDTO, len(ent.AllLimits))
	for _, k := range ent.AllLimits {
		lim := ent.LimitOf(plan, k)
		used := 0
		if lim != ent.Unlimited && s.usage != nil {
			if u, cErr := s.usage.Count(ctx, tenant, k); cErr == nil {
				used = u
			}
		}
		limits[string(k)] = LimitDTO{Limit: lim, Used: used}
	}

	prices := make(map[string]ent.Price, len(ent.AllPlans))
	for p, pr := range ent.PriceTable(region) {
		prices[string(p)] = pr
	}

	return &Snapshot{
		Plan:      string(plan),
		Region:    string(region),
		Features:  features,
		Limits:    limits,
		Trial:     trial,
		Prices:    prices,
		TrialDays: ent.TrialDays,
	}, nil
}
