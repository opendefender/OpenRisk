// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package billing is the payment-gateway seam. It defines a provider-agnostic
// Gateway interface and REST-backed implementations for Stripe (international
// cards), Notchpay and CinetPay (MoMo / Orange Money / Wave). No SDK dependency —
// each gateway makes documented REST calls with net/http and is httptest-testable,
// the same pattern as pkg/ticketing and pkg/notify.
//
// When a gateway has no API key configured it returns ErrNotConfigured — it never
// fabricates a payment URL. A self-hosted deployment with no keys is a valid state
// (Free plan + manual/sales-invoiced upgrades); the caller degrades honestly.
package billing

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// ErrNotConfigured is returned by a gateway that has no API credentials.
var ErrNotConfigured = errors.New("billing: payment provider not configured")

// Provider identifies a payment gateway.
type Provider string

const (
	ProviderStripe   Provider = "stripe"
	ProviderNotchpay Provider = "notchpay"
	ProviderCinetpay Provider = "cinetpay"
)

// CheckoutRequest is a provider-agnostic request to start a hosted payment.
type CheckoutRequest struct {
	TenantID      string
	Plan          string // pro | business
	AmountMinor   int    // amount in the currency's minor unit (cents) or XAF units
	Currency      string // EUR | XAF
	CustomerEmail string
	Reference     string // idempotency / reconciliation key
	SuccessURL    string
	CancelURL     string
}

// CheckoutSession is a hosted payment URL the caller redirects the user to.
type CheckoutSession struct {
	Provider  Provider `json:"provider"`
	URL       string   `json:"url"`
	Reference string   `json:"reference"`
}

// Gateway is one payment provider.
type Gateway interface {
	Provider() Provider
	// Configured reports whether credentials are present.
	Configured() bool
	// CreateCheckout returns a hosted payment URL, or ErrNotConfigured.
	CreateCheckout(ctx context.Context, req CheckoutRequest) (*CheckoutSession, error)
}

// HTTPDoer is the minimal http client surface (swappable in tests).
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

func defaultClient() HTTPDoer { return &http.Client{Timeout: 15 * time.Second} }

// Registry holds the configured gateways and picks one for a checkout.
type Registry struct {
	gateways map[Provider]Gateway
}

// NewRegistry builds a registry from the given gateways (nil ones ignored).
func NewRegistry(gws ...Gateway) *Registry {
	m := make(map[Provider]Gateway)
	for _, g := range gws {
		if g != nil {
			m[g.Provider()] = g
		}
	}
	return &Registry{gateways: m}
}

// Get returns the named gateway (nil if absent).
func (r *Registry) Get(p Provider) Gateway { return r.gateways[p] }

// Configured lists the providers that have credentials.
func (r *Registry) Configured() []Provider {
	var out []Provider
	for _, p := range []Provider{ProviderStripe, ProviderNotchpay, ProviderCinetpay} {
		if g, ok := r.gateways[p]; ok && g.Configured() {
			out = append(out, p)
		}
	}
	return out
}

// Default returns the best gateway for a currency: XAF → a mobile-money provider
// (Notchpay preferred, then CinetPay), anything else → Stripe. Falls back to any
// configured gateway. Returns nil if none is configured.
func (r *Registry) Default(currency string) Gateway {
	order := []Provider{ProviderStripe, ProviderNotchpay, ProviderCinetpay}
	if currency == "XAF" || currency == "XOF" {
		order = []Provider{ProviderNotchpay, ProviderCinetpay, ProviderStripe}
	}
	for _, p := range order {
		if g, ok := r.gateways[p]; ok && g.Configured() {
			return g
		}
	}
	return nil
}
