// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"time"

	"github.com/google/uuid"
)

// New canonical plan constants aligned with the open-core matrix
// (pkg/entitlements). The legacy OrgPlan values (free/starter/professional/
// enterprise) still parse — see entitlements.ParsePlan — but new writes use these.
const (
	PlanPro      OrgPlan = "pro"
	PlanBusiness OrgPlan = "business"
)

// SubscriptionStatus is the lifecycle state of a paid subscription.
type SubscriptionStatus string

const (
	SubTrialing   SubscriptionStatus = "trialing"
	SubActive     SubscriptionStatus = "active"
	SubPastDue    SubscriptionStatus = "past_due"
	SubCanceled   SubscriptionStatus = "canceled"
	SubIncomplete SubscriptionStatus = "incomplete"
)

// BillingProvider identifies which payment processor backs the subscription.
type BillingProvider string

const (
	ProviderStripe   BillingProvider = "stripe"   // international cards
	ProviderNotchpay BillingProvider = "notchpay" // MoMo / Orange Money / Wave (CEMAC/UEMOA)
	ProviderCinetpay BillingProvider = "cinetpay" // mobile money aggregator
	ProviderManual   BillingProvider = "manual"   // sales-invoiced / self-hosted
)

// Subscription is one paid (or trialing) subscription per organization. The
// effective plan an org gets is derived from this row when it is entitled
// (trialing/active); otherwise the org falls back to Free.
type Subscription struct {
	ID             uuid.UUID          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrganizationID uuid.UUID          `gorm:"type:uuid;uniqueIndex;not null" json:"organization_id"`
	Plan           string             `gorm:"not null;default:'free'" json:"plan"`
	Status         SubscriptionStatus `gorm:"not null;default:'active'" json:"status"`
	Region         string             `gorm:"not null;default:'eu'" json:"region"`
	Provider       BillingProvider    `json:"provider,omitempty"`

	// Provider linkage — opaque ids returned by Stripe/Notchpay/CinetPay. Never
	// secrets; safe to expose for reconciliation.
	ProviderCustomerID     string `json:"provider_customer_id,omitempty"`
	ProviderSubscriptionID string `json:"provider_subscription_id,omitempty"`

	TrialEndsAt       *time.Time `json:"trial_ends_at,omitempty"`
	CurrentPeriodEnd  *time.Time `json:"current_period_end,omitempty"`
	CancelAtPeriodEnd bool       `gorm:"default:false" json:"cancel_at_period_end"`
	CanceledAt        *time.Time `json:"canceled_at,omitempty"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Subscription) TableName() string { return "subscriptions" }

// Entitled reports whether the subscription currently grants its plan (trialing
// with time left, or active). A canceled/past-due/incomplete subscription is not
// entitled and the org falls back to Free.
func (s *Subscription) Entitled(now time.Time) bool {
	switch s.Status {
	case SubActive:
		return true
	case SubTrialing:
		return s.TrialEndsAt == nil || s.TrialEndsAt.After(now)
	default:
		return false
	}
}

// TrialActive reports whether the org is in an unexpired trial window.
func (s *Subscription) TrialActive(now time.Time) bool {
	return s.Status == SubTrialing && s.TrialEndsAt != nil && s.TrialEndsAt.After(now)
}

// Invoice is a billing document, mirrored locally from the provider so the
// billing page can render history without a live provider round-trip.
type Invoice struct {
	ID             uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrganizationID uuid.UUID       `gorm:"type:uuid;index;not null" json:"organization_id"`
	Number         string          `json:"number"`
	AmountCents    int             `json:"amount_cents"`
	Currency       string          `json:"currency"`
	Status         string          `json:"status"` // paid | open | void | uncollectible
	Provider       BillingProvider `json:"provider,omitempty"`
	ProviderID     string          `json:"provider_id,omitempty"`
	HostedURL      string          `json:"hosted_url,omitempty"`
	PeriodStart    *time.Time      `json:"period_start,omitempty"`
	PeriodEnd      *time.Time      `json:"period_end,omitempty"`
	CreatedAt      time.Time       `gorm:"autoCreateTime" json:"created_at"`
}

func (Invoice) TableName() string { return "invoices" }
