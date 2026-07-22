// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package collectors

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"

	"github.com/opendefender/openrisk/internal/domain"
	scanner "github.com/opendefender/openrisk/internal/scanner"
)

// Azure is a real Azure SDK CloudCollector. It enumerates the subscription's
// resources (VMs, App Services, SQL databases, Storage Accounts, Key Vaults, …)
// via a single Resource Graph KQL query — the same approach the Azure portal
// uses for cross-resource inventory.
type Azure struct{}

// NewAzure returns the Azure cloud collector.
func NewAzure() scanner.CloudCollector { return Azure{} }

const azureResourceQuery = `Resources | project id, name, type, location, tags, kind | limit 2000`

func (Azure) Collect(ctx context.Context, cfg scanner.ScanConfig, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery, errs chan<- error) {
	_ = findings // Defender-for-Cloud alerts are a documented follow-up; assets first.

	cred, err := azidentity.NewClientSecretCredential(
		cfg.Credentials["tenant_id"], cfg.Credentials["client_id"], cfg.Credentials["client_secret"], nil,
	)
	if err != nil {
		errs <- fmt.Errorf("azure: credential: %w", err)
		return
	}
	client, err := armresourcegraph.NewClient(cred, nil)
	if err != nil {
		errs <- fmt.Errorf("azure: resource graph client: %w", err)
		return
	}

	sub := cfg.Credentials["subscription_id"]
	resp, err := client.Resources(ctx, armresourcegraph.QueryRequest{
		Query:         to.Ptr(azureResourceQuery),
		Subscriptions: []*string{to.Ptr(sub)},
	}, nil)
	if err != nil {
		errs <- fmt.Errorf("azure: resource graph query: %w", err)
		return
	}

	rows, ok := resp.Data.([]any)
	if !ok {
		return
	}
	for _, r := range rows {
		row, ok := r.(map[string]any)
		if !ok {
			continue
		}
		assets <- azureAsset(row)
	}
}

func azureAsset(row map[string]any) scanner.AssetDiscovery {
	id := asString(row["id"])
	name := asString(row["name"])
	rtype := asString(row["type"])
	loc := asString(row["location"])

	tags := []string{"azure"}
	env := ""
	if tm, ok := row["tags"].(map[string]any); ok {
		for k, v := range tm {
			tags = append(tags, fmt.Sprintf("%s:%s", k, asString(v)))
			if lk := strings.ToLower(k); lk == "environment" || lk == "env" {
				env = asString(v)
			}
		}
	}

	a := scanner.AssetDiscovery{
		ExternalID:  id,
		Name:        firstNonEmpty(name, id),
		Type:        azureType(rtype),
		Environment: env,
		Tags:        tags,
		RawMetadata: map[string]any{"azure_type": rtype, "kind": asString(row["kind"])},
	}
	if loc != "" {
		a.Location = ptr(loc)
	}
	if cpe := azureCPE(rtype); len(cpe) > 0 {
		a.CPE = cpe
	}
	return a
}

func azureType(rtype string) domain.AssetType {
	t := strings.ToLower(rtype)
	switch {
	case strings.Contains(t, "virtualmachines"):
		return domain.AssetTypeVM
	case strings.Contains(t, "storageaccounts"):
		return domain.AssetTypeStorage
	case strings.Contains(t, "sql/servers"), strings.Contains(t, "databases"), strings.Contains(t, "cosmosdb"):
		return domain.AssetTypeDatabase
	case strings.Contains(t, "sites"), strings.Contains(t, "serverfarms"):
		return domain.AssetTypeServer
	case strings.Contains(t, "vaults"):
		return domain.AssetTypeIdentity
	case strings.Contains(t, "networksecuritygroups"), strings.Contains(t, "virtualnetworks"), strings.Contains(t, "publicip"):
		return domain.AssetTypeNetwork
	case strings.Contains(t, "containerservice"), strings.Contains(t, "managedclusters"):
		return domain.AssetTypeContainer
	default:
		return domain.AssetTypeUnknown
	}
}

func azureCPE(rtype string) []string {
	if strings.Contains(strings.ToLower(rtype), "virtualmachines") {
		return []string{"cpe:2.3:o:microsoft:windows"}
	}
	return nil
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
