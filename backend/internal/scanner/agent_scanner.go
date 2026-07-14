// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package scanner

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/opendefender/openrisk/internal/domain"
)

// MinAllowedPrefixIPv4 is the smallest CIDR block (largest network) an
// agent-based scan may target: /24. Anything wider is rejected both here
// (config-time) and on the Agent (scan-time). Master Prompt V5.
const MinAllowedPrefixIPv4 = 24

// agentScanner represents the on-prem providers ("nmap", "agent"). Their Scan is
// executed by the Agent, never in this backend — so Scan here is a guard that
// returns an error. What this type actually contributes is Validate: the
// server-side target-scope check performed when a ScanConfig is created.
type agentScanner struct {
	name     string
	provider domain.ScannerProvider
}

func newAgentScanner(name string, provider domain.ScannerProvider) *agentScanner {
	return &agentScanner{name: name, provider: provider}
}

// NewNmapScanner / NewAgentScanner register the two agent-based providers.
func NewNmapScanner() Scanner { return newAgentScanner("Nmap On-Prem Scanner", domain.ProviderNmap) }
func NewAgentScanner() Scanner {
	return newAgentScanner("OpenRisk Agent Scanner", domain.ProviderAgent)
}

func (s *agentScanner) Name() string       { return s.name }
func (s *agentScanner) Provider() string   { return string(s.provider) }
func (s *agentScanner) IsAgentBased() bool { return true }

// Validate enforces the target-scope rule: every target must be a single host or
// a CIDR no wider than /24. Empty target lists are rejected — an agent scan with
// no scope is a mistake, not a "scan everything".
func (s *agentScanner) Validate(_ context.Context, cfg ScanConfig) error {
	if len(cfg.Targets) == 0 {
		return domain.NewValidationError("agent scan requires at least one target (CIDR ≤ /24 or host)")
	}
	for _, t := range cfg.Targets {
		if err := ValidateTarget(t); err != nil {
			return err
		}
	}
	return nil
}

// Scan is never executed for agent-based providers — the Agent runs nmap/osquery
// on-prem and pushes results. Returning an error keeps the invariant explicit if
// anything ever tries to run it in-process.
func (s *agentScanner) Scan(_ context.Context, _ ScanConfig) (<-chan AssetDiscovery, <-chan FindingDiscovery, <-chan error) {
	assets := make(chan AssetDiscovery)
	findings := make(chan FindingDiscovery)
	errs := make(chan error, 1)
	errs <- fmt.Errorf("%s is agent-based: nmap/osquery run on the on-prem Agent, never in the SaaS backend", s.provider)
	close(assets)
	close(findings)
	close(errs)
	return assets, findings, errs
}

// ValidateTarget checks a single agent target: either a bare IP/hostname or a
// CIDR of prefix ≥ /24 (IPv4) / ≥ /120 (IPv6, the equivalent 256-address block).
func ValidateTarget(t string) error {
	t = strings.TrimSpace(t)
	if t == "" {
		return domain.NewValidationError("empty target")
	}
	if strings.Contains(t, "/") {
		ip, ipnet, err := net.ParseCIDR(t)
		if err != nil {
			return domain.NewValidationError(fmt.Sprintf("invalid CIDR %q: %v", t, err))
		}
		ones, _ := ipnet.Mask.Size()
		if ip.To4() != nil {
			if ones < MinAllowedPrefixIPv4 {
				return domain.NewValidationError(fmt.Sprintf("target %q is wider than /24 — refused (scope limit)", t))
			}
		} else if ones < 120 {
			return domain.NewValidationError(fmt.Sprintf("IPv6 target %q is wider than /120 — refused (scope limit)", t))
		}
		return nil
	}
	// Bare host: accept an IP or a plausible hostname.
	if net.ParseIP(t) != nil {
		return nil
	}
	if isHostname(t) {
		return nil
	}
	return domain.NewValidationError(fmt.Sprintf("invalid target %q: not an IP, hostname or CIDR", t))
}

func isHostname(h string) bool {
	if len(h) == 0 || len(h) > 253 {
		return false
	}
	for _, label := range strings.Split(h, ".") {
		if label == "" || len(label) > 63 {
			return false
		}
		for _, r := range label {
			if !(r == '-' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
				return false
			}
		}
	}
	return true
}
