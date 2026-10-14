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
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/application/threatintel"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/cti"
)

// The threat list's queries on PostgreSQL (#911). CPE matching uses the array
// overlap operator, which sqlite does not have. Needs a MIGRATED database (0074
// and the CTI, asset and vulnerability tables) and works inside one transaction
// it rolls back.
func TestThreatRepository_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	var hasTable bool
	db.Raw(`SELECT to_regclass('public.cti_threat_dismissals') IS NOT NULL`).Scan(&hasTable)
	if !hasTable {
		t.Skip("cti_threat_dismissals missing: point DATABASE_URL at a migrated database")
	}

	ctx := context.Background()
	mine, theirs := uuid.New(), uuid.New()
	cpe := "cpe:2.3:o:test" + uuid.NewString()[:8] + ":sslvpn:7.4.5"
	cve := "CVE-2099-" + time.Now().Format("150405")
	errRollback := errors.New("rollback")

	txErr := db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, tx.Create(&cti.CTIVulnerability{
			CVEID: cve, CVSSV3: 9.8, Severity: "CRITICAL", CISAKnown: true,
			AffectedCPE: pq.StringArray{cpe}, PublishedAt: time.Now(),
		}).Error)
		fw := domain.Asset{ID: uuid.New(), TenantID: mine, Name: "fw", Type: "server",
			Criticality: domain.CriticalityCritical, CPEs: pq.StringArray{cpe}}
		other := domain.Asset{ID: uuid.New(), TenantID: theirs, Name: "their fw", Type: "server",
			Criticality: domain.CriticalityHigh, CPEs: pq.StringArray{cpe}}
		require.NoError(t, tx.Create(&fw).Error)
		require.NoError(t, tx.Create(&other).Error)
		for _, v := range []domain.Vulnerability{
			{ID: uuid.New(), TenantID: mine, CVEID: cve, Title: "SSL-VPN RCE", Status: domain.VulnStatusOpen,
				AssetID: &fw.ID, AssetName: "fw", AssetCriticality: "CRITICAL", EPSS: 0.94, Source: "nessus"},
			// Accepted: no longer a live threat.
			{ID: uuid.New(), TenantID: mine, CVEID: "CVE-2099-0002", Title: "old", Status: domain.VulnStatusAccepted},
			{ID: uuid.New(), TenantID: theirs, CVEID: cve, Title: "theirs", Status: domain.VulnStatusOpen},
		} {
			require.NoError(t, tx.Create(&v).Error)
		}

		repo := NewGormThreatRepository(tx)
		uc := threatintel.NewListThreatsUseCase(repo, NewGormCTIRepository(tx))

		res, err := uc.Execute(ctx, mine)
		require.NoError(t, err)
		var got *threatintel.Threat
		for i := range res.Threats {
			if res.Threats[i].CVEID == cve {
				got = &res.Threats[i]
			}
		}
		require.NotNil(t, got, "the CVE reaches my firewall")
		require.Len(t, got.Assets, 1, "only my asset, never theirs")
		assert.Equal(t, "fw", got.Assets[0].Name)
		assert.ElementsMatch(t, []string{"cpe", "scan"}, got.Assets[0].Via)
		assert.Equal(t, 100, got.Relevance)

		findings, err := repo.FindingsWithCVE(ctx, mine)
		require.NoError(t, err)
		for _, f := range findings {
			assert.NotEqual(t, "CVE-2099-0002", f.CVEID, "an accepted finding is not a live threat")
		}

		// Dismiss twice, see it, restore across tenants refused, restore mine.
		d := func() *domain.CTIThreatDismissal {
			return &domain.CTIThreatDismissal{ID: uuid.New(), TenantID: mine, CVEID: cve, CreatedAt: time.Now()}
		}
		require.NoError(t, repo.Dismiss(ctx, d()))
		require.NoError(t, repo.Dismiss(ctx, d()), "a second dismissal is a no-op")
		m, err := repo.Dismissals(ctx, mine)
		require.NoError(t, err)
		assert.Contains(t, m, cve)
		theirsMap, _ := repo.Dismissals(ctx, theirs)
		assert.NotContains(t, theirsMap, cve)
		ok, err := repo.Restore(ctx, theirs, cve)
		require.NoError(t, err)
		assert.False(t, ok, "another tenant cannot restore my dismissal")
		ok, err = repo.Restore(ctx, mine, cve)
		require.NoError(t, err)
		assert.True(t, ok)
		return errRollback
	})
	require.ErrorIs(t, txErr, errRollback)
}
