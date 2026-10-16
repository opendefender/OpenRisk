// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

type recordedNotices struct{ got []domain.Notice }

func (r *recordedNotices) Deliver(_ context.Context, n domain.Notice) error {
	r.got = append(r.got, n)
	return nil
}

// #757: the owner of a risk hears when someone else moves it.
func TestTransitionNotice_Success(t *testing.T) {
	repo, tenant, id := newFixture(domain.StateIdentified)
	owner := uuid.New()
	repo.risk.OwnerID = &owner
	notices := &recordedNotices{}

	_, err := NewTransitionRiskStateUseCase(repo).WithNotices(notices).
		Execute(context.Background(), tenant, id, TransitionRiskStateInput{To: domain.StateAssessed, Actor: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if len(notices.got) != 1 {
		t.Fatalf("expected one notice, got %d", len(notices.got))
	}
	n := notices.got[0]
	if n.TenantID != tenant || n.UserID != owner || n.ResourceType != "risk" || *n.ResourceID != id {
		t.Fatalf("notice mis-addressed: %+v", n)
	}
	if n.Type != domain.NotificationTypeRiskStateChanged {
		t.Fatalf("an intermediate move is a risk-state-changed notice, got %s", n.Type)
	}
	fr, en := n.Compose("fr"), n.Compose("en")
	if !strings.Contains(fr.InAppText(), "Identifié") || !strings.Contains(fr.InAppText(), "Évalué") {
		t.Fatalf("French copy must name both states: %q", fr.InAppText())
	}
	if !strings.Contains(en.Subject, "Assessed") || !strings.Contains(en.Subject, "Fuite S3") {
		t.Fatalf("English subject must name the risk and the state: %q", en.Subject)
	}
}

func TestTransitionNotice_EndStateIsRiskResolved(t *testing.T) {
	repo, tenant, id := newFixture(domain.StateAssessed)
	owner := uuid.New()
	repo.risk.OwnerID = &owner
	notices := &recordedNotices{}

	if _, err := NewTransitionRiskStateUseCase(repo).WithNotices(notices).
		Execute(context.Background(), tenant, id, TransitionRiskStateInput{To: domain.StateClosed, Actor: uuid.New()}); err != nil {
		t.Fatal(err)
	}
	if len(notices.got) != 1 || notices.got[0].Type != domain.NotificationTypeRiskResolved {
		t.Fatalf("closing a risk is a risk-resolved notice: %+v", notices.got)
	}
}

// A refused transition, or one on another tenant's risk, tells nobody.
func TestTransitionNotice_NotFound(t *testing.T) {
	repo, _, id := newFixture(domain.StateIdentified)
	owner := uuid.New()
	repo.risk.OwnerID = &owner
	notices := &recordedNotices{}

	_, err := NewTransitionRiskStateUseCase(repo).WithNotices(notices).
		Execute(context.Background(), uuid.New(), id, TransitionRiskStateInput{To: domain.StateAssessed, Actor: uuid.New()})
	if err == nil {
		t.Fatal("cross-tenant transition must fail")
	}
	if len(notices.got) != 0 {
		t.Fatal("no notice for a transition that did not happen")
	}
}

// The owner is not told about their own move; an unowned risk tells nobody.
func TestTransitionNotice_Unauthorized(t *testing.T) {
	repo, tenant, id := newFixture(domain.StateIdentified)
	owner := uuid.New()
	repo.risk.OwnerID = &owner
	notices := &recordedNotices{}
	uc := NewTransitionRiskStateUseCase(repo).WithNotices(notices)

	if _, err := uc.Execute(context.Background(), tenant, id, TransitionRiskStateInput{To: domain.StateAssessed, Actor: owner}); err != nil {
		t.Fatal(err)
	}
	repo.risk.OwnerID = nil
	if _, err := uc.Execute(context.Background(), tenant, id, TransitionRiskStateInput{To: domain.StateTreatmentPlanned, Actor: uuid.New()}); err != nil {
		t.Fatal(err)
	}
	if len(notices.got) != 0 {
		t.Fatalf("expected no notice, got %d", len(notices.got))
	}
}

// PATCH /risks/:id with a status is a change of state too: the owner hears it.
func TestUpdateStatusNotice_Success(t *testing.T) {
	repo, tenant, id := newFixture(domain.StateIdentified)
	owner := uuid.New()
	repo.risk.OwnerID = &owner
	notices := &recordedNotices{}
	status := domain.RiskMitigated

	if _, err := NewUpdateRiskUseCase(repo).WithNotices(notices).
		Execute(context.Background(), tenant, id, UpdateRiskInput{Status: &status, Actor: uuid.New()}); err != nil {
		t.Fatal(err)
	}
	if len(notices.got) != 1 {
		t.Fatalf("expected one notice, got %d", len(notices.got))
	}
	n := notices.got[0]
	if n.UserID != owner || n.TenantID != tenant || n.Type != domain.NotificationTypeRiskResolved {
		t.Fatalf("notice mis-addressed: %+v", n)
	}

	// Saving other fields without touching the status is not a state change.
	notices.got = nil
	title := "Renamed"
	if _, err := NewUpdateRiskUseCase(repo).WithNotices(notices).
		Execute(context.Background(), tenant, id, UpdateRiskInput{Title: &title, Actor: uuid.New()}); err != nil {
		t.Fatal(err)
	}
	if len(notices.got) != 0 {
		t.Fatal("an update that leaves the status alone tells nobody")
	}
}

func TestUpdateStatusNotice_NotFound(t *testing.T) {
	repo, _, id := newFixture(domain.StateIdentified)
	owner := uuid.New()
	repo.risk.OwnerID = &owner
	notices := &recordedNotices{}
	status := domain.RiskMitigated

	if _, err := NewUpdateRiskUseCase(repo).WithNotices(notices).
		Execute(context.Background(), uuid.New(), id, UpdateRiskInput{Status: &status, Actor: uuid.New()}); err == nil {
		t.Fatal("another tenant's risk must not be updatable")
	}
	if len(notices.got) != 0 {
		t.Fatal("no notice for an update that did not happen")
	}
}

func TestUpdateStatusNotice_Unauthorized(t *testing.T) {
	repo, tenant, id := newFixture(domain.StateIdentified)
	owner := uuid.New()
	repo.risk.OwnerID = &owner
	notices := &recordedNotices{}
	status := domain.RiskMitigated

	if _, err := NewUpdateRiskUseCase(repo).WithNotices(notices).
		Execute(context.Background(), tenant, id, UpdateRiskInput{Status: &status, Actor: owner}); err != nil {
		t.Fatal(err)
	}
	if len(notices.got) != 0 {
		t.Fatal("the owner is not told about their own change")
	}
}

// Accepting a draft puts it in the register: its owner hears about it.
func TestDraftAcceptNotice_Success(t *testing.T) {
	repo, tenant, id := newFixture(domain.StateDraft)
	owner := uuid.New()
	repo.risk.OwnerID = &owner
	notices := &recordedNotices{}

	res, err := NewDraftReviewUseCase(repo).WithNotices(notices).
		BulkReview(context.Background(), tenant, uuid.New(), []uuid.UUID{id}, DecisionAccept)
	if err != nil || len(res.Accepted) != 1 {
		t.Fatalf("accept failed: %v %+v", err, res)
	}
	if len(notices.got) != 1 || notices.got[0].UserID != owner || notices.got[0].Type != domain.NotificationTypeRiskStateChanged {
		t.Fatalf("expected one risk-state-changed notice to the owner: %+v", notices.got)
	}
}

func TestDraftAcceptNotice_NotFound(t *testing.T) {
	repo, _, id := newFixture(domain.StateDraft)
	owner := uuid.New()
	repo.risk.OwnerID = &owner
	notices := &recordedNotices{}

	res, err := NewDraftReviewUseCase(repo).WithNotices(notices).
		BulkReview(context.Background(), uuid.New(), uuid.New(), []uuid.UUID{id}, DecisionAccept)
	if err != nil || len(res.Accepted) != 0 {
		t.Fatalf("another tenant's draft must not be accepted: %v %+v", err, res)
	}
	if len(notices.got) != 0 {
		t.Fatal("nobody is told about a draft that was not accepted")
	}
}

func TestDraftAcceptNotice_Unauthorized(t *testing.T) {
	repo, tenant, id := newFixture(domain.StateDraft)
	owner := uuid.New()
	repo.risk.OwnerID = &owner
	notices := &recordedNotices{}

	if _, err := NewDraftReviewUseCase(repo).WithNotices(notices).
		BulkReview(context.Background(), tenant, owner, []uuid.UUID{id}, DecisionAccept); err != nil {
		t.Fatal(err)
	}
	if len(notices.got) != 0 {
		t.Fatal("an owner accepting their own draft is not mailed about it")
	}
}
