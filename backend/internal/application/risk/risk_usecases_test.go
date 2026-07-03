// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package risk

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// Uses MockRiskRepository (test_utils_test.go) — the mockRiskRepo previously
// defined here did not implement the full domain.RiskRepository interface
// (missing BulkCreate, BulkUpdate, BulkDelete, UpdateScore, GetRiskScore,
// GetRisksByAssetID, GetHistory, CreateAuditEntry, GetBySource, GetByCVE),
// so this file never compiled under `go vet`/`go test`, independent of the
// Impact/Probability/Source type alignment. Consolidated onto the one
// complete mock instead of maintaining two parallel, drifting mocks.

// --- CreateRisk Tests ---

func TestCreateRisk_Success(t *testing.T) {
	repo := &MockRiskRepository{}
	uc := NewCreateRiskUseCase(repo)

	orgID := uuid.New()
	input := CreateRiskInput{
		Title:       "SQL Injection in login form",
		Description: "The login endpoint is vulnerable to SQLi",
		Impact:      8.0,
		Probability: 0.5,
		Owner:       "analyst@openrisk.io",
	}

	risk, err := uc.Execute(context.Background(), orgID, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if risk.Title != input.Title {
		t.Errorf("expected title %q, got %q", input.Title, risk.Title)
	}
	if risk.OrganizationID != orgID {
		t.Error("expected risk to be scoped to organization")
	}
	if risk.Score != 4.0 { // 8.0 * 0.5
		t.Errorf("expected score 4.0, got %.2f", risk.Score)
	}
	if risk.Status != domain.StatusDraft {
		t.Errorf("expected status DRAFT, got %s", risk.Status)
	}
}

func TestCreateRisk_ValidationError(t *testing.T) {
	repo := &MockRiskRepository{}
	uc := NewCreateRiskUseCase(repo)

	tests := []struct {
		name  string
		input CreateRiskInput
	}{
		{
			name:  "empty title",
			input: CreateRiskInput{Title: "", Impact: 5, Probability: 0.5},
		},
		{
			name:  "impact too low",
			input: CreateRiskInput{Title: "test", Impact: -1, Probability: 0.5},
		},
		{
			name:  "impact too high",
			input: CreateRiskInput{Title: "test", Impact: 11, Probability: 0.5},
		},
		{
			name:  "probability too low",
			input: CreateRiskInput{Title: "test", Impact: 5, Probability: -0.1},
		},
		{
			name:  "probability too high",
			input: CreateRiskInput{Title: "test", Impact: 5, Probability: 1.5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), uuid.New(), tt.input)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !errors.Is(err, domain.ErrValidation) {
				t.Errorf("expected ErrValidation, got %v", err)
			}
		})
	}
}

func TestCreateRisk_RepositoryError(t *testing.T) {
	repo := &MockRiskRepository{
		createFunc: func(ctx context.Context, risk *domain.Risk) error {
			return errors.New("db connection lost")
		},
	}
	uc := NewCreateRiskUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), CreateRiskInput{
		Title: "test", Impact: 5, Probability: 0.5,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetRisk Tests ---

func TestGetRisk_Success(t *testing.T) {
	orgID := uuid.New()
	riskID := uuid.New()

	repo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id uuid.UUID, tid uuid.UUID) (*domain.Risk, error) {
			if id == riskID && tid == orgID {
				return &domain.Risk{ID: riskID, OrganizationID: orgID, Title: "Test Risk"}, nil
			}
			return nil, nil
		},
	}

	uc := NewGetRiskUseCase(repo)
	risk, err := uc.Execute(context.Background(), orgID, riskID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if risk.Title != "Test Risk" {
		t.Errorf("expected 'Test Risk', got %q", risk.Title)
	}
}

func TestGetRisk_NotFound(t *testing.T) {
	repo := &MockRiskRepository{}
	uc := NewGetRiskUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetRisk_WrongTenant(t *testing.T) {
	orgA := uuid.New()
	orgB := uuid.New()
	riskID := uuid.New()

	// Risk belongs to orgA
	repo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id uuid.UUID, tid uuid.UUID) (*domain.Risk, error) {
			if id == riskID && tid == orgA {
				return &domain.Risk{ID: riskID, OrganizationID: orgA, Title: "OrgA Risk"}, nil
			}
			return nil, nil
		},
	}

	uc := NewGetRiskUseCase(repo)
	// Try to access from orgB → should not find
	_, err := uc.Execute(context.Background(), orgB, riskID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for cross-tenant access, got %v", err)
	}
}

// --- ListRisks Tests ---

func TestListRisks_Success(t *testing.T) {
	orgID := uuid.New()

	var risks []domain.Risk
	for i := 0; i < 5; i++ {
		risks = append(risks, domain.Risk{ID: uuid.New(), OrganizationID: orgID})
	}

	repo := &MockRiskRepository{
		listFunc: func(ctx context.Context, tid uuid.UUID, query domain.RiskQuery) (*domain.PaginatedResult[domain.Risk], error) {
			return &domain.PaginatedResult[domain.Risk]{Data: risks, Total: int64(len(risks))}, nil
		},
	}

	uc := NewListRisksUseCase(repo)
	result, err := uc.Execute(context.Background(), orgID, domain.NewRiskQuery())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 5 {
		t.Errorf("expected 5 risks, got %d", result.Total)
	}
}

func TestListRisks_EmptyOrg(t *testing.T) {
	repo := &MockRiskRepository{}
	uc := NewListRisksUseCase(repo)

	result, err := uc.Execute(context.Background(), uuid.New(), domain.NewRiskQuery())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("expected 0 risks, got %d", result.Total)
	}
}

// --- UpdateRisk Tests ---

func TestUpdateRisk_Success(t *testing.T) {
	orgID := uuid.New()
	riskID := uuid.New()
	existing := &domain.Risk{
		ID: riskID, OrganizationID: orgID,
		Title: "Old Title", Impact: 2, Probability: 0.5,
	}

	repo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id uuid.UUID, tid uuid.UUID) (*domain.Risk, error) {
			if id == riskID && tid == orgID {
				return existing, nil
			}
			return nil, nil
		},
	}

	uc := NewUpdateRiskUseCase(repo)
	newTitle := "Updated Title"
	newImpact := 4.0
	risk, err := uc.Execute(context.Background(), orgID, riskID, UpdateRiskInput{
		Title:  &newTitle,
		Impact: &newImpact,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if risk.Title != "Updated Title" {
		t.Errorf("expected 'Updated Title', got %q", risk.Title)
	}
	if risk.Score != 2.0 { // 4.0 * 0.5
		t.Errorf("expected score 2.0, got %.2f", risk.Score)
	}
}

func TestUpdateRisk_NotFound(t *testing.T) {
	repo := &MockRiskRepository{}
	uc := NewUpdateRiskUseCase(repo)

	title := "test"
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), UpdateRiskInput{Title: &title})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateRisk_ValidationError(t *testing.T) {
	orgID := uuid.New()
	riskID := uuid.New()
	existing := &domain.Risk{
		ID: riskID, OrganizationID: orgID,
		Title: "Test", Impact: 2, Probability: 0.5,
	}

	repo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id uuid.UUID, tid uuid.UUID) (*domain.Risk, error) {
			return existing, nil
		},
	}
	uc := NewUpdateRiskUseCase(repo)

	badImpact := 15.0 // ERD bound is [0,10] — 15 is out of range
	_, err := uc.Execute(context.Background(), orgID, riskID, UpdateRiskInput{Impact: &badImpact})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

// --- DeleteRisk Tests ---

func TestDeleteRisk_Success(t *testing.T) {
	orgID := uuid.New()
	riskID := uuid.New()
	existing := &domain.Risk{ID: riskID, OrganizationID: orgID}
	deleted := false

	repo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id uuid.UUID, tid uuid.UUID) (*domain.Risk, error) {
			if deleted {
				return nil, nil
			}
			return existing, nil
		},
		deleteFunc: func(ctx context.Context, id uuid.UUID, tid uuid.UUID) error {
			deleted = true
			return nil
		},
	}

	uc := NewDeleteRiskUseCase(repo)
	err := uc.Execute(context.Background(), orgID, riskID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleted {
		t.Error("expected risk to be deleted")
	}
}

func TestDeleteRisk_NotFound(t *testing.T) {
	repo := &MockRiskRepository{}
	uc := NewDeleteRiskUseCase(repo)

	err := uc.Execute(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteRisk_WrongTenant(t *testing.T) {
	orgA := uuid.New()
	orgB := uuid.New()
	riskID := uuid.New()
	existing := &domain.Risk{ID: riskID, OrganizationID: orgA}

	repo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id uuid.UUID, tid uuid.UUID) (*domain.Risk, error) {
			if id == riskID && tid == orgA {
				return existing, nil
			}
			return nil, nil
		},
	}

	uc := NewDeleteRiskUseCase(repo)
	err := uc.Execute(context.Background(), orgB, riskID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for cross-tenant delete, got %v", err)
	}
}
