// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package handler

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

// SAML sign-in is off until assertions are verified (#866). Every request to
// the two entry points must end on the login screen, with no session.

func newSAMLTestApp(t *testing.T) *fiber.App {
	t.Helper()
	prevBase := oauthAppBaseURL
	t.Cleanup(func() { oauthAppBaseURL = prevBase })
	oauthAppBaseURL = "https://app.test"

	app := fiber.New()
	app.Get("/api/v1/auth/saml2/login", SAML2InitiateLogin)
	app.Post("/api/v1/auth/saml2/acs", SAML2ACS)
	return app
}

func requireSAMLRefused(t *testing.T, resp *http.Response) {
	t.Helper()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	loc, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "https://app.test/login", loc.Scheme+"://"+loc.Host+loc.Path)
	require.Equal(t, "provider_not_configured", loc.Query().Get("error"))
	require.Equal(t, "saml2", loc.Query().Get("provider"))
	for _, ck := range resp.Cookies() {
		require.NotContains(t, []string{"or_access", "or_refresh", "or_csrf"}, ck.Name, "a refused SAML request must not set a session")
	}
}

func postACS(t *testing.T, app *fiber.App, form url.Values) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/saml2/acs", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	return resp
}

func TestSAML2ACS_RefusesAWellFormedSuccessResponse(t *testing.T) {
	app := newSAMLTestApp(t)
	response := `<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="r1" Version="2.0">
  <samlp:Status><samlp:StatusCode Value="urn:oasis:names:tc:SAML:2.0:status:Success"/></samlp:Status>
  <saml:Assertion><saml:Subject><saml:NameID>admin@opendefender.io</saml:NameID></saml:Subject></saml:Assertion>
</samlp:Response>`

	resp := postACS(t, app, url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString([]byte(response))}})

	requireSAMLRefused(t, resp)
}

func TestSAML2ACS_RefusesAnEmptyPost(t *testing.T) {
	requireSAMLRefused(t, postACS(t, newSAMLTestApp(t), url.Values{}))
}

func TestSAML2InitiateLogin_RedirectsToTheLoginScreen(t *testing.T) {
	app := newSAMLTestApp(t)
	t.Setenv("SAML2_IDP_URL", "https://idp.example")
	t.Setenv("SAML2_SP_ENTITY_ID", "openrisk")
	t.Setenv("SAML2_ACS_URL", "https://app.test/api/v1/auth/saml2/acs")

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/auth/saml2/login", nil), -1)
	require.NoError(t, err)

	requireSAMLRefused(t, resp)
}
