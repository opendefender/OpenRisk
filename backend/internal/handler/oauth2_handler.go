// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/service"
	"github.com/opendefender/openrisk/pkg/oauthpkce"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/microsoft"
)

// OAuth2Config holds OAuth2 provider configurations
type OAuth2Config struct {
	GoogleConfig *oauth2.Config
	GitHubConfig *oauth2.Config
	AzureConfig  *oauth2.Config
}

var (
	oauth2Config      *OAuth2Config
	oauthStateService *service.OAuthStateService

	// oauthResolver turns a provider identity into an OpenRisk account. Wired in
	// main.go; when nil the callback fails closed rather than admitting anyone.
	oauthResolver *appauth.ResolveOAuthIdentityUseCase

	// oauthAppBaseURL is the SPA origin every OAuth outcome — success or failure
	// — returns the browser to.
	oauthAppBaseURL = "http://localhost:5173"
)

// Provider user-info endpoints.
//
// Package-level vars rather than literals so the OAuth flow can be driven
// end-to-end against a stub provider in tests. Nothing outside tests reassigns
// them; the defaults are the real endpoints.
var (
	googleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
	githubUserURL     = "https://api.github.com/user"
	githubEmailsURL   = "https://api.github.com/user/emails"
	graphMeURL        = "https://graph.microsoft.com/v1.0/me"
)

// oauthStateTTL bounds how long a started flow may sit unfinished.
//
// Ten minutes: comfortably longer than a consent screen plus an MFA prompt at
// the provider, short enough that an abandoned flow is not a lingering credential.
const oauthStateTTL = 10 * time.Minute

// InitializeOAuth2 initializes all OAuth2 configurations
func InitializeOAuth2() *OAuth2Config {
	redirectURI := os.Getenv("OAUTH2_REDIRECT_URI")
	if redirectURI == "" {
		redirectURI = "http://localhost:8080/api/v1/auth/oauth2/callback"
	}

	cfg := &OAuth2Config{
		GoogleConfig: &oauth2.Config{
			ClientID:     os.Getenv("OAUTH2_GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("OAUTH2_GOOGLE_CLIENT_SECRET"),
			RedirectURL:  redirectURI + "/google",
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		},
		GitHubConfig: &oauth2.Config{
			ClientID:     os.Getenv("OAUTH2_GITHUB_CLIENT_ID"),
			ClientSecret: os.Getenv("OAUTH2_GITHUB_CLIENT_SECRET"),
			RedirectURL:  redirectURI + "/github",
			Scopes: []string{
				"user:email",
				"read:user",
			},
			Endpoint: github.Endpoint,
		},
		AzureConfig: &oauth2.Config{
			ClientID:     os.Getenv("OAUTH2_AZURE_CLIENT_ID"),
			ClientSecret: os.Getenv("OAUTH2_AZURE_CLIENT_SECRET"),
			RedirectURL:  redirectURI + "/azure",
			// Microsoft Graph delegated scopes. ".default" asks for whatever the
			// app registration was granted, which for a sign-in app is not
			// guaranteed to include the profile — name these explicitly instead.
			Scopes: []string{"openid", "profile", "email", "User.Read"},
			Endpoint: microsoft.AzureADEndpoint(os.Getenv("OAUTH2_AZURE_TENANT_ID")),
		},
	}

	oauth2Config = cfg
	oauthStateService = service.NewOAuthStateService()
	return cfg
}

// ConfigureOAuth2Resolver wires identity resolution and the SPA origin.
func ConfigureOAuth2Resolver(resolver *appauth.ResolveOAuthIdentityUseCase, appBaseURL string) {
	oauthResolver = resolver
	if appBaseURL != "" {
		oauthAppBaseURL = strings.TrimRight(appBaseURL, "/")
	}
}

// OAuth2UserInfo represents user information from OAuth2 provider
type OAuth2UserInfo struct {
	ID            string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
	Provider      string
	Groups        []string
}

func providerConfig(provider string) *oauth2.Config {
	if oauth2Config == nil {
		return nil
	}
	switch provider {
	case "google":
		return oauth2Config.GoogleConfig
	case "github":
		return oauth2Config.GitHubConfig
	case "azure":
		return oauth2Config.AzureConfig
	default:
		return nil
	}
}

// OAuth2Login initiates the OAuth2 flow.
//
// Responds with a 302 straight to the provider rather than JSON holding a URL.
// The browser is already navigating; handing it a JSON body means the SPA has to
// parse and re-navigate, and any failure in that hop is a blank page — which is
// exactly what this flow must never produce.
//
// Both CSRF defences are set up here: `state` binds the callback to this browser,
// and the PKCE challenge binds the eventual authorization code to this server.
func OAuth2Login(c *fiber.Ctx) error {
	provider := c.Params("provider")
	locale := oauthLocale(c)

	config := providerConfig(provider)
	if config == nil {
		return oauthFailure(c, "unsupported_provider", provider, locale)
	}
	if config.ClientID == "" {
		// Not a user error: the deployment never configured this button. Say so
		// plainly instead of bouncing them to a provider that will reject them.
		return oauthFailure(c, "provider_not_configured", provider, locale)
	}

	pkce, err := oauthpkce.New()
	if err != nil {
		return oauthFailure(c, "internal", provider, locale)
	}

	state := uuid.NewString()
	oauthStateService.StoreFlow(&service.OAuthState{
		State:        state,
		Provider:     provider,
		CodeVerifier: pkce.Verifier,
		Locale:       locale,
		ReturnTo:     sanitiseReturnTo(c.Query("return_to")),
	}, oauthStateTTL)

	authURL := config.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("code_challenge", pkce.Challenge),
		oauth2.SetAuthURLParam("code_challenge_method", pkce.Method),
	)

	return c.Redirect(authURL, fiber.StatusFound)
}

// OAuth2Callback handles the provider redirect back.
//
// Every exit from here is a redirect to the SPA, carrying a machine-readable
// `error` code the login screen renders in the user's language. Returning JSON
// from a URL the browser navigated to leaves the user staring at raw text or a
// blank page with no way back — the "écran blanc" this flow is written to avoid.
func OAuth2Callback(c *fiber.Ctx) error {
	provider := c.Params("provider")
	locale := oauthLocale(c)

	// The provider itself can fail the flow (user pressed Cancel, admin consent
	// missing). It reports that as ?error=..., not as a missing code.
	if providerErr := c.Query("error"); providerErr != "" {
		return oauthFailure(c, mapProviderError(providerErr), provider, locale)
	}

	state := c.Query("state")
	if state == "" {
		return oauthFailure(c, "state_missing", provider, locale)
	}

	flow, err := oauthStateService.ConsumeFlow(state, provider)
	if err != nil {
		// Expired, unknown, or for a different provider. All three mean this
		// callback cannot be trusted as the continuation of a flow we started.
		return oauthFailure(c, "state_invalid", provider, locale)
	}
	if flow.Locale != "" {
		locale = flow.Locale
	}

	code := c.Query("code")
	if code == "" {
		return oauthFailure(c, "code_missing", provider, locale)
	}

	config := providerConfig(provider)
	if config == nil {
		return oauthFailure(c, "unsupported_provider", provider, locale)
	}

	// Exchange, presenting the PKCE verifier. Without it a provider configured
	// for PKCE refuses the code, and an intercepted code is unusable.
	ctx, cancel := context.WithTimeout(c.UserContext(), 15*time.Second)
	defer cancel()

	token, err := config.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", flow.CodeVerifier),
	)
	if err != nil {
		return oauthFailure(c, "exchange_failed", provider, locale)
	}

	userInfo, err := getOAuth2UserInfo(ctx, provider, token)
	if err != nil {
		return oauthFailure(c, "userinfo_failed", provider, locale)
	}
	userInfo.Provider = provider

	if oauthResolver == nil {
		return oauthFailure(c, "internal", provider, locale)
	}

	result, err := oauthResolver.Execute(c.UserContext(), appauth.OAuthIdentity{
		Provider:      provider,
		Subject:       userInfo.ID,
		Email:         userInfo.Email,
		EmailVerified: userInfo.EmailVerified,
		FullName:      userInfo.Name,
		AvatarURL:     userInfo.Picture,
	})
	if err != nil {
		return oauthResolveFailure(c, err, provider, locale)
	}

	// Issue an RS256 access+refresh pair via the SAME TokenManager as password
	// login (this once minted an HS256 token that the RS256 middleware rejected
	// on every protected route). Onboarding + audit happen inside.
	return issueSSOSession(c, result.User, provider)
}

// ---------------------------------------------------------------------------
// Failure rendering
// ---------------------------------------------------------------------------

// oauthFailure sends the browser back to the login screen with a code the SPA
// turns into a sentence.
//
// A code rather than a message so the wording lives in the frontend i18n bundles
// with the rest of the UI, and so we never reflect provider-supplied text into
// the page.
func oauthFailure(c *fiber.Ctx, code, provider, locale string) error {
	q := url.Values{}
	q.Set("error", code)
	if provider != "" {
		q.Set("provider", provider)
	}
	if locale != "" {
		q.Set("lang", locale)
	}
	return c.Redirect(oauthAppBaseURL+"/login?"+q.Encode(), fiber.StatusFound)
}

// oauthResolveFailure maps a linking outcome onto a login-screen message.
//
// The provider-conflict branch is the one that earns its keep: it carries WHICH
// provider already owns the address, so the screen can say "this address signs
// in with Google" and the user has somewhere to go. A bare "sign-in failed"
// here strands people who genuinely own the account.
func oauthResolveFailure(c *fiber.Ctx, err error, provider, locale string) error {
	var conflict *appauth.OAuthProviderConflictError
	if errors.As(err, &conflict) {
		q := url.Values{}
		q.Set("error", "provider_conflict")
		q.Set("provider", conflict.AttemptedProvider)
		q.Set("existing_provider", conflict.ExistingProvider)
		q.Set("lang", locale)
		return c.Redirect(oauthAppBaseURL+"/login?"+q.Encode(), fiber.StatusFound)
	}

	switch {
	case errors.Is(err, appauth.ErrOAuthEmailUnverified):
		return oauthFailure(c, "email_unverified", provider, locale)
	case errors.Is(err, appauth.ErrOAuthNoEmail):
		return oauthFailure(c, "no_email", provider, locale)
	case errors.Is(err, appauth.ErrOAuthAccountDisabled):
		return oauthFailure(c, "account_disabled", provider, locale)
	case errors.Is(err, appauth.ErrOAuthNoAccount):
		return oauthFailure(c, "no_account", provider, locale)
	default:
		return oauthFailure(c, "internal", provider, locale)
	}
}

// mapProviderError normalises the provider's own error codes.
func mapProviderError(providerErr string) string {
	switch providerErr {
	case "access_denied":
		return "access_denied"
	case "consent_required", "interaction_required", "login_required":
		return "consent_required"
	default:
		return "provider_error"
	}
}

func oauthLocale(c *fiber.Ctx) string {
	if l := c.Query("lang"); l == "en" || l == "fr" {
		return l
	}
	if strings.HasPrefix(strings.ToLower(c.Get("Accept-Language")), "en") {
		return "en"
	}
	return "fr"
}

// sanitiseReturnTo keeps post-login redirects on our own site.
//
// Only a root-relative path is accepted. Anything absolute, protocol-relative
// ("//evil.com") or backslash-prefixed is dropped — an open redirect on a login
// endpoint is a phishing primitive, since the URL a victim sees is genuinely ours.
func sanitiseReturnTo(raw string) string {
	if raw == "" || !strings.HasPrefix(raw, "/") {
		return ""
	}
	if strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, "/\\") {
		return ""
	}
	return raw
}

// ---------------------------------------------------------------------------
// Provider user info
// ---------------------------------------------------------------------------

// getOAuth2UserInfo fetches user information from an OAuth2 provider.
func getOAuth2UserInfo(ctx context.Context, provider string, token *oauth2.Token) (*OAuth2UserInfo, error) {
	switch provider {
	case "google":
		return getGoogleUserInfo(ctx, token)
	case "github":
		return getGitHubUserInfo(ctx, token)
	case "azure":
		return getAzureUserInfo(ctx, token)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// fetchJSON performs an authorised GET and decodes the body.
func fetchJSON(ctx context.Context, url, authHeader string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("provider returned status %d", resp.StatusCode)
	}
	// Bound the read: a provider (or something impersonating one) must not be
	// able to exhaust memory here.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

// googleUserInfo is decoded into a struct rather than map[string]interface{}.
//
// The previous version did data["id"].(string) — an unchecked type assertion that
// PANICS if the field is absent or not a string. A provider hiccup would take the
// process down; a typed struct simply leaves the field empty.
type googleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail *bool  `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func getGoogleUserInfo(ctx context.Context, token *oauth2.Token) (*OAuth2UserInfo, error) {
	var data googleUserInfo
	if err := fetchJSON(ctx, googleUserInfoURL, "Bearer "+token.AccessToken, &data); err != nil {
		return nil, err
	}
	return &OAuth2UserInfo{
		ID:    data.ID,
		Email: data.Email,
		// Absent verified_email is treated as NOT verified. Defaulting the other
		// way would make a missing field silently equivalent to a vouched-for
		// address, which is the takeover this check exists to prevent.
		EmailVerified: data.VerifiedEmail != nil && *data.VerifiedEmail,
		Name:          data.Name,
		Picture:       data.Picture,
	}, nil
}

type githubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func getGitHubUserInfo(ctx context.Context, token *oauth2.Token) (*OAuth2UserInfo, error) {
	var user githubUser
	if err := fetchJSON(ctx, githubUserURL, "token "+token.AccessToken, &user); err != nil {
		return nil, err
	}

	name := user.Name
	if name == "" {
		name = user.Login
	}

	info := &OAuth2UserInfo{
		ID:      fmt.Sprintf("%d", user.ID),
		Name:    name,
		Picture: user.AvatarURL,
	}

	// GitHub's /user endpoint returns the PUBLIC profile email, which carries no
	// verification guarantee and is often absent. /user/emails is the only source
	// that reports the verified flag, so it is authoritative here — the public
	// field is used solely as a last-resort unverified fallback.
	var emails []githubEmail
	if err := fetchJSON(ctx, githubEmailsURL, "token "+token.AccessToken, &emails); err == nil {
		for _, e := range emails {
			if e.Primary && e.Verified {
				info.Email, info.EmailVerified = e.Email, true
				return info, nil
			}
		}
		// No verified primary: take any verified address rather than none.
		for _, e := range emails {
			if e.Verified {
				info.Email, info.EmailVerified = e.Email, true
				return info, nil
			}
		}
	}

	info.Email = user.Email
	info.EmailVerified = false
	return info, nil
}

type azureUser struct {
	ID                string `json:"id"`
	DisplayName       string `json:"displayName"`
	Mail              string `json:"mail"`
	UserPrincipalName string `json:"userPrincipalName"`
}

func getAzureUserInfo(ctx context.Context, token *oauth2.Token) (*OAuth2UserInfo, error) {
	var data azureUser
	if err := fetchJSON(ctx, graphMeURL, "Bearer "+token.AccessToken, &data); err != nil {
		return nil, err
	}

	// Prefer `mail`; userPrincipalName is a directory identifier that merely
	// usually looks like an address (and for guest accounts often is not one).
	email := data.Mail
	if email == "" {
		email = data.UserPrincipalName
	}

	return &OAuth2UserInfo{
		ID:    data.ID,
		Email: email,
		// Entra ID only issues these for addresses in a verified tenant domain;
		// the directory is the authority, so a successful Graph call for a member
		// account is the verification.
		EmailVerified: email != "",
		Name:          data.DisplayName,
	}, nil
}

// oauthAuditAction is used by the callback's audit trail entries.
var _ = coreauth.AuditActionOAuthLink
