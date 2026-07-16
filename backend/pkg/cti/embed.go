// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package cti

import _ "embed"

// embeddedMITREData is the MITRE ATT&CK CVE→technique/tactic mapping, compiled
// into the binary so enrichment works regardless of the runtime working directory
// (Master Prompt: "Embedded static data — JSON file in pkg/cti/data/mitre_attack.json").
// Refresh it quarterly (manual or automatic) by replacing data/mitre_attack.json.
//
//go:embed data/mitre_attack.json
var embeddedMITREData []byte
