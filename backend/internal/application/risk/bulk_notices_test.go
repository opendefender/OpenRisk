// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bulkapp "github.com/opendefender/openrisk/internal/application/bulk"
	"github.com/opendefender/openrisk/internal/domain"
)

// #599 — the post-commit hook that tells a risk's owner a governed bulk status
// change moved their risk (#757).

// tenantRisks is a tenant-scoped lookup: a risk is returned only for the
// tenant it belongs to, as GormRiskRepository.GetByID does.
type tenantRisks struct {
	byID    map[uuid.UUID]*domain.Risk
	tenants []uuid.UUID // the tenant each call was made with
}

func (l *tenantRisks) GetByID(_ context.Context, id, tenantID uuid.UUID) (*domain.Risk, error) {
	l.tenants = append(l.tenants, tenantID)
	r, ok := l.byID[id]
	if !ok || r.TenantID != tenantID {
		return nil, domain.NewNotFoundError("risk", id)
	}
	return r, nil
}

func ownedRisk(tenant uuid.UUID, title string) (*domain.Risk, uuid.UUID) {
	owner := uuid.New()
	r := &domain.Risk{ID: uuid.New(), TenantID: tenant, Title: title}
	r.OwnerID = &owner
	return r, owner
}

func stateMove(id uuid.UUID, from, to domain.RiskState) domain.BulkMutation {
	return domain.BulkMutation{
		ID:     id,
		Before: map[string]interface{}{"lifecycle_state": string(from)},
		After:  map[string]interface{}{"lifecycle_state": string(to)},
	}
}

var closeChange = domain.BulkChange{Action: domain.BulkActionChangeStatus, Status: "closed"}

func TestBulkStateNotices_Success(t *testing.T) {
	tenant := uuid.New()
	r, owner := ownedRisk(tenant, "Ransomware")
	lookup := &tenantRisks{byID: map[uuid.UUID]*domain.Risk{r.ID: r}}
	notices := &recordedNotices{}

	BulkStateNotices(lookup, notices)(context.Background(),
		bulkapp.Caller{TenantID: tenant, UserID: uuid.New()}, closeChange,
		[]domain.BulkMutation{stateMove(r.ID, domain.StateIdentified, domain.StateClosed)})

	require.Len(t, notices.got, 1)
	n := notices.got[0]
	assert.Equal(t, owner, n.UserID)
	assert.Equal(t, tenant, n.TenantID)
	assert.Equal(t, r.ID, *n.ResourceID)
	assert.Equal(t, "Ransomware", n.ResourceTitle)
	assert.Equal(t, domain.NotificationTypeRiskResolved, n.Type, "closing a risk is a resolution")
	assert.Equal(t, []uuid.UUID{tenant}, lookup.tenants, "the owner is read with the caller's tenant")
}

// A risk that cannot be read is skipped, and the rest of the batch is still
// announced: the change has already committed, and a notice is best-effort.
func TestBulkStateNotices_NotFound_SkipsAndContinues(t *testing.T) {
	tenant := uuid.New()
	r, owner := ownedRisk(tenant, "kept")
	lookup := &tenantRisks{byID: map[uuid.UUID]*domain.Risk{r.ID: r}}
	notices := &recordedNotices{}

	BulkStateNotices(lookup, notices)(context.Background(),
		bulkapp.Caller{TenantID: tenant, UserID: uuid.New()}, closeChange,
		[]domain.BulkMutation{
			stateMove(uuid.New(), domain.StateIdentified, domain.StateClosed), // gone
			stateMove(r.ID, domain.StateIdentified, domain.StateClosed),
		})

	require.Len(t, notices.got, 1)
	assert.Equal(t, owner, notices.got[0].UserID)
}

// Another tenant's risk is never read and never announced, and a zero tenant
// announces nothing.
func TestBulkStateNotices_Unauthorized_NoCrossTenantNotice(t *testing.T) {
	mine, theirs := uuid.New(), uuid.New()
	foreign, _ := ownedRisk(theirs, "foreign")
	lookup := &tenantRisks{byID: map[uuid.UUID]*domain.Risk{foreign.ID: foreign}}
	notices := &recordedNotices{}
	hook := BulkStateNotices(lookup, notices)
	moves := []domain.BulkMutation{stateMove(foreign.ID, domain.StateIdentified, domain.StateClosed)}

	hook(context.Background(), bulkapp.Caller{TenantID: mine, UserID: uuid.New()}, closeChange, moves)
	hook(context.Background(), bulkapp.Caller{TenantID: uuid.Nil, UserID: uuid.New()}, closeChange, moves)

	assert.Empty(t, notices.got)
	assert.Equal(t, []uuid.UUID{mine, uuid.Nil}, lookup.tenants, "every read carries the caller's tenant, never the risk's")
}

// Nothing to say: an unchanged state, a tag action, or the owner moving their
// own risk.
func TestBulkStateNotices_SilentWhenThereIsNoNews(t *testing.T) {
	tenant := uuid.New()
	r, owner := ownedRisk(tenant, "r")
	lookup := &tenantRisks{byID: map[uuid.UUID]*domain.Risk{r.ID: r}}
	notices := &recordedNotices{}
	hook := BulkStateNotices(lookup, notices)
	ctx := context.Background()

	hook(ctx, bulkapp.Caller{TenantID: tenant, UserID: uuid.New()}, closeChange,
		[]domain.BulkMutation{stateMove(r.ID, domain.StateClosed, domain.StateClosed)})
	hook(ctx, bulkapp.Caller{TenantID: tenant, UserID: uuid.New()},
		domain.BulkChange{Action: domain.BulkActionAddTags, Tags: []string{"pci"}},
		[]domain.BulkMutation{stateMove(r.ID, domain.StateIdentified, domain.StateClosed)})
	hook(ctx, bulkapp.Caller{TenantID: tenant, UserID: owner}, closeChange,
		[]domain.BulkMutation{stateMove(r.ID, domain.StateIdentified, domain.StateClosed)})

	assert.Empty(t, notices.got)
	assert.Len(t, lookup.tenants, 1, "an unchanged state and a tag action do not even read the risk")
}

func TestBulkStateNotices_NilDependenciesAreSafe(t *testing.T) {
	moves := []domain.BulkMutation{stateMove(uuid.New(), domain.StateIdentified, domain.StateClosed)}
	caller := bulkapp.Caller{TenantID: uuid.New(), UserID: uuid.New()}
	assert.NotPanics(t, func() {
		BulkStateNotices(nil, &recordedNotices{})(context.Background(), caller, closeChange, moves)
		BulkStateNotices(&tenantRisks{}, nil)(context.Background(), caller, closeChange, moves)
	})
}
