// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// CreateTestInput is the content of a new test definition. No field for the id,
// the tenant, the source, the confidence or the provenance.
type CreateTestInput struct {
	Name         string
	Checks       string
	Method       string
	Procedure    *string
	Collector    *string
	Cadence      *string
	PassCriteria string
}

// CreateTestUseCase adds a test definition to an implementation of the tenant.
type CreateTestUseCase struct {
	repo TestRepository
	now  Clock
}

// NewCreateTestUseCase builds the use case.
func NewCreateTestUseCase(repo TestRepository, now Clock) *CreateTestUseCase {
	return &CreateTestUseCase{repo: repo, now: now}
}

// Execute builds the test with a server-generated id and the user_entered
// envelope, validates it (checks and pass_criteria required, procedure while
// manual, collector while automated, cadence optional ISO 8601), and stores it
// with its audit entry in one transaction. The implementation must resolve live
// in the caller's tenant: a foreign one is ErrNotFound.
func (u *CreateTestUseCase) Execute(ctx context.Context, c Caller, implementationID uuid.UUID, in CreateTestInput) (*TestView, error) {
	if err := c.authorize(PermissionCreate); err != nil {
		return nil, err
	}
	if err := checkTestTexts(&in.Checks, in.Procedure, &in.PassCriteria, in.Collector, in.Cadence); err != nil {
		return nil, err
	}
	conf, err := userConfidence()
	if err != nil {
		return nil, validation(err)
	}
	now := u.now().UTC()
	id := uuid.New()
	t := canonical.ControlTest{
		Envelope:         newEnvelope(c, id, conf, now),
		ImplementationID: implementationID,
		Name:             in.Name,
		Checks:           in.Checks,
		Method:           canonical.TestMethod(in.Method),
		Procedure:        blankToNil(in.Procedure),
		Collector:        blankToNil(in.Collector),
		Cadence:          blankToNil(in.Cadence),
		PassCriteria:     in.PassCriteria,
	}
	if err := validation(t.Validate()); err != nil {
		return nil, err
	}
	after := testSnapshot(t)
	ev := auditEventOf(auditTestType, c, domain.AuditActionCreate, "control test created", nil, after, changedFields(nil, after), id, now)
	if err := u.repo.CreateTest(ctx, c.TenantID, t, ev); err != nil {
		return nil, fmt.Errorf("application.CreateControlTest: %w", err)
	}
	v := TestViewOf(t)
	return &v, nil
}
