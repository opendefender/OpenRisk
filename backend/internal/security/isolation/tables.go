// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package isolation

// TenantTables lists the tables whose tenant isolation is a property of their
// shape: each carries `tenant_id uuid NOT NULL`, and an index leads with it, so
// a query that names the tenant is served by an index and a row cannot be written
// without one. tables_pg_test.go asserts both against a real PostgreSQL schema.
//
// The registry in registry.go is route-keyed and says nothing about tables; this
// list is the table side (#553 criterion 9, D-108). A table is added with its
// migration, and a new table of the ADR 0008 control chain is not "registered"
// until it is here.
var TenantTables = []string{
	// The control chain (migration 0086, #553).
	"controls",
	"control_implementations",
	"control_tests",
	"control_test_results",
	"control_gap_findings",
	"canonical_edges",
	// Control exceptions (migration 0087, #1428). Not a chain table: a decision
	// recorded about a canonical control, gated on the control of the same tenant.
	"control_exceptions",
	// Control status evaluations (migration 0089, #555). The engine's cache, one row
	// per (tenant_id, control_id); engine output, never an input.
	"control_status_evaluations",
	// The compliance catalogue a tenant owns: its frameworks, their controls and the
	// crosswalk links between controls (#1490, follow-up of the #1483 security review).
	// They predate the control chain and are created by AutoMigrate from the domain
	// models (domain.ComplianceFramework, domain.ComplianceControl,
	// domain.ControlCrosswalk), not by a numbered SQL migration.
	"compliance_frameworks",
	"compliance_controls",
	"control_crosswalks",
}
