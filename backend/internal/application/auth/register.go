// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/notify"
)

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

// RegisterUseCase handles user registration
type RegisterUseCase struct {
	userRepo       UserRepository
	orgRepo        OrganizationRepository
	notifyService  notify.Service
	passwordHasher PasswordHasher
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

// Execute performs user registration
func (uc *RegisterUseCase) Execute(ctx context.Context, input RegisterInput) (*RegisterOutput, error) {
	// Validate input
	if err := uc.validateInput(input); err != nil {
		return nil, err
	}

	// Check if user already exists
	existingUser, err := uc.userRepo.GetByEmail(ctx, input.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existingUser != nil {
		return nil, domain.NewConflictError("user", "email")
	}

	// Check if username is taken
	existingUser, err = uc.userRepo.GetByUsername(ctx, input.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing username: %w", err)
	}
	if existingUser != nil {
		return nil, domain.NewConflictError("user", "username")
	}

	// Hash password
	hashedPassword, err := uc.passwordHasher.Hash(input.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create organization first
	org := &domain.Organization{
		Name:      input.CompanyName,
		Slug:      uc.generateSlug(input.CompanyName),
		OwnerID:   uuid.New(), // Will be updated after user creation
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := uc.orgRepo.Create(ctx, org); err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	// Create user
	user := &domain.User{
		Email:        input.Email,
		Username:     input.Username,
		Password:     hashedPassword,
		FullName:     input.FullName,
		DefaultOrgID: &org.ID,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := uc.userRepo.Create(ctx, user); err != nil {
		// Clean up organization if user creation fails
		if delErr := uc.orgRepo.Delete(ctx, org.ID); delErr != nil {
			fmt.Printf("Warning: failed to roll back organization %s after user creation failure: %v\n", org.ID, delErr)
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Update organization owner
	org.OwnerID = user.ID
	org.Owner = user
	if err := uc.orgRepo.Update(ctx, org); err != nil {
		// This is not critical, log and continue
		fmt.Printf("Warning: failed to update organization owner: %v\n", err)
	}

	// Create organization membership
	member := &domain.OrganizationMember{
		OrganizationID: org.ID,
		UserID:         user.ID,
		Role:           domain.RoleRoot,
		IsActive:       true,
		JoinedAt:       time.Now(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := uc.userRepo.CreateOrganizationMember(ctx, member); err != nil {
		// This is not critical for registration success, log and continue
		fmt.Printf("Warning: failed to create organization membership: %v\n", err)
	}

	// Send welcome email
	if uc.notifyService != nil {
		go func() {
			err := uc.notifyService.SendWelcomeEmail(ctx, user.Email, user.FullName)
			if err != nil {
				fmt.Printf("Warning: failed to send welcome email: %v\n", err)
			}
		}()
	}

	return &RegisterOutput{
		User:         user,
		Organization: org,
		Message:      "Registration successful. Please check your email for confirmation.",
	}, nil
}

func (uc *RegisterUseCase) validateInput(input RegisterInput) error {
	if input.Email == "" {
		return domain.NewValidationError("email is required")
	}
	if input.Username == "" {
		return domain.NewValidationError("username is required")
	}
	if input.Password == "" {
		return domain.NewValidationError("password is required")
	}
	if len(input.Password) < 8 {
		return domain.NewValidationError("password must be at least 8 characters")
	}
	if input.FullName == "" {
		return domain.NewValidationError("full name is required")
	}
	if input.CompanyName == "" {
		return domain.NewValidationError("company name is required")
	}
	return nil
}

func (uc *RegisterUseCase) generateSlug(companyName string) string {
	// Simple slug generation - in production, use a proper slug library
	slug := strings.ToLower(strings.ReplaceAll(companyName, " ", "-"))
	// Ensure uniqueness by checking database
	counter := 0
	originalSlug := slug
	for {
		exists, err := uc.orgRepo.SlugExists(context.Background(), slug)
		if err != nil {
			// If check fails, append counter
			counter++
			slug = fmt.Sprintf("%s-%d", originalSlug, counter)
			continue
		}
		if !exists {
			break
		}
		counter++
		slug = fmt.Sprintf("%s-%d", originalSlug, counter)
	}
	return slug
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
