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

// TestGitLabCollect drives the real gitlab client against an httptest server
// standing in for a self-managed GitLab, asserting projects → assets and public
// projects → exposure findings.
func TestGitLabCollect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v4/projects", r.URL.Path)
		assert.Equal(t, "glpat_test", r.Header.Get("Private-Token"))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Next-Page", "")
		_, _ = w.Write([]byte(`[
			{"id":10,"path_with_namespace":"acme/web","name":"web","visibility":"public","web_url":"https://gl/acme/web","archived":false,"default_branch":"main","namespace":{"full_path":"acme"}},
			{"id":11,"path_with_namespace":"acme/infra","name":"infra","visibility":"private","web_url":"https://gl/acme/infra","archived":false,"namespace":{"full_path":"acme"}}
		]`))
	}))
	defer srv.Close()

	assets := make(chan scanner.AssetDiscovery, 8)
	findings := make(chan scanner.FindingDiscovery, 8)
	errs := make(chan error, 8)

	cfg := scanner.ScanConfig{
		Provider:    domain.ProviderGitLab,
		Credentials: map[string]string{"token": "glpat_test", "base_url": srv.URL},
	}
	GitLab{}.Collect(context.Background(), cfg, assets, findings, errs)
	close(assets)
	close(findings)
	close(errs)

	var gotAssets []scanner.AssetDiscovery
	for a := range assets {
		gotAssets = append(gotAssets, a)
	}
	require.Len(t, gotAssets, 2)
	assert.Equal(t, domain.AssetTypeRepository, gotAssets[0].Type)
	assert.Equal(t, "acme/web", gotAssets[0].Name)

	var gotFindings []scanner.FindingDiscovery
	for f := range findings {
		gotFindings = append(gotFindings, f)
	}
	require.Len(t, gotFindings, 1, "only the public project should raise a finding")
	assert.Equal(t, "gitlab:project:10", gotFindings[0].AssetExternalID)

	for e := range errs {
		t.Fatalf("unexpected error: %v", e)
	}
}
