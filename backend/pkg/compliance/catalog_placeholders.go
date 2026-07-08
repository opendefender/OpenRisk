// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

// Placeholder catalogs for the African regulatory frameworks planned in ROADMAP.md M2
// (COBAC, BCEAO, ANSSI-CM, Cameroonian law 2024/017). Registered with Available: false and
// no Controls: this is a deliberate decision, not an oversight — modeling specific article/
// circular citations from training-data recall risks fabricating legal references, which is
// worse than not having the framework at all in a product real compliance officers rely on.
// Each needs real source documents (the regulator's actual text) before any controls are
// added here. See ROADMAP.md §3 M2.

func init() {
	register(Catalog{
		Key:         "cobac",
		Name:        "COBAC",
		Version:     "",
		Description: "Commission Bancaire de l'Afrique Centrale — not yet modeled, pending real source documents (circulaires COBAC).",
		Available:   false,
		Controls:    nil,
	})
	register(Catalog{
		Key:         "bceao",
		Name:        "BCEAO",
		Version:     "",
		Description: "Banque Centrale des États de l'Afrique de l'Ouest — not yet modeled, pending real source documents.",
		Available:   false,
		Controls:    nil,
	})
	register(Catalog{
		Key:     "anssi-cm",
		Name:    "ANSSI-CM",
		Version: "",
		// Deliberately not asserting which Cameroonian body this refers to or its full name —
		// that's exactly the kind of unverified specific claim this placeholder exists to avoid.
		Description: "Cameroonian cybersecurity/regulatory directives (as referenced in ROADMAP.md M2) — not yet modeled, pending real source documents and confirmation of the issuing body.",
		Available:   false,
		Controls:    nil,
	})
}
