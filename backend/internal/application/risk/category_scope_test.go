// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// fakeCategories holds categories per tenant and answers like
// GormRiskCategoryRepository: nil, nil for a row that is not the tenant's.
type fakeCategories struct{ rows map[[2]uuid.UUID]bool }

func (f fakeCategories) GetByID(_ context.Context, id, tenantID uuid.UUID) (*domain.RiskCategory, error) {
	if f.rows[[2]uuid.UUID{tenantID, id}] {
		return &domain.RiskCategory{ID: id, TenantID: tenantID}, nil
	}
	return nil, nil
}

// TestOwnsCategory covers the three answers the gate must give.
func TestOwnsCategory(t *testing.T) {
	tenantA, tenantB, cat := uuid.New(), uuid.New(), uuid.New()
	store := fakeCategories{rows: map[[2]uuid.UUID]bool{{tenantB, cat}: true}}
	ctx := context.Background()

	// Success: the tenant's own category, and no category at all.
	if err := ownsCategory(ctx, store, tenantB, &cat); err != nil {
		t.Fatalf("own category refused: %v", err)
	}
	if err := ownsCategory(ctx, store, tenantA, nil); err != nil {
		t.Fatalf("an unclassified risk must pass: %v", err)
	}
	// NotFound: another tenant's category, and a random one, answer the same.
	random := uuid.New()
	for name, id := range map[string]*uuid.UUID{"foreign": &cat, "random": &random} {
		if err := ownsCategory(ctx, store, tenantA, id); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("%s category: want ErrNotFound, got %v", name, err)
		}
	}
}

// TestCreateRisk_CrossTenantCategoryIsNotFound: creating a risk that names
// another tenant's category is refused before anything is written.
func TestCreateRisk_CrossTenantCategoryIsNotFound(t *testing.T) {
	tenantA, tenantB, cat := uuid.New(), uuid.New(), uuid.New()
	created := false
	repo := &MockRiskRepository{createFunc: func(context.Context, *domain.Risk) error { created = true; return nil }}
	uc := NewCreateRiskUseCase(repo).WithCategories(fakeCategories{rows: map[[2]uuid.UUID]bool{{tenantB, cat}: true}})

	_, err := uc.Execute(context.Background(), tenantA, CreateRiskInput{
		Title: "x", Impact: 5, Probability: 0.5, CategoryID: &cat,
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if created {
		t.Fatal("the risk was written despite the foreign category")
	}
}

// TestUpdateRisk_CrossTenantCategoryIsNotFound: the same gate on update, and the
// risk is left as it was.
func TestUpdateRisk_CrossTenantCategoryIsNotFound(t *testing.T) {
	tenantA, tenantB, cat, riskID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	updated := false
	repo := &MockRiskRepository{
		getByIDFunc: func(_ context.Context, id, tenantID uuid.UUID) (*domain.Risk, error) {
			return &domain.Risk{ID: id, TenantID: tenantID, Title: "x", Impact: 5, Probability: 0.5}, nil
		},
		updateFunc: func(context.Context, *domain.Risk) error { updated = true; return nil },
	}
	uc := NewUpdateRiskUseCase(repo).WithCategories(fakeCategories{rows: map[[2]uuid.UUID]bool{{tenantB, cat}: true}})

	_, err := uc.Execute(context.Background(), tenantA, riskID, UpdateRiskInput{
		Category: domain.NullableUUID{Present: true, Value: &cat},
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if updated {
		t.Fatal("the risk was updated despite the foreign category")
	}
}
