// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// #1415 — the implementation and test-definition use cases, against an in-memory
// repository that honours the tenant the way the real one does.

type fakeChain struct {
	ctl         *fakeRepo
	assets      map[uuid.UUID]uuid.UUID // asset id -> tenant
	impls       map[uuid.UUID]ImplementationRecord
	implDeleted map[uuid.UUID]bool
	tests       map[uuid.UUID]canonical.ControlTest
	testDeleted map[uuid.UUID]bool
	audits      []*domain.AuditEvent
	writes      int
	stale       map[uuid.UUID][]uuid.UUID // implementation -> targets whose asset was deleted
}

func newFakeChain(ctl *fakeRepo) *fakeChain {
	return &fakeChain{
		ctl: ctl, assets: map[uuid.UUID]uuid.UUID{}, impls: map[uuid.UUID]ImplementationRecord{},
		implDeleted: map[uuid.UUID]bool{}, tests: map[uuid.UUID]canonical.ControlTest{}, testDeleted: map[uuid.UUID]bool{},
		stale: map[uuid.UUID][]uuid.UUID{},
	}
}

func implNotFound() error {
	return domain.InternalControlError(domain.ErrNotFound, domain.IntCtlImplementationNotFound)
}
func testNotFoundErr() error {
	return domain.InternalControlError(domain.ErrNotFound, domain.IntCtlTestNotFound)
}

func (f *fakeChain) liveImpl(tenant, id uuid.UUID) (ImplementationRecord, bool) {
	r, ok := f.impls[id]
	if !ok || r.Implementation.TenantID != tenant || f.implDeleted[id] {
		return r, false
	}
	_, live := f.ctl.live(tenant, r.Implementation.ControlID)
	return r, live
}

func (f *fakeChain) resolveAssets(tenant uuid.UUID, ids []uuid.UUID) error {
	for _, id := range ids {
		if t, ok := f.assets[id]; !ok || t != tenant {
			return domain.NewNotFoundError("Asset", id)
		}
	}
	return nil
}

func edgeTargets(es []canonical.Edge) []uuid.UUID {
	out := make([]uuid.UUID, len(es))
	for i, e := range es {
		out[i] = e.To.ID
	}
	return out
}

func (f *fakeChain) CreateImplementation(_ context.Context, tenant uuid.UUID, impl canonical.ControlImplementation, edges []canonical.Edge, a *domain.AuditEvent) error {
	if _, ok := f.ctl.live(tenant, impl.ControlID); !ok {
		return notFound()
	}
	targets := edgeTargets(edges)
	if err := f.resolveAssets(tenant, targets); err != nil {
		return err
	}
	sortIDs(targets)
	f.impls[impl.ID] = ImplementationRecord{Implementation: impl, AppliesTo: targets}
	f.audits = append(f.audits, a)
	f.writes++
	return nil
}

func (f *fakeChain) GetImplementation(_ context.Context, tenant, id uuid.UUID) (*ImplementationRecord, error) {
	r, ok := f.liveImpl(tenant, id)
	if !ok {
		return nil, implNotFound()
	}
	return &r, nil
}

func (f *fakeChain) ListImplementations(_ context.Context, tenant, control uuid.UUID, limit, offset int) ([]ImplementationRecord, error) {
	if _, ok := f.ctl.live(tenant, control); !ok {
		return nil, notFound()
	}
	var out []ImplementationRecord
	for id, r := range f.impls {
		if _, ok := f.liveImpl(tenant, id); ok && r.Implementation.ControlID == control {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Implementation.ID.String() < out[j].Implementation.ID.String() })
	if offset > len(out) {
		offset = len(out)
	}
	out = out[offset:]
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeChain) UpdateImplementation(_ context.Context, tenant, id uuid.UUID,
	apply func(ImplementationRecord) (canonical.ControlImplementation, []canonical.Edge, []uuid.UUID, *domain.AuditEvent, error),
) (*ImplementationRecord, error) {
	cur, ok := f.liveImpl(tenant, id)
	if !ok {
		return nil, implNotFound()
	}
	cur.StaleTargets = f.stale[id]
	next, add, remove, ev, err := apply(cur)
	if err != nil {
		return nil, err
	}
	if ev == nil {
		return &cur, nil
	}
	if err := f.resolveAssets(tenant, edgeTargets(add)); err != nil {
		return nil, err
	}
	set := map[uuid.UUID]bool{}
	for _, a := range cur.AppliesTo {
		set[a] = true
	}
	for _, a := range remove {
		delete(set, a)
	}
	for _, a := range edgeTargets(add) {
		set[a] = true
	}
	var targets []uuid.UUID
	for a := range set {
		targets = append(targets, a)
	}
	sortIDs(targets)
	rec := ImplementationRecord{Implementation: next, AppliesTo: targets}
	f.impls[id] = rec
	if len(remove) > 0 {
		f.stale[id] = nil
	}
	f.audits = append(f.audits, ev)
	f.writes++
	return &rec, nil
}

func (f *fakeChain) DeleteImplementation(_ context.Context, tenant, id uuid.UUID, apply func(ImplementationRecord, []canonical.ControlTest) ([]*domain.AuditEvent, error)) error {
	cur, ok := f.liveImpl(tenant, id)
	if !ok {
		return implNotFound()
	}
	var tests []canonical.ControlTest
	var testIDs []uuid.UUID
	for tid, t := range f.tests {
		if t.TenantID == tenant && t.ImplementationID == id && !f.testDeleted[tid] {
			tests = append(tests, t)
			testIDs = append(testIDs, tid)
		}
	}
	sort.Slice(tests, func(i, j int) bool { return tests[i].ID.String() < tests[j].ID.String() })
	evs, err := apply(cur, tests)
	if err != nil {
		return err
	}
	f.implDeleted[id] = true
	for _, tid := range testIDs {
		f.testDeleted[tid] = true
	}
	f.audits = append(f.audits, evs...)
	f.writes++
	return nil
}

func (f *fakeChain) liveTest(tenant, id uuid.UUID) (canonical.ControlTest, bool) {
	t, ok := f.tests[id]
	if !ok || t.TenantID != tenant || f.testDeleted[id] {
		return t, false
	}
	_, live := f.liveImpl(tenant, t.ImplementationID)
	return t, live
}

func (f *fakeChain) CreateTest(_ context.Context, tenant uuid.UUID, t canonical.ControlTest, a *domain.AuditEvent) error {
	if _, ok := f.liveImpl(tenant, t.ImplementationID); !ok {
		return implNotFound()
	}
	f.tests[t.ID] = t
	f.audits = append(f.audits, a)
	f.writes++
	return nil
}

func (f *fakeChain) GetTest(_ context.Context, tenant, id uuid.UUID) (*canonical.ControlTest, error) {
	t, ok := f.liveTest(tenant, id)
	if !ok {
		return nil, testNotFoundErr()
	}
	return &t, nil
}

func (f *fakeChain) ListTests(_ context.Context, tenant, impl uuid.UUID, limit, offset int) ([]canonical.ControlTest, error) {
	if _, ok := f.liveImpl(tenant, impl); !ok {
		return nil, implNotFound()
	}
	var out []canonical.ControlTest
	for id, t := range f.tests {
		if _, ok := f.liveTest(tenant, id); ok && t.ImplementationID == impl {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if offset > len(out) {
		offset = len(out)
	}
	out = out[offset:]
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeChain) UpdateTest(_ context.Context, tenant, id uuid.UUID,
	apply func(canonical.ControlTest) (canonical.ControlTest, *domain.AuditEvent, error),
) (*canonical.ControlTest, error) {
	cur, ok := f.liveTest(tenant, id)
	if !ok {
		return nil, testNotFoundErr()
	}
	next, ev, err := apply(cur)
	if err != nil {
		return nil, err
	}
	if ev != nil {
		f.tests[id] = next
		f.audits = append(f.audits, ev)
		f.writes++
	}
	return &next, nil
}

func (f *fakeChain) DeleteTest(_ context.Context, tenant, id uuid.UUID, apply func(canonical.ControlTest) (*domain.AuditEvent, error)) error {
	cur, ok := f.liveTest(tenant, id)
	if !ok {
		return testNotFoundErr()
	}
	ev, err := apply(cur)
	if err != nil {
		return err
	}
	f.testDeleted[id] = true
	f.audits = append(f.audits, ev)
	f.writes++
	return nil
}

// chainFixture is a control of tenant A, an asset of each tenant, and the use
// cases' collaborators.
type chainFixture struct {
	*fixture
	chain   *fakeChain
	control *ControlView
	assetA  uuid.UUID
	assetA2 uuid.UUID
	assetB  uuid.UUID
}

func newChainFixture() *chainFixture {
	f := newFixture()
	cf := &chainFixture{fixture: f, chain: newFakeChain(f.repo)}
	cf.control = f.create(ctlInput("Access reviews"))
	cf.assetA, cf.assetA2, cf.assetB = uuid.New(), uuid.New(), uuid.New()
	cf.chain.assets[cf.assetA] = tenantA
	cf.chain.assets[cf.assetA2] = tenantA
	cf.chain.assets[cf.assetB] = tenantB
	return cf
}

func (cf *chainFixture) implInput(assets ...uuid.UUID) CreateImplementationInput {
	in := CreateImplementationInput{Description: "Quarterly access review on core banking"}
	for _, a := range assets {
		in.AppliesTo = append(in.AppliesTo, TargetRef{Type: "Asset", ID: a})
	}
	return in
}

func (cf *chainFixture) newImpl(assets ...uuid.UUID) *ImplementationView {
	v, err := NewCreateImplementationUseCase(cf.chain, cf.owners, clock).Execute(context.Background(), cf.caller, cf.control.ID, cf.implInput(assets...))
	if err != nil {
		panic(err)
	}
	return v
}

func manualTest(name string) CreateTestInput {
	return CreateTestInput{Name: name, Checks: "access list matches HR", Method: "manual", Procedure: str("export the list, compare"), PassCriteria: "no orphan account"}
}

func (cf *chainFixture) newTest(implID uuid.UUID) *TestView {
	v, err := NewCreateTestUseCase(cf.chain, clock).Execute(context.Background(), cf.caller, implID, manualTest("Quarterly review"))
	if err != nil {
		panic(err)
	}
	return v
}

var noPerms = func(c Caller) Caller { c.Permissions = nil; return c }

// ---------------------------------------------------------------------------
// CreateImplementation

func TestCreateControlImplementation_Success(t *testing.T) {
	cf := newChainFixture()
	in := cf.implInput(cf.assetA, cf.assetA2, cf.assetA) // a duplicate collapses
	in.OwnerID = &cf.owner
	in.Status = "in_place"
	in.InPlaceSince = str("2026-09-30")
	v, err := NewCreateImplementationUseCase(cf.chain, cf.owners, clock).Execute(context.Background(), cf.caller, cf.control.ID, in)
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, v.ID)
	assert.Equal(t, cf.control.ID, v.ControlID)
	assert.Equal(t, canonical.ImplementationInPlace, v.Status)
	assert.Equal(t, canonical.Date("2026-09-30"), *v.InPlaceSince)
	assert.Len(t, v.AppliesTo, 2)
	// The server-set envelope.
	assert.Equal(t, canonical.SourceOpenRiskUser, v.Source)
	assert.Equal(t, v.ID.String(), v.SourceID)
	assert.Equal(t, 0.70, v.Confidence)
	assert.Equal(t, canonical.Actor{Kind: canonical.ActorUser, ID: cf.caller.UserID.String()}, v.Provenance.Actor)
	assert.Nil(t, v.ObservedAt, "not confirmed yet")

	stored := cf.chain.impls[v.ID].Implementation
	assert.Equal(t, tenantA, stored.TenantID)
	require.Len(t, cf.chain.audits, 1)
	assert.Equal(t, "control_implementation", cf.chain.audits[0].EntityType)
	assert.Equal(t, domain.AuditActionCreate, cf.chain.audits[0].Action)
	assert.Equal(t, tenantA, cf.chain.audits[0].TenantID)

	got, err := NewGetImplementationUseCase(cf.chain).Execute(context.Background(), cf.caller, v.ID)
	require.NoError(t, err)
	assert.Equal(t, v.AppliesTo, got.AppliesTo, "it reads back with its targets")
}

func TestCreateControlImplementation_NotFound(t *testing.T) {
	cf := newChainFixture()
	uc := NewCreateImplementationUseCase(cf.chain, cf.owners, clock)
	// Another tenant's control, an absent one, and another tenant's asset are all 404, and nothing is written.
	_, err := uc.Execute(context.Background(), cf.other, cf.control.ID, cf.implInput(cf.assetB))
	require.ErrorIs(t, err, domain.ErrNotFound, "another tenant's control")
	_, err = uc.Execute(context.Background(), cf.caller, uuid.New(), cf.implInput(cf.assetA))
	require.ErrorIs(t, err, domain.ErrNotFound, "an absent control")
	_, err = uc.Execute(context.Background(), cf.caller, cf.control.ID, cf.implInput(cf.assetA, cf.assetB))
	require.ErrorIs(t, err, domain.ErrNotFound, "another tenant's asset")
	_, err = uc.Execute(context.Background(), cf.caller, cf.control.ID, cf.implInput(uuid.New()))
	require.ErrorIs(t, err, domain.ErrNotFound, "a fabricated asset")
	assert.Zero(t, cf.chain.writes)
	assert.Empty(t, cf.chain.impls)
}

func TestCreateControlImplementation_Unauthorized(t *testing.T) {
	cf := newChainFixture()
	uc := NewCreateImplementationUseCase(cf.chain, cf.owners, clock)
	for name, c := range map[string]Caller{
		"no tenant": {UserID: uuid.New(), Permissions: cf.caller.Permissions},
		"no user":   {TenantID: tenantA, Permissions: cf.caller.Permissions},
	} {
		_, err := uc.Execute(context.Background(), c, cf.control.ID, cf.implInput(cf.assetA))
		require.ErrorIs(t, err, domain.ErrUnauthorized, name)
	}
	readOnly := cf.caller
	readOnly.Permissions = []string{PermissionRead}
	_, err := uc.Execute(context.Background(), readOnly, cf.control.ID, cf.implInput(cf.assetA))
	require.ErrorIs(t, err, domain.ErrForbidden)
	assert.Zero(t, cf.chain.writes)
}

// AC2: an implementation without at least one applies_to asset is refused.
func TestCreateControlImplementation_RequiresAnAsset(t *testing.T) {
	cf := newChainFixture()
	uc := NewCreateImplementationUseCase(cf.chain, cf.owners, clock)
	for name, in := range map[string]CreateImplementationInput{
		"no targets":      cf.implInput(),
		"nil asset id":    {Description: "d", AppliesTo: []TargetRef{{Type: "Asset", ID: uuid.Nil}}},
		"system target":   {Description: "d", AppliesTo: []TargetRef{{Type: "System", ID: cf.assetA}}},
		"too many":        {Description: "d", AppliesTo: make([]TargetRef, MaxAppliesTo+1)},
		"blank text":      {Description: "  ", AppliesTo: []TargetRef{{ID: cf.assetA}}},
		"unknown status":  {Description: "d", Status: "done", AppliesTo: []TargetRef{{ID: cf.assetA}}},
		"bad date":        {Description: "d", InPlaceSince: str("30/09/2026"), AppliesTo: []TargetRef{{ID: cf.assetA}}},
		"too long":        {Description: string(make([]rune, maxLongText+1)), AppliesTo: []TargetRef{{ID: cf.assetA}}},
		"owner not found": {Description: "d", OwnerID: new(uuid.UUID), AppliesTo: []TargetRef{{ID: cf.assetA}}},
	} {
		_, err := uc.Execute(context.Background(), cf.caller, cf.control.ID, in)
		require.ErrorIs(t, err, domain.ErrValidation, name)
	}
	_, err := uc.Execute(context.Background(), cf.caller, cf.control.ID, cf.implInput())
	assert.Equal(t, domain.IntCtlAppliesToRequired, code(err))
	assert.Zero(t, cf.chain.writes)

	// An empty type is an asset.
	_, err = uc.Execute(context.Background(), cf.caller, cf.control.ID, CreateImplementationInput{Description: "d", AppliesTo: []TargetRef{{ID: cf.assetA}}})
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Get / List

func TestGetControlImplementation_Success(t *testing.T) {
	cf := newChainFixture()
	v := cf.newImpl(cf.assetA)
	got, err := NewGetImplementationUseCase(cf.chain).Execute(context.Background(), cf.caller, v.ID)
	require.NoError(t, err)
	assert.Equal(t, v.ID, got.ID)
	assert.Equal(t, []TargetView{{Type: canonical.TypeAsset, ID: cf.assetA}}, got.AppliesTo)
}

func TestGetControlImplementation_NotFound(t *testing.T) {
	cf := newChainFixture()
	v := cf.newImpl(cf.assetA)
	uc := NewGetImplementationUseCase(cf.chain)
	_, err := uc.Execute(context.Background(), cf.other, v.ID)
	require.ErrorIs(t, err, domain.ErrNotFound, "another tenant's id reads as absent")
	_, err = uc.Execute(context.Background(), cf.caller, uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound)

	// An implementation of a deleted control reads as absent.
	require.NoError(t, NewDeleteUseCase(cf.repo, clock).Execute(context.Background(), cf.caller, cf.control.ID))
	_, err = uc.Execute(context.Background(), cf.caller, v.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetControlImplementation_Unauthorized(t *testing.T) {
	cf := newChainFixture()
	v := cf.newImpl(cf.assetA)
	uc := NewGetImplementationUseCase(cf.chain)
	_, err := uc.Execute(context.Background(), Caller{UserID: uuid.New(), Permissions: cf.caller.Permissions}, v.ID)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
	_, err = uc.Execute(context.Background(), noPerms(cf.caller), v.ID)
	require.ErrorIs(t, err, domain.ErrForbidden)
}

func TestListControlImplementations_Success(t *testing.T) {
	cf := newChainFixture()
	cf.newImpl(cf.assetA)
	cf.newImpl(cf.assetA2)
	vs, err := NewListImplementationsUseCase(cf.chain).Execute(context.Background(), cf.caller, cf.control.ID, 0, 0)
	require.NoError(t, err)
	assert.Len(t, vs, 2)
	_, err = NewListImplementationsUseCase(cf.chain).Execute(context.Background(), cf.caller, cf.control.ID, -1, 0)
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestListControlImplementations_NotFound(t *testing.T) {
	cf := newChainFixture()
	cf.newImpl(cf.assetA)
	uc := NewListImplementationsUseCase(cf.chain)
	_, err := uc.Execute(context.Background(), cf.other, cf.control.ID, 0, 0)
	require.ErrorIs(t, err, domain.ErrNotFound, "another tenant's control")
	_, err = uc.Execute(context.Background(), cf.caller, uuid.New(), 0, 0)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestListControlImplementations_Unauthorized(t *testing.T) {
	cf := newChainFixture()
	uc := NewListImplementationsUseCase(cf.chain)
	_, err := uc.Execute(context.Background(), Caller{UserID: uuid.New(), Permissions: cf.caller.Permissions}, cf.control.ID, 0, 0)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
	_, err = uc.Execute(context.Background(), noPerms(cf.caller), cf.control.ID, 0, 0)
	require.ErrorIs(t, err, domain.ErrForbidden)
}

// ---------------------------------------------------------------------------
// Update

func TestUpdateControlImplementation_Success(t *testing.T) {
	cf := newChainFixture()
	v := cf.newImpl(cf.assetA)
	uc := NewUpdateImplementationUseCase(cf.chain, cf.owners, clock)

	out, err := uc.Execute(context.Background(), cf.caller, v.ID, UpdateImplementationInput{
		Status: str("in_place"), InPlaceSince: str("2026-10-01"),
		Ownership: domain.OwnershipPatch{Owner: domain.Assign(cf.owner)},
	})
	require.NoError(t, err)
	assert.Equal(t, canonical.ImplementationInPlace, out.Status)
	assert.Equal(t, cf.owner, *out.OwnerID)
	assert.Equal(t, []TargetView{{Type: canonical.TypeAsset, ID: cf.assetA}}, out.AppliesTo, "targets untouched when not supplied")
	last := cf.chain.audits[len(cf.chain.audits)-1]
	assert.Equal(t, domain.AuditActionUpdate, last.Action)
	assert.Contains(t, last.ChangedFields, "status")

	// Replacing the targets adds one, removes one, and the audit entry journals both sets.
	repl := []TargetRef{{ID: cf.assetA2}}
	out, err = uc.Execute(context.Background(), cf.caller, v.ID, UpdateImplementationInput{AppliesTo: &repl})
	require.NoError(t, err)
	assert.Equal(t, []TargetView{{Type: canonical.TypeAsset, ID: cf.assetA2}}, out.AppliesTo)
	last = cf.chain.audits[len(cf.chain.audits)-1]
	assert.Contains(t, last.ChangedFields, "applies_to")
	assert.Equal(t, []string{cf.assetA.String()}, last.Before["applies_to"])
	assert.Equal(t, []string{cf.assetA2.String()}, last.After["applies_to"])

	// A blank in_place_since clears it; an edit that changes nothing writes nothing.
	out, err = uc.Execute(context.Background(), cf.caller, v.ID, UpdateImplementationInput{InPlaceSince: str("")})
	require.NoError(t, err)
	assert.Nil(t, out.InPlaceSince)
	writes := cf.chain.writes
	_, err = uc.Execute(context.Background(), cf.caller, v.ID, UpdateImplementationInput{Status: str("in_place")})
	require.NoError(t, err)
	assert.Equal(t, writes, cf.chain.writes)
}

// A replacement also removes, and journals, the edge to a deleted asset.
func TestUpdateControlImplementation_JournalsTheEdgeOfADeletedAsset(t *testing.T) {
	cf := newChainFixture()
	v := cf.newImpl(cf.assetA)
	ghost := uuid.New()
	cf.chain.stale[v.ID] = []uuid.UUID{ghost}
	repl := []TargetRef{{ID: cf.assetA2}}
	_, err := NewUpdateImplementationUseCase(cf.chain, cf.owners, clock).Execute(context.Background(), cf.caller, v.ID, UpdateImplementationInput{AppliesTo: &repl})
	require.NoError(t, err)
	last := cf.chain.audits[len(cf.chain.audits)-1]
	before, _ := last.Before["applies_to"].([]string)
	assert.ElementsMatch(t, []string{cf.assetA.String(), ghost.String()}, before, "the stale edge is in the before set")
	assert.Empty(t, cf.chain.stale[v.ID], "and was removed")
}

func TestUpdateControlImplementation_NotFound(t *testing.T) {
	cf := newChainFixture()
	v := cf.newImpl(cf.assetA)
	uc := NewUpdateImplementationUseCase(cf.chain, cf.owners, clock)
	_, err := uc.Execute(context.Background(), cf.other, v.ID, UpdateImplementationInput{Status: str("retired")})
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = uc.Execute(context.Background(), cf.caller, uuid.New(), UpdateImplementationInput{Status: str("retired")})
	require.ErrorIs(t, err, domain.ErrNotFound)

	// Another tenant's asset in a replacement is 404 and the stored set is unchanged.
	repl := []TargetRef{{ID: cf.assetB}}
	_, err = uc.Execute(context.Background(), cf.caller, v.ID, UpdateImplementationInput{AppliesTo: &repl})
	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Equal(t, []uuid.UUID{cf.assetA}, cf.chain.impls[v.ID].AppliesTo)
	assert.Equal(t, canonical.ImplementationPlanned, cf.chain.impls[v.ID].Implementation.Status)
}

func TestUpdateControlImplementation_Unauthorized(t *testing.T) {
	cf := newChainFixture()
	v := cf.newImpl(cf.assetA)
	uc := NewUpdateImplementationUseCase(cf.chain, cf.owners, clock)
	_, err := uc.Execute(context.Background(), Caller{UserID: uuid.New(), Permissions: cf.caller.Permissions}, v.ID, UpdateImplementationInput{Status: str("retired")})
	require.ErrorIs(t, err, domain.ErrUnauthorized)
	readOnly := cf.caller
	readOnly.Permissions = []string{PermissionRead}
	_, err = uc.Execute(context.Background(), readOnly, v.ID, UpdateImplementationInput{Status: str("retired")})
	require.ErrorIs(t, err, domain.ErrForbidden)
	assert.Equal(t, canonical.ImplementationPlanned, cf.chain.impls[v.ID].Implementation.Status)
}

func TestUpdateControlImplementation_RejectsBadInput(t *testing.T) {
	cf := newChainFixture()
	v := cf.newImpl(cf.assetA)
	uc := NewUpdateImplementationUseCase(cf.chain, cf.owners, clock)
	empty := []TargetRef{}
	system := []TargetRef{{Type: "Service", ID: cf.assetA}}
	for name, in := range map[string]UpdateImplementationInput{
		"empty target set": {AppliesTo: &empty},
		"service target":   {AppliesTo: &system},
		"blank text":       {Description: str(" ")},
		"unknown status":   {Status: str("done")},
		"bad date":         {InPlaceSince: str("soon")},
		"foreign owner":    {Ownership: domain.OwnershipPatch{Owner: domain.Assign(uuid.New())}},
	} {
		_, err := uc.Execute(context.Background(), cf.caller, v.ID, in)
		require.ErrorIs(t, err, domain.ErrValidation, name)
	}
	assert.Equal(t, []uuid.UUID{cf.assetA}, cf.chain.impls[v.ID].AppliesTo, "an update never leaves the set empty")
}

// ---------------------------------------------------------------------------
// Delete

func TestDeleteControlImplementation_Success(t *testing.T) {
	cf := newChainFixture()
	v := cf.newImpl(cf.assetA)
	test := cf.newTest(v.ID)
	require.NoError(t, NewDeleteImplementationUseCase(cf.chain, clock).Execute(context.Background(), cf.caller, v.ID))

	assert.True(t, cf.chain.implDeleted[v.ID])
	assert.True(t, cf.chain.testDeleted[test.ID], "its live tests go with it")
	n := len(cf.chain.audits)
	require.GreaterOrEqual(t, n, 4)
	assert.Equal(t, "control_test", cf.chain.audits[n-1].EntityType, "one entry per test")
	assert.Equal(t, "Quarterly review", cf.chain.audits[n-1].Before["name"], "the cascaded entry carries the full before image")
	assert.Equal(t, "manual", cf.chain.audits[n-1].Before["method"])
	assert.NotEmpty(t, cf.chain.audits[n-1].Before["checks"])
	assert.Equal(t, "control_implementation", cf.chain.audits[n-2].EntityType)
	assert.Equal(t, domain.AuditActionDelete, cf.chain.audits[n-2].Action)
	_, err := NewGetImplementationUseCase(cf.chain).Execute(context.Background(), cf.caller, v.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = NewGetTestUseCase(cf.chain).Execute(context.Background(), cf.caller, test.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestDeleteControlImplementation_NotFound(t *testing.T) {
	cf := newChainFixture()
	v := cf.newImpl(cf.assetA)
	uc := NewDeleteImplementationUseCase(cf.chain, clock)
	require.ErrorIs(t, uc.Execute(context.Background(), cf.other, v.ID), domain.ErrNotFound)
	require.ErrorIs(t, uc.Execute(context.Background(), cf.caller, uuid.New()), domain.ErrNotFound)
	assert.False(t, cf.chain.implDeleted[v.ID])
	require.NoError(t, uc.Execute(context.Background(), cf.caller, v.ID))
	require.ErrorIs(t, uc.Execute(context.Background(), cf.caller, v.ID), domain.ErrNotFound, "twice")
}

func TestDeleteControlImplementation_Unauthorized(t *testing.T) {
	cf := newChainFixture()
	v := cf.newImpl(cf.assetA)
	uc := NewDeleteImplementationUseCase(cf.chain, clock)
	require.ErrorIs(t, uc.Execute(context.Background(), Caller{UserID: uuid.New(), Permissions: cf.caller.Permissions}, v.ID), domain.ErrUnauthorized)
	updateOnly := cf.caller
	updateOnly.Permissions = []string{PermissionUpdate}
	require.ErrorIs(t, uc.Execute(context.Background(), updateOnly, v.ID), domain.ErrForbidden)
	assert.False(t, cf.chain.implDeleted[v.ID])
}

// ---------------------------------------------------------------------------
// Tests: create

func TestCreateControlTest_Success(t *testing.T) {
	cf := newChainFixture()
	impl := cf.newImpl(cf.assetA)
	in := manualTest("Quarterly review")
	in.Cadence = str("P3M")
	v, err := NewCreateTestUseCase(cf.chain, clock).Execute(context.Background(), cf.caller, impl.ID, in)
	require.NoError(t, err)
	assert.Equal(t, impl.ID, v.ImplementationID)
	assert.Equal(t, canonical.TestManual, v.Method)
	assert.Equal(t, "P3M", *v.Cadence)
	assert.Equal(t, canonical.SourceOpenRiskUser, v.Source)
	assert.Equal(t, v.ID.String(), v.SourceID)
	assert.Equal(t, 0.70, v.Confidence)
	assert.Equal(t, canonical.Actor{Kind: canonical.ActorUser, ID: cf.caller.UserID.String()}, v.Provenance.Actor)
	assert.Equal(t, tenantA, cf.chain.tests[v.ID].TenantID)
	last := cf.chain.audits[len(cf.chain.audits)-1]
	assert.Equal(t, "control_test", last.EntityType)
	assert.Equal(t, domain.AuditActionCreate, last.Action)

	// An automated test with a collector and no cadence.
	auto, err := NewCreateTestUseCase(cf.chain, clock).Execute(context.Background(), cf.caller, impl.ID,
		CreateTestInput{Name: "Scan", Checks: "no open ports", Method: "automated", Collector: str("nessus"), PassCriteria: "zero findings"})
	require.NoError(t, err)
	assert.Nil(t, auto.Cadence)
}

func TestCreateControlTest_NotFound(t *testing.T) {
	cf := newChainFixture()
	impl := cf.newImpl(cf.assetA)
	uc := NewCreateTestUseCase(cf.chain, clock)
	_, err := uc.Execute(context.Background(), cf.other, impl.ID, manualTest("t"))
	require.ErrorIs(t, err, domain.ErrNotFound, "another tenant's implementation")
	_, err = uc.Execute(context.Background(), cf.caller, uuid.New(), manualTest("t"))
	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Empty(t, cf.chain.tests)
}

func TestCreateControlTest_Unauthorized(t *testing.T) {
	cf := newChainFixture()
	impl := cf.newImpl(cf.assetA)
	uc := NewCreateTestUseCase(cf.chain, clock)
	_, err := uc.Execute(context.Background(), Caller{UserID: uuid.New(), Permissions: cf.caller.Permissions}, impl.ID, manualTest("t"))
	require.ErrorIs(t, err, domain.ErrUnauthorized)
	readOnly := cf.caller
	readOnly.Permissions = []string{PermissionRead}
	_, err = uc.Execute(context.Background(), readOnly, impl.ID, manualTest("t"))
	require.ErrorIs(t, err, domain.ErrForbidden)
	assert.Empty(t, cf.chain.tests)
}

// AC4: checks, pass_criteria, the procedure of a manual test, the collector of an
// automated one; cadence is optional and ISO 8601.
func TestCreateControlTest_RejectsBadInput(t *testing.T) {
	cf := newChainFixture()
	impl := cf.newImpl(cf.assetA)
	uc := NewCreateTestUseCase(cf.chain, clock)
	mut := func(f func(in *CreateTestInput)) CreateTestInput { in := manualTest("t"); f(&in); return in }
	for name, in := range map[string]CreateTestInput{
		"no checks":              mut(func(in *CreateTestInput) { in.Checks = " " }),
		"no pass criteria":       mut(func(in *CreateTestInput) { in.PassCriteria = "" }),
		"manual without proc.":   mut(func(in *CreateTestInput) { in.Procedure = nil }),
		"manual blank proc.":     mut(func(in *CreateTestInput) { in.Procedure = str("  ") }),
		"automated no collector": mut(func(in *CreateTestInput) { in.Method = "automated"; in.Procedure = nil }),
		"unknown method":         mut(func(in *CreateTestInput) { in.Method = "magic" }),
		"cadence not ISO 8601":   mut(func(in *CreateTestInput) { in.Cadence = str("monthly") }),
		"cadence zero":           mut(func(in *CreateTestInput) { in.Cadence = str("P0D") }),
		"no name":                mut(func(in *CreateTestInput) { in.Name = "" }),
		"collector too long": mut(func(in *CreateTestInput) {
			in.Method = "automated"
			in.Collector = str(string(make([]rune, maxCollectorLen+1)))
		}),
		"cadence too long": mut(func(in *CreateTestInput) { in.Cadence = str("P" + strings.Repeat("1", maxCadenceLen) + "D") }),
		"checks too long":  mut(func(in *CreateTestInput) { in.Checks = string(make([]rune, maxLongText+1)) }),
	} {
		_, err := uc.Execute(context.Background(), cf.caller, impl.ID, in)
		require.ErrorIs(t, err, domain.ErrValidation, name)
	}
	assert.Empty(t, cf.chain.tests)
	// A blank cadence is none.
	v, err := uc.Execute(context.Background(), cf.caller, impl.ID, mut(func(in *CreateTestInput) { in.Cadence = str(" ") }))
	require.NoError(t, err)
	assert.Nil(t, v.Cadence)
}

// ---------------------------------------------------------------------------
// Tests: get / list / update / delete

func TestGetControlTest_Success(t *testing.T) {
	cf := newChainFixture()
	test := cf.newTest(cf.newImpl(cf.assetA).ID)
	got, err := NewGetTestUseCase(cf.chain).Execute(context.Background(), cf.caller, test.ID)
	require.NoError(t, err)
	assert.Equal(t, test.ID, got.ID)
}

func TestGetControlTest_NotFound(t *testing.T) {
	cf := newChainFixture()
	impl := cf.newImpl(cf.assetA)
	test := cf.newTest(impl.ID)
	uc := NewGetTestUseCase(cf.chain)
	_, err := uc.Execute(context.Background(), cf.other, test.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = uc.Execute(context.Background(), cf.caller, uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound)
	// The chain above a test is checked live: a deleted control hides its tests.
	require.NoError(t, NewDeleteUseCase(cf.repo, clock).Execute(context.Background(), cf.caller, cf.control.ID))
	_, err = uc.Execute(context.Background(), cf.caller, test.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetControlTest_Unauthorized(t *testing.T) {
	cf := newChainFixture()
	test := cf.newTest(cf.newImpl(cf.assetA).ID)
	uc := NewGetTestUseCase(cf.chain)
	_, err := uc.Execute(context.Background(), Caller{UserID: uuid.New(), Permissions: cf.caller.Permissions}, test.ID)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
	_, err = uc.Execute(context.Background(), noPerms(cf.caller), test.ID)
	require.ErrorIs(t, err, domain.ErrForbidden)
}

func TestListControlTests_Success(t *testing.T) {
	cf := newChainFixture()
	impl := cf.newImpl(cf.assetA)
	cf.newTest(impl.ID)
	cf.newTest(impl.ID)
	vs, err := NewListTestsUseCase(cf.chain).Execute(context.Background(), cf.caller, impl.ID, 0, 0)
	require.NoError(t, err)
	assert.Len(t, vs, 2)
	_, err = NewListTestsUseCase(cf.chain).Execute(context.Background(), cf.caller, impl.ID, MaxListLimit+1, 0)
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestListControlTests_NotFound(t *testing.T) {
	cf := newChainFixture()
	impl := cf.newImpl(cf.assetA)
	cf.newTest(impl.ID)
	uc := NewListTestsUseCase(cf.chain)
	_, err := uc.Execute(context.Background(), cf.other, impl.ID, 0, 0)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = uc.Execute(context.Background(), cf.caller, uuid.New(), 0, 0)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestListControlTests_Unauthorized(t *testing.T) {
	cf := newChainFixture()
	impl := cf.newImpl(cf.assetA)
	uc := NewListTestsUseCase(cf.chain)
	_, err := uc.Execute(context.Background(), Caller{UserID: uuid.New(), Permissions: cf.caller.Permissions}, impl.ID, 0, 0)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
	_, err = uc.Execute(context.Background(), noPerms(cf.caller), impl.ID, 0, 0)
	require.ErrorIs(t, err, domain.ErrForbidden)
}

func TestUpdateControlTest_Success(t *testing.T) {
	cf := newChainFixture()
	test := cf.newTest(cf.newImpl(cf.assetA).ID)
	uc := NewUpdateTestUseCase(cf.chain, clock)

	out, err := uc.Execute(context.Background(), cf.caller, test.ID, UpdateTestInput{Name: str("Renamed"), Cadence: str("P30D")})
	require.NoError(t, err)
	assert.Equal(t, "Renamed", out.Name)
	assert.Equal(t, "P30D", *out.Cadence)
	assert.Equal(t, canonical.SourceOpenRiskUser, out.Source, "the envelope is untouched")

	// Switching to automated needs the collector in the same edit; a blank cadence clears it.
	out, err = uc.Execute(context.Background(), cf.caller, test.ID, UpdateTestInput{Method: str("automated"), Collector: str("nessus"), Cadence: str("")})
	require.NoError(t, err)
	assert.Equal(t, canonical.TestAutomated, out.Method)
	assert.Nil(t, out.Cadence)

	writes := cf.chain.writes
	_, err = uc.Execute(context.Background(), cf.caller, test.ID, UpdateTestInput{Name: str("Renamed")})
	require.NoError(t, err)
	assert.Equal(t, writes, cf.chain.writes, "an edit that changes nothing writes nothing")
}

func TestUpdateControlTest_NotFound(t *testing.T) {
	cf := newChainFixture()
	test := cf.newTest(cf.newImpl(cf.assetA).ID)
	uc := NewUpdateTestUseCase(cf.chain, clock)
	_, err := uc.Execute(context.Background(), cf.other, test.ID, UpdateTestInput{Name: str("x")})
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = uc.Execute(context.Background(), cf.caller, uuid.New(), UpdateTestInput{Name: str("x")})
	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Equal(t, "Quarterly review", cf.chain.tests[test.ID].Name)
}

func TestUpdateControlTest_Unauthorized(t *testing.T) {
	cf := newChainFixture()
	test := cf.newTest(cf.newImpl(cf.assetA).ID)
	uc := NewUpdateTestUseCase(cf.chain, clock)
	_, err := uc.Execute(context.Background(), Caller{UserID: uuid.New(), Permissions: cf.caller.Permissions}, test.ID, UpdateTestInput{Name: str("x")})
	require.ErrorIs(t, err, domain.ErrUnauthorized)
	readOnly := cf.caller
	readOnly.Permissions = []string{PermissionRead}
	_, err = uc.Execute(context.Background(), readOnly, test.ID, UpdateTestInput{Name: str("x")})
	require.ErrorIs(t, err, domain.ErrForbidden)
	assert.Equal(t, "Quarterly review", cf.chain.tests[test.ID].Name)
}

func TestUpdateControlTest_RejectsBadInput(t *testing.T) {
	cf := newChainFixture()
	test := cf.newTest(cf.newImpl(cf.assetA).ID)
	uc := NewUpdateTestUseCase(cf.chain, clock)
	for name, in := range map[string]UpdateTestInput{
		"blank checks":              {Checks: str(" ")},
		"blank pass criteria":       {PassCriteria: str("")},
		"clear the manual proc.":    {Procedure: str("")},
		"automated without a coll.": {Method: str("automated")},
		"cadence not ISO 8601":      {Cadence: str("often")},
		"unknown method":            {Method: str("magic")},
	} {
		_, err := uc.Execute(context.Background(), cf.caller, test.ID, in)
		require.ErrorIs(t, err, domain.ErrValidation, name)
	}
	assert.Equal(t, canonical.TestManual, cf.chain.tests[test.ID].Method)
}

func TestDeleteControlTest_Success(t *testing.T) {
	cf := newChainFixture()
	test := cf.newTest(cf.newImpl(cf.assetA).ID)
	require.NoError(t, NewDeleteTestUseCase(cf.chain, clock).Execute(context.Background(), cf.caller, test.ID))
	assert.True(t, cf.chain.testDeleted[test.ID])
	last := cf.chain.audits[len(cf.chain.audits)-1]
	assert.Equal(t, domain.AuditActionDelete, last.Action)
	assert.Equal(t, test.ID.String(), last.EntityID)
	_, err := NewGetTestUseCase(cf.chain).Execute(context.Background(), cf.caller, test.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestDeleteControlTest_NotFound(t *testing.T) {
	cf := newChainFixture()
	test := cf.newTest(cf.newImpl(cf.assetA).ID)
	uc := NewDeleteTestUseCase(cf.chain, clock)
	require.ErrorIs(t, uc.Execute(context.Background(), cf.other, test.ID), domain.ErrNotFound)
	require.ErrorIs(t, uc.Execute(context.Background(), cf.caller, uuid.New()), domain.ErrNotFound)
	assert.False(t, cf.chain.testDeleted[test.ID])
}

func TestDeleteControlTest_Unauthorized(t *testing.T) {
	cf := newChainFixture()
	test := cf.newTest(cf.newImpl(cf.assetA).ID)
	uc := NewDeleteTestUseCase(cf.chain, clock)
	require.ErrorIs(t, uc.Execute(context.Background(), Caller{UserID: uuid.New(), Permissions: cf.caller.Permissions}, test.ID), domain.ErrUnauthorized)
	updateOnly := cf.caller
	updateOnly.Permissions = []string{PermissionUpdate}
	require.ErrorIs(t, uc.Execute(context.Background(), updateOnly, test.ID), domain.ErrForbidden)
	assert.False(t, cf.chain.testDeleted[test.ID])
}
