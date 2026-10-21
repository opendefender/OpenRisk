// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Registration (#687). The use case had no tests at all, and four defects:
// people sharing an email local part were refused, a case-variant address made a
// second account for one mailbox, the three rows were written without a
// transaction, and the slug search retried a failing lookup for ever.

package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/authmfa"
)

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

type registerUsers struct {
	byEmail    map[string]*domain.User
	byUsername map[string]*domain.User
	emailErr   error
}

func (r *registerUsers) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	if r.emailErr != nil {
		return nil, r.emailErr
	}
	return r.byEmail[domain.NormaliseEmail(email)], nil
}

func (r *registerUsers) GetByUsername(_ context.Context, username string) (*domain.User, error) {
	return r.byUsername[username], nil
}

func (r *registerUsers) GetUserDefaultOrganization(context.Context, uuid.UUID) (*domain.Organization, error) {
	return nil, nil
}

func (r *registerUsers) GetOrganizationMember(context.Context, uuid.UUID, uuid.UUID) (*domain.OrganizationMember, error) {
	return nil, nil
}
func (r *registerUsers) Create(context.Context, *domain.User) error { return nil }
func (r *registerUsers) Update(context.Context, *domain.User) error { return nil }
func (r *registerUsers) CreateOrganizationMember(context.Context, *domain.OrganizationMember) error {
	return nil
}

type registerOrgs struct {
	taken   map[string]bool
	slugErr error
	calls   int
}

func (o *registerOrgs) Create(context.Context, *domain.Organization) error { return nil }
func (o *registerOrgs) Update(context.Context, *domain.Organization) error { return nil }
func (o *registerOrgs) Delete(context.Context, uuid.UUID) error            { return nil }
func (o *registerOrgs) SlugExists(_ context.Context, slug string) (bool, error) {
	o.calls++
	if o.slugErr != nil {
		return false, o.slugErr
	}
	return o.taken[slug], nil
}

// registerAccounts records the ONE transactional write registration may make.
type registerAccounts struct {
	calls  int
	org    *domain.Organization
	user   *domain.User
	member *domain.OrganizationMember
	err    error
}

func (a *registerAccounts) CreateAccount(_ context.Context, org *domain.Organization, user *domain.User, member *domain.OrganizationMember) error {
	a.calls++
	if a.err != nil {
		return a.err
	}
	a.org, a.user, a.member = org, user, member
	return nil
}

type registerHasher struct{}

func (registerHasher) Hash(password string) (string, error) { return "hashed:" + password, nil }
func (registerHasher) Verify(hashed, plain string) bool     { return hashed == "hashed:"+plain }

func newRegisterHarness() (*RegisterUseCase, *registerUsers, *registerOrgs, *registerAccounts) {
	users := &registerUsers{byEmail: map[string]*domain.User{}, byUsername: map[string]*domain.User{}}
	orgs := &registerOrgs{taken: map[string]bool{}}
	accounts := &registerAccounts{}
	uc := NewRegisterUseCase(users, orgs, nil, registerHasher{}).WithAccounts(accounts)
	return uc, users, orgs, accounts
}

func validInput() RegisterInput {
	return RegisterInput{
		Email:       "karim@alpha.test",
		Password:    "Ancre-Vitrail7-Cobalt",
		FullName:    "Karim Alpha",
		CompanyName: "Alpha SARL",
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestRegister_Success(t *testing.T) {
	uc, _, _, accounts := newRegisterHarness()

	out, err := uc.Execute(context.Background(), validInput())
	require.NoError(t, err)

	// One write, not three.
	assert.Equal(t, 1, accounts.calls)
	require.NotNil(t, accounts.org)
	require.NotNil(t, accounts.user)
	require.NotNil(t, accounts.member)

	// The organization carries its real owner from the first insert, and the
	// owner is a root member of it.
	assert.Equal(t, accounts.user.ID, accounts.org.OwnerID)
	assert.Equal(t, accounts.org.ID, accounts.member.OrganizationID)
	assert.Equal(t, accounts.user.ID, accounts.member.UserID)
	assert.Equal(t, domain.RoleRoot, accounts.member.Role)
	assert.True(t, accounts.member.IsActive)

	assert.Equal(t, "karim@alpha.test", out.User.Email)
	assert.Equal(t, "karim", out.User.Username) // derived from the local part
	assert.Equal(t, "hashed:Ancre-Vitrail7-Cobalt", out.User.Password)
}

func TestRegister_NormalisesTheAddress(t *testing.T) {
	uc, _, _, accounts := newRegisterHarness()

	in := validInput()
	in.Email = "  Karim@Alpha.TEST "
	_, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)

	assert.Equal(t, "karim@alpha.test", accounts.user.Email)
}

func TestRegister_DuplicateEmailCaseInsensitive(t *testing.T) {
	uc, users, _, accounts := newRegisterHarness()
	users.byEmail["karim@alpha.test"] = &domain.User{ID: uuid.New(), Email: "karim@alpha.test"}

	in := validInput()
	in.Email = "Karim@Alpha.TEST"
	_, err := uc.Execute(context.Background(), in)

	// A second account for one mailbox used to be created here.
	require.ErrorIs(t, err, ErrEmailAlreadyRegistered)
	assert.Equal(t, 0, accounts.calls)
}

func TestRegister_SameLocalPartDifferentDomain(t *testing.T) {
	uc, users, _, accounts := newRegisterHarness()
	// Someone else already goes by "karim", from karim@alpha.test.
	users.byUsername["karim"] = &domain.User{ID: uuid.New(), Username: "karim"}

	in := validInput()
	in.Email = "karim@beta.test" // a different person, at a different company
	out, err := uc.Execute(context.Background(), in)

	// This used to be refused as "user already exists", which the sign-up screen
	// showed as "this email is already registered".
	require.NoError(t, err)
	assert.Equal(t, 1, accounts.calls)
	assert.NotEqual(t, "karim", out.User.Username)
	assert.True(t, strings.HasPrefix(out.User.Username, "karim"), out.User.Username)
}

func TestRegister_UsernameTakenIsNotEmailConflict(t *testing.T) {
	uc, users, _, accounts := newRegisterHarness()
	users.byUsername["chosen.name"] = &domain.User{ID: uuid.New(), Username: "chosen.name"}

	in := validInput()
	in.Username = "chosen.name" // a client that picks its own username
	_, err := uc.Execute(context.Background(), in)

	require.ErrorIs(t, err, ErrUsernameTaken)
	assert.NotErrorIs(t, err, ErrEmailAlreadyRegistered)
	assert.Equal(t, 0, accounts.calls)
}

func TestRegister_FailedAccountWriteLeavesNothingBehind(t *testing.T) {
	uc, _, _, accounts := newRegisterHarness()
	accounts.err = errors.New("insert failed")

	out, err := uc.Execute(context.Background(), validInput())

	require.Error(t, err)
	assert.Nil(t, out)
	// The transaction is the repository's; the use case must not paper over it
	// with best-effort cleanup, as it used to.
	assert.Equal(t, 1, accounts.calls)
}

func TestRegister_SlugLookupFailureIsReturned(t *testing.T) {
	uc, _, orgs, accounts := newRegisterHarness()
	orgs.slugErr = errors.New("database unavailable")

	_, err := uc.Execute(context.Background(), validInput())

	// The old loop retried this for ever, hanging the request.
	require.Error(t, err)
	assert.Equal(t, 1, orgs.calls)
	assert.Equal(t, 0, accounts.calls)
}

func TestRegister_SlugCollisionTakesTheNextOne(t *testing.T) {
	uc, _, orgs, accounts := newRegisterHarness()
	orgs.taken["alpha-sarl"] = true

	_, err := uc.Execute(context.Background(), validInput())
	require.NoError(t, err)

	assert.Equal(t, "alpha-sarl-2", accounts.org.Slug)
}

func TestRegister_RefusesWithoutAnAccountWriter(t *testing.T) {
	users := &registerUsers{byEmail: map[string]*domain.User{}, byUsername: map[string]*domain.User{}}
	orgs := &registerOrgs{taken: map[string]bool{}}
	uc := NewRegisterUseCase(users, orgs, nil, registerHasher{}) // no WithAccounts

	_, err := uc.Execute(context.Background(), validInput())

	// Better a refusal than the three unprotected writes this replaces.
	require.Error(t, err)
}

func TestRegister_ValidationFailures(t *testing.T) {
	uc, _, _, accounts := newRegisterHarness()

	cases := map[string]func(*RegisterInput){
		"no email":      func(in *RegisterInput) { in.Email = "" },
		"weak password": func(in *RegisterInput) { in.Password = "short" },
		"no full name":  func(in *RegisterInput) { in.FullName = "" },
		"no company":    func(in *RegisterInput) { in.CompanyName = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := validInput()
			mutate(&in)
			_, err := uc.Execute(context.Background(), in)
			require.Error(t, err)
		})
	}
	assert.Equal(t, 0, accounts.calls)
}

// ---------------------------------------------------------------------------
// A sign-up for an address that already has an account (#686)
// ---------------------------------------------------------------------------

type sentDuplicateAlert struct{ to, name, signIn, reset, locale string }

type fakeDuplicateMailer struct {
	sent []sentDuplicateAlert
	err  error
}

func (m *fakeDuplicateMailer) SendDuplicateSignup(_ context.Context, to, fullName, signInURL, resetURL, locale string) error {
	m.sent = append(m.sent, sentDuplicateAlert{to, fullName, signInURL, resetURL, locale})
	return m.err
}

func newDuplicateHarness(t *testing.T) (*RegisterUseCase, *domain.User, *fakeDuplicateMailer, *authmfa.MemoryAttemptStore) {
	t.Helper()
	uc, users, _, _ := newRegisterHarness()
	owner := &domain.User{ID: uuid.New(), Email: "karim@alpha.test", FullName: "Karim Alpha"}
	users.byEmail[owner.Email] = owner
	store := authmfa.NewMemoryAttemptStore()
	mailer := &fakeDuplicateMailer{}
	uc.WithDuplicateGuard(store, mailer, "https://risk.alpha.test/")
	return uc, owner, mailer, store
}

func TestRegister_DuplicateEmail(t *testing.T) {
	uc, owner, mailer, _ := newDuplicateHarness(t)

	in := validInput()
	in.Locale = "en"
	out, err := uc.Execute(context.Background(), in)

	require.ErrorIs(t, err, ErrEmailAlreadyRegistered)
	assert.Nil(t, out)
	// The owner is told, with both ways back in, and nothing about the
	// requester: not their name, not the password they typed.
	require.Len(t, mailer.sent, 1)
	got := mailer.sent[0]
	assert.Equal(t, owner.Email, got.to)
	assert.Equal(t, owner.FullName, got.name)
	assert.Equal(t, "https://risk.alpha.test/login", got.signIn)
	assert.Equal(t, "https://risk.alpha.test/forgot-password", got.reset)
	assert.Equal(t, "en", got.locale)
	for _, field := range []string{got.signIn, got.reset} {
		assert.NotContains(t, field, in.Password)
		assert.NotContains(t, field, owner.Email, "the address never goes in a URL")
	}
}

func TestRegister_DuplicateAlertUsesTheOwnersLanguage(t *testing.T) {
	uc, owner, mailer, _ := newDuplicateHarness(t)
	owner.Locale = "fr"

	in := validInput()
	in.Locale = "en"
	_, err := uc.Execute(context.Background(), in)

	require.ErrorIs(t, err, ErrEmailAlreadyRegistered)
	require.Len(t, mailer.sent, 1)
	assert.Equal(t, "fr", mailer.sent[0].locale)
}

func TestRegister_DuplicateAlertThrottled(t *testing.T) {
	uc, _, mailer, store := newDuplicateHarness(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		in := validInput()
		if i == 1 {
			in.Email = "  KARIM@alpha.test" // another spelling of the same mailbox
		}
		_, err := uc.Execute(ctx, in)
		require.ErrorIs(t, err, ErrEmailAlreadyRegistered, "attempt %d", i+1)
	}

	// Three attempts, one message: a stranger retrying must not turn the
	// owner's inbox into the attack.
	assert.Len(t, mailer.sent, 1)
	left, err := store.LockedFor(ctx, registerAlertKey("karim@alpha.test"))
	require.NoError(t, err)
	assert.InDelta(t, RegisterDuplicateAlertInterval.Seconds(), left.Seconds(), 2)
}

func TestRegister_DuplicateAlertFailureKeepsTheAnswer(t *testing.T) {
	uc, _, mailer, _ := newDuplicateHarness(t)
	mailer.err = errors.New("smtp down")

	_, err := uc.Execute(context.Background(), validInput())

	require.ErrorIs(t, err, ErrEmailAlreadyRegistered)
	assert.Len(t, mailer.sent, 1)
}

func TestRegister_RateLimited(t *testing.T) {
	uc, _, mailer, _ := newDuplicateHarness(t)
	ctx := context.Background()

	// The attempt that reaches the limit is still evaluated.
	for i := 1; i <= RegisterMaxDuplicatesPerAddress; i++ {
		_, err := uc.Execute(ctx, validInput())
		require.ErrorIs(t, err, ErrEmailAlreadyRegistered, "attempt %d", i)
	}

	in := validInput()
	in.Email = "Karim@Alpha.test"
	_, err := uc.Execute(ctx, in)

	var limited *RegisterRateLimitedError
	require.ErrorAs(t, err, &limited)
	assert.InDelta(t, RegisterDuplicateLockDuration.Seconds(), limited.RetryAfter.Seconds(), 2)
	assert.Len(t, mailer.sent, 1)

	// Another address is not affected, and a new account there still goes
	// through.
	other := validInput()
	other.Email = "awa@beta.test"
	_, err = uc.Execute(ctx, other)
	require.NoError(t, err)
}

func TestRegister_UsernameTakenNeitherCountsNorAlerts(t *testing.T) {
	uc, _, mailer, _ := newDuplicateHarness(t)
	uc.userRepo.(*registerUsers).byUsername["chosen.name"] = &domain.User{ID: uuid.New(), Username: "chosen.name"}

	in := validInput()
	in.Email = "new@gamma.test"
	in.Username = "chosen.name"
	_, err := uc.Execute(context.Background(), in)

	require.ErrorIs(t, err, ErrUsernameTaken)
	assert.NotErrorIs(t, err, ErrEmailAlreadyRegistered)
	assert.Empty(t, mailer.sent)
}

func TestRegister_DuplicateAlertConcurrentMailsOnce(t *testing.T) {
	uc, _, _, _ := newDuplicateHarness(t)
	mailer := &lockedDuplicateMailer{}
	uc.duplicates.mailer = mailer

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = uc.Execute(context.Background(), validInput())
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, mailer.count())
}

func TestRegister_DuplicateStoreErrorStillConflicts(t *testing.T) {
	uc, _, mailer, _ := newDuplicateHarness(t)
	uc.duplicates.store = failingAttemptStore{}

	_, err := uc.Execute(context.Background(), validInput())

	// A broken store is a bug, not a reason to refuse sign-ups or to mail the
	// owner on every attempt.
	require.ErrorIs(t, err, ErrEmailAlreadyRegistered)
	assert.Empty(t, mailer.sent)
}

type lockedDuplicateMailer struct {
	mu sync.Mutex
	n  int
}

func (m *lockedDuplicateMailer) SendDuplicateSignup(context.Context, string, string, string, string, string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.n++
	return nil
}

func (m *lockedDuplicateMailer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.n
}

type failingAttemptStore struct{}

var errStoreDown = errors.New("store down")

func (failingAttemptStore) Increment(context.Context, string, time.Duration) (int64, error) {
	return 0, errStoreDown
}
func (failingAttemptStore) Lock(context.Context, string, time.Duration) (bool, error) {
	return false, errStoreDown
}
func (failingAttemptStore) LockedFor(context.Context, string) (time.Duration, error) {
	return 0, errStoreDown
}
func (failingAttemptStore) Delete(context.Context, string) error { return errStoreDown }

// #686 review: a taken username used to be checked after the address, so it
// answered differently for a registered and a free address and let any caller
// test addresses without creating an account or meeting the per-address limit.
func TestRegister_TakenUsernameDoesNotRevealTheAddress(t *testing.T) {
	uc, owner, mailer, store := newDuplicateHarness(t)
	uc.userRepo.(*registerUsers).byUsername["chosen.name"] = &domain.User{ID: uuid.New(), Username: "chosen.name"}
	ctx := context.Background()

	for _, email := range []string{owner.Email, "nobody@gamma.test"} {
		in := validInput()
		in.Email = email
		in.Username = "chosen.name"
		_, err := uc.Execute(ctx, in)

		require.ErrorIs(t, err, ErrUsernameTaken, email)
		assert.NotErrorIs(t, err, ErrEmailAlreadyRegistered, email)
	}

	// The registered address was never reached: no alert, nothing counted.
	assert.Empty(t, mailer.sent)
	n, err := store.Increment(ctx, registerDupKey(owner.Email), RegisterDuplicateWindow)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}
