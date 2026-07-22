// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package scanner

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// dedupeAssets collapses assets sharing the same ExternalID (within a single
// scan / tenant — the pipeline is always invoked per-tenant). Later occurrences
// merge their CPEs and tags into the first, taking the max criticality. Assets
// with an empty ExternalID fall back to a hostname/IP/name key so a bare nmap
// host without a cloud ID still de-dupes sensibly.
func dedupeAssets(assets []AssetDiscovery) []AssetDiscovery {
	index := make(map[string]int, len(assets))
	out := make([]AssetDiscovery, 0, len(assets))
	for _, a := range assets {
		key := assetDedupeKey(a)
		if i, ok := index[key]; ok {
			out[i].CPE = normalizeCPEList(append(out[i].CPE, a.CPE...))
			out[i].Tags = dedupeStrings(append(out[i].Tags, a.Tags...))
			if a.Criticality > out[i].Criticality {
				out[i].Criticality = a.Criticality
			}
			// Fill in any field the first occurrence left empty.
			mergeAssetHoles(&out[i], a)
			continue
		}
		index[key] = len(out)
		out = append(out, a)
	}
	return out
}

func assetDedupeKey(a AssetDiscovery) string {
	if id := strings.TrimSpace(a.ExternalID); id != "" {
		return "ext:" + strings.ToLower(id)
	}
	if a.Hostname != nil && strings.TrimSpace(*a.Hostname) != "" {
		return "host:" + strings.ToLower(strings.TrimSpace(*a.Hostname))
	}
	if a.IP != nil && strings.TrimSpace(*a.IP) != "" {
		return "ip:" + strings.TrimSpace(*a.IP)
	}
	return "name:" + strings.ToLower(strings.TrimSpace(a.Name))
}

func mergeAssetHoles(dst *AssetDiscovery, src AssetDiscovery) {
	if dst.IP == nil {
		dst.IP = src.IP
	}
	if dst.Hostname == nil {
		dst.Hostname = src.Hostname
	}
	if dst.OS == nil {
		dst.OS = src.OS
	}
	if dst.OSVersion == nil {
		dst.OSVersion = src.OSVersion
	}
	if dst.Location == nil {
		dst.Location = src.Location
	}
	if dst.Environment == "" {
		dst.Environment = src.Environment
	}
}

// dedupeFindings collapses findings that describe the same issue on the same
// asset. The key is (assetExternalID, CVE OR title, sorted affected CPEs).
func dedupeFindings(findings []FindingDiscovery) []FindingDiscovery {
	seen := make(map[string]struct{}, len(findings))
	out := make([]FindingDiscovery, 0, len(findings))
	for _, f := range findings {
		key := findingDedupeKey(f)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, f)
	}
	return out
}

func findingDedupeKey(f FindingDiscovery) string {
	var b strings.Builder
	b.WriteString(strings.ToLower(strings.TrimSpace(f.AssetExternalID)))
	b.WriteByte('|')
	if f.CVE != nil && *f.CVE != "" {
		b.WriteString(strings.ToUpper(*f.CVE))
	} else {
		b.WriteString(strings.ToLower(strings.TrimSpace(f.Title)))
	}
	b.WriteByte('|')
	cpes := append([]string(nil), f.AffectedCPE...)
	sort.Strings(cpes)
	b.WriteString(strings.Join(cpes, ","))
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
