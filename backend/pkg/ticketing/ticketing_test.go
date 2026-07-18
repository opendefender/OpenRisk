// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package ticketing

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
