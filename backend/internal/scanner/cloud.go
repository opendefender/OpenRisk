// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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

// --- Auto-discovery API providers ------------------------------------------
//
// These run in-process in the SaaS worker exactly like the cloud scanners (they
// reuse cloudScanner + the CloudCollector seam + the whole pipeline). Each
// carries its endpoint AND secrets in the encrypted credentials map, so nothing
// else in the plumbing changes. Pass nil for the collector to register the
// provider with live validation but no enumeration (honest empty preview).

// NewKubernetesScanner builds the Kubernetes cluster scanner.
//
// Required credentials: api_server (https URL), token (ServiceAccount bearer).
// Optional: ca_cert (PEM; omitted → TLS verification is skipped for self-signed
// clusters). The collector enumerates Nodes and Pods via the Kubernetes REST API.
func NewKubernetesScanner(collector CloudCollector) Scanner {
	return &cloudScanner{
		name:      "Kubernetes Scanner",
		provider:  domain.ProviderKubernetes,
		required:  []string{"api_server", "token"},
		collector: collector,
	}
}

// NewDockerScanner builds the Docker Engine scanner.
//
// Required credentials: host (tcp://host:2376 or unix socket path). Optional:
// ca_cert/client_cert/client_key (PEM) for mTLS. The collector enumerates
// containers and images via the Docker Engine API.
func NewDockerScanner(collector CloudCollector) Scanner {
	return &cloudScanner{
		name:      "Docker Scanner",
		provider:  domain.ProviderDocker,
		required:  []string{"host"},
		collector: collector,
	}
}

// NewVMwareScanner builds the VMware vCenter scanner.
//
// Required credentials: url (https://vcenter/sdk), username, password. The
// collector enumerates virtual machines via govmomi.
func NewVMwareScanner(collector CloudCollector) Scanner {
	return &cloudScanner{
		name:      "VMware vCenter Scanner",
		provider:  domain.ProviderVMware,
		required:  []string{"url", "username", "password"},
		collector: collector,
	}
}

// NewActiveDirectoryScanner builds the Active Directory scanner.
//
// Required credentials: url (ldap[s]://host:389), bind_dn, password, base_dn.
// The collector enumerates computer and user objects via LDAP.
func NewActiveDirectoryScanner(collector CloudCollector) Scanner {
	return &cloudScanner{
		name:      "Active Directory Scanner",
		provider:  domain.ProviderActiveDirectory,
		required:  []string{"url", "bind_dn", "password", "base_dn"},
		collector: collector,
	}
}

// NewM365Scanner builds the Microsoft 365 scanner.
//
// Required credentials: tenant_id, client_id, client_secret (an Entra ID app
// registration with Directory.Read.All / Device.Read.All application perms).
// The collector enumerates users and managed devices via Microsoft Graph.
func NewM365Scanner(collector CloudCollector) Scanner {
	return &cloudScanner{
		name:      "Microsoft 365 Scanner",
		provider:  domain.ProviderM365,
		required:  []string{"tenant_id", "client_id", "client_secret"},
		collector: collector,
	}
}

// NewGitHubScanner builds the GitHub scanner.
//
// Required credentials: token (a PAT or fine-grained token). Optional: base_url
// (GitHub Enterprise Server API root) and org (scope to one organisation). The
// collector enumerates repositories via the GitHub API.
func NewGitHubScanner(collector CloudCollector) Scanner {
	return &cloudScanner{
		name:      "GitHub Scanner",
		provider:  domain.ProviderGitHub,
		required:  []string{"token"},
		collector: collector,
	}
}

// NewGitLabScanner builds the GitLab scanner.
//
// Required credentials: token (a personal/group access token). Optional:
// base_url (self-managed GitLab, default https://gitlab.com). The collector
// enumerates projects via the GitLab API.
func NewGitLabScanner(collector CloudCollector) Scanner {
	return &cloudScanner{
		name:      "GitLab Scanner",
		provider:  domain.ProviderGitLab,
		required:  []string{"token"},
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
