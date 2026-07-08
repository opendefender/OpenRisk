// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

// Placeholder catalog(s) for frameworks that are announced but not yet modeled because we
// don't have the regulator's actual source text. Registered with Available: false and no
// Controls, this is a deliberate choice, not an oversight: modeling specific article
// citations from training-data recall risks fabricating legal references, which is worse
// than not shipping the framework at all in a product real compliance officers rely on.
//
// The three previously-placeholder African frameworks (COBAC, BCEAO, and the Cameroonian
// cybersecurity law) are now real, cited catalogs — see catalog_cobac_2016.go,
// catalog_bceao_2002.go and catalog_antic_cm_2010.go — because the source documents were
// provided. What remains here is a framework we still lack the text for.

func init() {
	register(Catalog{
		Key:     "cm-loi-2024-017",
		Name:    "Cameroun — Protection des données personnelles",
		Version: "",
		// Referenced in ROADMAP.md M2 as a planned framework. Kept as a placeholder until
		// the actual legal text is available and reviewed — same policy as before.
		Description: "Cadre camerounais de protection des données à caractère personnel — non encore modélisé, en attente du texte source officiel.",
		Available:   false,
		Controls:    nil,
	})
}
