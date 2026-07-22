// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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
