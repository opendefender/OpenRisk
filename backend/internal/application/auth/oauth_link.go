// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// OAuthIdentity is what a provider told us about the person signing in.
type OAuthIdentity struct {
	Provider string // "google" | "github" | "azure"
	// Subject is the provider's stable user ID. This, not the email, is the
	// identity: emails get changed and reassigned, subjects do not.
	Subject string
	Email   string
	// EmailVerified reports whether the PROVIDER has verified the address.
	//
	// This is the pivot of the whole linking decision. An unverified address is
	// an attacker-chosen string: sign up at a provider claiming
	// ceo@victim-corp.com, and if we linked on the address alone, we would hand
	// over that account. Google and Microsoft state this explicitly; for GitHub
	// it is derived from the verified flag on the primary email.
	EmailVerified bool
	FullName      string
	AvatarURL     string
}

// OAuthLinkRepository stores provider links.
type OAuthLinkRepository interface {
	// FindByProviderSubject looks a link up by its stable provider identity.
	FindByProviderSubject(ctx context.Context, provider, subject string) (*domain.OAuthProvider, error)
	// ListByUser returns every provider linked to a user.
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.OAuthProvider, error)
	// Create links a provider to a user.
	Create(ctx context.Context, link *domain.OAuthProvider) error
	// TouchLogin records a successful sign-in through a link.
	TouchLogin(ctx context.Context, id uuid.UUID, at time.Time) error
}

// OAuthUserRepository is the narrow slice of user storage linking needs.
type OAuthUserRepository interface {
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

// Linking outcomes that the handler renders as user-facing messages.
var (
	// ErrOAuthEmailUnverified — the provider did not vouch for the address, so it
	// cannot be used to reach an existing account.
	ErrOAuthEmailUnverified = errors.New("oauth email not verified by provider")

	// ErrOAuthProviderConflict — the address belongs to an account already tied
	// to a different provider.
	ErrOAuthProviderConflict = errors.New("email already linked to another provider")

	// ErrOAuthNoEmail — the provider returned no address at all.
	ErrOAuthNoEmail = errors.New("oauth provider returned no email address")

	// ErrOAuthAccountDisabled — the account exists but is deactivated.
	ErrOAuthAccountDisabled = errors.New("account is disabled")

	// ErrOAuthNoAccount — no account, and auto-provisioning is off.
	ErrOAuthNoAccount = errors.New("no account for this identity")
)

// OAuthProviderConflictError carries which provider already owns the address, so
// the login screen can say "this address signs in with Google" instead of a
// generic refusal that leaves the user with nothing to try.
type OAuthProviderConflictError struct {
	ExistingProvider  string
	AttemptedProvider string
}

func (e *OAuthProviderConflictError) Error() string {
	return fmt.Sprintf("email already linked to %s, not %s", e.ExistingProvider, e.AttemptedProvider)
}

func (e *OAuthProviderConflictError) Unwrap() error { return ErrOAuthProviderConflict }

// ResolveOAuthIdentityOutput reports how the identity was resolved.
type ResolveOAuthIdentityOutput struct {
	User *domain.User
	// Linked is true when this sign-in created the provider link.
	Linked bool
	// Provisioned is true when this sign-in created the account.
	Provisioned bool
}

// UserProvisioner creates an account for an identity that has none.
//
// Optional: when nil, an unknown identity is refused with ErrOAuthNoAccount
// rather than silently admitted. Deployments that want SSO to be invite-only
// simply leave it unwired.
type UserProvisioner interface {
	ProvisionFromOAuth(ctx context.Context, identity OAuthIdentity) (*domain.User, error)
}

// ResolveOAuthIdentityUseCase turns a provider identity into an OpenRisk account.
type ResolveOAuthIdentityUseCase struct {
	users       OAuthUserRepository
	links       OAuthLinkRepository
	provisioner UserProvisioner
}

// NewResolveOAuthIdentityUseCase builds the use case.
func NewResolveOAuthIdentityUseCase(users OAuthUserRepository, links OAuthLinkRepository) *ResolveOAuthIdentityUseCase {
	return &ResolveOAuthIdentityUseCase{users: users, links: links}
}

// WithProvisioner enables account creation for unknown identities.
func (uc *ResolveOAuthIdentityUseCase) WithProvisioner(p UserProvisioner) *ResolveOAuthIdentityUseCase {
	uc.provisioner = p
	return uc
}

// Execute resolves an identity to an account, linking or provisioning as needed.
//
// The order of the three branches is the security design:
//
//  1. Known link (provider + subject). The strongest signal, and checked first
//     so a user whose email changed at the provider keeps their account.
//
//  2. Verified email matching an existing account → link it. This is the only
//     way a provider identity may attach to an account it has never signed into,
//     and it is gated on the PROVIDER having verified the address. Skipping that
//     check is the classic OAuth account-takeover: register at any provider with
//     someone else's address and inherit their account.
//
//     If that account already signs in through a DIFFERENT provider, we stop and
//     say so rather than quietly adding a second door to it.
//
//  3. No account → provision, if a provisioner is wired.
func (uc *ResolveOAuthIdentityUseCase) Execute(ctx context.Context, identity OAuthIdentity) (*ResolveOAuthIdentityOutput, error) {
	if identity.Provider == "" || identity.Subject == "" {
		return nil, domain.NewValidationError("provider identity is incomplete")
	}

	// --- 1. Already linked ---------------------------------------------------
	link, err := uc.links.FindByProviderSubject(ctx, identity.Provider, identity.Subject)
	if err != nil {
		return nil, fmt.Errorf("failed to look up provider link: %w", err)
	}
	if link != nil {
		user, err := uc.users.GetByID(ctx, link.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to load linked user: %w", err)
		}
		if user == nil {
			// A link whose user is gone is stale data, not an identity.
			return nil, ErrOAuthNoAccount
		}
		if !user.IsActive {
			return nil, ErrOAuthAccountDisabled
		}
		_ = uc.links.TouchLogin(ctx, link.ID, time.Now())
		return &ResolveOAuthIdentityOutput{User: user}, nil
	}

	email := domain.NormaliseEmail(identity.Email)
	if email == "" {
		return nil, ErrOAuthNoEmail
	}

	// --- 2. Match an existing account by VERIFIED email ----------------------
	existing, err := uc.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to look up user by email: %w", err)
	}

	if existing != nil {
		// Gate the match on provider verification. Note this check sits AFTER the
		// lookup only for code shape; it refuses before anything is written.
		if !identity.EmailVerified {
			return nil, ErrOAuthEmailUnverified
		}
		if !existing.IsActive {
			return nil, ErrOAuthAccountDisabled
		}

		// Does this account already sign in through some other provider?
		others, err := uc.links.ListByUser(ctx, existing.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list existing links: %w", err)
		}
		for _, other := range others {
			if !strings.EqualFold(other.Provider, identity.Provider) {
				return nil, &OAuthProviderConflictError{
					ExistingProvider:  other.Provider,
					AttemptedProvider: identity.Provider,
				}
			}
		}

		newLink := &domain.OAuthProvider{
			UserID:         existing.ID,
			Provider:       identity.Provider,
			ProviderUserID: identity.Subject,
			Email:          email,
		}
		if existing.DefaultOrgID != nil {
			newLink.TenantID = *existing.DefaultOrgID
		}
		if err := uc.links.Create(ctx, newLink); err != nil {
			return nil, fmt.Errorf("failed to link provider: %w", err)
		}
		return &ResolveOAuthIdentityOutput{User: existing, Linked: true}, nil
	}

	// --- 3. Brand new identity ----------------------------------------------
	if uc.provisioner == nil {
		return nil, ErrOAuthNoAccount
	}
	// Provisioning creates an account keyed on this address, so it needs the same
	// verification guarantee as linking does.
	if !identity.EmailVerified {
		return nil, ErrOAuthEmailUnverified
	}

	user, err := uc.provisioner.ProvisionFromOAuth(ctx, identity)
	if err != nil {
		return nil, fmt.Errorf("failed to provision user: %w", err)
	}

	newLink := &domain.OAuthProvider{
		UserID:         user.ID,
		Provider:       identity.Provider,
		ProviderUserID: identity.Subject,
		Email:          email,
	}
	if user.DefaultOrgID != nil {
		newLink.TenantID = *user.DefaultOrgID
	}
	if err := uc.links.Create(ctx, newLink); err != nil {
		return nil, fmt.Errorf("failed to link provider: %w", err)
	}

	return &ResolveOAuthIdentityOutput{User: user, Linked: true, Provisioned: true}, nil
}
