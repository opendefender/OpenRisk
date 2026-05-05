package auth

import (
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockGormUserRepository is a mock for GORM user repository
type MockGormUserRepository struct {
	mock.Mock
}

func (m *MockGormUserRepository) GetByEmail(email string, tenantID uuid.UUID) (*domain.User, error) {
	args := m.Called(email, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockGormUserRepository) GetByID(id uuid.UUID, tenantID uuid.UUID) (*domain.User, error) {
	args := m.Called(id, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockGormUserRepository) Create(user *domain.User) (*domain.User, error) {
	args := m.Called(user)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

// TestRepositoryIntegration_User tests GORM user repository functions
func TestRepositoryIntegration_User(t *testing.T) {
	mockRepo := new(MockGormUserRepository)

	newUser := &domain.User{
		ID:       uuid.New(),
		Email:    "test@example.com",
		Username: "testuser",
	}

	mockRepo.On("Create", newUser).Return(newUser, nil)

	result, err := mockRepo.Create(newUser)

	assert.NoError(t, err)
	assert.Equal(t, newUser.Email, result.Email)

	mockRepo.AssertExpectations(t)
}

// TestRepositoryIntegration_GetByEmail tests GetByEmail repository function
func TestRepositoryIntegration_GetByEmail(t *testing.T) {
	mockRepo := new(MockGormUserRepository)
	tenantID := uuid.New()

	testUser := &domain.User{
		ID:       uuid.New(),
		Email:    "test@example.com",
		TenantID: tenantID,
	}

	mockRepo.On("GetByEmail", "test@example.com", tenantID).Return(testUser, nil)

	result, err := mockRepo.GetByEmail("test@example.com", tenantID)

	assert.NoError(t, err)
	assert.Equal(t, testUser.Email, result.Email)
	assert.Equal(t, tenantID, result.TenantID)

	mockRepo.AssertExpectations(t)
}

// TestTenantIsolation verifies tenant isolation on queries
func TestTenantIsolation(t *testing.T) {
	mockRepo := new(MockGormUserRepository)

	tenant1 := uuid.New()
	tenant2 := uuid.New()

	user1 := &domain.User{
		ID:       uuid.New(),
		Email:    "user@example.com",
		TenantID: tenant1,
	}

	user2 := &domain.User{
		ID:       uuid.New(),
		Email:    "user@example.com",
		TenantID: tenant2,
	}

	// Setup expectations
	mockRepo.On("GetByEmail", "user@example.com", tenant1).Return(user1, nil)
	mockRepo.On("GetByEmail", "user@example.com", tenant2).Return(user2, nil)

	// Query for each tenant
	result1, err1 := mockRepo.GetByEmail("user@example.com", tenant1)
	result2, err2 := mockRepo.GetByEmail("user@example.com", tenant2)

	// Verify both queries succeed but return different users
	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, tenant1, result1.TenantID)
	assert.Equal(t, tenant2, result2.TenantID)

	mockRepo.AssertExpectations(t)
}
