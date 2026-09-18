// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
)

// reminderHorizon is the widest reminder window: nothing further out is due.
const reminderHorizon = 7 * 24 * time.Hour

// ReminderStore is the sweep's view of the assessments (#672, ADR 0004 D6).
type ReminderStore interface {
	// ListOpenAssessmentsDueWithin returns open assessments, ACROSS TENANTS,
	// whose due date is after now and within horizon, and which still have a
	// reminder unsent. Cross-tenant by necessity: a scheduled sweep holds no
	// session. Every row carries its own tenant_id.
	ListOpenAssessmentsDueWithin(ctx context.Context, now time.Time, horizon time.Duration) ([]domain.VendorAssessment, error)

	// RecordReminder stamps the reminder (and every larger offset) and, when
	// tok is not nil, supersedes the active token and inserts tok — in one
	// transaction, only while the assessment is open and that reminder is still
	// unsent. False when another sweep got there first or the assessment closed.
	RecordReminder(ctx context.Context, a *domain.VendorAssessment, offset int, tok *domain.VendorAssessmentToken, at time.Time) (bool, error)
}

// ReminderNotifier raises the in-app notice to the assessment's owner. The
// tenant it is given is the ROW's tenant.
type ReminderNotifier func(ctx context.Context, tenantID, userID, assessmentID uuid.UUID, subject, message string)

// ReminderOutcomeStatus says what one reminder did.
type ReminderOutcomeStatus string

const (
	// ReminderSent: a new link was issued and mailed.
	ReminderSent ReminderOutcomeStatus = "sent"
	// ReminderMailFailed: a new link was issued, the mail failed. The previous
	// link no longer works; the owner is told to resend. Not retried.
	ReminderMailFailed ReminderOutcomeStatus = "mail_failed"
	// ReminderMailUnavailable: no transport. No new link was issued, so the
	// vendor's current link keeps working; the owner is told to chase by hand.
	ReminderMailUnavailable ReminderOutcomeStatus = "mail_unavailable"
	// ReminderSkippedNoEntitlement: the tenant's plan no longer includes TPRM.
	ReminderSkippedNoEntitlement ReminderOutcomeStatus = "skipped_no_entitlement"
	// ReminderRaced: another sweep sent it, or the assessment closed meanwhile.
	ReminderRaced ReminderOutcomeStatus = "raced"
	// ReminderError: the entitlement or the store failed; nothing was sent.
	ReminderError ReminderOutcomeStatus = "error"
)

// ReminderOutcome is one row of a sweep, for the worker to log. It never
// carries a token.
type ReminderOutcome struct {
	TenantID     uuid.UUID
	AssessmentID uuid.UUID
	Offset       int
	Status       ReminderOutcomeStatus
	Err          error
}

// SendVendorAssessmentRemindersUseCase is one sweep of the J-7 / J-3 / J-1
// reminders (#672, ADR 0004 D6).
//
// Per due assessment, in order:
//  1. the tenant's plan must still include TPRM — checked once per tenant;
//  2. the reminder is STAMPED before anything is sent, and the new token is
//     minted in the same transaction. A reminder whose mail fails is therefore
//     never re-sent on every tick: a missed reminder is a smaller harm than an
//     hourly one, and it is never a duplicate;
//  3. the audit event is written, then the mail goes out, then the owner is
//     notified in-app with what happened.
//
// Without a mail transport, no new token is minted: superseding the vendor's
// working link without delivering a new one would strand them.
//
// On the audit event: ADR 0004 D6 wants it in the stamping transaction. The
// audit recorder keeps its own hash chain in its own store, so it is written
// right after the commit instead — best-effort, as every other TPRM audit event.
type SendVendorAssessmentRemindersUseCase struct {
	deps   AssessmentDeps
	store  ReminderStore
	notify ReminderNotifier
}

func NewSendVendorAssessmentRemindersUseCase(deps AssessmentDeps, store ReminderStore, notify ReminderNotifier) *SendVendorAssessmentRemindersUseCase {
	return &SendVendorAssessmentRemindersUseCase{deps: deps, store: store, notify: notify}
}

func (uc *SendVendorAssessmentRemindersUseCase) Execute(ctx context.Context) ([]ReminderOutcome, error) {
	now := uc.deps.now()
	rows, err := uc.store.ListOpenAssessmentsDueWithin(ctx, now, reminderHorizon)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}

	outcomes := []ReminderOutcome{}
	entitled := map[uuid.UUID]bool{}

	for i := range rows {
		a := rows[i]
		offset, due := a.ReminderDue(now)
		if !due {
			continue
		}
		outcome := ReminderOutcome{TenantID: a.TenantID, AssessmentID: a.ID, Offset: offset}

		allowed, checked := entitled[a.TenantID]
		if !checked {
			// Fails closed, like the public link: no checker, no reminder.
			if uc.deps.Features != nil {
				ok, _, _, ferr := uc.deps.Features.Allowed(ctx, a.TenantID, ent.FeatVendorRisk)
				if ferr != nil {
					outcome.Status, outcome.Err = ReminderError, ferr
					outcomes = append(outcomes, outcome)
					continue
				}
				allowed = ok
			}
			entitled[a.TenantID] = allowed
		}
		if !allowed {
			outcome.Status = ReminderSkippedNoEntitlement
			outcomes = append(outcomes, outcome)
			continue
		}

		var tok *domain.VendorAssessmentToken
		var token string
		if uc.deps.Mailer != nil {
			tok, token, err = domain.NewVendorAssessmentToken(a.TenantID, a.ID, domain.VendorTokenReasonReminder, now)
			if err != nil {
				outcome.Status, outcome.Err = ReminderError, err
				outcomes = append(outcomes, outcome)
				continue
			}
		}

		recorded, err := uc.store.RecordReminder(ctx, &a, offset, tok, now)
		if err != nil {
			outcome.Status, outcome.Err = ReminderError, err
			outcomes = append(outcomes, outcome)
			continue
		}
		if !recorded {
			outcome.Status = ReminderRaced
			outcomes = append(outcomes, outcome)
			continue
		}
		a.MarkRemindersSent(offset, now)

		outcome.Status = ReminderSent
		if uc.deps.Mailer == nil {
			outcome.Status = ReminderMailUnavailable
		} else {
			mail := QuestionnaireMail{
				To:               a.ContactEmail,
				Locale:           a.ContactLanguage,
				OrganizationName: uc.deps.organizationName(ctx, a.TenantID),
				VendorName:       uc.deps.vendorName(ctx, &a),
				QuestionnaireURL: uc.deps.questionnaireURL(token),
				DueAt:            a.DueAt,
				Reason:           domain.VendorTokenReasonReminder,
			}
			if merr := uc.deps.Mailer.SendQuestionnaire(ctx, mail); merr != nil {
				outcome.Status, outcome.Err = ReminderMailFailed, merr
			}
		}

		uc.deps.record(ctx, a.TenantID, uuid.Nil, domain.AuditActionUpdate, "vendor_assessment", a.ID.String(),
			fmt.Sprintf("Reminder J-%d for the questionnaire sent to %s: %s", offset, a.ContactEmail, outcome.Status),
			domain.JSONMap{
				"actor":       "system:vendor_assessment_reminders",
				"offset_days": offset,
				"delivery":    string(outcome.Status),
				"new_link":    tok != nil,
			})

		if uc.notify != nil && a.OwnerUserID != uuid.Nil {
			subject, message := reminderOwnerCopy(uc.deps.vendorName(ctx, &a), &a, offset, outcome.Status)
			uc.notify(ctx, a.TenantID, a.OwnerUserID, a.ID, subject, message)
		}
		outcomes = append(outcomes, outcome)
	}
	return outcomes, nil
}

// reminderOwnerCopy tells the owner what the reminder did, and what is theirs to
// do when it could not do its job.
func reminderOwnerCopy(vendorName string, a *domain.VendorAssessment, offset int, status ReminderOutcomeStatus) (string, string) {
	who := vendorName
	if who == "" {
		who = a.ContactEmail
	}
	subject := fmt.Sprintf("Relance fournisseur J-%d : %s", offset, who)
	lead := fmt.Sprintf("Le questionnaire envoyé à %s (%s) est dû le %s et n'est pas encore soumis.",
		who, a.ContactEmail, a.DueAt.UTC().Format("02/01/2006"))

	switch status {
	case ReminderMailFailed:
		return subject, lead + " La relance n'a pas pu être envoyée par e-mail et son ancien lien ne fonctionne plus : renvoyez le questionnaire depuis la fiche du fournisseur."
	case ReminderMailUnavailable:
		return subject, lead + " Aucun transport e-mail n'est configuré sur ce déploiement : relancez le fournisseur vous-même. Son lien actuel fonctionne toujours."
	default:
		return subject, lead + " Une relance lui a été envoyée avec un nouveau lien ; le précédent ne fonctionne plus."
	}
}
