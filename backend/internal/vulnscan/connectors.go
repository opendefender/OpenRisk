// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package vulnscan

import "github.com/opendefender/openrisk/internal/domain"

// ConnectorInfo describes a supported vulnerability integration for the UI.
type ConnectorInfo struct {
	Source   domain.VulnSource `json:"source"`
	Label    string            `json:"label"`
	Category string            `json:"category"` // network_scanner | edr | cloud
	Ingest   bool              `json:"ingest"`   // findings can be imported/normalised
	LivePull bool              `json:"live_pull"` // API polling implemented (vs import-only)
	Notes    string            `json:"notes"`
}

// Connectors returns the catalogue surfaced in the UI. Every provider supports
// normalised INGEST (upload/POST the tool's native findings). Live API polling
// is implemented for AWS Inspector (SDK already vendored); the others expose the
// same honest seam as the scanner — import works today, live pull activates when
// credentials + client are configured (never fabricated data).
func Connectors() []ConnectorInfo {
	return []ConnectorInfo{
		{domain.VulnSourceNessus, "Tenable Nessus", "network_scanner", true, false, "Import .nessus / vuln-export JSON."},
		{domain.VulnSourceOpenVAS, "OpenVAS / Greenbone", "network_scanner", true, false, "Import GMP results JSON."},
		{domain.VulnSourceQualys, "Qualys VMDR", "network_scanner", true, false, "Import VM detection JSON."},
		{domain.VulnSourceMSDefender, "Microsoft Defender for Endpoint", "edr", true, false, "Import TVM vulnerabilities."},
		{domain.VulnSourceAWSInspector, "AWS Inspector", "cloud", true, true, "Import findings or live-pull via SDK."},
		{domain.VulnSourceAzureDefender, "Microsoft Defender for Cloud", "cloud", true, false, "Import security sub-assessments."},
		{domain.VulnSourceCrowdStrike, "CrowdStrike Falcon Spotlight", "edr", true, false, "Import combined-vulnerabilities JSON."},
	}
}
