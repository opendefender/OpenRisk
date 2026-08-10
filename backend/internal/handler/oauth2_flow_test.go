// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"

	"github.com/opendefender/openrisk/internal/service"
	"github.com/opendefender/openrisk/pkg/oauthpkce"
)

// ---------------------------------------------------------------------------
// Stub provider
// ---------------------------------------------------------------------------

// stubProvider is a stand-in authorization server.
//
// It exists so the three provider flows can be exercised for real — authorize →
// token exchange → user info — without credentials for Google, GitHub or Entra.
// Crucially it ENFORCES PKCE: the token endpoint recomputes the challenge from
// the presented verifier and refuses a mismatch, exactly as a real provider does.
// A test against a stub that ignored PKCE would pass even if we stopped sending
// the verifier.
type stubProvider struct {
	server *httptest.Server

	// challengeFor records the challenge presented at /authorize, keyed by code.
	challengeFor map[string]string
	// lastAuthQuery is what the client sent to /authorize.
	lastAuthQuery url.Values
	// tokenCalls counts successful exchanges.
	tokenCalls int
	// rejectVerifier makes the token endpoint refuse regardless, to prove the
	// handler surfaces an exchange failure rather than crashing.
	rejectVerifier bool

	userInfo   map[string]any
	emailsBody any
}

func newStubProvider(t *testing.T) *stubProvider {
	t.Helper()
	p := &stubProvider{challengeFor: map[string]string{}}

	mux := http.NewServeMux()

	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		p.lastAuthQuery = r.URL.Query()
		code := "auth-code-" + r.URL.Query().Get("state")
		p.challengeFor[code] = r.URL.Query().Get("code_challenge")
		http.Redirect(w, r, "/done?code="+code, http.StatusFound)
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		code := r.Form.Get("code")
		verifier := r.Form.Get("code_verifier")

		// This is the PKCE check a real provider performs.
		if p.rejectVerifier || verifier == "" || !oauthpkce.Verify(verifier, p.challengeFor[code]) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}

		p.tokenCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"provider-access-token","token_type":"Bearer","expires_in":3600}`))
	})

	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Authorization"), "provider-access-token") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(p.userInfo)
	})

	mux.HandleFunc("/emails", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Authorization"), "provider-access-token") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(p.emailsBody)
	})

	p.server = httptest.NewServer(mux)
	t.Cleanup(p.server.Close)
	return p
}

func (p *stubProvider) oauthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:8080/api/v1/auth/oauth2/callback",
		Endpoint: oauth2.Endpoint{
			AuthURL:  p.server.URL + "/authorize",
			TokenURL: p.server.URL + "/token",
		},
	}
}

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

// newOAuthTestApp builds a Fiber app with the two OAuth routes mounted and the
// package globals pointed at a stub provider. It restores them afterwards.
func newOAuthTestApp(t *testing.T, provider string, p *stubProvider) *fiber.App {
	t.Helper()

	prevConfig, prevState, prevBase := oauth2Config, oauthStateService, oauthAppBaseURL
	prevGoogle, prevGHUser, prevGHEmails, prevGraph := googleUserInfoURL, githubUserURL, githubEmailsURL, graphMeURL
	t.Cleanup(func() {
		oauth2Config, oauthStateService, oauthAppBaseURL = prevConfig, prevState, prevBase
		googleUserInfoURL, githubUserURL, githubEmailsURL, graphMeURL = prevGoogle, prevGHUser, prevGHEmails, prevGraph
	})

	cfg := p.oauthConfig()
	oauth2Config = &OAuth2Config{}
	switch provider {
	case "google":
		oauth2Config.GoogleConfig = cfg
		googleUserInfoURL = p.server.URL + "/userinfo"
	case "github":
		oauth2Config.GitHubConfig = cfg
		githubUserURL = p.server.URL + "/userinfo"
		githubEmailsURL = p.server.URL + "/emails"
	case "azure":
		oauth2Config.AzureConfig = cfg
		graphMeURL = p.server.URL + "/userinfo"
	}
	oauthStateService = service.NewOAuthStateService()
	oauthAppBaseURL = "https://app.test"

	app := fiber.New()
	app.Get("/api/v1/auth/oauth2/login/:provider", OAuth2Login)
	app.Get("/api/v1/auth/oauth2/callback/:provider", OAuth2Callback)
	return app
}

// startFlow drives /login and returns the state the handler minted.
func startFlow(t *testing.T, app *fiber.App, provider string) (state string, authQuery url.Values) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth2/login/"+provider, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected a 302 to the provider, got %d", resp.StatusCode)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	q := loc.Query()
	return q.Get("state"), q
}

// callbackError drives /callback and returns the error code on the SPA redirect.
func callbackError(t *testing.T, app *fiber.App, provider, query string) (status int, params url.Values) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth2/callback/"+provider+"?"+query, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := url.Parse(resp.Header.Get("Location"))
	if loc == nil {
		return resp.StatusCode, url.Values{}
	}
	return resp.StatusCode, loc.Query()
}

// ---------------------------------------------------------------------------
// Per-provider flows
// ---------------------------------------------------------------------------

func TestOAuthLogin_SendsPKCEChallengeAndState_AllProviders(t *testing.T) {
	// Both CSRF defences must be present on the authorization request for every
	// provider: state binds the callback to this browser, PKCE binds the code to
	// this server.
	for _, provider := range []string{"google", "github", "azure"} {
		t.Run(provider, func(t *testing.T) {
			p := newStubProvider(t)
			app := newOAuthTestApp(t, provider, p)

			state, q := startFlow(t, app, provider)

			if state == "" {
				t.Error("no state on the authorization request — the callback could not be bound to this browser")
			}
			challenge := q.Get("code_challenge")
			if challenge == "" {
				t.Fatal("no code_challenge — an intercepted authorization code would be redeemable")
			}
			if q.Get("code_challenge_method") != "S256" {
				t.Errorf("expected S256, got %q (plain offers no protection)", q.Get("code_challenge_method"))
			}
			// The verifier itself must NOT travel through the browser.
			for key, values := range q {
				for _, v := range values {
					if key != "code_challenge" && oauthpkce.Verify(v, challenge) {
						t.Errorf("the PKCE verifier leaked into the authorization URL as %q", key)
					}
				}
			}
		})
	}
}

func TestOAuthCallback_GoogleFlow_VerifiedEmailReachesTheResolver(t *testing.T) {
	p := newStubProvider(t)
	p.userInfo = map[string]any{
		"id": "google-sub-1", "email": "member@opendefender.io",
		"verified_email": true, "name": "Alex Dembele", "picture": "https://x/y.png",
	}
	app := newOAuthTestApp(t, "google", p)

	state, _ := startFlow(t, app, "google")
	code := "auth-code-" + state
	// Register the challenge against the code, standing in for the provider's own
	// /authorize hop which this test skips.
	registerChallengeForCode(t, p, "google", state, code)

	// No resolver is wired in this test, so the flow reaches resolution and stops
	// there — which is exactly the boundary this test is about: everything up to
	// and including PKCE exchange + user info must have succeeded.
	_, params := callbackError(t, app, "google", "state="+state+"&code="+code)

	if p.tokenCalls != 1 {
		t.Fatalf("expected exactly one PKCE-validated token exchange, got %d", p.tokenCalls)
	}
	if got := params.Get("error"); got != "internal" {
		t.Fatalf("expected the flow to reach identity resolution (error=internal with no resolver wired), got %q", got)
	}
}

// registerChallengeForCode teaches the stub which challenge belongs to a code.
//
// The handler stored the verifier against `state`; the stub's token endpoint
// looks challenges up by `code`. Recover the verifier from the state service and
// put the flow straight back, since ConsumeFlow is single-use and the callback
// under test still needs it.
func registerChallengeForCode(t *testing.T, p *stubProvider, provider, state, code string) {
	t.Helper()
	flow, err := oauthStateService.ConsumeFlow(state, provider)
	if err != nil {
		t.Fatalf("expected a live flow for state %s: %v", state, err)
	}
	p.challengeFor[code] = oauthpkce.Challenge(flow.CodeVerifier)
	oauthStateService.StoreFlow(flow, oauthStateTTL)
}

func TestOAuthCallback_GitHubFlow_UsesTheVerifiedPrimaryEmail(t *testing.T) {
	// GitHub's /user email carries no verification guarantee; /user/emails is the
	// only source with the verified flag, so it must be preferred.
	p := newStubProvider(t)
	p.userInfo = map[string]any{
		"id": 4242, "login": "adembele", "name": "Alex Dembele",
		"email": "public-unverified@example.com", "avatar_url": "https://x/y.png",
	}
	p.emailsBody = []map[string]any{
		{"email": "secondary@opendefender.io", "primary": false, "verified": true},
		{"email": "primary@opendefender.io", "primary": true, "verified": true},
	}

	_ = newOAuthTestApp(t, "github", p)

	got, err := getGitHubUserInfo(t.Context(), &oauth2.Token{AccessToken: "provider-access-token"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Email != "primary@opendefender.io" {
		t.Errorf("expected the verified PRIMARY address, got %q", got.Email)
	}
	if !got.EmailVerified {
		t.Error("expected EmailVerified from the /user/emails verified flag")
	}
	if got.ID != "4242" {
		t.Errorf("expected the numeric subject stringified, got %q", got.ID)
	}
}

func TestGitHubUserInfo_FallsBackToUnverifiedPublicEmail(t *testing.T) {
	// When /user/emails yields nothing verified, the public address may still be
	// reported — but never as verified, so it cannot reach an existing account.
	p := newStubProvider(t)
	p.userInfo = map[string]any{
		"id": 7, "login": "ghost", "email": "public@example.com",
	}
	p.emailsBody = []map[string]any{
		{"email": "unconfirmed@example.com", "primary": true, "verified": false},
	}
	_ = newOAuthTestApp(t, "github", p)

	got, err := getGitHubUserInfo(t.Context(), &oauth2.Token{AccessToken: "provider-access-token"})
	if err != nil {
		t.Fatal(err)
	}
	if got.EmailVerified {
		t.Error("an unverified address must never be reported as verified")
	}
}

func TestGoogleUserInfo_MissingVerifiedFlagIsNotVerified(t *testing.T) {
	// Defaulting an absent verified_email to true would make a provider hiccup
	// equivalent to a vouched-for address.
	p := newStubProvider(t)
	p.userInfo = map[string]any{"id": "g1", "email": "someone@opendefender.io", "name": "N"}
	_ = newOAuthTestApp(t, "google", p)

	got, err := getGoogleUserInfo(t.Context(), &oauth2.Token{AccessToken: "provider-access-token"})
	if err != nil {
		t.Fatal(err)
	}
	if got.EmailVerified {
		t.Error("absent verified_email must read as NOT verified")
	}
}

func TestGoogleUserInfo_MalformedPayloadDoesNotPanic(t *testing.T) {
	// The previous implementation did data["id"].(string) — an unchecked type
	// assertion that panicked on an unexpected shape, taking the process down.
	p := newStubProvider(t)
	p.userInfo = map[string]any{"id": 12345, "email": nil, "name": []string{"weird"}}
	_ = newOAuthTestApp(t, "google", p)

	// A shape mismatch is a decode error, never a panic.
	_, err := getGoogleUserInfo(t.Context(), &oauth2.Token{AccessToken: "provider-access-token"})
	if err == nil {
		t.Log("decoded with zero values — acceptable; the point is that it did not panic")
	}
}

func TestAzureUserInfo_PrefersMailOverUserPrincipalName(t *testing.T) {
	p := newStubProvider(t)
	p.userInfo = map[string]any{
		"id": "aad-1", "displayName": "Alex Dembele",
		"mail":              "alex@opendefender.io",
		"userPrincipalName": "alex_opendefender.io#EXT#@tenant.onmicrosoft.com",
	}
	_ = newOAuthTestApp(t, "azure", p)

	got, err := getAzureUserInfo(t.Context(), &oauth2.Token{AccessToken: "provider-access-token"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "alex@opendefender.io" {
		t.Errorf("expected `mail` preferred over the UPN directory identifier, got %q", got.Email)
	}
}

// ---------------------------------------------------------------------------
// Failure handling — never a blank page
// ---------------------------------------------------------------------------

func TestOAuthCallback_EveryFailureRedirectsToTheLoginScreenWithACode(t *testing.T) {
	// The browser navigated here. A JSON body or a bare 500 is a dead end with no
	// way back; every exit must land on /login carrying a code the SPA can render.
	p := newStubProvider(t)
	app := newOAuthTestApp(t, "google", p)

	liveState, _ := startFlow(t, app, "google")

	cases := []struct {
		name, query, wantError string
	}{
		{"provider refused consent", "error=access_denied&state=x", "access_denied"},
		{"consent required", "error=consent_required&state=x", "consent_required"},
		{"unknown provider error", "error=weird_thing&state=x", "provider_error"},
		{"no state", "code=abc", "state_missing"},
		{"unknown state", "state=not-a-real-state&code=abc", "state_invalid"},
		{"no code", "state=" + liveState, "code_missing"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, params := callbackError(t, app, "google", tc.query)

			if status != fiber.StatusFound {
				t.Fatalf("expected a redirect, got %d — a non-redirect here is the blank page", status)
			}
			if got := params.Get("error"); got != tc.wantError {
				t.Errorf("expected error=%q, got %q", tc.wantError, got)
			}
		})
	}
}

func TestOAuthCallback_StateIsSingleUse(t *testing.T) {
	// A replayable state would defeat the CSRF binding it exists to provide.
	p := newStubProvider(t)
	app := newOAuthTestApp(t, "google", p)

	state, _ := startFlow(t, app, "google")
	query := "state=" + state + "&code=whatever"

	_, _ = callbackError(t, app, "google", query)
	_, second := callbackError(t, app, "google", query)

	if second.Get("error") != "state_invalid" {
		t.Errorf("expected the state to be spent after one use, got %q", second.Get("error"))
	}
}

func TestOAuthCallback_StateFromAnotherProviderIsRefused(t *testing.T) {
	p := newStubProvider(t)
	app := newOAuthTestApp(t, "google", p)
	// Register github too so the route resolves a config for it.
	oauth2Config.GitHubConfig = p.oauthConfig()

	state, _ := startFlow(t, app, "google")

	_, params := callbackError(t, app, "github", "state="+state+"&code=abc")
	if params.Get("error") != "state_invalid" {
		t.Errorf("a state minted for google must not complete a github callback, got %q", params.Get("error"))
	}
}

func TestOAuthCallback_ExchangeFailureIsReportedNotCrashed(t *testing.T) {
	p := newStubProvider(t)
	p.rejectVerifier = true
	app := newOAuthTestApp(t, "google", p)

	state, _ := startFlow(t, app, "google")
	_, params := callbackError(t, app, "google", "state="+state+"&code=auth-code-"+state)

	if params.Get("error") != "exchange_failed" {
		t.Errorf("expected exchange_failed, got %q", params.Get("error"))
	}
}

func TestOAuthLogin_UnconfiguredProviderSaysSoInsteadOfBouncing(t *testing.T) {
	p := newStubProvider(t)
	app := newOAuthTestApp(t, "google", p)
	oauth2Config.GoogleConfig.ClientID = "" // never configured in this deployment

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth2/login/google", nil)
	resp, _ := app.Test(req, -1)
	loc, _ := url.Parse(resp.Header.Get("Location"))

	if loc.Query().Get("error") != "provider_not_configured" {
		t.Errorf("expected provider_not_configured, got %q", loc.Query().Get("error"))
	}
}

func TestOAuthLogin_UnsupportedProvider(t *testing.T) {
	p := newStubProvider(t)
	app := newOAuthTestApp(t, "google", p)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth2/login/facebook", nil)
	resp, _ := app.Test(req, -1)
	loc, _ := url.Parse(resp.Header.Get("Location"))

	if loc.Query().Get("error") != "unsupported_provider" {
		t.Errorf("expected unsupported_provider, got %q", loc.Query().Get("error"))
	}
}

// ---------------------------------------------------------------------------
// Open redirect
// ---------------------------------------------------------------------------

func TestSanitiseReturnTo_RejectsOffsiteTargets(t *testing.T) {
	// An open redirect on a login endpoint is a phishing primitive: the URL the
	// victim inspects is genuinely ours.
	for _, hostile := range []string{
		"https://evil.example.com",
		"//evil.example.com",
		"/\\evil.example.com",
		"http://evil.example.com/path",
		"javascript:alert(1)",
	} {
		if got := sanitiseReturnTo(hostile); got != "" {
			t.Errorf("sanitiseReturnTo(%q) = %q, want it dropped", hostile, got)
		}
	}

	for _, ok := range []string{"/risks", "/settings?tab=sessions"} {
		if got := sanitiseReturnTo(ok); got != ok {
			t.Errorf("sanitiseReturnTo(%q) = %q, want it kept", ok, got)
		}
	}
}
