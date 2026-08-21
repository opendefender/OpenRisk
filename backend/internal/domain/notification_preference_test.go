// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package domain

import "testing"

// W0-05 / D2 — the Settings screen offered eight notification switches, the API
// stored what they said, and every producer sent regardless. These tests pin the
// rule that finally reads them.

func TestNotificationPreference_Allows_NilIsPermissive(t *testing.T) {
	var np *NotificationPreference
	if !np.Allows(NotificationTypeCriticalRisk, NotificationChannelEmail) {
		t.Fatal("a user with no stored preferences must still be notified")
	}
}

func TestNotificationPreference_Allows_DisableAllWinsOverEverything(t *testing.T) {
	np := &NotificationPreference{
		DisableAllNotifications: true,
		// Every per-event switch says yes; the global switch must still win.
		EmailOnCriticalRisk:       true,
		EmailOnMitigationDeadline: true,
		EmailOnActionAssigned:     true,
		SlackEnabled:              true,
		SlackOnCriticalRisk:       true,
		WebhookEnabled:            true,
		WebhookOnCriticalRisk:     true,
	}

	channels := []NotificationChannel{
		NotificationChannelInApp,
		NotificationChannelEmail,
		NotificationChannelSlack,
		NotificationChannelWebhook,
	}
	for _, ch := range channels {
		if np.Allows(NotificationTypeCriticalRisk, ch) {
			t.Errorf("channel %q: DisableAllNotifications must silence every channel, in-app included", ch)
		}
	}
}

func TestNotificationPreference_Allows_EmailPerEvent(t *testing.T) {
	np := &NotificationPreference{
		EmailOnMitigationDeadline: false,
		EmailOnCriticalRisk:       true,
		EmailOnActionAssigned:     false,
		EmailOnRiskUpdate:         true,
		EmailOnRiskResolved:       false,
	}

	cases := []struct {
		notifType NotificationType
		want      bool
		why       string
	}{
		{NotificationTypeMitigationDeadline, false, "turned off"},
		{NotificationTypeCriticalRisk, true, "turned on"},
		{NotificationTypeActionAssigned, false, "turned off"},
		{NotificationTypeRiskUpdate, true, "turned on"},
		{NotificationTypeRiskResolved, false, "turned off"},
		// An SLA breach is the escalation half of a critical-risk alert and
		// follows the same switch rather than a column nobody set.
		{NotificationTypeSLABreach, true, "follows critical-risk"},
		// No dedicated column: allowed rather than silently dropped.
		{NotificationTypeScanComplete, true, "no column, fails open"},
		{NotificationTypeRiskReview, true, "no column, fails open"},
		{NotificationTypeAutomation, true, "no column, fails open"},
	}
	for _, c := range cases {
		if got := np.Allows(c.notifType, NotificationChannelEmail); got != c.want {
			t.Errorf("Allows(%q, email) = %v, want %v (%s)", c.notifType, got, c.want, c.why)
		}
	}
}

func TestNotificationPreference_Allows_ChannelMustBeEnabledFirst(t *testing.T) {
	// Slack and webhook need configuring before they can carry anything. A
	// per-event switch left at its permissive default must not be read as "yes,
	// send to Slack" when Slack was never connected — that is exactly the shape
	// of the fake-integration bug this wave exists to remove.
	np := &NotificationPreference{
		SlackEnabled:          false,
		SlackOnCriticalRisk:   true,
		WebhookEnabled:        false,
		WebhookOnCriticalRisk: true,
	}

	if np.Allows(NotificationTypeCriticalRisk, NotificationChannelSlack) {
		t.Error("Slack must not be used while SlackEnabled is false")
	}
	if np.Allows(NotificationTypeCriticalRisk, NotificationChannelWebhook) {
		t.Error("webhook must not be used while WebhookEnabled is false")
	}

	np.SlackEnabled = true
	if !np.Allows(NotificationTypeCriticalRisk, NotificationChannelSlack) {
		t.Error("once enabled, the per-event switch governs")
	}
	np.SlackOnCriticalRisk = false
	if np.Allows(NotificationTypeCriticalRisk, NotificationChannelSlack) {
		t.Error("per-event switch must be honoured on an enabled channel")
	}
}

func TestNotificationPreference_Allows_InAppOnlyGlobalSwitch(t *testing.T) {
	// The schema has EmailOn*/SlackOn*/WebhookOn* but no in-app equivalent, so
	// in-app delivery of a specific event cannot be filtered without inventing a
	// rule the user never set. Only the global switch applies.
	np := &NotificationPreference{
		EmailOnCriticalRisk:       false,
		EmailOnMitigationDeadline: false,
	}
	if !np.Allows(NotificationTypeCriticalRisk, NotificationChannelInApp) {
		t.Error("turning e-mail off must not silence the in-app bell")
	}

	np.DisableAllNotifications = true
	if np.Allows(NotificationTypeCriticalRisk, NotificationChannelInApp) {
		t.Error("the global switch must silence in-app")
	}
}
