// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlexception

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

var (
	now     = time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	clock   = func() time.Time { return now }
	tenantA = uuid.MustParse("00000000-0000-4000-8000-00000000000a")
	tenantB = uuid.MustParse("00000000-0000-4000-8000-00000000000b")
)

// fakeRepo is an in-memory Repository that honours the tenant the way the real
// one does: nothing is found outside the tenant it is asked with.
type fakeRepo struct {
	controls   map[uuid.UUID]uuid.UUID // control id -> tenant
	exceptions []domain.ControlException
	audits     []*domain.AuditEvent
}

func newFakeRepo() *fakeRepo { return &fakeRepo{controls: map[uuid.UUID]uuid.UUID{}} }

func (f *fakeRepo) ControlExists(_ context.Context, tenant, control uuid.UUID) (bool, error) {
	t, ok := f.controls[control]
	return ok && t == tenant, nil
}

func (f *fakeRepo) Create(_ context.Context, ex *domain.ControlException, a *domain.AuditEvent) error {
	f.exceptions = append(f.exceptions, *ex)
	f.audits = append(f.audits, a)
	return nil
}

func (f *fakeRepo) ListByControl(_ context.Context, tenant, control uuid.UUID) ([]domain.ControlException, error) {
	var out []domain.ControlException
	for _, e := range f.exceptions {
		if e.TenantID == tenant && e.ControlID == control {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (f *fakeRepo) Revoke(_ context.Context, tenant, id uuid.UUID,
	apply func(*domain.ControlException) (*domain.AuditEvent, error)) (*domain.ControlException, error) {
	for i := range f.exceptions {
		if f.exceptions[i].TenantID != tenant || f.exceptions[i].ID != id {
			continue
		}
		cur := f.exceptions[i]
		ev, err := apply(&cur)
		if err != nil {
			return nil, err
		}
		f.exceptions[i] = cur
		f.audits = append(f.audits, ev)
		return &cur, nil
	}
	return nil, domain.ControlExceptionError(domain.ErrNotFound, domain.CtlExcNotFound)
}

// members is an OwnerValidator over a (tenant, user) set.
type members map[[2]uuid.UUID]bool

func (m members) Validate(_ context.Context, tenant uuid.UUID, p domain.OwnershipPatch) error {
	for _, id := range p.CandidateIDs() {
		if !m[[2]uuid.UUID{tenant, id}] {
			return domain.NewValidationError("user " + id.String() + " is not a member of this organization")
		}
	}
	return nil
}

type fixture struct {
	repo    *fakeRepo
	owners  members
	control uuid.UUID
	owner   uuid.UUID
	caller  Caller
}

func newFixture() *fixture {
	f := &fixture{repo: newFakeRepo(), owners: members{}, control: uuid.New(), owner: uuid.New()}
	f.repo.controls[f.control] = tenantA
	f.owners[[2]uuid.UUID{tenantA, f.owner}] = true
	f.caller = Caller{
		TenantID: tenantA, UserID: uuid.New(), Permissions: []string{PermissionUpdate, PermissionRead},
		Origin: Origin{ActorType: domain.AuditActorUser, IPAddress: "203.0.113.7", UserAgent: "ua", RequestID: "req-1"},
	}
	return f
}

func (f *fixture) create() *CreateUseCase { return NewCreateUseCase(f.repo, f.owners, clock) }
func (f *fixture) list() *ListUseCase     { return NewListUseCase(f.repo, clock) }
func (f *fixture) revoke() *RevokeUseCase { return NewRevokeUseCase(f.repo, clock) }

func (f *fixture) input() CreateInput {
	exp := now.Add(30 * 24 * time.Hour)
	return CreateInput{ControlID: f.control, OwnerID: f.owner, Reason: "vendor fix pending", ExpiresAt: &exp}
}

func code(err error) string { return domain.MessageFromError(err) }

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestCreateException_Success(t *testing.T) {
	f := newFixture()
	v, err := f.create().Execute(context.Background(), f.caller, f.input())
	require.NoError(t, err)

	assert.Equal(t, domain.ControlExceptionOpen, v.Status)
	assert.Equal(t, f.owner, v.OwnerID)
	assert.Equal(t, f.caller.UserID, v.ApprovedBy, "approved_by is the caller, not the body")
	assert.Equal(t, now, v.StartsAt)

	require.Len(t, f.repo.exceptions, 1)
	assert.Equal(t, tenantA, f.repo.exceptions[0].TenantID, "the tenant is the session's")

	require.Len(t, f.repo.audits, 1, "stored and audited")
	a := f.repo.audits[0]
	assert.Equal(t, tenantA, a.TenantID)
	assert.Equal(t, domain.AuditActionCreate, a.Action)
	assert.Equal(t, "control_exception", a.EntityType)
	assert.Equal(t, v.ID.String(), a.EntityID)
	assert.Equal(t, domain.AuditSourceExplicit, a.Source)
	assert.Equal(t, f.caller.UserID, *a.ActorID)
	assert.Equal(t, domain.AuditActorUser, a.ActorType)
	assert.Equal(t, "203.0.113.7", a.IPAddress)
	assert.Equal(t, "req-1", a.RequestID)
	assert.Nil(t, a.Before)
	assert.Equal(t, "open", a.After["status"])

	// listed as open.
	list, err := f.list().Execute(context.Background(), f.caller, f.control)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, domain.ControlExceptionOpen, list[0].Status)
}

func TestCreateException_AcceptsResourceWildcardPermission(t *testing.T) {
	f := newFixture()
	f.caller.Permissions = []string{"compliance:*"}
	_, err := f.create().Execute(context.Background(), f.caller, f.input())
	require.NoError(t, err)
	f.caller.Permissions = []string{"*"}
	_, err = f.create().Execute(context.Background(), f.caller, f.input())
	require.NoError(t, err)
}

func TestCreateException_NotFound(t *testing.T) {
	f := newFixture()

	in := f.input()
	in.ControlID = uuid.New()
	_, err := f.create().Execute(context.Background(), f.caller, in)
	assert.True(t, errors.Is(err, domain.ErrNotFound), "unknown control")
	assert.Equal(t, domain.CtlExcControlNotFound, code(err))

	// A control of tenant B, named by a user of tenant A, reads as absent.
	foreign := uuid.New()
	f.repo.controls[foreign] = tenantB
	in.ControlID = foreign
	_, err = f.create().Execute(context.Background(), f.caller, in)
	assert.True(t, errors.Is(err, domain.ErrNotFound), "another tenant's control")
	assert.Empty(t, f.repo.exceptions)
	assert.Empty(t, f.repo.audits)
}

func TestCreateException_Unauthorized(t *testing.T) {
	f := newFixture()
	for name, c := range map[string]Caller{
		"nil tenant": {UserID: f.caller.UserID, Permissions: f.caller.Permissions},
		"nil user":   {TenantID: tenantA, Permissions: f.caller.Permissions},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := f.create().Execute(context.Background(), c, f.input())
			assert.True(t, errors.Is(err, domain.ErrUnauthorized))
		})
	}
	assert.Empty(t, f.repo.exceptions)
}

func TestCreateException_Forbidden(t *testing.T) {
	f := newFixture()
	for name, perms := range map[string][]string{
		"none":      nil,
		"read only": {PermissionRead},
		"other":     {"risks:update"},
	} {
		t.Run(name, func(t *testing.T) {
			f.caller.Permissions = perms
			_, err := f.create().Execute(context.Background(), f.caller, f.input())
			assert.True(t, errors.Is(err, domain.ErrForbidden))
		})
	}
	assert.Empty(t, f.repo.exceptions)
	assert.Empty(t, f.repo.audits)
}

func TestCreateException_Validation(t *testing.T) {
	foreignOwner := uuid.New() // a user of tenant B only
	past := now.Add(-time.Minute)
	cases := []struct {
		name string
		mut  func(f *fixture, in *CreateInput)
		code string
	}{
		{"no expires_at", func(_ *fixture, in *CreateInput) { in.ExpiresAt = nil }, domain.CtlExcExpiresRequired},
		{"expires_at in the past", func(_ *fixture, in *CreateInput) { in.ExpiresAt = &past }, domain.CtlExcExpiresInPast},
		{"expires_at is the caller's instant", func(_ *fixture, in *CreateInput) { n := now; in.ExpiresAt = &n }, domain.CtlExcExpiresInPast},
		{"expires_at past the horizon", func(_ *fixture, in *CreateInput) {
			e := now.Add(domain.ControlExceptionMaxHorizon + time.Second)
			in.ExpiresAt = &e
		}, domain.CtlExcExpiresTooFar},
		{"starts_at before the skew tolerance", func(_ *fixture, in *CreateInput) {
			s := now.Add(-domain.ControlExceptionStartSkew - time.Second)
			in.StartsAt = &s
		}, domain.CtlExcStartsInPast},
		{"starts_at equals expires_at", func(_ *fixture, in *CreateInput) { s := *in.ExpiresAt; in.StartsAt = &s }, domain.CtlExcStartsNotBeforeEnds},
		{"starts_at after expires_at", func(_ *fixture, in *CreateInput) { s := in.ExpiresAt.Add(time.Hour); in.StartsAt = &s }, domain.CtlExcStartsNotBeforeEnds},
		{"empty reason", func(_ *fixture, in *CreateInput) { in.Reason = "   " }, domain.CtlExcReasonRequired},
		{"reason too long", func(_ *fixture, in *CreateInput) { in.Reason = strings.Repeat("a", 2001) }, domain.CtlExcReasonTooLong},
		{"no owner", func(_ *fixture, in *CreateInput) { in.OwnerID = uuid.Nil }, domain.CtlExcOwnerRequired},
		{"owner of another tenant", func(f *fixture, in *CreateInput) {
			f.owners[[2]uuid.UUID{tenantB, foreignOwner}] = true
			in.OwnerID = foreignOwner
		}, domain.CtlExcOwnerInvalid},
		{"owner deactivated in this tenant", func(f *fixture, in *CreateInput) {
			deactivated := uuid.New()
			f.owners[[2]uuid.UUID{tenantA, deactivated}] = false
			in.OwnerID = deactivated
		}, domain.CtlExcOwnerInvalid},
		{"owner unknown", func(_ *fixture, in *CreateInput) { in.OwnerID = uuid.New() }, domain.CtlExcOwnerInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture()
			in := f.input()
			tc.mut(f, &in)
			_, err := f.create().Execute(context.Background(), f.caller, in)
			require.Error(t, err)
			assert.True(t, errors.Is(err, domain.ErrValidation))
			assert.Equal(t, tc.code, code(err))
			assert.NotContains(t, err.Error(), foreignOwner.String(), "the message names no user")
			assert.Empty(t, f.repo.exceptions, "never a silent store")
			assert.Empty(t, f.repo.audits)
		})
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestListExceptions_Success(t *testing.T) {
	f := newFixture()
	mk := func(created time.Time, exp time.Time) domain.ControlException {
		return domain.ControlException{
			ID: uuid.New(), TenantID: tenantA, ControlID: f.control, OwnerID: f.owner, Reason: "r",
			StartsAt: created, ExpiresAt: exp, CreatedAt: created,
		}
	}
	older := mk(now.Add(-48*time.Hour), now.Add(-time.Hour)) // expired by the clock, never revoked
	newer := mk(now.Add(-time.Hour), now.Add(time.Hour))
	revoked := mk(now.Add(-2*time.Hour), now.Add(time.Hour))
	at := now.Add(-time.Minute)
	by := uuid.New()
	revoked.RevokedAt, revoked.RevokedBy = &at, &by
	f.repo.exceptions = []domain.ControlException{older, revoked, newer}

	got, err := f.list().Execute(context.Background(), f.caller, f.control)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, newer.ID, got[0].ID, "newest first")
	assert.Equal(t, domain.ControlExceptionOpen, got[0].Status)
	assert.Equal(t, domain.ControlExceptionRevoked, got[1].Status)
	assert.Equal(t, older.ID, got[2].ID)
	assert.Equal(t, domain.ControlExceptionExpired, got[2].Status, "expired by the caller's instant, no job ran")

	// The same rows read open earlier: status is derived at the clock, not stored.
	early := NewListUseCase(f.repo, func() time.Time { return now.Add(-30 * time.Hour) })
	got, err = early.Execute(context.Background(), f.caller, f.control)
	require.NoError(t, err)
	byID := map[uuid.UUID]domain.ControlExceptionStatus{}
	for _, v := range got {
		byID[v.ID] = v.Status
	}
	assert.Equal(t, domain.ControlExceptionOpen, byID[older.ID])
}

func TestListExceptions_NotFound(t *testing.T) {
	f := newFixture()
	_, err := f.list().Execute(context.Background(), f.caller, uuid.New())
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	// Tenant B lists a control of tenant A: not found, and tenant A's exception
	// is never in the answer.
	_, err = f.create().Execute(context.Background(), f.caller, f.input())
	require.NoError(t, err)
	b := Caller{TenantID: tenantB, UserID: uuid.New(), Permissions: []string{PermissionRead}}
	got, err := f.list().Execute(context.Background(), b, f.control)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
	assert.Nil(t, got)
}

func TestListExceptions_Unauthorized(t *testing.T) {
	f := newFixture()
	_, err := f.list().Execute(context.Background(), Caller{UserID: f.caller.UserID, Permissions: f.caller.Permissions}, f.control)
	assert.True(t, errors.Is(err, domain.ErrUnauthorized))
	_, err = f.list().Execute(context.Background(), Caller{TenantID: tenantA, Permissions: f.caller.Permissions}, f.control)
	assert.True(t, errors.Is(err, domain.ErrUnauthorized))
	f.caller.Permissions = []string{PermissionUpdate} // update does not imply read
	_, err = f.list().Execute(context.Background(), f.caller, f.control)
	assert.True(t, errors.Is(err, domain.ErrForbidden))
}

// ---------------------------------------------------------------------------
// Revoke
// ---------------------------------------------------------------------------

func (f *fixture) created(t *testing.T) uuid.UUID {
	t.Helper()
	v, err := f.create().Execute(context.Background(), f.caller, f.input())
	require.NoError(t, err)
	return v.ID
}

func TestRevokeException_Success(t *testing.T) {
	f := newFixture()
	id := f.created(t)

	v, err := f.revoke().Execute(context.Background(), f.caller, id, "  fixed upstream ")
	require.NoError(t, err)
	assert.Equal(t, domain.ControlExceptionRevoked, v.Status)
	assert.Equal(t, "fixed upstream", v.RevokeReason)
	assert.Equal(t, f.caller.UserID, *v.RevokedBy)

	require.Len(t, f.repo.audits, 2)
	a := f.repo.audits[1]
	assert.Equal(t, domain.AuditActionUpdate, a.Action)
	assert.Equal(t, id.String(), a.EntityID)
	assert.Equal(t, "open", a.Before["status"])
	assert.Equal(t, "revoked", a.After["status"])
}

func TestRevokeException_NotFound(t *testing.T) {
	f := newFixture()
	id := f.created(t)

	_, err := f.revoke().Execute(context.Background(), f.caller, uuid.New(), "")
	assert.True(t, errors.Is(err, domain.ErrNotFound), "unknown id")

	b := Caller{TenantID: tenantB, UserID: uuid.New(), Permissions: []string{PermissionUpdate}}
	_, err = f.revoke().Execute(context.Background(), b, id, "")
	assert.True(t, errors.Is(err, domain.ErrNotFound), "tenant A's exception, asked by tenant B")
	assert.False(t, errors.Is(err, domain.ErrForbidden))
	assert.Nil(t, f.repo.exceptions[0].RevokedAt, "untouched")
	assert.Len(t, f.repo.audits, 1)
}

func TestRevokeException_Unauthorized(t *testing.T) {
	f := newFixture()
	id := f.created(t)
	_, err := f.revoke().Execute(context.Background(), Caller{UserID: f.caller.UserID, Permissions: f.caller.Permissions}, id, "")
	assert.True(t, errors.Is(err, domain.ErrUnauthorized))
	_, err = f.revoke().Execute(context.Background(), Caller{TenantID: tenantA, Permissions: f.caller.Permissions}, id, "")
	assert.True(t, errors.Is(err, domain.ErrUnauthorized))
	f.caller.Permissions = []string{PermissionRead}
	_, err = f.revoke().Execute(context.Background(), f.caller, id, "")
	assert.True(t, errors.Is(err, domain.ErrForbidden))
	assert.Nil(t, f.repo.exceptions[0].RevokedAt)
}

func TestRevokeException_TwiceIsConflict(t *testing.T) {
	f := newFixture()
	id := f.created(t)
	_, err := f.revoke().Execute(context.Background(), f.caller, id, "")
	require.NoError(t, err)
	_, err = f.revoke().Execute(context.Background(), f.caller, id, "")
	assert.True(t, errors.Is(err, domain.ErrConflict))
	assert.Equal(t, domain.CtlExcAlreadyRevoked, code(err))
	assert.Len(t, f.repo.audits, 2, "the refused revoke wrote no audit entry")
}

func TestRevokeException_ExpiredIsConflict(t *testing.T) {
	f := newFixture()
	id := f.created(t)
	late := NewRevokeUseCase(f.repo, func() time.Time { return now.Add(31 * 24 * time.Hour) })
	_, err := late.Execute(context.Background(), f.caller, id, "")
	assert.True(t, errors.Is(err, domain.ErrConflict))
	assert.Equal(t, domain.CtlExcAlreadyExpired, code(err))
	assert.Nil(t, f.repo.exceptions[0].RevokedAt)
}

func TestRevokeException_ReasonTooLong(t *testing.T) {
	f := newFixture()
	id := f.created(t)
	_, err := f.revoke().Execute(context.Background(), f.caller, id, strings.Repeat("x", 2001))
	assert.True(t, errors.Is(err, domain.ErrValidation))
	assert.Nil(t, f.repo.exceptions[0].RevokedAt)
}
