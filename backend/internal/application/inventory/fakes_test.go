// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package inventory

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// fakeRepo keeps every tenant apart, as the GORM repository does.
type fakeRepo struct {
	mu   sync.Mutex
	invs map[uuid.UUID]*domain.HostInventory
	pkgs map[uuid.UUID][]domain.HostPackage
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{invs: map[uuid.UUID]*domain.HostInventory{}, pkgs: map[uuid.UUID][]domain.HostPackage{}}
}

func (r *fakeRepo) Replace(_ context.Context, inv *domain.HostInventory, pkgs []domain.HostPackageInput) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.invs {
		if e.TenantID == inv.TenantID && e.HostKey == inv.HostKey {
			inv.ID = e.ID
		}
	}
	if inv.ID == uuid.Nil {
		inv.ID = uuid.New()
	}
	cp := *inv
	r.invs[inv.ID] = &cp
	rows := make([]domain.HostPackage, 0, len(pkgs))
	for _, p := range pkgs {
		rows = append(rows, domain.HostPackage{ID: uuid.New(), TenantID: inv.TenantID, InventoryID: inv.ID,
			Name: p.Name, Version: p.Version, Source: p.Source, Publisher: p.Publisher, Arch: p.Arch})
	}
	r.pkgs[inv.ID] = rows
	return nil
}

func (r *fakeRepo) Get(_ context.Context, tenantID, id uuid.UUID) (*domain.HostInventory, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.invs[id]; ok && e.TenantID == tenantID {
		cp := *e
		return &cp, nil
	}
	return nil, nil
}

func (r *fakeRepo) List(_ context.Context, tenantID uuid.UUID) ([]domain.HostInventory, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.HostInventory
	for _, e := range r.invs {
		if e.TenantID == tenantID {
			out = append(out, *e)
		}
	}
	return out, nil
}

func (r *fakeRepo) ListByAsset(ctx context.Context, tenantID, assetID uuid.UUID) ([]domain.HostInventory, error) {
	all, _ := r.List(ctx, tenantID)
	var out []domain.HostInventory
	for _, e := range all {
		if e.AssetID != nil && *e.AssetID == assetID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (r *fakeRepo) Packages(_ context.Context, tenantID, id uuid.UUID) ([]domain.HostPackage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.invs[id]; ok && e.TenantID == tenantID {
		return append([]domain.HostPackage(nil), r.pkgs[id]...), nil
	}
	return nil, nil
}

func (r *fakeRepo) MarkMatched(_ context.Context, tenantID, id uuid.UUID, at time.Time, code string, uncheckable int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.invs[id]; ok && e.TenantID == tenantID {
		e.MatchedAt, e.MatchError, e.UncheckableCount = &at, code, uncheckable
	}
	return nil
}

func (r *fakeRepo) TenantsWithInventories(context.Context) ([]uuid.UUID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	seen := map[uuid.UUID]bool{}
	var out []uuid.UUID
	for _, e := range r.invs {
		if !seen[e.TenantID] {
			seen[e.TenantID] = true
			out = append(out, e.TenantID)
		}
	}
	return out, nil
}

type fakeAssets struct{ byTenant map[uuid.UUID][]domain.Asset }

func (f fakeAssets) List(_ context.Context, t uuid.UUID) ([]domain.Asset, error) {
	return f.byTenant[t], nil
}

type recordingMatcher struct {
	calls []uuid.UUID
}

func (m *recordingMatcher) MatchInventory(_ context.Context, _, id uuid.UUID) error {
	m.calls = append(m.calls, id)
	return nil
}

func syncNow(uc *StoreInventoryUseCase) *StoreInventoryUseCase {
	uc.async = func(f func()) { f() }
	return uc
}
