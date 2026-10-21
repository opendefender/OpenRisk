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

// fakeSessionRepo records the keep-hash RevokeAllExcept was called with.
type fakeSessionRepo struct {
	keepHash string
	calls    []string
	revoked  int64
}

func (f *fakeSessionRepo) ListByUser(context.Context, uuid.UUID) ([]domain.SessionRecord, error) {
	return nil, nil
}
func (f *fakeSessionRepo) Revoke(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (f *fakeSessionRepo) RevokeAllExcept(_ context.Context, _ uuid.UUID, keepHash string) (int64, error) {
	f.keepHash = keepHash
	f.calls = append(f.calls, "revoke")
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
