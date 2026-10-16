// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package mitigation

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// reviewRepo serves one plan to its own tenant only.
type reviewRepo struct {
	fakeMitigationRepo
	plan *domain.Mitigation
}

func (r *reviewRepo) GetByID(tenantID string, id uuid.UUID) (*domain.Mitigation, error) {
	if r.plan == nil || r.plan.ID != id || r.plan.TenantID.String() != tenantID {
		return nil, domain.ErrNotFound
	}
	return r.plan, nil
}

type noticeRecorder struct{ got []domain.Notice }

func (n *noticeRecorder) Deliver(_ context.Context, notice domain.Notice) error {
	n.got = append(n.got, notice)
	return nil
}

func planInReview() *domain.Mitigation {
	assignee := uuid.New()
	return &domain.Mitigation{
		ID: uuid.New(), TenantID: uuid.New(), Title: "Rotate keys",
		Status: domain.MitigationReview, CreatedBy: uuid.New(),
		Ownership: domain.Ownership{AssigneeID: &assignee},
	}
}

// #757: validating a plan tells whoever did the work.
func TestValidatePlanNotice_Success(t *testing.T) {
	plan := planInReview()
	notices := &noticeRecorder{}
	uc := NewValidateMitigationPlanUseCase(&reviewRepo{plan: plan}, notices)

	if err := uc.Execute(ValidateMitigationPlanInput{TenantID: plan.TenantID, PlanID: plan.ID, ReviewedBy: uuid.New()}); err != nil {
		t.Fatal(err)
	}
	if plan.Status != domain.MitigationDone {
		t.Fatalf("plan must be done, got %s", plan.Status)
	}
	if len(notices.got) != 1 {
		t.Fatalf("expected one notice, got %d", len(notices.got))
	}
	n := notices.got[0]
	if n.UserID != *plan.AssigneeID || n.TenantID != plan.TenantID || n.Type != domain.NotificationTypeMitigationValidated {
		t.Fatalf("notice mis-addressed: %+v", n)
	}
	if !strings.Contains(n.Compose("en").Subject, "validated") || !strings.Contains(n.Compose("fr").Subject, "validée") {
		t.Fatal("copy exists in both languages")
	}
}

func TestValidatePlanNotice_NotFound(t *testing.T) {
	plan := planInReview()
	notices := &noticeRecorder{}
	uc := NewValidateMitigationPlanUseCase(&reviewRepo{plan: plan}, notices)

	// Another tenant's id: the plan does not exist for them.
	if err := uc.Execute(ValidateMitigationPlanInput{TenantID: uuid.New(), PlanID: plan.ID, ReviewedBy: uuid.New()}); err == nil {
		t.Fatal("cross-tenant validation must fail")
	}
	if len(notices.got) != 0 || plan.Status != domain.MitigationReview {
		t.Fatal("nothing changes and nobody is told")
	}
}

// The reviewer who validated their own work is not told about it, and a
// missing identity is refused before anything happens.
func TestValidatePlanNotice_Unauthorized(t *testing.T) {
	plan := planInReview()
	notices := &noticeRecorder{}
	uc := NewValidateMitigationPlanUseCase(&reviewRepo{plan: plan}, notices)

	if err := uc.Execute(ValidateMitigationPlanInput{TenantID: plan.TenantID, PlanID: plan.ID}); err == nil {
		t.Fatal("a validation without a reviewer must be refused")
	}
	if err := uc.Execute(ValidateMitigationPlanInput{TenantID: plan.TenantID, PlanID: plan.ID, ReviewedBy: *plan.AssigneeID}); err != nil {
		t.Fatal(err)
	}
	if len(notices.got) != 0 {
		t.Fatal("nobody is told about their own approval")
	}

	// A nil deliverer (unwired deployment) validates without telling anyone.
	plan2 := planInReview()
	if err := NewValidateMitigationPlanUseCase(&reviewRepo{plan: plan2}, nil).
		Execute(ValidateMitigationPlanInput{TenantID: plan2.TenantID, PlanID: plan2.ID, ReviewedBy: uuid.New()}); err != nil {
		t.Fatal(err)
	}
}
