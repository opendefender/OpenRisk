// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

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

// serviceNowProvider creates records via the ServiceNow Table API, authenticated
// with Basic auth. The default table is `incident`.
type serviceNowProvider struct{}

func (serviceNowProvider) Name() string { return ProviderServiceNow }

// snPriority maps our normalised priority to ServiceNow's 1(Critical)–4(Low).
func snPriority(p string) string {
	switch strings.ToLower(p) {
	case "critical":
		return "1"
	case "high":
		return "2"
	case "medium":
		return "3"
	default:
		return "4"
	}
}

func (serviceNowProvider) Create(ctx context.Context, req CreateRequest) (Ticket, error) {
	if req.BaseURL == "" {
		return Ticket{}, fmt.Errorf("servicenow: base_url is required")
	}
	if err := req.checkBaseURL("servicenow"); err != nil {
		return Ticket{}, err
	}
	user := req.cred("username", "user", "email")
	pass := req.cred("password", "api_token", "token")
	if user == "" || pass == "" {
		return Ticket{}, fmt.Errorf("servicenow: username + password credentials are required")
	}
	table := req.ProjectOrTable
	if table == "" {
		table = "incident"
	}
	if !validTable(table) {
		return Ticket{}, fmt.Errorf("servicenow: invalid table name %q", table)
	}

	payload, _ := json.Marshal(map[string]any{
		"short_description": req.Summary,
		"description":       req.Description,
		"urgency":           snPriority(req.Priority),
		"impact":            snPriority(req.Priority),
		"category":          "security",
	})

	endpoint := strings.TrimRight(req.BaseURL, "/") + "/api/now/table/" + table
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return Ticket{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", basicAuth(user, pass))

	resp, err := req.http().Do(httpReq)
	if err != nil {
		return Ticket{}, fmt.Errorf("servicenow: request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Ticket{}, statusError("servicenow", "create record", resp.StatusCode)
	}

	var out struct {
		Result struct {
			Number string `json:"number"`
			SysID  string `json:"sys_id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.Result.Number == "" {
		return Ticket{}, fmt.Errorf("servicenow: unexpected response: no record number")
	}
	url := strings.TrimRight(req.BaseURL, "/")
	if out.Result.SysID != "" {
		url += fmt.Sprintf("/nav_to.do?uri=%s.do?sys_id=%s", table, out.Result.SysID)
	}
	return Ticket{Provider: ProviderServiceNow, Key: out.Result.Number, URL: url}, nil
}

// validTable keeps the table name a single path segment: ServiceNow table names
// are letters, digits and underscores.
func validTable(t string) bool {
	for _, r := range t {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
			return false
		}
	}
	return t != ""
}
