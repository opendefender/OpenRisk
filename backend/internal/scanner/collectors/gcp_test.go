// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package collectors

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// gcpKey builds a syntactically valid service-account key whose token_uri is
// the given URL.
func gcpKey(t *testing.T, tokenURI string) string {
	t.Helper()
	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(pk)
	require.NoError(t, err)
	b, err := json.Marshal(map[string]string{
		"type":           "service_account",
		"project_id":     "p",
		"private_key_id": "k",
		"private_key":    string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"client_email":   "scanner@p.iam.gserviceaccount.com",
		"token_uri":      tokenURI,
	})
	require.NoError(t, err)
	return string(b)
}

// externalAccountKey is the credential of GHSA-jxfm-p72g-w272: the Google auth
// library GETs credential_source.url and POSTs to token_url, both chosen by
// whoever wrote the JSON.
func externalAccountKey(t *testing.T, base string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"type":               "external_account",
		"audience":           "//iam.googleapis.com/projects/1/locations/global/workloadIdentityPools/p/providers/p",
		"subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
		"token_url":          base + "/token",
		"credential_source":  map[string]string{"url": base + "/metadata"},
	})
	require.NoError(t, err)
	return string(b)
}

// #1315, GHSA-jxfm-p72g-w272: a token_uri other than Google's is refused
// before the SDK is built, so the server behind it is never reached.
func TestGCPCollect_RefusesForeignTokenURI(t *testing.T) {
	srv, hits := loopbackServer(t, true)
	u, _ := url.Parse(srv.URL)
	errs := collectErrs(t, GCP{}, domain.ProviderGCP, map[string]string{
		"service_account_json": gcpKey(t, "https://localhost:"+u.Port()+"/token"),
		"project_id":           "p",
	})
	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Error(), "token_uri must be Google's token endpoint")
	assert.NotContains(t, errs[0].Error(), u.Port(), "the error must not echo the key's token_uri")
	assert.Zero(t, hits.Load(), "the token_uri must never be reached")
}

// GHSA-jxfm-p72g-w272: an external_account key is refused, so neither its
// credential_source.url nor its token_url is fetched.
func TestGCPCollect_RefusesExternalAccount(t *testing.T) {
	srv, hits := loopbackServer(t, false)
	errs := collectErrs(t, GCP{}, domain.ProviderGCP, map[string]string{
		"service_account_json": externalAccountKey(t, srv.URL),
		"project_id":           "p",
	})
	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Error(), `must be a service account key`)
	assert.Zero(t, hits.Load(), "credential_source.url and token_url must never be fetched")
}

// GHSA-jxfm-p72g-w272: an empty key (a config stored before the fix) must not
// fall back to Application Default Credentials, the API pod's own identity.
// ADC is pointed at a credential that calls a counting server, so a fallback
// would show as a hit.
func TestGCPCollect_RefusesEmptyKey(t *testing.T) {
	srv, hits := loopbackServer(t, false)
	adc := filepath.Join(t.TempDir(), "adc.json")
	require.NoError(t, os.WriteFile(adc, []byte(externalAccountKey(t, srv.URL)), 0o600))
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", adc)

	for _, key := range []string{"", "  "} {
		errs := collectErrs(t, GCP{}, domain.ProviderGCP, map[string]string{
			"service_account_json": key,
			"project_id":           "p",
		})
		require.NotEmpty(t, errs, "%q", key)
		assert.Contains(t, errs[0].Error(), `"service_account_json" is empty`)
	}
	assert.Zero(t, hits.Load(), "the collector must never authenticate with the server's own identity")
}
