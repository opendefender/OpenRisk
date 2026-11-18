// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlstatus

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	app "github.com/opendefender/openrisk/internal/application/controlstatus"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/tenantguard"
)

// ClaimLease is how long a claimed row is hidden from the next claim. The worker
// recomputes and saves well inside it; a crash in between is retried when it ends.
const ClaimLease = 5 * time.Minute

// Store is the evaluation cache. Every statement that is not the worker's claim
// binds the tenant and the control. The claim is the one cross-tenant read.
type Store struct{ db *gorm.DB }

// NewStore builds the store.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

var _ app.Store = (*Store)(nil)

// sweepDB marks a statement as the worker's cross-tenant claim: each row it
// returns carries its own tenant, and everything after it is scoped to that row's.
func sweepDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	return tenantguard.CrossTenant(tx.WithContext(ctx), "control status sweep; each row carries its tenant")
}

type claimRow struct {
	TenantID  uuid.UUID
	ControlID uuid.UUID
}

// Claim takes up to limit controls: stale or due rows first (oldest evaluation
// first), then live controls with no row (cold fill), whose row it creates as a
// placeholder so a write that lands during the computation has a row to mark.
// stale is cleared and next_change_at set to now + ClaimLease on the rows taken,
// all in one short transaction; on PostgreSQL the rows are locked SKIP LOCKED, so
// two replicas never take the same one.
func (s *Store) Claim(ctx context.Context, now time.Time, limit int) ([]app.ControlRef, error) {
	if limit <= 0 {
		return nil, nil
	}
	now = now.UTC()
	lease := now.Add(ClaimLease)
	var out []app.ControlRef
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := sweepDB(ctx, tx).Table(TableControlStatusEvaluations).Select("tenant_id, control_id").
			Where("stale = ? OR next_change_at <= ?", true, now).Order("evaluated_at ASC").Limit(limit)
		if tx.Dialector.Name() == "postgres" {
			q = q.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		var due []claimRow
		if err := q.Scan(&due).Error; err != nil {
			return err
		}
		if len(due) > 0 {
			pairs := make([][]interface{}, len(due))
			for i, d := range due {
				pairs[i] = []interface{}{d.TenantID, d.ControlID}
				out = append(out, app.ControlRef{TenantID: d.TenantID, ControlID: d.ControlID})
			}
			if err := sweepDB(ctx, tx).Table(TableControlStatusEvaluations).
				Where("(tenant_id, control_id) IN ?", pairs).
				Updates(map[string]interface{}{"stale": false, "next_change_at": lease}).Error; err != nil {
				return err
			}
		}
		room := limit - len(due)
		if room <= 0 {
			return nil
		}
		var cold []claimRow
		if err := sweepDB(ctx, tx).Raw(`SELECT c.tenant_id AS tenant_id, c.id AS control_id FROM controls c
			LEFT JOIN control_status_evaluations s ON s.tenant_id = c.tenant_id AND s.control_id = c.id
			WHERE c.deleted_at IS NULL AND s.control_id IS NULL ORDER BY c.created_at ASC LIMIT ?`, room).Scan(&cold).Error; err != nil {
			return err
		}
		if len(cold) == 0 {
			return nil
		}
		rows := make([]EvaluationRow, len(cold))
		for i, c := range cold {
			rows[i] = EvaluationRow{
				TenantID: c.TenantID, ControlID: c.ControlID, Status: string(domain.ComputedStatusInsufficientEvidence),
				UnderlyingStatus: string(domain.ComputedStatusInsufficientEvidence),
				EvaluatedAt:      time.Unix(0, 0).UTC(), NextChangeAt: &lease,
			}
			out = append(out, app.ControlRef{TenantID: c.TenantID, ControlID: c.ControlID})
		}
		return tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
	})
	if err != nil {
		return nil, fmt.Errorf("controlstatus.Claim: %w", err)
	}
	return out, nil
}

// Save upserts the evaluation of one control of the tenant. The conflict update
// is guarded by evaluated_at (an older computation never overwrites a newer one)
// and does not touch stale: a write that marked the row after the claim keeps its
// mark and the row is taken again on the next tick.
func (s *Store) Save(ctx context.Context, tenantID, controlID uuid.UUID, ev app.Evaluation, evaluatedAt time.Time) error {
	if tenantID == uuid.Nil {
		return domain.ErrForbidden
	}
	row := EvaluationRow{
		TenantID: tenantID, ControlID: controlID, Status: string(ev.Status), UnderlyingStatus: string(ev.Underlying),
		FreshSignals: ev.Fresh, CountedSignals: ev.Counted, FreshnessPercent: ev.Percent,
		EvaluatedAt: evaluatedAt.UTC(), NextChangeAt: ev.NextChangeAt,
	}
	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "control_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"status", "underlying_status", "fresh_signals", "counted_signals", "freshness_percent", "evaluated_at", "next_change_at",
		}),
		Where: clause.Where{Exprs: []clause.Expression{
			clause.Expr{SQL: TableControlStatusEvaluations + ".evaluated_at <= ?", Vars: []interface{}{evaluatedAt.UTC()}},
		}},
	}).Create(&row).Error
	if err != nil {
		return fmt.Errorf("controlstatus.Save: %w", err)
	}
	return nil
}

// Delete removes the row of a control of the tenant.
func (s *Store) Delete(ctx context.Context, tenantID, controlID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return domain.ErrForbidden
	}
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND control_id = ?", tenantID, controlID).
		Delete(&EvaluationRow{}).Error
	if err != nil {
		return fmt.Errorf("controlstatus.Delete: %w", err)
	}
	return nil
}
