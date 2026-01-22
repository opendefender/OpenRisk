package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/core/domain"
	"gorm.io/gorm"
)

// MarketplaceService handles marketplace operations
type MarketplaceService struct {
	db            *gorm.DB
	mu            sync.RWMutex
	connectors    map[string]*domain.Connector
	installations map[string]*domain.MarketplaceApp
	syncWorkers   map[string]context.CancelFunc
	syncMu        sync.RWMutex
	logger        *log.Logger
}

// NewMarketplaceService creates a new MarketplaceService
func NewMarketplaceService(db *gorm.DB, logger *log.Logger) *MarketplaceService {
	if logger == nil {
		logger = log.New(nil, "", 0)
	}
	return &MarketplaceService{
		db:            db,
		connectors:    make(map[string]*domain.Connector),
		installations: make(map[string]*domain.MarketplaceApp),
		syncWorkers:   make(map[string]context.CancelFunc),
		logger:        logger,
	}
}

// RegisterConnector registers a new connector in the marketplace
func (m *MarketplaceService) RegisterConnector(ctx context.Context, connector *domain.Connector) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := connector.Validate(); err != nil {
		return err
	}

	if connector.ID == "" {
		connector.ID = uuid.New().String()
	}

	now := time.Now()
	connector.CreatedAt = now
	connector.UpdatedAt = now

	if err := m.db.WithContext(ctx).Create(connector).Error; err != nil {
		return fmt.Errorf("failed to register connector: %w", err)
	}

	m.connectors[connector.ID] = connector
	m.logger.Printf("Connector registered: %s v%s", connector.Name, connector.Version)

	return nil
}

// GetConnector retrieves a connector by ID
func (m *MarketplaceService) GetConnector(ctx context.Context, connectorID string) (*domain.Connector, error) {
	m.mu.RLock()
	if connector, exists := m.connectors[connectorID]; exists {
		m.mu.RUnlock()
		return connector, nil
	}
	m.mu.RUnlock()

	var connector domain.Connector
	if err := m.db.WithContext(ctx).First(&connector, "id = ?", connectorID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("connector not found")
		}
		return nil, fmt.Errorf("failed to retrieve connector: %w", err)
	}

	return &connector, nil
}

// ListConnectors lists all available connectors with filtering
func (m *MarketplaceService) ListConnectors(ctx context.Context, status *domain.ConnectorStatus, category string, limit int, offset int) ([]domain.Connector, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var connectors []domain.Connector
	query := m.db.WithContext(ctx)

	if status != nil {
		query = query.Where("status = ?", status.String())
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}

	var total int64
	if err := query.Model(&domain.Connector{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count connectors: %w", err)
	}

	if err := query.Order("rating DESC, install_count DESC").
		Limit(limit).
		Offset(offset).
		Find(&connectors).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list connectors: %w", err)
	}

	return connectors, total, nil
}

// SearchConnectors searches connectors by name, description, or author
func (m *MarketplaceService) SearchConnectors(ctx context.Context, query string, limit int, offset int) ([]domain.Connector, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var connectors []domain.Connector
	var total int64

	dbQuery := m.db.WithContext(ctx).Where(
		"LOWER(name) LIKE ? OR LOWER(description) LIKE ? OR LOWER(author) LIKE ?",
		"%"+query+"%", "%"+query+"%", "%"+query+"%",
	)

	if err := dbQuery.Model(&domain.Connector{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	if err := dbQuery.Order("rating DESC").Limit(limit).Offset(offset).Find(&connectors).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to search connectors: %w", err)
	}

	return connectors, total, nil
}

// InstallApp installs a connector as a marketplace app
func (m *MarketplaceService) InstallApp(ctx context.Context, connectorID, tenantID, userID, appName string, config map[string]interface{}) (*domain.MarketplaceApp, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	connector, err := m.GetConnector(ctx, connectorID)
	if err != nil {
		return nil, fmt.Errorf("connector not found: %w", err)
	}

	if connector.Status == domain.ConnectorStatusDeprecated {
		return nil, fmt.Errorf("cannot install deprecated connector")
	}

	// Check if already installed
	var existing domain.MarketplaceApp
	if err := m.db.WithContext(ctx).
		Where("connector_id = ? AND tenant_id = ?", connectorID, tenantID).
		First(&existing).Error; err == nil {
		return nil, fmt.Errorf("connector already installed for this tenant")
	}

	// Generate webhook secret
	webhookSecret, err := generateRandomSecret(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate webhook secret: %w", err)
	}

	app := &domain.MarketplaceApp{
		ID:               uuid.New().String(),
		ConnectorID:      connectorID,
		TenantID:         tenantID,
		UserID:           userID,
		Name:             appName,
		Description:      connector.Description,
		Version:          connector.Version,
		Status:           domain.InstallationStatusPending,
		Configuration:    config,
		Enabled:          true,
		AutoSync:         false,
		SyncInterval:     300, // 5 minutes
		WebhookSecret:    webhookSecret,
		InstallationDate: time.Now(),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := app.Validate(); err != nil {
		return nil, fmt.Errorf("invalid installation configuration: %w", err)
	}

	if err := m.db.WithContext(ctx).Create(app).Error; err != nil {
		return nil, fmt.Errorf("failed to install app: %w", err)
	}

	// Increment install count
	if err := m.db.WithContext(ctx).
		Model(&domain.Connector{}).
		Where("id = ?", connectorID).
		Update("install_count", gorm.Expr("install_count + 1")).Error; err != nil {
		m.logger.Printf("Warning: failed to increment install count: %v", err)
	}

	m.installations[app.ID] = app
	m.logAction(ctx, app.ID, userID, "install", map[string]interface{}{"connector_id": connectorID}, "success")

	m.logger.Printf("App installed: %s (connector: %s)", app.Name, connector.Name)

	return app, nil
}

// GetApp retrieves a marketplace app by ID
func (m *MarketplaceService) GetApp(ctx context.Context, appID string) (*domain.MarketplaceApp, error) {
	m.mu.RLock()
	if app, exists := m.installations[appID]; exists {
		m.mu.RUnlock()
		return app, nil
	}
	m.mu.RUnlock()

	var app domain.MarketplaceApp
	if err := m.db.WithContext(ctx).First(&app, "id = ?", appID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("app not found")
		}
		return nil, fmt.Errorf("failed to retrieve app: %w", err)
	}

	return &app, nil
}

// ListApps lists all installed apps for a tenant
func (m *MarketplaceService) ListApps(ctx context.Context, tenantID string, limit int, offset int) ([]domain.MarketplaceApp, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var apps []domain.MarketplaceApp
	var total int64

	query := m.db.WithContext(ctx).Where("tenant_id = ?", tenantID)

	if err := query.Model(&domain.MarketplaceApp{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count apps: %w", err)
	}

	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&apps).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list apps: %w", err)
	}

	return apps, total, nil
}

// UpdateApp updates a marketplace app configuration
func (m *MarketplaceService) UpdateApp(ctx context.Context, appID, userID string, config map[string]interface{}) (*domain.MarketplaceApp, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, err := m.GetApp(ctx, appID)
	if err != nil {
		return nil, err
	}

	app.Configuration = config
	app.UpdatedAt = time.Now()

	if err := m.db.WithContext(ctx).Save(app).Error; err != nil {
		return nil, fmt.Errorf("failed to update app: %w", err)
	}

	m.logAction(ctx, appID, userID, "config_change", map[string]interface{}{"config": config}, "success")

	return app, nil
}

// EnableApp enables a marketplace app
func (m *MarketplaceService) EnableApp(ctx context.Context, appID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.db.WithContext(ctx).
		Model(&domain.MarketplaceApp{}).
		Where("id = ?", appID).
		Updates(map[string]interface{}{"enabled": true, "updated_at": time.Now()}).Error; err != nil {
		return fmt.Errorf("failed to enable app: %w", err)
	}

	m.logAction(ctx, appID, userID, "enable", nil, "success")
	return nil
}

// DisableApp disables a marketplace app
func (m *MarketplaceService) DisableApp(ctx context.Context, appID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.db.WithContext(ctx).
		Model(&domain.MarketplaceApp{}).
		Where("id = ?", appID).
		Updates(map[string]interface{}{"enabled": false, "updated_at": time.Now()}).Error; err != nil {
		return fmt.Errorf("failed to disable app: %w", err)
	}

	m.logAction(ctx, appID, userID, "disable", nil, "success")
	return nil
}

// UninstallApp uninstalls a marketplace app
func (m *MarketplaceService) UninstallApp(ctx context.Context, appID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, err := m.GetApp(ctx, appID)
	if err != nil {
		return err
	}

	// Stop sync worker if running
	m.stopSyncWorker(appID)

	if err := m.db.WithContext(ctx).
		Model(&domain.MarketplaceApp{}).
		Where("id = ?", appID).
		Updates(map[string]interface{}{
			"status":     domain.InstallationStatusUninstalled,
			"enabled":    false,
			"updated_at": time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("failed to uninstall app: %w", err)
	}

	// Decrement install count
	if err := m.db.WithContext(ctx).
		Model(&domain.Connector{}).
		Where("id = ?", app.ConnectorID).
		Update("install_count", gorm.Expr("install_count - 1")).Error; err != nil {
		m.logger.Printf("Warning: failed to decrement install count: %v", err)
	}

	delete(m.installations, appID)
	m.logAction(ctx, appID, userID, "uninstall", nil, "success")

	m.logger.Printf("App uninstalled: %s", appID)
	return nil
}

// UpdateAppSync updates sync configuration and starts/stops sync worker
func (m *MarketplaceService) UpdateAppSync(ctx context.Context, appID, userID string, autoSync bool, syncInterval int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, err := m.GetApp(ctx, appID)
	if err != nil {
		return err
	}

	app.AutoSync = autoSync
	app.SyncInterval = syncInterval
	app.UpdatedAt = time.Now()

	if err := m.db.WithContext(ctx).Save(app).Error; err != nil {
		return fmt.Errorf("failed to update sync config: %w", err)
	}

	if autoSync {
		m.startSyncWorker(app)
	} else {
		m.stopSyncWorker(appID)
	}

	m.logAction(ctx, appID, userID, "sync_config_change",
		map[string]interface{}{"auto_sync": autoSync, "interval": syncInterval}, "success")

	return nil
}

// TriggerSync manually triggers a sync for an app
func (m *MarketplaceService) TriggerSync(ctx context.Context, appID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, err := m.GetApp(ctx, appID)
	if err != nil {
		return err
	}

	if !app.Enabled {
		return fmt.Errorf("cannot sync disabled app")
	}

	now := time.Now()
	app.LastSyncAt = &now

	// Simulate successful sync (in production, this would call the connector)
	app.LastSyncStatus = "success"
	app.LastSyncError = nil

	if err := m.db.WithContext(ctx).Save(app).Error; err != nil {
		return fmt.Errorf("failed to update sync status: %w", err)
	}

	m.logAction(ctx, appID, userID, "manual_sync", nil, "success")
	m.logger.Printf("Manual sync triggered for app: %s", appID)

	return nil
}

// GetAppLogs retrieves logs for an app
func (m *MarketplaceService) GetAppLogs(ctx context.Context, appID string, action string, limit int, offset int) ([]domain.MarketplaceLog, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var logs []domain.MarketplaceLog
	var total int64

	query := m.db.WithContext(ctx).Where("app_id = ?", appID)
	if action != "" {
		query = query.Where("action = ?", action)
	}

	if err := query.Model(&domain.MarketplaceLog{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count logs: %w", err)
	}

	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to retrieve logs: %w", err)
	}

	return logs, total, nil
}

// AddConnectorReview adds a review to a connector
func (m *MarketplaceService) AddConnectorReview(ctx context.Context, connectorID, userID, author string, rating int, comment string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rating < 1 || rating > 5 {
		return fmt.Errorf("rating must be between 1 and 5")
	}

	connector, err := m.GetConnector(ctx, connectorID)
	if err != nil {
		return err
	}

	review := domain.ConnectorReview{
		ID:        uuid.New().String(),
		Rating:    rating,
		Comment:   comment,
		Author:    author,
		CreatedAt: time.Now(),
	}

	connector.Reviews = append(connector.Reviews, review)

	// Recalculate average rating
	totalRating := 0.0
	for _, r := range connector.Reviews {
		totalRating += float64(r.Rating)
	}
	connector.Rating = totalRating / float64(len(connector.Reviews))

	if err := m.db.WithContext(ctx).Save(connector).Error; err != nil {
		return fmt.Errorf("failed to add review: %w", err)
	}

	m.logger.Printf("Review added to connector %s: rating=%d", connectorID, rating)
	return nil
}

// Helper functions

func (m *MarketplaceService) startSyncWorker(app *domain.MarketplaceApp) {
	m.syncMu.Lock()
	defer m.syncMu.Unlock()

	// Stop existing worker if any
	if cancel, exists := m.syncWorkers[app.ID]; exists {
		cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.syncWorkers[app.ID] = cancel

	go func() {
		ticker := time.NewTicker(time.Duration(app.SyncInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.logger.Printf("Auto-sync triggered for app: %s", app.ID)
				m.TriggerSync(ctx, app.ID, app.UserID)
			}
		}
	}()
}

func (m *MarketplaceService) stopSyncWorker(appID string) {
	m.syncMu.Lock()
	defer m.syncMu.Unlock()

	if cancel, exists := m.syncWorkers[appID]; exists {
		cancel()
		delete(m.syncWorkers, appID)
	}
}

func (m *MarketplaceService) logAction(ctx context.Context, appID, userID, action string, details map[string]interface{}, status string) {
	go func() {
		log := &domain.MarketplaceLog{
			ID:        uuid.New().String(),
			AppID:     appID,
			UserID:    userID,
			Action:    action,
			Details:   details,
			Status:    status,
			CreatedAt: time.Now(),
		}

		if err := m.db.WithContext(ctx).Create(log).Error; err != nil {
			m.logger.Printf("Warning: failed to log action: %v", err)
		}
	}()
}

func generateRandomSecret(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
