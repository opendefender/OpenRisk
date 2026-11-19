// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlstatus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	app "github.com/opendefender/openrisk/internal/application/controlstatus"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// Loader reads the inputs of one control in one read-only snapshot, in at most
// five queries (ADR 0012 point 5, #555 criterion 5):
//
//	Q1 the control, with its open risk count as a scalar subquery
//	Q2 its in-scope tests (live tests of live, not retired implementations)
//	Q3 the latest result of each test (window function, skipped without tests)
//	Q4 the evidence reached by an evidenced_by edge from the control or from the
//	   latest result of a test (skipped when there is nothing to reach)
//	Q5 its exceptions that are not revoked and not yet over
//
// Every query binds the tenant on every tenant-scoped table it reads, as a `?`
// value, so the tenant guard reads it. A miss on Q1 is ErrNotFound and nothing
// else is read.
type Loader struct{ db *gorm.DB }

// NewLoader builds the loader.
func NewLoader(db *gorm.DB) *Loader { return &Loader{db: db} }

var _ app.Loader = (*Loader)(nil)

func notFound() error {
	return domain.ControlStatusError(domain.ErrNotFound, domain.CtlStatusNotFound)
}

// openRiskStatuses are the statuses of a risk that is still open: the current
// vocabulary (open, in_progress) and the legacy one that is still stored (DRAFT,
// ACTIVE). mitigated, accepted and closed are not open.
const openRiskStatuses = `('open', 'in_progress', 'DRAFT', 'ACTIVE')`

type controlRead struct {
	Applicability  string
	DeclaredStatus string
	OpenRisks      int
}

type testRead struct {
	ID      uuid.UUID
	Cadence *string
}

type resultRead struct {
	ID         uuid.UUID
	TestID     uuid.UUID
	Outcome    string
	ObservedAt time.Time
	RecordedAt time.Time
	Source     string
}

type evidenceRead struct {
	FromType    string
	FromID      uuid.UUID
	Qualifier   uuid.UUID
	ID          uuid.UUID
	Review      string
	ValidUntil  *time.Time
	CollectedAt time.Time
	FileRef     string
	ContentHash string
}

type exceptionRead struct {
	ID        uuid.UUID
	StartsAt  time.Time
	ExpiresAt time.Time
}

// Load reads the control of the tenant. now is the caller's instant: the
// exceptions already over at it are not read.
func (l *Loader) Load(ctx context.Context, tenantID, controlID uuid.UUID, now time.Time) (*app.Snapshot, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrForbidden
	}
	var snap *app.Snapshot
	run := func(tx *gorm.DB) error {
		var err error
		snap, err = load(ctx, tx, tenantID, controlID, now.UTC())
		return err
	}
	var err error
	if l.db.Dialector.Name() == "postgres" {
		// One snapshot for all five queries: a write that commits between two of them
		// is not half-seen.
		err = l.db.WithContext(ctx).Transaction(run, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	} else {
		err = run(l.db.WithContext(ctx))
	}
	if err != nil {
		var appErr *domain.AppError
		if errors.As(err, &appErr) {
			return nil, err
		}
		return nil, fmt.Errorf("controlstatus.Load: %w", err)
	}
	return snap, nil
}

func load(ctx context.Context, db *gorm.DB, tenantID, controlID uuid.UUID, now time.Time) (*app.Snapshot, error) {
	// Q1
	var ctl []controlRead
	if err := db.WithContext(ctx).Raw(`SELECT c.applicability, c.declared_status,
		(SELECT count(*) FROM canonical_edges e JOIN risks r ON r.id = e.from_id AND r.tenant_id = e.tenant_id
		 WHERE e.tenant_id = ? AND e.verb = ? AND e.from_type = ? AND e.to_type = ? AND e.to_id = c.id
		   AND r.tenant_id = ? AND r.deleted_at IS NULL AND r.status IN `+openRiskStatuses+`) AS open_risks
		FROM controls c WHERE c.tenant_id = ? AND c.id = ? AND c.deleted_at IS NULL`,
		tenantID, string(canonical.VerbMitigatedBy), string(canonical.TypeRisk), string(canonical.TypeControl),
		tenantID, tenantID, controlID).Scan(&ctl).Error; err != nil {
		return nil, err
	}
	if len(ctl) == 0 {
		return nil, notFound()
	}
	snap := &app.Snapshot{
		DeclaredStatus: canonical.DeclaredStatus(ctl[0].DeclaredStatus),
		OpenRiskCount:  ctl[0].OpenRisks,
		Inputs:         app.Inputs{Applicability: canonical.Applicability(ctl[0].Applicability)},
	}

	// Q2
	var tests []testRead
	if err := db.WithContext(ctx).Raw(`SELECT t.id, t.cadence
		FROM control_tests t JOIN control_implementations i ON i.id = t.implementation_id AND i.tenant_id = t.tenant_id
		WHERE t.tenant_id = ? AND t.deleted_at IS NULL
		  AND i.tenant_id = ? AND i.control_id = ? AND i.deleted_at IS NULL AND i.status <> 'retired'
		ORDER BY t.id ASC`, tenantID, tenantID, controlID).Scan(&tests).Error; err != nil {
		return nil, err
	}
	inputs := make([]app.TestInput, len(tests))
	byTest := make(map[uuid.UUID]int, len(tests))
	testIDs := make([]uuid.UUID, len(tests))
	for i, t := range tests {
		inputs[i] = app.TestInput{ID: t.ID, Cadence: t.Cadence}
		byTest[t.ID] = i
		testIDs[i] = t.ID
	}

	// Q3
	var resultIDs []uuid.UUID
	if len(tests) > 0 {
		var results []resultRead
		if err := db.WithContext(ctx).Raw(`SELECT x.id, x.test_id, x.outcome, x.observed_at, x.recorded_at, x.source FROM (
			SELECT r.id, r.test_id, r.outcome, r.observed_at, r.recorded_at, r.source,
			       ROW_NUMBER() OVER (PARTITION BY r.test_id ORDER BY r.observed_at DESC, r.recorded_at DESC, r.id DESC) AS rn
			FROM control_test_results r
			WHERE r.tenant_id = ? AND r.test_id IN (
				SELECT t.id FROM control_tests t JOIN control_implementations i ON i.id = t.implementation_id AND i.tenant_id = t.tenant_id
				WHERE t.tenant_id = ? AND t.deleted_at IS NULL
				  AND i.tenant_id = ? AND i.control_id = ? AND i.deleted_at IS NULL AND i.status <> 'retired')
			) x WHERE x.rn = 1`, tenantID, tenantID, tenantID, controlID).Scan(&results).Error; err != nil {
			return nil, err
		}
		for _, r := range results {
			i, ok := byTest[r.TestID]
			if !ok {
				continue
			}
			inputs[i].Latest = &app.ResultInput{
				ID: r.ID, Outcome: canonical.Outcome(r.Outcome), ObservedAt: r.ObservedAt.UTC(), RecordedAt: r.RecordedAt.UTC(),
				Source: canonical.Source(r.Source),
			}
			resultIDs = append(resultIDs, r.ID)
		}
	}

	// Q4: control-level evidence, and the evidence of each test's latest result.
	// The edge's qualifier is the result id (D5); an edge on an older result is
	// not read, so evidence on history counts for nothing.
	var evs []evidenceRead
	q := db.WithContext(ctx).Table(TableCanonicalEdges+" e").
		Select("e.from_type, e.from_id, e.qualifier, ev.id, ev.review, ev.valid_until, ev.collected_at, ev.file_ref, ev.content_hash").
		Joins("JOIN evidences ev ON ev.id = e.to_id AND ev.tenant_id = e.tenant_id AND ev.deleted_at IS NULL").
		Where("e.tenant_id = ? AND e.verb = ? AND e.to_type = ? AND ev.tenant_id = ?",
			tenantID, string(canonical.VerbEvidencedBy), string(canonical.TypeEvidence), tenantID)
	if len(resultIDs) > 0 {
		q = q.Where("((e.from_type = ? AND e.from_id = ?) OR (e.from_type = ? AND e.from_id IN ? AND e.qualifier IN ?))",
			string(canonical.TypeControl), controlID, string(canonical.TypeControlTest), testIDs, resultIDs)
	} else {
		q = q.Where("e.from_type = ? AND e.from_id = ?", string(canonical.TypeControl), controlID)
	}
	if err := q.Order("e.created_at ASC, e.id ASC").Scan(&evs).Error; err != nil {
		return nil, err
	}
	latestOf := make(map[uuid.UUID]uuid.UUID, len(resultIDs))
	for _, t := range inputs {
		if t.Latest != nil {
			latestOf[t.ID] = t.Latest.ID
		}
	}
	for _, e := range evs {
		in := app.EvidenceInput{
			ID: e.ID, Review: domain.EvidenceReview(e.Review), ValidUntil: e.ValidUntil, CollectedAt: e.CollectedAt.UTC(),
			FileRef: e.FileRef, ContentHash: e.ContentHash,
		}
		switch e.FromType {
		case string(canonical.TypeControl):
			snap.Inputs.ControlEvidence = append(snap.Inputs.ControlEvidence, in)
		case string(canonical.TypeControlTest):
			i, ok := byTest[e.FromID]
			if ok && latestOf[e.FromID] == e.Qualifier {
				inputs[i].Evidence = append(inputs[i].Evidence, in)
			}
		}
	}
	snap.Inputs.Tests = inputs

	// Q5: not revoked and not over at the caller's instant; one that has not
	// started is read too, Compute decides it is not active yet.
	var exs []exceptionRead
	if err := db.WithContext(ctx).Raw(`SELECT id, starts_at, expires_at FROM control_exceptions
		WHERE tenant_id = ? AND control_id = ? AND revoked_at IS NULL AND expires_at > ?
		ORDER BY expires_at ASC, id ASC`, tenantID, controlID, now).Scan(&exs).Error; err != nil {
		return nil, err
	}
	for _, x := range exs {
		snap.Inputs.Exceptions = append(snap.Inputs.Exceptions, domain.ControlException{
			ID: x.ID, TenantID: tenantID, ControlID: controlID, StartsAt: x.StartsAt.UTC(), ExpiresAt: x.ExpiresAt.UTC(),
		})
	}
	return snap, nil
}

// TableCanonicalEdges is the table of migration 0086 (the loader does not import
// infrastructure/controlchain, which imports this package).
const TableCanonicalEdges = "canonical_edges"
