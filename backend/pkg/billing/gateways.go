// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// --- Stripe (international cards) ---------------------------------------------

// StripeGateway creates Stripe Checkout Sessions via the REST API.
type StripeGateway struct {
	SecretKey string
	BaseURL   string // default https://api.stripe.com
	Client    HTTPDoer
}

func NewStripeGateway(secretKey string) *StripeGateway {
	return &StripeGateway{SecretKey: secretKey, BaseURL: "https://api.stripe.com", Client: defaultClient()}
}

func (g *StripeGateway) Provider() Provider { return ProviderStripe }
func (g *StripeGateway) Configured() bool   { return strings.TrimSpace(g.SecretKey) != "" }

func (g *StripeGateway) CreateCheckout(ctx context.Context, req CheckoutRequest) (*CheckoutSession, error) {
	if !g.Configured() {
		return nil, ErrNotConfigured
	}
	// Stripe uses form-encoded bodies. One inline price_data line item.
	form := url.Values{}
	form.Set("mode", "subscription")
	form.Set("success_url", req.SuccessURL)
	form.Set("cancel_url", req.CancelURL)
	form.Set("client_reference_id", req.Reference)
	if req.CustomerEmail != "" {
		form.Set("customer_email", req.CustomerEmail)
	}
	form.Set("line_items[0][quantity]", "1")
	form.Set("line_items[0][price_data][currency]", strings.ToLower(req.Currency))
	form.Set("line_items[0][price_data][recurring][interval]", "month")
	form.Set("line_items[0][price_data][unit_amount]", strconv.Itoa(req.AmountMinor))
	form.Set("line_items[0][price_data][product_data][name]", "OpenRisk "+capitalize(req.Plan))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(g.BaseURL, "/")+"/v1/checkout/sessions", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+g.SecretKey)
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	body, err := doRead(g.Client, httpReq)
	if err != nil {
		return nil, err
	}
	var out struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.URL == "" {
		return nil, fmt.Errorf("stripe: unexpected checkout response")
	}
	return &CheckoutSession{Provider: ProviderStripe, URL: out.URL, Reference: out.ID}, nil
}

// --- Notchpay (MoMo / Orange Money / Wave) -----------------------------------

// NotchpayGateway initialises Notchpay payments via the REST API.
type NotchpayGateway struct {
	PublicKey string
	BaseURL   string // default https://api.notchpay.co
	Client    HTTPDoer
}

func NewNotchpayGateway(publicKey string) *NotchpayGateway {
	return &NotchpayGateway{PublicKey: publicKey, BaseURL: "https://api.notchpay.co", Client: defaultClient()}
}

func (g *NotchpayGateway) Provider() Provider { return ProviderNotchpay }
func (g *NotchpayGateway) Configured() bool   { return strings.TrimSpace(g.PublicKey) != "" }

func (g *NotchpayGateway) CreateCheckout(ctx context.Context, req CheckoutRequest) (*CheckoutSession, error) {
	if !g.Configured() {
		return nil, ErrNotConfigured
	}
	payload := map[string]any{
		"amount":      req.AmountMinor,
		"currency":    req.Currency,
		"email":       req.CustomerEmail,
		"reference":   req.Reference,
		"description": "OpenRisk " + req.Plan,
		"callback":    req.SuccessURL,
	}
	httpReq, err := jsonRequest(ctx, http.MethodPost, strings.TrimRight(g.BaseURL, "/")+"/payments", payload)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", g.PublicKey)

	body, err := doRead(g.Client, httpReq)
	if err != nil {
		return nil, err
	}
	var out struct {
		AuthorizationURL string `json:"authorization_url"`
		Transaction      struct {
			AuthorizationURL string `json:"authorization_url"`
		} `json:"transaction"`
	}
	_ = json.Unmarshal(body, &out)
	u := out.AuthorizationURL
	if u == "" {
		u = out.Transaction.AuthorizationURL
	}
	if u == "" {
		return nil, fmt.Errorf("notchpay: unexpected payment response")
	}
	return &CheckoutSession{Provider: ProviderNotchpay, URL: u, Reference: req.Reference}, nil
}

// --- CinetPay (mobile-money aggregator) --------------------------------------

// CinetpayGateway initialises CinetPay payments via the REST API.
type CinetpayGateway struct {
	APIKey  string
	SiteID  string
	BaseURL string // default https://api-checkout.cinetpay.com
	Client  HTTPDoer
}

func NewCinetpayGateway(apiKey, siteID string) *CinetpayGateway {
	return &CinetpayGateway{APIKey: apiKey, SiteID: siteID, BaseURL: "https://api-checkout.cinetpay.com", Client: defaultClient()}
}

func (g *CinetpayGateway) Provider() Provider { return ProviderCinetpay }
func (g *CinetpayGateway) Configured() bool {
	return strings.TrimSpace(g.APIKey) != "" && strings.TrimSpace(g.SiteID) != ""
}

func (g *CinetpayGateway) CreateCheckout(ctx context.Context, req CheckoutRequest) (*CheckoutSession, error) {
	if !g.Configured() {
		return nil, ErrNotConfigured
	}
	payload := map[string]any{
		"apikey":         g.APIKey,
		"site_id":        g.SiteID,
		"transaction_id": req.Reference,
		"amount":         req.AmountMinor,
		"currency":       req.Currency,
		"description":    "OpenRisk " + req.Plan,
		"return_url":     req.SuccessURL,
		"notify_url":     req.SuccessURL,
		"customer_email": req.CustomerEmail,
	}
	httpReq, err := jsonRequest(ctx, http.MethodPost, strings.TrimRight(g.BaseURL, "/")+"/v2/payment", payload)
	if err != nil {
		return nil, err
	}
	body, err := doRead(g.Client, httpReq)
	if err != nil {
		return nil, err
	}
	var out struct {
		Data struct {
			PaymentURL string `json:"payment_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.Data.PaymentURL == "" {
		return nil, fmt.Errorf("cinetpay: unexpected payment response")
	}
	return &CheckoutSession{Provider: ProviderCinetpay, URL: out.Data.PaymentURL, Reference: req.Reference}, nil
}

// --- shared helpers ----------------------------------------------------------

func jsonRequest(ctx context.Context, method, url string, payload any) (*http.Request, error) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(string(buf)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func doRead(client HTTPDoer, req *http.Request) ([]byte, error) {
	if client == nil {
		client = defaultClient()
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("payment provider returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
