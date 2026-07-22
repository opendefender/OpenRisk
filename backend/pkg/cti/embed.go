// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package cti

import _ "embed"

// embeddedMITREData is the MITRE ATT&CK CVE→technique/tactic mapping, compiled
// into the binary so enrichment works regardless of the runtime working directory
// (Master Prompt: "Embedded static data — JSON file in pkg/cti/data/mitre_attack.json").
// Refresh it quarterly (manual or automatic) by replacing data/mitre_attack.json.
//
//go:embed data/mitre_attack.json
var embeddedMITREData []byte
