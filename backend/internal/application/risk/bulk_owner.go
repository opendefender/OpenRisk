// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	bulkapp "github.com/opendefender/openrisk/internal/application/bulk"
	"github.com/opendefender/openrisk/internal/domain"
)

// The governed set_owner bulk action (#913): the register's Owner column writes
// owner_id exactly as a single-risk owner change does, through the governed
// engine, so it gets the preview, the fingerprint, the audit entry per risk and
// the plan permission like the other actions.

// BulkOwnerValidator is the engine's ChangeValidator for set_owner: the new
// owner must be an active member of the CALLER's tenant (the same check as the
// single-risk owner change, through the ownership manager). It runs on preview
// and on apply, before any row is read for the selection, so a foreign or
// deactivated user is a 400 and nothing is written. Other actions pass.
//
// A nil manager refuses set_owner rather than apply it unchecked.
func BulkOwnerValidator(mgr OwnershipManager) bulkapp.ChangeValidator {
	return func(ctx context.Context, caller bulkapp.Caller, change domain.BulkChange) error {
		if change.Action != domain.BulkActionSetOwner {
			return nil
		}
		if change.OwnerID == nil || *change.OwnerID == uuid.Nil {
			return domain.NewValidationError("owner_id is required for set_owner")
		}
		if mgr == nil {
			return domain.NewInternalError("ownership validation is not configured")
		}
		owner := *change.OwnerID
		patch := domain.OwnershipPatch{Owner: domain.NullableUUID{Present: true, Value: &owner}}
		// The scratch block is discarded: only the membership verdict matters.
		_, err := mgr.Apply(ctx, caller.TenantID, &domain.Ownership{}, patch, caller.UserID)
		return err
	}
}

// BulkOwnerNotices tells the new owner once per batch, after commit. One risk
// keeps the single-risk wording; several become ONE notice ("N risks have been
// assigned to you"), so a 100-risk request is one email and not a hundred.
// Nobody is told about their own action, nor about a risk whose owner did not
// actually change.
func BulkOwnerNotices(mgr OwnershipManager, notices NoticeDeliverer) bulkapp.AfterApply {
	return func(ctx context.Context, caller bulkapp.Caller, change domain.BulkChange, mutations []domain.BulkMutation) {
		if change.Action != domain.BulkActionSetOwner || change.OwnerID == nil {
			return
		}
		owner := *change.OwnerID
		if owner == caller.UserID {
			return
		}
		var moved []domain.BulkMutation
		for _, m := range mutations {
			if before, _ := m.Before["owner_id"].(string); before != owner.String() {
				moved = append(moved, m)
			}
		}
		switch {
		case len(moved) == 0:
			return
		case len(moved) == 1:
			if mgr == nil {
				return
			}
			mgr.Notify(ctx, caller.TenantID,
				[]domain.OwnershipChange{{Role: domain.RoleOwner, To: &owner, Actor: caller.UserID}},
				domain.OwnershipSubject{ResourceType: "risk", ResourceID: moved[0].ID, Title: moved[0].Label, Locale: "fr"})
		default:
			if notices == nil {
				return
			}
			n := len(moved)
			_ = notices.Deliver(ctx, domain.Notice{
				TenantID:     caller.TenantID,
				UserID:       owner,
				Type:         domain.NotificationTypeActionAssigned,
				ResourceType: "risk",
				Compose:      func(locale string) domain.NoticeMessage { return ownerBatchNoticeCopy(locale, n) },
			})
		}
	}
}

// ownerBatchNoticeCopy words the one notice a bulk owner change sends.
func ownerBatchNoticeCopy(locale string, n int) domain.NoticeMessage {
	if locale == "en" {
		return domain.NoticeMessage{
			Subject: fmt.Sprintf("%d risks have been assigned to you", n),
			Lines:   []string{fmt.Sprintf("You have been made owner of %d risks. You answer for their outcome.", n)},
		}
	}
	return domain.NoticeMessage{
		Subject: fmt.Sprintf("%d risques vous ont été attribués", n),
		Lines:   []string{fmt.Sprintf("Vous êtes désormais responsable de %d risques. Vous répondez de leur issue.", n)},
	}
}
