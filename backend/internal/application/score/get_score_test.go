// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package score

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/scoring"
)

// Stubs record the tenant they were asked about, so every test also proves the
// use case never queries a source for anyone but the caller's tenant.

type stubRiskCounts struct {
	counts map[string]int
	asked  uuid.UUID
}

func (s *stubRiskCounts) CountRisksByCriticality(_ context.Context, tenantID uuid.UUID) (map[string]int, error) {
	s.asked = tenantID
	return s.counts, nil
}

type stubCompliance struct {
	total, gaps int
	asked       uuid.UUID
}

func (s *stubCompliance) ControlTotals(_ context.Context, tenantID uuid.UUID) (int, int, error) {
	s.asked = tenantID
	return s.total, s.gaps, nil
}

type stubVulnStats struct {
	stats *domain.VulnStats
	asked uuid.UUID
}

func (s *stubVulnStats) Stats(_ context.Context, tenantID uuid.UUID) (*domain.VulnStats, error) {
	s.asked = tenantID
	return s.stats, nil
}

type stubIncidents struct{ asked uuid.UUID }

func (s *stubIncidents) OpenIncidentCounts(_ context.Context, tenantID uuid.UUID) (int, int, error) {
	s.asked = tenantID
	return 0, 0, nil
}

type stubRiskReader struct{}

func (stubRiskReader) GetByID(_ context.Context, id, _ uuid.UUID) (*domain.Risk, error) {
	return nil, domain.NewNotFoundError("risk", id)
}

func emptyTenantUseCase() (*UseCase, *stubRiskCounts, *stubCompliance, *stubVulnStats, *stubIncidents) {
	rc := &stubRiskCounts{counts: map[string]int{}}
	cc := &stubCompliance{}
	vs := &stubVulnStats{stats: &domain.VulnStats{BySeverity: map[string]int64{}}}
	ic := &stubIncidents{}
	uc := New().WithRiskCounts(rc).WithCompliance(cc).WithVulnStats(vs).WithIncidents(ic)
	return uc, rc, cc, vs, ic
}

func TestGetScore_Success(t *testing.T) {
	uc, rc, cc, vs, ic := emptyTenantUseCase()
	rc.counts = map[string]int{"critical": 2, "high": 1, "low": 3}
	tenant := uuid.New()

	r, err := uc.Execute(context.Background(), tenant, scoring.ScopeTenant, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Measured || r.Value <= 0 {
		t.Fatalf("a register with critical risks must be measured and non-zero: %+v", r)
	}
	for name, asked := range map[string]uuid.UUID{"risks": rc.asked, "compliance": cc.asked, "vulns": vs.asked, "incidents": ic.asked} {
		if asked != tenant {
			t.Errorf("%s source queried for tenant %s, want %s", name, asked, tenant)
		}
	}
}

// #287 — the empty tenant: every source answers, and every answer is "nothing".
func TestGetScore_EmptyTenantIsNotMeasured(t *testing.T) {
	uc, _, _, _, _ := emptyTenantUseCase()

	r, err := uc.Execute(context.Background(), uuid.New(), scoring.ScopeTenant, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Measured {
		t.Fatalf("empty tenant reported as measured with value %v", r.Value)
	}
	if r.ReasonI18nKey != scoring.ReasonNoData {
		t.Errorf("reason = %q", r.ReasonI18nKey)
	}
	for _, f := range r.Breakdown {
		if f.Available && (f.Key == scoring.FactorRiskExposure || f.Key == scoring.FactorVulnerabilityPressure) {
			t.Errorf("%s scored from zero records", f.Key)
		}
	}
}

// Zero vulnerabilities on record is "never imported", not "clean": the factor
// must not pull a measured tenant's score towards zero.
func TestGetScore_NoVulnRecordsIsNotAMeasurement(t *testing.T) {
	uc, rc, _, _, _ := emptyTenantUseCase()
	rc.counts = map[string]int{"low": 1}

	r, err := uc.Execute(context.Background(), uuid.New(), scoring.ScopeTenant, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range r.Breakdown {
		if f.Key == scoring.FactorVulnerabilityPressure && f.Available {
			t.Error("vulnerability_pressure available with zero vulnerabilities on record")
		}
	}
}

func TestGetScore_NotFound(t *testing.T) {
	uc := New().WithRisk(stubRiskReader{})
	_, err := uc.Execute(context.Background(), uuid.New(), scoring.ScopeRisk, uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetScore_Unauthorized(t *testing.T) {
	uc, _, _, _, _ := emptyTenantUseCase()
	_, err := uc.Execute(context.Background(), uuid.Nil, scoring.ScopeTenant, uuid.Nil)
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("a request with no tenant must be refused, got %v", err)
	}
}
