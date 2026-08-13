// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package vulnrisk

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// InAppNotifier is the narrow slice of the notification use case this package
// needs. Declared here rather than imported so vulnrisk does not depend on the
// whole notification application package.
type InAppNotifier interface {
	NotifyInApp(userID, tenantID uuid.UUID, notifType domain.NotificationType, subject, message string, resourceID *uuid.UUID, resourceType string) error
}

// DraftRiskNotifier tells the tenant's risk owners that automation proposed a
// draft risk.
//
// A draft nobody is told about is a draft nobody reviews, which turns the
// vuln→risk rule into a silent generator of register clutter. Everything here is
// best-effort: a notification failure must never fail an ingest.
type DraftRiskNotifier struct {
	db     *gorm.DB
	notify InAppNotifier
}

func NewDraftRiskNotifier(db *gorm.DB, notify InAppNotifier) *DraftRiskNotifier {
	return &DraftRiskNotifier{db: db, notify: notify}
}

// NotifyDraftRiskProposed notifies the tenant's admins.
//
// Recipients are the org's admins rather than "everyone": a proposal is a
// request for a decision, and broadcasting decisions to people who cannot make
// them is how notification panels become noise nobody opens.
func (n *DraftRiskNotifier) NotifyDraftRiskProposed(
	ctx context.Context,
	tenantID, riskID uuid.UUID,
	v *domain.Vulnerability,
	reason string,
) {
	if n == nil || n.notify == nil || n.db == nil {
		return
	}
	// Never let a notification path panic an ingest.
	defer func() { _ = recover() }()

	var userIDs []uuid.UUID
	if err := n.db.WithContext(ctx).
		Model(&domain.OrganizationMember{}).
		Where("organization_id = ? AND role IN ?", tenantID, []string{"admin", "root"}).
		Pluck("user_id", &userIDs).Error; err != nil {
		return
	}

	label := v.CVEID
	if label == "" {
		label = v.Title
	}
	subject := fmt.Sprintf("Risque proposé (brouillon) : %s", label)
	message := fmt.Sprintf(
		"Une vulnérabilité a déclenché votre règle de création de risque. Le risque a été créé en BROUILLON et n'entrera dans le registre qu'après votre validation.\n\nMotif : %s",
		reason,
	)
	if v.AssetName != "" {
		message += fmt.Sprintf("\nActif concerné : %s", v.AssetName)
	}

	id := riskID
	for _, uid := range userIDs {
		_ = n.notify.NotifyInApp(uid, tenantID, domain.NotificationTypeRiskReview, subject, message, &id, "risk")
	}
}
