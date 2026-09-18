// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// ListOpenAssessmentsDueWithin and RecordReminder make the in-memory
// assessmentStore a ReminderStore, with the repository's conditions: the list
// spans tenants, and the stamp is conditional on an open status and an unsent
// reminder.

func (s *assessmentStore) ListOpenAssessmentsDueWithin(_ context.Context, now time.Time, horizon time.Duration) ([]domain.VendorAssessment, error) {
	out := []domain.VendorAssessment{}
	for _, a := range s.assessments {
		if !isOpen(a) || !a.DueAt.After(now) || a.DueAt.After(now.Add(horizon)) {
			continue
		}
		if a.ReminderD7SentAt != nil && a.ReminderD3SentAt != nil && a.ReminderD1SentAt != nil {
			continue
		}
		a = copyAssessment(a)
		a.Items = nil
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DueAt.Before(out[j].DueAt) })
	return out, nil
}

func unsent(a domain.VendorAssessment, offset int) bool {
	switch offset {
	case 7:
		return a.ReminderD7SentAt == nil
	case 3:
		return a.ReminderD3SentAt == nil
	case 1:
		return a.ReminderD1SentAt == nil
	}
	return false
}

func (s *assessmentStore) RecordReminder(_ context.Context, a *domain.VendorAssessment, offset int, tok *domain.VendorAssessmentToken, at time.Time) (bool, error) {
	cur, ok := s.assessments[a.ID]
	if !ok || cur.TenantID != a.TenantID || !isOpen(cur) || !unsent(cur, offset) {
		return false, nil
	}
	cur = copyAssessment(cur)
	cur.MarkRemindersSent(offset, at)
	s.assessments[a.ID] = cur
	if tok != nil {
		for i := range s.tokens {
			if s.tokens[i].AssessmentID == a.ID && s.tokens[i].SupersededAt == nil {
				t := at
				s.tokens[i].SupersededAt = &t
			}
		}
		s.tokens = append(s.tokens, *tok)
	}
	return true, nil
}

type notice struct {
	tenantID, userID, assessmentID uuid.UUID
	subject, message               string
}

type noticeBoard struct{ notices []notice }

func (b *noticeBoard) notify(_ context.Context, tenantID, userID, assessmentID uuid.UUID, subject, message string) {
	b.notices = append(b.notices, notice{tenantID, userID, assessmentID, subject, message})
}
