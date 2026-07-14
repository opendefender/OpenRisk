// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package scanner

import (
	"context"
	"fmt"

	"github.com/opendefender/openrisk/internal/domain"
)

// CloudCollector performs the provider-specific API enumeration for a cloud
// scan. It is the seam where the official SDK integration plugs in: the
// cloudScanner owns validation, channel lifecycle and pipeline integration,
// while a CloudCollector owns "talk to the cloud API and emit discoveries".
//
// A collector receives an ALREADY-DECRYPTED ScanConfig (credentials in
// cfg.Credentials) and streams discoveries/errors on the provided channels. It
// MUST NOT close them — the cloudScanner owns their lifecycle. It MUST honour
// ctx cancellation (the caller enforces a deadline).
type CloudCollector interface {
	Collect(ctx context.Context, cfg ScanConfig, assets chan<- AssetDiscovery, findings chan<- FindingDiscovery, errs chan<- error)
}

// cloudScanner is the shared Scanner implementation for aws/azure/gcp. Each
// provider differs only in its human name, required credential keys, and the
// CloudCollector that does the enumeration.
type cloudScanner struct {
	name      string
	provider  domain.ScannerProvider
	required  []string // credential keys that must be present & non-empty
	collector CloudCollector
}

func (s *cloudScanner) Name() string       { return s.name }
func (s *cloudScanner) Provider() string   { return string(s.provider) }
func (s *cloudScanner) IsAgentBased() bool { return false }

// Validate ensures the decrypted credentials carry every required key. It never
// logs the values — only which key is missing.
func (s *cloudScanner) Validate(_ context.Context, cfg ScanConfig) error {
	if cfg.Provider != s.provider {
		return domain.NewValidationError(fmt.Sprintf("config provider %q does not match scanner %q", cfg.Provider, s.provider))
	}
	for _, k := range s.required {
		if v, ok := cfg.Credentials[k]; !ok || v == "" {
			return domain.NewValidationError(fmt.Sprintf("%s scanner: missing required credential %q", s.provider, k))
		}
	}
	return nil
}

// Scan spins the collector on a goroutine and owns the three channels' lifecycle
// (they are all closed when Collect returns). The pipeline drains them.
func (s *cloudScanner) Scan(ctx context.Context, cfg ScanConfig) (<-chan AssetDiscovery, <-chan FindingDiscovery, <-chan error) {
	assets := make(chan AssetDiscovery, 64)
	findings := make(chan FindingDiscovery, 64)
	errs := make(chan error, 8)

	collector := s.collector
	if collector == nil {
		collector = unavailableCollector{provider: s.provider}
	}

	go func() {
		defer close(assets)
		defer close(findings)
		defer close(errs)
		collector.Collect(ctx, cfg, assets, findings, errs)
	}()

	return assets, findings, errs
}

// --- Provider constructors -------------------------------------------------
//
// Pass nil for the collector to register a scanner whose credential validation
// and pipeline wiring are live but whose enumeration is not yet connected to the
// SDK (Scan then emits a single, clear "collector not configured" error and an
// empty preview — honest, never a fake result). Pass a real CloudCollector to
// enable live enumeration.

// NewAWSScanner builds the AWS cloud scanner.
//
// Required credentials: access_key_id, secret_access_key (session_token and
// region optional; regions come from cfg.Regions). The reference collector is
// expected to enumerate, via aws-sdk-go-v2:
//   - EC2 DescribeInstances (all regions) → instances, IPs, AMI, tags→criticality, SGs
//   - RDS/Aurora/DynamoDB DescribeDBInstances + DescribeDBClusters
//   - S3 ListBuckets + GetBucketPolicy + GetBucketEncryption (misconfigs → findings)
//   - IAM GetAccountAuthorizationDetails + SimulatePrincipalPolicy (excessive rights)
//   - Security Hub GetFindings (severity ≥ medium) → FindingDiscovery
//   - VPC/SecurityGroups/NetworkACLs/WAF, EKS/ECS/Lambda
//   - CPE from Platform+AMI+OS+installed software (SSM Inventory if enabled)
func NewAWSScanner(collector CloudCollector) Scanner {
	return &cloudScanner{
		name:      "AWS Cloud Scanner",
		provider:  domain.ProviderAWS,
		required:  []string{"access_key_id", "secret_access_key"},
		collector: collector,
	}
}

// NewAzureScanner builds the Azure cloud scanner.
//
// Required credentials: tenant_id, client_id, client_secret, subscription_id.
// The reference collector is expected to enumerate:
//   - KQL Resource Graph: VMs, App Services, SQL Databases, Storage Accounts, Key Vaults
//   - Defender for Cloud: alerts/findings
//   - Network: NSG rules, Firewall, Private Endpoints
//   - IAM: role assignments + conditional access
//   - CPE from image reference + extensions
func NewAzureScanner(collector CloudCollector) Scanner {
	return &cloudScanner{
		name:      "Azure Cloud Scanner",
		provider:  domain.ProviderAzure,
		required:  []string{"tenant_id", "client_id", "client_secret", "subscription_id"},
		collector: collector,
	}
}

// NewGCPScanner builds the GCP cloud scanner.
//
// Required credentials: service_account_json (project_id optional). The
// reference collector is expected to enumerate:
//   - Compute Engine ListInstances, ListDisks
//   - Cloud SQL / Spanner / Firestore
//   - Security Command Center ListFindings
//   - VPC + Firewall rules
//   - IAM Policy Analyzer
func NewGCPScanner(collector CloudCollector) Scanner {
	return &cloudScanner{
		name:      "GCP Cloud Scanner",
		provider:  domain.ProviderGCP,
		required:  []string{"service_account_json"},
		collector: collector,
	}
}

// unavailableCollector is the default when no SDK collector is wired. It emits a
// single, clear error and no discoveries — so a scan against an unconfigured
// provider produces an honest empty preview with the reason attached, never a
// fabricated result.
type unavailableCollector struct{ provider domain.ScannerProvider }

func (u unavailableCollector) Collect(_ context.Context, _ ScanConfig, _ chan<- AssetDiscovery, _ chan<- FindingDiscovery, errs chan<- error) {
	errs <- fmt.Errorf("%s live enumeration is not configured on this deployment: no cloud SDK collector wired", u.provider)
}
