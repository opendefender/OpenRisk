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
	if n.Type != domain.NotificationTypeRiskUpdate {
		t.Fatalf("an intermediate move is a risk update, got %s", n.Type)
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
