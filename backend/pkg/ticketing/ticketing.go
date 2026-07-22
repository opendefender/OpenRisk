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
	"net/http"
	"time"
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
	return &http.Client{Timeout: 20 * time.Second}
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
