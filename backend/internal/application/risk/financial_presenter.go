// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/pkg/crq"
)

// OrgCurrencyReader resolves a tenant's chosen display currency (from org
// settings). Optional/nil-safe: absent → XAF (the base). GormOrganizationRepository
// satisfies it via its concrete OrgCurrency method.
type OrgCurrencyReader interface {
	OrgCurrency(ctx context.Context, tenantID uuid.UUID) (string, error)
}

// RatesProvider hands out the current FX rate table (with its AsOf date). Optional:
// absent → the built-in static reference table. The daily FX worker implements it.
type RatesProvider interface {
	Current() *crq.RateTable
}

// FinancialPresenterFactory builds a per-request crq.Presenter for a tenant: it
// resolves the tenant's display currency and the live rate table so every figure
// is converted consistently and can show its reference date.
type FinancialPresenterFactory struct {
	q        *crq.Quantifier
	currency OrgCurrencyReader // optional
	rates    RatesProvider     // optional
}

// NewFinancialPresenterFactory builds the factory around the shared quantifier.
func NewFinancialPresenterFactory(q *crq.Quantifier) *FinancialPresenterFactory {
	return &FinancialPresenterFactory{q: q}
}

// WithCurrency attaches the tenant-currency resolver.
func (f *FinancialPresenterFactory) WithCurrency(r OrgCurrencyReader) *FinancialPresenterFactory {
	f.currency = r
	return f
}

// WithRates attaches the live FX rate provider.
func (f *FinancialPresenterFactory) WithRates(r RatesProvider) *FinancialPresenterFactory {
	f.rates = r
	return f
}

// RateTable returns the current rate table (live provider or static fallback).
func (f *FinancialPresenterFactory) RateTable() *crq.RateTable {
	if f != nil && f.rates != nil {
		if t := f.rates.Current(); t != nil {
			return t
		}
	}
	return crq.DefaultRateTable()
}

// CurrencyFor resolves the tenant's display currency (best-effort → XAF).
func (f *FinancialPresenterFactory) CurrencyFor(ctx context.Context, tenantID uuid.UUID) crq.Currency {
	if f != nil && f.currency != nil {
		if code, err := f.currency.OrgCurrency(ctx, tenantID); err == nil {
			return crq.NormalizeCurrency(code)
		}
	}
	return crq.CurrencyXAF
}

// For builds the presenter for a tenant.
func (f *FinancialPresenterFactory) For(ctx context.Context, tenantID uuid.UUID) crq.Presenter {
	xafPerUSD := crq.DefaultXAFPerUSD
	if f != nil && f.q != nil && f.q.XAFPerUSD > 0 {
		xafPerUSD = f.q.XAFPerUSD
	}
	return crq.NewPresenter(f.CurrencyFor(ctx, tenantID), f.RateTable(), xafPerUSD)
}
