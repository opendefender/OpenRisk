// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package ticketing

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opendefender/openrisk/pkg/netguard"
)

func TestJiraProvider_CreatesIssue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/rest/api/2/issue") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Basic ") {
			t.Error("expected basic auth")
		}
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		_ = json.Unmarshal(body, &parsed)
		fields, _ := parsed["fields"].(map[string]any)
		if proj, _ := fields["project"].(map[string]any); proj["key"] != "SEC" {
			t.Errorf("expected project SEC, got %v", fields["project"])
		}
		w.WriteHeader(201)
		w.Write([]byte(`{"id":"1001","key":"SEC-42","self":"..."}`))
	}))
	defer srv.Close()

	tk, err := (jiraProvider{}).Create(context.Background(), CreateRequest{
		HTTP:           srv.Client(),
		BaseURL:        srv.URL,
		Credentials:    map[string]string{"email": "a@b.co", "api_token": "tok"},
		ProjectOrTable: "SEC",
		Summary:        "[CVE-2021-44228] Log4Shell",
		Description:    "desc",
		Priority:       "critical",
		Labels:         []string{"vuln", "cisa kev"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tk.Key != "SEC-42" {
		t.Errorf("expected key SEC-42, got %q", tk.Key)
	}
	if !strings.HasSuffix(tk.URL, "/browse/SEC-42") {
		t.Errorf("unexpected URL %q", tk.URL)
	}
}

func TestServiceNowProvider_CreatesIncident(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/api/now/table/incident") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(201)
		w.Write([]byte(`{"result":{"number":"INC0012345","sys_id":"abc123"}}`))
	}))
	defer srv.Close()

	tk, err := (serviceNowProvider{}).Create(context.Background(), CreateRequest{
		HTTP:        srv.Client(),
		BaseURL:     srv.URL,
		Credentials: map[string]string{"username": "u", "password": "p"},
		Summary:     "[CVE-2021-44228] Log4Shell",
		Description: "desc",
		Priority:    "critical",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tk.Key != "INC0012345" {
		t.Errorf("expected INC0012345, got %q", tk.Key)
	}
	if !strings.Contains(tk.URL, "sys_id=abc123") {
		t.Errorf("expected sys_id in URL, got %q", tk.URL)
	}
}

func TestProvider_AuthErrorNotFabricated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errorMessages":["Unauthorized"]}`, 401)
	}))
	defer srv.Close()

	_, err := (jiraProvider{}).Create(context.Background(), CreateRequest{
		HTTP:           srv.Client(),
		BaseURL:        srv.URL,
		Credentials:    map[string]string{"email": "a@b.co", "api_token": "bad"},
		ProjectOrTable: "SEC",
		Summary:        "x",
	})
	if err == nil {
		t.Fatal("expected a real auth error, not a fabricated ticket")
	}
}

func TestProviderFor(t *testing.T) {
	if _, ok := ProviderFor(ProviderJira); !ok {
		t.Error("expected jira provider")
	}
	if _, ok := ProviderFor(ProviderServiceNow); !ok {
		t.Error("expected servicenow provider")
	}
	if _, ok := ProviderFor("unknown"); ok {
		t.Error("expected no provider for unknown")
	}
}

func TestServiceNowPriorityMap(t *testing.T) {
	cases := map[string]string{"critical": "1", "high": "2", "medium": "3", "low": "4", "": "4"}
	for in, want := range cases {
		if got := snPriority(in); got != want {
			t.Errorf("snPriority(%q)=%q want %q", in, got, want)
		}
	}
}

// Without an injected client, a private instance URL is refused before any
// request — the ticketing credentials never leave for an internal host.
func TestProvider_RefusesPrivateBaseURL(t *testing.T) {
	for _, p := range []Provider{jiraProvider{}, serviceNowProvider{}} {
		for _, base := range []string{"https://169.254.169.254", "http://acme.atlassian.net", "https://10.0.0.8"} {
			_, err := p.Create(context.Background(), CreateRequest{
				BaseURL:        base,
				Credentials:    map[string]string{"email": "a@b.co", "username": "u", "api_token": "t", "password": "p"},
				ProjectOrTable: "SEC",
				Summary:        "x",
			})
			if err == nil || !netguard.IsDenied(err) {
				t.Errorf("%s %s: err = %v, want netguard.ErrDenied", p.Name(), base, err)
			}
		}
	}
}

func TestProviders_DoNotEchoResponseBody(t *testing.T) {
	const secret = "internal-admin-panel-token"
	for _, status := range []int{http.StatusUnauthorized, http.StatusOK} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			w.Write([]byte(`{"detail":"` + secret + `"}`))
		}))
		for _, p := range []Provider{jiraProvider{}, serviceNowProvider{}} {
			_, err := p.Create(context.Background(), CreateRequest{
				HTTP:           srv.Client(),
				BaseURL:        srv.URL,
				Credentials:    map[string]string{"email": "a@b.co", "username": "u", "api_token": "t", "password": "p"},
				ProjectOrTable: "SEC",
				Summary:        "x",
			})
			if err == nil {
				t.Errorf("%s status %d: expected an error", p.Name(), status)
			} else if strings.Contains(err.Error(), secret) {
				t.Errorf("%s status %d: error echoes the remote body: %v", p.Name(), status, err)
			}
		}
		srv.Close()
	}
}

func TestServiceNowProvider_RejectsTablePath(t *testing.T) {
	_, err := (serviceNowProvider{}).Create(context.Background(), CreateRequest{
		HTTP:           http.DefaultClient,
		BaseURL:        "https://acme.service-now.com",
		Credentials:    map[string]string{"username": "u", "password": "p"},
		ProjectOrTable: "../../../api/now/v1/users",
		Summary:        "x",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid table name") {
		t.Fatalf("err = %v, want invalid table name", err)
	}
}
