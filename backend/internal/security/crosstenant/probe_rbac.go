// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package crosstenant

// Legacy RBAC routes (#1518). They were unprobed (RR-03) because registration
// wrote no tenants or user_tenants row, so a registered root could not get a 2xx
// control. Registration now writes both.
//
// {{viewer}} and {{analyst}} are the ids of the predefined roles. They are
// deployment-wide, so the harness fills them identically for the attack, the
// random-id twin and the control: a random role id would make the twin fail on
// validation and fake an oracle.

const (
	rbacTenantIgnoredReason = "the handler takes the tenant from the session and ignores the path id: A gets 200 about its OWN tenant for B's id and for a random id alike; nothing of B is read or changed. The tenants row is not mutation-verified: KindTenant has no read path"
)

var rbacProbes = []Probe{
	// Aggregates that passed vacuously before: now each control shows a row of B.
	list("/api/v1/rbac/roles", KindRBACRole),
	list("/api/v1/rbac/users", KindRBACUser),

	get("/api/v1/rbac/roles/:role_id", KindRBACRole),
	patch("/api/v1/rbac/roles/:role_id", `{"name":"changed by attacker","is_active":true}`, KindRBACRole),
	del("/api/v1/rbac/roles/:role_id", KindRBACRole),
	get("/api/v1/rbac/roles/:role_id/permissions", KindRBACRole),
	post("/api/v1/rbac/roles/:role_id/permissions", `{"permission_id":"{{permission}}"}`, KindRBACRole, KindPermission),
	delWithBody("/api/v1/rbac/roles/:role_id/permissions", `{"permission_id":"{{permission}}"}`, KindRBACRole, KindPermission),

	// Mounted before /:user_id, so "stats" is no longer read as a user id (#1519).
	agg("/api/v1/rbac/users/stats"),
	get("/api/v1/rbac/users/:user_id", KindRBACUser),
	get("/api/v1/rbac/users/:user_id/permissions", KindRBACUser),
	// A user with no row in the caller's tenant, B's or a random id, answers 404 (#1520).
	del("/api/v1/rbac/users/:user_id", KindRBACUser),
	patch("/api/v1/rbac/users/:user_id/role", `{"role_id":"{{analyst}}"}`, KindRBACUser),
	// The target must be an active member of the caller's organisation: B's user and a random id both answer 404.
	post("/api/v1/rbac/users", `{"user_id":"{{user}}","role_id":"{{viewer}}"}`, KindUser),

	allow(get("/api/v1/rbac/tenants/:tenant_id", KindTenant), rbacTenantIgnoredReason, "RR-15", 200),
	allow(patch("/api/v1/rbac/tenants/:tenant_id", `{"name":"xt-{{run}}-renamed","is_active":true}`, KindTenant), rbacTenantIgnoredReason, "RR-15", 200),
	allow(get("/api/v1/rbac/tenants/:tenant_id/stats", KindTenant), rbacTenantIgnoredReason, "RR-15", 200),
	allow(get("/api/v1/rbac/tenants/:tenant_id/users", KindTenant), rbacTenantIgnoredReason, "RR-15", 200),
}
