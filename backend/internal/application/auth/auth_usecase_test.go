// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockUserRepository is a mock for the UserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) GetByEmail(email string) (*domain.User, error) {
	args := m.Called(email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByID(id uuid.UUID) (*domain.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) Create(user *domain.User) (*domain.User, error) {
	args := m.Called(user)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

// MockOrganizationRepository is a mock for the OrganizationRepository
type MockOrganizationRepository struct {
	mock.Mock
}

func (m *MockOrganizationRepository) Create(org *domain.Organization) (*domain.Organization, error) {
	args := m.Called(org)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Organization), args.Error(1)
}

func (m *MockOrganizationRepository) GetByID(id uuid.UUID) (*domain.Organization, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Organization), args.Error(1)
}

// MockPasswordHasher is a mock for PasswordHasher
type MockPasswordHasher struct {
	mock.Mock
}

func (m *MockPasswordHasher) Hash(password string) (string, error) {
	args := m.Called(password)
	return args.String(0), args.Error(1)
}

func (m *MockPasswordHasher) Verify(hashedPassword, plainPassword string) bool {
	args := m.Called(hashedPassword, plainPassword)
	return args.Bool(0)
}

// MockNotificationService is a mock for NotificationService
type MockNotificationService struct {
	mock.Mock
}

func (m *MockNotificationService) SendWelcomeEmail(user *domain.User) error {
	args := m.Called(user)
	return args.Error(0)
}

// Test LoginUseCase_Success
func TestLoginUseCase_Success(t *testing.T) {
	userID := uuid.New()
	testUser := &domain.User{
		ID:       userID,
		Email:    "test@example.com",
		Password: "hashedpassword123",
		IsActive: true,
	}

	mockUserRepo := new(MockUserRepository)
	mockUserRepo.On("GetByEmail", "test@example.com").Return(testUser, nil)

	mockHasher := new(MockPasswordHasher)
	mockHasher.On("Verify", "hashedpassword123", "password123").Return(true)

	mockNotif := new(MockNotificationService)

	useCase := NewLoginUseCase(mockUserRepo, mockHasher, mockNotif)

	result, err := useCase.Execute(nil, LoginInput{
		Email:    "test@example.com",
		Password: "password123",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result.User)
	assert.Equal(t, testUser.Email, result.User.Email)

	mockUserRepo.AssertExpectations(t)
	mockHasher.AssertExpectations(t)
}

// Test LoginUseCase_UserNotFound
func TestLoginUseCase_UserNotFound(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockUserRepo.On("GetByEmail", "notfound@example.com").Return(nil, domain.ErrNotFound)

	mockHasher := new(MockPasswordHasher)
	mockNotif := new(MockNotificationService)

	useCase := NewLoginUseCase(mockUserRepo, mockHasher, mockNotif)

	result, err := useCase.Execute(nil, LoginInput{
		Email:    "notfound@example.com",
		Password: "password123",
	})

	assert.Error(t, err)
	assert.Nil(t, result)

	mockUserRepo.AssertExpectations(t)
}

// Test LoginUseCase_InvalidPassword
func TestLoginUseCase_InvalidPassword(t *testing.T) {
	testUser := &domain.User{
		ID:       uuid.New(),
		Email:    "test@example.com",
		Password: "hashedpassword123",
		IsActive: true,
	}

	mockUserRepo := new(MockUserRepository)
	mockUserRepo.On("GetByEmail", "test@example.com").Return(testUser, nil)

	mockHasher := new(MockPasswordHasher)
	mockHasher.On("Verify", "hashedpassword123", "wrongpassword").Return(false)

	mockNotif := new(MockNotificationService)

	useCase := NewLoginUseCase(mockUserRepo, mockHasher, mockNotif)

	result, err := useCase.Execute(nil, LoginInput{
		Email:    "test@example.com",
		Password: "wrongpassword",
	})

	assert.Error(t, err)
	assert.Nil(t, result)

	mockUserRepo.AssertExpectations(t)
	mockHasher.AssertExpectations(t)
}

// Test RegisterUseCase_Success
func TestRegisterUseCase_Success(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockUserRepo.On("GetByEmail", "newuser@example.com").Return(nil, domain.ErrNotFound)
	mockUserRepo.On("Create", mock.MatchedBy(func(u *domain.User) bool {
		return u.Email == "newuser@example.com"
	})).Return(&domain.User{
		ID:       uuid.New(),
		Email:    "newuser@example.com",
		Username: "newuser",
		FullName: "New User",
		IsActive: true,
	}, nil)

	mockOrgRepo := new(MockOrganizationRepository)
	mockOrgRepo.On("Create", mock.MatchedBy(func(o *domain.Organization) bool {
		return o.Name == "Test Company"
	})).Return(&domain.Organization{
		ID:   uuid.New(),
		Name: "Test Company",
	}, nil)

	mockHasher := new(MockPasswordHasher)
	mockHasher.On("Hash", "securepassword123").Return("hashedpassword123", nil)

	mockNotif := new(MockNotificationService)
	mockNotif.On("SendWelcomeEmail", mock.Anything).Return(nil)

	useCase := NewRegisterUseCase(mockUserRepo, mockOrgRepo, mockHasher, mockNotif)

	result, err := useCase.Execute(nil, RegisterInput{
		Email:       "newuser@example.com",
		Username:    "newuser",
		Password:    "securepassword123",
		FullName:    "New User",
		CompanyName: "Test Company",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result.User)
	assert.NotNil(t, result.Organization)
	assert.Equal(t, "newuser@example.com", result.User.Email)

	mockUserRepo.AssertExpectations(t)
	mockOrgRepo.AssertExpectations(t)
	mockHasher.AssertExpectations(t)
	mockNotif.AssertExpectations(t)
}

// Test RegisterUseCase_EmailAlreadyExists
func TestRegisterUseCase_EmailAlreadyExists(t *testing.T) {
	existingUser := &domain.User{
		ID:    uuid.New(),
		Email: "existing@example.com",
	}

	mockUserRepo := new(MockUserRepository)
	mockUserRepo.On("GetByEmail", "existing@example.com").Return(existingUser, nil)

	mockOrgRepo := new(MockOrganizationRepository)
	mockHasher := new(MockPasswordHasher)
	mockNotif := new(MockNotificationService)

	useCase := NewRegisterUseCase(mockUserRepo, mockOrgRepo, mockHasher, mockNotif)

	result, err := useCase.Execute(nil, RegisterInput{
		Email:       "existing@example.com",
		Username:    "newuser",
		Password:    "securepassword123",
		FullName:    "New User",
		CompanyName: "Test Company",
	})

	assert.Error(t, err)
	assert.Nil(t, result)

	mockUserRepo.AssertExpectations(t)
}

// Test RefreshTokenUseCase_Success
func TestRefreshTokenUseCase_Success(t *testing.T) {
	userID := uuid.New()
	testUser := &domain.User{
		ID:       userID,
		Email:    "test@example.com",
		IsActive: true,
	}

	mockTokenRepo := new(MockRefreshTokenRepository)
	mockTokenRepo.On("GetByToken", mock.Anything).Return(&domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}, nil)

	mockUserRepo := new(MockUserRepository)
	mockUserRepo.On("GetByID", userID).Return(testUser, nil)

	useCase := NewRefreshTokenUseCase(mockTokenRepo, mockUserRepo)

	result, err := useCase.Execute(nil, RefreshTokenInput{
		RefreshToken: "valid_token",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result.TokenPair)

	mockTokenRepo.AssertExpectations(t)
	mockUserRepo.AssertExpectations(t)
}

// Test RefreshTokenUseCase_InvalidToken
func TestRefreshTokenUseCase_InvalidToken(t *testing.T) {
	mockTokenRepo := new(MockRefreshTokenRepository)
	mockTokenRepo.On("GetByToken", "invalid_token").Return(nil, domain.ErrNotFound)

	mockUserRepo := new(MockUserRepository)

	useCase := NewRefreshTokenUseCase(mockTokenRepo, mockUserRepo)

	result, err := useCase.Execute(nil, RefreshTokenInput{
		RefreshToken: "invalid_token",
	})

	assert.Error(t, err)
	assert.Nil(t, result)

	mockTokenRepo.AssertExpectations(t)
}

// Test LogoutUseCase_Success
func TestLogoutUseCase_Success(t *testing.T) {
	mockTokenRepo := new(MockRefreshTokenRepository)
	mockTokenRepo.On("RevokeByToken", mock.Anything).Return(nil)

	useCase := NewLogoutUseCase(mockTokenRepo)

	err := useCase.Execute(nil, LogoutInput{
		RefreshToken: "valid_token",
	})

	assert.NoError(t, err)

	mockTokenRepo.AssertExpectations(t)
}

// Mock RefreshTokenRepository for testing
type MockRefreshTokenRepository struct {
	mock.Mock
}

func (m *MockRefreshTokenRepository) GetByToken(token string) (*domain.RefreshToken, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RefreshToken), args.Error(1)
}

func (m *MockRefreshTokenRepository) RevokeByToken(token string) error {
	args := m.Called(token)
	return args.Error(0)
}

func (m *MockRefreshTokenRepository) Create(rt *domain.RefreshToken) (*domain.RefreshToken, error) {
	args := m.Called(rt)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RefreshToken), args.Error(1)
}
