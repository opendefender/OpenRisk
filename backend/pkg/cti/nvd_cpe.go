// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package cti

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// =============================================================================
// Windows programs → CVEs through NVD (#910, owner decision D-071)
// =============================================================================
//
// OSV has no Windows ecosystem. A Windows program (publisher, name, version) is
// resolved to an NVD product, then NVD is asked which CVEs affect that exact
// version with virtualMatchString, so NVD applies its own version ranges (which
// the catalogue sync does not keep). The resolution is deliberately strict: a
// candidate is kept only when its vendor matches the publisher (or the program
// name) and its product matches the name. No candidate means "not checkable",
// never a guess. Callers mark these findings with a lower match confidence.

const (
	nvdCPEsURL = "https://services.nvd.nist.gov/rest/json/cpes/2.0"
	nvdCVEsURL = "https://services.nvd.nist.gov/rest/json/cves/2.0"
)

var (
	parenthetical = regexp.MustCompile(`\([^)]*\)`)
	versionToken  = regexp.MustCompile(`\b[vV]?\d+(\.\d+)+\b`)
	nonAlnum      = regexp.MustCompile(`[^a-z0-9]+`)
)

// ProgramKeyword strips what makes a Windows display name unsearchable: the
// version and the parenthesised edition. "7-Zip 23.01 (x64)" → "7-Zip".
func ProgramKeyword(name string) string {
	s := parenthetical.ReplaceAllString(name, " ")
	s = versionToken.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

func squash(s string) string { return nonAlnum.ReplaceAllString(strings.ToLower(s), "") }

type nvdCPEResponse struct {
	Products []struct {
		CPE struct {
			CPEName    string `json:"cpeName"`
			Deprecated bool   `json:"deprecated"`
		} `json:"cpe"`
	} `json:"products"`
}

// ResolveCPE returns the NVD "vendor:product" of a Windows program, or "" when
// no candidate passes the vendor and product checks.
func (c *ExternalClient) ResolveCPE(ctx context.Context, publisher, name string) (string, error) {
	kw := ProgramKeyword(name)
	if kw == "" {
		return "", nil
	}
	params := url.Values{}
	params.Set("keywordSearch", kw)
	params.Set("resultsPerPage", "50")
	data, err := c.fetchWithRetry(ctx, nvdCPEsURL+"?"+params.Encode(), c.nvdHeaders())
	if err != nil {
		return "", fmt.Errorf("NVD CPE search failed: %w", err)
	}
	var resp nvdCPEResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("NVD CPE parse failed: %w", err)
	}
	return pickCPE(resp, publisher, kw), nil
}

// pickCPE applies the strict checks and prefers the longest product match.
func pickCPE(resp nvdCPEResponse, publisher, keyword string) string {
	pub, kw := squash(publisher), squash(keyword)
	best, bestLen := "", 0
	for _, p := range resp.Products {
		if p.CPE.Deprecated {
			continue
		}
		f := strings.Split(p.CPE.CPEName, ":")
		// cpe:2.3:<part>:<vendor>:<product>:…; applications only.
		if len(f) < 5 || f[2] != "a" {
			continue
		}
		vendor, product := squash(f[3]), squash(f[4])
		if vendor == "" || product == "" {
			continue
		}
		vendorOK := (pub != "" && (strings.Contains(pub, vendor) || strings.Contains(vendor, pub))) ||
			strings.Contains(kw, vendor)
		productOK := strings.Contains(kw, product) || strings.Contains(product, kw)
		if vendorOK && productOK && len(product) > bestLen {
			best, bestLen = f[3]+":"+f[4], len(product)
		}
	}
	return best
}

// FetchNVDByCPE returns the CVEs NVD says affect that exact version of the
// product ("vendor:product"). Severity is kept as NVD gives it.
func (c *ExternalClient) FetchNVDByCPE(ctx context.Context, vendorProduct, version string) ([]CTIVulnerability, error) {
	if vendorProduct == "" || strings.TrimSpace(version) == "" {
		return nil, nil
	}
	params := url.Values{}
	params.Set("virtualMatchString", "cpe:2.3:a:"+vendorProduct+":"+cpeEscape(version))
	params.Set("resultsPerPage", "200")
	data, err := c.fetchWithRetry(ctx, nvdCVEsURL+"?"+params.Encode(), c.nvdHeaders())
	if err != nil {
		return nil, fmt.Errorf("NVD CVE search failed: %w", err)
	}
	var resp nvdResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("NVD parse failed: %w", err)
	}
	out := make([]CTIVulnerability, 0, len(resp.Vulnerabilities))
	for _, e := range resp.Vulnerabilities {
		out = append(out, c.nvdEntryToVuln(e))
	}
	return out, nil
}

// cpeEscape writes a version as a CPE 2.3 formatted-string value: letters,
// digits, "_", "-" and "." stay, a space becomes "_", and every other
// character is backslash-quoted (backslash first, so it is never doubled by
// mistake). A host's own software can carry any of them.
func cpeEscape(v string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(v) {
		switch {
		case r == ' ':
			b.WriteByte('_')
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('\\')
			b.WriteRune(r)
		}
	}
	return b.String()
}
