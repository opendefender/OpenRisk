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
}
