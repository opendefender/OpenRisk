// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestVendorAssessment_ReminderDue(t *testing.T) {
	due := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	stamped := due.Add(-10 * 24 * time.Hour)
	day := 24 * time.Hour

	tests := []struct {
		name       string
		status     VendorAssessmentStatus
		now        time.Time
		d7, d3, d1 *time.Time
		wantOffset int
		wantDue    bool
	}{
		{"ten days out: nothing yet", VendorAssessmentSent, due.Add(-10 * day), nil, nil, nil, 0, false},
		{"just over seven days: not yet", VendorAssessmentSent, due.Add(-7*day - time.Hour), nil, nil, nil, 0, false},
		{"exactly seven days: J-7", VendorAssessmentSent, due.Add(-7 * day), nil, nil, nil, 7, true},
		{"five days: J-7", VendorAssessmentInProgress, due.Add(-5 * day), nil, nil, nil, 7, true},
		{"five days, J-7 already sent: nothing", VendorAssessmentSent, due.Add(-5 * day), &stamped, nil, nil, 0, false},
		{"three days: J-3", VendorAssessmentSent, due.Add(-3 * day), &stamped, nil, nil, 3, true},
		{"two days, nothing ever sent: only J-3, not J-7", VendorAssessmentSent, due.Add(-2 * day), nil, nil, nil, 3, true},
		{"two days, J-3 already sent: nothing", VendorAssessmentSent, due.Add(-2 * day), nil, &stamped, nil, 0, false},
		{"one day: J-1", VendorAssessmentSent, due.Add(-1 * day), &stamped, &stamped, nil, 1, true},
		{"twelve hours, nothing ever sent: only J-1", VendorAssessmentSent, due.Add(-12 * time.Hour), nil, nil, nil, 1, true},
		{"everything sent: nothing", VendorAssessmentSent, due.Add(-12 * time.Hour), &stamped, &stamped, &stamped, 0, false},
		{"past the due date, within grace: not chased", VendorAssessmentSent, due.Add(time.Hour), nil, nil, nil, 0, false},
		{"submitted: never", VendorAssessmentSubmitted, due.Add(-1 * day), nil, nil, nil, 0, false},
		{"revoked: never", VendorAssessmentRevoked, due.Add(-1 * day), nil, nil, nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &VendorAssessment{Status: tt.status, DueAt: due, ReminderD7SentAt: tt.d7, ReminderD3SentAt: tt.d3, ReminderD1SentAt: tt.d1}
			offset, ok := a.ReminderDue(tt.now)
			assert.Equal(t, tt.wantDue, ok)
			assert.Equal(t, tt.wantOffset, offset)
		})
	}
}

func TestVendorAssessment_MarkRemindersSentStampsLargerOffsetsAndKeepsEarlierStamps(t *testing.T) {
	earlier := time.Date(2026, 3, 13, 9, 0, 0, 0, time.UTC)
	now := earlier.Add(5 * 24 * time.Hour)
	a := &VendorAssessment{ReminderD7SentAt: &earlier}

	a.MarkRemindersSent(1, now)

	assert.Equal(t, earlier, *a.ReminderD7SentAt, "the J-7 stamp records when J-7 really went out")
	if assert.NotNil(t, a.ReminderD3SentAt) {
		assert.Equal(t, now, *a.ReminderD3SentAt)
	}
	if assert.NotNil(t, a.ReminderD1SentAt) {
		assert.Equal(t, now, *a.ReminderD1SentAt)
	}

	fresh := &VendorAssessment{}
	fresh.MarkRemindersSent(7, now)
	assert.NotNil(t, fresh.ReminderD7SentAt)
	assert.Nil(t, fresh.ReminderD3SentAt, "a smaller offset is not stamped by a larger reminder")
	assert.Nil(t, fresh.ReminderD1SentAt)
}
