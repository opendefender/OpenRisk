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

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

type fakeLinks struct {
	rows    []domain.OAuthProvider
	touched []uuid.UUID
}

func (f *fakeLinks) FindByProviderSubject(_ context.Context, provider, subject string) (*domain.OAuthProvider, error) {
	for i := range f.rows {
		if f.rows[i].Provider == provider && f.rows[i].ProviderUserID == subject {
			return &f.rows[i], nil
		}
	}
	return nil, nil
}

func (f *fakeLinks) ListByUser(_ context.Context, userID uuid.UUID) ([]domain.OAuthProvider, error) {
	var out []domain.OAuthProvider
	for _, r := range f.rows {
		if r.UserID == userID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeLinks) Create(_ context.Context, link *domain.OAuthProvider) error {
	if link.ID == uuid.Nil {
		link.ID = uuid.New()
	}
	f.rows = append(f.rows, *link)
	return nil
}

func (f *fakeLinks) TouchLogin(_ context.Context, id uuid.UUID, _ time.Time) error {
	f.touched = append(f.touched, id)
	return nil
}

type fakeProvisioner struct {
	created []OAuthIdentity
}

func (p *fakeProvisioner) ProvisionFromOAuth(_ context.Context, id OAuthIdentity) (*domain.User, error) {
	p.created = append(p.created, id)
	return &domain.User{ID: uuid.New(), Email: id.Email, FullName: id.FullName, IsActive: true}, nil
}

// verifiedIdentity is a well-formed, provider-vouched identity.
func verifiedIdentity(provider, subject, email string) OAuthIdentity {
	return OAuthIdentity{
		Provider: provider, Subject: subject, Email: email,
		EmailVerified: true, FullName: "Test Person",
	}
}

// ---------------------------------------------------------------------------
// 1. Known link
// ---------------------------------------------------------------------------

func TestResolveOAuth_ExistingLinkSignsInEvenIfTheEmailChanged(t *testing.T) {
	// The provider subject is the identity, not the address. Someone who changed
	// their email at Google must keep their OpenRisk account.
	user := activeUser("old-address@opendefender.io")
	links := &fakeLinks{rows: []domain.OAuthProvider{{
		ID: uuid.New(), UserID: user.ID, Provider: "google",
		ProviderUserID: "google-sub-1", Email: "old-address@opendefender.io",
	}}}

	uc := NewResolveOAuthIdentityUseCase(newFakeResetUsers(user), links)
	out, err := uc.Execute(context.Background(), verifiedIdentity("google", "google-sub-1", "brand-new@opendefender.io"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.User.ID != user.ID {
		t.Error("expected the linked account, resolved by subject")
	}
	if out.Linked {
		t.Error("an existing link must not be recreated")
	}
	if len(links.touched) != 1 {
		t.Error("expected the sign-in recorded on the link")
	}
}

func TestResolveOAuth_DisabledAccountIsRefused(t *testing.T) {
	user := activeUser("disabled@opendefender.io")
	user.IsActive = false
	links := &fakeLinks{rows: []domain.OAuthProvider{{
		ID: uuid.New(), UserID: user.ID, Provider: "github", ProviderUserID: "gh-1",
	}}}

	uc := NewResolveOAuthIdentityUseCase(newFakeResetUsers(user), links)
	_, err := uc.Execute(context.Background(), verifiedIdentity("github", "gh-1", user.Email))

	if !errors.Is(err, ErrOAuthAccountDisabled) {
		t.Fatalf("expected ErrOAuthAccountDisabled, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// 2. Linking by verified email — the account-takeover gate
// ---------------------------------------------------------------------------

func TestResolveOAuth_LinksToExistingAccountOnVerifiedEmail(t *testing.T) {
	user := activeUser("member@opendefender.io")
	links := &fakeLinks{}

	uc := NewResolveOAuthIdentityUseCase(newFakeResetUsers(user), links)
	out, err := uc.Execute(context.Background(), verifiedIdentity("google", "google-sub-9", user.Email))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.User.ID != user.ID || !out.Linked {
		t.Fatalf("expected a new link to the existing account, got %+v", out)
	}
	if len(links.rows) != 1 || links.rows[0].ProviderUserID != "google-sub-9" {
		t.Error("expected the provider subject persisted on the link")
	}
}

func TestResolveOAuth_UnverifiedEmailCannotReachAnExistingAccount(t *testing.T) {
	// The classic OAuth takeover: register at any provider claiming someone
	// else's address. Without the verification gate, this returns their account.
	victim := activeUser("ceo@opendefender.io")
	links := &fakeLinks{}

	identity := verifiedIdentity("github", "attacker-sub", victim.Email)
	identity.EmailVerified = false // the provider did NOT vouch for it

	uc := NewResolveOAuthIdentityUseCase(newFakeResetUsers(victim), links)
	_, err := uc.Execute(context.Background(), identity)

	if !errors.Is(err, ErrOAuthEmailUnverified) {
		t.Fatalf("expected ErrOAuthEmailUnverified, got %v", err)
	}
	if len(links.rows) != 0 {
		t.Fatal("no link may be written for an unverified address")
	}
}

func TestResolveOAuth_ProviderConflictNamesTheProviderThatOwnsTheAddress(t *testing.T) {
	// The account already signs in with Google. Signing in with GitHub on the
	// same address must not quietly add a second door — and the refusal has to
	// tell the user where to go, or they are stranded on their own account.
	user := activeUser("member@opendefender.io")
	links := &fakeLinks{rows: []domain.OAuthProvider{{
		ID: uuid.New(), UserID: user.ID, Provider: "google", ProviderUserID: "google-sub-1",
	}}}

	uc := NewResolveOAuthIdentityUseCase(newFakeResetUsers(user), links)
	_, err := uc.Execute(context.Background(), verifiedIdentity("github", "gh-sub-2", user.Email))

	if !errors.Is(err, ErrOAuthProviderConflict) {
		t.Fatalf("expected a provider conflict, got %v", err)
	}

	var conflict *OAuthProviderConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected a typed conflict carrying both providers, got %T", err)
	}
	if conflict.ExistingProvider != "google" || conflict.AttemptedProvider != "github" {
		t.Errorf("expected google→github, got %s→%s", conflict.ExistingProvider, conflict.AttemptedProvider)
	}
	if len(links.rows) != 1 {
		t.Error("no second link may be created on conflict")
	}
}

func TestResolveOAuth_SameProviderDifferentSubjectIsNotAConflict(t *testing.T) {
	// Same provider, new subject (e.g. the workspace account was recreated). That
	// is an additional identity at a provider the account already trusts, not a
	// cross-provider takeover.
	user := activeUser("member@opendefender.io")
	links := &fakeLinks{rows: []domain.OAuthProvider{{
		ID: uuid.New(), UserID: user.ID, Provider: "google", ProviderUserID: "google-sub-old",
	}}}

	uc := NewResolveOAuthIdentityUseCase(newFakeResetUsers(user), links)
	out, err := uc.Execute(context.Background(), verifiedIdentity("google", "google-sub-new", user.Email))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Linked || out.User.ID != user.ID {
		t.Errorf("expected the new subject linked to the same account, got %+v", out)
	}
}

func TestResolveOAuth_NoEmailFromProvider(t *testing.T) {
	uc := NewResolveOAuthIdentityUseCase(newFakeResetUsers(), &fakeLinks{})

	identity := verifiedIdentity("github", "gh-1", "")
	_, err := uc.Execute(context.Background(), identity)

	if !errors.Is(err, ErrOAuthNoEmail) {
		t.Fatalf("expected ErrOAuthNoEmail, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// 3. Provisioning
// ---------------------------------------------------------------------------

func TestResolveOAuth_UnknownIdentityIsRefusedWithoutAProvisioner(t *testing.T) {
	// SSO signs EXISTING members in; it does not create tenants. Failing closed
	// is the deliberate default.
	uc := NewResolveOAuthIdentityUseCase(newFakeResetUsers(), &fakeLinks{})

	_, err := uc.Execute(context.Background(), verifiedIdentity("google", "sub-x", "stranger@example.com"))

	if !errors.Is(err, ErrOAuthNoAccount) {
		t.Fatalf("expected ErrOAuthNoAccount, got %v", err)
	}
}

func TestResolveOAuth_ProvisionsWhenEnabled(t *testing.T) {
	prov := &fakeProvisioner{}
	links := &fakeLinks{}
	uc := NewResolveOAuthIdentityUseCase(newFakeResetUsers(), links).WithProvisioner(prov)

	out, err := uc.Execute(context.Background(), verifiedIdentity("azure", "aad-1", "newhire@opendefender.io"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Provisioned || !out.Linked {
		t.Errorf("expected the account created and linked, got %+v", out)
	}
	if len(prov.created) != 1 || len(links.rows) != 1 {
		t.Error("expected exactly one account and one link")
	}
}

func TestResolveOAuth_ProvisioningAlsoRequiresAVerifiedEmail(t *testing.T) {
	// Provisioning keys the new account on this address, so it needs the same
	// guarantee as linking: otherwise anyone can create an OpenRisk account under
	// a colleague's address and be there when that colleague is invited.
	prov := &fakeProvisioner{}
	uc := NewResolveOAuthIdentityUseCase(newFakeResetUsers(), &fakeLinks{}).WithProvisioner(prov)

	identity := verifiedIdentity("github", "gh-9", "someone@opendefender.io")
	identity.EmailVerified = false

	_, err := uc.Execute(context.Background(), identity)

	if !errors.Is(err, ErrOAuthEmailUnverified) {
		t.Fatalf("expected ErrOAuthEmailUnverified, got %v", err)
	}
	if len(prov.created) != 0 {
		t.Error("no account may be provisioned from an unverified address")
	}
}

func TestResolveOAuth_IncompleteIdentityIsRejected(t *testing.T) {
	uc := NewResolveOAuthIdentityUseCase(newFakeResetUsers(), &fakeLinks{})

	for _, id := range []OAuthIdentity{
		{Provider: "", Subject: "s", Email: "a@b.io", EmailVerified: true},
		{Provider: "google", Subject: "", Email: "a@b.io", EmailVerified: true},
	} {
		if _, err := uc.Execute(context.Background(), id); err == nil {
			t.Errorf("expected %+v to be rejected", id)
		}
	}
}

func TestResolveOAuth_EmailMatchIsCaseInsensitive(t *testing.T) {
	user := activeUser("member@opendefender.io")
	uc := NewResolveOAuthIdentityUseCase(newFakeResetUsers(user), &fakeLinks{})

	out, err := uc.Execute(context.Background(), verifiedIdentity("google", "sub-1", "Member@OpenDefender.IO"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.User.ID != user.ID {
		t.Error("address matching must be case-insensitive")
	}
}
