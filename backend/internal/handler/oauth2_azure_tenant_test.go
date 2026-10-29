// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// Microsoft sign-in is pinned to the operator's directory (#1153).

func TestAzureConfig_GenericTenantLeavesMicrosoftSignInUnconfigured(t *testing.T) {
	for _, tenant := range []string{"", "  ", "common", "Common", "organizations", "consumers", "9188040D-6C67-4C5B-B112-36A304B66DAD"} {
		t.Run("tenant="+tenant, func(t *testing.T) {
			t.Setenv("OAUTH2_AZURE_CLIENT_ID", "client-id")
			t.Setenv("OAUTH2_AZURE_TENANT_ID", tenant)

			cfg := azureConfig("https://app.test/api/v1/auth/oauth2/callback")

			require.Empty(t, cfg.ClientID, "a generic tenant must leave Microsoft sign-in unconfigured")
		})
	}
}

func TestAzureConfig_SpecificTenantPinsTheEndpoints(t *testing.T) {
	for _, tenant := range []string{"0b3a5c1e-2f4d-4e6a-8b9c-0d1e2f3a4b5c", "contoso.onmicrosoft.com"} {
		t.Run(tenant, func(t *testing.T) {
			t.Setenv("OAUTH2_AZURE_CLIENT_ID", "client-id")
			t.Setenv("OAUTH2_AZURE_TENANT_ID", tenant)

			cfg := azureConfig("https://app.test/api/v1/auth/oauth2/callback")

			require.Equal(t, "client-id", cfg.ClientID)
			require.True(t, strings.HasPrefix(cfg.Endpoint.AuthURL, "https://login.microsoftonline.com/"+tenant+"/"), cfg.Endpoint.AuthURL)
			require.True(t, strings.HasPrefix(cfg.Endpoint.TokenURL, "https://login.microsoftonline.com/"+tenant+"/"), cfg.Endpoint.TokenURL)
			require.Equal(t, "https://app.test/api/v1/auth/oauth2/callback/azure", cfg.RedirectURL)
		})
	}
}

func TestAzureConfig_NoClientIDStaysUnconfigured(t *testing.T) {
	t.Setenv("OAUTH2_AZURE_CLIENT_ID", "")
	t.Setenv("OAUTH2_AZURE_TENANT_ID", "contoso.onmicrosoft.com")

	require.Empty(t, azureConfig("https://app.test/cb").ClientID)
}

func TestOAuthLogin_MicrosoftWithoutATenantSaysNotConfigured(t *testing.T) {
	p := newStubProvider(t)
	app := newOAuthTestApp(t, "azure", p)
	t.Setenv("OAUTH2_AZURE_CLIENT_ID", "client-id")
	t.Setenv("OAUTH2_AZURE_TENANT_ID", "common")
	oauth2Config.AzureConfig = azureConfig("https://app.test/api/v1/auth/oauth2/callback")

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth2/login/azure", nil), -1)
	require.NoError(t, err)

	require.Equal(t, http.StatusFound, resp.StatusCode)
	loc, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "/login", loc.Path)
	require.Equal(t, "provider_not_configured", loc.Query().Get("error"))
	require.Equal(t, "azure", loc.Query().Get("provider"))
	require.NotContains(t, resp.Header.Get("Location"), "login.microsoftonline.com")
}

func TestInitializeOAuth2_GoogleAndGitHubAreUnaffectedByTheAzureTenant(t *testing.T) {
	prevConfig, prevState := oauth2Config, oauthStateService
	t.Cleanup(func() { oauth2Config, oauthStateService = prevConfig, prevState })
	t.Setenv("OAUTH2_GOOGLE_CLIENT_ID", "google-id")
	t.Setenv("OAUTH2_GITHUB_CLIENT_ID", "github-id")
	t.Setenv("OAUTH2_AZURE_CLIENT_ID", "azure-id")
	t.Setenv("OAUTH2_AZURE_TENANT_ID", "")

	cfg := InitializeOAuth2()

	require.Equal(t, "google-id", cfg.GoogleConfig.ClientID)
	require.Equal(t, "github-id", cfg.GitHubConfig.ClientID)
	require.Empty(t, cfg.AzureConfig.ClientID)
}

func TestAzureUserInfo_GuestAddressIsNotVerified(t *testing.T) {
	prev := graphMeURL
	t.Cleanup(func() { graphMeURL = prev })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"oid-1","displayName":"Guest","mail":"ceo@victim.example","userPrincipalName":"ceo_victim.example#EXT#@operator.onmicrosoft.com"}`))
	}))
	t.Cleanup(srv.Close)
	graphMeURL = srv.URL

	info, err := getAzureUserInfo(t.Context(), &oauth2.Token{AccessToken: "token"})
	require.NoError(t, err)

	require.Equal(t, "ceo@victim.example", info.Email)
	require.False(t, info.EmailVerified, "a guest's address was typed by the inviter, not verified")
}

func TestAzureUserInfo_MemberAddressIsVerified(t *testing.T) {
	prev := graphMeURL
	t.Cleanup(func() { graphMeURL = prev })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"oid-2","displayName":"Member","mail":"","userPrincipalName":"member@operator.example"}`))
	}))
	t.Cleanup(srv.Close)
	graphMeURL = srv.URL

	info, err := getAzureUserInfo(t.Context(), &oauth2.Token{AccessToken: "token"})
	require.NoError(t, err)

	require.Equal(t, "member@operator.example", info.Email)
	require.True(t, info.EmailVerified)
}
