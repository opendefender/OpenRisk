// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package governance

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

func submitTwoStep(t *testing.T) (*fakeApprovalRepo, uuid.UUID, uuid.UUID, *domain.ApprovalRequest) {
	t.Helper()
	repo := newFakeApprovalRepo()
	tenant, requester := uuid.New(), uuid.New()
	seedTwoStepWorkflow(t, repo, tenant)
	req, err := NewSubmitApprovalRequestUseCase(repo, repo).Execute(context.Background(), tenant, requester, SubmitApprovalInput{
		EntityType: "risk_acceptance", Action: "accept", EntityID: "risk-1", Title: "Accept residual risk",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	return repo, tenant, requester, req
}

// #903: deferring leaves the request pending at the same step, with a dated
// note, and the next approval still works as before.
func TestDeferApproval_Success(t *testing.T) {
	repo, tenant, _, req := submitTwoStep(t)
	decide := NewDecideApprovalUseCase(repo)
	manager := uuid.New()

	got, err := decide.Defer(context.Background(), tenant, req.ID,
		ApproverIdentity{UserID: manager, Email: "m@x.io", Roles: []string{"manager"}}, "  needs the Q4 budget  ")
	if err != nil {
		t.Fatalf("defer: %v", err)
	}
	if got.Status != domain.ApprovalPending || got.CurrentStep != req.CurrentStep {
		t.Fatalf("defer moved the circuit: status=%s step=%d", got.Status, got.CurrentStep)
	}
	if len(got.Deferrals) != 1 || got.Deferrals[0].Comment != "needs the Q4 budget" || got.Deferrals[0].DeferredBy != manager.String() {
		t.Fatalf("deferral not recorded: %+v", got.Deferrals)
	}
	if len(got.Decisions) != 0 {
		t.Fatalf("a deferral must not be a decision: %+v", got.Decisions)
	}
	// The same manager can still approve the step afterwards.
	after, err := decide.Execute(context.Background(), tenant, req.ID,
		ApproverIdentity{UserID: manager, Roles: []string{"manager"}}, DecideInput{Decision: "approve"})
	if err != nil || after.CurrentStep != 1 {
		t.Fatalf("approve after defer: err=%v step=%d", err, after.CurrentStep)
	}
}

func TestDeferApproval_NotFound(t *testing.T) {
	repo, tenant, _, _ := submitTwoStep(t)
	_, err := NewDecideApprovalUseCase(repo).Defer(context.Background(), tenant, uuid.New(),
		ApproverIdentity{UserID: uuid.New(), IsAdmin: true}, "")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

// Only someone who could sign may defer: not the requester (four-eyes), not an
// ineligible role, not another tenant.
func TestDeferApproval_Unauthorized(t *testing.T) {
	repo, tenant, requester, req := submitTwoStep(t)
	decide := NewDecideApprovalUseCase(repo)
	if _, err := decide.Defer(context.Background(), tenant, req.ID,
		ApproverIdentity{UserID: requester, Roles: []string{"manager"}}, ""); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("maker deferred own request: %v", err)
	}
	if _, err := decide.Defer(context.Background(), tenant, req.ID,
		ApproverIdentity{UserID: uuid.New(), Roles: []string{"viewer"}}, ""); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("ineligible role deferred: %v", err)
	}
	if _, err := decide.Defer(context.Background(), uuid.New(), req.ID,
		ApproverIdentity{UserID: uuid.New(), IsAdmin: true}, ""); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other tenant reached the request: %v", err)
	}
}
