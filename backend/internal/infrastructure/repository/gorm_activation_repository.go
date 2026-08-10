// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormActivationRepository implements domain.ActivationRepository and
// domain.OnboardingRepository over GORM. Every query filters by tenant_id
// (RULE #2); a row owned by another tenant reads back as absent.
//
// The event table is APPEND-ONLY by contract: there is no Update and no Delete
// here, and the read path takes MIN(occurred_at) per key. That is what makes a
// completed step stay completed — a later occurrence of the same event cannot
// move a completed_at, and cannot re-fire a celebration.
type GormActivationRepository struct {
	db *gorm.DB
}

// NewGormActivationRepository builds the repository.
func NewGormActivationRepository(db *gorm.DB) *GormActivationRepository {
	return &GormActivationRepository{db: db}
}

// =============================================================================
// Activation events
// =============================================================================

// RecordEvent appends one occurrence.
func (r *GormActivationRepository) RecordEvent(ctx context.Context, event *domain.ActivationEvent) error {
	if event == nil {
		return fmt.Errorf("activation event is nil")
	}
	if event.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	if event.EventKey == "" {
		return fmt.Errorf("event_key is required")
	}
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	return r.db.WithContext(ctx).Create(event).Error
}

// MIN(occurred_at) loses the column's declared type: Postgres hands back a
// timestamp, sqlite (which the repository tests run on) hands back TEXT. Scanning
// it strictly into a time.Time fails on sqlite — which would mean the tests could
// never exercise the production query, and the query would only ever be proven on
// the engine nobody tests against. asTime normalises whatever the driver gives.

// asTime normalises whatever the driver returned for an aggregated timestamp.
// Unparseable values are dropped rather than defaulting to the zero time, which
// would date a completed step to year 1 and make it look like the oldest event
// the tenant ever had.
func asTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case *time.Time:
		if t == nil {
			return time.Time{}, false
		}
		return *t, true
	case []byte:
		return parseDriverTime(string(t))
	case string:
		return parseDriverTime(t)
	}
	return time.Time{}, false
}

// parseDriverTime accepts the layouts the supported drivers emit for a timestamp
// rendered as text.
func parseDriverTime(raw string) (time.Time, bool) {
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// FirstOccurrences returns the earliest occurrence per event key for one tenant,
// in a single grouped query (no N+1, and no "load every event then reduce in Go"
// — an active tenant accumulates thousands of risk.created rows).
func (r *GormActivationRepository) FirstOccurrences(ctx context.Context, tenantID uuid.UUID) (map[domain.ActivationEventKey]time.Time, error) {
	out := map[domain.ActivationEventKey]time.Time{}
	if tenantID == uuid.Nil {
		// Fail closed: no tenant context means no activation state, never
		// everyone's activation state.
		return out, nil
	}

	// Raw rows rather than a struct scan: the aggregate's type is engine-dependent
	// (see firstOccurrenceRow), and GORM's struct scan rejects an `any` field
	// outright. database/sql scans into an interface on every driver.
	rows, err := r.db.WithContext(ctx).
		Model(&domain.ActivationEvent{}).
		Select("event_key, MIN(occurred_at) AS first_at").
		Where("tenant_id = ?", tenantID).
		Group("event_key").
		Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to read activation events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var key string
		var raw any
		if err := rows.Scan(&key, &raw); err != nil {
			return nil, fmt.Errorf("failed to read activation events: %w", err)
		}
		if at, ok := asTime(raw); ok {
			out[domain.ActivationEventKey(key)] = at
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read activation events: %w", err)
	}
	return out, nil
}

// HasEvent reports whether a key has ever occurred for a tenant.
func (r *GormActivationRepository) HasEvent(ctx context.Context, tenantID uuid.UUID, key domain.ActivationEventKey) (bool, error) {
	if tenantID == uuid.Nil {
		return false, nil
	}
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.ActivationEvent{}).
		Where("tenant_id = ? AND event_key = ?", tenantID, key).
		Limit(1).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check activation event: %w", err)
	}
	return count > 0, nil
}

// =============================================================================
// Celebrations
// =============================================================================

// CelebratedSteps returns the steps this user has already celebrated.
func (r *GormActivationRepository) CelebratedSteps(ctx context.Context, tenantID, userID uuid.UUID) (map[string]time.Time, error) {
	out := map[string]time.Time{}
	if tenantID == uuid.Nil || userID == uuid.Nil {
		return out, nil
	}

	var rows []domain.ActivationCelebration
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to read activation celebrations: %w", err)
	}
	for _, row := range rows {
		out[row.StepKey] = row.CelebratedAt
	}
	return out, nil
}

// MarkCelebrated records that the user has seen the burst for a step. Idempotent:
// the unique index on (user_id, step_key) turns a repeat into a no-op rather than
// an error, so a double-fired client callback cannot 500.
func (r *GormActivationRepository) MarkCelebrated(ctx context.Context, tenantID, userID uuid.UUID, stepKey string) error {
	if tenantID == uuid.Nil || userID == uuid.Nil {
		return fmt.Errorf("tenant_id and user_id are required")
	}
	if stepKey == "" {
		return fmt.Errorf("step_key is required")
	}

	row := &domain.ActivationCelebration{
		ID:           uuid.New(),
		TenantID:     tenantID,
		UserID:       userID,
		StepKey:      stepKey,
		CelebratedAt: time.Now().UTC(),
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "step_key"}},
			DoNothing: true,
		}).
		Create(row).Error
}

// BackfillExistingMembers marks every EXISTING member as already onboarded, and
// anchors a signup event for every existing organization.
//
// This is not a convenience: the new route guard blocks the whole application
// until onboarding.completed is true, so without this an upgrade would greet
// every current customer with a signup wizard they never asked for and cannot
// skip. Migration 0043 does the same thing in SQL — this runs it again from Go
// because RunMigrations only executes when DATABASE_URL is set, and "the guard
// works only if you happened to configure golang-migrate" is not an acceptable
// dependency for a lockout.
//
// Idempotent (both statements are conditional) and best-effort: a failure here
// logs and lets boot continue rather than taking the server down.
func (r *GormActivationRepository) BackfillExistingMembers(ctx context.Context) error {
	// Existing members are past onboarding by definition — they were using the
	// product before it existed.
	if err := r.db.WithContext(ctx).Exec(`
		INSERT INTO onboarding_progress (id, tenant_id, user_id, current_step, completed, completed_at)
		SELECT gen_random_uuid(), om.organization_id, om.user_id, 'team', true, now()
		FROM organization_members om
		ON CONFLICT (user_id) DO NOTHING
	`).Error; err != nil {
		return fmt.Errorf("backfill onboarding progress: %w", err)
	}

	// A signup anchor per organization, so time-to-Aha has an honest t0 for
	// tenants that predate the metric (their organization's creation date).
	if err := r.db.WithContext(ctx).Exec(`
		INSERT INTO activation_events (id, tenant_id, user_id, event_key, occurred_at)
		SELECT gen_random_uuid(), o.id, o.owner_id, ?, o.created_at
		FROM organizations o
		WHERE NOT EXISTS (
			SELECT 1 FROM activation_events ae
			WHERE ae.tenant_id = o.id AND ae.event_key = ?
		)
	`, string(domain.ActivationSignup), string(domain.ActivationSignup)).Error; err != nil {
		return fmt.Errorf("backfill signup anchors: %w", err)
	}

	return nil
}

// =============================================================================
// Onboarding wizard progress
// =============================================================================

// Get returns the user's wizard state, or (nil, nil) when they have none yet.
// The tenant filter is part of the lookup, so a user rehomed to another tenant
// starts a fresh wizard there instead of inheriting foreign answers.
func (r *GormActivationRepository) Get(ctx context.Context, tenantID, userID uuid.UUID) (*domain.OnboardingProgress, error) {
	if tenantID == uuid.Nil || userID == uuid.Nil {
		return nil, nil
	}

	var p domain.OnboardingProgress
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		First(&p).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get onboarding progress: %w", err)
	}
	return &p, nil
}

// Save inserts or updates the user's wizard state. The row is keyed by user_id
// (unique), so a resumed wizard always writes back to the same row.
func (r *GormActivationRepository) Save(ctx context.Context, p *domain.OnboardingProgress) error {
	if p == nil {
		return fmt.Errorf("onboarding progress is nil")
	}
	if p.TenantID == uuid.Nil || p.UserID == uuid.Nil {
		return fmt.Errorf("tenant_id and user_id are required")
	}
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.CurrentStep == "" {
		p.CurrentStep = domain.OnboardingStepOrganization
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"tenant_id", "current_step", "completed", "completed_at",
				"industry", "country", "goal", "answers", "updated_at",
			}),
		}).
		Create(p).Error
}
