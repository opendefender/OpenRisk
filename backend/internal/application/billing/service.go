// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package billing (application) drives the subscription lifecycle: start a no-card
// trial, open a hosted checkout, apply a plan change (from a webhook or an admin),
// and cancel. It keeps the Organization.plan column and the subscription row in
// sync, and invalidates the entitlements cache so a plan change takes effect at
// once.
package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	pkgbilling "github.com/opendefender/openrisk/pkg/billing"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
)

// SubscriptionStore persists subscriptions/invoices and the org plan mirror.
type SubscriptionStore interface {
	GetByTenant(ctx context.Context, tenant uuid.UUID) (*domain.Subscription, error)
	Upsert(ctx context.Context, sub *domain.Subscription) error
	SetOrganizationPlanAndRegion(ctx context.Context, tenant uuid.UUID, plan, region string) error
	ListInvoices(ctx context.Context, tenant uuid.UUID) ([]domain.Invoice, error)
}

// CacheInvalidator drops the cached effective plan for a tenant.
type CacheInvalidator interface{ Invalidate(tenant uuid.UUID) }

// Service is the billing application service.
type Service struct {
	store    SubscriptionStore
	gateways *pkgbilling.Registry
	cache    CacheInvalidator
	baseURL  string
	now      func() time.Time
}

func NewService(store SubscriptionStore, gateways *pkgbilling.Registry) *Service {
	return &Service{store: store, gateways: gateways, now: time.Now, baseURL: ""}
}

// WithCache attaches the entitlements cache so plan changes take effect at once.
func (s *Service) WithCache(c CacheInvalidator) *Service { s.cache = c; return s }

// WithBaseURL sets the app URL used to build checkout success/cancel links.
func (s *Service) WithBaseURL(u string) *Service { s.baseURL = u; return s }

var (
	// ErrInvalidPlan is returned for a plan that cannot be subscribed to directly.
	ErrInvalidPlan = errors.New("billing: plan is not purchasable (choose pro or business)")
	// ErrAlreadySubscribed is returned when a trial is requested but the tenant
	// already has an entitled subscription.
	ErrAlreadySubscribed = errors.New("billing: organization already has an active subscription")
	// ErrNoGateway is returned when no payment provider is configured.
	ErrNoGateway = errors.New("billing: no payment provider configured for this currency")
)

// purchasable reports whether a plan can be self-served (pro/business). Free is a
// downgrade target, Enterprise is quote-based (sales).
func purchasable(p ent.Plan) bool { return p == ent.PlanPro || p == ent.PlanBusiness }

// StartTrial begins a 14-day, no-credit-card trial of a paid plan. The org gets
// full plan entitlements immediately; the subscription is TRIALING until the trial
// ends. Refuses if the tenant is already entitled.
func (s *Service) StartTrial(ctx context.Context, tenant uuid.UUID, planStr, regionStr string) (*domain.Subscription, error) {
	plan := ent.ParsePlan(planStr)
	if !purchasable(plan) {
		return nil, ErrInvalidPlan
	}
	region := ent.ParseRegion(regionStr)
	now := s.now()

	if existing, err := s.store.GetByTenant(ctx, tenant); err != nil {
		return nil, err
	} else if existing != nil && existing.Entitled(now) {
		return nil, ErrAlreadySubscribed
	}

	trialEnds := now.Add(time.Duration(ent.TrialDays) * 24 * time.Hour)
	sub := &domain.Subscription{
		OrganizationID: tenant,
		Plan:           string(plan),
		Status:         domain.SubTrialing,
		Region:         string(region),
		Provider:       domain.ProviderManual,
		TrialEndsAt:    &trialEnds,
	}
	if err := s.store.Upsert(ctx, sub); err != nil {
		return nil, err
	}
	if err := s.store.SetOrganizationPlanAndRegion(ctx, tenant, string(plan), string(region)); err != nil {
		return nil, err
	}
	s.invalidate(tenant)
	return sub, nil
}

// Checkout opens a hosted payment session for a plan on the tenant's region,
// choosing (or honouring) a provider. It does NOT change the plan — that happens
// when the provider webhook confirms payment (ApplyPlan).
func (s *Service) Checkout(ctx context.Context, tenant uuid.UUID, planStr, regionStr, email string, provider pkgbilling.Provider) (*pkgbilling.CheckoutSession, error) {
	plan := ent.ParsePlan(planStr)
	if !purchasable(plan) {
		return nil, ErrInvalidPlan
	}
	region := ent.ParseRegion(regionStr)
	price := ent.PriceFor(region, plan)
	if price.Custom {
		return nil, ErrInvalidPlan
	}

	var gw pkgbilling.Gateway
	if provider != "" {
		gw = s.gateways.Get(provider)
	}
	if gw == nil || !gw.Configured() {
		gw = s.gateways.Default(price.Currency)
	}
	if gw == nil {
		return nil, ErrNoGateway
	}

	// Stripe expects the minor unit (cents); XAF has no minor unit, so the amount
	// is used as-is.
	amountMinor := price.Amount
	if price.Currency == "EUR" {
		amountMinor = price.Amount * 100
	}
	ref := fmt.Sprintf("%s:%s:%d", tenant.String(), plan, s.now().Unix())
	return gw.CreateCheckout(ctx, pkgbilling.CheckoutRequest{
		TenantID:      tenant.String(),
		Plan:          string(plan),
		AmountMinor:   amountMinor,
		Currency:      price.Currency,
		CustomerEmail: email,
		Reference:     ref,
		SuccessURL:    s.baseURL + "/settings?tab=billing&checkout=success",
		CancelURL:     s.baseURL + "/settings?tab=billing&checkout=cancel",
	})
}

// ApplyPlan activates a plan for a tenant (called by a confirmed webhook or by an
// admin/manual change). Free downgrades the org to the open-core plan and cancels
// any subscription.
func (s *Service) ApplyPlan(ctx context.Context, tenant uuid.UUID, planStr, regionStr string, provider domain.BillingProvider, providerSubID string) (*domain.Subscription, error) {
	plan := ent.ParsePlan(planStr)
	region := ent.ParseRegion(regionStr)
	now := s.now()

	sub, err := s.store.GetByTenant(ctx, tenant)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		sub = &domain.Subscription{OrganizationID: tenant}
	}
	sub.Region = string(region)
	if plan == ent.PlanFree {
		sub.Plan = string(ent.PlanFree)
		sub.Status = domain.SubCanceled
		sub.CanceledAt = &now
	} else {
		sub.Plan = string(plan)
		sub.Status = domain.SubActive
		sub.Provider = provider
		sub.ProviderSubscriptionID = providerSubID
		sub.CanceledAt = nil
		sub.CancelAtPeriodEnd = false
		periodEnd := now.AddDate(0, 1, 0)
		sub.CurrentPeriodEnd = &periodEnd
	}
	if err := s.store.Upsert(ctx, sub); err != nil {
		return nil, err
	}
	if err := s.store.SetOrganizationPlanAndRegion(ctx, tenant, sub.Plan, string(region)); err != nil {
		return nil, err
	}
	s.invalidate(tenant)
	return sub, nil
}

// Cancel schedules a cancellation. A trialing subscription is cancelled at once
// (and the org drops to Free); an active one is set to cancel at period end so the
// customer keeps what they paid for until the period closes.
func (s *Service) Cancel(ctx context.Context, tenant uuid.UUID) (*domain.Subscription, error) {
	sub, err := s.store.GetByTenant(ctx, tenant)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, ErrInvalidPlan
	}
	now := s.now()
	if sub.Status == domain.SubTrialing {
		sub.Status = domain.SubCanceled
		sub.CanceledAt = &now
		if err := s.store.Upsert(ctx, sub); err != nil {
			return nil, err
		}
		if err := s.store.SetOrganizationPlanAndRegion(ctx, tenant, string(ent.PlanFree), sub.Region); err != nil {
			return nil, err
		}
		s.invalidate(tenant)
		return sub, nil
	}
	sub.CancelAtPeriodEnd = true
	sub.CanceledAt = &now
	if err := s.store.Upsert(ctx, sub); err != nil {
		return nil, err
	}
	s.invalidate(tenant)
	return sub, nil
}

// Get returns the tenant's subscription (nil if none) and its invoices.
func (s *Service) Get(ctx context.Context, tenant uuid.UUID) (*domain.Subscription, []domain.Invoice, error) {
	sub, err := s.store.GetByTenant(ctx, tenant)
	if err != nil {
		return nil, nil, err
	}
	inv, err := s.store.ListInvoices(ctx, tenant)
	if err != nil {
		return sub, nil, err
	}
	return sub, inv, nil
}

func (s *Service) invalidate(tenant uuid.UUID) {
	if s.cache != nil {
		s.cache.Invalidate(tenant)
	}
}
