// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The vendor reminder notice has no preference column of its own (#672). It must
// still reach the owner in-app when their e-mail switches are off — and must
// not when they silenced everything.
func TestNotificationPreference_VendorReminderFollowsOnlyTheGlobalSwitch(t *testing.T) {
	quiet := &NotificationPreference{
		EmailOnMitigationDeadline: false,
		EmailOnCriticalRisk:       false,
		EmailOnActionAssigned:     false,
		EmailOnRiskUpdate:         false,
		EmailOnRiskResolved:       false,
	}
	assert.True(t, quiet.Allows(NotificationTypeVendorAssessmentReminder, NotificationChannelInApp))

	silenced := &NotificationPreference{DisableAllNotifications: true}
	assert.False(t, silenced.Allows(NotificationTypeVendorAssessmentReminder, NotificationChannelInApp))

	var none *NotificationPreference
	assert.True(t, none.Allows(NotificationTypeVendorAssessmentReminder, NotificationChannelInApp), "no stored preferences: defaults apply")
}
