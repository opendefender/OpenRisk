// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var mfaNow = time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

func TestDecideMFA_EnrolledIsDoneWhateverTheRole(t *testing.T) {
	for _, privileged := range []bool{false, true} {
		d := DecideMFA(MFADecisionInput{
			Enrolled: true, Privileged: privileged,
			GraceStartedAt: mfaNow.Add(-365 * 24 * time.Hour), GraceDays: 7, Now: mfaNow,
		})
		assert.Equal(t, MFAStateConfigured, d.State)
		assert.True(t, d.Configured)
		assert.False(t, d.Required, "an enrolled account is never blocked, even long past any deadline")
		assert.Nil(t, d.Deadline)
	}
}

func TestDecideMFA_StandardMemberIsNeverRequired(t *testing.T) {
	// THE OR26-03 CHANGE. An ordinary member with no authenticator reaches the
	// product. Anything else is a wall in front of the value.
	d := DecideMFA(MFADecisionInput{
		Enrolled: false, Privileged: false,
		GraceStartedAt: mfaNow.Add(-3650 * 24 * time.Hour), GraceDays: 7, Now: mfaNow,
	})
	assert.Equal(t, MFAStateRecommended, d.State)
	assert.False(t, d.Required)
	assert.False(t, d.Privileged)
	assert.Nil(t, d.Deadline, "a member the requirement does not apply to has no deadline to show")
}

func TestDecideMFA_PrivilegedInsideTheWindowMayDefer(t *testing.T) {
	started := mfaNow.Add(-2 * 24 * time.Hour)
	d := DecideMFA(MFADecisionInput{Privileged: true, GraceStartedAt: started, GraceDays: 7, Now: mfaNow})

	assert.Equal(t, MFAStateGraceActive, d.State)
	assert.False(t, d.Required, "an admin two days into a seven-day window keeps working")
	assert.True(t, d.GraceActive)
	require.NotNil(t, d.Deadline)
	assert.Equal(t, started.Add(7*24*time.Hour), *d.Deadline)
}

func TestDecideMFA_PrivilegedEscalatesInTheFinalStretch(t *testing.T) {
	// A separate state so the banner can shout without the client inventing a
	// threshold of its own — two clients would invent two.
	started := mfaNow.Add(-6 * 24 * time.Hour) // 24h left of 7 days
	d := DecideMFA(MFADecisionInput{Privileged: true, GraceStartedAt: started, GraceDays: 7, Now: mfaNow})

	assert.Equal(t, MFAStateGraceExpiring, d.State)
	assert.False(t, d.Required)
	assert.True(t, d.GraceActive)
}

func TestDecideMFA_PrivilegedPastTheWindowIsRequired(t *testing.T) {
	// Invariant 3: a privileged account cannot stay without MFA indefinitely.
	started := mfaNow.Add(-8 * 24 * time.Hour)
	d := DecideMFA(MFADecisionInput{Privileged: true, GraceStartedAt: started, GraceDays: 7, Now: mfaNow})

	assert.Equal(t, MFAStateRequired, d.State)
	assert.True(t, d.Required)
	assert.False(t, d.GraceActive)
	require.NotNil(t, d.Deadline)
}

func TestDecideMFA_DeadlineIsInclusive(t *testing.T) {
	// Exactly at the deadline the grace is over — a boundary that reads "still
	// fine" is one that quietly grants an extra request window.
	started := mfaNow.Add(-7 * 24 * time.Hour)
	d := DecideMFA(MFADecisionInput{Privileged: true, GraceStartedAt: started, GraceDays: 7, Now: mfaNow})
	assert.True(t, d.Required)
}

func TestDecideMFA_ZeroGraceDaysRequiresImmediately(t *testing.T) {
	// The pre-OR26-03 behaviour, still expressible for deployments that want it.
	d := DecideMFA(MFADecisionInput{Privileged: true, GraceStartedAt: mfaNow, GraceDays: 0, Now: mfaNow})
	assert.Equal(t, MFAStateRequired, d.State)
	assert.True(t, d.Required)
}

func TestDecideMFA_UnknownAnchorFailsClosed(t *testing.T) {
	// A missing timestamp must never read as infinite grace: that is exactly the
	// state someone would try to produce.
	d := DecideMFA(MFADecisionInput{Privileged: true, GraceStartedAt: time.Time{}, GraceDays: 7, Now: mfaNow})
	assert.Equal(t, MFAStateRequired, d.State)
	assert.True(t, d.Required)
}

func TestMFAPolicy_ValidateBounds(t *testing.T) {
	require.NoError(t, MFAPolicy{GraceDays: 0}.Validate())
	require.NoError(t, MFAPolicy{GraceDays: 7}.Validate())
	require.NoError(t, MFAPolicy{GraceDays: 90}.Validate())

	err := MFAPolicy{GraceDays: -1}.Validate()
	require.Error(t, err)
	appErr, ok := err.(*AppError)
	require.True(t, ok)
	assert.Equal(t, ErrValidation, appErr.Err)

	require.Error(t, MFAPolicy{GraceDays: 91}.Validate(),
		"an unbounded value would turn 'configure the policy' into 'switch it off'")
}

func TestMFAPolicy_EffectiveGraceDaysClampsOutOfBandRows(t *testing.T) {
	// A row written by a direct SQL edit must not be able to express "never".
	assert.Equal(t, MFAGraceDaysMax, MFAPolicy{GraceDays: 100000}.EffectiveGraceDays())
	assert.Equal(t, MFAGraceDaysMin, MFAPolicy{GraceDays: -5}.EffectiveGraceDays())
	assert.Equal(t, 7, MFAPolicy{GraceDays: 7}.EffectiveGraceDays())
}

func TestDefaultMFAPolicyIsSevenDays(t *testing.T) {
	p := DefaultMFAPolicy(uuid.New())
	assert.Equal(t, MFAGraceDaysDefault, p.GraceDays)
	assert.Equal(t, 7, MFAGraceDaysDefault)
}

func TestMFAPrivilegeSet_CoversBothRoleVocabularies(t *testing.T) {
	orgRoles, businessRoles := DefaultMFAPrivilegeRoles()
	set := NewMFAPrivilegeSet(orgRoles, businessRoles)

	assert.True(t, set.Includes(RoleAdmin, ""), "org role admin is privileged")
	assert.True(t, set.Includes(RoleRoot, ""), "org role root is privileged")
	assert.True(t, set.Includes(RoleUser, BusinessRoleRSSI),
		"the security officer is privileged through the business-role preset, not the org role")
	assert.False(t, set.Includes(RoleUser, BusinessRoleViewer))
	assert.False(t, set.Includes(RoleUser, ""))
}

func TestMFAPrivilegeSet_MatchingIsCaseInsensitive(t *testing.T) {
	set := NewMFAPrivilegeSet([]string{"Admin"}, []string{"RSSI"})
	assert.True(t, set.Includes(MemberRole("ADMIN"), ""))
	assert.True(t, set.Includes(RoleUser, BusinessRoleKey("rssi")))
}

func TestMFAPrivilegeSet_EmptyDisablesTheMandate(t *testing.T) {
	set := NewMFAPrivilegeSet(nil, nil)
	assert.True(t, set.Empty())
	assert.False(t, set.Includes(RoleAdmin, BusinessRoleRSSI))
}

func TestMFAPrivilegeSet_IgnoresBlankEntries(t *testing.T) {
	set := NewMFAPrivilegeSet([]string{"", "  ", "admin"}, []string{""})
	assert.Len(t, set.OrgRoles, 1)
	assert.True(t, set.Empty() == false)
	assert.False(t, set.Includes(RoleUser, ""), "a blank business role must not match a blank entry")
}

func TestMFAPolicy_IsAuditable(t *testing.T) {
	// Invariant 7: policy changes are audited. Implementing Auditable is what
	// makes the GORM plugin capture them without any call site remembering to.
	var a Auditable = MFAPolicy{}
	assert.Equal(t, "mfa_policy", a.AuditEntityType())
}
