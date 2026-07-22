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

// TestCreateRisk_RejectsProbabilityOutOfRange pins the ERD bound
// probability ∈ [0,1] (numeric(5,3)) — 1.5 must be rejected as ErrValidation.
func TestCreateRisk_RejectsProbabilityOutOfRange(t *testing.T) {
	repo := &MockRiskRepository{}
	uc := NewCreateRiskUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), CreateRiskInput{
		Title:       "test",
		Impact:      5,
		Probability: 1.5,
	})

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

// TestCreateRisk_RejectsInvalidSource pins the domain.RiskSource enum
// contract — a value outside the 6 known constants must be rejected as
// ErrValidation, not silently accepted or causing a compile-time issue.
func TestCreateRisk_RejectsInvalidSource(t *testing.T) {
	repo := &MockRiskRepository{}
	uc := NewCreateRiskUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), CreateRiskInput{
		Title:       "test",
		Impact:      5,
		Probability: 0.5,
		Source:      "not_a_real_source",
	})

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}
