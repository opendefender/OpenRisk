// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package livepull

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"

	"github.com/opendefender/openrisk/internal/domain"
)

// firstNonEmpty returns the first non-empty argument.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// ---- Microsoft Defender for Endpoint (Threat & Vulnerability Mgmt) ---------

type msDefenderPuller struct{}

func (msDefenderPuller) LivePullSupported() bool { return true }

func (msDefenderPuller) Pull(ctx context.Context, cfg PullConfig) ([]map[string]any, error) {
	tenant := cfg.cred("tenant_id", "tenant")
	clientID := cfg.cred("client_id")
	secret := cfg.cred("client_secret", "secret")
	if tenant == "" {
		return nil, errMissingCred("tenant_id")
	}
	if clientID == "" {
		return nil, errMissingCred("client_id")
	}
	if secret == "" {
		return nil, errMissingCred("client_secret")
	}

	tokenURL := firstNonEmpty(cfg.cred("token_url"),
		fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/token", tenant))
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", secret)
	form.Set("resource", "https://api.securitycenter.microsoft.com")
	token, err := oauthClientCredentials(ctx, cfg.http(), tokenURL, form)
	if err != nil {
		return nil, err
	}

	apiBase := firstNonEmpty(cfg.BaseURL, "https://api.securitycenter.microsoft.com")
	body, err := getJSON(ctx, cfg.http(), strings.TrimRight(apiBase, "/")+"/api/vulnerabilities",
		map[string]string{"Authorization": "Bearer " + token})
	if err != nil {
		return nil, err
	}
	return asObjects(body["value"]), nil
}

// ---- CrowdStrike Falcon Spotlight ------------------------------------------

type crowdStrikePuller struct{}

func (crowdStrikePuller) LivePullSupported() bool { return true }

func (crowdStrikePuller) Pull(ctx context.Context, cfg PullConfig) ([]map[string]any, error) {
	clientID := cfg.cred("client_id")
	secret := cfg.cred("client_secret", "secret")
	if clientID == "" {
		return nil, errMissingCred("client_id")
	}
	if secret == "" {
		return nil, errMissingCred("client_secret")
	}
	apiBase := strings.TrimRight(firstNonEmpty(cfg.BaseURL, "https://api.crowdstrike.com"), "/")

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", secret)
	token, err := oauthClientCredentials(ctx, cfg.http(), firstNonEmpty(cfg.cred("token_url"), apiBase+"/oauth2/token"), form)
	if err != nil {
		return nil, err
	}

	body, err := getJSON(ctx, cfg.http(),
		apiBase+"/spotlight/combined/vulnerabilities/v1?limit=400&filter="+url.QueryEscape("status:'open'"),
		map[string]string{"Authorization": "Bearer " + token})
	if err != nil {
		return nil, err
	}
	return asObjects(body["resources"]), nil
}

// ---- Tenable Nessus / Tenable.io -------------------------------------------

type nessusPuller struct{}

func (nessusPuller) LivePullSupported() bool { return true }

func (nessusPuller) Pull(ctx context.Context, cfg PullConfig) ([]map[string]any, error) {
	access := cfg.cred("access_key", "accessKey")
	secret := cfg.cred("secret_key", "secretKey")
	if access == "" {
		return nil, errMissingCred("access_key")
	}
	if secret == "" {
		return nil, errMissingCred("secret_key")
	}
	apiBase := strings.TrimRight(firstNonEmpty(cfg.BaseURL, "https://cloud.tenable.com"), "/")

	body, err := getJSON(ctx, cfg.http(), apiBase+"/workbenches/vulnerabilities",
		map[string]string{"X-ApiKeys": fmt.Sprintf("accessKey=%s;secretKey=%s", access, secret)})
	if err != nil {
		return nil, err
	}
	// Tenable.io returns {vulnerabilities:[...]}; Nessus Pro exports are also arrays.
	if v := asObjects(body["vulnerabilities"]); len(v) > 0 {
		return v, nil
	}
	return asObjects(body["findings"]), nil
}

// ---- Qualys VMDR (XML) -----------------------------------------------------

type qualysPuller struct{}

func (qualysPuller) LivePullSupported() bool { return true }

// qualysXML mirrors the fragment of the VM detection output we consume.
type qualysXML struct {
	Response struct {
		HostList struct {
			Hosts []struct {
				IP        string `xml:"IP"`
				DNS       string `xml:"DNS"`
				Detections struct {
					Detection []struct {
						QID      string `xml:"QID"`
						Severity string `xml:"SEVERITY"`
						Results  string `xml:"RESULTS"`
						Title    string `xml:"TITLE"`
					} `xml:"DETECTION"`
				} `xml:"DETECTION_LIST"`
			} `xml:"HOST"`
		} `xml:"HOST_LIST"`
	} `xml:"RESPONSE"`
}

func (qualysPuller) Pull(ctx context.Context, cfg PullConfig) ([]map[string]any, error) {
	user := cfg.cred("username", "user")
	pass := cfg.cred("password", "pass")
	if user == "" {
		return nil, errMissingCred("username")
	}
	if pass == "" {
		return nil, errMissingCred("password")
	}
	if cfg.BaseURL == "" {
		return nil, errMissingCred("base_url") // Qualys pod URL is mandatory (differs per subscription)
	}
	endpoint := strings.TrimRight(cfg.BaseURL, "/") +
		"/api/2.0/fo/asset/host/vm/detection/?action=list&show_results=1"
	raw, err := getRaw(ctx, cfg.http(), endpoint, map[string]string{
		"Authorization":    basicAuth(user, pass),
		"X-Requested-With": "OpenRisk",
	})
	if err != nil {
		return nil, err
	}

	var parsed qualysXML
	if err := xml.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("qualys XML parse failed: %w", err)
	}
	var out []map[string]any
	for _, h := range parsed.Response.HostList.Hosts {
		host := firstNonEmpty(h.DNS, h.IP)
		for _, d := range h.Detections.Detection {
			out = append(out, map[string]any{
				"QID":       d.QID,
				"SEVERITY":  d.Severity,
				"TITLE":     firstNonEmpty(d.Title, "Qualys QID "+d.QID),
				"DIAGNOSIS": d.Results,
				"DNS":       h.DNS,
				"IP":        h.IP,
				"host":      host,
			})
		}
	}
	return out, nil
}

// ---- Microsoft Defender for Cloud (Azure sub-assessments) ------------------

type azureDefenderPuller struct{}

func (azureDefenderPuller) LivePullSupported() bool { return true }

func (azureDefenderPuller) Pull(ctx context.Context, cfg PullConfig) ([]map[string]any, error) {
	tenant := cfg.cred("tenant_id", "tenant")
	clientID := cfg.cred("client_id")
	secret := cfg.cred("client_secret", "secret")
	sub := cfg.cred("subscription_id", "subscription")
	if tenant == "" {
		return nil, errMissingCred("tenant_id")
	}
	if clientID == "" {
		return nil, errMissingCred("client_id")
	}
	if secret == "" {
		return nil, errMissingCred("client_secret")
	}
	if sub == "" {
		return nil, errMissingCred("subscription_id")
	}

	tokenURL := firstNonEmpty(cfg.cred("token_url"),
		fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/token", tenant))
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", secret)
	form.Set("resource", "https://management.azure.com/")
	token, err := oauthClientCredentials(ctx, cfg.http(), tokenURL, form)
	if err != nil {
		return nil, err
	}

	armBase := strings.TrimRight(firstNonEmpty(cfg.BaseURL, "https://management.azure.com"), "/")
	endpoint := fmt.Sprintf("%s/subscriptions/%s/providers/Microsoft.Security/subAssessments?api-version=2019-01-01-preview", armBase, sub)
	body, err := getJSON(ctx, cfg.http(), endpoint, map[string]string{"Authorization": "Bearer " + token})
	if err != nil {
		return nil, err
	}
	// Flatten each sub-assessment's `properties` (where displayName/status/
	// additionalData/resourceDetails live) into the shape the Azure normaliser reads.
	var out []map[string]any
	for _, item := range asObjects(body["value"]) {
		flat := map[string]any{}
		if props, ok := item["properties"].(map[string]any); ok {
			for k, v := range props {
				flat[k] = v
			}
		}
		if id, ok := item["id"].(string); ok {
			flat["id"] = id
		}
		out = append(out, flat)
	}
	return out, nil
}

// ---- honest seam -----------------------------------------------------------

type seamPuller struct{ reason string }

func (seamPuller) LivePullSupported() bool { return false }

func (s seamPuller) Pull(ctx context.Context, cfg PullConfig) ([]map[string]any, error) {
	return nil, domain.NewValidationError("live pull not available: " + s.reason)
}
