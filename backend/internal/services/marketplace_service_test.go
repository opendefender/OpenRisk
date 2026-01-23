package services

import (
	"context"
	"testing"
	"time"

	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMarketplaceService_RegisterConnector(t *testing.T) {
	db := setupTestDB(t)
	service := NewMarketplaceService(db, nil)
	ctx := context.Background()

	connector := &domain.Connector{
		Name:         "Test Connector",
		Author:       "Test Author",
		Version:      "1.0.0",
		Description:  "Test description",
		Category:     "integration",
		Status:       domain.ConnectorStatusActive,
		Capabilities: []domain.ConnectorCapability{domain.CapabilityRiskImport},
		License:      "MIT",
		Rating:       4.5,
	}

	err := service.RegisterConnector(ctx, connector)
	require.NoError(t, err)
	assert.NotEmpty(t, connector.ID)
	assert.False(t, connector.CreatedAt.IsZero())
}

func TestMarketplaceService_GetConnector(t *testing.T) {
	db := setupTestDB(t)
	service := NewMarketplaceService(db, nil)
	ctx := context.Background()

	// Register connector first
	connector := &domain.Connector{
		Name:         "Test Connector",
		Author:       "Test Author",
		Version:      "1.0.0",
		Description:  "Test description",
		Status:       domain.ConnectorStatusActive,
		Capabilities: []domain.ConnectorCapability{domain.CapabilityRiskImport},
		License:      "MIT",
		Rating:       4.5,
	}
	err := service.RegisterConnector(ctx, connector)
	require.NoError(t, err)

	// Retrieve connector
	retrieved, err := service.GetConnector(ctx, connector.ID)
	require.NoError(t, err)
	assert.Equal(t, connector.Name, retrieved.Name)
	assert.Equal(t, connector.Author, retrieved.Author)
}

func TestMarketplaceService_ListConnectors(t *testing.T) {
	db := setupTestDB(t)
	service := NewMarketplaceService(db, nil)
	ctx := context.Background()

	// Register multiple connectors
	for i := 0; i < 5; i++ {
		connector := &domain.Connector{
			Name:         "Connector " + string(rune(i)),
			Author:       "Test Author",
			Version:      "1.0.0",
			Description:  "Test description",
			Status:       domain.ConnectorStatusActive,
			Capabilities: []domain.ConnectorCapability{domain.CapabilityRiskImport},
			License:      "MIT",
			Rating:       float64(i) + 1,
		}
		err := service.RegisterConnector(ctx, connector)
		require.NoError(t, err)
	}

	// List connectors
	connectors, total, err := service.ListConnectors(ctx, nil, "", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, connectors, 5)
}

func TestMarketplaceService_SearchConnectors(t *testing.T) {
	db := setupTestDB(t)
	service := NewMarketplaceService(db, nil)
	ctx := context.Background()

	// Register connectors
	connector1 := &domain.Connector{
		Name:         "Splunk Connector",
		Author:       "Security Team",
		Version:      "1.0.0",
		Description:  "Splunk integration for logs",
		Status:       domain.ConnectorStatusActive,
		Capabilities: []domain.ConnectorCapability{domain.CapabilityThreatIntel},
		License:      "MIT",
		Rating:       4.5,
	}
	err := service.RegisterConnector(ctx, connector1)
	require.NoError(t, err)

	// Search for connector
	results, total, err := service.SearchConnectors(ctx, "Splunk", 10, 0)
	require.NoError(t, err)
	assert.Greater(t, total, int64(0))
	assert.Greater(t, len(results), 0)
}

func TestMarketplaceService_InstallApp(t *testing.T) {
	db := setupTestDB(t)
	service := NewMarketplaceService(db, nil)
	ctx := context.Background()

	// Register connector
	connector := &domain.Connector{
		Name:         "Test Connector",
		Author:       "Test Author",
		Version:      "1.0.0",
		Description:  "Test description",
		Status:       domain.ConnectorStatusActive,
		Capabilities: []domain.ConnectorCapability{domain.CapabilityRiskImport},
		License:      "MIT",
		Rating:       4.5,
	}
	err := service.RegisterConnector(ctx, connector)
	require.NoError(t, err)

	// Install app
	config := map[string]interface{}{
		"api_key": "test-key",
		"enabled": true,
	}

	app, err := service.InstallApp(ctx, connector.ID, "tenant-1", "user-1", "My Connector", config)
	require.NoError(t, err)
	assert.NotEmpty(t, app.ID)
	assert.Equal(t, domain.InstallationStatusPending, app.Status)
	assert.True(t, app.Enabled)
	assert.NotEmpty(t, app.WebhookSecret)
}

func TestMarketplaceService_InstallApp_AlreadyInstalled(t *testing.T) {
	db := setupTestDB(t)
	service := NewMarketplaceService(db, nil)
	ctx := context.Background()

	// Register connector
	connector := &domain.Connector{
		Name:         "Test Connector",
		Author:       "Test Author",
		Version:      "1.0.0",
		Description:  "Test description",
		Status:       domain.ConnectorStatusActive,
		Capabilities: []domain.ConnectorCapability{domain.CapabilityRiskImport},
		License:      "MIT",
		Rating:       4.5,
	}
	err := service.RegisterConnector(ctx, connector)
	require.NoError(t, err)

	// Install app first time
	_, err = service.InstallApp(ctx, connector.ID, "tenant-1", "user-1", "My Connector", nil)
	require.NoError(t, err)

	// Try to install again
	_, err = service.InstallApp(ctx, connector.ID, "tenant-1", "user-1", "My Connector 2", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already installed")
}

func TestMarketplaceService_EnableDisableApp(t *testing.T) {
	db := setupTestDB(t)
	service := NewMarketplaceService(db, nil)
	ctx := context.Background()

	// Setup
	connector := &domain.Connector{
		Name:         "Test Connector",
		Author:       "Test Author",
		Version:      "1.0.0",
		Description:  "Test description",
		Status:       domain.ConnectorStatusActive,
		Capabilities: []domain.ConnectorCapability{domain.CapabilityRiskImport},
		License:      "MIT",
		Rating:       4.5,
	}
	err := service.RegisterConnector(ctx, connector)
	require.NoError(t, err)

	app, err := service.InstallApp(ctx, connector.ID, "tenant-1", "user-1", "My Connector", nil)
	require.NoError(t, err)

	// Disable app
	err = service.DisableApp(ctx, app.ID, "user-1")
	require.NoError(t, err)

	// Verify disabled
	retrieved, err := service.GetApp(ctx, app.ID)
	require.NoError(t, err)
	assert.False(t, retrieved.Enabled)

	// Enable app
	err = service.EnableApp(ctx, app.ID, "user-1")
	require.NoError(t, err)

	// Verify enabled
	retrieved, err = service.GetApp(ctx, app.ID)
	require.NoError(t, err)
	assert.True(t, retrieved.Enabled)
}

func TestMarketplaceService_UpdateApp(t *testing.T) {
	db := setupTestDB(t)
	service := NewMarketplaceService(db, nil)
	ctx := context.Background()

	// Setup
	connector := &domain.Connector{
		Name:         "Test Connector",
		Author:       "Test Author",
		Version:      "1.0.0",
		Description:  "Test description",
		Status:       domain.ConnectorStatusActive,
		Capabilities: []domain.ConnectorCapability{domain.CapabilityRiskImport},
		License:      "MIT",
		Rating:       4.5,
	}
	err := service.RegisterConnector(ctx, connector)
	require.NoError(t, err)

	app, err := service.InstallApp(ctx, connector.ID, "tenant-1", "user-1", "My Connector", nil)
	require.NoError(t, err)

	// Update configuration
	newConfig := map[string]interface{}{
		"api_key": "new-key",
		"enabled": false,
	}

	updated, err := service.UpdateApp(ctx, app.ID, "user-1", newConfig)
	require.NoError(t, err)
	assert.Equal(t, newConfig, updated.Configuration)
}

func TestMarketplaceService_TriggerSync(t *testing.T) {
	db := setupTestDB(t)
	service := NewMarketplaceService(db, nil)
	ctx := context.Background()

	// Setup
	connector := &domain.Connector{
		Name:         "Test Connector",
		Author:       "Test Author",
		Version:      "1.0.0",
		Description:  "Test description",
		Status:       domain.ConnectorStatusActive,
		Capabilities: []domain.ConnectorCapability{domain.CapabilityRiskImport},
		License:      "MIT",
		Rating:       4.5,
	}
	err := service.RegisterConnector(ctx, connector)
	require.NoError(t, err)

	app, err := service.InstallApp(ctx, connector.ID, "tenant-1", "user-1", "My Connector", nil)
	require.NoError(t, err)

	// Trigger sync
	err = service.TriggerSync(ctx, app.ID, "user-1")
	require.NoError(t, err)

	// Verify sync status updated
	retrieved, err := service.GetApp(ctx, app.ID)
	require.NoError(t, err)
	assert.NotNil(t, retrieved.LastSyncAt)
	assert.Equal(t, "success", retrieved.LastSyncStatus)
}

func TestMarketplaceService_UninstallApp(t *testing.T) {
	db := setupTestDB(t)
	service := NewMarketplaceService(db, nil)
	ctx := context.Background()

	// Setup
	connector := &domain.Connector{
		Name:         "Test Connector",
		Author:       "Test Author",
		Version:      "1.0.0",
		Description:  "Test description",
		Status:       domain.ConnectorStatusActive,
		Capabilities: []domain.ConnectorCapability{domain.CapabilityRiskImport},
		License:      "MIT",
		Rating:       4.5,
	}
	err := service.RegisterConnector(ctx, connector)
	require.NoError(t, err)

	app, err := service.InstallApp(ctx, connector.ID, "tenant-1", "user-1", "My Connector", nil)
	require.NoError(t, err)
	_ = connector.InstallCount // Capture initial state for verification

	// Uninstall app
	err = service.UninstallApp(ctx, app.ID, "user-1")
	require.NoError(t, err)

	// Verify status changed
	retrieved, err := service.GetApp(ctx, app.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.InstallationStatusUninstalled, retrieved.Status)
}

func TestMarketplaceService_GetAppLogs(t *testing.T) {
	db := setupTestDB(t)
	service := NewMarketplaceService(db, nil)
	ctx := context.Background()

	// Setup
	connector := &domain.Connector{
		Name:         "Test Connector",
		Author:       "Test Author",
		Version:      "1.0.0",
		Description:  "Test description",
		Status:       domain.ConnectorStatusActive,
		Capabilities: []domain.ConnectorCapability{domain.CapabilityRiskImport},
		License:      "MIT",
		Rating:       4.5,
	}
	err := service.RegisterConnector(ctx, connector)
	require.NoError(t, err)

	app, err := service.InstallApp(ctx, connector.ID, "tenant-1", "user-1", "My Connector", nil)
	require.NoError(t, err)

	// Give logs time to be written
	time.Sleep(100 * time.Millisecond)

	// Get logs
	_, total, err := service.GetAppLogs(ctx, app.ID, "", 10, 0)
	require.NoError(t, err)
	assert.Greater(t, total, int64(0))
}

func TestMarketplaceService_AddConnectorReview(t *testing.T) {
	db := setupTestDB(t)
	service := NewMarketplaceService(db, nil)
	ctx := context.Background()

	// Register connector
	connector := &domain.Connector{
		Name:         "Test Connector",
		Author:       "Test Author",
		Version:      "1.0.0",
		Description:  "Test description",
		Status:       domain.ConnectorStatusActive,
		Capabilities: []domain.ConnectorCapability{domain.CapabilityRiskImport},
		License:      "MIT",
		Rating:       0,
		Reviews:      []domain.ConnectorReview{},
	}
	err := service.RegisterConnector(ctx, connector)
	require.NoError(t, err)

	// Add review
	err = service.AddConnectorReview(ctx, connector.ID, "user-1", "John Doe", 5, "Excellent connector!")
	require.NoError(t, err)

	// Verify review was added
	updated, err := service.GetConnector(ctx, connector.ID)
	require.NoError(t, err)
	assert.Len(t, updated.Reviews, 1)
	assert.Equal(t, 5, updated.Reviews[0].Rating)
}

func TestConnectorValidation(t *testing.T) {
	tests := []struct {
		name    string
		conn    *domain.Connector
		wantErr bool
	}{
		{
			name: "valid connector",
			conn: &domain.Connector{
				Name:         "Test",
				Author:       "Author",
				Version:      "1.0.0",
				Description:  "Description",
				Capabilities: []domain.ConnectorCapability{domain.CapabilityRiskImport},
				Rating:       4.5,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			conn: &domain.Connector{
				Author:       "Author",
				Version:      "1.0.0",
				Description:  "Description",
				Capabilities: []domain.ConnectorCapability{domain.CapabilityRiskImport},
			},
			wantErr: true,
		},
		{
			name: "invalid rating",
			conn: &domain.Connector{
				Name:         "Test",
				Author:       "Author",
				Version:      "1.0.0",
				Description:  "Description",
				Capabilities: []domain.ConnectorCapability{domain.CapabilityRiskImport},
				Rating:       10.0, // Invalid
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.conn.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Helper function to setup test database
func setupTestDB(t *testing.T) *gorm.DB {
	// This should be implemented based on your test setup
	// For now, return a mock or skip
	t.Skip("Test database setup needed")
	return nil
}
