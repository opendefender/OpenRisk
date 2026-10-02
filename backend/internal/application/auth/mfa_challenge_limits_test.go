// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/authmfa"
	"github.com/opendefender/openrisk/pkg/crypto"
	"github.com/opendefender/openrisk/pkg/otp"
)

// challengeFixture is one enrolled account behind a challenge use case with
// the #689 limits wired to in-memory fakes.
type challengeFixture struct {
	uc       *ChallengeMFAUseCase
	userID   uuid.UUID
	tenantID uuid.UUID
	secret   string
	backup   string
	store    *authmfa.MemoryAttemptStore
	revoker  *fakeJTIRevoker
	mailer   *fakeLockMailer
}

type fakeJTIRevoker struct {
	mu   sync.Mutex
	jtis map[string]time.Duration
}

func (r *fakeJTIRevoker) BlacklistJTI(_ context.Context, jti string, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jtis[jti] = ttl
	return nil
}

func (r *fakeJTIRevoker) revoked(jti string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.jtis[jti]
	return ok
}

type fakeLockMailer struct {
	mu   sync.Mutex
	sent []string
}

func (m *fakeLockMailer) SendMFALocked(_ context.Context, to, _, locale string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, to+"|"+locale)
	return nil
}

func (m *fakeLockMailer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

type fakeLockUsers struct{ user *domain.User }

func (f fakeLockUsers) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	if f.user != nil && f.user.ID == id {
		return f.user, nil
	}
	return nil, nil
}

func newChallengeFixture(t *testing.T) *challengeFixture {
	t.Helper()
	key := make([]byte, 32)
	repo := NewMockMFARepository()
	f := &challengeFixture{
		userID:   uuid.New(),
		tenantID: uuid.New(),
		store:    authmfa.NewMemoryAttemptStore(),
		revoker:  &fakeJTIRevoker{jtis: map[string]time.Duration{}},
		mailer:   &fakeLockMailer{},
		backup:   "K7M2Q9XT",
	}

	secret, err := otp.GenerateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	f.secret = secret
	enc, err := crypto.EncryptAES256GCM(secret, key)
	if err != nil {
		t.Fatal(err)
	}
	_ = repo.CreateMFASecret(context.Background(), &domain.MFASecret{
		ID: uuid.New(), UserID: f.userID, TenantID: f.tenantID, SecretEncrypted: enc, IsVerified: true,
	})
	hash, _ := bcrypt.GenerateFromPassword([]byte(f.backup), bcrypt.MinCost)
	_ = repo.SaveBackupCodes(context.Background(), []*domain.MFABackupCode{
		{ID: uuid.New(), UserID: f.userID, TenantID: f.tenantID, CodeHash: string(hash)},
	})

	user := &domain.User{ID: f.userID, Email: "awa.diallo@example.test", FullName: "Awa Diallo", Locale: "en"}
	f.uc = NewChallengeMFAUseCase(repo, key).
		WithAttemptLimits(f.store, f.revoker).
		WithLockNotice(fakeLockUsers{user: user}, f.mailer)
	return f
}

func (f *challengeFixture) try(jti, code string) error {
	_, err := f.uc.Execute(context.Background(), ChallengeMFAInput{
		UserID: f.userID, TenantID: f.tenantID, Code: code,
		ChallengeJTI: jti, ChallengeExpiresAt: time.Now().Add(5 * time.Minute),
	})
	return err
}

func (f *challengeFixture) goodCode(t *testing.T) string {
	t.Helper()
	code, err := totp.GenerateCode(f.secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return code
}

// badCode is a six-digit code the authenticator would not accept right now.
func (f *challengeFixture) badCode(t *testing.T, seed int) string {
	t.Helper()
	for i := seed; ; i++ {
		c := fmt.Sprintf("%06d", (i*7919+13)%1000000)
		if !otp.VerifyTOTP(f.secret, c) {
			return c
		}
	}
}

func isInvalidCode(err error) bool {
	var appErr *domain.AppError
	return errors.As(err, &appErr) && errors.Is(appErr.Err, domain.ErrValidation)
}

func TestMFAChallenge_Success(t *testing.T) {
	f := newChallengeFixture(t)
	if err := f.try("jti-1", f.goodCode(t)); err != nil {
		t.Fatalf("a correct code must pass: %v", err)
	}
}

func TestMFAChallenge_NotFound(t *testing.T) {
	f := newChallengeFixture(t)
	_, err := f.uc.Execute(context.Background(), ChallengeMFAInput{
		UserID: uuid.New(), TenantID: f.tenantID, Code: "123456", ChallengeJTI: "jti-x",
	})
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || !errors.Is(appErr.Err, domain.ErrNotFound) {
		t.Fatalf("an account without a factor must be NotFound, got %v", err)
	}
}

// A locked account is refused even with the right code: the lock is checked
// before the code is.
func TestMFAChallenge_Unauthorized(t *testing.T) {
	f := newChallengeFixture(t)
	if _, err := f.store.Lock(context.Background(), userLockKey(f.userID), time.Minute); err != nil {
		t.Fatal(err)
	}
	var locked *MFAChallengeLockedError
	if err := f.try("jti-1", f.goodCode(t)); !errors.As(err, &locked) {
		t.Fatalf("a locked account must refuse a correct code, got %v", err)
	}
	if locked.JustLocked {
		t.Fatal("an existing lock must not read as newly set")
	}
}

func TestMFAChallenge_TokenDiesAfterFiveFailures(t *testing.T) {
	f := newChallengeFixture(t)
	for i := 1; i < MFAChallengeMaxAttemptsPerToken; i++ {
		if err := f.try("jti-1", f.badCode(t, i)); !isInvalidCode(err) {
			t.Fatalf("failure %d: want invalid code, got %v", i, err)
		}
	}
	if f.revoker.revoked("jti-1") {
		t.Fatal("the token must survive four failures")
	}

	if err := f.try("jti-1", f.badCode(t, 99)); !errors.Is(err, ErrMFAChallengeExhausted) {
		t.Fatalf("fifth failure: want ErrMFAChallengeExhausted, got %v", err)
	}
	if !f.revoker.revoked("jti-1") {
		t.Fatal("the fifth failure must blacklist the token")
	}

	// Even with the blacklist failing open, the counter refuses the token,
	// and it refuses it before looking at the code.
	if err := f.try("jti-1", f.goodCode(t)); !errors.Is(err, ErrMFAChallengeExhausted) {
		t.Fatalf("a spent token must refuse a correct code, got %v", err)
	}
	// A fresh login gets a fresh token.
	if err := f.try("jti-2", f.goodCode(t)); err != nil {
		t.Fatalf("a new token must work: %v", err)
	}
}

func TestMFAChallenge_UserBackoffAcrossTokens(t *testing.T) {
	f := newChallengeFixture(t)
	// Nine failures over three tokens: 4 + 4 + 1. No token is spent, the
	// account is one failure short of the lock.
	tokens := []string{"a", "a", "a", "a", "b", "b", "b", "b", "c"}
	for i, jti := range tokens {
		if err := f.try(jti, f.badCode(t, i)); !isInvalidCode(err) {
			t.Fatalf("failure %d: want invalid code, got %v", i+1, err)
		}
	}

	var locked *MFAChallengeLockedError
	if err := f.try("c", f.badCode(t, 50)); !errors.As(err, &locked) {
		t.Fatalf("the tenth failure must lock the account, got %v", err)
	}
	if !locked.JustLocked || locked.RetryAfter != MFAChallengeLockDuration {
		t.Fatalf("lock: JustLocked=%v RetryAfter=%v", locked.JustLocked, locked.RetryAfter)
	}

	// Another token, another address: still locked, with what remains.
	if err := f.try("d", f.goodCode(t)); !errors.As(err, &locked) {
		t.Fatalf("a correct code on a new token must be refused during the lock, got %v", err)
	}
	if locked.JustLocked || locked.RetryAfter <= 0 || locked.RetryAfter > MFAChallengeLockDuration+time.Second {
		t.Fatalf("refusal under lock: JustLocked=%v RetryAfter=%v", locked.JustLocked, locked.RetryAfter)
	}
}

func TestMFAChallenge_BackupCodesCounted(t *testing.T) {
	f := newChallengeFixture(t)
	// Wrong backup-shaped codes spend the token exactly like wrong TOTP codes.
	wrong := []string{"AAAAAAAA", "BBBBBBBB", "CCCCCCCC", "DDDDDDDD"}
	for _, c := range wrong {
		if err := f.try("jti-1", c); !isInvalidCode(err) {
			t.Fatalf("%s: want invalid code, got %v", c, err)
		}
	}
	if err := f.try("jti-1", "EEEEEEEE"); !errors.Is(err, ErrMFAChallengeExhausted) {
		t.Fatalf("fifth wrong backup code: want ErrMFAChallengeExhausted, got %v", err)
	}
	// The real backup code cannot rescue a spent token.
	if err := f.try("jti-1", f.backup); !errors.Is(err, ErrMFAChallengeExhausted) {
		t.Fatalf("a valid backup code on a spent token: got %v", err)
	}
	// And the five backup failures count towards the account too.
	if n, _ := f.store.Count(context.Background(), userFailuresKey(f.userID)); n < 5 {
		t.Fatalf("account failures: got %d, want at least 5", n)
	}
	// On a fresh token the real backup code still works.
	if err := f.try("jti-2", f.backup); err != nil {
		t.Fatalf("a valid backup code on a fresh token: %v", err)
	}
}

func TestMFAChallenge_SuccessClearsFailureCount(t *testing.T) {
	f := newChallengeFixture(t)
	for i := range 4 {
		_ = f.try("a", f.badCode(t, i))
	}
	for i := range 4 {
		_ = f.try("b", f.badCode(t, 10+i))
	}
	if err := f.try("c", f.goodCode(t)); err != nil {
		t.Fatalf("correct code: %v", err)
	}
	// Eight earlier failures are forgiven: eight more do not lock.
	for i := range 4 {
		_ = f.try("d", f.badCode(t, 20+i))
	}
	for i := range 4 {
		if err := f.try("e", f.badCode(t, 30+i)); !isInvalidCode(err) {
			t.Fatalf("after a success the count restarts, got %v", err)
		}
	}
}

func TestMFAChallenge_LockSendsOneEmail(t *testing.T) {
	f := newChallengeFixture(t)
	for i := range MFAChallengeMaxFailuresPerUser {
		_ = f.try(fmt.Sprintf("t%d", i/4), f.badCode(t, i))
	}
	for i := range 20 {
		_ = f.try(fmt.Sprintf("x%d", i), f.goodCode(t))
	}
	if got := f.mailer.count(); got != 1 {
		t.Fatalf("one lock must send one email, got %d", got)
	}
	if f.mailer.sent[0] != "awa.diallo@example.test|en" {
		t.Fatalf("email sent to %q", f.mailer.sent[0])
	}
}

type brokenStore struct{ *authmfa.MemoryAttemptStore }

func (brokenStore) LockedFor(context.Context, string) (time.Duration, error) {
	return 0, errors.New("store down")
}

// If the limiter cannot answer, the challenge must not go ahead unchecked.
func TestMFAChallenge_StoreErrorFailsClosed(t *testing.T) {
	f := newChallengeFixture(t)
	f.uc.WithAttemptLimits(brokenStore{authmfa.NewMemoryAttemptStore()}, f.revoker)
	if err := f.try("jti-1", f.goodCode(t)); err == nil {
		t.Fatal("a store error must refuse the attempt")
	}
}
