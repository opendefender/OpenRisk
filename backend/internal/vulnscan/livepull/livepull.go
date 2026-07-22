// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

// Package livepull turns a configured vulnerability integration into a set of
// NATIVE findings by calling the tool's REST API. The findings it returns are in
// the same shape the tool exports, so they flow straight into the existing
// per-source normalisers (internal/vulnscan) → prioritisation → upsert.
//
// HONEST BOUNDARY: pullers make REAL authenticated HTTP calls. With absent or
// wrong credentials they return the tool's real auth error — never fabricated
// findings. Tools without a clean REST+JSON contract (OpenVAS speaks GMP over a
// TLS socket; AWS Inspector is an SDK) are represented by an honest seam that
// reports "live pull not wired — use the webhook or import" rather than guessing.
package livepull

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/opendefender/openrisk/internal/domain"
)

// HTTPDoer is the seam used for real requests (satisfied by *http.Client) and for
// httptest-backed unit tests.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// PullConfig is everything a puller needs. HTTP defaults to a 30s client.
type PullConfig struct {
	Source      domain.VulnSource
	BaseURL     string
	Credentials map[string]string
	HTTP        HTTPDoer
}

func (c PullConfig) http() HTTPDoer {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c PullConfig) cred(keys ...string) string {
	for _, k := range keys {
		if v, ok := c.Credentials[k]; ok && v != "" {
			return v
		}
	}
	return ""
}

// Puller fetches native findings from one tool's API.
type Puller interface {
	// Pull returns findings in the tool's native shape (ready for the matching
	// internal/vulnscan normaliser). Returns a typed error on auth/transport
	// failure; NEVER fabricates data.
	Pull(ctx context.Context, cfg PullConfig) ([]map[string]any, error)
	// LivePullSupported reports whether real API polling is wired for this source.
	LivePullSupported() bool
}

// registry maps a source to its puller.
var registry = map[domain.VulnSource]Puller{
	domain.VulnSourceMSDefender:    msDefenderPuller{},
	domain.VulnSourceCrowdStrike:   crowdStrikePuller{},
	domain.VulnSourceNessus:        nessusPuller{},
	domain.VulnSourceQualys:        qualysPuller{},
	domain.VulnSourceAzureDefender: azureDefenderPuller{},
	// Honest seams — real live pull not available via a clean REST contract here.
	domain.VulnSourceOpenVAS:      seamPuller{reason: "OpenVAS/Greenbone speaks GMP over a TLS socket, not REST — use the webhook or import GMP results JSON."},
	domain.VulnSourceAWSInspector: seamPuller{reason: "AWS Inspector live enumeration runs through the scanner's SDK collector — use the webhook or import inspector2 findings here."},
}

// PullerFor returns the puller for a source.
func PullerFor(src domain.VulnSource) (Puller, bool) {
	p, ok := registry[src]
	return p, ok
}

// LivePullSupported reports whether a source has a real (non-seam) puller.
func LivePullSupported(src domain.VulnSource) bool {
	p, ok := registry[src]
	return ok && p.LivePullSupported()
}

// ---- shared helpers -------------------------------------------------------

// errAuth marks a missing-credential configuration error (mapped to 400 upstream).
func errMissingCred(name string) error {
	return domain.NewValidationError("missing required credential: " + name)
}

// oauthClientCredentials performs an OAuth2 client-credentials token request
// (application/x-www-form-urlencoded) and returns the access token.
func oauthClientCredentials(ctx context.Context, doer HTTPDoer, tokenURL string, form url.Values) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := doer.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("token parse failed: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("token endpoint returned no access_token")
	}
	return tok.AccessToken, nil
}

// getJSON issues a GET with the provided headers and decodes the JSON body.
func getJSON(ctx context.Context, doer HTTPDoer, endpoint string, headers map[string]string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("response parse failed: %w", err)
	}
	return out, nil
}

// getRaw issues a GET and returns the raw body (for XML APIs like Qualys).
func getRaw(ctx context.Context, doer HTTPDoer, endpoint string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// asObjects coerces a decoded JSON array (any of these keys) into []map[string]any.
func asObjects(v any) []map[string]any {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(arr))
	for _, e := range arr {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// basicAuth returns the value for an Authorization: Basic header.
func basicAuth(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}
