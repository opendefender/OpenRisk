// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
)

// The D-078 counts, on the real schema. RequireCapacity and the use-case
// quotas fail OPEN on a count error, so a broken count is no cap at all. Each
// count is filtered by tenant (a cross-tenant row never counts) and counts only
// what its cap is about: enabled rules, scan jobs of this month, workflows not
// deleted. Temp tables shadow the real ones and the transaction rolls back.
func TestCountPlanCaps_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	rollback := errors.New("rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		for _, table := range []string{
			"automation_rules", "scan_configs", "scan_jobs", "approval_workflows",
			"compliance_frameworks", "organization_members", "invitations",
		} {
			require.NoError(t, tx.Exec(`CREATE TEMP TABLE `+table+` (LIKE public.`+table+` INCLUDING ALL) ON COMMIT DROP`).Error, table)
		}
		repo := NewGormEntitlementRepository(tx)
		ctx := context.Background()
		tenant, other := uuid.New(), uuid.New()

		rule := func(tid uuid.UUID, enabled bool) {
			r := domain.AutomationRule{TenantID: tid, Name: "r", Trigger: domain.TriggerVulnerabilityDetected}
			require.NoError(t, tx.Create(&r).Error)
			// The column defaults to true, so GORM leaves a false bool out of the insert.
			require.NoError(t, tx.Model(&r).Update("enabled", enabled).Error)
		}
		rule(tenant, true)
		rule(tenant, true)
		rule(tenant, false) // a suspended rule costs nothing
		rule(other, true)
		deletedRule := domain.AutomationRule{TenantID: tenant, Name: "gone", Trigger: domain.TriggerVulnerabilityDetected}
		require.NoError(t, tx.Create(&deletedRule).Error)
		require.NoError(t, tx.Delete(&deletedRule).Error) // soft-deleted, even enabled, costs nothing

		cfg := func(tid uuid.UUID) uuid.UUID {
			c := domain.ScanConfig{TenantID: tid, Name: "s", Provider: domain.ProviderAgent}
			require.NoError(t, tx.Create(&c).Error)
			return c.ID
		}
		mine := cfg(tenant)
		theirs := cfg(other)
		cfg(other)
		goneCfg := domain.ScanConfig{TenantID: tenant, Name: "gone", Provider: domain.ProviderAgent}
		require.NoError(t, tx.Create(&goneCfg).Error)
		require.NoError(t, tx.Delete(&goneCfg).Error)

		fw := func(tid uuid.UUID, name string) *domain.ComplianceFramework {
			f := domain.ComplianceFramework{TenantID: tid, Name: name}
			require.NoError(t, tx.Create(&f).Error)
			return &f
		}
		fw(tenant, "iso")
		require.NoError(t, tx.Delete(fw(tenant, "deleted")).Error) // a deleted framework frees its slot
		fw(other, "theirs")

		// Seats: active members plus invitations that can still be accepted.
		member := func(org uuid.UUID, active bool) {
			m := domain.OrganizationMember{ID: uuid.New(), OrganizationID: org, UserID: uuid.New(), Role: domain.RoleUser, IsActive: true}
			require.NoError(t, tx.Create(&m).Error)
			require.NoError(t, tx.Model(&m).Update("is_active", active).Error)
		}
		member(tenant, true)
		member(tenant, false)
		member(other, true)
		invite := func(org uuid.UUID, status domain.InvitationStatus, expires time.Time) {
			inv := domain.Invitation{
				ID: uuid.New(), OrganizationID: org, Email: uuid.NewString() + "@example.test", Role: domain.RoleUser,
				TokenHash: uuid.NewString(), Status: status, ExpiresAt: expires, InvitedByID: uuid.New(), LastSentAt: time.Now(),
			}
			require.NoError(t, tx.Create(&inv).Error)
		}
		later := time.Now().Add(24 * time.Hour)
		invite(tenant, domain.InvitationPending, later)
		invite(tenant, domain.InvitationPending, time.Now().Add(-time.Hour)) // expired: no seat
		invite(tenant, domain.InvitationAccepted, later)                     // the member row counts it
		invite(other, domain.InvitationPending, later)

		now := time.Now().UTC()
		lastMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Add(-time.Hour)
		for _, j := range []struct {
			tid, cfg uuid.UUID
			at       time.Time
		}{{tenant, mine, now}, {tenant, mine, now}, {tenant, mine, lastMonth}, {other, theirs, now}} {
			job := domain.ScanJob{TenantID: j.tid, ConfigID: j.cfg, Provider: domain.ProviderAgent, Status: domain.ScanQueued}
			require.NoError(t, tx.Create(&job).Error)
			require.NoError(t, tx.Model(&job).Update("created_at", j.at).Error)
		}

		gone := domain.ApprovalWorkflow{TenantID: tenant, Name: "gone"}
		require.NoError(t, tx.Create(&domain.ApprovalWorkflow{TenantID: tenant, Name: "w"}).Error)
		require.NoError(t, tx.Create(&gone).Error)
		require.NoError(t, tx.Delete(&gone).Error) // soft delete frees the slot
		require.NoError(t, tx.Create(&domain.ApprovalWorkflow{TenantID: other, Name: "o"}).Error)

		for key, want := range map[ent.LimitKey]int{
			ent.LimitAutomationRules:   2,
			ent.LimitScanSources:       1,
			ent.LimitScansPerMonth:     2,
			ent.LimitApprovalWorkflows: 1,
			ent.LimitFrameworks:        1,
			ent.LimitUsers:             2,
		} {
			n, err := repo.Count(ctx, tenant, key, time.Now())
			require.NoError(t, err, key)
			require.Equal(t, want, n, "%s", key)
		}
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
