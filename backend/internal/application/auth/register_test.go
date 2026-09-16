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
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
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
