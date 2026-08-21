// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package domain

import (
	"bytes"
	"encoding/json"
	"testing"
)

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

// The GET and the PATCH must speak the same vocabulary, and neither may carry a
// credential. Both were broken until W0-05: encoding/json emitted Go field names
// ("DisableAllNotifications") while PATCH accepted snake_case, so no client
// could round-trip a preference — which is part of why the Settings screen kept
// its own copy in localStorage instead.
func TestNotificationPreference_JSONContract(t *testing.T) {
	prefs := &NotificationPreference{
		DisableAllNotifications: true,
		EmailOnCriticalRisk:     true,
		// Populated on purpose: these must not appear in the payload even when set.
		SlackWebhookURL: "https://hooks.slack.com/services/SECRET",
		WebhookURL:      "https://example.test/hook",
		WebhookSecret:   "hmac-secret",
	}

	raw, err := json.Marshal(prefs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// The keys the PATCH handler binds, so a client can read one and write it back.
	for _, key := range []string{
		"disable_all_notifications",
		"email_on_mitigation_deadline",
		"email_on_critical_risk",
		"email_on_action_assigned",
		"slack_enabled",
		"slack_on_critical_risk",
		"webhook_enabled",
		"enable_sound_notifications",
		"enable_desktop_notifications",
	} {
		if _, ok := payload[key]; !ok {
			t.Errorf("payload is missing %q — GET and PATCH must use the same names", key)
		}
	}

	// Go field names must be gone, or a client written against the response
	// cannot write it back.
	if _, ok := payload["DisableAllNotifications"]; ok {
		t.Error("payload still carries Go field names")
	}

	// RULE #6: no secret leaves the server, whatever the struct is holding.
	for _, forbidden := range []string{"SlackWebhookURL", "slack_webhook_url", "WebhookURL", "webhook_url", "WebhookSecret", "webhook_secret"} {
		if _, ok := payload[forbidden]; ok {
			t.Errorf("payload leaks %q", forbidden)
		}
	}
	if bytes.Contains(raw, []byte("hmac-secret")) || bytes.Contains(raw, []byte("hooks.slack.com")) {
		t.Errorf("a credential appears in the payload: %s", raw)
	}

	// The relations would ship a whole user record inside every response.
	for _, rel := range []string{"User", "Tenant", "user", "tenant"} {
		if _, ok := payload[rel]; ok {
			t.Errorf("payload carries the %q relation", rel)
		}
	}
}
