// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package billing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStripe_CreateCheckout_RealRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/checkout/sessions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer sk_test") {
			t.Errorf("missing bearer auth")
		}
		_ = r.ParseForm()
		if r.Form.Get("line_items[0][price_data][unit_amount]") != "4900" {
			t.Errorf("amount = %s", r.Form.Get("line_items[0][price_data][unit_amount]"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cs_123","url":"https://checkout.stripe.com/pay/cs_123"}`))
	}))
	defer srv.Close()

	g := NewStripeGateway("sk_test_abc")
	g.BaseURL = srv.URL
	sess, err := g.CreateCheckout(context.Background(), CheckoutRequest{
		Plan: "pro", AmountMinor: 4900, Currency: "EUR", Reference: "t1",
		SuccessURL: "https://app/ok", CancelURL: "https://app/no",
	})
	if err != nil {
		t.Fatal(err)
	}
	if sess.Provider != ProviderStripe || !strings.Contains(sess.URL, "checkout.stripe.com") {
		t.Fatalf("bad session %+v", sess)
	}
}

func TestNotchpay_CreateCheckout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "pk_test" {
			t.Errorf("auth = %s", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"transaction":{"authorization_url":"https://pay.notchpay.co/abc"}}`))
	}))
	defer srv.Close()
	g := NewNotchpayGateway("pk_test")
	g.BaseURL = srv.URL
	sess, err := g.CreateCheckout(context.Background(), CheckoutRequest{Plan: "pro", AmountMinor: 12500, Currency: "XAF", Reference: "t2"})
	if err != nil || !strings.Contains(sess.URL, "pay.notchpay.co") {
		t.Fatalf("notchpay session err=%v sess=%+v", err, sess)
	}
}

func TestCinetpay_CreateCheckout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":"201","data":{"payment_url":"https://checkout.cinetpay.com/xyz"}}`))
	}))
	defer srv.Close()
	g := NewCinetpayGateway("api", "site1")
	g.BaseURL = srv.URL
	sess, err := g.CreateCheckout(context.Background(), CheckoutRequest{Plan: "business", AmountMinor: 39000, Currency: "XAF", Reference: "t3"})
	if err != nil || !strings.Contains(sess.URL, "cinetpay.com") {
		t.Fatalf("cinetpay session err=%v sess=%+v", err, sess)
	}
}

func TestGateways_NotConfigured(t *testing.T) {
	for _, g := range []Gateway{NewStripeGateway(""), NewNotchpayGateway(""), NewCinetpayGateway("", "")} {
		if g.Configured() {
			t.Errorf("%s should be unconfigured", g.Provider())
		}
		if _, err := g.CreateCheckout(context.Background(), CheckoutRequest{}); err != ErrNotConfigured {
			t.Errorf("%s: expected ErrNotConfigured, got %v", g.Provider(), err)
		}
	}
}

func TestRegistry_DefaultByCurrency(t *testing.T) {
	stripe := NewStripeGateway("sk")
	notch := NewNotchpayGateway("pk")
	reg := NewRegistry(stripe, notch)

	if reg.Default("EUR").Provider() != ProviderStripe {
		t.Fatal("EUR should default to Stripe")
	}
	if reg.Default("XAF").Provider() != ProviderNotchpay {
		t.Fatal("XAF should default to Notchpay (mobile money)")
	}
	if got := reg.Configured(); len(got) != 2 {
		t.Fatalf("configured = %v", got)
	}
	// Empty registry → no default.
	if NewRegistry().Default("EUR") != nil {
		t.Fatal("empty registry should have no default")
	}
}
