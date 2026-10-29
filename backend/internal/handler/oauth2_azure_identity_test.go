// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// Microsoft identity checks (#1154).

const (
	testTenantID = "0b3a5c1e-2f4d-4e6a-8b9c-0d1e2f3a4b5c"
	testClientID = "client-id"
)

// azureFixture is a stand-in for Microsoft: Graph /me and /organization, and a
// tenant discovery document, behind one test server.
type azureFixture struct {
	server    *httptest.Server
	me        map[string]any
	domains   []string
	discovery int
}

func newAzureFixture(t *testing.T, tenant string) *azureFixture {
	t.Helper()
	f := &azureFixture{
		me:      map[string]any{"id": "oid-1", "displayName": "Member", "mail": "member@operator.example", "userPrincipalName": "member@operator.example"},
		domains: []string{"operator.example", "operator.onmicrosoft.com"},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/me", func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(f.me) })
	mux.HandleFunc("/organization", func(w http.ResponseWriter, _ *http.Request) {
		var vd []map[string]string
		for _, d := range f.domains {
			vd = append(vd, map[string]string{"name": d})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"value": []map[string]any{{"verifiedDomains": vd}}})
	})
	mux.HandleFunc("/operator.example/v2.0/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		f.discovery++
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": f.server.URL + "/" + testTenantID + "/v2.0"})
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)

	prevMe, prevOrg, prevBase, prevTenant, prevConfig := graphMeURL, graphOrganizationURL, azureLoginBase, azureTenant, oauth2Config
	t.Cleanup(func() {
		graphMeURL, graphOrganizationURL, azureLoginBase, azureTenant, oauth2Config = prevMe, prevOrg, prevBase, prevTenant, prevConfig
		azureTenantIDs.Delete("operator.example")
	})
	graphMeURL = f.server.URL + "/me"
	graphOrganizationURL = f.server.URL + "/organization"
	azureLoginBase = f.server.URL
	azureTenant = tenant
	oauth2Config = &OAuth2Config{AzureConfig: &oauth2.Config{ClientID: testClientID}}
	return f
}

// claims returns valid ID token claims for the fixture's member.
func (f *azureFixture) claims() jwt.MapClaims {
	return jwt.MapClaims{
		"iss": f.server.URL + "/" + testTenantID + "/v2.0",
		"aud": testClientID,
		"exp": time.Now().Add(time.Hour).Unix(),
		"tid": testTenantID,
		"oid": "oid-1",
	}
}

func tokenWith(t *testing.T, claims jwt.MapClaims) *oauth2.Token {
	t.Helper()
	// The signature is not what is checked (OIDC Core 3.1.3.7), so any key will do.
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-key"))
	require.NoError(t, err)
	return (&oauth2.Token{AccessToken: "provider-access-token"}).WithExtra(map[string]any{"id_token": raw})
}

func TestAzureUserInfo_MemberOnAVerifiedDomainIsVerified(t *testing.T) {
	f := newAzureFixture(t, testTenantID)

	info, err := getAzureUserInfo(t.Context(), tokenWith(t, f.claims()))

	require.NoError(t, err)
	require.Equal(t, "oid-1", info.ID)
	require.Equal(t, "member@operator.example", info.Email)
	require.True(t, info.EmailVerified)
}

func TestAzureUserInfo_DomainMatchIgnoresCase(t *testing.T) {
	f := newAzureFixture(t, testTenantID)
	f.me["mail"] = "Member@Operator.EXAMPLE"

	info, err := getAzureUserInfo(t.Context(), tokenWith(t, f.claims()))

	require.NoError(t, err)
	require.True(t, info.EmailVerified)
}

func TestAzureUserInfo_AddressOutsideTheVerifiedDomainsIsNotVerified(t *testing.T) {
	// A directory administrator set `mail` to someone else's address.
	f := newAzureFixture(t, testTenantID)
	f.me["mail"] = "ceo@victim.example"

	info, err := getAzureUserInfo(t.Context(), tokenWith(t, f.claims()))

	require.NoError(t, err)
	require.Equal(t, "ceo@victim.example", info.Email)
	require.False(t, info.EmailVerified)
}

func TestAzureUserInfo_SubdomainOfAVerifiedDomainIsNotVerified(t *testing.T) {
	f := newAzureFixture(t, testTenantID)
	f.me["mail"] = "someone@mail.operator.example"

	info, err := getAzureUserInfo(t.Context(), tokenWith(t, f.claims()))

	require.NoError(t, err)
	require.False(t, info.EmailVerified, "only exact verified domains count")
}

func TestAzureUserInfo_SuffixLookalikeIsNotVerified(t *testing.T) {
	f := newAzureFixture(t, testTenantID)
	f.me["mail"] = "someone@evil-operator.example"

	info, err := getAzureUserInfo(t.Context(), tokenWith(t, f.claims()))

	require.NoError(t, err)
	require.False(t, info.EmailVerified)
}

func TestAzureUserInfo_PrefersMailOverUserPrincipalName(t *testing.T) {
	f := newAzureFixture(t, testTenantID)
	f.me["mail"] = "alias@operator.example"
	f.me["userPrincipalName"] = "member@operator.onmicrosoft.com"

	info, err := getAzureUserInfo(t.Context(), tokenWith(t, f.claims()))

	require.NoError(t, err)
	require.Equal(t, "alias@operator.example", info.Email)
	require.True(t, info.EmailVerified)
}

func TestAzureUserInfo_FallsBackToTheUserPrincipalName(t *testing.T) {
	f := newAzureFixture(t, testTenantID)
	f.me["mail"] = ""

	info, err := getAzureUserInfo(t.Context(), tokenWith(t, f.claims()))

	require.NoError(t, err)
	require.Equal(t, "member@operator.example", info.Email)
	require.True(t, info.EmailVerified)
}

func TestAzureUserInfo_GuestAddressIsNotVerified(t *testing.T) {
	f := newAzureFixture(t, testTenantID)
	f.me["mail"] = "member@operator.example" // even on a verified domain
	f.me["userPrincipalName"] = "member_operator.example#EXT#@operator.onmicrosoft.com"

	info, err := getAzureUserInfo(t.Context(), tokenWith(t, f.claims()))

	require.NoError(t, err)
	require.False(t, info.EmailVerified, "a guest's address was typed by the inviter, not verified")
}

func TestAzureUserInfo_RefusesAnInvalidIDToken(t *testing.T) {
	cases := map[string]func(f *azureFixture, c jwt.MapClaims){
		"another directory": func(_ *azureFixture, c jwt.MapClaims) {
			c["tid"] = "11111111-2222-3333-4444-555555555555"
		},
		"another directory, matching issuer": func(f *azureFixture, c jwt.MapClaims) {
			c["tid"] = "11111111-2222-3333-4444-555555555555"
			c["iss"] = f.server.URL + "/11111111-2222-3333-4444-555555555555/v2.0"
		},
		"wrong issuer":        func(_ *azureFixture, c jwt.MapClaims) { c["iss"] = "https://evil.example/" + testTenantID + "/v2.0" },
		"another application": func(_ *azureFixture, c jwt.MapClaims) { c["aud"] = "other-client" },
		"expired":             func(_ *azureFixture, c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() },
		"no expiry":           func(_ *azureFixture, c jwt.MapClaims) { delete(c, "exp") },
		"no user":             func(_ *azureFixture, c jwt.MapClaims) { delete(c, "oid") },
		"another user":        func(_ *azureFixture, c jwt.MapClaims) { c["oid"] = "oid-2" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newAzureFixture(t, testTenantID)
			claims := f.claims()
			mutate(f, claims)

			_, err := getAzureUserInfo(t.Context(), tokenWith(t, claims))

			require.Error(t, err)
		})
	}
}

func TestAzureUserInfo_RefusesAMissingOrMalformedIDToken(t *testing.T) {
	newAzureFixture(t, testTenantID)

	_, err := getAzureUserInfo(t.Context(), &oauth2.Token{AccessToken: "provider-access-token"})
	require.Error(t, err)

	malformed := (&oauth2.Token{AccessToken: "provider-access-token"}).WithExtra(map[string]any{"id_token": "not-a-jwt"})
	_, err = getAzureUserInfo(t.Context(), malformed)
	require.Error(t, err)
}

func TestAzureUserInfo_TenantConfiguredByDomainIsResolvedOnce(t *testing.T) {
	f := newAzureFixture(t, "operator.example")

	for range 2 {
		info, err := getAzureUserInfo(t.Context(), tokenWith(t, f.claims()))
		require.NoError(t, err)
		require.True(t, info.EmailVerified)
	}
	require.Equal(t, 1, f.discovery, "the tenant ID is resolved once, then cached")
}

func TestAzureUserInfo_TenantConfiguredByDomainRefusesAnotherDirectory(t *testing.T) {
	f := newAzureFixture(t, "operator.example")
	claims := f.claims()
	claims["tid"] = "11111111-2222-3333-4444-555555555555"
	claims["iss"] = f.server.URL + "/11111111-2222-3333-4444-555555555555/v2.0"

	_, err := getAzureUserInfo(t.Context(), tokenWith(t, claims))

	require.Error(t, err)
}

func TestOAuthCallback_AzureWithoutAnIDTokenEndsOnUserinfoFailed(t *testing.T) {
	p := newStubProvider(t)
	p.userInfo = map[string]any{"id": "oid-1", "mail": "member@operator.example"}
	app := newOAuthTestApp(t, "azure", p)
	prevTenant := azureTenant
	t.Cleanup(func() { azureTenant = prevTenant })
	azureTenant = testTenantID

	state, _ := startFlow(t, app, "azure")
	code := "auth-code-" + state
	registerChallengeForCode(t, p, "azure", state, code)
	_, params := callbackError(t, app, "azure", "state="+state+"&code="+code)

	require.Equal(t, "userinfo_failed", params.Get("error"))
}
