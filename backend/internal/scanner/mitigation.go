// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package scanner

import (
	"fmt"
	"time"
)

// detectMitigations diffs the previous scan's findings against the current ones
// for the same config: a finding that was present before but is gone now looks
// like it was remediated (port closed, service removed, software patched, CVE
// eliminated). These are surfaced on the preview's "Auto-detected Mitigations"
// tab for the user to confirm — the pipeline never auto-closes anything.
//
// Pure function: previous/current come from the caller (previous is loaded from
// the last completed preview of the same config).
func detectMitigations(previous, current []FindingDiscovery, now time.Time) []AutoMitigation {
	currentKeys := make(map[string]struct{}, len(current))
	for _, f := range current {
		currentKeys[findingDedupeKey(f)] = struct{}{}
	}

	out := make([]AutoMitigation, 0)
	emitted := make(map[string]struct{})
	for _, p := range previous {
		key := findingDedupeKey(p)
		if _, still := currentKeys[key]; still {
			continue // still present → not mitigated
		}
		if _, done := emitted[key]; done {
			continue
		}
		emitted[key] = struct{}{}
		out = append(out, AutoMitigation{
			AssetExternalID: p.AssetExternalID,
			CVE:             p.CVE,
			Title:           p.Title,
			Severity:        p.Severity,
			Evidence:        mitigationEvidence(p),
			DetectedAt:      now,
		})
	}
	return out
}

func mitigationEvidence(f FindingDiscovery) string {
	if f.CVE != nil && *f.CVE != "" {
		return fmt.Sprintf("%s no longer detected on %s (was: %s)", *f.CVE, f.AssetExternalID, f.Evidence)
	}
	return fmt.Sprintf("%q no longer detected on %s (was: %s)", f.Title, f.AssetExternalID, f.Evidence)
}
