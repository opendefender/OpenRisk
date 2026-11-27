// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package crosstenant

// Writes whose target is named in the body, not in the path: the surface the
// isolation package calls a blind spot. A names B's ids in the body and must be
// refused exactly as for random ids.
var bodyProbes = []Probe{
	post("/api/v1/risks", `{"title":"attacker","description":"x","impact":5,"probability":0.4,"category_id":"{{riskcategory}}"}`, KindCategory),
	post("/api/v1/governance/delegations", `{"delegate_id":"{{user}}","permissions":["risks:read"],"starts_at":"2026-01-01T00:00:00Z","ends_at":"2035-01-01T00:00:00Z"}`, KindUser),
	post("/api/v1/risks/bulk/add-tags", `{"ids":["{{risk}}"],"tags":["attacker"]}`, KindRisk),
	post("/api/v1/risks/bulk/remove-tags", `{"ids":["{{risk}}"],"tags":["attacker"]}`, KindRisk),
	post("/api/v1/risks/bulk/change-status", `{"ids":["{{risk}}"],"status":"MITIGATED"}`, KindRisk),
	allow(post("/api/v1/risks/bulk/set-owner", `{"ids":["{{risk}}"],"owner_id":"{{user}}"}`, KindRisk, KindUser),
		"a user id that is not a member of the caller's organisation is a typed validation error (400, 'is not a member of this organization'), identical for a foreign and a random id; nothing is assigned", "RR-11", 400),
	post("/api/v1/risks/bulk/delete", `{"ids":["{{risk}}"]}`, KindRisk),
	allow(post("/api/v1/risks/bulk/preview", `{"ids":["{{risk}}"],"action":"delete"}`, KindRisk),
		"a dry run answers 200 with found:0 and the id under missing, identically for a foreign and a random id; nothing is changed", "RR-10", 200),
	post("/api/v1/assets/bulk/delete", `{"ids":["{{asset}}"]}`, KindAsset),
	allow(post("/api/v1/assets/bulk/preview", `{"ids":["{{asset}}"],"action":"delete"}`, KindAsset),
		"a dry run answers 200 with found:0 and the id under missing, identically for a foreign and a random id; nothing is changed", "RR-10", 200),
	allow(post("/api/v1/compliance/audits", `{"title":"attacker","framework_id":"{{framework}}"}`, KindFramework),
		"framework_id is stored without an ownership check, the same for a foreign and a random id; reads filter by tenant and the audit's remediation generation finds no controls, so nothing of B's is joined or returned", "RR-12", 201),
	allow(post("/api/v1/compliance/remediations", `{"title":"attacker","control_id":"{{control}}","framework_id":"{{framework}}"}`, KindControl),
		"a control that is not the caller's answers 400 'linked control not found', identically for a foreign and a random id; nothing is written", "RR-11", 400),
	allow(post("/api/v1/governance/approvals", `{"title":"attacker","entity_type":"risk_acceptance","entity_id":"{{risk}}","request_type":"risk_acceptance"}`, KindRisk),
		"entity_id is stored without an ownership check, the same for a foreign and a random id; no read resolves it to the other tenant's risk", "RR-12", 201),
	allow(post("/api/v1/incidents", `{"title":"attacker","description":"x","severity":"high","risk_ids":["{{risk}}"]}`, KindRisk),
		"risk_ids is accepted and not resolved: incidents hold unsigned integer risk ids while risks are UUIDs, so nothing foreign is joined or returned", "RR-12", 201),
	post("/api/v1/evidence", `{"title":"attacker","description":"x","control_ids":["{{control}}"]}`, KindControl),
	post("/api/v1/auth/switch-org", `{"organization_id":"{{tenant}}"}`, KindTenant),
	get("/api/v1/score-engine/configs/:id", KindScoreConfig),
	put("/api/v1/score-engine/configs/:id", `{"base_formula":"impact*probability","risk_matrix_thresholds":{"low":4,"medium":9,"high":16,"critical":23}}`, KindScoreConfig),
	allow(q(post("/api/v1/score-engine/configs", `{"name":"attacker","base_formula":"impact*probability","risk_matrix_thresholds":{"low":4,"medium":9,"high":16,"critical":23}}`, KindScoreConfig), "id={{scoreconfig}}"),
		"the id is per tenant: A's write under B's id creates A's own copy and B's configuration is verified unchanged", "RR-13", 201),
	allow(post("/api/v1/score-engine/compute", `{"impact":3,"probability":3,"config_id":"{{scoreconfig}}"}`, KindScoreConfig),
		"an id that is not the caller's scores with the built-in default; B's configuration is not read", "RR-13", 200),
	allow(post("/api/v1/score-engine/classify", `{"score":10,"config_id":"{{scoreconfig}}"}`, KindScoreConfig),
		"an id that is not the caller's classifies with the built-in default; B's configuration is not read", "RR-13", 200),
}

// ownBodyProbes is the second body-id shape: A's OWN object in the path, B's id
// in the body (#1521). The route must refuse it as it refuses a random id of the
// same shape, and nothing of B's may be stored on A's object. The positive
// control is the same request with A's own in-tenant ids.
var ownBodyProbes = []Probe{
	own(patch("/api/v1/risks/:id", `{"category_id":"{{riskcategory}}"}`, KindCategory), KindRisk),
	own(post("/api/v1/risks/:id/transfer-owner", `{"new_owner_id":"{{user}}"}`, KindUser), KindRisk),
	own(post("/api/v1/mitigations/:id/transfer-owner", `{"new_owner_id":"{{user}}"}`, KindUser), KindMitigation),
	own(post("/api/v1/incidents/:id/transfer-owner", `{"new_owner_id":"{{user}}"}`, KindUser), KindIncident),
	own(patch("/api/v1/risks/:id", `{"owner_id":"{{user}}"}`, KindUser), KindRisk),
	// The asset store resolves asset_ids inside A's tenant only, so B's id is
	// dropped and the update answers 200, as it does for a random id. Today the
	// update answers 500 for every caller, A's own asset included: the audit
	// plugin's pre-image query breaks the association replace on Postgres (#1517).
	blockedBy(dropsForeign(own(patch("/api/v1/risks/:id", `{"asset_ids":["{{asset}}"]}`, KindAsset), KindRisk),
		// risk_assets has no tenant_id: gated through the parent risk's tenant.
		`SELECT count(*) FROM risk_assets ra JOIN risks r ON r.id = ra.risk_id
		  WHERE r.tenant_id = $1 AND ra.risk_id = $2 AND ra.asset_id = $3`), "#1517"),
}
