// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package livepull

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opendefender/openrisk/internal/domain"
)

func TestMSDefenderPuller_RealFlow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/token"):
			_ = r.ParseForm()
			if r.Form.Get("grant_type") != "client_credentials" {
				t.Errorf("expected client_credentials grant, got %q", r.Form.Get("grant_type"))
			}
			w.Write([]byte(`{"access_token":"tok-123"}`))
		case strings.HasSuffix(r.URL.Path, "/api/vulnerabilities"):
			if r.Header.Get("Authorization") != "Bearer tok-123" {
				t.Errorf("expected bearer token, got %q", r.Header.Get("Authorization"))
			}
			w.Write([]byte(`{"value":[{"id":"CVE-2021-44228","cvssV3":10,"severity":"Critical"},{"id":"CVE-2020-0001","cvssV3":5}]}`))
		default:
			http.Error(w, "not found", 404)
		}
	}))
	defer srv.Close()

	got, err := (msDefenderPuller{}).Pull(context.Background(), PullConfig{
		BaseURL: srv.URL,
		Credentials: map[string]string{
			"tenant_id": "t", "client_id": "c", "client_secret": "s", "token_url": srv.URL + "/token",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(got))
	}
	if got[0]["id"] != "CVE-2021-44228" {
		t.Errorf("unexpected first finding: %v", got[0])
	}
}

func TestCrowdStrikePuller_RealFlow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/oauth2/token"):
			w.Write([]byte(`{"access_token":"cs-tok"}`))
		case strings.Contains(r.URL.Path, "/spotlight/combined/vulnerabilities"):
			if r.Header.Get("Authorization") != "Bearer cs-tok" {
				t.Errorf("expected bearer, got %q", r.Header.Get("Authorization"))
			}
			w.Write([]byte(`{"resources":[{"id":"v1","cve":{"id":"CVE-2021-1","base_score":9.8}}]}`))
		default:
			http.Error(w, "not found", 404)
		}
	}))
	defer srv.Close()

	got, err := (crowdStrikePuller{}).Pull(context.Background(), PullConfig{
		BaseURL:     srv.URL,
		Credentials: map[string]string{"client_id": "c", "client_secret": "s"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(got))
	}
}

func TestNessusPuller_APIKeyHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("X-ApiKeys"), "accessKey=ak;secretKey=sk") {
			t.Errorf("expected X-ApiKeys header, got %q", r.Header.Get("X-ApiKeys"))
		}
		w.Write([]byte(`{"vulnerabilities":[{"plugin_id":"19506","plugin_name":"SSH","severity":3}]}`))
	}))
	defer srv.Close()

	got, err := (nessusPuller{}).Pull(context.Background(), PullConfig{
		BaseURL:     srv.URL,
		Credentials: map[string]string{"access_key": "ak", "secret_key": "sk"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0]["plugin_id"] != "19506" {
		t.Fatalf("unexpected findings: %v", got)
	}
}

func TestQualysPuller_ParsesXML(t *testing.T) {
	const xmlBody = `<?xml version="1.0"?>
<HOST_LIST_VM_DETECTION_OUTPUT><RESPONSE><HOST_LIST>
  <HOST><IP>10.0.0.1</IP><DNS>web-01</DNS><DETECTION_LIST>
    <DETECTION><QID>38173</QID><SEVERITY>4</SEVERITY><TITLE>TLS</TITLE><RESULTS>weak cipher</RESULTS></DETECTION>
    <DETECTION><QID>91234</QID><SEVERITY>5</SEVERITY></DETECTION>
  </DETECTION_LIST></HOST>
</HOST_LIST></RESPONSE></HOST_LIST_VM_DETECTION_OUTPUT>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Basic ") {
			t.Errorf("expected basic auth, got %q", r.Header.Get("Authorization"))
		}
		w.Write([]byte(xmlBody))
	}))
	defer srv.Close()

	got, err := (qualysPuller{}).Pull(context.Background(), PullConfig{
		BaseURL:     srv.URL,
		Credentials: map[string]string{"username": "u", "password": "p"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 detections, got %d", len(got))
	}
	if got[0]["QID"] != "38173" || got[0]["host"] != "web-01" {
		t.Errorf("unexpected first detection: %v", got[0])
	}
}

func TestPuller_MissingCredential(t *testing.T) {
	_, err := (msDefenderPuller{}).Pull(context.Background(), PullConfig{Credentials: map[string]string{}})
	if err == nil {
		t.Fatal("expected missing-credential error")
	}
}

func TestSeamPuller_HonestSeam(t *testing.T) {
	p, ok := PullerFor(domain.VulnSourceOpenVAS)
	if !ok {
		t.Fatal("expected an OpenVAS entry")
	}
	if p.LivePullSupported() {
		t.Error("OpenVAS live pull should be an honest seam (not supported)")
	}
	if _, err := p.Pull(context.Background(), PullConfig{}); err == nil {
		t.Error("seam puller should return an explanatory error, not fabricated data")
	}
	if !LivePullSupported(domain.VulnSourceMSDefender) {
		t.Error("MS Defender should report live pull supported")
	}
}
