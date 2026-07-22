// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package domain

// Auditable opt-ins (spec §15). A model gains automatic, immutable audit-trail
// coverage for every struct-form Create/Update/Delete simply by declaring its
// entity type here — the audittrail GORM plugin does the rest, so a developer
// can never forget to journal a mutation. To cover a new entity, add one line.
//
// Risk is intentionally NOT listed: it is written on a hot path by the
// Score Engine worker (targeted map updates the plugin already skips), and its
// point-in-time changes are captured separately in risk_histories.

func (Asset) AuditEntityType() string { return "asset" }

func (ComplianceControl) AuditEntityType() string { return "compliance_control" }
