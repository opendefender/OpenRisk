// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// Test definitions of an implementation (#1415, #553 slice S4): what is checked
// (checks), how (method manual or automated, with its procedure or collector),
// how often (cadence, ISO 8601, optional) and what counts as a pass. Results and
// evidence are #1416; nothing here records one. The files of this half avoid the
// _test.go suffix on purpose: it would make them test files.
//
// A test follows its implementation's delete semantics (implementation_common.go):
// soft delete, and every read resolves the implementation and its control live in
// the tenant.

// Limits of the free text of a test, in runes. collector and cadence are
// varchar(255) and varchar(32) in the table; a longer value must be a 400, not a
// database error.
const (
	maxCollectorLen = 255
	maxCadenceLen   = 32
)

// TestRepository is the persistence the test-definition use cases need. Every
// method is scoped by the tenant it is given; another tenant's id reads as absent.
type TestRepository interface {
	// CreateTest resolves the implementation (and its control) live in the tenant,
	// then stores the test and its audit entry in one transaction.
	CreateTest(ctx context.Context, tenantID uuid.UUID, t canonical.ControlTest, audit *domain.AuditEvent) error
	GetTest(ctx context.Context, tenantID, id uuid.UUID) (*canonical.ControlTest, error)
	// ListTests returns the live tests of a live implementation.
	ListTests(ctx context.Context, tenantID, implementationID uuid.UUID, limit, offset int) ([]canonical.ControlTest, error)
	// UpdateTest loads the live test under lock and lets apply decide; a nil entry
	// means nothing changed and nothing is written.
	UpdateTest(ctx context.Context, tenantID, id uuid.UUID,
		apply func(cur canonical.ControlTest) (canonical.ControlTest, *domain.AuditEvent, error)) (*canonical.ControlTest, error)
	// DeleteTest soft-deletes the live test after apply accepted it.
	DeleteTest(ctx context.Context, tenantID, id uuid.UUID, apply func(cur canonical.ControlTest) (*domain.AuditEvent, error)) error
}

// TestView is a test definition as the API returns it: its content and its
// envelope. Results are #1416. The tenant is not part of it.
type TestView struct {
	ID               uuid.UUID            `json:"id"`
	ImplementationID uuid.UUID            `json:"implementation_id"`
	Name             string               `json:"name"`
	Checks           string               `json:"checks"`
	Method           canonical.TestMethod `json:"method"`
	Procedure        *string              `json:"procedure"`
	Collector        *string              `json:"collector"`
	Cadence          *string              `json:"cadence"`
	PassCriteria     string               `json:"pass_criteria"`
	Source           canonical.Source     `json:"source"`
	SourceID         string               `json:"source_id"`
	Confidence       float64              `json:"confidence"`
	Provenance       canonical.Provenance `json:"provenance"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

// TestViewOf renders a test definition.
func TestViewOf(t canonical.ControlTest) TestView {
	return TestView{
		ID: t.ID, ImplementationID: t.ImplementationID, Name: t.Name, Checks: t.Checks, Method: t.Method,
		Procedure: t.Procedure, Collector: t.Collector, Cadence: t.Cadence, PassCriteria: t.PassCriteria,
		Source: t.Source, SourceID: t.SourceID, Confidence: t.Confidence, Provenance: t.Provenance,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func testViewsOf(ts []canonical.ControlTest) []TestView {
	out := make([]TestView, len(ts))
	for i := range ts {
		out[i] = TestViewOf(ts[i])
	}
	return out
}

const auditTestType = "control_test"

// checkTestTexts caps the free text of a test.
func checkTestTexts(checks, procedure, passCriteria, collector, cadence *string) error {
	for _, f := range []struct {
		name string
		v    *string
	}{{"checks", checks}, {"procedure", procedure}, {"pass_criteria", passCriteria}} {
		if err := checkLongText(f.name, f.v); err != nil {
			return err
		}
	}
	if err := checkMax("collector", collector, maxCollectorLen); err != nil {
		return err
	}
	return checkMax("cadence", cadence, maxCadenceLen)
}

// testSnapshot is the audited image of a test definition.
func testSnapshot(t canonical.ControlTest) domain.JSONMap {
	m := domain.JSONMap{
		"id":                t.ID.String(),
		"tenant_id":         t.TenantID.String(),
		"implementation_id": t.ImplementationID.String(),
		"name":              t.Name,
		"checks":            t.Checks,
		"method":            string(t.Method),
		"pass_criteria":     t.PassCriteria,
		"source":            string(t.Source),
	}
	for k, v := range map[string]*string{"procedure": t.Procedure, "collector": t.Collector, "cadence": t.Cadence} {
		if v != nil {
			m[k] = *v
		}
	}
	return m
}
