// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entity

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// =============================================================================
// TenantScope
// =============================================================================

// A tenant id of uuid.Nil is not a tenant. It is the zero value that a missing
// claim, an unparsed header or a failed middleware all produce, and it is one
// missing predicate away from matching every row in the table.
func TestTenantScope_RefusesTheNilTenant(t *testing.T) {
	_, err := NewTenantScope(Caller{UserID: uuid.New(), TenantID: uuid.Nil, Perms: perms("*")})
	if err == nil {
		t.Fatal("NewTenantScope accepted the nil tenant; every read derived from it would be unscoped")
	}
	if got := domain.HTTPStatusFromError(err); got != 401 {
		t.Fatalf("nil tenant answered %d, want 401 — the caller has not proven who they are, "+
			"which is a different statement from having proven it and lacking a permission", got)
	}
}

func TestTenantScope_CarriesTheCallerTenant(t *testing.T) {
	tenant := uuid.New()
	scope, err := NewTenantScope(Caller{UserID: uuid.New(), TenantID: tenant, Perms: perms("*")})
	if err != nil {
		t.Fatalf("NewTenantScope(valid caller) = %v", err)
	}
	if scope.UUID() != tenant {
		t.Fatalf("scope carries %s, want %s", scope.UUID(), tenant)
	}
	if scope.IsZero() {
		t.Fatal("a constructed scope reports itself zero")
	}
	if (TenantScope{}).IsZero() != true {
		t.Fatal("the zero TenantScope does not report itself zero")
	}
}

// =============================================================================
// TenantGate — closed to this package
// =============================================================================

// The gate set is closed by an unexported marker method. This test pins the two
// members; the property that a THIRD cannot be declared outside package entity
// is enforced by the compiler, not by this test — an external type cannot
// implement isTenantGate(), so `go build` rejects it. That is the guarantee, and
// it is why the marker is unexported rather than a string field someone can
// set to a new value.
func TestTenantGate_IsClosedToTheEntityPackage(t *testing.T) {
	var gates = []TenantGate{
		OwnColumn{Table: "assets"},
		ThroughParent{Parent: TypeRisk, Table: "risk_histories",
			GateFunc: "JOIN risks r ON r.id = h.risk_id AND r.tenant_id = ?"},
	}
	strategies := map[string]bool{}
	for _, g := range gates {
		if g.Strategy() == "" {
			t.Errorf("%T has an empty strategy name", g)
		}
		if g.Describe() == "" {
			t.Errorf("%T describes itself as nothing", g)
		}
		strategies[g.Strategy()] = true
	}
	if len(strategies) != 2 {
		t.Fatalf("the two gates report %d distinct strategies, want 2", len(strategies))
	}
}

// =============================================================================
// Register — fail closed by construction
// =============================================================================

// Every field Register validates is a decision somebody has to make. Omitting
// one is not a typo to be defaulted, it is a type nobody has decided how to
// scope — so Register panics rather than serving it.
func TestRegistry_IncompleteRegistrationPanics(t *testing.T) {
	complete := func() Registration {
		return Registration{
			Type:          TypeRisk,
			Resolver:      NewRiskResolver(newFakeRisks(), newFakeRelations()),
			IDShape:       IDShapeUUID,
			Gate:          OwnColumn{Table: "risks"},
			IsolationTest: "TestRegistry_EveryRegisteredTypeIsTenantIsolated",
		}
	}

	// The complete registration must NOT panic, or the subtests below would pass
	// for the wrong reason.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("a complete registration panicked: %v", r)
			}
		}()
		NewRegistry().Register(complete())
	}()

	cases := map[string]func(*Registration){
		"no Type":          func(r *Registration) { r.Type = "" },
		"no Resolver":      func(r *Registration) { r.Resolver = nil },
		"no IDShape":       func(r *Registration) { r.IDShape = "" },
		"no Gate":          func(r *Registration) { r.Gate = nil },
		"no IsolationTest": func(r *Registration) { r.IsolationTest = "" },
	}

	for name, omit := range cases {
		t.Run(name, func(t *testing.T) {
			reg := complete()
			omit(&reg)
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("Register accepted a registration with %s; "+
						"the type would have been served with that decision unmade", name)
				}
				if !strings.Contains(strings.ToLower(toString(r)), "incomplete registration") {
					t.Errorf("panic message %q does not say the registration was incomplete", r)
				}
			}()
			NewRegistry().Register(reg)
		})
	}
}

// A sequential id is enumerable by anyone holding one of them. That is not
// automatically a defect — it is a defect only if nothing stops the walk — so
// the registry demands the mitigation be STATED rather than assumed.
func TestRegistry_SequentialIDRequiresAnEnumerationMitigation(t *testing.T) {
	base := Registration{
		Type:          TypeIncident,
		Resolver:      NewIncidentResolver(newFakeIncidents(), newFakeRelations()),
		IDShape:       IDShapeSequential,
		Gate:          OwnColumn{Table: "incidents"},
		IsolationTest: "TestRegistry_EveryRegisteredTypeIsTenantIsolated",
	}

	t.Run("without a mitigation it panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("a sequential-id type was registered with no enumeration mitigation; " +
					"this is the exact shape that made /incidents/:id/timeline an IDOR")
			}
		}()
		NewRegistry().Register(base)
	})

	t.Run("with a mitigation it registers", func(t *testing.T) {
		reg := base
		reg.EnumerationMitigation = "tenant_id is filtered on every read"
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("a complete sequential registration panicked: %v", r)
			}
		}()
		NewRegistry().Register(reg)
	})

	// A uuid type needs no mitigation — the id is not guessable.
	t.Run("a uuid type needs none", func(t *testing.T) {
		reg := base
		reg.Type = TypeRisk
		reg.Resolver = NewRiskResolver(newFakeRisks(), newFakeRelations())
		reg.IDShape = IDShapeUUID
		reg.Gate = OwnColumn{Table: "risks"}
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("a uuid registration with no mitigation panicked: %v", r)
			}
		}()
		NewRegistry().Register(reg)
	})
}

// =============================================================================
// The derived isolation sweep — the criterion that closes the 2026-07-23 class
// =============================================================================

// Every REGISTERED type is read cross-tenant on every route that takes an id,
// with the case list derived from the registry rather than written out.
//
// A registration with no tenant-B fixture FAILS here. It is not skipped, and
// that distinction is the whole value of the test: a skip is how `vendor` was
// absent from the HTTP sweep while the suite stayed green.
func TestRegistry_EveryRegisteredTypeIsTenantIsolated(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	caller := w.admin(w.tenantA) // holds "*" — so a leak here is a TENANT failure, not a permission one

	regs := w.registry.Registrations()
	if len(regs) != len(Types) {
		t.Fatalf("registry holds %d of %d types; an unregistered type is unreachable "+
			"and its isolation is untested", len(regs), len(Types))
	}

	fx := w.fixtures()
	for _, reg := range regs {
		f, ok := fx[reg.Type]
		if !ok {
			t.Errorf("type %q is registered but has no tenant-B fixture: seed one in "+
				"world.fixtures() so this type's isolation is actually exercised", reg.Type)
			continue
		}

		t.Run(string(reg.Type), func(t *testing.T) {
			// Sanity: the tenant-A row must actually resolve, or a 404 below
			// would prove nothing at all.
			if _, err := w.svc.Get(ctx, caller, reg.Type, f.idA); err != nil {
				t.Fatalf("the tenant-A fixture does not resolve (%v); "+
					"the cross-tenant assertions below would pass vacuously", err)
			}

			routes := map[string]func(string) error{
				"entity": func(id string) error {
					_, err := w.svc.Get(ctx, caller, reg.Type, id)
					return err
				},
				"relations": func(id string) error {
					_, err := w.svc.Relations(ctx, caller, reg.Type, id)
					return err
				},
				"timeline": func(id string) error {
					_, err := w.svc.Timeline(ctx, caller, reg.Type, id, "", TimelineFilter{})
					return err
				},
				"audit": func(id string) error {
					_, err := w.svc.Audit(ctx, caller, reg.Type, id, 20, 0)
					return err
				},
			}

			for name, call := range routes {
				t.Run(name, func(t *testing.T) {
					err := call(f.idB)
					if got := domain.HTTPStatusFromError(err); got != 404 {
						t.Fatalf("a tenant-A caller reading tenant-B's %s over /%s answered %d, want 404 "+
							"— another tenant's row was reachable", reg.Type, name, got)
					}
				})
			}
		})
	}
}

// A registration's IsolationTest must name a test that exists. Without this the
// field is a comment: it would keep naming a function through the rename that
// deleted it, and the registry would still claim the type was proven isolated.
func TestRegistry_IsolationTestNamesAnExistingFunction(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parsing the package's test files: %v", err)
	}

	declared := map[string]bool{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, d := range file.Decls {
				if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil {
					declared[fn.Name.Name] = true
				}
			}
		}
	}
	if len(declared) == 0 {
		t.Fatal("the AST scan found no test functions at all; the check would pass vacuously")
	}

	for _, t2 := range Types {
		reg, ok := ProfileFor(t2)
		if !ok {
			t.Errorf("type %q has no isolation profile", t2)
			continue
		}
		if reg.IsolationTest == "" {
			t.Errorf("type %q names no isolation test", t2)
			continue
		}
		if !declared[reg.IsolationTest] {
			t.Errorf("type %q names isolation test %q, which does not exist in this package "+
				"— it was renamed or deleted and the registration still claims it proves the gate",
				t2, reg.IsolationTest)
		}
	}
}

// Every profile states a gate, and a gate that describes itself as nothing is
// not a decision.
func TestRegistry_EveryTypeDeclaresAGate(t *testing.T) {
	for _, ty := range Types {
		reg, ok := ProfileFor(ty)
		if !ok {
			t.Errorf("type %q has no isolation profile", ty)
			continue
		}
		if reg.Gate == nil {
			t.Errorf("type %q declares no tenant gate", ty)
			continue
		}
		if reg.Gate.Describe() == "" {
			t.Errorf("type %q has a gate that describes itself as nothing", ty)
		}
		if reg.IDShape == IDShapeSequential && reg.EnumerationMitigation == "" {
			t.Errorf("type %q has sequential ids and states no enumeration mitigation", ty)
		}
	}
}

// =============================================================================
// The tenantless caller is 401, not 403
// =============================================================================

// Replaces TestGet_TenantlessCallerIsRefused, which asserted only err != nil and
// so could not tell 401 from 403 from 500.
func TestGet_TenantlessCallerIs401(t *testing.T) {
	w := newWorld(t)
	c := Caller{UserID: uuid.New(), TenantID: uuid.Nil, Perms: perms("*")}
	ctx := context.Background()

	routes := map[string]func() error{
		"entity": func() error {
			_, err := w.svc.Get(ctx, c, TypeRisk, w.riskA.ID.String())
			return err
		},
		"relations": func() error {
			_, err := w.svc.Relations(ctx, c, TypeRisk, w.riskA.ID.String())
			return err
		},
		"timeline": func() error {
			_, err := w.svc.Timeline(ctx, c, TypeRisk, w.riskA.ID.String(), "", TimelineFilter{})
			return err
		},
		"audit": func() error {
			_, err := w.svc.Audit(ctx, c, TypeRisk, w.riskA.ID.String(), 20, 0)
			return err
		},
	}

	for name, call := range routes {
		t.Run(name, func(t *testing.T) {
			if got := domain.HTTPStatusFromError(call()); got != 401 {
				t.Fatalf("/%s served a tenantless caller %d, want 401", name, got)
			}
		})
	}
}

// toString renders a recovered panic value for message assertions.
func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if e, ok := v.(error); ok {
		return e.Error()
	}
	return ""
}
