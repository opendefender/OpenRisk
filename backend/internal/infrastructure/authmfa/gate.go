// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package authmfa provides a reusable MFA verification gate for high-risk actions
// (e.g. danger-zone org deletion) built on the existing MFA secret store + TOTP.
package authmfa

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/pkg/crypto"
	"github.com/opendefender/openrisk/pkg/otp"
)

// Gate verifies a TOTP code for a user who has MFA enrolled.
type Gate struct {
	mfaRepo repository.MFARepository
	encKey  []byte
}

func NewGate(mfaRepo repository.MFARepository, encKey []byte) *Gate {
	return &Gate{mfaRepo: mfaRepo, encKey: encKey}
}

// ErrInvalidCode is returned when an enrolled user's TOTP code is wrong.
var ErrInvalidCode = errors.New("mfa: invalid code")

// VerifyRequired returns nil when the user is NOT enrolled (exempt — you cannot
// force TOTP on someone without an authenticator) OR the supplied code is valid;
// otherwise ErrInvalidCode. This is the contract orgdeletion.MFAGate expects.
func (g *Gate) VerifyRequired(ctx context.Context, user, tenant uuid.UUID, code string) error {
	secret, err := g.mfaRepo.GetMFASecret(ctx, user, tenant)
	if err != nil || secret == nil || !secret.IsVerified {
		// Not enrolled → exempt.
		return nil
	}
	plain, err := crypto.DecryptAES256GCM(secret.SecretEncrypted, g.encKey)
	if err != nil {
		return ErrInvalidCode
	}
	if !otp.VerifyTOTP(plain, code) {
		return ErrInvalidCode
	}
	return nil
}
