// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entity

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// =============================================================================
// TenantScope — a tenant id that cannot be transposed with an entity id
// =============================================================================

// TenantScope is the caller's tenant, proven non-nil at construction.
//
// It exists because `f(ctx, tenantID, entityID uuid.UUID)` has two interchangeable
// parameters of the same type. Transposing them compiles, passes review and
// produces a query that filters the wrong column — and on the drawer's routes
// that is a cross-tenant read. A distinct type makes the transposition a
// compile error instead of a security incident.
//
// The id is unexported and there is exactly one constructor, so a TenantScope in
// hand is a proof that NewTenantScope accepted a caller. A zero-value
// TenantScope{} carries uuid.Nil and is rejected by every method that reads it.
type TenantScope struct {
	id uuid.UUID
}

// NewTenantScope derives the scope from an authenticated caller.
//
// A caller with no tenant is refused here, once, rather than at each repository:
// uuid.Nil reaching a `WHERE tenant_id = ?` matches no row on a healthy table,
// but it is one missing predicate away from matching every row, and that is the
// shape RULE #2 exists to forbid. This is 401, not 403 — the caller has not
// proven who they are, which is a different answer from having proven it and
// lacking a permission.
func NewTenantScope(c Caller) (TenantScope, error) {
	if c.TenantID == uuid.Nil {
		return TenantScope{}, domain.NewUnauthorizedError("no tenant on the caller")
	}
	return TenantScope{id: c.TenantID}, nil
}

// UUID returns the tenant id for a repository call. This is the only way out of
// the type, and it is deliberately a method rather than an exported field so
// that every use site reads `scope.UUID()` and is greppable.
func (s TenantScope) UUID() uuid.UUID { return s.id }

// IsZero reports a scope that was never constructed.
func (s TenantScope) IsZero() bool { return s.id == uuid.Nil }

// String renders the scope for logs. It prints the tenant id, which is not a
// secret — it is in every JWT the caller holds.
func (s TenantScope) String() string { return s.id.String() }

// =============================================================================
// TenantGate — how a type is scoped, declared rather than assumed
// =============================================================================

// TenantGate declares HOW a registered type is confined to its tenant.
//
// The marker method is unexported, so the set of gates is closed to this
// package: a third strategy cannot be declared from outside, and `go build`
// fails on the attempt. That is the point — "we scope it somehow" is how
// `RiskHistory`, a table with no tenant_id at all, ended up feeding an
// all-tenants read on 2026-07-23 (JOURNAL item 36). A type must now say which
// of the two shapes it is, and a table with no tenant column of its own is
// forced to name the parent it gates through.
type TenantGate interface {
	// isTenantGate is an unexported marker. It closes this interface to the
	// entity package; do not give it a body that does anything.
	isTenantGate()
	// Strategy names the gate in test output and in the ADR table.
	Strategy() string
	// Describe renders the gate as prose for an isolation report.
	Describe() string
}

// OwnColumn is the simple case: the type's own table carries tenant_id, and
// every query filters on it directly.
type OwnColumn struct {
	// Table is the physical table name, so the ADR's TenantGate table can be
	// checked against the schema rather than trusted.
	Table string
}

func (OwnColumn) isTenantGate()    {}
func (OwnColumn) Strategy() string { return "own_column" }
func (g OwnColumn) Describe() string {
	return fmt.Sprintf("%s.tenant_id filtered directly", g.Table)
}

// ThroughParent is the dangerous case: the type's own table has NO tenant_id, so
// isolation is inherited from a parent row that does. Naming the parent and the
// predicate that enforces it here is what makes that inheritance reviewable
// instead of implicit. This is the RiskHistory / IncidentTimeline shape, stated
// rather than assumed — see ADR-0001.
type ThroughParent struct {
	// Parent is the gating entity type.
	Parent Type
	// Table is the physical table with no tenant_id of its own.
	Table string
	// GateFunc names the ownsX function or the JOIN predicate that enforces the
	// gate, e.g. "JOIN risks r ON r.id = h.risk_id AND r.tenant_id = ?".
	GateFunc string
}

func (ThroughParent) isTenantGate()    {}
func (ThroughParent) Strategy() string { return "through_parent" }
func (g ThroughParent) Describe() string {
	return fmt.Sprintf("%s has no tenant_id; gated through %s by %s",
		g.Table, g.Parent, g.GateFunc)
}

// =============================================================================
// IDShape — because a sequential id is enumerable and a uuid is not
// =============================================================================

// IDShape is the form a type's primary key takes.
type IDShape string

const (
	// IDShapeUUID is an unguessable v4 identifier.
	IDShapeUUID IDShape = "uuid"
	// IDShapeSequential is an auto-incrementing integer. Anyone holding one id
	// can name every other id, so a sequential type MUST declare what stops a
	// caller walking the range — see Registration.EnumerationMitigation.
	IDShapeSequential IDShape = "sequential"
)

// =============================================================================
// Registration — a type cannot enter the registry without declaring its gate
// =============================================================================

// Registration is everything the registry must know to serve a type safely.
//
// Register takes one of these rather than a bare (Type, Resolver) pair, which is
// the whole mechanism: the fields that matter for isolation cannot be omitted,
// because omitting one panics at wiring time — in main(), at boot, in front of
// whoever added the type — rather than silently shipping an unscoped read.
type Registration struct {
	// Type is the wire name.
	Type Type
	// Resolver answers the four drawer questions for this type.
	Resolver Resolver
	// IDShape is the form of this type's primary key.
	IDShape IDShape
	// Gate declares how the type is confined to its tenant.
	Gate TenantGate
	// IsolationTest names the Go test function that proves the gate holds. It is
	// checked against the source by TestRegistry_IsolationTestNamesAnExistingFunction,
	// so it cannot drift into naming a test that was renamed or deleted.
	IsolationTest string
	// EnumerationMitigation is required when IDShape is sequential: it states
	// what stops a caller who holds one id from walking the whole range.
	EnumerationMitigation string
}

// validate reports why a registration may not be registered.
func (reg Registration) validate() error {
	switch {
	case reg.Type == "":
		return fmt.Errorf("Type is empty")
	case reg.Resolver == nil:
		return fmt.Errorf("Resolver is nil for type %q", reg.Type)
	case reg.IDShape == "":
		return fmt.Errorf("IDShape is empty for type %q", reg.Type)
	case reg.Gate == nil:
		return fmt.Errorf("Gate is nil for type %q", reg.Type)
	case reg.IsolationTest == "":
		return fmt.Errorf("IsolationTest is empty for type %q", reg.Type)
	}
	if reg.IDShape == IDShapeSequential && reg.EnumerationMitigation == "" {
		return fmt.Errorf(
			"type %q has sequential ids and declares no EnumerationMitigation: "+
				"state what stops a caller walking the id range", reg.Type)
	}
	return nil
}

// =============================================================================
// The isolation profile table — one declaration site for all eight gates
// =============================================================================

// isolationProfiles declares everything about a registration EXCEPT its
// resolver: the id shape, the tenant gate, the enumeration mitigation and the
// name of the test that proves the gate holds.
//
// It lives here, in the package, rather than at the wiring site in main() for
// one reason: the wiring site is not the only place that builds a registry. The
// service tests and the HTTP tests build their own, and when the gate metadata
// is written out at each of those sites they drift — which is precisely how
// `vendor` came to be absent from TestEntityDrawer_ResolvesEveryTypeOverHTTP
// while nothing went red. One table, three consumers, no drift.
//
// Every one of the eight gates on its own tenant_id column today. That is a
// fact about the current schema, not a rule: ThroughParent exists because the
// journals behind the timeline do NOT, and a future type whose table has no
// tenant of its own must say so here rather than quietly inherit.
var isolationProfiles = map[Type]Registration{
	TypeAsset: {
		Type: TypeAsset, IDShape: IDShapeUUID,
		Gate:          OwnColumn{Table: "assets"},
		IsolationTest: "TestRegistry_EveryRegisteredTypeIsTenantIsolated",
	},
	TypeVendor: {
		// A vendor is an asset of category "vendor" — same table, same gate.
		Type: TypeVendor, IDShape: IDShapeUUID,
		Gate:          OwnColumn{Table: "assets"},
		IsolationTest: "TestRegistry_EveryRegisteredTypeIsTenantIsolated",
	},
	TypeRisk: {
		Type: TypeRisk, IDShape: IDShapeUUID,
		Gate:          OwnColumn{Table: "risks"},
		IsolationTest: "TestRegistry_EveryRegisteredTypeIsTenantIsolated",
	},
	TypeVulnerability: {
		Type: TypeVulnerability, IDShape: IDShapeUUID,
		Gate:          OwnColumn{Table: "vulnerabilities"},
		IsolationTest: "TestRegistry_EveryRegisteredTypeIsTenantIsolated",
	},
	TypeFinding: {
		// A finding is the scan-sourced projection of a vulnerability row.
		Type: TypeFinding, IDShape: IDShapeUUID,
		Gate:          OwnColumn{Table: "vulnerabilities"},
		IsolationTest: "TestRegistry_EveryRegisteredTypeIsTenantIsolated",
	},
	TypeControl: {
		Type: TypeControl, IDShape: IDShapeUUID,
		Gate:          OwnColumn{Table: "compliance_controls"},
		IsolationTest: "TestRegistry_EveryRegisteredTypeIsTenantIsolated",
	},
	TypeIncident: {
		// The only sequential id in the drawer. incidents.id is an
		// auto-incrementing integer, so a caller holding one id can name every
		// other one — the shape that made /incidents/:id/timeline and
		// /incidents/:id/actions read-and-write IDOR on 2026-07-23
		// (docs/JOURNAL.md item 36).
		Type: TypeIncident, IDShape: IDShapeSequential,
		Gate: OwnColumn{Table: "incidents"},
		EnumerationMitigation: "incidents.tenant_id is filtered on every read, so walking " +
			"the id range answers 404 for every row outside the caller's tenant, " +
			"identically to a fabricated id",
		IsolationTest: "TestRegistry_EveryRegisteredTypeIsTenantIsolated",
	},
	TypeEvidence: {
		Type: TypeEvidence, IDShape: IDShapeUUID,
		Gate:          OwnColumn{Table: "evidences"},
		IsolationTest: "TestRegistry_EveryRegisteredTypeIsTenantIsolated",
	},
}

// ProfileFor returns the declared isolation profile for a type, without a
// resolver bound to it.
func ProfileFor(t Type) (Registration, bool) {
	reg, ok := isolationProfiles[t]
	return reg, ok
}

// Bind produces the Registration for a type by joining its declared isolation
// profile to a resolver.
//
// It panics on a type with no profile. A type that reached Types and a resolver
// but never had its tenant gate decided is the exact gap this issue closes, and
// failing at boot is the correct answer to it.
func Bind(t Type, res Resolver) Registration {
	reg, ok := isolationProfiles[t]
	if !ok {
		panic(fmt.Sprintf(
			"entity.Bind: type %q has no isolation profile: declare its tenant gate "+
				"in isolationProfiles before registering it", t))
	}
	reg.Resolver = res
	return reg
}
