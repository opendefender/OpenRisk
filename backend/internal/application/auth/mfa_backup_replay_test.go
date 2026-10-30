// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// staleBackupReads hands out every backup code, spent or not: what each of two
// racing challenges sees when both read before either one writes (#946).
type staleBackupReads struct{ *MockMFARepository }

func (s staleBackupReads) GetUnusedBackupCodes(_ context.Context, userID, tenantID uuid.UUID) ([]*domain.MFABackupCode, error) {
	return s.codes[userID.String()+":"+tenantID.String()], nil
}

// #946 — a backup code opens one session, never two.
func TestMFAChallenge_BackupCodeUsedOnce(t *testing.T) {
	f := newChallengeFixture(t)
	if err := f.try("jti-1", f.backup); err != nil {
		t.Fatalf("first use of a backup code must pass: %v", err)
	}
	if err := f.try("jti-2", f.backup); !isInvalidCode(err) {
		t.Fatalf("second use of a backup code: want invalid code, got %v", err)
	}
}

// The race itself: the second challenge read the code as unused and its hash
// matches, but the conditional write finds it spent. That refusal is a failed
// attempt for the #689 limits, exactly like a wrong code.
func TestMFAChallenge_BackupCodeLostRaceIsAFailedAttempt(t *testing.T) {
	f := newChallengeFixture(t)
	f.uc.mfaRepo = staleBackupReads{f.repo}

	if err := f.try("jti-1", f.backup); err != nil {
		t.Fatalf("the winner must pass: %v", err)
	}
	before, _ := f.store.Count(context.Background(), userFailuresKey(f.userID))
	if err := f.try("jti-2", f.backup); !isInvalidCode(err) {
		t.Fatalf("the loser: want invalid code, got %v", err)
	}
	after, _ := f.store.Count(context.Background(), userFailuresKey(f.userID))
	if after != before+1 {
		t.Fatalf("account failures: got %d, want %d", after, before+1)
	}
}

// A code id is only spent for the user and tenant it belongs to.
func TestMockMFARepository_MarkBackupCodeAsUsed_ScopedToOwner(t *testing.T) {
	f := newChallengeFixture(t)
	ctx := context.Background()
	code := f.repo.codes[f.userID.String()+":"+f.tenantID.String()][0]

	if ok, _ := f.repo.MarkBackupCodeAsUsed(ctx, code.ID, f.userID, uuid.New()); ok {
		t.Fatal("another tenant must not spend the code")
	}
	if ok, _ := f.repo.MarkBackupCodeAsUsed(ctx, code.ID, f.userID, f.tenantID); !ok {
		t.Fatal("the owner must spend the code once")
	}
	if ok, _ := f.repo.MarkBackupCodeAsUsed(ctx, code.ID, f.userID, f.tenantID); ok {
		t.Fatal("a spent code must not be spent twice")
	}
}
