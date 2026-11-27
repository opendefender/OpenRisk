// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package crosstenant

// Governance, organisation, RBAC, entities and audit trails.
var orgProbes = []Probe{
	list("/api/v1/governance/approvals", KindApproval),
	get("/api/v1/governance/approvals/:id", KindApproval),
	post("/api/v1/governance/approvals/:id/cancel", `{"reason":"attacker"}`, KindApproval),
	list("/api/v1/governance/delegations", KindDelegation),
	post("/api/v1/governance/delegations/:id/revoke", `{}`, KindDelegation),
	list("/api/v1/governance/workflows", KindWorkflow),
	// approver_user_ids names B's user: refused 404 like a random id, and no
	// workflow is stored (#1520). request_type policy_change collides with no
	// seeded workflow. The PUT probe below cannot isolate the approver check:
	// the workflow lookup, also B's id, answers 404 first; the use-case unit test
	// TestUpdateWorkflow_NotFound pins the PUT approver check.
	post("/api/v1/governance/workflows", `{"name":"attacker","request_type":"policy_change","steps":[{"approver_role":"admin","approver_user_ids":["{{user}}"]}]}`, KindUser),
	put("/api/v1/governance/workflows/:id", `{"name":"changed by attacker","request_type":"security_exception","steps":[{"approver_role":"admin"}]}`, KindWorkflow),
	del("/api/v1/governance/workflows/:id", KindWorkflow),

	get("/api/v1/organization/members/:memberId", KindMember),
	put("/api/v1/organization/members/:memberId/role", `{"role":"admin"}`, KindMember),
	put("/api/v1/organization/members/:memberId/status", `{"status":"deactivated"}`, KindMember),
	list("/api/v1/organization/invitations", KindInvitation),
	del("/api/v1/organization/invitations/:invitationId", KindInvitation),
	list("/api/v1/organization/members", KindMember),

	get("/api/v1/entities/:type/:id", "=risk", KindRisk),
	get("/api/v1/entities/:type/:id/audit", "=risk", KindRisk),
	get("/api/v1/entities/:type/:id/relations", "=risk", KindRisk),
	get("/api/v1/entities/:type/:id/timeline", "=risk", KindRisk),

	get("/api/v1/audit-logs/user/:user_id", KindUser),
	// The audit trail of the caller's own organisation (#1519): an ip_address
	// the driver returns as text made every read of a row that carried one a 500.
	list("/api/v1/audit-logs", KindUser),
	// The path segment is an action name, not an id. Seeding a user writes a
	// user_create row in each tenant, so B's control must show B's user and A's
	// page must carry none of B's rows (not Aggregate: the VACUOUS guard stays on).
	{Method: "GET", Path: "/api/v1/audit-logs/action/:action", Kinds: []Kind{"=user_create", KindUser}, Collection: true},
	get("/api/v1/attack-surface/topology/:id/compromise-chain", KindAsset),
}
