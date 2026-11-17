// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// RecordResultInput is the content of a new result. No field for the id, the
// tenant, the source, the confidence, the recorded_at or the provenance.
type RecordResultInput struct {
	Outcome    string
	ObservedAt time.Time
	Note       *string
	// ExecutedBy is a user of the tenant who ran the test; nil is the caller.
	ExecutedBy *uuid.UUID
}

// RecordResultUseCase records a run of a test of the tenant.
type RecordResultUseCase struct {
	repo   ResultRepository
	owners OwnerValidator
	now    Clock
}

// NewRecordResultUseCase builds the use case. owners must not be nil: another
// executor is validated as a member of the tenant.
func NewRecordResultUseCase(repo ResultRepository, owners OwnerValidator, now Clock) *RecordResultUseCase {
	return &RecordResultUseCase{repo: repo, owners: owners, now: now}
}

// Execute builds the result with a server-generated id and the user_entered
// envelope (source openrisk_user, source_id its own id, confidence 0.70,
// recorded_at the server clock), validates it, and stores it with its audit entry
// in one transaction. observed_at is required, normalised to UTC, and may not be
// more than five minutes ahead of the server. The executor is the caller unless a
// user of the tenant is named; system and vendor_contact are not settable here.
// The test must resolve live in the caller's tenant: a foreign one is ErrNotFound.
func (u *RecordResultUseCase) Execute(ctx context.Context, c Caller, testID uuid.UUID, in RecordResultInput) (*ResultView, error) {
	if err := c.authorize(PermissionUpdate); err != nil {
		return nil, err
	}
	if err := checkLongText("note", in.Note); err != nil {
		return nil, err
	}
	now := u.now().UTC()
	if in.ObservedAt.IsZero() {
		return nil, domain.InternalControlError(domain.ErrValidation, domain.IntCtlInvalidObservedAt)
	}
	if in.ObservedAt.After(now.Add(maxObservedSkew)) {
		return nil, domain.InternalControlError(domain.ErrValidation, domain.IntCtlInvalidObservedAt)
	}
	executor := c.UserID
	if in.ExecutedBy != nil && *in.ExecutedBy != c.UserID {
		if *in.ExecutedBy == uuid.Nil {
			return nil, domain.InternalControlError(domain.ErrValidation, domain.IntCtlOwnerInvalid)
		}
		if err := validateOwners(ctx, u.owners, c.TenantID, domain.OwnershipPatch{Owner: domain.Assign(*in.ExecutedBy)}); err != nil {
			return nil, err
		}
		executor = *in.ExecutedBy
	}
	conf, err := userConfidence()
	if err != nil {
		return nil, validation(err)
	}
	id := uuid.New()
	r := canonical.ControlTestResult{
		ResultID:   id,
		Outcome:    canonical.Outcome(in.Outcome),
		ObservedAt: in.ObservedAt.UTC(),
		RecordedAt: now,
		Source:     canonical.SourceOpenRiskUser,
		SourceID:   id.String(),
		Confidence: conf,
		ExecutedBy: canonical.Actor{Kind: canonical.ActorUser, ID: executor.String()},
		Note:       blankToNil(in.Note),
	}
	if err := validation(r.Validate()); err != nil {
		return nil, err
	}
	after := resultSnapshot(c.TenantID, testID, r)
	ev := auditEventOf(auditResultType, c, domain.AuditActionCreate, "control test result recorded", nil, after, changedFields(nil, after), id, now)
	if err := u.repo.RecordResult(ctx, c.TenantID, testID, r, ev); err != nil {
		return nil, fmt.Errorf("application.RecordControlTestResult: %w", err)
	}
	v := ResultViewOf(testID, r)
	return &v, nil
}
