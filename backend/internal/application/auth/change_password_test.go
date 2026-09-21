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
	"github.com/opendefender/openrisk/pkg/pwpolicy"
)

type fakeReissuer struct {
	revokedFor uuid.UUID
	issuedFor  uuid.UUID
	issuedOrg  uuid.UUID
	calls      []string
	issueErr   error
}

func (f *fakeReissuer) RevokeAllUserTokens(_ context.Context, userID uuid.UUID) error {
	f.calls = append(f.calls, "revoke")
	f.revokedFor = userID
	return nil
}

func (f *fakeReissuer) IssueSessionForOrg(_ context.Context, userID, orgID uuid.UUID, _ auth.DeviceContext) (*auth.TokenPair, error) {
	f.calls = append(f.calls, "issue")
	if f.issueErr != nil {
		return nil, f.issueErr
	}
	f.issuedFor, f.issuedOrg = userID, orgID
	return &auth.TokenPair{AccessToken: "new-access", RefreshToken: "new-refresh"}, nil
}

type fakeChangedMailer struct{ to, locale string }

func (f *fakeChangedMailer) SendPasswordChanged(_ context.Context, to, _ string, locale string) error {
	f.to, f.locale = to, locale
	return nil
}

const strongPassword = "Violet-Kilimanjaro-Anchor-2026!"

func changeHarness(user *domain.User) (*ChangePasswordUseCase, *fakeResetUsers, *fakeReissuer, *fakeChangedMailer) {
	users := newFakeResetUsers(user)
	sessions := &fakeReissuer{}
	mailer := &fakeChangedMailer{}
	return NewChangePasswordUseCase(users, fakeHasher{}, pwpolicy.New(), sessions, mailer), users, sessions, mailer
}

func userWithPassword(plain string) *domain.User {
	u := activeUser("alice@opendefender.io")
	u.Password = "hashed:" + plain
	return u
}

func TestChangePassword_Success(t *testing.T) {
	user := userWithPassword("Old-Password-Still-Long-1")
	uc, users, sessions, mailer := changeHarness(user)

	out, err := uc.Execute(context.Background(), ChangePasswordInput{
		UserID: user.ID, TenantID: uuid.New(), CurrentPassword: "Old-Password-Still-Long-1", NewPassword: strongPassword,
		Locale: "en",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(users.updated) != 1 || users.updated[0].Password != "hashed:"+strongPassword {
		t.Fatalf("the new password was not stored: %+v", users.updated)
	}
	if !(fakeHasher{}).Verify(user.Password, strongPassword) || (fakeHasher{}).Verify(user.Password, "Old-Password-Still-Long-1") {
		t.Fatal("the new password must verify and the old one must not")
	}
	if out.TokenPair == nil || out.TokenPair.AccessToken != "new-access" {
		t.Fatalf("the calling device must get a fresh session: %+v", out)
	}
	if mailer.to != "alice@opendefender.io" || mailer.locale != "en" {
		t.Fatalf("notice: %+v", mailer)
	}
	_ = sessions
}

func TestChangePassword_RevokesOtherSessions(t *testing.T) {
	user := userWithPassword("Old-Password-Still-Long-1")
	uc, _, sessions, _ := changeHarness(user)
	org := uuid.New()
	if _, err := uc.Execute(context.Background(), ChangePasswordInput{
		UserID: user.ID, TenantID: org, CurrentPassword: "Old-Password-Still-Long-1", NewPassword: strongPassword,
	}); err != nil {
		t.Fatal(err)
	}
	// Every session ends FIRST, then exactly one new one is minted for this
	// device in the same organization — so no pre-change session survives.
	if len(sessions.calls) != 2 || sessions.calls[0] != "revoke" || sessions.calls[1] != "issue" {
		t.Fatalf("want revoke then issue, got %v", sessions.calls)
	}
	if sessions.revokedFor != user.ID || sessions.issuedFor != user.ID || sessions.issuedOrg != org {
		t.Fatalf("scope: %+v", sessions)
	}
}

func TestChangePassword_ReissueFailureStillChangesThePassword(t *testing.T) {
	user := userWithPassword("Old-Password-Still-Long-1")
	uc, users, sessions, _ := changeHarness(user)
	sessions.issueErr = errors.New("resolver down")
	out, err := uc.Execute(context.Background(), ChangePasswordInput{
		UserID: user.ID, TenantID: uuid.New(), CurrentPassword: "Old-Password-Still-Long-1", NewPassword: strongPassword,
	})
	if err != nil || out.TokenPair != nil || len(users.updated) != 1 {
		t.Fatalf("the change stands and the caller signs in again: %+v, %v", out, err)
	}
}

func TestChangePassword_NotFound(t *testing.T) {
	uc, users, _, _ := changeHarness(userWithPassword("x"))
	_, err := uc.Execute(context.Background(), ChangePasswordInput{UserID: uuid.New(), CurrentPassword: "x", NewPassword: strongPassword})
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %v", err)
	}
	if len(users.updated) != 0 {
		t.Fatal("nothing may be written")
	}
}

func TestChangePassword_Unauthorized(t *testing.T) {
	user := userWithPassword("Old-Password-Still-Long-1")
	uc, users, sessions, mailer := changeHarness(user)

	_, err := uc.Execute(context.Background(), ChangePasswordInput{UserID: uuid.Nil, CurrentPassword: "x", NewPassword: strongPassword})
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != http.StatusUnauthorized {
		t.Fatalf("no session: want 401, got %v", err)
	}

	for _, wrong := range []string{"", "old-password-still-long-1", "Old-Password-Still-Long-2"} {
		_, err = uc.Execute(context.Background(), ChangePasswordInput{UserID: user.ID, CurrentPassword: wrong, NewPassword: strongPassword})
		if !errors.Is(err, ErrCurrentPasswordIncorrect) {
			t.Fatalf("wrong current password %q: got %v", wrong, err)
		}
	}
	if len(users.updated) != 0 || len(sessions.calls) != 0 || mailer.to != "" {
		t.Fatal("a refused change must write, revoke and send nothing")
	}
}

func TestChangePassword_PolicyViolation(t *testing.T) {
	user := userWithPassword("Old-Password-Still-Long-1")
	uc, users, _, _ := changeHarness(user)

	out, err := uc.Execute(context.Background(), ChangePasswordInput{UserID: user.ID, CurrentPassword: "Old-Password-Still-Long-1", NewPassword: "short"})
	if err == nil || out == nil || out.Assessment == nil || out.Assessment.OK {
		t.Fatalf("a weak password must be refused with its assessment: %+v, %v", out, err)
	}

	_, err = uc.Execute(context.Background(), ChangePasswordInput{UserID: user.ID, CurrentPassword: "Old-Password-Still-Long-1", NewPassword: "Old-Password-Still-Long-1"})
	if !errors.Is(err, ErrSamePassword) {
		t.Fatalf("same password: got %v", err)
	}
	if len(users.updated) != 0 {
		t.Fatal("a refused change must write nothing")
	}
}

func TestChangePassword_NoLocalPassword(t *testing.T) {
	user := activeUser("sso@opendefender.io") // provisioned by SAML/OAuth: no password
	uc, users, _, _ := changeHarness(user)
	_, err := uc.Execute(context.Background(), ChangePasswordInput{UserID: user.ID, CurrentPassword: "anything", NewPassword: strongPassword})
	if !errors.Is(err, ErrNoLocalPassword) {
		t.Fatalf("got %v", err)
	}
	if len(users.updated) != 0 {
		t.Fatal("nothing may be written")
	}
}
