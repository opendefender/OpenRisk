// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

// Withdrawn catalog(s): registered in code, but NOT offered for import.
//
// The distinction from Available:false matters. An unavailable catalog is one
// the product intends to ship and has not modelled yet — it appears in the
// picker as "coming soon", which is a promise. A withdrawn one is removed from
// the picker entirely, because offering a framework we cannot cite article by
// article invites a compliance officer to import a shell and believe they have a
// programme. The code stays so the key keeps resolving for tenants who already
// imported it, and so the reconstruction has somewhere to land.

func init() {
	register(Catalog{
		Key:     "cm-loi-2024-017",
		Name:    "Cameroun — Protection des données personnelles",
		Version: "",
		Description: "Loi n° 2024/017 du Cameroun relative à la protection des données à caractère personnel. " +
			"Retiré du catalogue d'import en attendant une modélisation article par article vérifiée " +
			"contre le texte officiel — voir docs/tickets/CM-LOI-2024-017-reconstruction.md.",
		Available: false,
		// Withdrawn keeps it out of the import picker. Modelling article citations
		// from recall rather than from the published text risks putting fabricated
		// legal references in front of a regulator, which is worse than shipping
		// nothing: a shell framework reads as coverage in every dashboard,
		// percentage and report in the product.
		Withdrawn:        true,
		WithdrawalReason: "Texte source officiel non disponible : la modélisation article par article n'a pas été vérifiée contre le Journal officiel.",
		TrackingTicket:   "docs/tickets/CM-LOI-2024-017-reconstruction.md",
		Controls:         nil,
	})
}
