// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package netguard

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
)

func TestAddrAllowed(t *testing.T) {
	cases := map[string]bool{
		"8.8.8.8":                true,
		"1.1.1.1":                true,
		"2606:4700::1111":        true,
		"127.0.0.1":              false,
		"127.9.9.9":              false,
		"::1":                    false,
		"0.0.0.0":                false,
		"::":                     false,
		"169.254.169.254":        false, // AWS / GCP / Azure metadata
		"fe80::1":                false,
		"10.1.2.3":               false,
		"172.16.0.1":             false,
		"192.168.1.1":            false,
		"100.100.100.200":        false, // Alibaba metadata (CGNAT)
		"fd00:ec2::254":          false, // AWS IPv6 metadata (ULA)
		"::ffff:169.254.169.254": false, // IPv4-mapped
		"::ffff:10.0.0.1":        false,
		"64:ff9b::a9fe:a9fe":     false, // NAT64 of 169.254.169.254
		"2002:a9fe:a9fe::":       false, // 6to4 of 169.254.169.254
		"255.255.255.255":        false,
		"224.0.0.1":              false,
	}
	var p Policy
	for s, want := range cases {
		if got := p.AddrAllowed(netip.MustParseAddr(s)); got != want {
			t.Errorf("AddrAllowed(%s) = %v, want %v", s, got, want)
		}
	}
}

func TestAddrAllowed_OperatorAllowList(t *testing.T) {
	p := Policy{Allowed: []netip.Prefix{
		netip.MustParsePrefix("10.20.0.0/16"),
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("169.254.0.0/16"),
	}}
	if !p.AddrAllowed(netip.MustParseAddr("10.20.3.4")) {
		t.Error("an allow-listed private range must be reachable")
	}
	if p.AddrAllowed(netip.MustParseAddr("10.21.3.4")) {
		t.Error("a private address outside the allow-list must stay denied")
	}
	// The allow-list can never open loopback or link-local (metadata).
	for _, s := range []string{"127.0.0.1", "169.254.169.254"} {
		if p.AddrAllowed(netip.MustParseAddr(s)) {
			t.Errorf("%s must stay denied whatever the allow-list says", s)
		}
	}
}

func TestPolicyFromEnv(t *testing.T) {
	t.Setenv(AllowedCIDRsEnv, " 10.0.0.0/8 , 192.168.10.0/24 ")
	p, err := PolicyFromEnv()
	if err != nil || len(p.Allowed) != 2 {
		t.Fatalf("PolicyFromEnv = %+v, %v", p, err)
	}
	t.Setenv(AllowedCIDRsEnv, "10.0.0.0/8,not-a-cidr")
	if _, err := PolicyFromEnv(); err == nil {
		t.Fatal("a malformed entry must be an error, not silently ignored")
	}
}

func TestValidateURL(t *testing.T) {
	var p Policy
	ok := []string{
		"https://cloud.tenable.com",
		"https://acme.atlassian.net/",
		"https://qualysapi.qg2.apps.qualys.eu/api/2.0",
		"https://8.8.8.8:8834",
	}
	for _, u := range ok {
		if err := p.ValidateURL(u); err != nil {
			t.Errorf("ValidateURL(%q) = %v, want nil", u, err)
		}
	}
	bad := []string{
		"",
		"cloud.tenable.com",
		"http://cloud.tenable.com",
		"file:///etc/passwd",
		"gopher://127.0.0.1:6379/_x",
		"https://user:pass@cloud.tenable.com",
		"https://169.254.169.254/latest/meta-data/",
		"https://[::ffff:169.254.169.254]/",
		"https://127.0.0.1:6379",
		"https://localhost:8080",
		"https://LOCALHOST.",
		"https://api.localhost",
		"https://10.0.0.5",
		"https://[fd00:ec2::254]/",
	}
	for _, u := range bad {
		err := p.ValidateURL(u)
		if err == nil || !IsDenied(err) {
			t.Errorf("ValidateURL(%q) = %v, want ErrDenied", u, err)
		}
	}
}

// A hostname passes ValidateURL (no DNS there) but resolves to loopback: the
// dialer must refuse the connection. This is the DNS-rebinding path.
func TestClient_RefusesResolvedPrivateAddress(t *testing.T) {
	hit := false
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	target := "https://localhost:" + u.Port() + "/"

	_, err := Policy{}.Client(Options{}).Get(target)
	if err == nil || !IsDenied(err) {
		t.Fatalf("Get(%s) err = %v, want ErrDenied", target, err)
	}
	if hit {
		t.Fatal("the server behind a denied address must never be reached")
	}
}

func TestClient_RedirectIsRevalidated(t *testing.T) {
	c := Policy{}.Client(Options{})
	for _, to := range []string{"http://example.com/", "https://169.254.169.254/"} {
		req, _ := http.NewRequest(http.MethodGet, to, nil)
		if err := c.CheckRedirect(req, []*http.Request{{}}); err == nil || !IsDenied(err) {
			t.Errorf("redirect to %s: err = %v, want ErrDenied", to, err)
		}
	}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/next", nil)
	if err := c.CheckRedirect(req, []*http.Request{{}}); err != nil {
		t.Errorf("redirect to a public https URL: err = %v, want nil", err)
	}
}

func TestClient_NoRedirects(t *testing.T) {
	c := Policy{}.Client(Options{NoRedirects: true})
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/", nil)
	if err := c.CheckRedirect(req, nil); err != http.ErrUseLastResponse {
		t.Fatalf("NoRedirects must stop at the first 3xx, got %v", err)
	}
}

func TestClient_IgnoresEnvironmentProxy(t *testing.T) {
	c := Policy{}.Client(Options{})
	tr, ok := c.Transport.(*http.Transport)
	if !ok || tr.Proxy != nil {
		t.Fatal("the guarded transport must not route through HTTP(S)_PROXY")
	}
	if !strings.Contains(ErrDenied.Error(), "not allowed") {
		t.Fatal("ErrDenied text changed; callers surface it to users")
	}
}
