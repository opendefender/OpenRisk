// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package automation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	appauto "github.com/opendefender/openrisk/internal/application/automation"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/pkg/ticketing"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Risk actions — create / assign / resolve on the real Risk entity.
// ---------------------------------------------------------------------------

// RiskActions implements appauto.RiskCreator, RiskAssigner and RiskResolver on
// top of the GORM risk + user repositories.
type RiskActions struct {
	risks  *repository.GormRiskRepository
	users  *repository.GormUserRepository
	db     *gorm.DB
	logger zerolog.Logger
}

// NewRiskActions builds the risk action adapter.
func NewRiskActions(risks *repository.GormRiskRepository, users *repository.GormUserRepository, db *gorm.DB, logger zerolog.Logger) *RiskActions {
	return &RiskActions{risks: risks, users: users, db: db, logger: logger}
}

var (
	_ appauto.RiskCreator     = (*RiskActions)(nil)
	_ appauto.RiskAssigner    = (*RiskActions)(nil)
	_ appauto.RiskResolver    = (*RiskActions)(nil)
	_ appauto.RiskStateLookup = (*RiskActions)(nil)
)

// IsRiskResolved reports whether a risk has reached a resolved state, used by
// the SLA auto-close sweep.
func (a *RiskActions) IsRiskResolved(ctx context.Context, tenantID, riskID uuid.UUID) (bool, error) {
	risk, err := a.risks.GetByID(ctx, riskID, tenantID)
	if err != nil {
		return false, err
	}
	switch risk.Status {
	case domain.RiskMitigated, domain.RiskClosed, domain.RiskAccepted,
		domain.StatusMitigated, domain.StatusAccepted:
		return true, nil
	default:
		return false, nil
	}
}

// EnsureRisk reuses an existing risk for the CVE (idempotent) or opens a new one.
func (a *RiskActions) EnsureRisk(ctx context.Context, req appauto.RiskRequest) (appauto.RiskResult, error) {
	if strings.TrimSpace(req.CVEID) != "" {
		if existing, err := a.risks.GetByCVE(ctx, req.CVEID, req.TenantID); err == nil && existing != nil {
			return appauto.RiskResult{RiskID: existing.ID, Created: false}, nil
		}
	}
	prob, impact, crit := severityToRisk(req.Severity)
	title := req.Title
	if title == "" {
		title = "Automated risk"
	}
	r := &domain.Risk{
		ID:             uuid.New(),
		TenantID:       req.TenantID,
		OrganizationID: req.TenantID,
		Name:           title,
		Title:          title,
		Description:    "Opened automatically by OpenRisk Security Automation.",
		Probability:    prob,
		Impact:         impact,
		Criticality:    crit,
		Status:         domain.RiskOpen,
		LifecyclePhase: domain.PhaseIdentified,
		Source:         domain.SourceScanAuto,
		AssetID:        req.AssetID,
		CreatedBy:      req.CreatedBy,
	}
	if req.CVEID != "" {
		cve := req.CVEID
		r.SourceCVEID = &cve
	}
	if err := a.risks.Create(ctx, r); err != nil {
		return appauto.RiskResult{}, err
	}
	return appauto.RiskResult{RiskID: r.ID, Created: true}, nil
}

// Assign resolves a target (user id / email / role) to a user and assigns the risk.
func (a *RiskActions) Assign(ctx context.Context, tenantID, riskID uuid.UUID, target string) (uuid.UUID, error) {
	assignee, err := a.resolveAssignee(ctx, tenantID, target)
	if err != nil {
		return uuid.Nil, err
	}
	if assignee == uuid.Nil {
		return uuid.Nil, fmt.Errorf("no assignee resolved for target %q", target)
	}
	risk, err := a.risks.GetByID(ctx, riskID, tenantID)
	if err != nil {
		return uuid.Nil, err
	}
	risk.AssignedTo = &assignee
	if err := a.risks.Update(ctx, risk); err != nil {
		return uuid.Nil, err
	}
	return assignee, nil
}

// Resolve marks a risk mitigated (auto-close on confirmed remediation).
func (a *RiskActions) Resolve(ctx context.Context, tenantID, riskID uuid.UUID) error {
	risk, err := a.risks.GetByID(ctx, riskID, tenantID)
	if err != nil {
		return err
	}
	now := time.Now()
	risk.Status = domain.RiskMitigated
	risk.LastMitigatedAt = &now
	return a.risks.Update(ctx, risk)
}

func (a *RiskActions) resolveAssignee(ctx context.Context, tenantID uuid.UUID, target string) (uuid.UUID, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		// Default: first admin of the tenant.
		return a.firstMemberWithRole(ctx, tenantID, domain.RoleAdmin, domain.RoleRoot), nil
	}
	if id, err := uuid.Parse(target); err == nil {
		return id, nil
	}
	if strings.Contains(target, "@") && a.users != nil {
		if u, err := a.users.GetByEmail(ctx, target); err == nil && u != nil {
			return u.ID, nil
		}
	}
	// Treat as a role.
	switch strings.ToLower(target) {
	case "admin", "manager":
		return a.firstMemberWithRole(ctx, tenantID, domain.RoleAdmin, domain.RoleRoot), nil
	case "root":
		return a.firstMemberWithRole(ctx, tenantID, domain.RoleRoot), nil
	}
	return uuid.Nil, nil
}

func (a *RiskActions) firstMemberWithRole(ctx context.Context, tenantID uuid.UUID, roles ...domain.MemberRole) uuid.UUID {
	var id uuid.UUID
	if err := a.db.WithContext(ctx).
		Model(&domain.OrganizationMember{}).
		Where("organization_id = ? AND role IN ?", tenantID, roles).
		Order("created_at ASC").
		Limit(1).
		Pluck("user_id", &id).Error; err != nil {
		a.logger.Debug().Err(err).Msg("automation assign: member lookup failed")
	}
	return id
}

func severityToRisk(sev string) (prob, impact float64, crit domain.CriticalityLevel) {
	switch strings.ToLower(sev) {
	case "critical":
		return 0.9, 9.0, domain.RiskCriticalityCritical
	case "high":
		return 0.7, 7.0, domain.RiskCriticalityHigh
	case "medium":
		return 0.5, 5.0, domain.RiskCriticalityMedium
	default:
		return 0.3, 3.0, domain.RiskCriticalityLow
	}
}

// ---------------------------------------------------------------------------
// Ticket action — reuse the tenant ITSM config + pkg/ticketing.
// ---------------------------------------------------------------------------

// credDecryptor is the AES-GCM credential cipher (scanner.CredentialCipher).
type credDecryptor interface {
	DecryptCredentials(ciphertext string) (map[string]string, error)
}

// Ticketer implements appauto.Ticketer using the tenant's VulnTicketingConfig.
type Ticketer struct {
	integrations domain.VulnIntegrationRepository
	cipher       credDecryptor
}

// NewTicketer builds the ITSM ticket adapter.
func NewTicketer(integrations domain.VulnIntegrationRepository, cipher credDecryptor) *Ticketer {
	return &Ticketer{integrations: integrations, cipher: cipher}
}

var _ appauto.Ticketer = (*Ticketer)(nil)

// OpenTicket opens a ticket via the tenant's configured provider.
func (t *Ticketer) OpenTicket(ctx context.Context, req appauto.TicketRequest) (appauto.TicketResult, error) {
	cfg, err := t.integrations.GetTicketing(ctx, req.TenantID)
	if err != nil {
		return appauto.TicketResult{}, err
	}
	if cfg == nil || !cfg.Enabled || cfg.Provider == domain.TicketProviderNone {
		return appauto.TicketResult{}, fmt.Errorf("ITSM ticketing not configured")
	}
	providerName := req.Provider
	if providerName == "" {
		providerName = string(cfg.Provider)
	}
	provider, ok := ticketing.ProviderFor(providerName)
	if !ok {
		return appauto.TicketResult{}, fmt.Errorf("unknown ticket provider %q", providerName)
	}
	creds, err := t.cipher.DecryptCredentials(cfg.EncryptedCredentials)
	if err != nil {
		return appauto.TicketResult{}, err
	}
	tk, err := provider.Create(ctx, ticketing.CreateRequest{
		BaseURL:        cfg.BaseURL,
		Credentials:    creds,
		ProjectOrTable: cfg.ProjectOrTable,
		IssueType:      cfg.DefaultIssueType,
		Summary:        req.Summary,
		Description:    req.Description,
		Priority:       strings.ToLower(req.Severity),
		Labels:         req.Labels,
	})
	if err != nil {
		return appauto.TicketResult{}, err
	}
	return appauto.TicketResult{Provider: tk.Provider, Key: tk.Key, URL: tk.URL}, nil
}

// ---------------------------------------------------------------------------
// Scan action — trigger a real scan to confirm exposure.
// ---------------------------------------------------------------------------

// scanTrigger is the slice of the scanner TriggerScanUseCase this adapter needs.
type scanTrigger interface {
	Execute(ctx context.Context, tenantID, triggeredBy, configID uuid.UUID) (*domain.ScanJob, error)
}

// ScanAction implements appauto.AssetScanner. The scanner is scan-config based,
// so a "targeted asset scan" is fulfilled by triggering the tenant's first
// enabled scan config as an exposure-confirmation run. Honest limitation: it is
// not filtered to the single asset (documented).
type ScanAction struct {
	configs domain.ScanConfigRepository
	trigger scanTrigger
	logger  zerolog.Logger
}

// NewScanAction builds the scan adapter.
func NewScanAction(configs domain.ScanConfigRepository, trigger scanTrigger, logger zerolog.Logger) *ScanAction {
	return &ScanAction{configs: configs, trigger: trigger, logger: logger}
}

var _ appauto.AssetScanner = (*ScanAction)(nil)

// ScanAsset triggers the tenant's first enabled scan config. Returns its job ID.
func (s *ScanAction) ScanAsset(ctx context.Context, tenantID, assetID uuid.UUID) (string, error) {
	configs, err := s.configs.List(ctx, tenantID)
	if err != nil {
		return "", err
	}
	for i := range configs {
		cfg := configs[i]
		if !cfg.Enabled {
			continue
		}
		job, err := s.trigger.Execute(ctx, tenantID, uuid.Nil, cfg.ID)
		if err != nil {
			s.logger.Debug().Err(err).Str("config", cfg.ID.String()).Msg("automation scan: trigger skipped")
			continue
		}
		return job.ID.String(), nil
	}
	return "", fmt.Errorf("no enabled scan config to confirm exposure")
}
