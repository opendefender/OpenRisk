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

	"github.com/opendefender/openrisk/internal/domain"
)

// Seeding the checklist from data that already exists.
//
// The activation event log is FORWARD-ONLY: a step ticks because a use case
// recorded an event as it happened. That is correct for a tenant created after
// activation shipped, and visibly wrong for every tenant created before it. A
// bank with two hundred risks, an imported ISO 27001 catalogue and a mapped
// asset estate is told to "create your first risk" — the one panel whose job is
// to say where you are says something untrue, and there is no user action that
// can correct it, because no endpoint lets a client mark a step complete.
//
// BackfillExistingMembers already anchors `signup` per organization. This adds
// the rest: for each checklist step, if the tenant HOLDS the record that step is
// about, anchor one event dated from that record.
//
// Three properties matter more than the feature:
//
//  1. Tenancy (RULE #2). Every derivation groups by the source table's own
//     tenant column and writes that same value back. There is no join between a
//     source row and a tenant it does not belong to, so there is no arrangement
//     of the data that can make tenant B's records tick tenant A's checklist.
//  2. Idempotency. This runs on every boot. A tenant that already has an event
//     for a key is skipped entirely, so reboots neither duplicate rows nor move
//     a completed_at. (First-occurrence-wins would protect the DATE anyway; it
//     would not protect the table from growing without bound.)
//  3. Honest dates. occurred_at is MIN(created_at) of the underlying record, not
//     now(). Stamping the boot time would date every historic tenant's first
//     risk to the deploy and poison openrisk_time_to_aha_seconds — the metric
//     the launch gate is meant to cite.
//
// `profile` is deliberately NOT derived. No stored record proves a user filled
// in the profile step: BackfillExistingMembers marks pre-existing wizards
// complete without industry / country / goal, so deriving it from the wizard row
// would tick a step for work nobody did. An undone step shown as undone is the
// truthful outcome, and the one the tests pin.

// activationSource declares which stored record proves which checklist step.
//
// Model is a domain struct rather than a table name so GORM resolves the table
// AND applies the soft-delete scope: a tenant whose only risk was deleted has
// not created a risk, and raw SQL would have silently counted the tombstone.
type activationSource struct {
	EventKey  domain.ActivationEventKey
	Model     any
	TenantCol string
	TimeCol   string
	// Scope narrows what counts as proof. Nil means "any surviving row".
	Scope func(*gorm.DB) *gorm.DB
}

// activationBackfillSources maps the six derivable checklist steps to their
// evidence. `tenant_id` is the canonical column on every model here — Risk and
// Asset also carry a legacy `organization_id` alias, and every other repository
// in this package filters on tenant_id, so this does too.
func activationBackfillSources() []activationSource {
	return []activationSource{
		{
			EventKey:  domain.ActivationRiskCreated,
			Model:     &domain.Risk{},
			TenantCol: "tenant_id",
			TimeCol:   "created_at",
		},
		{
			EventKey:  domain.ActivationFrameworkImported,
			Model:     &domain.ComplianceFramework{},
			TenantCol: "tenant_id",
			TimeCol:   "created_at",
		},
		{
			EventKey:  domain.ActivationAssetConnected,
			Model:     &domain.Asset{},
			TenantCol: "tenant_id",
			TimeCol:   "created_at",
		},
		{
			EventKey:  domain.ActivationMitigationCreated,
			Model:     &domain.Mitigation{},
			TenantCol: "tenant_id",
			TimeCol:   "created_at",
		},
		{
			EventKey:  domain.ActivationReportGenerated,
			Model:     &domain.Report{},
			TenantCol: "tenant_id",
			TimeCol:   "created_at",
			// A queued or failed run is not a generated report. Only a
			// succeeded one is something the user actually got.
			Scope: func(q *gorm.DB) *gorm.DB {
				return q.Where("run_state = ?", string(domain.ReportRunSucceeded))
			},
		},
		{
			EventKey:  domain.ActivationMemberInvited,
			Model:     &domain.OrganizationMember{},
			TenantCol: "organization_id",
			TimeCol:   "created_at",
			// "Invited a teammate" means a member arrived who was not the
			// founding one. The correlated subquery excludes each organization's
			// earliest membership row and is correlated ON the tenant column, so
			// it cannot reach across tenants. Portable to both engines — a window
			// function would not be, and organizations.owner_id is not reliable
			// here (ownership can be transferred after the fact).
			Scope: func(q *gorm.DB) *gorm.DB {
				return q.Where(`created_at > (
					SELECT MIN(m2.created_at) FROM organization_members m2
					WHERE m2.organization_id = organization_members.organization_id
				)`)
			},
		},
	}
}

// derivedTenant is one (tenant, earliest evidence) pair.
type derivedTenant struct {
	TenantID uuid.UUID
	FirstAt  time.Time
}

// BackfillDerivedEvents anchors one activation event per checklist step for
// every tenant that already holds the underlying record.
//
// Runs in a single transaction: a partial seed would leave a tenant with a
// half-true checklist, which is the exact defect this exists to remove. Safe to
// call on every boot.
func (r *GormActivationRepository) BackfillDerivedEvents(ctx context.Context) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, src := range activationBackfillSources() {
			if err := backfillActivationSource(tx, src); err != nil {
				return fmt.Errorf("backfill %s: %w", src.EventKey, err)
			}
		}
		return nil
	})
}

// backfillActivationSource seeds one step.
func backfillActivationSource(tx *gorm.DB, src activationSource) error {
	// A deployment that has not migrated this table yet must not fail the boot
	// backfill for every other step.
	if !tx.Migrator().HasTable(src.Model) {
		return nil
	}

	seeded, err := tenantsWithEvent(tx, src.EventKey)
	if err != nil {
		return err
	}

	candidates, err := tenantsHoldingEvidence(tx, src)
	if err != nil {
		return err
	}

	events := make([]*domain.ActivationEvent, 0, len(candidates))
	for _, c := range candidates {
		if _, done := seeded[c.TenantID]; done {
			// Already has a first occurrence, from live use or a previous boot.
			continue
		}
		events = append(events, &domain.ActivationEvent{
			ID:       uuid.New(),
			TenantID: c.TenantID,
			// No UserID: the tenant reached this milestone, but which member did
			// is not recoverable from the record, and inventing one would put a
			// name on an act that person may not have performed.
			UserID:     nil,
			EventKey:   src.EventKey,
			OccurredAt: c.FirstAt.UTC(),
		})
	}
	if len(events) == 0 {
		return nil
	}
	return tx.CreateInBatches(events, 200).Error
}

// tenantsWithEvent returns the tenants that already have an occurrence of a key.
func tenantsWithEvent(tx *gorm.DB, key domain.ActivationEventKey) (map[uuid.UUID]struct{}, error) {
	out := map[uuid.UUID]struct{}{}
	rows, err := tx.Model(&domain.ActivationEvent{}).
		Select("DISTINCT tenant_id").
		Where("event_key = ?", string(key)).
		Rows()
	if err != nil {
		return nil, fmt.Errorf("read seeded tenants: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var raw any
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("read seeded tenants: %w", err)
		}
		if id, ok := asUUID(raw); ok {
			out[id] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read seeded tenants: %w", err)
	}
	return out, nil
}

// tenantsHoldingEvidence returns, per tenant, the earliest record proving a step.
//
// One grouped query per step, never one per tenant: an install with a thousand
// organizations must not issue a thousand statements at boot.
func tenantsHoldingEvidence(tx *gorm.DB, src activationSource) ([]derivedTenant, error) {
	q := tx.Model(src.Model).
		Select(fmt.Sprintf("%s AS tenant_id, MIN(%s) AS first_at", src.TenantCol, src.TimeCol)).
		Where(fmt.Sprintf("%s IS NOT NULL", src.TenantCol)).
		Group(src.TenantCol)
	if src.Scope != nil {
		q = src.Scope(q)
	}

	rows, err := q.Rows()
	if err != nil {
		return nil, fmt.Errorf("read evidence: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []derivedTenant
	for rows.Next() {
		var rawTenant, rawAt any
		if err := rows.Scan(&rawTenant, &rawAt); err != nil {
			return nil, fmt.Errorf("read evidence: %w", err)
		}
		tenantID, ok := asUUID(rawTenant)
		if !ok {
			// An unparseable or zero tenant is dropped rather than defaulting to
			// uuid.Nil, which would collect unrelated orphan rows into one
			// pseudo-tenant and tick a checklist nobody owns.
			continue
		}
		at, ok := asTime(rawAt)
		if !ok || at.IsZero() {
			continue
		}
		out = append(out, derivedTenant{TenantID: tenantID, FirstAt: at})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read evidence: %w", err)
	}
	return out, nil
}

// asUUID normalises whatever the driver returned for a uuid column. Postgres
// hands back a uuid.UUID or a string; sqlite (which the repository tests run on)
// hands back TEXT or a 16-byte BLOB. Same reasoning as asTime: without this the
// production query could only ever be proven on the engine nobody tests against.
//
// uuid.Nil reads as "not a tenant" — never as a valid grouping key.
func asUUID(v any) (uuid.UUID, bool) {
	var (
		id  uuid.UUID
		err error
	)
	switch t := v.(type) {
	case uuid.UUID:
		id = t
	case *uuid.UUID:
		if t == nil {
			return uuid.Nil, false
		}
		id = *t
	case string:
		id, err = uuid.Parse(t)
	case []byte:
		if len(t) == 16 {
			id, err = uuid.FromBytes(t)
		} else {
			id, err = uuid.Parse(string(t))
		}
	default:
		return uuid.Nil, false
	}
	if err != nil || id == uuid.Nil {
		return uuid.Nil, false
	}
	return id, true
}
