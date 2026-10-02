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
	"github.com/pquerna/otp/totp"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/crypto"
	"github.com/opendefender/openrisk/pkg/otp"
)

// uniqueMFARepository enforces mfa_secrets.user_id UNIQUE, which the plain mock
// does not: it overwrote on Create, so the defect behind #889 never showed.
type uniqueMFARepository struct{ *MockMFARepository }

func (r uniqueMFARepository) CreateMFASecret(ctx context.Context, s *domain.MFASecret) error {
	for _, existing := range r.secrets {
		if existing.UserID == s.UserID {
			return errors.New("duplicated key not allowed")
		}
	}
	return r.MockMFARepository.CreateMFASecret(ctx, s)
}

func TestSetupMFA_ReplacesAnUnverifiedSecret(t *testing.T) {
	ctx := context.Background()
	key := make([]byte, 32)
	repo := uniqueMFARepository{NewMockMFARepository()}
	uc := NewSetupMFAUseCase(repo, key)
	in := SetupMFAInput{UserID: uuid.New(), TenantID: uuid.New(), Email: "awa@example.test"}

	first, err := uc.Execute(ctx, in)
	if err != nil {
		t.Fatalf("first setup: %v", err)
	}
	// The enrolment is abandoned: the secret stays unverified. Starting over
	// must work, not fail on the unique user_id forever.
	second, err := uc.Execute(ctx, in)
	if err != nil {
		t.Fatalf("setup after an unfinished one must succeed, got %v", err)
	}
	if second.Secret == first.Secret {
		t.Fatal("a fresh enrolment must get a fresh secret")
	}

	stored, _ := repo.GetMFASecret(ctx, in.UserID, in.TenantID)
	plain, err := crypto.DecryptAES256GCM(stored.SecretEncrypted, key)
	if err != nil {
		t.Fatal(err)
	}
	if plain != second.Secret {
		t.Fatal("the stored secret must be the new one, so only the new QR code verifies")
	}
	if stored.IsVerified {
		t.Fatal("the replaced secret must still await verification")
	}
	code, _ := totp.GenerateCode(second.Secret, time.Now())
	if !otp.VerifyTOTP(plain, code) {
		t.Fatal("a code from the new QR code must verify against the stored secret")
	}
	oldCode, _ := totp.GenerateCode(first.Secret, time.Now())
	if oldCode != code && otp.VerifyTOTP(plain, oldCode) {
		t.Fatal("a code from the abandoned QR code must no longer verify")
	}
}

func TestSetupMFA_VerifiedSecretIsKept(t *testing.T) {
	ctx := context.Background()
	key := make([]byte, 32)
	repo := uniqueMFARepository{NewMockMFARepository()}
	userID, tenantID := uuid.New(), uuid.New()
	enc, _ := crypto.EncryptAES256GCM("KEEPME", key)
	_ = repo.MockMFARepository.CreateMFASecret(ctx, &domain.MFASecret{
		ID: uuid.New(), UserID: userID, TenantID: tenantID, SecretEncrypted: enc, IsVerified: true,
	})

	_, err := NewSetupMFAUseCase(repo, key).Execute(ctx, SetupMFAInput{UserID: userID, TenantID: tenantID, Email: "a@example.test"})
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || !errors.Is(appErr.Err, domain.ErrConflict) {
		t.Fatalf("a verified secret must answer Conflict, got %v", err)
	}
	stored, _ := repo.GetMFASecret(ctx, userID, tenantID)
	if stored.SecretEncrypted != enc {
		t.Fatal("a verified secret must not be touched")
	}
}
