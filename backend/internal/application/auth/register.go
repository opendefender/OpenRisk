// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/notify"
)

const (
	// How long the welcome email may take once the request has been answered.
	welcomeEmailTimeout = 30 * time.Second
	// Bounded searches: a free username, and a free organization slug. The slug
	// search used to be unbounded and retried a FAILING lookup for ever, so a
	// database blip hung the request and spun a core.
	usernameAttempts = 20
	slugAttempts     = 20
)

// The two conflicts registration can answer with (#687).
//
// They are separate errors because they are separate problems for the person at
// the screen: "you already have an account" is something they can act on, while
// "a stranger took that username" is not the same statement. Reporting the
// second as the first told new users something false and left them nowhere.
var (
	ErrEmailAlreadyRegistered = errors.New("an account already exists for this email address")
	ErrUsernameTaken          = errors.New("username is already taken")
)

// AccountCreator writes the organization, its owner and the owner's membership
// in ONE transaction (ABSOLUTE RULE #7). Satisfied by
// repository.GormRegistrationRepository.
type AccountCreator interface {
	CreateAccount(ctx context.Context, org *domain.Organization, user *domain.User, member *domain.OrganizationMember) error
}

// RegisterInput represents the input for user registration
type RegisterInput struct {
	Email       string
	Username    string
	Password    string
	FullName    string
	CompanyName string
}

// RegisterOutput represents the output of successful registration
type RegisterOutput struct {
	User         *domain.User
	Organization *domain.Organization
	Message      string
}

// ActivationRecorder anchors t0 for the time-to-Aha metric. Narrow port,
// satisfied structurally by application/activation.Recorder; nil-safe.
//
// RecordOnce rather than Record: signup happens exactly once per tenant, and a
// duplicate row here would corrupt every subsequent time-to-Aha measurement.
type ActivationRecorder interface {
	RecordOnce(ctx context.Context, tenantID uuid.UUID, key string, payload map[string]interface{}) bool
}

// RegisterUseCase handles user registration
type RegisterUseCase struct {
	userRepo       UserRepository
	orgRepo        OrganizationRepository
	notifyService  notify.Service
	passwordHasher PasswordHasher
	activation     ActivationRecorder
	accounts       AccountCreator
}

// WithActivation attaches the optional activation recorder.
func (uc *RegisterUseCase) WithActivation(rec ActivationRecorder) *RegisterUseCase {
	uc.activation = rec
	return uc
}

// WithAccounts attaches the transactional writer. Registration refuses to run
// without it rather than fall back to the three independent writes that used to
// leave half-made accounts behind.
func (uc *RegisterUseCase) WithAccounts(a AccountCreator) *RegisterUseCase {
	uc.accounts = a
	return uc
}

// NewRegisterUseCase creates a new register use case
func NewRegisterUseCase(
	userRepo UserRepository,
	orgRepo OrganizationRepository,
	notifyService notify.Service,
	passwordHasher PasswordHasher,
) *RegisterUseCase {
	return &RegisterUseCase{
		userRepo:       userRepo,
		orgRepo:        orgRepo,
		notifyService:  notifyService,
		passwordHasher: passwordHasher,
	}
}

// Execute performs user registration.
//
// The organization, its owner and the owner's root membership are written in one
// transaction (#687). They used to be three independent writes: a failed user
// insert deleted the organization best-effort, and a failed membership insert
// was printed and ignored — leaving an account that could sign in but belonged
// to nothing, with no permissions and no way to repair itself.
func (uc *RegisterUseCase) Execute(ctx context.Context, input RegisterInput) (*RegisterOutput, error) {
	if err := uc.validateInput(input); err != nil {
		return nil, err
	}
	if uc.accounts == nil {
		return nil, domain.NewInternalError("registration is not configured: no account writer")
	}

	// One spelling of an address, everywhere. Password reset already normalised;
	// registration did not, so Alex@Example.com and alex@example.com could each
	// hold an account for one mailbox, and which one a reset or a login reached
	// depended on the casing typed that day.
	email := domain.NormaliseEmail(input.Email)

	existingUser, err := uc.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existingUser != nil {
		return nil, ErrEmailAlreadyRegistered
	}

	username, err := uc.resolveUsername(ctx, input.Username, email)
	if err != nil {
		return nil, err
	}

	hashedPassword, err := uc.passwordHasher.Hash(input.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	slug, err := uc.generateSlug(ctx, input.CompanyName)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	// The user's id is assigned here so the organization carries its real owner
	// from the first insert, rather than a placeholder corrected by a later
	// update whose failure used to be printed and ignored.
	userID := uuid.New()
	org := &domain.Organization{
		ID:        uuid.New(),
		Name:      input.CompanyName,
		Slug:      slug,
		OwnerID:   userID,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	user := &domain.User{
		ID:           userID,
		Email:        email,
		Username:     username,
		Password:     hashedPassword,
		FullName:     input.FullName,
		DefaultOrgID: &org.ID,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	member := &domain.OrganizationMember{
		OrganizationID: org.ID,
		UserID:         user.ID,
		Role:           domain.RoleRoot,
		IsActive:       true,
		JoinedAt:       now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := uc.accounts.CreateAccount(ctx, org, user, member); err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	// Anchor t0 for time-to-Aha. Recorded here rather than at first login so the
	// clock starts when the account is created — which is when the newcomer's
	// eight minutes actually start.
	if uc.activation != nil {
		uc.activation.RecordOnce(ctx, org.ID, string(domain.ActivationSignup), map[string]interface{}{
			"user_id": user.ID.String(),
		})
	}

	// The welcome email outlives the request. It used to run on the request's own
	// context, which the handler cancels the moment it answers, so the send could
	// be aborted halfway. The address is never logged; the user id is.
	if uc.notifyService != nil {
		mailCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), welcomeEmailTimeout)
		recipient, name, id := user.Email, user.FullName, user.ID
		go func() {
			defer cancel()
			if err := uc.notifyService.SendWelcomeEmail(mailCtx, recipient, name); err != nil {
				log.Printf("register: welcome email failed for user %s: %v", id, err)
			}
		}()
	}

	return &RegisterOutput{
		User:         user,
		Organization: org,
		Message:      "Registration successful. Please check your email for confirmation.",
	}, nil
}

// resolveUsername returns a username that is free.
//
// A username the CLIENT chose is refused when taken: that caller can choose
// again. A username nobody chose — the sign-up screen sends none (#687) — is
// derived from the address and made unique here, because the person never saw
// the field and cannot act on a collision with a stranger.
func (uc *RegisterUseCase) resolveUsername(ctx context.Context, requested, email string) (string, error) {
	if chosen := strings.TrimSpace(requested); chosen != "" {
		taken, err := uc.usernameTaken(ctx, chosen)
		if err != nil {
			return "", err
		}
		if taken {
			return "", ErrUsernameTaken
		}
		return chosen, nil
	}

	base := usernameFromEmail(email)
	candidate := base
	for attempt := 1; attempt <= usernameAttempts; attempt++ {
		taken, err := uc.usernameTaken(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s%d", base, attempt+1)
	}
	// Still colliding after a bounded search: take a random suffix rather than
	// search for ever.
	return fmt.Sprintf("%s-%s", base, uuid.NewString()[:8]), nil
}

func (uc *RegisterUseCase) usernameTaken(ctx context.Context, username string) (bool, error) {
	existing, err := uc.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return false, fmt.Errorf("failed to check existing username: %w", err)
	}
	return existing != nil, nil
}

// usernameFromEmail keeps the address's local part, minus anything a username
// may not carry, and pads a very short one so it clears the 3-character floor.
func usernameFromEmail(email string) string {
	local, _, _ := strings.Cut(email, "@")
	var b strings.Builder
	for _, r := range strings.ToLower(local) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '.', r == '-':
			b.WriteRune(r)
		}
	}
	name := strings.Trim(b.String(), ".-")
	if len(name) < 3 {
		name = "user" + name
	}
	return name
}

func (uc *RegisterUseCase) validateInput(input RegisterInput) error {
	if input.Email == "" {
		return domain.NewValidationError("email is required")
	}
	// No username check: the field is optional (#687) and resolveUsername
	// derives a free one when the client sends none.
	// Single source of truth for the policy (domain.ValidatePassword) so the
	// rules cannot drift between entry points — they already had, with the code
	// enforcing 8 characters while the README promised 12 (audit finding F-05).
	if err := domain.ValidatePassword(input.Password); err != nil {
		return err
	}
	if input.FullName == "" {
		return domain.NewValidationError("full name is required")
	}
	if input.CompanyName == "" {
		return domain.NewValidationError("company name is required")
	}
	return nil
}

// generateSlug finds a free slug for the organization, on the request's own
// context and in a bounded number of tries.
//
// The previous loop had neither: a failing lookup was retried for ever, so a
// database blip turned one registration into a hung request spinning a core, and
// it ran on context.Background() where a cancelled request could not stop it.
func (uc *RegisterUseCase) generateSlug(ctx context.Context, companyName string) (string, error) {
	base := strings.ToLower(strings.TrimSpace(companyName))
	base = strings.ReplaceAll(base, " ", "-")
	if base == "" {
		base = "organisation"
	}

	slug := base
	for attempt := 1; attempt <= slugAttempts; attempt++ {
		exists, err := uc.orgRepo.SlugExists(ctx, slug)
		if err != nil {
			return "", fmt.Errorf("failed to check organization slug: %w", err)
		}
		if !exists {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, attempt+1)
	}
	return fmt.Sprintf("%s-%s", base, uuid.NewString()[:8]), nil
}

// OrganizationRepository interface for organization operations
type OrganizationRepository interface {
	Create(ctx context.Context, org *domain.Organization) error
	Update(ctx context.Context, org *domain.Organization) error
	Delete(ctx context.Context, id uuid.UUID) error
	SlugExists(ctx context.Context, slug string) (bool, error)
}

// PasswordHasher interface for password hashing
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(hashedPassword, plainPassword string) bool
}
