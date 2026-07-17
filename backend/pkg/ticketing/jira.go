// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package ticketing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// jiraProvider creates issues via the Jira Cloud REST API (v2), authenticated
// with Basic auth (account email + API token).
type jiraProvider struct{}

func (jiraProvider) Name() string { return ProviderJira }

func (jiraProvider) Create(ctx context.Context, req CreateRequest) (Ticket, error) {
	if req.BaseURL == "" {
		return Ticket{}, fmt.Errorf("jira: base_url is required")
	}
	email := req.cred("email", "username", "user")
	token := req.cred("api_token", "token", "password")
	if email == "" || token == "" {
		return Ticket{}, fmt.Errorf("jira: email + api_token credentials are required")
	}
	if req.ProjectOrTable == "" {
		return Ticket{}, fmt.Errorf("jira: project key is required")
	}
	issueType := req.IssueType
	if issueType == "" {
		issueType = "Bug"
	}

	fields := map[string]any{
		"project":     map[string]any{"key": req.ProjectOrTable},
		"summary":     req.Summary,
		"description": req.Description,
		"issuetype":   map[string]any{"name": issueType},
	}
	if len(req.Labels) > 0 {
		fields["labels"] = sanitizeLabels(req.Labels)
	}
	payload, _ := json.Marshal(map[string]any{"fields": fields})

	endpoint := strings.TrimRight(req.BaseURL, "/") + "/rest/api/2/issue"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return Ticket{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", basicAuth(email, token))

	resp, err := req.http().Do(httpReq)
	if err != nil {
		return Ticket{}, fmt.Errorf("jira: request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Ticket{}, fmt.Errorf("jira: create issue returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.Key == "" {
		return Ticket{}, fmt.Errorf("jira: unexpected response: %s", strings.TrimSpace(string(body)))
	}
	return Ticket{
		Provider: ProviderJira,
		Key:      out.Key,
		URL:      strings.TrimRight(req.BaseURL, "/") + "/browse/" + out.Key,
	}, nil
}

// sanitizeLabels strips spaces (Jira labels cannot contain whitespace).
func sanitizeLabels(in []string) []string {
	out := make([]string, 0, len(in))
	for _, l := range in {
		l = strings.ReplaceAll(strings.TrimSpace(l), " ", "-")
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}
