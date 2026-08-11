// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package domain

import (
	"testing"
	"time"
)

func TestComputeMitigationProgress_FromSubActions(t *testing.T) {
	cases := []struct {
		total, completed int
		want             int
	}{
		{total: 4, completed: 0, want: 0},
		{total: 4, completed: 1, want: 25},
		{total: 4, completed: 2, want: 50},
		{total: 4, completed: 4, want: 100},
		{total: 3, completed: 1, want: 33}, // integer division, deliberately
		{total: 3, completed: 2, want: 66},
	}
	for _, c := range cases {
		// The status is irrelevant once there is a checklist: the checklist IS
		// the progress. A PLANNED plan with every step done reads 100.
		for _, status := range []MitigationStatus{MitigationPlanned, MitigationInProgress, MitigationDone} {
			if got := ComputeMitigationProgress(status, c.total, c.completed); got != c.want {
				t.Errorf("ComputeMitigationProgress(%s, %d, %d) = %d, want %d",
					status, c.total, c.completed, got, c.want)
			}
		}
	}
}

// The reported bug: a plan with no checklist read 0 % even when it was DONE.
func TestComputeMitigationProgress_FallsBackToStatusWithoutSubActions(t *testing.T) {
	cases := map[MitigationStatus]int{
		MitigationPlanned:       0,
		MitigationInProgress:    50,
		MitigationReview:        50,
		MitigationDone:          100,
		MitigationCancelled:     0, // abandonment is not progress
		MitigationStatus("???"): 0,
	}
	for status, want := range cases {
		if got := ComputeMitigationProgress(status, 0, 0); got != want {
			t.Errorf("ComputeMitigationProgress(%s, 0, 0) = %d, want %d", status, got, want)
		}
	}
}

func TestComputeMitigationProgress_ClampsNonsense(t *testing.T) {
	if got := ComputeMitigationProgress(MitigationInProgress, 3, 9); got != 100 {
		t.Fatalf("more completed than total must clamp to 100, got %d", got)
	}
	if got := ComputeMitigationProgress(MitigationInProgress, 3, -2); got != 0 {
		t.Fatalf("a negative count must clamp to 0, got %d", got)
	}
}

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
}

func planDue(due time.Time, status MitigationStatus) *Mitigation {
	d := due
	return &Mitigation{Title: "Patch Log4j", DueDate: &d, Status: status}
}

func TestMitigation_IsOverdue(t *testing.T) {
	now := day(2026, time.August, 11)
	past, future := day(2026, time.August, 1), day(2026, time.August, 20)

	if !planDue(past, MitigationInProgress).IsOverdue(now) {
		t.Fatal("an unfinished plan past its date is overdue")
	}
	if planDue(future, MitigationInProgress).IsOverdue(now) {
		t.Fatal("a plan due later is not overdue")
	}
	// Finished or abandoned work cannot be late — there is nothing left to be
	// late for, and a red badge there is noise.
	if planDue(past, MitigationDone).IsOverdue(now) {
		t.Fatal("a completed plan is never overdue")
	}
	if planDue(past, MitigationCancelled).IsOverdue(now) {
		t.Fatal("a cancelled plan is never overdue")
	}
	if (&Mitigation{Status: MitigationInProgress}).IsOverdue(now) {
		t.Fatal("a plan with no due date cannot be overdue")
	}
}

func TestMitigation_DaysUntilDue_ComparesCalendarDays(t *testing.T) {
	// Late in the day vs early the next morning must still read "1 day", not
	// flip because of the hour the page happened to load.
	due := time.Date(2026, time.August, 12, 9, 0, 0, 0, time.UTC)
	m := &Mitigation{DueDate: &due}

	for _, now := range []time.Time{
		time.Date(2026, time.August, 11, 0, 30, 0, 0, time.UTC),
		time.Date(2026, time.August, 11, 23, 30, 0, 0, time.UTC),
	} {
		days, ok := m.DaysUntilDue(now)
		if !ok || days != 1 {
			t.Fatalf("DaysUntilDue at %v = %d (%v), want 1", now, days, ok)
		}
	}

	overdue, _ := m.DaysUntilDue(time.Date(2026, time.August, 15, 0, 0, 0, 0, time.UTC))
	if overdue != -3 {
		t.Fatalf("overdue must read negative, got %d", overdue)
	}

	if _, ok := (&Mitigation{}).DaysUntilDue(time.Now()); ok {
		t.Fatal("no due date → no answer, rather than a fabricated 0")
	}
}

func TestMitigation_DueReminder_SendsEachNudgeOnce(t *testing.T) {
	due := day(2026, time.August, 20)
	m := planDue(due, MitigationInProgress)

	// Too early.
	if _, ok := m.DueReminderDue(day(2026, time.August, 1)); ok {
		t.Fatal("nothing is due 19 days out")
	}

	// Inside the D-7 window.
	atD7 := day(2026, time.August, 14)
	offset, ok := m.DueReminderDue(atD7)
	if !ok || offset != 7 {
		t.Fatalf("expected the D-7 nudge, got %d (%v)", offset, ok)
	}
	m.MarkReminderSent(offset, atD7)

	// The same sweep an hour later must not resend it.
	if _, ok := m.DueReminderDue(atD7.Add(time.Hour)); ok {
		t.Fatal("D-7 must be sent exactly once")
	}

	// D-1.
	atD1 := day(2026, time.August, 19)
	offset, ok = m.DueReminderDue(atD1)
	if !ok || offset != 1 {
		t.Fatalf("expected the D-1 nudge, got %d (%v)", offset, ok)
	}
	m.MarkReminderSent(offset, atD1)

	// And nothing after that, however overdue it gets — the badge carries the
	// lateness, the inbox does not need to repeat it daily.
	if _, ok := m.DueReminderDue(day(2026, time.September, 1)); ok {
		t.Fatal("no further reminders after D-1")
	}
}

// A plan created inside the window must get the URGENT nudge, not the one it
// already blew past, and must not then send D-7 afterwards.
func TestMitigation_DueReminder_LateDiscoverySkipsToD1(t *testing.T) {
	due := day(2026, time.August, 20)
	m := planDue(due, MitigationInProgress)

	offset, ok := m.DueReminderDue(day(2026, time.August, 19))
	if !ok || offset != 1 {
		t.Fatalf("expected D-1, got %d (%v)", offset, ok)
	}
	m.MarkReminderSent(offset, day(2026, time.August, 19))

	if m.ReminderD7SentAt == nil {
		t.Fatal("reaching D-1 must close D-7 too, or the earlier nudge fires afterwards as noise")
	}
	if _, ok := m.DueReminderDue(day(2026, time.August, 19).Add(time.Hour)); ok {
		t.Fatal("nothing left to send")
	}
}

// A missed sweep (deploy, outage) must not swallow the nudge.
func TestMitigation_DueReminder_SurvivesAMissedTick(t *testing.T) {
	due := day(2026, time.August, 20)
	m := planDue(due, MitigationInProgress)

	// The worker was down through the whole D-7 window; the first tick back is
	// at D-3. The rule is "past the threshold and not yet sent".
	offset, ok := m.DueReminderDue(day(2026, time.August, 17))
	if !ok || offset != 7 {
		t.Fatalf("a late sweep must still send the missed D-7 nudge, got %d (%v)", offset, ok)
	}
}

func TestMitigation_DueReminder_SilentWhenFinishedOrUndated(t *testing.T) {
	due := day(2026, time.August, 20)
	now := day(2026, time.August, 19)

	for _, status := range []MitigationStatus{MitigationDone, MitigationCancelled} {
		if _, ok := planDue(due, status).DueReminderDue(now); ok {
			t.Fatalf("%s plans must not be nudged", status)
		}
	}
	if _, ok := (&Mitigation{Status: MitigationInProgress}).DueReminderDue(now); ok {
		t.Fatal("no due date → no reminder")
	}
}

func TestMitigation_ClearReminders_RestartsTheSchedule(t *testing.T) {
	due := day(2026, time.August, 20)
	m := planDue(due, MitigationInProgress)
	m.MarkReminderSent(7, day(2026, time.August, 14))

	// The deadline is pushed out; the nudges must fire again on the new schedule
	// rather than staying silent because the old date's were already sent.
	m.ClearReminders()
	if m.ReminderD7SentAt != nil || m.ReminderD1SentAt != nil {
		t.Fatal("ClearReminders must forget both stamps")
	}
	if _, ok := m.DueReminderDue(day(2026, time.August, 14)); !ok {
		t.Fatal("a postponed plan must be nudged again")
	}
}
