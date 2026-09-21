// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

// Package ticketing opens ITSM tickets (Jira, ServiceNow) from OpenRisk. Each
// provider makes REAL authenticated REST calls; with absent/wrong credentials it
// returns the tool's real error — never a fake ticket. It has no dependency on
// the domain layer (provider names are plain strings) so it stays reusable.
package ticketing

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/opendefender/openrisk/pkg/netguard"
)

// Provider names (kept as plain constants; the caller maps its own enum).
const (
	ProviderJira       = "jira"
	ProviderServiceNow = "servicenow"
)

// HTTPDoer is the seam for real requests (*http.Client) and httptest.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// CreateRequest is a provider-agnostic ticket request.
type CreateRequest struct {
	BaseURL        string
	Credentials    map[string]string
	ProjectOrTable string // Jira project key | ServiceNow table
	IssueType      string // Jira issue type (default Bug)
	Summary        string
	Description    string
	Priority       string   // normalised: critical|high|medium|low
	Labels         []string // best-effort (Jira labels)
	HTTP           HTTPDoer
}

func (r CreateRequest) http() HTTPDoer {
	if r.HTTP != nil {
		return r.HTTP
	}
	return netguard.Client(netguard.Options{Timeout: 20 * time.Second})
}

// checkBaseURL re-validates the instance URL at request time, so a config saved
// before the guard existed cannot aim the server at a private address. An
// injected HTTPDoer (tests) brings its own transport and skips the URL check;
// the production client is guarded at dial time as well.
func (r CreateRequest) checkBaseURL(provider string) error {
	if r.HTTP != nil {
		return nil
	}
	if err := netguard.ValidateURL(r.BaseURL); err != nil {
		return fmt.Errorf("%s: %w", provider, err)
	}
	return nil
}

// statusError reports a non-2xx answer by status only. The remote body is never
// echoed back to the caller.
func statusError(provider, what string, code int) error {
	return fmt.Errorf("%s: %s returned %d %s", provider, what, code, http.StatusText(code))
}

func (r CreateRequest) cred(keys ...string) string {
	for _, k := range keys {
		if v, ok := r.Credentials[k]; ok && v != "" {
			return v
		}
	}
	return ""
}

// Ticket is the result of a successful create.
type Ticket struct {
	Provider string `json:"provider"`
	Key      string `json:"key"` // human ref (SEC-12 / INC0012345)
	URL      string `json:"url"`
}

// Provider opens a ticket from a CreateRequest.
type Provider interface {
	Name() string
	Create(ctx context.Context, req CreateRequest) (Ticket, error)
}

// ProviderFor returns the provider implementation for a name.
func ProviderFor(name string) (Provider, bool) {
	switch name {
	case ProviderJira:
		return jiraProvider{}, true
	case ProviderServiceNow:
		return serviceNowProvider{}, true
	default:
		return nil, false
	}
}

// basicAuth builds an Authorization: Basic header value.
func basicAuth(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}
