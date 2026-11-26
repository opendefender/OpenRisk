// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package crosstenant

// Risk-centred routes: risks, mitigations, timeline, incidents and the
// taxonomy that hangs off a risk.
var riskProbes = []Probe{
	list("/api/v1/risks", KindRisk),
	get("/api/v1/risks/:id", KindRisk),
	patch("/api/v1/risks/:id", `{"description":"changed by attacker"}`, KindRisk),
	del("/api/v1/risks/:id", KindRisk),
	allow(get("/api/v1/risks/:id/control-mappings", KindRisk),
		"answers 200 with an empty list for any risk id, foreign or random, without reading the risk; B's mappings are not returned", "RR-02", 200),
	get("/api/v1/risks/:id/financial", KindRisk),
	get("/api/v1/risks/:id/mitigations", KindRisk),
	post("/api/v1/risks/:id/mitigations", `{"title":"attacker","description":"x","status":"PLANNED"}`, KindRisk),
	post("/api/v1/risks/:id/review", `{}`, KindRisk),
	get("/api/v1/risks/:id/score-working", KindRisk),
	post("/api/v1/risks/:id/simulate", `{}`, KindRisk),
	get("/api/v1/risks/:id/smart-score", KindRisk),
	post("/api/v1/risks/:id/smart-score/simulate", `{}`, KindRisk),
	get("/api/v1/risks/:id/timeline", KindRisk),
	get("/api/v1/risks/:id/timeline/changes/:type", KindRisk, "=score"),
	get("/api/v1/risks/:id/timeline/score-changes", KindRisk),
	q(get("/api/v1/risks/:id/timeline/since/:timestamp", KindRisk, "=1700000000"), "since=1700000000"),
	get("/api/v1/risks/:id/timeline/status-changes", KindRisk),
	get("/api/v1/risks/:id/timeline/trend", KindRisk),
	post("/api/v1/risks/:id/transfer-owner", `{"new_owner_id":"{{user}}"}`, KindRisk, KindUser),
	post("/api/v1/risks/:id/transition", `{"to":"identified"}`, KindRisk),
	get("/api/v1/risks/:id/transitions", KindRisk),

	list("/api/v1/mitigations", KindMitigation),
	get("/api/v1/mitigations/:id", KindMitigation),
	patch("/api/v1/mitigations/:id", `{"description":"changed by attacker"}`, KindMitigation),
	del("/api/v1/mitigations/:id", KindMitigation),
	patch("/api/v1/mitigations/:id/reorder-subactions", `{"sub_actions":[{"id":"{{subaction}}","order":1}]}`, KindMitigation, KindSubAction),
	post("/api/v1/mitigations/:id/sub-actions", `{"title":"attacker"}`, KindMitigation),
	del("/api/v1/mitigations/:id/sub-actions/:aid", KindMitigation, KindSubAction),
	patch("/api/v1/mitigations/:id/sub-actions/:aid", `{"title":"changed by attacker"}`, KindMitigation, KindSubAction),
	post("/api/v1/mitigations/:id/sub-actions/:aid/complete", `{}`, KindMitigation, KindSubAction),
	post("/api/v1/mitigations/:id/sub-actions/:aid/revert", `{}`, KindMitigation, KindSubActionDone),
	post("/api/v1/mitigations/:id/transfer-owner", `{"new_owner_id":"{{user}}"}`, KindMitigation, KindUser),
	patch("/api/v1/mitigations/:id/validate", `{}`, KindMitigationInReview),

	list("/api/v1/incidents", KindIncident),
	get("/api/v1/incidents/:id", KindIncident),
	put("/api/v1/incidents/:id", `{"title":"changed by attacker","description":"x","severity":"high","status":"open"}`, KindIncident),
	del("/api/v1/incidents/:id", KindIncident),
	get("/api/v1/incidents/:id/actions", KindIncident),
	post("/api/v1/incidents/:id/actions", `{"title":"attacker","description":"x"}`, KindIncident),
	get("/api/v1/incidents/:id/post-mortem", KindIncident),
	put("/api/v1/incidents/:id/post-mortem", `{"summary":"attacker"}`, KindIncident),
	post("/api/v1/incidents/:id/post-mortem/publish", `{}`, KindIncidentPM),
	get("/api/v1/incidents/:id/timeline", KindIncident),
	post("/api/v1/incidents/:id/transfer-owner", `{"new_owner_id":"{{user}}"}`, KindIncident, KindUser),
	agg("/api/v1/incidents/origins"),
	agg("/api/v1/incidents/stats"),
}
