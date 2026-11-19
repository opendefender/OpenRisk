// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package collectors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/opendefender/openrisk/internal/domain"
	scanner "github.com/opendefender/openrisk/internal/scanner"
)

// GCP is a real Google Cloud SDK CloudCollector. It enumerates Compute Engine
// instances across all zones (aggregated list) for the project. Security Command
// Center findings are a documented follow-up (they require an org-level source
// path beyond a plain service account).
type GCP struct{}

// NewGCP returns the GCP cloud collector.
func NewGCP() scanner.CloudCollector { return GCP{} }

func (GCP) Collect(ctx context.Context, cfg scanner.ScanConfig, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery, errs chan<- error) {
	_ = findings // SCC findings are a documented follow-up.

	// The key is checked again here, not only when the config is created: a
	// config stored before GHSA-jxfm-p72g-w272 may carry an empty or foreign
	// key. An empty one must never reach the SDK, which would fall back to the
	// API pod's own identity (Application Default Credentials).
	saJSON := cfg.Credentials["service_account_json"]
	if err := scanner.CheckGCPServiceAccountKey(saJSON); err != nil {
		errs <- fmt.Errorf("gcp: %w", err)
		return
	}
	projectID := cfg.Credentials["project_id"]
	if projectID == "" {
		projectID = parseProjectID(saJSON)
	}
	if projectID == "" {
		errs <- fmt.Errorf("gcp: project_id missing (not in credentials nor service account JSON)")
		return
	}

	creds, err := gcpCredentials(saJSON)
	if err != nil {
		errs <- fmt.Errorf("gcp: credentials: %w", err)
		return
	}
	client, err := compute.NewInstancesRESTClient(ctx, option.WithAuthCredentials(creds))
	if err != nil {
		errs <- fmt.Errorf("gcp: compute client: %w", err)
		return
	}
	defer client.Close()

	it := client.AggregatedList(ctx, &computepb.AggregatedListInstancesRequest{Project: projectID})
	for {
		pair, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			errs <- fmt.Errorf("gcp: aggregated list: %w", err)
			return
		}
		for _, inst := range pair.Value.GetInstances() {
			assets <- gcpInstanceAsset(inst, pair.Key)
		}
	}
}

// gcpCredentials builds service-account credentials whose token exchange goes
// through the guarded egress client (#1315). The key file is tenant input and
// its token_uri is where the SDK POSTs a signed assertion: Collect has already
// held it to Google's endpoints, and the guarded client stays as the second
// line should a resolver ever point one of them inward.
func gcpCredentials(saJSON string) (*auth.Credentials, error) {
	return credentials.NewCredentialsFromJSON(credentials.ServiceAccount, []byte(saJSON), &credentials.DetectOptions{
		Scopes: compute.DefaultAuthScopes(),
		Client: egress.client(scanHTTPTimeout),
	})
}

func gcpInstanceAsset(inst *computepb.Instance, scope string) scanner.AssetDiscovery {
	zone := strings.TrimPrefix(scope, "zones/")
	tags := []string{"gcp", "compute"}
	env := ""
	for k, v := range inst.GetLabels() {
		tags = append(tags, fmt.Sprintf("%s:%s", k, v))
		if lk := strings.ToLower(k); lk == "environment" || lk == "env" {
			env = v
		}
	}

	a := scanner.AssetDiscovery{
		ExternalID:  fmt.Sprintf("gce:%s/%s", zone, inst.GetName()),
		Name:        inst.GetName(),
		Type:        domain.AssetTypeVM,
		Environment: env,
		Tags:        tags,
		Location:    ptr(zone),
		RawMetadata: map[string]any{"machine_type": lastPathSegment(inst.GetMachineType()), "status": inst.GetStatus()},
	}
	if nics := inst.GetNetworkInterfaces(); len(nics) > 0 && nics[0].GetNetworkIP() != "" {
		a.IP = ptr(nics[0].GetNetworkIP())
	}
	return a
}

func parseProjectID(saJSON string) string {
	if saJSON == "" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(saJSON), &m); err != nil {
		return ""
	}
	if p, ok := m["project_id"].(string); ok {
		return p
	}
	return ""
}

func lastPathSegment(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}
