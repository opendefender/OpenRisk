// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

// Package vulnscan is the vulnerability-management integration layer. Each
// supported product (Nessus, OpenVAS, Qualys, Microsoft Defender, AWS Inspector,
// Azure Defender, CrowdStrike) has a normaliser that maps its native finding
// JSON onto a provider-agnostic NormalizedFinding. Ingest → normalise → prioritise
// → upsert lives in application/vulnerability; this package is pure mapping, no I/O.
package vulnscan

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/opendefender/openrisk/internal/domain"
)

// NormalizedFinding is the common shape produced by every connector.
type NormalizedFinding struct {
	CVEID            string
	Title            string
	Description      string
	CVSSScore        float64
	CVSSVector       string
	Severity         string // may be empty → derived from CVSS by the use case
	EPSS             float64
	KEV              bool
	ExploitAvailable bool
	ExploitMaturity  string
	ExternalID       string // the source tool's own finding id (dedup key)
	AssetName        string // hostname / resource id from the tool
	AssetExternalID  string // used to match an existing asset, when known
	RemediationHint  string
	Raw              map[string]any
}

// Normalizer converts one raw finding (decoded JSON object) into a NormalizedFinding.
type Normalizer func(raw map[string]any) NormalizedFinding

var normalizers = map[domain.VulnSource]Normalizer{
	domain.VulnSourceNessus:        normalizeNessus,
	domain.VulnSourceOpenVAS:       normalizeOpenVAS,
	domain.VulnSourceQualys:        normalizeQualys,
	domain.VulnSourceMSDefender:    normalizeMSDefender,
	domain.VulnSourceAWSInspector:  normalizeAWSInspector,
	domain.VulnSourceAzureDefender: normalizeAzureDefender,
	domain.VulnSourceCrowdStrike:   normalizeCrowdStrike,
	domain.VulnSourceManual:        normalizeGeneric,
	domain.VulnSourceScanner:       normalizeGeneric,
}

// SupportsNormalization reports whether a source has a normaliser.
func SupportsNormalization(src domain.VulnSource) bool {
	_, ok := normalizers[src]
	return ok
}

// Normalize maps a single raw finding for the given source. Unknown sources fall
// back to the generic normaliser (best-effort field guessing).
func Normalize(src domain.VulnSource, raw map[string]any) NormalizedFinding {
	fn, ok := normalizers[src]
	if !ok {
		fn = normalizeGeneric
	}
	nf := fn(raw)
	nf.Raw = raw
	return nf
}

// NormalizeBatch maps a slice of raw findings.
func NormalizeBatch(src domain.VulnSource, raws []map[string]any) []NormalizedFinding {
	out := make([]NormalizedFinding, 0, len(raws))
	for _, r := range raws {
		out = append(out, Normalize(src, r))
	}
	return out
}

var cveRe = regexp.MustCompile(`(?i)CVE-\d{4}-\d{4,7}`)

// ---- provider normalisers -------------------------------------------------

// Nessus (Tenable) export / vuln API.
func normalizeNessus(r map[string]any) NormalizedFinding {
	nf := NormalizedFinding{
		Title:           firstStr(r, "plugin_name", "pluginName", "name", "synopsis"),
		Description:     firstStr(r, "description", "synopsis"),
		CVSSScore:       firstFloat(r, "cvss3_base_score", "cvssV3BaseScore", "cvss_base_score", "cvss_score"),
		CVSSVector:      firstStr(r, "cvss3_vector", "cvss_vector"),
		Severity:        nessusSeverity(r),
		ExternalID:      firstStr(r, "plugin_id", "pluginID", "uuid"),
		AssetName:       firstStr(r, "host", "hostname", "host-fqdn", "host_ip"),
		RemediationHint: firstStr(r, "solution", "remediation"),
	}
	nf.CVEID = firstCVE(r, "cve")
	return nf
}

// nessusSeverity: Nessus uses 0–4 (0 Info … 4 Critical), sometimes a word.
func nessusSeverity(r map[string]any) string {
	if s := firstStr(r, "severity_name", "risk_factor"); s != "" {
		return strings.ToLower(s)
	}
	switch int(firstFloat(r, "severity")) {
	case 4:
		return "critical"
	case 3:
		return "high"
	case 2:
		return "medium"
	case 1:
		return "low"
	}
	return ""
}

// OpenVAS / Greenbone GVM result.
func normalizeOpenVAS(r map[string]any) NormalizedFinding {
	nf := NormalizedFinding{
		Title:           firstStr(r, "name", "nvt_name"),
		Description:     firstStr(r, "description", "summary"),
		CVSSScore:       firstFloat(r, "severity", "cvss", "cvss_base"),
		Severity:        strings.ToLower(firstStr(r, "threat")),
		ExternalID:      firstStr(r, "oid", "nvt_oid", "id"),
		AssetName:       firstStr(r, "host", "hostname"),
		RemediationHint: firstStr(r, "solution"),
	}
	nf.CVEID = firstCVE(r, "cve", "cves")
	return nf
}

// Qualys VMDR.
func normalizeQualys(r map[string]any) NormalizedFinding {
	nf := NormalizedFinding{
		Title:           firstStr(r, "TITLE", "title"),
		Description:     firstStr(r, "DIAGNOSIS", "THREAT", "description"),
		CVSSScore:       firstFloat(r, "CVSS_BASE", "CVSS", "cvss"),
		Severity:        qualysSeverity(r),
		ExternalID:      firstStr(r, "QID", "qid"),
		AssetName:       firstStr(r, "DNS", "IP", "dns", "ip"),
		RemediationHint: firstStr(r, "SOLUTION", "solution"),
	}
	nf.CVEID = firstCVE(r, "CVE_ID", "CVE_ID_LIST", "cve_id", "cve")
	return nf
}

// qualysSeverity: Qualys uses 1–5.
func qualysSeverity(r map[string]any) string {
	switch int(firstFloat(r, "SEVERITY", "severity")) {
	case 5:
		return "critical"
	case 4:
		return "high"
	case 3:
		return "medium"
	case 2:
		return "low"
	case 1:
		return "info"
	}
	return ""
}

// Microsoft Defender for Endpoint (TVM vulnerabilities).
func normalizeMSDefender(r map[string]any) NormalizedFinding {
	nf := NormalizedFinding{
		Title:            firstStr(r, "name", "id"),
		Description:      firstStr(r, "description"),
		CVSSScore:        firstFloat(r, "cvssV3", "cvss", "cvssScore"),
		Severity:         strings.ToLower(firstStr(r, "severity")),
		ExploitAvailable: firstBool(r, "publicExploit", "exploitVerified", "exploitInKit"),
		ExternalID:       firstStr(r, "id"),
		AssetName:        firstStr(r, "deviceName", "computerDnsName"),
	}
	nf.AffectedFromCount(int(firstFloat(r, "exposedMachinesCount", "exposedMachines")))
	nf.CVEID = firstCVE(r, "id", "cveId")
	return nf
}

// AWS Inspector (inspector2 finding).
func normalizeAWSInspector(r map[string]any) NormalizedFinding {
	nf := NormalizedFinding{
		Title:            firstStr(r, "title", "description"),
		Description:      firstStr(r, "description"),
		CVSSScore:        firstFloat(r, "inspectorScore", "cvss", "cvssScore"),
		Severity:         strings.ToLower(firstStr(r, "severity")),
		ExploitAvailable: strings.EqualFold(firstStr(r, "exploitAvailable"), "YES"),
		ExternalID:       firstStr(r, "findingArn", "arn", "id"),
	}
	// packageVulnerabilityDetails.vulnerabilityId holds the CVE; resources[0].id the asset.
	if pvd, ok := r["packageVulnerabilityDetails"].(map[string]any); ok {
		nf.CVEID = firstCVE(pvd, "vulnerabilityId", "cve")
	}
	if nf.CVEID == "" {
		nf.CVEID = firstCVE(r, "vulnerabilityId", "cve")
	}
	if res, ok := r["resources"].([]any); ok && len(res) > 0 {
		if r0, ok := res[0].(map[string]any); ok {
			nf.AssetExternalID = firstStr(r0, "id")
			nf.AssetName = firstStr(r0, "id")
		}
	}
	if rem, ok := r["remediation"].(map[string]any); ok {
		if rec, ok := rem["recommendation"].(map[string]any); ok {
			nf.RemediationHint = firstStr(rec, "text")
		}
	}
	return nf
}

// Azure Defender for Cloud (assessment / sub-assessment).
func normalizeAzureDefender(r map[string]any) NormalizedFinding {
	nf := NormalizedFinding{
		Title:       firstStr(r, "displayName", "name"),
		Description: firstStr(r, "description"),
		ExternalID:  firstStr(r, "id", "name"),
	}
	// severity often under status.severity
	if st, ok := r["status"].(map[string]any); ok {
		nf.Severity = strings.ToLower(firstStr(st, "severity"))
	}
	if nf.Severity == "" {
		nf.Severity = strings.ToLower(firstStr(r, "severity"))
	}
	if ad, ok := r["additionalData"].(map[string]any); ok {
		nf.CVSSScore = firstFloat(ad, "cvss", "cvssScore")
		nf.CVEID = firstCVE(ad, "cve", "cveId")
		nf.RemediationHint = firstStr(ad, "remediation")
	}
	if nf.CVEID == "" {
		nf.CVEID = firstCVE(r, "id", "displayName")
	}
	if rd, ok := r["resourceDetails"].(map[string]any); ok {
		nf.AssetExternalID = firstStr(rd, "id", "resourceId")
		nf.AssetName = firstStr(rd, "id", "resourceId")
	}
	return nf
}

// CrowdStrike Falcon Spotlight (combined vulnerabilities).
func normalizeCrowdStrike(r map[string]any) NormalizedFinding {
	nf := NormalizedFinding{ExternalID: firstStr(r, "id")}
	if cve, ok := r["cve"].(map[string]any); ok {
		nf.CVEID = strings.ToUpper(firstStr(cve, "id"))
		nf.CVSSScore = firstFloat(cve, "base_score", "score")
		nf.Severity = strings.ToLower(firstStr(cve, "severity"))
		nf.Title = firstStr(cve, "description", "id")
		nf.Description = firstStr(cve, "description")
		// exploit_status: CrowdStrike uses 0/30/60/90; >0 means a known exploit.
		if es := firstFloat(cve, "exploit_status"); es > 0 {
			nf.ExploitAvailable = true
			switch {
			case es >= 90:
				nf.ExploitMaturity = "high"
			case es >= 60:
				nf.ExploitMaturity = "functional"
			default:
				nf.ExploitMaturity = "poc"
			}
		}
		if strings.EqualFold(firstStr(cve, "exprt_rating"), "CRITICAL") {
			nf.ExploitMaturity = "high"
		}
	}
	if hi, ok := r["host_info"].(map[string]any); ok {
		nf.AssetName = firstStr(hi, "hostname", "local_ip")
	}
	if rem, ok := r["remediation"].(map[string]any); ok {
		if ents, ok := rem["entities"].([]any); ok && len(ents) > 0 {
			if e0, ok := ents[0].(map[string]any); ok {
				nf.RemediationHint = firstStr(e0, "action")
			}
		}
	}
	return nf
}

// Generic best-effort normaliser (manual entry, built-in scanner, unknown tools).
func normalizeGeneric(r map[string]any) NormalizedFinding {
	nf := NormalizedFinding{
		Title:            firstStr(r, "title", "name", "plugin_name"),
		Description:      firstStr(r, "description", "summary"),
		CVSSScore:        firstFloat(r, "cvss", "cvss_score", "cvssScore", "score"),
		CVSSVector:       firstStr(r, "cvss_vector", "cvssVector"),
		Severity:         strings.ToLower(firstStr(r, "severity")),
		EPSS:             firstFloat(r, "epss"),
		KEV:              firstBool(r, "kev", "known_exploited"),
		ExploitAvailable: firstBool(r, "exploit_available", "exploitAvailable"),
		ExploitMaturity:  strings.ToLower(firstStr(r, "exploit_maturity")),
		ExternalID:       firstStr(r, "external_id", "id", "external_id"),
		AssetName:        firstStr(r, "asset", "asset_name", "host", "hostname"),
		AssetExternalID:  firstStr(r, "asset_external_id", "asset_id"),
		RemediationHint:  firstStr(r, "remediation", "solution", "remediation_hint"),
	}
	nf.CVEID = firstCVE(r, "cve", "cve_id", "cveId")
	if nf.CVEID == "" {
		nf.CVEID = firstCVE(r, "title", "name")
	}
	return nf
}

// AffectedFromCount records a blast-radius hint carried by the source; stored on
// Raw so the use case can prefer the tenant-wide DB count when available.
func (nf *NormalizedFinding) AffectedFromCount(n int) {
	if n > 0 {
		if nf.Raw == nil {
			nf.Raw = map[string]any{}
		}
		nf.Raw["_affected_hint"] = n
	}
}

// ---- tolerant getters -----------------------------------------------------

func firstStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if t != "" {
					return t
				}
			case []any:
				if len(t) > 0 {
					if s, ok := t[0].(string); ok && s != "" {
						return s
					}
				}
			}
		}
	}
	return ""
}

func firstFloat(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case float64:
				return t
			case int:
				return float64(t)
			case string:
				if f, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
					return f
				}
			case map[string]any:
				// e.g. Azure additionalData.cvss = {"base": 7.5}
				if b := firstFloat(t, "base", "baseScore", "score"); b > 0 {
					return b
				}
			}
		}
	}
	return 0
}

func firstBool(m map[string]any, keys ...string) bool {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case bool:
				return t
			case string:
				if b, err := strconv.ParseBool(strings.TrimSpace(t)); err == nil {
					return b
				}
				if strings.EqualFold(t, "yes") {
					return true
				}
			}
		}
	}
	return false
}

// firstCVE extracts the first CVE id from any of the given keys (string, array,
// or a value that merely contains a CVE substring).
func firstCVE(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if id := cveRe.FindString(t); id != "" {
					return strings.ToUpper(id)
				}
			case []any:
				for _, e := range t {
					if s, ok := e.(string); ok {
						if id := cveRe.FindString(s); id != "" {
							return strings.ToUpper(id)
						}
					}
				}
			}
		}
	}
	return ""
}
