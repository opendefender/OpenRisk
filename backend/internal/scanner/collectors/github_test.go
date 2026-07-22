// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package collectors

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	scanner "github.com/opendefender/openrisk/internal/scanner"
)

// TestGitHubCollect drives the real go-github client against an httptest server
// standing in for a GitHub Enterprise API, and asserts repos become assets and
// public repos become exposure findings.
func TestGitHubCollect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/user/repos", r.URL.Path)
		assert.Equal(t, "Bearer ghp_test", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"node_id":"R_1","full_name":"acme/api","private":false,"visibility":"public","html_url":"https://gh/acme/api","language":"Go","archived":false,"default_branch":"main","owner":{"login":"acme"}},
			{"node_id":"R_2","full_name":"acme/secrets","private":true,"visibility":"private","html_url":"https://gh/acme/secrets","language":"Go","archived":false,"owner":{"login":"acme"}}
		]`))
	}))
	defer srv.Close()

	assets := make(chan scanner.AssetDiscovery, 8)
	findings := make(chan scanner.FindingDiscovery, 8)
	errs := make(chan error, 8)

	cfg := scanner.ScanConfig{
		Provider:    domain.ProviderGitHub,
		Credentials: map[string]string{"token": "ghp_test", "base_url": srv.URL + "/"},
	}
	GitHub{}.Collect(context.Background(), cfg, assets, findings, errs)
	close(assets)
	close(findings)
	close(errs)

	var gotAssets []scanner.AssetDiscovery
	for a := range assets {
		gotAssets = append(gotAssets, a)
	}
	require.Len(t, gotAssets, 2)
	assert.Equal(t, domain.AssetTypeRepository, gotAssets[0].Type)
	assert.Equal(t, "acme/api", gotAssets[0].Name)
	assert.Contains(t, gotAssets[0].Tags, "public")

	var gotFindings []scanner.FindingDiscovery
	for f := range findings {
		gotFindings = append(gotFindings, f)
	}
	require.Len(t, gotFindings, 1, "only the public repo should raise a finding")
	assert.Equal(t, "R_1", gotFindings[0].AssetExternalID)
	assert.Equal(t, scanner.SeverityLow, gotFindings[0].Severity)

	for e := range errs {
		t.Fatalf("unexpected error: %v", e)
	}
}
