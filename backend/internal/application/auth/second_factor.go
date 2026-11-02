// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// SecondFactorStep is what a sign-in still owes before a session may exist.
type SecondFactorStep int

const (
	// SecondFactorNone: the first factor is enough, a session may be issued.
	SecondFactorNone SecondFactorStep = iota
	// SecondFactorChallenge: a verified authenticator is enrolled and must be
	// presented at /auth/mfa/challenge.
	SecondFactorChallenge
	// SecondFactorEnrollment: the role mandates MFA, nothing is enrolled and the
	// grace window is over; the enrolment endpoints are the only way on.
	SecondFactorEnrollment
)

// SecondFactorOutcome is the gate's answer. MFAToken is set for every step but
// SecondFactorNone: an MFA_REQUIRED token for a challenge, an MFA_ENROLLMENT
// token for enrolment. Neither carries permissions or a refresh token.
type SecondFactorOutcome struct {
	Step      SecondFactorStep
	MFAToken  string
	MFAStatus domain.MFADecision
}

// SSOSecondFactor runs the password sign-in's MFA gate for a user an identity
// provider has just authenticated (GHSA-rv5c-pr27-gpxh).
//
// The IdP proves the first factor only. An authenticator the user enrolled in
// OpenRisk is still asked for, and the role policy still applies, exactly as
// after a correct password. The organisation is the default one, the same the
// session resolver and the MFA challenge issue the session for.
func (uc *LoginUseCase) SSOSecondFactor(ctx context.Context, userID uuid.UUID) (*SecondFactorOutcome, error) {
	org, err := uc.userRepo.GetUserDefaultOrganization(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user organization: %w", err)
	}
	if org == nil {
		return nil, domain.NewValidationError("user has no organization")
	}
	member, err := uc.userRepo.GetOrganizationMember(ctx, userID, org.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization membership: %w", err)
	}
	// Refused before any factor is asked for, as password login does: a code
	// prompt the session resolver would then refuse anyway is a dead end.
	if member != nil && !member.IsActive {
		return nil, domain.NewValidationError("your access to this organization has been revoked")
	}
	out, err := uc.secondFactor(ctx, userID, org.ID, member)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// SecondFactorForOrg runs the same gate for a session moving into another
// organisation (GHSA-45mr-j2w6-m4jv). Secrets and policies are kept per
// membership (D-066), so the factor proved for the organisation the caller
// left says nothing about this one. The caller has already checked that the
// membership is active.
func (uc *LoginUseCase) SecondFactorForOrg(ctx context.Context, userID, orgID uuid.UUID, member *domain.OrganizationMember) (*SecondFactorOutcome, error) {
	out, err := uc.secondFactor(ctx, userID, orgID, member)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// secondFactor is the MFA gate every sign-in passes, whatever proved the first
// factor.
//
// A deployment that wired no MFA repository cannot enrol or enforce anything,
// so the honest resolved state is "recommended": the banner is all there is to
// say. Seeding it here means the status is never the empty string, which the
// SPA would have to special-case. Any read that fails refuses the sign-in.
func (uc *LoginUseCase) secondFactor(ctx context.Context, userID, orgID uuid.UUID, member *domain.OrganizationMember) (SecondFactorOutcome, error) {
	out := SecondFactorOutcome{
		MFAStatus: domain.DecideMFA(domain.MFADecisionInput{GraceDays: domain.MFAGraceDaysDefault, Now: uc.clock()}),
	}
	if uc.mfaRepo == nil {
		return out, nil
	}

	mfaSecret, err := uc.mfaRepo.GetMFASecret(ctx, userID, orgID)
	if err != nil {
		return SecondFactorOutcome{}, fmt.Errorf("failed to check MFA status: %w", err)
	}
	if mfaSecret != nil && mfaSecret.IsVerified {
		token, tErr := uc.tokenManager.GenerateMFAChallengeToken(userID, orgID)
		if tErr != nil {
			return SecondFactorOutcome{}, fmt.Errorf("failed to issue MFA challenge: %w", tErr)
		}
		return SecondFactorOutcome{
			Step:      SecondFactorChallenge,
			MFAToken:  token,
			MFAStatus: uc.decideMFA(ctx, member, orgID, true),
		}, nil
	}

	// OR26-03 — nothing enrolled. Whether that stops the sign-in is a policy
	// question, not a role check: an ordinary member is let in with a banner,
	// and a privileged one until their grace window runs out.
	out.MFAStatus = uc.decideMFA(ctx, member, orgID, false)
	if !out.MFAStatus.Required {
		return out, nil
	}
	token, err := uc.tokenManager.GenerateMFAEnrollmentToken(userID, orgID)
	if err != nil {
		return SecondFactorOutcome{}, fmt.Errorf("failed to issue MFA enrollment token: %w", err)
	}
	out.Step = SecondFactorEnrollment
	out.MFAToken = token
	return out, nil
}
