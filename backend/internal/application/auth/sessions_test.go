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

	"github.com/opendefender/openrisk/internal/domain"
)

// fakeSessionRepo records what the revocations were asked to keep.
type fakeSessionRepo struct {
	keepHash   string
	keepFamily uuid.UUID
	calls      []string
	revoked    int64
	records    []domain.SessionRecord
	// gone lists families that no longer hold a live session.
	gone map[uuid.UUID]bool
}

func (f *fakeSessionRepo) HasLiveSession(_ context.Context, _ uuid.UUID, family uuid.UUID) (bool, error) {
	return !f.gone[family], nil
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

// The browser's case: the access token's sid names the caller's session, and
// its whole family is kept. The count is the other devices only.
func TestRevokeOtherSessions_Success(t *testing.T) {
	repo := &fakeSessionRepo{revoked: 1}
	family := uuid.New()
	out, err := NewRevokeOtherSessionsUseCase(repo).Execute(context.Background(),
		RevokeOtherSessionsInput{UserID: uuid.New(), CurrentSession: family, CurrentHash: "presented"})
	if err != nil {
		t.Fatal(err)
	}
	if repo.keepFamily != family || out.Reauthenticate || out.Revoked != 1 {
		t.Fatalf("want the named family kept, got keep=%v %+v", repo.keepFamily, out)
	}
	if len(repo.calls) != 1 || repo.calls[0] != "revoke-family" {
		t.Fatalf("want the family sweep alone, got %v", repo.calls)
	}
}

// An API client on a token without sid that names its session keeps it.
func TestRevokeOtherSessions_KeepsPresentedSession(t *testing.T) {
	repo := &fakeSessionRepo{revoked: 1}
	out, err := NewRevokeOtherSessionsUseCase(repo).Execute(context.Background(),
		RevokeOtherSessionsInput{UserID: uuid.New(), CurrentHash: "presented"})
	if err != nil {
		t.Fatal(err)
	}
	if repo.keepHash != "presented" || out.Reauthenticate {
		t.Fatalf("want the presented session kept, got keep=%q %+v", repo.keepHash, out)
	}
}

// When nothing names the caller's session, everyone is signed out, the caller
// included, and told so. No session is minted in its place (#1060).
func TestRevokeOtherSessions_NotFound(t *testing.T) {
	repo := &fakeSessionRepo{revoked: 3}
	out, err := NewRevokeOtherSessionsUseCase(repo).Execute(context.Background(),
		RevokeOtherSessionsInput{UserID: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if repo.keepHash != "" || !out.Reauthenticate || out.Revoked != 3 {
		t.Fatalf("want every session revoked and a re-authentication, got keep=%q %+v", repo.keepHash, out)
	}
}

func TestRevokeOtherSessions_Unauthorized(t *testing.T) {
	repo := &fakeSessionRepo{}
	_, err := NewRevokeOtherSessionsUseCase(repo).Execute(context.Background(),
		RevokeOtherSessionsInput{CurrentSession: uuid.New()})
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != http.StatusBadRequest {
		t.Fatalf("want a validation error, got %v", err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("an anonymous call must touch nothing, got %v", repo.calls)
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

// #1059: a revoked session's leftover access token cannot sign out the rest.
func TestRevokeOtherSessions_RevokedSessionIsRefused(t *testing.T) {
	family := uuid.New()
	repo := &fakeSessionRepo{revoked: 3, gone: map[uuid.UUID]bool{family: true}}
	_, err := NewRevokeOtherSessionsUseCase(repo).Execute(context.Background(),
		RevokeOtherSessionsInput{UserID: uuid.New(), CurrentSession: family})
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != http.StatusUnauthorized {
		t.Fatalf("want an unauthorized error, got %v", err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("nothing may be revoked, got %v", repo.calls)
	}
}
