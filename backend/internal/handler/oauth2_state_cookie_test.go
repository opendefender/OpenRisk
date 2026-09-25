// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"

	"github.com/opendefender/openrisk/internal/service"
)

// The state cookie binds a callback to the browser that started the flow (#295).
// Without it, a callback URL minted by an attacker completes in the victim's
// browser and signs the victim in to the attacker's account.

// loginSetCookie drives /login and returns the minted state and the state cookie.
func loginSetCookie(t *testing.T, app *fiber.App, provider string) (string, *http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth2/login/"+provider, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	for _, ck := range resp.Cookies() {
		if ck.Name == OAuthStateCookie {
			return loc.Query().Get("state"), ck
		}
	}
	t.Fatalf("/login set no %s cookie", OAuthStateCookie)
	return "", nil
}

// stateCookieOn returns the state cookie a response sets, or nil.
func stateCookieOn(resp *http.Response) *http.Cookie {
	for _, ck := range resp.Cookies() {
		if ck.Name == OAuthStateCookie {
			return ck
		}
	}
	return nil
}

func errorCode(t *testing.T, resp *http.Response) string {
	t.Helper()
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil || loc == nil {
		t.Fatalf("expected a redirect to the SPA, got %d", resp.StatusCode)
	}
	return loc.Query().Get("error")
}

func TestOAuthLogin_SetsAShortLivedHttpOnlyStateCookie(t *testing.T) {
	app := newOAuthTestApp(t, "google", newStubProvider(t))

	state, ck := loginSetCookie(t, app, "google")

	if ck.Value != state {
		t.Errorf("cookie must carry the minted state %q, got %q", state, ck.Value)
	}
	if !ck.HttpOnly {
		t.Error("state cookie must be HttpOnly so no script can read it")
	}
	if ck.SameSite != http.SameSiteLaxMode {
		t.Errorf("state cookie must be SameSite=Lax (Strict is withheld on the provider's redirect back), got %v", ck.SameSite)
	}
	if ck.MaxAge != int(oauthStateTTL/time.Second) {
		t.Errorf("state cookie must live exactly the flow TTL (%v), got Max-Age=%d", oauthStateTTL, ck.MaxAge)
	}
	if ck.Path != "/api/v1/auth/oauth2/callback" {
		t.Errorf("state cookie must be scoped to the callback path, got %q", ck.Path)
	}
}

func TestOAuthLogin_StateCookieIsSecureInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	app := newOAuthTestApp(t, "google", newStubProvider(t))

	_, ck := loginSetCookie(t, app, "google")
	if !ck.Secure {
		t.Error("state cookie must be Secure in production")
	}
}

func TestOAuthLogin_StateCookieIsNotSecureInDevelopment(t *testing.T) {
	// Secure cookies are dropped over http://, which is how development runs.
	t.Setenv("APP_ENV", "development")
	app := newOAuthTestApp(t, "google", newStubProvider(t))

	_, ck := loginSetCookie(t, app, "google")
	if ck.Secure {
		t.Error("state cookie must not be Secure in development")
	}
}

func TestOAuthStateCookiePath_FollowsTheProviderRedirectURL(t *testing.T) {
	cases := []struct{ redirect, want string }{
		{"https://grc.example.com/api/v1/auth/oauth2/callback/google", "/api/v1/auth/oauth2/callback/google"},
		{"https://example.com/openrisk/api/v1/auth/oauth2/callback/azure", "/openrisk/api/v1/auth/oauth2/callback/azure"},
		{"", oauthCallbackFallbackPath},
		{"::not a url", oauthCallbackFallbackPath},
	}
	for _, tc := range cases {
		if got := oauthStateCookiePath(&oauth2.Config{RedirectURL: tc.redirect}); got != tc.want {
			t.Errorf("redirect %q: expected path %q, got %q", tc.redirect, tc.want, got)
		}
	}
	if got := oauthStateCookiePath(nil); got != oauthCallbackFallbackPath {
		t.Errorf("nil config: expected the fallback path, got %q", got)
	}
}

func TestOAuthCallback_MissingStateCookieIsRefused(t *testing.T) {
	// The login-CSRF case: a valid, live state arriving from a browser that never
	// started the flow.
	app := newOAuthTestApp(t, "google", newStubProvider(t))
	state, _ := loginSetCookie(t, app, "google")

	resp := callbackWithCookie(t, app, "google", "state="+state+"&code=abc", "")
	if got := errorCode(t, resp); got != "state_invalid" {
		t.Errorf("expected state_invalid without the cookie, got %q", got)
	}
}

func TestOAuthCallback_MismatchedStateCookieIsRefused(t *testing.T) {
	// The attacker's own browser has a cookie too — for its own flow, not the
	// victim's. Two live flows; each cookie only vouches for its own state.
	app := newOAuthTestApp(t, "google", newStubProvider(t))
	victimState, _ := loginSetCookie(t, app, "google")
	_, attackerCookie := loginSetCookie(t, app, "google")

	resp := callbackWithCookie(t, app, "google", "state="+victimState+"&code=abc", attackerCookie.Value)
	if got := errorCode(t, resp); got != "state_invalid" {
		t.Errorf("expected state_invalid for a cookie from another flow, got %q", got)
	}
}

func TestOAuthCallback_RefusedCallbackDoesNotSpendTheFlow(t *testing.T) {
	// A forged callback must not burn the real user's flow: once the genuine
	// browser comes back with its cookie, the flow is still there.
	app := newOAuthTestApp(t, "google", newStubProvider(t))
	state, ck := loginSetCookie(t, app, "google")

	_ = callbackWithCookie(t, app, "google", "state="+state+"&code=abc", "")

	// The genuine callback with no code gets past the state checks and stops at
	// code_missing — proof the flow survived the forged attempt.
	resp := callbackWithCookie(t, app, "google", "state="+state, ck.Value)
	if got := errorCode(t, resp); got != "code_missing" {
		t.Errorf("expected the flow to survive a forged callback (code_missing), got %q", got)
	}
}

func TestOAuthCallback_ExpiredStateIsRefusedEvenWithItsCookie(t *testing.T) {
	app := newOAuthTestApp(t, "google", newStubProvider(t))

	// Park a flow that expired a second ago; the browser still holds its cookie.
	oauthStateService.StoreFlow(&service.OAuthState{State: "stale-state", Provider: "google"}, -time.Second)

	resp := callbackWithCookie(t, app, "google", "state=stale-state&code=abc", "stale-state")
	if got := errorCode(t, resp); got != "state_invalid" {
		t.Errorf("expected state_invalid for an expired flow, got %q", got)
	}
}

func TestOAuthCallback_BoundStateCompletesTheExchange(t *testing.T) {
	p := newStubProvider(t)
	p.userInfo = map[string]any{
		"id": "google-sub-1", "email": "member@opendefender.io", "verified_email": true,
	}
	app := newOAuthTestApp(t, "google", p)

	state, ck := loginSetCookie(t, app, "google")
	code := "auth-code-" + state
	registerChallengeForCode(t, p, "google", state, code)

	resp := callbackWithCookie(t, app, "google", "state="+state+"&code="+code, ck.Value)

	if p.tokenCalls != 1 {
		t.Fatalf("expected the cookie-bound callback to reach the token exchange, got %d calls", p.tokenCalls)
	}
	// No resolver is wired, so the flow stops right after user info.
	if got := errorCode(t, resp); got != "internal" {
		t.Errorf("expected the flow to reach identity resolution, got %q", got)
	}
}

func TestOAuthCallback_EveryExitClearsTheStateCookie(t *testing.T) {
	app := newOAuthTestApp(t, "google", newStubProvider(t))
	state, ck := loginSetCookie(t, app, "google")

	cases := []struct{ name, query, cookie string }{
		{"provider error", "error=access_denied&state=" + state, ck.Value},
		{"no state", "code=abc", ck.Value},
		{"cookie mismatch", "state=" + state + "&code=abc", "other"},
		{"bound but no code", "state=" + state, ck.Value},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := callbackWithCookie(t, app, "google", tc.query, tc.cookie)
			cleared := stateCookieOn(resp)
			if cleared == nil {
				t.Fatal("callback did not clear the state cookie")
			}
			if cleared.Value != "" || !cleared.Expires.Before(time.Now()) {
				t.Errorf("expected an empty cookie expiring in the past, got value=%q expires=%v", cleared.Value, cleared.Expires)
			}
			if cleared.Path != ck.Path {
				t.Errorf("cleared with Path %q but set with %q — the original would survive", cleared.Path, ck.Path)
			}
		})
	}
}
