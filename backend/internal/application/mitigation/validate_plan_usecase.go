// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package mitigation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// NoticeDeliverer tells one member about an event, in-app and by e-mail
// (application/notification.Dispatcher). Optional: nil means nobody is told.
type NoticeDeliverer interface {
	Deliver(ctx context.Context, n domain.Notice) error
}

// ValidateMitigationPlanUseCase transitions a plan from REVIEW to DONE (reviewer approval)
type ValidateMitigationPlanUseCase struct {
	mitigationRepo repository.MitigationRepository
	notifier       NoticeDeliverer
}

func NewValidateMitigationPlanUseCase(
	mitigationRepo repository.MitigationRepository,
	notifier NoticeDeliverer,
) *ValidateMitigationPlanUseCase {
	return &ValidateMitigationPlanUseCase{
		mitigationRepo: mitigationRepo,
		notifier:       notifier,
	}
}

type ValidateMitigationPlanInput struct {
	TenantID   uuid.UUID
	PlanID     uuid.UUID
	ReviewedBy uuid.UUID
}

// Execute validates (approves) a mitigation plan
func (uc *ValidateMitigationPlanUseCase) Execute(input ValidateMitigationPlanInput) error {
	if input.TenantID == uuid.Nil || input.PlanID == uuid.Nil || input.ReviewedBy == uuid.Nil {
		return fmt.Errorf("tenant_id, plan_id, and reviewed_by are required")
	}

	mitigation, err := uc.mitigationRepo.GetByID(input.TenantID.String(), input.PlanID)
	if err != nil {
		return err
	}

	// Can only validate plans in REVIEW status (auto-filled when progress = 100)
	if mitigation.Status != domain.MitigationReview {
		return fmt.Errorf("plan must be in REVIEW status, current: %s", mitigation.Status)
	}

	now := time.Now()
	mitigation.Status = domain.MitigationDone
	mitigation.ApprovedBy = &input.ReviewedBy
	mitigation.ApprovedAt = &now
	mitigation.UpdatedAt = now

	if err := uc.mitigationRepo.Update(input.TenantID.String(), mitigation); err != nil {
		return err
	}
	uc.notifyValidated(input.TenantID, mitigation, input.ReviewedBy)
	return nil
}

// notifyValidated tells whoever did the work that the reviewer approved it
// (#757). Best-effort: the plan is already DONE. The reviewer is not told
// about their own approval.
func (uc *ValidateMitigationPlanUseCase) notifyValidated(tenantID uuid.UUID, m *domain.Mitigation, reviewer uuid.UUID) {
	if uc.notifier == nil || m == nil {
		return
	}
	recipient := uuid.Nil
	switch {
	case m.AssigneeID != nil && *m.AssigneeID != uuid.Nil:
		recipient = *m.AssigneeID
	case m.OwnerID != nil && *m.OwnerID != uuid.Nil:
		recipient = *m.OwnerID
	default:
		recipient = m.CreatedBy
	}
	if recipient == uuid.Nil || recipient == reviewer {
		return
	}
	id, title := m.ID, m.Title
	_ = uc.notifier.Deliver(context.Background(), domain.Notice{
		TenantID:     tenantID,
		UserID:       recipient,
		Type:         domain.NotificationTypeMitigationValidated,
		ResourceID:   &id,
		ResourceType: "mitigation",
		Compose: func(locale string) domain.NoticeMessage {
			if locale == "en" {
				return domain.NoticeMessage{
					Subject: fmt.Sprintf("Mitigation validated: %s", title),
					Lines:   []string{fmt.Sprintf("The reviewer approved “%s”. The plan is now done.", title)},
				}
			}
			return domain.NoticeMessage{
				Subject: fmt.Sprintf("Mitigation validée : %s", title),
				Lines:   []string{fmt.Sprintf("Le validateur a approuvé « %s ». Le plan est désormais terminé.", title)},
			}
		},
	})
}
