// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRefreshRevoker struct {
	known   map[string]bool
	revoked []string
}

func (f *fakeRefreshRevoker) RevokeRefreshToken(_ context.Context, v string) error {
	if !f.known[v] {
		return errors.New("refresh token not found")
	}
	f.revoked = append(f.revoked, v)
	return nil
}

type failingJTIRevoker struct{ calls int }

func (f *failingJTIRevoker) BlacklistJTI(context.Context, string, time.Duration) error {
	f.calls++
	return errors.New("redis down")
}

func newLogoutFixture() (*LogoutUseCase, *fakeRefreshRevoker, *fakeJTIRevoker) {
	refresh := &fakeRefreshRevoker{known: map[string]bool{"rt-1": true}}
	access := &fakeJTIRevoker{jtis: map[string]time.Duration{}}
	return NewLogoutUseCase(refresh).WithAccessTokenRevocation(access), refresh, access
}

func TestLogout_Success(t *testing.T) {
	uc, refresh, access := newLogoutFixture()
	err := uc.Execute(context.Background(), LogoutInput{
		RefreshToken: "rt-1", AccessJTI: "jti-1", AccessExpiresAt: time.Now().Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(refresh.revoked) != 1 {
		t.Fatal("the refresh token must be revoked")
	}
	ttl, ok := access.jtis["jti-1"]
	if !ok || ttl <= 9*time.Minute || ttl > 10*time.Minute {
		t.Fatalf("the access token must be blacklisted for what it has left, got %v %v", ok, ttl)
	}
}

// A refresh token that is already gone still lets the access token die.
func TestLogout_NotFound(t *testing.T) {
	uc, _, access := newLogoutFixture()
	err := uc.Execute(context.Background(), LogoutInput{
		RefreshToken: "rt-unknown", AccessJTI: "jti-1", AccessExpiresAt: time.Now().Add(time.Minute),
	})
	if err == nil {
		t.Fatal("an unknown refresh token must be reported")
	}
	if !access.revoked("jti-1") {
		t.Fatal("the access token must be revoked even when the refresh token is unknown")
	}
}

func TestLogout_Unauthorized(t *testing.T) {
	uc, refresh, access := newLogoutFixture()
	if err := uc.Execute(context.Background(), LogoutInput{}); err == nil {
		t.Fatal("a logout with no credential must fail")
	}
	if len(refresh.revoked) != 0 || len(access.jtis) != 0 {
		t.Fatal("nothing may be revoked without a credential")
	}
}

func TestLogout_ExpiredAccessTokenNeedsNoBlacklist(t *testing.T) {
	uc, _, access := newLogoutFixture()
	_ = uc.Execute(context.Background(), LogoutInput{
		RefreshToken: "rt-1", AccessJTI: "jti-1", AccessExpiresAt: time.Now().Add(-time.Second),
	})
	if access.revoked("jti-1") {
		t.Fatal("an expired token is already dead; blacklisting it wastes a key")
	}
}

// A Redis outage on the blacklist must not spare the refresh token.
func TestLogout_BlacklistOutageStillRevokesRefreshToken(t *testing.T) {
	refresh := &fakeRefreshRevoker{known: map[string]bool{"rt-1": true}}
	broken := &failingJTIRevoker{}
	uc := NewLogoutUseCase(refresh).WithAccessTokenRevocation(broken)

	err := uc.Execute(context.Background(), LogoutInput{
		RefreshToken: "rt-1", AccessJTI: "jti-1", AccessExpiresAt: time.Now().Add(time.Minute),
	})
	if err == nil {
		t.Fatal("the outage must be reported")
	}
	if broken.calls != 1 || len(refresh.revoked) != 1 {
		t.Fatalf("both revocations must be attempted: blacklist=%d refresh=%d", broken.calls, len(refresh.revoked))
	}
}
