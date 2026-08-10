// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/pwpolicy"
)

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type fakeResetUsers struct {
	byEmail map[string]*domain.User
	byID    map[uuid.UUID]*domain.User
	updated []*domain.User
	getErr  error
}

func newFakeResetUsers(users ...*domain.User) *fakeResetUsers {
	f := &fakeResetUsers{byEmail: map[string]*domain.User{}, byID: map[uuid.UUID]*domain.User{}}
	for _, u := range users {
		f.byEmail[domain.NormaliseEmail(u.Email)] = u
		f.byID[u.ID] = u
	}
	return f
}

func (f *fakeResetUsers) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	u, ok := f.byEmail[domain.NormaliseEmail(email)]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (f *fakeResetUsers) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (f *fakeResetUsers) Update(_ context.Context, u *domain.User) error {
	f.updated = append(f.updated, u)
	return nil
}

// fakeResetTokens is an in-memory PasswordResetRepository with a real
// conditional claim, so the single-use race is genuinely exercised.
type fakeResetTokens struct {
	mu   sync.Mutex
	rows []*domain.PasswordResetToken
}

func (f *fakeResetTokens) Create(_ context.Context, tok *domain.PasswordResetToken) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if tok.ID == uuid.Nil {
		tok.ID = uuid.New()
	}
	if tok.CreatedAt.IsZero() {
		tok.CreatedAt = time.Now()
	}
	f.rows = append(f.rows, tok)
	return nil
}

func (f *fakeResetTokens) FindByTokenHash(_ context.Context, hash string) (*domain.PasswordResetToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.TokenHash != nil && *r.TokenHash == hash {
			return r, nil
		}
	}
	return nil, nil
}

func (f *fakeResetTokens) ClaimToken(_ context.Context, id uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.ID == id && r.UsedAt == nil {
			now := time.Now()
			r.UsedAt = &now
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeResetTokens) CountRecentByEmailHash(_ context.Context, emailHash string, since time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, r := range f.rows {
		if r.EmailHash == emailHash && !r.CreatedAt.Before(since) {
			n++
		}
	}
	return n, nil
}

func (f *fakeResetTokens) InvalidateOutstandingForUser(_ context.Context, userID, except uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	for _, r := range f.rows {
		if r.UserID != nil && *r.UserID == userID && r.ID != except && r.UsedAt == nil {
			r.UsedAt = &now
		}
	}
	return nil
}

type sentMail struct {
	kind, to, link, locale string
}

type fakeMailer struct {
	mu   sync.Mutex
	sent []sentMail
}

func (m *fakeMailer) SendResetLink(_ context.Context, to, _, link, locale string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, sentMail{"link", to, link, locale})
	return nil
}

func (m *fakeMailer) SendResetConfirmation(_ context.Context, to, _, locale string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, sentMail{"confirm", to, "", locale})
	return nil
}

func (m *fakeMailer) count(kind string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, s := range m.sent {
		if s.kind == kind {
			n++
		}
	}
	return n
}

type fakeRevoker struct {
	revoked []uuid.UUID
	err     error
}

func (r *fakeRevoker) RevokeAllUserTokens(_ context.Context, userID uuid.UUID) error {
	if r.err != nil {
		return r.err
	}
	r.revoked = append(r.revoked, userID)
	return nil
}

type fakeHasher struct{}

func (fakeHasher) Hash(p string) (string, error)  { return "hashed:" + p, nil }
func (fakeHasher) Verify(hash, plain string) bool { return hash == "hashed:"+plain }

func activeUser(email string) *domain.User {
	return &domain.User{
		ID:       uuid.New(),
		Email:    email,
		Username: strings.Split(email, "@")[0],
		FullName: "Test Person",
		IsActive: true,
	}
}

// ---------------------------------------------------------------------------
// Request — the anti-enumeration contract
// ---------------------------------------------------------------------------

func TestRequestReset_KnownAndUnknownAddressesAreIndistinguishable(t *testing.T) {
	// The core property: for any address, existing or not, the caller gets the
	// same output and the same error. The ONLY difference is an email landing in
	// a mailbox the requester must already control.
	user := activeUser("real@opendefender.io")
	tokens := &fakeResetTokens{}
	mailer := &fakeMailer{}
	uc := NewRequestPasswordResetUseCase(newFakeResetUsers(user), tokens, mailer)

	known, errKnown := uc.Execute(context.Background(), RequestPasswordResetInput{
		Email: "real@opendefender.io", BaseURL: "https://app.test",
	})
	unknown, errUnknown := uc.Execute(context.Background(), RequestPasswordResetInput{
		Email: "ghost@opendefender.io", BaseURL: "https://app.test",
	})

	if errKnown != nil || errUnknown != nil {
		t.Fatalf("both branches must succeed: known=%v unknown=%v", errKnown, errUnknown)
	}
	if *known != *unknown {
		t.Errorf("outputs must be identical, got known=%+v unknown=%+v", known, unknown)
	}

	// Both attempts must be RECORDED, or the rate limiter becomes the oracle the
	// uniform response was written to close.
	if got := len(tokens.rows); got != 2 {
		t.Fatalf("expected both attempts recorded, got %d rows", got)
	}
	// Only the real account gets a usable token.
	var withToken, withoutToken int
	for _, r := range tokens.rows {
		if r.TokenHash == nil {
			withoutToken++
		} else {
			withToken++
		}
	}
	if withToken != 1 || withoutToken != 1 {
		t.Errorf("expected exactly one usable token, got %d usable / %d recorded-only", withToken, withoutToken)
	}
	if mailer.count("link") != 1 {
		t.Errorf("expected exactly one email, got %d", mailer.count("link"))
	}
}

func TestRequestReset_RateLimitCountsUnknownAddressesToo(t *testing.T) {
	// If the cap only counted real accounts, the fourth request would 429 for an
	// address that exists and succeed forever for one that does not — handing an
	// attacker the exact bit the uniform response hides.
	tokens := &fakeResetTokens{}
	uc := NewRequestPasswordResetUseCase(newFakeResetUsers(), tokens, &fakeMailer{})

	for i := 0; i < domain.PasswordResetMaxPerHour; i++ {
		if _, err := uc.Execute(context.Background(), RequestPasswordResetInput{Email: "ghost@opendefender.io"}); err != nil {
			t.Fatalf("request %d should succeed: %v", i+1, err)
		}
	}

	_, err := uc.Execute(context.Background(), RequestPasswordResetInput{Email: "ghost@opendefender.io"})
	if !errors.Is(err, ErrResetRateLimited) {
		t.Fatalf("expected the cap to apply to an address with no account, got %v", err)
	}
}

func TestRequestReset_RateLimitIsPerAddress(t *testing.T) {
	tokens := &fakeResetTokens{}
	uc := NewRequestPasswordResetUseCase(newFakeResetUsers(), tokens, &fakeMailer{})

	for i := 0; i < domain.PasswordResetMaxPerHour; i++ {
		_, _ = uc.Execute(context.Background(), RequestPasswordResetInput{Email: "a@opendefender.io"})
	}

	// A different address is unaffected.
	if _, err := uc.Execute(context.Background(), RequestPasswordResetInput{Email: "b@opendefender.io"}); err != nil {
		t.Fatalf("a different address must have its own budget, got %v", err)
	}
}

func TestRequestReset_CaseAndWhitespaceCannotBypassTheCap(t *testing.T) {
	// Without normalisation, "A@x.io", "a@x.io" and " a@x.io " are three budgets.
	tokens := &fakeResetTokens{}
	uc := NewRequestPasswordResetUseCase(newFakeResetUsers(), tokens, &fakeMailer{})

	for _, variant := range []string{"user@opendefender.io", "User@OpenDefender.io", "  USER@opendefender.io  "} {
		_, _ = uc.Execute(context.Background(), RequestPasswordResetInput{Email: variant})
	}

	_, err := uc.Execute(context.Background(), RequestPasswordResetInput{Email: "user@opendefender.io"})
	if !errors.Is(err, ErrResetRateLimited) {
		t.Fatalf("case/whitespace variants must share one budget, got %v", err)
	}
}

func TestRequestReset_DisabledAccountGetsNoTokenButLooksIdentical(t *testing.T) {
	// Bouncing a disabled account differently would advertise that the address is
	// known. Record the attempt, send nothing, answer the same.
	disabled := activeUser("disabled@opendefender.io")
	disabled.IsActive = false
	tokens := &fakeResetTokens{}
	mailer := &fakeMailer{}
	uc := NewRequestPasswordResetUseCase(newFakeResetUsers(disabled), tokens, mailer)

	out, err := uc.Execute(context.Background(), RequestPasswordResetInput{Email: disabled.Email})
	if err != nil {
		t.Fatalf("expected the uniform success answer, got %v", err)
	}
	if out == nil {
		t.Fatal("expected an output")
	}
	if len(tokens.rows) != 1 || tokens.rows[0].TokenHash != nil {
		t.Error("a disabled account must record the attempt without a usable token")
	}
	if mailer.count("link") != 0 {
		t.Error("a disabled account must not receive a reset link")
	}
}

func TestRequestReset_LookupFailureStillAnswersUniformly(t *testing.T) {
	// A storage error on the "does this account exist" lookup must not become a
	// 500 for real addresses and a 200 for unknown ones — the error rate would be
	// the oracle.
	users := newFakeResetUsers()
	users.getErr = errors.New("database is down")
	uc := NewRequestPasswordResetUseCase(users, &fakeResetTokens{}, &fakeMailer{})

	if _, err := uc.Execute(context.Background(), RequestPasswordResetInput{Email: "someone@opendefender.io"}); err != nil {
		t.Fatalf("expected the uniform answer despite the lookup failure, got %v", err)
	}
}

func TestRequestReset_LinkPointsAtTheSPAAndCarriesTheSecret(t *testing.T) {
	user := activeUser("real@opendefender.io")
	tokens := &fakeResetTokens{}
	mailer := &fakeMailer{}
	uc := NewRequestPasswordResetUseCase(newFakeResetUsers(user), tokens, mailer)

	_, err := uc.Execute(context.Background(), RequestPasswordResetInput{
		Email: user.Email, BaseURL: "https://app.test/", Locale: "en",
	})
	if err != nil {
		t.Fatal(err)
	}

	if mailer.sent[0].locale != "en" {
		t.Errorf("expected the requester's locale carried into the email, got %q", mailer.sent[0].locale)
	}
	link := mailer.sent[0].link
	if !strings.HasPrefix(link, "https://app.test/reset-password?token=") {
		t.Fatalf("unexpected link %q", link)
	}
	// The stored form must be the HASH; a database leak must not yield live links.
	secret := strings.TrimPrefix(link, "https://app.test/reset-password?token=")
	if tokens.rows[0].TokenHash == nil || *tokens.rows[0].TokenHash == secret {
		t.Error("the reset secret must be stored hashed, never in the clear")
	}
	if *tokens.rows[0].TokenHash != domain.HashResetToken(secret) {
		t.Error("stored hash does not match the emailed secret")
	}
}

// ---------------------------------------------------------------------------
// Confirm
// ---------------------------------------------------------------------------

// issueToken drives the request use case and returns the emailed secret.
func issueToken(t *testing.T, users *fakeResetUsers, tokens *fakeResetTokens, email string) string {
	t.Helper()
	mailer := &fakeMailer{}
	uc := NewRequestPasswordResetUseCase(users, tokens, mailer)
	if _, err := uc.Execute(context.Background(), RequestPasswordResetInput{Email: email, BaseURL: "https://app.test"}); err != nil {
		t.Fatal(err)
	}
	link := mailer.sent[0].link
	return strings.TrimPrefix(link, "https://app.test/reset-password?token=")
}

func newConfirmUC(users *fakeResetUsers, tokens *fakeResetTokens, revoker *fakeRevoker, mailer *fakeMailer) *ConfirmPasswordResetUseCase {
	return NewConfirmPasswordResetUseCase(users, tokens, fakeHasher{}, pwpolicy.New(), revoker, mailer)
}

func TestConfirmReset_SetsPasswordAndEndsEverySession(t *testing.T) {
	user := activeUser("real@opendefender.io")
	users, tokens := newFakeResetUsers(user), &fakeResetTokens{}
	secret := issueToken(t, users, tokens, user.Email)

	revoker, mailer := &fakeRevoker{}, &fakeMailer{}
	out, err := newConfirmUC(users, tokens, revoker, mailer).Execute(context.Background(), ConfirmPasswordResetInput{
		Token: secret, NewPassword: "Ancre-Vitrail7-Cobalt",
	})
	if err != nil {
		t.Fatalf("expected the reset to succeed, got %v", err)
	}

	if !out.SessionsRevoked {
		t.Error("expected sessions to be reported revoked")
	}
	if len(revoker.revoked) != 1 || revoker.revoked[0] != user.ID {
		t.Errorf("expected every session for %s revoked, got %v", user.ID, revoker.revoked)
	}
	if len(users.updated) != 1 || users.updated[0].Password != "hashed:Ancre-Vitrail7-Cobalt" {
		t.Error("expected the new password persisted, hashed")
	}
	if mailer.count("confirm") != 1 {
		t.Error("expected a confirmation notice — it is how a victim learns of a takeover")
	}
}

func TestConfirmReset_TokenIsSingleUse(t *testing.T) {
	user := activeUser("real@opendefender.io")
	users, tokens := newFakeResetUsers(user), &fakeResetTokens{}
	secret := issueToken(t, users, tokens, user.Email)
	uc := newConfirmUC(users, tokens, &fakeRevoker{}, &fakeMailer{})

	if _, err := uc.Execute(context.Background(), ConfirmPasswordResetInput{Token: secret, NewPassword: "Ancre-Vitrail7-Cobalt"}); err != nil {
		t.Fatalf("first use should succeed: %v", err)
	}

	_, err := uc.Execute(context.Background(), ConfirmPasswordResetInput{Token: secret, NewPassword: "Autre-Passage9-Menthe"})
	if !errors.Is(err, ErrInvalidResetToken) {
		t.Fatalf("expected the second use to be refused, got %v", err)
	}
}

func TestConfirmReset_ConcurrentUseElectsExactlyOneWinner(t *testing.T) {
	// The claim is a conditional update, not read-then-write. Two requests
	// carrying the same link race here; one must win and one must be refused.
	user := activeUser("real@opendefender.io")
	users, tokens := newFakeResetUsers(user), &fakeResetTokens{}
	secret := issueToken(t, users, tokens, user.Email)
	uc := newConfirmUC(users, tokens, &fakeRevoker{}, &fakeMailer{})

	const racers = 8
	var wg sync.WaitGroup
	results := make([]error, racers)
	wg.Add(racers)
	for i := 0; i < racers; i++ {
		go func(i int) {
			defer wg.Done()
			_, results[i] = uc.Execute(context.Background(), ConfirmPasswordResetInput{
				Token: secret, NewPassword: "Ancre-Vitrail7-Cobalt",
			})
		}(i)
	}
	wg.Wait()

	wins := 0
	for _, err := range results {
		if err == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("expected exactly one winner, got %d", wins)
	}
}

func TestConfirmReset_ExpiredTokenIsRefused(t *testing.T) {
	user := activeUser("real@opendefender.io")
	users, tokens := newFakeResetUsers(user), &fakeResetTokens{}
	secret := issueToken(t, users, tokens, user.Email)

	// Wind the window shut.
	tokens.rows[0].ExpiresAt = time.Now().Add(-time.Minute)

	_, err := newConfirmUC(users, tokens, &fakeRevoker{}, &fakeMailer{}).Execute(context.Background(),
		ConfirmPasswordResetInput{Token: secret, NewPassword: "Ancre-Vitrail7-Cobalt"})
	if !errors.Is(err, ErrInvalidResetToken) {
		t.Fatalf("expected an expired token to be refused, got %v", err)
	}
}

func TestConfirmReset_TTLIsThirtyMinutes(t *testing.T) {
	user := activeUser("real@opendefender.io")
	users, tokens := newFakeResetUsers(user), &fakeResetTokens{}
	_ = issueToken(t, users, tokens, user.Email)

	window := time.Until(tokens.rows[0].ExpiresAt)
	if window > domain.PasswordResetTTL || window < domain.PasswordResetTTL-time.Minute {
		t.Errorf("expected a ~30 minute window, got %s", window)
	}
	if domain.PasswordResetTTL != 30*time.Minute {
		t.Errorf("the decided TTL is 30 minutes, got %s", domain.PasswordResetTTL)
	}
}

func TestConfirmReset_UnknownTokenIsRefusedWithTheSameError(t *testing.T) {
	// Unknown, expired and already-used must be indistinguishable: saying a token
	// "expired" confirms it once existed, and with it the account.
	users, tokens := newFakeResetUsers(), &fakeResetTokens{}

	_, err := newConfirmUC(users, tokens, &fakeRevoker{}, &fakeMailer{}).Execute(context.Background(),
		ConfirmPasswordResetInput{Token: "not-a-real-token", NewPassword: "Ancre-Vitrail7-Cobalt"})
	if !errors.Is(err, ErrInvalidResetToken) {
		t.Fatalf("expected the shared invalid-token error, got %v", err)
	}
}

func TestConfirmReset_WeakPasswordIsRefusedWithoutBurningTheLink(t *testing.T) {
	// A rejected password must not spend the token: otherwise one typo sends the
	// user back to their inbox, which is how people end up choosing worse
	// passwords.
	user := activeUser("real@opendefender.io")
	users, tokens := newFakeResetUsers(user), &fakeResetTokens{}
	secret := issueToken(t, users, tokens, user.Email)
	uc := newConfirmUC(users, tokens, &fakeRevoker{}, &fakeMailer{})

	out, err := uc.Execute(context.Background(), ConfirmPasswordResetInput{Token: secret, NewPassword: "short"})
	if err == nil {
		t.Fatal("expected a weak password to be refused")
	}
	if out == nil || out.Assessment == nil || len(out.Assessment.Blocking) == 0 {
		t.Fatal("expected the assessment attached so the UI can say what to fix")
	}
	if tokens.rows[0].UsedAt != nil {
		t.Error("a policy refusal must not spend the reset link")
	}

	// And the same link still works with an acceptable password.
	if _, err := uc.Execute(context.Background(), ConfirmPasswordResetInput{
		Token: secret, NewPassword: "Ancre-Vitrail7-Cobalt",
	}); err != nil {
		t.Fatalf("expected the link to survive the refusal, got %v", err)
	}
}

func TestConfirmReset_PolicyIsServerAuthoritative(t *testing.T) {
	// Whatever the browser decided, these are refused here.
	user := activeUser("real@opendefender.io")

	for _, weak := range []string{
		"Sh0rt!",              // under 12
		"alllowercaseletters", // one character class
		"Password1234!",       // decorated dictionary word
		"OpenRisk1234!",       // the product's own name
	} {
		users, tokens := newFakeResetUsers(user), &fakeResetTokens{}
		secret := issueToken(t, users, tokens, user.Email)
		uc := newConfirmUC(users, tokens, &fakeRevoker{}, &fakeMailer{})

		if _, err := uc.Execute(context.Background(), ConfirmPasswordResetInput{Token: secret, NewPassword: weak}); err == nil {
			t.Errorf("expected %q to be refused by the server", weak)
		}
	}
}

func TestConfirmReset_InvalidatesOtherOutstandingLinks(t *testing.T) {
	// If an attacker also requested a link, theirs must not outlive the
	// legitimate reset.
	user := activeUser("real@opendefender.io")
	users, tokens := newFakeResetUsers(user), &fakeResetTokens{}

	attackerSecret := issueToken(t, users, tokens, user.Email)
	ownerSecret := issueToken(t, users, tokens, user.Email)
	uc := newConfirmUC(users, tokens, &fakeRevoker{}, &fakeMailer{})

	if _, err := uc.Execute(context.Background(), ConfirmPasswordResetInput{
		Token: ownerSecret, NewPassword: "Ancre-Vitrail7-Cobalt",
	}); err != nil {
		t.Fatal(err)
	}

	_, err := uc.Execute(context.Background(), ConfirmPasswordResetInput{
		Token: attackerSecret, NewPassword: "Autre-Passage9-Menthe",
	})
	if !errors.Is(err, ErrInvalidResetToken) {
		t.Fatalf("expected other outstanding links to be dead, got %v", err)
	}
}

func TestConfirmReset_ReportsWhenSessionRevocationFailed(t *testing.T) {
	// The password is already changed at that point, so the caller must not be
	// told to retry the whole flow — but nor may we claim a clean sweep.
	user := activeUser("real@opendefender.io")
	users, tokens := newFakeResetUsers(user), &fakeResetTokens{}
	secret := issueToken(t, users, tokens, user.Email)

	revoker := &fakeRevoker{err: errors.New("redis unavailable")}
	out, err := newConfirmUC(users, tokens, revoker, &fakeMailer{}).Execute(context.Background(),
		ConfirmPasswordResetInput{Token: secret, NewPassword: "Ancre-Vitrail7-Cobalt"})

	if err == nil {
		t.Fatal("expected the revocation failure surfaced, not swallowed")
	}
	if out.SessionsRevoked {
		t.Error("must not claim sessions were revoked when they were not")
	}
	if len(users.updated) != 1 {
		t.Error("the password change itself should still have landed")
	}
}
