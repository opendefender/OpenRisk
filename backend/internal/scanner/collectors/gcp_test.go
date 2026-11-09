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
	"testing"

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

// #1315: the token_uri of a tenant's key is where the SDK POSTs the signed
// assertion. "localhost" passes a literal check, so this is the DNS path a
// stored key or a rebinding name would take: the guarded dialer must stop it
// before the server behind it is reached.
func TestGCPCollect_RefusesLoopbackTokenURI(t *testing.T) {
	srv, hits := loopbackServer(t, true)
	u, _ := url.Parse(srv.URL)
	errs := collectErrs(t, GCP{}, domain.ProviderGCP, map[string]string{
		"service_account_json": gcpKey(t, "https://localhost:"+u.Port()+"/token"),
		"project_id":           "p",
	})
	requireDenied(t, errs, hits)
}
