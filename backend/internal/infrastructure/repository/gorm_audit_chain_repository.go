// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormAuditChainRepository is the append-only, hash-chained audit store.
//
// Appending is serialised per tenant so two concurrent writers can never claim
// the same sequence number or link to the same predecessor:
//
//   - inside the process, a per-tenant mutex;
//   - inside the database, the append runs in a transaction that reads the
//     current head FOR UPDATE (Postgres), and a unique index on
//     (tenant_id, sequence) is the final backstop — a racing insert fails
//     rather than silently forking the chain.
//
// Honest limit: the FOR UPDATE row lock is what makes this correct across
// multiple API instances. On SQLite (tests) the locking clause is skipped and
// the process mutex plus the unique index carry the guarantee.
type GormAuditChainRepository struct {
	db *gorm.DB

	mu    sync.Mutex
	locks map[uuid.UUID]*sync.Mutex
}

// NewGormAuditChainRepository builds the chained audit store.
func NewGormAuditChainRepository(db *gorm.DB) *GormAuditChainRepository {
	return &GormAuditChainRepository{db: db, locks: map[uuid.UUID]*sync.Mutex{}}
}

func (r *GormAuditChainRepository) tenantLock(tenantID uuid.UUID) *sync.Mutex {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.locks[tenantID]
	if !ok {
		l = &sync.Mutex{}
		r.locks[tenantID] = l
	}
	return l
}

func (r *GormAuditChainRepository) supportsRowLock() bool {
	return r.db.Dialector.Name() == "postgres"
}

// Append seals the event onto the tenant's chain and stores it. The caller's
// Sequence/PrevHash/Hash fields are ignored and overwritten — a client can never
// choose its own position in the chain.
func (r *GormAuditChainRepository) Append(ctx context.Context, e *domain.AuditEvent) error {
	if e == nil {
		return fmt.Errorf("nil audit event")
	}
	if e.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}

	lock := r.tenantLock(e.TenantID)
	lock.Lock()
	defer lock.Unlock()

	// SkipHooks so an append never re-enters the audittrail GORM plugin.
	return r.db.WithContext(ctx).Session(&gorm.Session{SkipHooks: true}).
		Transaction(func(tx *gorm.DB) error {
			q := tx.Model(&domain.AuditEvent{}).Where("tenant_id = ?", e.TenantID)
			if r.supportsRowLock() {
				q = q.Clauses(clause.Locking{Strength: "UPDATE"})
			}
			var head domain.AuditEvent
			err := q.Order("sequence DESC").Limit(1).Take(&head).Error
			switch {
			case err == nil:
				e.SealChain(head.Sequence+1, head.Hash)
			case err == gorm.ErrRecordNotFound:
				e.SealChain(1, domain.GenesisHash)
			default:
				return fmt.Errorf("failed to read audit chain head: %w", err)
			}
			return tx.Create(e).Error
		})
}

// List returns a filtered, paginated page of the trail (newest first).
func (r *GormAuditChainRepository) List(ctx context.Context, tenantID uuid.UUID, f domain.AuditEventFilter) ([]domain.AuditEvent, int64, error) {
	q := r.filtered(ctx, tenantID, f)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count audit events: %w", err)
	}

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var events []domain.AuditEvent
	if err := q.Order("sequence DESC").Limit(limit).Offset(f.Offset).Find(&events).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list audit events: %w", err)
	}
	return events, total, nil
}

// ListAll returns every matching event ordered by sequence ASC — the order
// verification and export need. Bounded to keep a runaway export from pinning
// the process; the bound is reported honestly by the caller.
func (r *GormAuditChainRepository) ListAll(ctx context.Context, tenantID uuid.UUID, f domain.AuditEventFilter) ([]domain.AuditEvent, error) {
	q := r.filtered(ctx, tenantID, f)
	limit := f.Limit
	if limit <= 0 {
		limit = 100000
	}
	var events []domain.AuditEvent
	if err := q.Order("sequence ASC").Limit(limit).Find(&events).Error; err != nil {
		return nil, fmt.Errorf("failed to read audit events: %w", err)
	}
	return events, nil
}

func (r *GormAuditChainRepository) filtered(ctx context.Context, tenantID uuid.UUID, f domain.AuditEventFilter) *gorm.DB {
	q := r.db.WithContext(ctx).Model(&domain.AuditEvent{}).Where("tenant_id = ?", tenantID)
	if f.EntityType != "" {
		q = q.Where("entity_type = ?", f.EntityType)
	}
	if f.EntityID != "" {
		q = q.Where("entity_id = ?", f.EntityID)
	}
	if f.Action != "" {
		q = q.Where("action = ?", f.Action)
	}
	if f.ActorID != nil {
		q = q.Where("actor_id = ?", *f.ActorID)
	}
	if f.RequestID != "" {
		q = q.Where("request_id = ?", f.RequestID)
	}
	if f.Source != "" {
		q = q.Where("source = ?", f.Source)
	}
	if f.From != nil {
		q = q.Where("created_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("created_at <= ?", *f.To)
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where(
			"LOWER(summary) LIKE ? OR LOWER(entity_type) LIKE ? OR LOWER(entity_id) LIKE ? OR LOWER(COALESCE(path,'')) LIKE ?",
			like, like, like, like)
	}
	return q
}

// ListSeals returns the tenant's retention seals, oldest first.
func (r *GormAuditChainRepository) ListSeals(ctx context.Context, tenantID uuid.UUID) ([]domain.AuditChainSeal, error) {
	var seals []domain.AuditChainSeal
	if err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).
		Order("from_sequence ASC").Find(&seals).Error; err != nil {
		return nil, fmt.Errorf("failed to list audit chain seals: %w", err)
	}
	return seals, nil
}

// Prune removes events strictly older than `before` and writes a seal recording
// the hash the surviving head must link back to, so verification can cross the
// gap. Returns nil when nothing was eligible.
//
// Pruning never removes the whole chain: at least the newest event is kept, so
// there is always a live head to anchor future appends.
func (r *GormAuditChainRepository) Prune(ctx context.Context, tenantID uuid.UUID, before time.Time) (*domain.AuditChainSeal, error) {
	if tenantID == uuid.Nil {
		return nil, fmt.Errorf("tenant_id is required")
	}
	lock := r.tenantLock(tenantID)
	lock.Lock()
	defer lock.Unlock()

	var seal *domain.AuditChainSeal
	err := r.db.WithContext(ctx).Session(&gorm.Session{SkipHooks: true}).
		Transaction(func(tx *gorm.DB) error {
			// Authorise the append-only trigger to let this transaction's DELETE
			// through (migration 0055, Postgres only). SET LOCAL is
			// transaction-scoped, so no other session or pooled connection inherits
			// the maintenance flag. Guarded by dialect: the GUC syntax is invalid on
			// sqlite (used by tests), where no such trigger exists anyway.
			if tx.Dialector.Name() == "postgres" {
				if err := tx.Exec("SET LOCAL openrisk.audit_maintenance = 'on'").Error; err != nil {
					return err
				}
			}
			var head domain.AuditEvent
			if err := tx.Model(&domain.AuditEvent{}).Where("tenant_id = ?", tenantID).
				Order("sequence DESC").Limit(1).Take(&head).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					return nil
				}
				return err
			}
			// The last entry is never pruned — it anchors the chain.
			var last domain.AuditEvent
			err := tx.Model(&domain.AuditEvent{}).
				Where("tenant_id = ? AND created_at < ? AND sequence < ?", tenantID, before, head.Sequence).
				Order("sequence DESC").Limit(1).Take(&last).Error
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			if err != nil {
				return err
			}
			var first domain.AuditEvent
			if err := tx.Model(&domain.AuditEvent{}).
				Where("tenant_id = ? AND sequence <= ?", tenantID, last.Sequence).
				Order("sequence ASC").Limit(1).Take(&first).Error; err != nil {
				return err
			}

			res := tx.Where("tenant_id = ? AND sequence <= ?", tenantID, last.Sequence).
				Delete(&domain.AuditEvent{})
			if res.Error != nil {
				return res.Error
			}
			s := &domain.AuditChainSeal{
				ID:           uuid.New(),
				TenantID:     tenantID,
				Reason:       "retention_prune",
				FromSequence: first.Sequence,
				ToSequence:   last.Sequence,
				PrunedCount:  res.RowsAffected,
				LastHash:     last.Hash,
				CreatedAt:    time.Now().UTC(),
			}
			if err := tx.Create(s).Error; err != nil {
				return err
			}
			seal = s
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("failed to prune audit events: %w", err)
	}
	return seal, nil
}

// =============================================================================
// Retention policy store
// =============================================================================

// GormAuditRetentionRepository persists the per-tenant retention window.
type GormAuditRetentionRepository struct{ db *gorm.DB }

// NewGormAuditRetentionRepository builds the retention store.
func NewGormAuditRetentionRepository(db *gorm.DB) *GormAuditRetentionRepository {
	return &GormAuditRetentionRepository{db: db}
}

// Get returns the tenant policy, or nil when none was ever configured.
func (r *GormAuditRetentionRepository) Get(ctx context.Context, tenantID uuid.UUID) (*domain.AuditRetentionPolicy, error) {
	var p domain.AuditRetentionPolicy
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Take(&p).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get audit retention policy: %w", err)
	}
	return &p, nil
}

// Upsert saves the tenant policy (one row per tenant).
func (r *GormAuditRetentionRepository) Upsert(ctx context.Context, p *domain.AuditRetentionPolicy) error {
	if p.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	existing, err := r.Get(ctx, p.TenantID)
	if err != nil {
		return err
	}
	if existing == nil {
		if p.ID == uuid.Nil {
			p.ID = uuid.New()
		}
		return r.db.WithContext(ctx).Create(p).Error
	}
	p.ID = existing.ID
	p.CreatedAt = existing.CreatedAt
	return r.db.WithContext(ctx).Model(&domain.AuditRetentionPolicy{}).
		Where("id = ? AND tenant_id = ?", existing.ID, p.TenantID).
		Updates(map[string]interface{}{
			"retention_days": p.RetentionDays,
			"last_pruned_at": p.LastPrunedAt,
			"updated_by":     p.UpdatedBy,
			"updated_at":     time.Now().UTC(),
		}).Error
}

// ListWithRetention returns every policy with a finite window — the input to the
// cross-tenant pruning sweep. Each row carries its own tenant.
func (r *GormAuditRetentionRepository) ListWithRetention(ctx context.Context) ([]domain.AuditRetentionPolicy, error) {
	var out []domain.AuditRetentionPolicy
	if err := r.db.WithContext(ctx).Where("retention_days > 0").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("failed to list retention policies: %w", err)
	}
	return out, nil
}
