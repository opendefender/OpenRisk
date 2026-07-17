// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package collectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"

	"github.com/opendefender/openrisk/internal/domain"
	scanner "github.com/opendefender/openrisk/internal/scanner"
)

const (
	graphBaseURL = "https://graph.microsoft.com/v1.0"
	graphScope   = "https://graph.microsoft.com/.default"
)

// httpDoer is the minimal HTTP surface the Graph enumeration needs, so the
// pagination/normalisation logic is unit-testable with an httptest server while
// the real path uses http.DefaultClient. *http.Client satisfies it.
type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// M365 is a real Microsoft Graph CloudCollector. It authenticates with an Entra
// ID app registration (client-credentials via azidentity) and enumerates users
// (Identity assets) and managed devices (Workstation assets), flagging
// non-compliant managed devices.
type M365 struct{}

// NewM365 returns the Microsoft 365 collector.
func NewM365() scanner.CloudCollector { return M365{} }

func (M365) Collect(ctx context.Context, cfg scanner.ScanConfig, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery, errs chan<- error) {
	cred, err := azidentity.NewClientSecretCredential(
		cfg.Credentials["tenant_id"], cfg.Credentials["client_id"], cfg.Credentials["client_secret"], nil)
	if err != nil {
		errs <- fmt.Errorf("m365: credential: %w", err)
		return
	}
	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{graphScope}})
	if err != nil {
		errs <- fmt.Errorf("m365: acquire token: %w", err)
		return
	}
	collectGraph(ctx, http.DefaultClient, graphBaseURL, tok.Token, assets, findings, errs)
}

// graphList is a paginated Microsoft Graph collection response.
type graphList struct {
	Value    []map[string]any `json:"value"`
	NextLink string           `json:"@odata.nextLink"`
}

// collectGraph enumerates users and devices from a Graph endpoint using any
// httpDoer and bearer token.
func collectGraph(ctx context.Context, client httpDoer, baseURL, token string, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery, errs chan<- error) {
	usersURL := baseURL + "/users?$select=id,displayName,userPrincipalName,accountEnabled,userType&$top=100"
	if err := graphPaged(ctx, client, usersURL, token, func(v map[string]any) { emitGraphUser(v, assets) }); err != nil {
		errs <- fmt.Errorf("m365: users: %w", err)
	}
	devicesURL := baseURL + "/devices?$select=id,displayName,operatingSystem,operatingSystemVersion,isCompliant,isManaged,trustType&$top=100"
	if err := graphPaged(ctx, client, devicesURL, token, func(v map[string]any) { emitGraphDevice(v, assets, findings) }); err != nil {
		errs <- fmt.Errorf("m365: devices: %w", err)
	}
}

// graphPaged walks a Graph collection following @odata.nextLink, invoking fn per item.
func graphPaged(ctx context.Context, client httpDoer, url, token string, fn func(map[string]any)) error {
	for url != "" {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("graph %s: status %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
		}
		var page graphList
		if err := json.Unmarshal(body, &page); err != nil {
			return err
		}
		for _, v := range page.Value {
			fn(v)
		}
		url = page.NextLink
	}
	return nil
}

func emitGraphUser(v map[string]any, assets chan<- scanner.AssetDiscovery) {
	id := gstr(v, "id")
	name := gstr(v, "displayName")
	upn := gstr(v, "userPrincipalName")
	if name == "" {
		name = upn
	}
	tags := []string{"m365", "identity"}
	if ut := gstr(v, "userType"); ut != "" {
		tags = append(tags, strings.ToLower(ut))
	}
	if enabled, ok := v["accountEnabled"].(bool); ok && !enabled {
		tags = append(tags, "disabled")
	}
	assets <- scanner.AssetDiscovery{
		ExternalID:  "m365:user:" + id,
		Name:        name,
		Type:        domain.AssetTypeIdentity,
		Tags:        tags,
		RawMetadata: map[string]any{"upn": upn, "user_type": gstr(v, "userType")},
	}
}

func emitGraphDevice(v map[string]any, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery) {
	id := gstr(v, "id")
	name := gstr(v, "displayName")
	os := gstr(v, "operatingSystem")
	externalID := "m365:device:" + id
	a := scanner.AssetDiscovery{
		ExternalID:  externalID,
		Name:        name,
		Type:        domain.AssetTypeWorkstation,
		Tags:        []string{"m365", "device"},
		RawMetadata: map[string]any{"operating_system": os, "trust_type": gstr(v, "trustType")},
	}
	if os != "" {
		a.OS = ptr(os)
		if ver := gstr(v, "operatingSystemVersion"); ver != "" {
			a.OSVersion = ptr(ver)
		}
	}
	assets <- a

	managed, _ := v["isManaged"].(bool)
	compliant, hasCompliant := v["isCompliant"].(bool)
	if managed && hasCompliant && !compliant {
		findings <- scanner.FindingDiscovery{
			Title:           "Non-compliant managed device",
			Description:     fmt.Sprintf("Device %q is Intune-managed but reports non-compliant with policy.", name),
			Severity:        scanner.SeverityMedium,
			Evidence:        "isManaged=true, isCompliant=false",
			RemediationHint: "Investigate the device's compliance state in Intune and remediate the failing policies.",
			Source:          "m365",
			AssetExternalID: externalID,
		}
	}
}

// gstr reads a string field from a Graph JSON object, tolerating absence/null.
func gstr(m map[string]any, key string) string {
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}
