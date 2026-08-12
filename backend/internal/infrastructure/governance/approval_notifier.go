// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package governance holds the infrastructure adapters of the Governance module:
// how an approval reaches the people who have to act on it, and how a user's
// roles are resolved for delegation-aware eligibility.
package governance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	appgov "github.com/opendefender/openrisk/internal/application/governance"
	"github.com/opendefender/openrisk/internal/domain"
)

// InAppNotifier is the notification use case (in-app persistence).
type InAppNotifier interface {
	NotifyInApp(userID, tenantID uuid.UUID, notifType domain.NotificationType, subject, message string, resourceID *uuid.UUID, resourceType string) error
}

// EmailSender is the platform mail transport.
type EmailSender interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

// ApprovalNotifier tells approvers that a decision awaits them, and tells the
// requester how it ended.
//
// An approval inbox nobody is told to open is a queue, not a workflow — which is
// why this exists rather than relying on people checking the Governance page.
// Delivery is best-effort on every channel: a mail server outage must never
// unwind a decision that was validly recorded.
type ApprovalNotifier struct {
	db     *gorm.DB
	inApp  InAppNotifier
	email  EmailSender
	logger zerolog.Logger
}

// NewApprovalNotifier builds the dispatcher.
func NewApprovalNotifier(db *gorm.DB, inApp InAppNotifier, email EmailSender, logger zerolog.Logger) *ApprovalNotifier {
	return &ApprovalNotifier{db: db, inApp: inApp, email: email, logger: logger}
}

var _ appgov.ApprovalNotifier = (*ApprovalNotifier)(nil)

// NotifyPending alerts everyone who can currently sign: the named approvers of
// the open steps, plus every member holding an accepted role, plus anyone
// currently holding a delegation from one of them.
func (n *ApprovalNotifier) NotifyPending(ctx context.Context, tenantID uuid.UUID, req *domain.ApprovalRequest, roles []string, userIDs []uuid.UUID) {
	if n == nil || req == nil {
		return
	}
	recipients := n.resolveRecipients(ctx, tenantID, roles, userIDs)
	// Never ask the requester to approve their own request (four-eyes would
	// refuse it anyway, so the notification would only be noise).
	delete(recipients, req.RequestedBy)
	if len(recipients) == 0 {
		n.logger.Debug().Str("request", req.ID.String()).
			Msg("governance: no eligible approver to notify")
		return
	}

	subject := "Approval needed: " + req.Title
	body := n.pendingBody(req)
	for id, email := range recipients {
		if n.inApp != nil {
			rid := req.ID
			if err := n.inApp.NotifyInApp(id, tenantID, domain.NotificationTypeActionAssigned,
				subject, body, &rid, "approval_request"); err != nil {
				n.logger.Debug().Err(err).Msg("governance: in-app approval notice failed")
			}
		}
		if n.email != nil && email != "" {
			if err := n.email.SendEmail(ctx, email, subject, body); err != nil {
				n.logger.Debug().Err(err).Msg("governance: approval e-mail failed")
			}
		}
	}
}

// NotifyResolved tells the requester the outcome, including WHY when refused.
func (n *ApprovalNotifier) NotifyResolved(ctx context.Context, tenantID uuid.UUID, req *domain.ApprovalRequest) {
	if n == nil || req == nil || req.RequestedBy == uuid.Nil {
		return
	}
	subject := "Your request was " + string(req.Status) + ": " + req.Title
	body := n.resolvedBody(req)

	if n.inApp != nil {
		rid := req.ID
		notifType := domain.NotificationTypeRiskUpdate
		if err := n.inApp.NotifyInApp(req.RequestedBy, tenantID, notifType,
			subject, body, &rid, "approval_request"); err != nil {
			n.logger.Debug().Err(err).Msg("governance: in-app resolution notice failed")
		}
	}
	if n.email != nil {
		if email := n.emailFor(ctx, req.RequestedBy); email != "" {
			if err := n.email.SendEmail(ctx, email, subject, body); err != nil {
				n.logger.Debug().Err(err).Msg("governance: resolution e-mail failed")
			}
		}
	}
}

// resolveRecipients maps roles + named users to (user id → email).
func (n *ApprovalNotifier) resolveRecipients(ctx context.Context, tenantID uuid.UUID, roles []string, userIDs []uuid.UUID) map[uuid.UUID]string {
	out := map[uuid.UUID]string{}
	if n.db == nil {
		for _, id := range userIDs {
			out[id] = ""
		}
		return out
	}

	ids := append([]uuid.UUID{}, userIDs...)
	if len(roles) > 0 {
		var members []domain.OrganizationMember
		if err := n.db.WithContext(ctx).
			Where("organization_id = ? AND LOWER(role) IN ?", tenantID, lowerAll(roles)).
			Find(&members).Error; err == nil {
			for _, m := range members {
				ids = append(ids, m.UserID)
			}
		}
	}
	// Anyone currently covering for an eligible approver is eligible too, and
	// should be told — otherwise a delegation silently changes nothing.
	if len(ids) > 0 {
		var delegations []domain.Delegation
		if err := n.db.WithContext(ctx).
			Where("tenant_id = ? AND delegator_id IN ? AND status = ? AND starts_at <= ? AND ends_at >= ?",
				tenantID, ids, domain.DelegationActive, time.Now().UTC(), time.Now().UTC()).
			Find(&delegations).Error; err == nil {
			for _, d := range delegations {
				ids = append(ids, d.DelegateID)
			}
		}
	}
	if len(ids) == 0 {
		return out
	}

	var users []domain.User
	if err := n.db.WithContext(ctx).Where("id IN ?", ids).Find(&users).Error; err != nil {
		for _, id := range ids {
			out[id] = ""
		}
		return out
	}
	for _, u := range users {
		out[u.ID] = u.Email
	}
	return out
}

func (n *ApprovalNotifier) emailFor(ctx context.Context, id uuid.UUID) string {
	if n.db == nil {
		return ""
	}
	var u domain.User
	if err := n.db.WithContext(ctx).Where("id = ?", id).Take(&u).Error; err != nil {
		return ""
	}
	return u.Email
}

func (n *ApprovalNotifier) pendingBody(req *domain.ApprovalRequest) string {
	var b strings.Builder
	b.WriteString("A request is waiting for your decision.\n\n")
	b.WriteString("Title: " + req.Title + "\n")
	if req.Description != "" {
		b.WriteString("Details: " + req.Description + "\n")
	}
	b.WriteString("Workflow: " + req.WorkflowName + "\n")
	progress := domain.Progress(req)
	for _, p := range progress {
		if p.Open {
			b.WriteString(fmt.Sprintf("Awaiting step %d (%s): %d of %d approval(s)\n",
				p.Order+1, p.Name, p.Approvals, p.Required))
		}
	}
	if req.ExpiresAt != nil {
		b.WriteString("Deadline: " + req.ExpiresAt.Format(time.RFC3339) +
			" — after that the request closes as expired without a decision.\n")
	}
	return b.String()
}

func (n *ApprovalNotifier) resolvedBody(req *domain.ApprovalRequest) string {
	var b strings.Builder
	b.WriteString("Your request \"" + req.Title + "\" is now " + string(req.Status) + ".\n\n")
	switch req.Status {
	case domain.ApprovalRejected:
		// The reason is the whole value of the message: without it the requester
		// re-submits the same thing.
		for i := len(req.Decisions) - 1; i >= 0; i-- {
			if req.Decisions[i].Decision == "reject" {
				b.WriteString("Reason given: " + req.Decisions[i].Comment + "\n")
				if req.Decisions[i].ApproverEmail != "" {
					b.WriteString("Refused by: " + req.Decisions[i].ApproverEmail + "\n")
				}
				break
			}
		}
	case domain.ApprovalExpired:
		b.WriteString("Nobody decided before the deadline, so it closed automatically. " +
			"Nobody refused it — you can raise it again.\n")
	case domain.ApprovalApproved:
		b.WriteString("Every step of the chain signed off. You can proceed.\n")
	}
	return b.String()
}

func lowerAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, strings.ToLower(strings.TrimSpace(s)))
	}
	return out
}

// =============================================================================
// Role resolution (for delegation-aware eligibility)
// =============================================================================

// RoleResolver reports a user's org roles inside a tenant.
type RoleResolver struct{ db *gorm.DB }

// NewRoleResolver builds the resolver.
func NewRoleResolver(db *gorm.DB) *RoleResolver { return &RoleResolver{db: db} }

var _ appgov.RoleResolver = (*RoleResolver)(nil)

// RolesFor returns the org role plus the business role a user holds, both of
// which a workflow step may name.
func (r *RoleResolver) RolesFor(ctx context.Context, tenantID, userID uuid.UUID) ([]string, error) {
	var member domain.OrganizationMember
	if err := r.db.WithContext(ctx).
		Where("organization_id = ? AND user_id = ?", tenantID, userID).
		Take(&member).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	out := []string{}
	if role := strings.TrimSpace(string(member.Role)); role != "" {
		out = append(out, role)
	}
	if br := strings.TrimSpace(string(member.BusinessRole)); br != "" {
		out = append(out, br)
	}
	return out, nil
}
