// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

const day = 24 * time.Hour

func (h *harness) reminders(board *noticeBoard, deps AssessmentDeps) *SendVendorAssessmentRemindersUseCase {
	return NewSendVendorAssessmentRemindersUseCase(deps, h.as, board.notify)
}

func TestSendVendorAssessmentReminders_Success(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	board := &noticeBoard{}
	due := f.delivery.Assessment.DueAt
	h.now = due.Add(-6*day - 12*time.Hour)

	outcomes, err := h.reminders(board, h.deps()).Execute(context.Background())

	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	assert.Equal(t, ReminderSent, outcomes[0].Status)
	assert.Equal(t, 7, outcomes[0].Offset)
	assert.Equal(t, f.tenant, outcomes[0].TenantID)

	require.Len(t, h.mail.sent, 1)
	mail := h.mail.sent[0]
	assert.Equal(t, domain.VendorTokenReasonReminder, mail.Reason)
	assert.Equal(t, "security@acme.example", mail.To)
	assert.Equal(t, "Banque Exemple", mail.OrganizationName)
	newToken := tokenFromURL(t, mail.QuestionnaireURL)

	ctx := context.Background()
	_, err = NewResolvePublicAssessmentUseCase(h.deps()).Execute(ctx, f.token)
	assert.Equal(t, 410, domain.HTTPStatusFromError(err), "the previous link is superseded")
	_, err = NewResolvePublicAssessmentUseCase(h.deps()).Execute(ctx, newToken)
	assert.NoError(t, err, "the reminder's link works")

	stored := h.as.assessments[f.delivery.Assessment.ID]
	assert.NotNil(t, stored.ReminderD7SentAt)
	assert.Nil(t, stored.ReminderD3SentAt)

	require.Len(t, board.notices, 1)
	assert.Equal(t, f.tenant, board.notices[0].tenantID, "the notice is addressed with the row's tenant")
	assert.Equal(t, f.actor, board.notices[0].userID, "the owner is notified")
	assert.Contains(t, board.notices[0].subject, "J-7")

	for _, e := range h.audit.ofType("vendor_assessment") {
		raw, err := json.Marshal(e)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), newToken, "the audit trail never carries a token")
	}
}

func TestSendVendorAssessmentReminders_AtMostOncePerOffset(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	h.now = f.delivery.Assessment.DueAt.Add(-5 * day)
	uc := h.reminders(&noticeBoard{}, h.deps())

	first, err := uc.Execute(context.Background())
	require.NoError(t, err)
	require.Len(t, first, 1)

	h.now = h.now.Add(time.Hour) // the next hourly tick, and a restart after it
	second, err := uc.Execute(context.Background())
	require.NoError(t, err)
	third, err := NewSendVendorAssessmentRemindersUseCase(h.deps(), h.as, nil).Execute(context.Background())
	require.NoError(t, err)

	assert.Empty(t, second)
	assert.Empty(t, third)
	assert.Len(t, h.mail.sent, 1)
}

// ADR 0004 D6: after missed ticks, one reminder — the most imminent — never a burst.
func TestSendVendorAssessmentReminders_AfterMissedTicksSendsOnlyTheMostImminent(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	uc := h.reminders(&noticeBoard{}, h.deps())
	due := f.delivery.Assessment.DueAt

	// Nothing ran until two days before the deadline.
	h.now = due.Add(-2 * day)
	outcomes, err := uc.Execute(context.Background())
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	assert.Equal(t, 3, outcomes[0].Offset)
	stored := h.as.assessments[f.delivery.Assessment.ID]
	assert.NotNil(t, stored.ReminderD7SentAt, "J-7 is stamped: it will not be sent late")
	assert.NotNil(t, stored.ReminderD3SentAt)
	assert.Nil(t, stored.ReminderD1SentAt)

	h.now = due.Add(-12 * time.Hour)
	outcomes, err = uc.Execute(context.Background())
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	assert.Equal(t, 1, outcomes[0].Offset)

	h.now = due.Add(-1 * time.Hour)
	outcomes, err = uc.Execute(context.Background())
	require.NoError(t, err)
	assert.Empty(t, outcomes)
	assert.Len(t, h.mail.sent, 2, "J-3 then J-1; J-7 was never sent late")
}

func TestSendVendorAssessmentReminders_SkipsWhatMustNotBeChased(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, h *harness, f fixture)
		outcome []ReminderOutcomeStatus
	}{
		{"a submitted questionnaire", func(t *testing.T, h *harness, f fixture) { submitSent(t, h, f) }, nil},
		{"a revoked questionnaire", func(t *testing.T, h *harness, f fixture) {
			_, err := NewRevokeVendorAssessmentUseCase(h.deps()).Execute(context.Background(), f.tenant, f.actor, f.delivery.Assessment.ID)
			require.NoError(t, err)
		}, nil},
		{"a tenant whose plan lost TPRM", func(t *testing.T, h *harness, f fixture) {
			h.features.denied[f.tenant] = true
		}, []ReminderOutcomeStatus{ReminderSkippedNoEntitlement}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness()
			f := h.sendOne(t)
			tt.setup(t, h, f)
			h.now = f.delivery.Assessment.DueAt.Add(-1 * day)

			outcomes, err := h.reminders(&noticeBoard{}, h.deps()).Execute(context.Background())

			require.NoError(t, err)
			var got []ReminderOutcomeStatus
			for _, o := range outcomes {
				got = append(got, o.Status)
			}
			assert.Equal(t, tt.outcome, got)
			assert.Empty(t, h.mail.sent)
			stored := h.as.assessments[f.delivery.Assessment.ID]
			assert.Nil(t, stored.ReminderD1SentAt, "nothing stamped")
		})
	}

	t.Run("past the due date, within grace", func(t *testing.T) {
		h := newHarness()
		f := h.sendOne(t)
		h.now = f.delivery.Assessment.DueAt.Add(time.Hour)

		outcomes, err := h.reminders(&noticeBoard{}, h.deps()).Execute(context.Background())

		require.NoError(t, err)
		assert.Empty(t, outcomes)
		assert.Empty(t, h.mail.sent)
	})

	t.Run("no feature checker wired: fails closed", func(t *testing.T) {
		h := newHarness()
		f := h.sendOne(t)
		h.now = f.delivery.Assessment.DueAt.Add(-1 * day)
		deps := h.deps()
		deps.Features = nil

		outcomes, err := h.reminders(&noticeBoard{}, deps).Execute(context.Background())

		require.NoError(t, err)
		require.Len(t, outcomes, 1)
		assert.Equal(t, ReminderSkippedNoEntitlement, outcomes[0].Status)
		assert.Empty(t, h.mail.sent)
	})
}

func TestSendVendorAssessmentReminders_WithoutMailKeepsTheVendorsCurrentLink(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	board := &noticeBoard{}
	h.now = f.delivery.Assessment.DueAt.Add(-1 * day)

	outcomes, err := h.reminders(board, h.depsWithoutMail()).Execute(context.Background())

	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	assert.Equal(t, ReminderMailUnavailable, outcomes[0].Status)
	assert.Len(t, h.as.tokens, 1, "no token was minted")
	_, err = NewResolvePublicAssessmentUseCase(h.deps()).Execute(context.Background(), f.token)
	assert.NoError(t, err, "the vendor's link still works")
	assert.NotNil(t, h.as.assessments[f.delivery.Assessment.ID].ReminderD1SentAt, "stamped, so the owner is not told hourly")
	require.Len(t, board.notices, 1)
	assert.Contains(t, board.notices[0].message, "fonctionne toujours")
}

func TestSendVendorAssessmentReminders_AFailedMailIsReportedAndNotRetried(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	board := &noticeBoard{}
	h.mail.fail = true
	h.now = f.delivery.Assessment.DueAt.Add(-3 * day)
	uc := h.reminders(board, h.deps())

	outcomes, err := uc.Execute(context.Background())
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	assert.Equal(t, ReminderMailFailed, outcomes[0].Status)
	assert.Error(t, outcomes[0].Err)
	require.Len(t, board.notices, 1)
	assert.Contains(t, board.notices[0].message, "renvoyez le questionnaire")

	h.now = h.now.Add(time.Hour)
	again, err := uc.Execute(context.Background())
	require.NoError(t, err)
	assert.Empty(t, again, "a failed reminder is not re-sent every tick")
}

// Cross-tenant by necessity: every row addresses its own tenant.
func TestSendVendorAssessmentReminders_EachRowAddressesItsOwnTenant(t *testing.T) {
	h := newHarness()
	a := h.sendOne(t)
	b := h.sendOne(t)
	h.orgs[b.tenant] = "Autre Banque"
	board := &noticeBoard{}
	h.now = a.delivery.Assessment.DueAt.Add(-1 * day)

	outcomes, err := h.reminders(board, h.deps()).Execute(context.Background())

	require.NoError(t, err)
	require.Len(t, outcomes, 2)
	orgByTenant := map[string]string{}
	for _, m := range h.mail.sent {
		orgByTenant[m.OrganizationName] = m.To
	}
	assert.Contains(t, orgByTenant, "Banque Exemple")
	assert.Contains(t, orgByTenant, "Autre Banque")

	require.Len(t, board.notices, 2)
	for _, n := range board.notices {
		switch n.assessmentID {
		case a.delivery.Assessment.ID:
			assert.Equal(t, a.tenant, n.tenantID)
			assert.Equal(t, a.actor, n.userID)
		case b.delivery.Assessment.ID:
			assert.Equal(t, b.tenant, n.tenantID)
			assert.Equal(t, b.actor, n.userID)
		default:
			t.Fatalf("a notice for an assessment that was not due: %s", n.assessmentID)
		}
	}
	for _, e := range h.audit.ofType("vendor_assessment") {
		if e.After["actor"] == "system:vendor_assessment_reminders" {
			assert.Contains(t, []string{a.tenant.String(), b.tenant.String()}, e.TenantID.String())
			if e.EntityID == a.delivery.Assessment.ID.String() {
				assert.Equal(t, a.tenant, e.TenantID)
			}
		}
	}
}
