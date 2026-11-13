// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	bulkapp "github.com/opendefender/openrisk/internal/application/bulk"
	"github.com/opendefender/openrisk/internal/domain"
)

// #599 against real SQL: the risk register on the shared engine.
//
// The engine and the store run together here, so criterion 1 (the preview
// writes nothing) and criterion 2 (a moved selection is refused with 409) are
// proven on a real table rather than on a fake that keeps its promises by
// construction.

type riskBulkJournal struct{ events []*domain.AuditEvent }

func (j *riskBulkJournal) Append(_ context.Context, e *domain.AuditEvent) error {
	j.events = append(j.events, e)
	return nil
}

func setupRiskBulkEngine(t *testing.T) (*bulkapp.Engine, *riskBulkJournal, *gorm.DB) {
	t.Helper()
	_, db := setupRiskBulkRepo(t)
	j := &riskBulkJournal{}
	return bulkapp.New(NewRiskBulkStore(db), j), j, db
}

// riskRowDump reads every column of a risk as text, so "unchanged" means the
// whole row and not just the fields the preview happens to show.
func riskRowDump(t *testing.T, db *gorm.DB, id uuid.UUID) map[string]string {
	t.Helper()
	rows, err := db.Raw(`SELECT * FROM risks WHERE id = ?`, id.String()).Rows()
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	cols, err := rows.Columns()
	require.NoError(t, err)
	require.True(t, rows.Next(), "risk %s not found", id)
	vals := make([]interface{}, len(cols))
	ptrs := make([]interface{}, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	require.NoError(t, rows.Scan(ptrs...))
	out := make(map[string]string, len(cols))
	for i, c := range cols {
		out[c] = fmt.Sprintf("%v", vals[i])
	}
	return out
}

func historyCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM risk_histories`).Scan(&n).Error)
	return n
}

func riskCaller(tenant uuid.UUID) bulkapp.Caller {
	return bulkapp.Caller{TenantID: tenant, UserID: uuid.New()}
}

// ---------------------------------------------------------------------------
// Criterion 1 — the preview returns the impact and writes nothing
// ---------------------------------------------------------------------------

func TestRiskBulkStore_Preview_Success_WritesNothing(t *testing.T) {
	e, j, db := setupRiskBulkEngine(t)
	ctx := context.Background()
	tenant := uuid.New()
	a := seedBulkRisk(t, db, tenant, "open")
	b := seedBulkRisk(t, db, tenant, "closed")

	beforeA, beforeB := riskRowDump(t, db, a), riskRowDump(t, db, b)
	histBefore := historyCount(t, db)

	for _, change := range []domain.BulkChange{
		{Action: domain.BulkActionChangeStatus, Status: "closed"},
		{Action: domain.BulkActionAddTags, Tags: []string{"pci"}},
		{Action: domain.BulkActionRemoveTags, Tags: []string{"pci"}},
		{Action: domain.BulkActionDelete},
	} {
		p, err := e.Preview(ctx, riskCaller(tenant), bulkapp.Request{IDs: []uuid.UUID{a, b}, Change: change})
		require.NoError(t, err, change.Action)
		assert.Equal(t, 2, p.Requested, change.Action)
		assert.Equal(t, 2, p.Found, change.Action)
		assert.Len(t, p.Fingerprint, 64, change.Action)
		assert.Equal(t, change.Action, p.Action)
	}

	assert.Equal(t, beforeA, riskRowDump(t, db, a), "a preview must leave the row byte-identical")
	assert.Equal(t, beforeB, riskRowDump(t, db, b))
	assert.Equal(t, int64(2), riskCount(t, db), "a delete preview deletes nothing")
	assert.Equal(t, histBefore, historyCount(t, db), "a preview writes no history row")
	assert.Empty(t, j.events, "a preview writes no audit entry")
}

// The preview counts honestly: a risk already in the target state is found but
// not affected, and the sample shows the derived fields that will move.
func TestRiskBulkStore_Preview_ReportsTheRealImpact(t *testing.T) {
	e, _, db := setupRiskBulkEngine(t)
	tenant := uuid.New()
	open := seedBulkRisk(t, db, tenant, "open")

	p, err := e.Preview(context.Background(), riskCaller(tenant), bulkapp.Request{
		IDs:    []uuid.UUID{open, uuid.New()},
		Change: domain.BulkChange{Action: domain.BulkActionChangeStatus, Status: "closed"},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, p.Requested)
	assert.Equal(t, 1, p.Found)
	assert.Len(t, p.Missing, 1, "an absent id is shown before the user commits")
	assert.Equal(t, 1, p.Affected)
	require.Len(t, p.Sample, 1)
	assert.Equal(t, "seed", p.Sample[0].Label, "the preview names risks, not ids")
	assert.Contains(t, p.Sample[0].ChangedFields, "lifecycle_state")
}

// ---------------------------------------------------------------------------
// Apply — the write matches the preview, all or none
// ---------------------------------------------------------------------------

func TestRiskBulkStore_Apply_Success_MatchesThePreview(t *testing.T) {
	e, j, db := setupRiskBulkEngine(t)
	ctx := context.Background()
	tenant := uuid.New()
	a := seedBulkRisk(t, db, tenant, "open")
	b := seedBulkRisk(t, db, tenant, "open")
	change := domain.BulkChange{Action: domain.BulkActionChangeStatus, Status: "closed", Justification: "audit 2026"}

	p, err := e.Preview(ctx, riskCaller(tenant), bulkapp.Request{IDs: []uuid.UUID{a, b}, Change: change})
	require.NoError(t, err)

	res, err := e.Apply(ctx, riskCaller(tenant), bulkapp.Request{IDs: []uuid.UUID{a, b}, Change: change, Fingerprint: p.Fingerprint})
	require.NoError(t, err)
	assert.Equal(t, 2, res.Applied)
	assert.Equal(t, 2, res.Audited)

	assert.Equal(t, "closed", statusOf(t, db, a))
	assert.Equal(t, "closed", statusOf(t, db, b))

	// What the trail records as "after" is what the preview promised.
	byID := map[string]*domain.AuditEvent{}
	for _, ev := range j.events {
		byID[ev.EntityID] = ev
		assert.Equal(t, "risk", ev.EntityType)
		assert.Contains(t, ev.Summary, "audit 2026")
	}
	for _, s := range p.Sample {
		ev := byID[s.ID.String()]
		require.NotNil(t, ev)
		assert.Equal(t, fmt.Sprintf("%v", s.After), fmt.Sprintf("%v", map[string]interface{}(ev.After)),
			"the preview must show exactly what the write produced")
	}
}

func TestRiskBulkStore_Apply_Tags(t *testing.T) {
	e, _, db := setupRiskBulkEngine(t)
	ctx := context.Background()
	tenant := uuid.New()
	a := seedBulkRisk(t, db, tenant, "open")

	_, err := e.Apply(ctx, riskCaller(tenant), bulkapp.Request{IDs: []uuid.UUID{a},
		Change: domain.BulkChange{Action: domain.BulkActionAddTags, Tags: []string{"pci", "iso"}}})
	require.NoError(t, err)
	rows, err := NewRiskBulkStore(db).LoadBulk(ctx, tenant, []uuid.UUID{a})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, []string{"iso", "pci"}, rows[0].Snapshot["tags"])

	_, err = e.Apply(ctx, riskCaller(tenant), bulkapp.Request{IDs: []uuid.UUID{a},
		Change: domain.BulkChange{Action: domain.BulkActionRemoveTags, Tags: []string{"pci"}}})
	require.NoError(t, err)
	rows, err = NewRiskBulkStore(db).LoadBulk(ctx, tenant, []uuid.UUID{a})
	require.NoError(t, err)
	assert.Equal(t, []string{"iso"}, rows[0].Snapshot["tags"])
}

// ---------------------------------------------------------------------------
// Criterion 2 — a selection that moved is refused with 409, nothing written
// ---------------------------------------------------------------------------

func TestRiskBulkStore_Apply_Conflict_WhenTheSelectionMoved(t *testing.T) {
	e, j, db := setupRiskBulkEngine(t)
	ctx := context.Background()
	tenant := uuid.New()
	a := seedBulkRisk(t, db, tenant, "open")
	b := seedBulkRisk(t, db, tenant, "open")

	for _, change := range []domain.BulkChange{
		{Action: domain.BulkActionChangeStatus, Status: "closed"},
		{Action: domain.BulkActionDelete},
	} {
		p, err := e.Preview(ctx, riskCaller(tenant), bulkapp.Request{IDs: []uuid.UUID{a, b}, Change: change})
		require.NoError(t, err)

		// Someone else moves one risk between the preview and the confirmation.
		require.NoError(t, db.Exec(`UPDATE risks SET status = 'mitigated', lifecycle_state = 'mitigated' WHERE id = ?`, b.String()).Error)
		before := riskRowDump(t, db, a)

		_, err = e.Apply(ctx, riskCaller(tenant), bulkapp.Request{IDs: []uuid.UUID{a, b}, Change: change, Fingerprint: p.Fingerprint})
		require.Error(t, err, change.Action)
		assert.True(t, errors.Is(err, domain.ErrConflict), "want 409, got %v", err)
		assert.Equal(t, fiberConflict, domain.HTTPStatusFromError(err))

		assert.Equal(t, before, riskRowDump(t, db, a), "a refused apply writes nothing")
		assert.Equal(t, int64(2), riskCount(t, db))
		assert.Empty(t, j.events)

		require.NoError(t, db.Exec(`UPDATE risks SET status = 'open', lifecycle_state = 'identified' WHERE id = ?`, b.String()).Error)
	}
}

const fiberConflict = 409

// ---------------------------------------------------------------------------
// NotFound / Unauthorized / tenant isolation
// ---------------------------------------------------------------------------

func TestRiskBulkStore_Apply_NotFound_WritesNothing(t *testing.T) {
	e, j, db := setupRiskBulkEngine(t)
	tenant := uuid.New()
	a := seedBulkRisk(t, db, tenant, "open")

	_, err := e.Apply(context.Background(), riskCaller(tenant), bulkapp.Request{IDs: []uuid.UUID{a, uuid.New()},
		Change: domain.BulkChange{Action: domain.BulkActionChangeStatus, Status: "closed"}})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
	assert.Equal(t, "open", statusOf(t, db, a), "one absent id rolls back the batch")
	assert.Empty(t, j.events)
}

func TestRiskBulkStore_Apply_Unauthorized_ZeroTenant(t *testing.T) {
	e, _, db := setupRiskBulkEngine(t)
	a := seedBulkRisk(t, db, uuid.New(), "open")

	for _, action := range []domain.BulkAction{domain.BulkActionChangeStatus, domain.BulkActionDelete} {
		_, err := e.Apply(context.Background(), bulkapp.Caller{UserID: uuid.New()}, bulkapp.Request{
			IDs: []uuid.UUID{a}, Change: domain.BulkChange{Action: action, Status: "closed"}})
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrForbidden))
	}
	assert.Equal(t, "open", statusOf(t, db, a))
	assert.Equal(t, int64(1), riskCount(t, db))
}

func TestRiskBulkStore_ForeignTenant_IsInvisibleAndUntouched(t *testing.T) {
	e, _, db := setupRiskBulkEngine(t)
	ctx := context.Background()
	mine, theirs := uuid.New(), uuid.New()
	foreign := seedBulkRisk(t, db, theirs, "open")

	p, err := e.Preview(ctx, riskCaller(mine), bulkapp.Request{IDs: []uuid.UUID{foreign},
		Change: domain.BulkChange{Action: domain.BulkActionDelete}})
	require.NoError(t, err)
	assert.Equal(t, 0, p.Found)
	assert.Empty(t, p.Sample, "another tenant's risk is never described")

	_, err = e.Apply(ctx, riskCaller(mine), bulkapp.Request{IDs: []uuid.UUID{foreign},
		Change: domain.BulkChange{Action: domain.BulkActionDelete}})
	assert.True(t, errors.Is(err, domain.ErrNotFound))
	_, err = e.Apply(ctx, riskCaller(mine), bulkapp.Request{IDs: []uuid.UUID{foreign},
		Change: domain.BulkChange{Action: domain.BulkActionChangeStatus, Status: "closed"}})
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	assert.Equal(t, "open", statusOf(t, db, foreign))
	assert.Equal(t, int64(1), riskCount(t, db))
}

func TestRiskBulkStore_Delete_Success_AuditsWhatWent(t *testing.T) {
	e, j, db := setupRiskBulkEngine(t)
	tenant := uuid.New()
	a := seedBulkRisk(t, db, tenant, "open")

	res, err := e.Apply(context.Background(), riskCaller(tenant), bulkapp.Request{IDs: []uuid.UUID{a},
		Change: domain.BulkChange{Action: domain.BulkActionDelete}})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Applied)
	assert.Equal(t, int64(0), riskCount(t, db))
	require.Len(t, j.events, 1)
	assert.Equal(t, domain.AuditActionDelete, j.events[0].Action)
	assert.Equal(t, "open", j.events[0].Before["status"], "a deletion records what went")
}

func TestRiskBulkStore_RefusesBulkResidualAcceptance(t *testing.T) {
	e, _, db := setupRiskBulkEngine(t)
	tenant := uuid.New()
	a := seedBulkRisk(t, db, tenant, "open")

	_, err := e.Apply(context.Background(), riskCaller(tenant), bulkapp.Request{IDs: []uuid.UUID{a},
		Change: domain.BulkChange{Action: domain.BulkActionChangeStatus, Status: "accepted"}})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrValidation))
	assert.Equal(t, "open", statusOf(t, db, a))
}
