// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1

package collectors

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	scanner "github.com/opendefender/openrisk/internal/scanner"
)

// TestM365CollectGraph drives the real Graph pagination/normalisation against an
// httptest Graph endpoint (token acquisition via azidentity is bypassed by
// calling collectGraph directly with a fixed bearer).
func TestM365CollectGraph(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer tok123", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/users"):
			_, _ = w.Write([]byte(`{"value":[
				{"id":"u1","displayName":"Ada Lovelace","userPrincipalName":"ada@corp.io","accountEnabled":true,"userType":"Member"},
				{"id":"u2","displayName":"Guest X","userPrincipalName":"guest@ext.io","accountEnabled":false,"userType":"Guest"}
			]}`))
		case strings.HasPrefix(r.URL.Path, "/devices"):
			_, _ = w.Write([]byte(`{"value":[
				{"id":"d1","displayName":"LAPTOP-01","operatingSystem":"Windows","operatingSystemVersion":"10.0.19045","isManaged":true,"isCompliant":false,"trustType":"AzureAd"}
			]}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	assets := make(chan scanner.AssetDiscovery, 16)
	findings := make(chan scanner.FindingDiscovery, 16)
	errs := make(chan error, 16)

	collectGraph(context.Background(), srv.Client(), srv.URL, "tok123", assets, findings, errs)
	close(assets)
	close(findings)
	close(errs)

	var users, devices int
	for a := range assets {
		switch a.Type {
		case domain.AssetTypeIdentity:
			users++
		case domain.AssetTypeWorkstation:
			devices++
		}
	}
	assert.Equal(t, 2, users)
	assert.Equal(t, 1, devices)

	var gotFindings []scanner.FindingDiscovery
	for f := range findings {
		gotFindings = append(gotFindings, f)
	}
	require.Len(t, gotFindings, 1, "the non-compliant managed device raises a finding")
	assert.Equal(t, "m365:device:d1", gotFindings[0].AssetExternalID)
	assert.Equal(t, scanner.SeverityMedium, gotFindings[0].Severity)

	for e := range errs {
		t.Fatalf("unexpected error: %v", e)
	}
}
