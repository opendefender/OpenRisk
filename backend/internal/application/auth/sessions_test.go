// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
)

// fakeSessionRepo records what the revocations were asked to keep.
type fakeSessionRepo struct {
	keepHash   string
	keepFamily uuid.UUID
	calls      []string
	revoked    int64
	records    []domain.SessionRecord
}

func (f *fakeSessionRepo) ListByUser(context.Context, uuid.UUID) ([]domain.SessionRecord, error) {
	return f.records, nil
}
func (f *fakeSessionRepo) Revoke(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (f *fakeSessionRepo) RevokeAllExcept(_ context.Context, _ uuid.UUID, keepHash string) (int64, error) {
	f.keepHash = keepHash
	f.calls = append(f.calls, "revoke")
	return f.revoked, nil
}
func (f *fakeSessionRepo) RevokeAllExceptFamily(_ context.Context, _ uuid.UUID, keepFamily uuid.UUID) (int64, error) {
	f.keepFamily = keepFamily
	f.calls = append(f.calls, "revoke-family")
	return f.revoked, nil
}
func (f *fakeSessionRepo) HasSeenDevice(context.Context, uuid.UUID, string) (bool, error) {
	return true, nil
}

type fakeSessionIssuer struct {
	repo   *fakeSessionRepo
	org    uuid.UUID
	device auth.DeviceContext
	err    error
}

func (f *fakeSessionIssuer) IssueSessionForOrg(_ context.Context, _, orgID uuid.UUID, device auth.DeviceContext) (*auth.TokenPair, error) {
	f.repo.calls = append(f.repo.calls, "issue")
	f.org, f.device = orgID, device
	if f.err != nil {
		return nil, f.err
	}
	return &auth.TokenPair{AccessToken: "fresh-access", RefreshToken: "fresh-refresh"}, nil
}

// A browser caller presents no refresh token: its device gets a new session
// BEFORE the sweep, and the sweep spares exactly that one.
func TestRevokeOtherSessions_Success(t *testing.T) {
	repo := &fakeSessionRepo{revoked: 2}
	issuer := &fakeSessionIssuer{repo: repo}
	org := uuid.New()
	device := auth.DeviceContext{UserAgent: "laptop", IP: "10.0.0.1"}

	out, err := NewRevokeOtherSessionsUseCase(repo).WithReissue(issuer).Execute(context.Background(),
		RevokeOtherSessionsInput{UserID: uuid.New(), TenantID: org, Device: device})
	if err != nil {
		t.Fatal(err)
	}
	if out.TokenPair == nil || out.Reauthenticate || out.Revoked != 2 {
		t.Fatalf("want a new session and 2 revoked, got %+v", out)
	}
	if repo.keepHash != auth.HashToken("fresh-refresh") {
		t.Fatalf("the sweep must keep the new session, kept %q", repo.keepHash)
	}
	if len(repo.calls) != 2 || repo.calls[0] != "issue" {
		t.Fatalf("the session must be issued before the sweep, got %v", repo.calls)
	}
	if issuer.org != org || issuer.device != device {
		t.Fatalf("the session must be issued for the caller's org and device, got %v %+v", issuer.org, issuer.device)
	}
}

// An API client that names its session keeps it; nothing is re-issued.
func TestRevokeOtherSessions_KeepsPresentedSession(t *testing.T) {
	repo := &fakeSessionRepo{revoked: 1}
	out, err := NewRevokeOtherSessionsUseCase(repo).WithReissue(&fakeSessionIssuer{repo: repo}).Execute(context.Background(),
		RevokeOtherSessionsInput{UserID: uuid.New(), TenantID: uuid.New(), CurrentHash: "presented"})
	if err != nil {
		t.Fatal(err)
	}
	if repo.keepHash != "presented" || out.TokenPair != nil || out.Reauthenticate {
		t.Fatalf("want the presented session kept and no re-issue, got keep=%q %+v", repo.keepHash, out)
	}
	if len(repo.calls) != 1 {
		t.Fatalf("want the sweep alone, got %v", repo.calls)
	}
}

// When the caller's session cannot be found or minted, everyone is signed out,
// the caller included, and told so: leaving an intruder in to spare the caller
// a sign-in is the wrong trade.
func TestRevokeOtherSessions_NotFound(t *testing.T) {
	cases := map[string]struct {
		issuer *fakeSessionIssuer
		tenant uuid.UUID
	}{
		"mint refused":    {issuer: &fakeSessionIssuer{err: errors.New("membership not found")}, tenant: uuid.New()},
		"no tenant":       {issuer: &fakeSessionIssuer{}, tenant: uuid.Nil},
		"no issuer wired": {issuer: nil, tenant: uuid.New()},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeSessionRepo{revoked: 3}
			uc := NewRevokeOtherSessionsUseCase(repo)
			if tc.issuer != nil {
				tc.issuer.repo = repo
				uc = uc.WithReissue(tc.issuer)
			}
			out, err := uc.Execute(context.Background(), RevokeOtherSessionsInput{UserID: uuid.New(), TenantID: tc.tenant})
			if err != nil {
				t.Fatal(err)
			}
			if repo.keepHash != "" || !out.Reauthenticate || out.TokenPair != nil || out.Revoked != 3 {
				t.Fatalf("want every session revoked and a re-authentication, got keep=%q %+v", repo.keepHash, out)
			}
		})
	}
}

func TestRevokeOtherSessions_Unauthorized(t *testing.T) {
	repo := &fakeSessionRepo{}
	_, err := NewRevokeOtherSessionsUseCase(repo).WithReissue(&fakeSessionIssuer{repo: repo}).Execute(context.Background(),
		RevokeOtherSessionsInput{TenantID: uuid.New()})
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != http.StatusBadRequest {
		t.Fatalf("want a validation error, got %v", err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("an anonymous call must touch nothing, got %v", repo.calls)
	}
}

// An access token that names its session keeps that session's whole family:
// nothing is re-issued, and the count is the other devices only.
func TestRevokeOtherSessions_KeepsSessionNamedByToken(t *testing.T) {
	repo := &fakeSessionRepo{revoked: 1}
	family := uuid.New()
	out, err := NewRevokeOtherSessionsUseCase(repo).WithReissue(&fakeSessionIssuer{repo: repo}).Execute(context.Background(),
		RevokeOtherSessionsInput{UserID: uuid.New(), TenantID: uuid.New(), CurrentSession: family, CurrentHash: "presented"})
	if err != nil {
		t.Fatal(err)
	}
	if repo.keepFamily != family || out.TokenPair != nil || out.Reauthenticate || out.Revoked != 1 {
		t.Fatalf("want the named family kept and no re-issue, got keep=%v %+v", repo.keepFamily, out)
	}
	if len(repo.calls) != 1 || repo.calls[0] != "revoke-family" {
		t.Fatalf("want the family sweep alone, got %v", repo.calls)
	}
}

func sessionRecords(families ...uuid.UUID) []domain.SessionRecord {
	out := make([]domain.SessionRecord, 0, len(families))
	for i, f := range families {
		out = append(out, domain.SessionRecord{ID: uuid.New(), FamilyID: f, TokenHash: "hash-" + string(rune('a'+i))})
	}
	return out
}

func currentRows(records []domain.SessionRecord) []int {
	var idx []int
	for i, r := range records {
		if r.Current {
			idx = append(idx, i)
		}
	}
	return idx
}

// The browser's case: no refresh token, only the sid of its access token.
func TestListSessions_Success(t *testing.T) {
	mine := uuid.New()
	repo := &fakeSessionRepo{records: sessionRecords(uuid.New(), mine, uuid.New())}

	records, err := NewListSessionsUseCase(repo).Execute(context.Background(), uuid.New(), mine, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := currentRows(records); len(got) != 1 || got[0] != 1 {
		t.Fatalf("want exactly row 1 marked current, got %v", got)
	}
	for _, r := range records {
		if r.TokenHash != "" {
			t.Fatal("the token hash must not leave the use case")
		}
	}
}

// An access token minted before sid existed still finds its row through a
// refresh token an API client presents.
func TestListSessions_FallsBackToPresentedToken(t *testing.T) {
	repo := &fakeSessionRepo{records: sessionRecords(uuid.New(), uuid.New())}

	records, err := NewListSessionsUseCase(repo).Execute(context.Background(), uuid.New(), uuid.Nil, "hash-b")
	if err != nil {
		t.Fatal(err)
	}
	if got := currentRows(records); len(got) != 1 || got[0] != 1 {
		t.Fatalf("want exactly row 1 marked current, got %v", got)
	}
}

// A session that is no longer listed (revoked elsewhere) marks nothing.
func TestListSessions_NotFound(t *testing.T) {
	repo := &fakeSessionRepo{records: sessionRecords(uuid.New(), uuid.New())}

	records, err := NewListSessionsUseCase(repo).Execute(context.Background(), uuid.New(), uuid.New(), "")
	if err != nil {
		t.Fatal(err)
	}
	if got := currentRows(records); len(got) != 0 {
		t.Fatalf("want no row marked current, got %v", got)
	}
}

func TestListSessions_Unauthorized(t *testing.T) {
	_, err := NewListSessionsUseCase(&fakeSessionRepo{}).Execute(context.Background(), uuid.Nil, uuid.New(), "")
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != http.StatusBadRequest {
		t.Fatalf("want a validation error, got %v", err)
	}
}
