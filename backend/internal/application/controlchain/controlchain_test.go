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
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

var (
	now     = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	clock   = func() time.Time { return now }
	tenantA = uuid.MustParse("00000000-0000-4000-8000-00000000000a")
	tenantB = uuid.MustParse("00000000-0000-4000-8000-00000000000b")
)

type edgeKey struct{ tenant, requirement, control uuid.UUID }

// fakeRepo is an in-memory Repository that honours the tenant the way the real
// one does: nothing is found outside the tenant it is asked with.
type fakeRepo struct {
	controls     map[uuid.UUID]canonical.Control
	deleted      map[uuid.UUID]bool
	requirements map[uuid.UUID]uuid.UUID // requirement id -> tenant
	edges        map[edgeKey]bool
	evidenced    map[uuid.UUID]bool // control id -> has live evidence
	audits       []*domain.AuditEvent
	writes       int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		controls: map[uuid.UUID]canonical.Control{}, deleted: map[uuid.UUID]bool{},
		requirements: map[uuid.UUID]uuid.UUID{}, edges: map[edgeKey]bool{}, evidenced: map[uuid.UUID]bool{},
	}
}

func notFound() error { return domain.InternalControlError(domain.ErrNotFound, domain.IntCtlNotFound) }

func (f *fakeRepo) live(tenant, id uuid.UUID) (canonical.Control, bool) {
	c, ok := f.controls[id]
	return c, ok && c.TenantID == tenant && !f.deleted[id]
}

func (f *fakeRepo) CreateControl(_ context.Context, c canonical.Control, a *domain.AuditEvent) error {
	f.controls[c.ID] = c
	f.audits = append(f.audits, a)
	f.writes++
	return nil
}

func (f *fakeRepo) GetControl(_ context.Context, tenant, id uuid.UUID) (*canonical.Control, error) {
	c, ok := f.live(tenant, id)
	if !ok {
		return nil, notFound()
	}
	return &c, nil
}

func (f *fakeRepo) ListControls(_ context.Context, tenant uuid.UUID, limit, offset int) ([]canonical.Control, error) {
	var out []canonical.Control
	for id, c := range f.controls {
		if _, ok := f.live(tenant, id); ok {
			out = append(out, c)
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

func (f *fakeRepo) UpdateControl(_ context.Context, tenant, id uuid.UUID,
	apply func(canonical.Control, func(time.Time) (bool, error)) (canonical.Control, *domain.AuditEvent, error),
) (*canonical.Control, error) {
	cur, ok := f.live(tenant, id)
	if !ok {
		return nil, notFound()
	}
	next, ev, err := apply(cur, func(time.Time) (bool, error) { return f.evidenced[id], nil })
	if err != nil {
		return nil, err
	}
	if ev != nil {
		f.controls[id] = next
		f.audits = append(f.audits, ev)
		f.writes++
	}
	return &next, nil
}

func (f *fakeRepo) DeleteControl(_ context.Context, tenant, id uuid.UUID, apply func(canonical.Control) (*domain.AuditEvent, error)) error {
	cur, ok := f.live(tenant, id)
	if !ok {
		return notFound()
	}
	ev, err := apply(cur)
	if err != nil {
		return err
	}
	f.deleted[id] = true
	f.audits = append(f.audits, ev)
	f.writes++
	return nil
}

func (f *fakeRepo) LinkRequirement(_ context.Context, tenant uuid.UUID, e canonical.Edge, a *domain.AuditEvent) (bool, error) {
	if f.requirements[e.From.ID] != tenant {
		return false, notFound()
	}
	if _, ok := f.live(tenant, e.To.ID); !ok {
		return false, notFound()
	}
	k := edgeKey{tenant, e.From.ID, e.To.ID}
	if f.edges[k] {
		return false, nil
	}
	f.edges[k] = true
	f.audits = append(f.audits, a)
	f.writes++
	return true, nil
}

func (f *fakeRepo) UnlinkRequirement(_ context.Context, tenant, control, requirement uuid.UUID, a *domain.AuditEvent) error {
	if _, ok := f.live(tenant, control); !ok {
		return notFound()
	}
	k := edgeKey{tenant, requirement, control}
	if !f.edges[k] {
		return domain.InternalControlError(domain.ErrNotFound, domain.IntCtlLinkNotFound)
	}
	delete(f.edges, k)
	f.audits = append(f.audits, a)
	f.writes++
	return nil
}

func (f *fakeRepo) ListRequirementsOfControl(_ context.Context, tenant, control uuid.UUID) ([]domain.InternalControlRequirement, error) {
	if _, ok := f.live(tenant, control); !ok {
		return nil, notFound()
	}
	var out []domain.InternalControlRequirement
	for k := range f.edges {
		if k.tenant == tenant && k.control == control {
			out = append(out, domain.InternalControlRequirement{RequirementID: k.requirement})
		}
	}
	return out, nil
}

func (f *fakeRepo) ListControlsOfRequirement(_ context.Context, tenant, requirement uuid.UUID) ([]canonical.Control, error) {
	if f.requirements[requirement] != tenant {
		return nil, notFound()
	}
	var out []canonical.Control
	for k := range f.edges {
		if c, ok := f.live(tenant, k.control); ok && k.tenant == tenant && k.requirement == requirement {
			out = append(out, c)
		}
	}
	return out, nil
}

// members is an OwnerValidator over a (tenant, user) set.
type members map[[2]uuid.UUID]bool

func (m members) Validate(_ context.Context, tenant uuid.UUID, p domain.OwnershipPatch) error {
	for _, id := range p.CandidateIDs() {
		if !m[[2]uuid.UUID{tenant, id}] {
			return domain.NewValidationError("not a member")
		}
	}
	return nil
}

type fixture struct {
	repo   *fakeRepo
	owners members
	owner  uuid.UUID
	caller Caller
	other  Caller // tenant B
}

func newFixture() *fixture {
	f := &fixture{repo: newFakeRepo(), owners: members{}, owner: uuid.New()}
	f.owners[[2]uuid.UUID{tenantA, f.owner}] = true
	perms := []string{PermissionRead, PermissionCreate, PermissionUpdate, PermissionDelete}
	origin := Origin{ActorType: domain.AuditActorUser, IPAddress: "203.0.113.7", UserAgent: "ua", RequestID: "req-1"}
	f.caller = Caller{TenantID: tenantA, UserID: uuid.New(), Permissions: perms, Origin: origin}
	f.other = Caller{TenantID: tenantB, UserID: uuid.New(), Permissions: perms, Origin: origin}
	return f
}

func (f *fixture) requirement(tenant uuid.UUID) uuid.UUID {
	id := uuid.New()
	f.repo.requirements[id] = tenant
	return id
}

func (f *fixture) create(in CreateInput) *ControlView {
	v, err := NewCreateUseCase(f.repo, f.owners, clock).Execute(context.Background(), f.caller, in)
	if err != nil {
		panic(err)
	}
	return v
}

func (f *fixture) update(c Caller, id uuid.UUID, in UpdateInput) (*ControlView, error) {
	return NewUpdateUseCase(f.repo, f.owners, clock).Execute(context.Background(), c, id, in)
}

// splitControl stores a control the way the legacy split makes it.
func (f *fixture) splitControl(tenant uuid.UUID, status string) canonical.Control {
	req := uuid.New()
	f.repo.requirements[req] = tenant
	c, _ := splitOf(req, tenant, status)
	f.repo.controls[c.ID] = c
	f.repo.edges[edgeKey{tenant, req, c.ID}] = true
	return c
}

func splitOf(req, tenant uuid.UUID, status string) (canonical.Control, uuid.UUID) {
	id := canonical.ControlIDFor(req)
	c := canonical.Control{
		Envelope: canonical.Envelope{
			ID: id, TenantID: tenant, Source: canonical.SourceOpenRiskUser, SourceID: id.String(),
			CreatedAt: now, UpdatedAt: now, Confidence: 0.7,
			Provenance: canonical.Provenance{
				Method: canonical.MethodManualEntry, Actor: canonical.Actor{Kind: canonical.ActorSystem, ID: SplitActorID},
				ConfidenceRule: canonical.ConfidenceRule,
			},
		},
		Name: "split", Applicability: canonical.Applicable, DeclaredStatus: canonical.DeclaredStatus(status), Tags: canonical.List[string]{},
	}
	return c, id
}

func ctlInput(name string) CreateInput { return CreateInput{Name: name} }

func code(err error) string { return domain.MessageFromError(err) }

func str(s string) *string { return &s }

// ---------------------------------------------------------------------------
// Create

func TestCreateInternalControl_Success(t *testing.T) {
	f := newFixture()
	in := ctlInput("Access control policy")
	in.Description = str("who may do what")
	in.OwnerID = &f.owner
	in.Tags = []string{" iso ", "access"}
	v, err := NewCreateUseCase(f.repo, f.owners, clock).Execute(context.Background(), f.caller, in)
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, v.ID)
	assert.Equal(t, "Access control policy", v.Name)
	assert.Equal(t, canonical.Applicable, v.Applicability)
	assert.Equal(t, canonical.DeclaredNotImplemented, v.DeclaredStatus)
	assert.Equal(t, []string{"iso", "access"}, v.Tags)
	// The server-set envelope: user_entered, default confidence, manual_entry by the caller.
	assert.Equal(t, canonical.SourceOpenRiskUser, v.Source)
	assert.Equal(t, v.ID.String(), v.SourceID)
	assert.Equal(t, 0.70, v.Confidence)
	assert.Equal(t, canonical.MethodManualEntry, v.Provenance.Method)
	assert.Equal(t, canonical.Actor{Kind: canonical.ActorUser, ID: f.caller.UserID.String()}, v.Provenance.Actor)
	assert.Equal(t, now, v.CreatedAt)

	stored := f.repo.controls[v.ID]
	assert.Equal(t, tenantA, stored.TenantID)
	require.Len(t, f.repo.audits, 1)
	assert.Equal(t, domain.AuditActionCreate, f.repo.audits[0].Action)
	assert.Equal(t, v.ID.String(), f.repo.audits[0].EntityID)
	assert.Equal(t, tenantA, f.repo.audits[0].TenantID)
}

func TestCreateInternalControl_NotFound(t *testing.T) {
	// A creation has no id to resolve; the nearest gate is the owner, who must be a
	// member of the caller's tenant. Another tenant's user is refused and nothing is stored.
	f := newFixture()
	foreign := uuid.New()
	f.owners[[2]uuid.UUID{tenantB, foreign}] = true
	in := ctlInput("c")
	in.OwnerID = &foreign
	_, err := NewCreateUseCase(f.repo, f.owners, clock).Execute(context.Background(), f.caller, in)
	require.ErrorIs(t, err, domain.ErrValidation)
	assert.Equal(t, domain.IntCtlOwnerInvalid, code(err))
	assert.Zero(t, f.repo.writes)
}

func TestCreateInternalControl_Unauthorized(t *testing.T) {
	f := newFixture()
	uc := NewCreateUseCase(f.repo, f.owners, clock)
	for name, c := range map[string]Caller{
		"no tenant": {UserID: uuid.New(), Permissions: f.caller.Permissions},
		"no user":   {TenantID: tenantA, Permissions: f.caller.Permissions},
	} {
		_, err := uc.Execute(context.Background(), c, ctlInput("c"))
		require.ErrorIs(t, err, domain.ErrUnauthorized, name)
	}
	denied := f.caller
	denied.Permissions = []string{PermissionRead}
	_, err := uc.Execute(context.Background(), denied, ctlInput("c"))
	require.ErrorIs(t, err, domain.ErrForbidden)
	assert.Zero(t, f.repo.writes)
}

func TestCreateInternalControl_NotApplicableNeedsAJustification(t *testing.T) {
	f := newFixture()
	in := ctlInput("c")
	in.Applicability = "not_applicable"
	_, err := NewCreateUseCase(f.repo, f.owners, clock).Execute(context.Background(), f.caller, in)
	require.ErrorIs(t, err, domain.ErrValidation)
	assert.Contains(t, code(err), "applicability_justification")

	in.ApplicabilityJustification = str("   ")
	_, err = NewCreateUseCase(f.repo, f.owners, clock).Execute(context.Background(), f.caller, in)
	require.ErrorIs(t, err, domain.ErrValidation, "a blank justification is none")

	in.ApplicabilityJustification = str("no payment card data is processed")
	v, err := NewCreateUseCase(f.repo, f.owners, clock).Execute(context.Background(), f.caller, in)
	require.NoError(t, err)
	assert.False(t, v.ApplicabilityReviewPending, "the flag is the split's, never a request's")
}

func TestCreateInternalControl_ImplementedNeedsEvidence(t *testing.T) {
	// A control created now has no evidence and no path to attach any before #1416,
	// so it cannot be created as implemented.
	f := newFixture()
	in := ctlInput("c")
	in.DeclaredStatus = "implemented"
	_, err := NewCreateUseCase(f.repo, f.owners, clock).Execute(context.Background(), f.caller, in)
	require.ErrorIs(t, err, domain.ErrValidation)
	assert.Equal(t, "evidence_required", code(err))
	assert.Zero(t, f.repo.writes)
}

func TestCreateInternalControl_RejectsBadInput(t *testing.T) {
	f := newFixture()
	uc := NewCreateUseCase(f.repo, f.owners, clock)
	for name, in := range map[string]CreateInput{
		"blank name":           ctlInput("  "),
		"unknown applicab.":    {Name: "c", Applicability: "maybe"},
		"unknown status":       {Name: "c", DeclaredStatus: "done"},
		"blank tag":            {Name: "c", Tags: []string{" "}},
		"too many tags":        {Name: "c", Tags: make([]string, maxTags+1)},
		"owner nil uuid given": {Name: "c", OwnerID: new(uuid.UUID)},
	} {
		_, err := uc.Execute(context.Background(), f.caller, in)
		require.ErrorIs(t, err, domain.ErrValidation, name)
	}
	assert.Zero(t, f.repo.writes)
}

// ---------------------------------------------------------------------------
// Get / List

func TestGetInternalControl_Success(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	got, err := NewGetUseCase(f.repo).Execute(context.Background(), f.caller, v.ID)
	require.NoError(t, err)
	assert.Equal(t, v.ID, got.ID)
}

func TestGetInternalControl_NotFound(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	uc := NewGetUseCase(f.repo)
	_, err := uc.Execute(context.Background(), f.other, v.ID)
	require.ErrorIs(t, err, domain.ErrNotFound, "another tenant's id reads as absent")
	_, err = uc.Execute(context.Background(), f.caller, uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetInternalControl_Unauthorized(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	_, err := NewGetUseCase(f.repo).Execute(context.Background(), Caller{UserID: uuid.New(), Permissions: f.caller.Permissions}, v.ID)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
	_, err = NewGetUseCase(f.repo).Execute(context.Background(), Caller{TenantID: tenantA, UserID: uuid.New()}, v.ID)
	require.ErrorIs(t, err, domain.ErrForbidden)
}

func TestListInternalControls_Success(t *testing.T) {
	f := newFixture()
	f.create(ctlInput("b"))
	f.create(ctlInput("a"))
	split := f.splitControl(tenantA, "implemented")
	vs, err := NewListUseCase(f.repo).Execute(context.Background(), f.caller, 0, 0)
	require.NoError(t, err)
	require.Len(t, vs, 3, "split controls are controls: the library lists them")
	var sawSplit bool
	for _, v := range vs {
		if v.ID == split.ID {
			sawSplit = true
			assert.Equal(t, canonical.ActorSystem, v.Provenance.Actor.Kind, "the envelope tells them apart")
		}
	}
	assert.True(t, sawSplit)
}

func TestListInternalControls_NotFound(t *testing.T) {
	// A list has no id; another tenant's controls read as nothing.
	f := newFixture()
	f.create(ctlInput("a"))
	vs, err := NewListUseCase(f.repo).Execute(context.Background(), f.other, 0, 0)
	require.NoError(t, err)
	assert.Empty(t, vs)
	assert.NotNil(t, vs, "an empty page is [] not null")

	_, err = NewListUseCase(f.repo).Execute(context.Background(), f.caller, MaxListLimit+1, 0)
	require.ErrorIs(t, err, domain.ErrValidation)
	_, err = NewListUseCase(f.repo).Execute(context.Background(), f.caller, 0, -1)
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestListInternalControls_Unauthorized(t *testing.T) {
	f := newFixture()
	_, err := NewListUseCase(f.repo).Execute(context.Background(), Caller{Permissions: f.caller.Permissions}, 0, 0)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

// ---------------------------------------------------------------------------
// Update

func TestUpdateInternalControl_Success(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("old"))
	f.repo.audits = nil
	later := now.Add(time.Hour)
	uc := NewUpdateUseCase(f.repo, f.owners, func() time.Time { return later })
	got, err := uc.Execute(context.Background(), f.caller, v.ID, UpdateInput{
		Name: str("new"), Description: str("d"), Ownership: domain.OwnershipPatch{Owner: domain.Assign(f.owner)},
		Tags: &[]string{"x"}, DeclaredStatus: str("in_progress"),
	})
	require.NoError(t, err)
	assert.Equal(t, "new", got.Name)
	assert.Equal(t, canonical.DeclaredInProgress, got.DeclaredStatus)
	assert.Equal(t, later, got.UpdatedAt)
	assert.Equal(t, now, got.CreatedAt)
	// The envelope is untouched by an edit.
	assert.Equal(t, v.Provenance, got.Provenance)
	assert.Equal(t, v.Confidence, got.Confidence)
	assert.Equal(t, v.Source, got.Source)
	require.Len(t, f.repo.audits, 1)
	assert.Equal(t, domain.AuditActionUpdate, f.repo.audits[0].Action)
	assert.Contains(t, []string(f.repo.audits[0].ChangedFields), "name")
}

func TestUpdateInternalControl_NotFound(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	_, err := f.update(f.other, v.ID, UpdateInput{Name: str("hijack")})
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = f.update(f.caller, uuid.New(), UpdateInput{Name: str("x")})
	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Equal(t, "c", f.repo.controls[v.ID].Name)
}

func TestUpdateInternalControl_Unauthorized(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	_, err := f.update(Caller{UserID: uuid.New(), Permissions: f.caller.Permissions}, v.ID, UpdateInput{Name: str("x")})
	require.ErrorIs(t, err, domain.ErrUnauthorized)
	_, err = f.update(Caller{TenantID: tenantA, UserID: uuid.New(), Permissions: []string{PermissionRead}}, v.ID, UpdateInput{Name: str("x")})
	require.ErrorIs(t, err, domain.ErrForbidden)
	assert.Equal(t, "c", f.repo.controls[v.ID].Name)
}

func TestUpdateInternalControl_UnknownOwnerIsRefused(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	_, err := f.update(f.caller, v.ID, UpdateInput{Ownership: domain.OwnershipPatch{Owner: domain.Assign(uuid.New())}})
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestUpdateInternalControl_EmptyEditWritesNothing(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	writes := f.repo.writes
	got, err := f.update(f.caller, v.ID, UpdateInput{})
	require.NoError(t, err)
	assert.Equal(t, v.ID, got.ID)
	assert.Equal(t, writes, f.repo.writes)
	_, err = f.update(f.caller, v.ID, UpdateInput{Name: str("c")})
	require.NoError(t, err)
	assert.Equal(t, writes, f.repo.writes, "writing the stored value is not a change")
}

// AC5: not_applicable without a justification is refused.
func TestUpdateInternalControl_NotApplicableNeedsAJustification(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	_, err := f.update(f.caller, v.ID, UpdateInput{Applicability: str("not_applicable")})
	require.ErrorIs(t, err, domain.ErrValidation)
	assert.Equal(t, canonical.Applicable, f.repo.controls[v.ID].Applicability)

	got, err := f.update(f.caller, v.ID, UpdateInput{Applicability: str("not_applicable"), ApplicabilityJustification: str("out of scope")})
	require.NoError(t, err)
	assert.Equal(t, canonical.NotApplicable, got.Applicability)

	// Blanking the justification of a settled not_applicable control is refused too.
	_, err = f.update(f.caller, v.ID, UpdateInput{ApplicabilityJustification: str("")})
	require.ErrorIs(t, err, domain.ErrValidation)
}

// AC5 / A10.1: a pending control accepts an empty justification and clears the
// flag on the first write that supplies one.
func TestUpdateInternalControl_PendingFlagClearsOnTheFirstJustification(t *testing.T) {
	f := newFixture()
	c, _ := splitOf(uuid.New(), tenantA, "not_implemented")
	c.Applicability = canonical.NotApplicable
	c = canonical.WithApplicabilityReviewPending(c, true)
	f.repo.controls[c.ID] = c

	// A write that supplies no justification keeps the flag and is accepted.
	got, err := f.update(f.caller, c.ID, UpdateInput{Name: str("renamed")})
	require.NoError(t, err)
	assert.True(t, got.ApplicabilityReviewPending)
	assert.Nil(t, got.ApplicabilityJustification)
	got, err = f.update(f.caller, c.ID, UpdateInput{ApplicabilityJustification: str("  ")})
	require.NoError(t, err)
	assert.True(t, got.ApplicabilityReviewPending, "a blank justification is none")

	// The first justification clears it, in the same write.
	got, err = f.update(f.caller, c.ID, UpdateInput{ApplicabilityJustification: str("no card data")})
	require.NoError(t, err)
	assert.False(t, got.ApplicabilityReviewPending)
	assert.Equal(t, canonical.NotApplicable, got.Applicability)
	assert.False(t, f.repo.controls[c.ID].ApplicabilityReviewPending())

	// Once cleared, the rule applies again.
	_, err = f.update(f.caller, c.ID, UpdateInput{ApplicabilityJustification: str("")})
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestUpdateInternalControl_PendingFlagClearsWhenTheControlBecomesApplicable(t *testing.T) {
	f := newFixture()
	c, _ := splitOf(uuid.New(), tenantA, "not_implemented")
	c.Applicability = canonical.NotApplicable
	c = canonical.WithApplicabilityReviewPending(c, true)
	f.repo.controls[c.ID] = c
	got, err := f.update(f.caller, c.ID, UpdateInput{Applicability: str("applicable")})
	require.NoError(t, err)
	assert.False(t, got.ApplicabilityReviewPending)
	assert.Equal(t, canonical.Applicable, got.Applicability)
}

// D-108 Q16: moving INTO implemented needs live, accepted, unexpired evidence.
func TestUpdateInternalControl_ImplementedNeedsEvidence(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	_, err := f.update(f.caller, v.ID, UpdateInput{DeclaredStatus: str("implemented")})
	require.ErrorIs(t, err, domain.ErrValidation)
	assert.Equal(t, "evidence_required", code(err))
	assert.Equal(t, canonical.DeclaredNotImplemented, f.repo.controls[v.ID].DeclaredStatus)

	f.repo.evidenced[v.ID] = true
	got, err := f.update(f.caller, v.ID, UpdateInput{DeclaredStatus: str("implemented")})
	require.NoError(t, err)
	assert.Equal(t, canonical.DeclaredImplemented, got.DeclaredStatus)
}

func TestUpdateInternalControl_ResavingAnImplementedControlIsNeverBlocked(t *testing.T) {
	f := newFixture()
	split := f.splitControl(tenantA, "implemented") // no evidence recorded
	got, err := f.update(f.caller, split.ID, UpdateInput{Name: str("renamed"), DeclaredStatus: str("implemented")})
	require.NoError(t, err)
	assert.Equal(t, "renamed", got.Name)
	// Moving out and back in is a move INTO implemented again.
	_, err = f.update(f.caller, split.ID, UpdateInput{DeclaredStatus: str("in_progress")})
	require.NoError(t, err)
	_, err = f.update(f.caller, split.ID, UpdateInput{DeclaredStatus: str("implemented")})
	require.ErrorIs(t, err, domain.ErrValidation)
}

// ---------------------------------------------------------------------------
// Delete

func TestDeleteInternalControl_Success(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	req := f.requirement(tenantA)
	_, err := NewLinkUseCase(f.repo, clock).Execute(context.Background(), f.caller, v.ID, req)
	require.NoError(t, err)

	require.NoError(t, NewDeleteUseCase(f.repo, clock).Execute(context.Background(), f.caller, v.ID))
	_, err = NewGetUseCase(f.repo).Execute(context.Background(), f.caller, v.ID)
	require.ErrorIs(t, err, domain.ErrNotFound, "a deleted control reads as absent")
	assert.True(t, f.repo.edges[edgeKey{tenantA, req, v.ID}], "the edge is kept; readers filter on liveness")
	last := f.repo.audits[len(f.repo.audits)-1]
	assert.Equal(t, domain.AuditActionDelete, last.Action)
}

func TestDeleteInternalControl_NotFound(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	uc := NewDeleteUseCase(f.repo, clock)
	require.ErrorIs(t, uc.Execute(context.Background(), f.other, v.ID), domain.ErrNotFound)
	require.ErrorIs(t, uc.Execute(context.Background(), f.caller, uuid.New()), domain.ErrNotFound)
	assert.False(t, f.repo.deleted[v.ID])
}

func TestDeleteInternalControl_Unauthorized(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	uc := NewDeleteUseCase(f.repo, clock)
	require.ErrorIs(t, uc.Execute(context.Background(), Caller{UserID: uuid.New(), Permissions: f.caller.Permissions}, v.ID), domain.ErrUnauthorized)
	require.ErrorIs(t, uc.Execute(context.Background(), Caller{TenantID: tenantA, UserID: uuid.New(), Permissions: []string{PermissionUpdate}}, v.ID), domain.ErrForbidden)
	assert.False(t, f.repo.deleted[v.ID])
}

func TestDeleteInternalControl_ASplitControlIsRefused(t *testing.T) {
	f := newFixture()
	split := f.splitControl(tenantA, "implemented")
	err := NewDeleteUseCase(f.repo, clock).Execute(context.Background(), f.caller, split.ID)
	require.ErrorIs(t, err, domain.ErrConflict)
	assert.Equal(t, domain.IntCtlSplitControl, code(err))
	assert.False(t, f.repo.deleted[split.ID])
}

// ---------------------------------------------------------------------------
// Link / unlink / reads

func TestLinkInternalControl_Success(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	r1, r2 := f.requirement(tenantA), f.requirement(tenantA)
	uc := NewLinkUseCase(f.repo, clock)
	f.repo.audits = nil

	created, err := uc.Execute(context.Background(), f.caller, v.ID, r1)
	require.NoError(t, err)
	assert.True(t, created)
	created, err = uc.Execute(context.Background(), f.caller, v.ID, r2)
	require.NoError(t, err)
	assert.True(t, created)
	require.Len(t, f.repo.audits, 2)

	// Linking twice is the same edge and writes no second audit entry.
	created, err = uc.Execute(context.Background(), f.caller, v.ID, r1)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Len(t, f.repo.audits, 2)

	rs, err := NewListRequirementsUseCase(f.repo).Execute(context.Background(), f.caller, v.ID)
	require.NoError(t, err)
	assert.Len(t, rs, 2)
}

func TestLinkInternalControl_NotFound(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	mine := f.requirement(tenantA)
	theirs := f.requirement(tenantB)
	uc := NewLinkUseCase(f.repo, clock)

	_, err := uc.Execute(context.Background(), f.caller, v.ID, theirs)
	require.ErrorIs(t, err, domain.ErrNotFound, "the requirement is another tenant's")
	_, err = uc.Execute(context.Background(), f.other, v.ID, theirs)
	require.ErrorIs(t, err, domain.ErrNotFound, "the control is another tenant's")
	_, err = uc.Execute(context.Background(), f.caller, uuid.New(), mine)
	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Empty(t, f.repo.edges)
}

func TestLinkInternalControl_Unauthorized(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	req := f.requirement(tenantA)
	uc := NewLinkUseCase(f.repo, clock)
	_, err := uc.Execute(context.Background(), Caller{UserID: uuid.New(), Permissions: f.caller.Permissions}, v.ID, req)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
	_, err = uc.Execute(context.Background(), Caller{TenantID: tenantA, UserID: uuid.New(), Permissions: []string{PermissionRead}}, v.ID, req)
	require.ErrorIs(t, err, domain.ErrForbidden)
	assert.Empty(t, f.repo.edges)
}

func TestUnlinkInternalControl_Success(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	req := f.requirement(tenantA)
	_, err := NewLinkUseCase(f.repo, clock).Execute(context.Background(), f.caller, v.ID, req)
	require.NoError(t, err)
	require.NoError(t, NewUnlinkUseCase(f.repo, clock).Execute(context.Background(), f.caller, v.ID, req))
	assert.Empty(t, f.repo.edges)
	require.ErrorIs(t, NewUnlinkUseCase(f.repo, clock).Execute(context.Background(), f.caller, v.ID, req), domain.ErrNotFound, "nothing left to unlink")
}

func TestUnlinkInternalControl_NotFound(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	req := f.requirement(tenantA)
	_, err := NewLinkUseCase(f.repo, clock).Execute(context.Background(), f.caller, v.ID, req)
	require.NoError(t, err)
	uc := NewUnlinkUseCase(f.repo, clock)
	require.ErrorIs(t, uc.Execute(context.Background(), f.other, v.ID, req), domain.ErrNotFound)
	require.ErrorIs(t, uc.Execute(context.Background(), f.caller, uuid.New(), req), domain.ErrNotFound)
	assert.Len(t, f.repo.edges, 1)
}

func TestUnlinkInternalControl_Unauthorized(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	req := f.requirement(tenantA)
	_, err := NewLinkUseCase(f.repo, clock).Execute(context.Background(), f.caller, v.ID, req)
	require.NoError(t, err)
	uc := NewUnlinkUseCase(f.repo, clock)
	require.ErrorIs(t, uc.Execute(context.Background(), Caller{UserID: uuid.New(), Permissions: f.caller.Permissions}, v.ID, req), domain.ErrUnauthorized)
	require.ErrorIs(t, uc.Execute(context.Background(), Caller{TenantID: tenantA, UserID: uuid.New()}, v.ID, req), domain.ErrForbidden)
	assert.Len(t, f.repo.edges, 1)
}

func TestUnlinkInternalControl_TheSplitControlsOwnLinkIsRefused(t *testing.T) {
	// The reconciler re-inserts the (requirement, split control) edge at boot, so
	// an unlink would silently revert: ErrConflict. A split control linked to a
	// DIFFERENT requirement unlinks freely.
	f := newFixture()
	split := f.splitControl(tenantA, "implemented")
	var own uuid.UUID
	for k := range f.repo.edges {
		own = k.requirement
	}
	uc := NewUnlinkUseCase(f.repo, clock)
	err := uc.Execute(context.Background(), f.caller, split.ID, own)
	require.ErrorIs(t, err, domain.ErrConflict)
	assert.Equal(t, domain.IntCtlSplitLink, code(err))
	assert.Len(t, f.repo.edges, 1)

	other := f.requirement(tenantA)
	_, err = NewLinkUseCase(f.repo, clock).Execute(context.Background(), f.caller, split.ID, other)
	require.NoError(t, err)
	require.NoError(t, uc.Execute(context.Background(), f.caller, split.ID, other))
	assert.Len(t, f.repo.edges, 1)
}

func TestListControlRequirements_NotFound(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("c"))
	uc := NewListRequirementsUseCase(f.repo)
	_, err := uc.Execute(context.Background(), f.other, v.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = uc.Execute(context.Background(), Caller{UserID: uuid.New()}, v.ID)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
	rs, err := uc.Execute(context.Background(), f.caller, v.ID)
	require.NoError(t, err)
	assert.NotNil(t, rs)
	assert.Empty(t, rs)
}

// AC1 / AC2: one control linked to two requirements; read from both; edit once.
func TestListRequirementControls_OneControlTwoRequirementsEditedOnce(t *testing.T) {
	f := newFixture()
	v := f.create(ctlInput("Access review"))
	iso, soc2 := f.requirement(tenantA), f.requirement(tenantA)
	link := NewLinkUseCase(f.repo, clock)
	_, err := link.Execute(context.Background(), f.caller, v.ID, iso)
	require.NoError(t, err)
	_, err = link.Execute(context.Background(), f.caller, v.ID, soc2)
	require.NoError(t, err)

	read := NewListForRequirementUseCase(f.repo)
	for name, req := range map[string]uuid.UUID{"ISO A.5.15": iso, "SOC 2 CC6.1": soc2} {
		cs, err := read.Execute(context.Background(), f.caller, req)
		require.NoError(t, err, name)
		require.Len(t, cs, 1, name)
		assert.Equal(t, v.ID, cs[0].ID, name)
		assert.Equal(t, "Access review", cs[0].Name, name)
	}

	_, err = f.update(f.caller, v.ID, UpdateInput{Name: str("Quarterly access review")})
	require.NoError(t, err)
	for name, req := range map[string]uuid.UUID{"ISO A.5.15": iso, "SOC 2 CC6.1": soc2} {
		cs, err := read.Execute(context.Background(), f.caller, req)
		require.NoError(t, err, name)
		require.Len(t, cs, 1, name)
		assert.Equal(t, "Quarterly access review", cs[0].Name, name, "one edit, both requirements show it")
	}
}

func TestListRequirementControls_ShowsTheSplitControlNextToUserControls(t *testing.T) {
	f := newFixture()
	split := f.splitControl(tenantA, "implemented")
	var req uuid.UUID
	for k := range f.repo.edges {
		req = k.requirement
	}
	v := f.create(ctlInput("mine"))
	_, err := NewLinkUseCase(f.repo, clock).Execute(context.Background(), f.caller, v.ID, req)
	require.NoError(t, err)
	cs, err := NewListForRequirementUseCase(f.repo).Execute(context.Background(), f.caller, req)
	require.NoError(t, err)
	require.Len(t, cs, 2)
	ids := []uuid.UUID{cs[0].ID, cs[1].ID}
	assert.ElementsMatch(t, []uuid.UUID{split.ID, v.ID}, ids)
}

func TestListRequirementControls_NotFound(t *testing.T) {
	f := newFixture()
	req := f.requirement(tenantA)
	uc := NewListForRequirementUseCase(f.repo)
	_, err := uc.Execute(context.Background(), f.other, req)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = uc.Execute(context.Background(), f.caller, uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestListRequirementControls_Unauthorized(t *testing.T) {
	f := newFixture()
	req := f.requirement(tenantA)
	uc := NewListForRequirementUseCase(f.repo)
	_, err := uc.Execute(context.Background(), Caller{UserID: uuid.New(), Permissions: f.caller.Permissions}, req)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
	_, err = uc.Execute(context.Background(), Caller{TenantID: tenantA, UserID: uuid.New()}, req)
	require.ErrorIs(t, err, domain.ErrForbidden)
}

func TestCreateAndUpdateInternalControl_LongTextIsCapped(t *testing.T) {
	f := newFixture()
	atLimit := strings.Repeat("é", maxLongText)
	over := atLimit + "x"
	in := ctlInput("c")
	in.Description, in.Objective = &atLimit, &atLimit
	v, err := NewCreateUseCase(f.repo, f.owners, clock).Execute(context.Background(), f.caller, in)
	require.NoError(t, err, "at the limit is accepted")

	for _, field := range []string{"description", "objective", "applicability_justification"} {
		in := ctlInput("c2")
		up := UpdateInput{}
		switch field {
		case "description":
			in.Description, up.Description = &over, &over
		case "objective":
			in.Objective, up.Objective = &over, &over
		default:
			in.ApplicabilityJustification, up.ApplicabilityJustification = &over, &over
		}
		_, err := NewCreateUseCase(f.repo, f.owners, clock).Execute(context.Background(), f.caller, in)
		require.ErrorIs(t, err, domain.ErrValidation, field)
		assert.Equal(t, field+" is too long", code(err))
		_, err = f.update(f.caller, v.ID, up)
		require.ErrorIs(t, err, domain.ErrValidation, field)
		assert.Equal(t, field+" is too long", code(err))
	}
	up := UpdateInput{Objective: &atLimit}
	_, err = f.update(f.caller, v.ID, up)
	require.NoError(t, err, "at the limit is accepted on update")
}

func TestUnlinkInternalControl_SplitPairOfAnotherTenantIsNotFound(t *testing.T) {
	f := newFixture()
	theirs := f.splitControl(tenantB, "implemented")
	var theirReq uuid.UUID
	for k := range f.repo.edges {
		if k.tenant == tenantB {
			theirReq = k.requirement
		}
	}
	uc := NewUnlinkUseCase(f.repo, clock)
	err := uc.Execute(context.Background(), f.caller, theirs.ID, theirReq)
	require.ErrorIs(t, err, domain.ErrNotFound, "foreign split pair is 404, not 409")
	// A fabricated requirement id: the derived control id is absent too.
	err = uc.Execute(context.Background(), f.caller, canonical.ControlIDFor(uuid.New()), uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound)

	own := f.splitControl(tenantA, "implemented")
	var ownReq uuid.UUID
	for k := range f.repo.edges {
		if k.tenant == tenantA {
			ownReq = k.requirement
		}
	}
	require.ErrorIs(t, uc.Execute(context.Background(), f.caller, own.ID, ownReq), domain.ErrConflict)
}
