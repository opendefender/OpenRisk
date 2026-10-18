// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"context"

	"github.com/google/uuid"

	actioncenterapp "github.com/opendefender/openrisk/internal/application/actioncenter"
	notificationapp "github.com/opendefender/openrisk/internal/application/notification"
)

// noticeDirectory answers "who is this user, inside THIS tenant" for the
// notice dispatcher (#757). It reads the membership row of (tenant, user) — the
// same tenant-scoped lookup the ownership picker uses — so a user id from
// another organisation, or a member who has since been deactivated, resolves to
// nobody and gets nothing.
type noticeDirectory struct{ members memberDirectory }

func (d noticeDirectory) NoticeRecipient(ctx context.Context, tenantID, userID uuid.UUID) (*notificationapp.Recipient, error) {
	m, err := d.members.GetMember(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	if m == nil || !m.IsActive || m.User == nil || !m.User.IsActive {
		return nil, nil
	}
	return &notificationapp.Recipient{Email: m.User.Email, FullName: m.User.FullName, Locale: m.User.Locale}, nil
}

// noticeLink maps a notice's resource to the page the bell would open, using
// the Action Center's routes so e-mail and app agree on where things live.
func noticeLink(resourceType string, resourceID *uuid.UUID) string {
	if resourceID == nil || *resourceID == uuid.Nil {
		return ""
	}
	id := resourceID.String()
	switch resourceType {
	case "risk":
		return actioncenterapp.RiskLink(id)
	case "mitigation":
		return actioncenterapp.MitigationLink(id)
	case "incident":
		return actioncenterapp.IncidentLink(id)
	case "evidence":
		return actioncenterapp.EvidenceLink(id)
	case "remediation", "remediation_plan":
		return actioncenterapp.RemediationLink(id)
	default:
		return ""
	}
}
