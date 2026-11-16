// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ctlExcNow = time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

func TestControlException_StatusAndActiveBoundaries(t *testing.T) {
	revokedAt := ctlExcNow
	ex := ControlException{StartsAt: ctlExcNow.Add(-time.Hour), ExpiresAt: ctlExcNow.Add(time.Hour)}

	cases := []struct {
		name   string
		ex     ControlException
		at     time.Time
		status ControlExceptionStatus
		active bool
	}{
		{"before start is open but not active", ex, ctlExcNow.Add(-2 * time.Hour), ControlExceptionOpen, false},
		{"at start is open and active", ex, ex.StartsAt, ControlExceptionOpen, true},
		{"inside the window", ex, ctlExcNow, ControlExceptionOpen, true},
		{"one nanosecond before expiry is open", ex, ex.ExpiresAt.Add(-time.Nanosecond), ControlExceptionOpen, true},
		{"at expires_at is expired", ex, ex.ExpiresAt, ControlExceptionExpired, false},
		{"after expiry is expired", ex, ex.ExpiresAt.Add(time.Minute), ControlExceptionExpired, false},
		{"revoked wins over open", withRevoked(ex, &revokedAt), ctlExcNow, ControlExceptionRevoked, false},
		{"revoked wins over expired", withRevoked(ex, &revokedAt), ex.ExpiresAt.Add(time.Hour), ControlExceptionRevoked, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.status, tc.ex.StatusAt(tc.at))
			assert.Equal(t, tc.active, tc.ex.ActiveAt(tc.at))
		})
	}
}

func withRevoked(e ControlException, at *time.Time) ControlException {
	e.RevokedAt = at
	return e
}

func TestNewControlException_Success(t *testing.T) {
	exp := ctlExcNow.Add(24 * time.Hour)
	ex, err := NewControlException(NewControlExceptionInput{
		TenantID: uuid.New(), ControlID: uuid.New(), OwnerID: uuid.New(), ApprovedBy: uuid.New(),
		Reason: "  vendor patch pending  ", ExpiresAt: &exp,
	}, ctlExcNow)
	require.NoError(t, err)
	assert.Equal(t, "vendor patch pending", ex.Reason)
	assert.Equal(t, ctlExcNow, ex.StartsAt, "starts_at defaults to the caller's instant")
	assert.Equal(t, ControlExceptionOpen, ex.StatusAt(ctlExcNow))
	assert.NotEqual(t, uuid.Nil, ex.ID)
}

func TestNewControlException_Validation(t *testing.T) {
	future := ctlExcNow.Add(time.Hour)
	past := ctlExcNow.Add(-time.Hour)
	zero := time.Time{}
	base := func() NewControlExceptionInput {
		return NewControlExceptionInput{TenantID: uuid.New(), ControlID: uuid.New(), OwnerID: uuid.New(), Reason: "why", ExpiresAt: &future}
	}
	cases := []struct {
		name string
		mut  func(*NewControlExceptionInput)
		code string
	}{
		{"no owner", func(i *NewControlExceptionInput) { i.OwnerID = uuid.Nil }, CtlExcOwnerRequired},
		{"blank reason", func(i *NewControlExceptionInput) { i.Reason = " \t\n" }, CtlExcReasonRequired},
		{"reason over 2000 runes", func(i *NewControlExceptionInput) { i.Reason = strings.Repeat("é", 2001) }, CtlExcReasonTooLong},
		{"no expires_at", func(i *NewControlExceptionInput) { i.ExpiresAt = nil }, CtlExcExpiresRequired},
		{"zero expires_at", func(i *NewControlExceptionInput) { i.ExpiresAt = &zero }, CtlExcExpiresRequired},
		{"expires_at in the past", func(i *NewControlExceptionInput) { i.ExpiresAt = &past }, CtlExcExpiresInPast},
		{"expires_at is now", func(i *NewControlExceptionInput) { n := ctlExcNow; i.ExpiresAt = &n }, CtlExcExpiresInPast},
		{"starts_at equals expires_at", func(i *NewControlExceptionInput) { s := future; i.StartsAt = &s }, CtlExcStartsNotBeforeEnds},
		{"starts_at after expires_at", func(i *NewControlExceptionInput) { s := future.Add(time.Minute); i.StartsAt = &s }, CtlExcStartsNotBeforeEnds},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := base()
			tc.mut(&in)
			_, err := NewControlException(in, ctlExcNow)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrValidation))
			assert.Equal(t, tc.code, MessageFromError(err))
		})
	}
	// 2000 runes exactly is accepted.
	in := base()
	in.Reason = strings.Repeat("é", 2000)
	_, err := NewControlException(in, ctlExcNow)
	assert.NoError(t, err)
}

func TestControlException_Revoke(t *testing.T) {
	newOpen := func() *ControlException {
		return &ControlException{StartsAt: ctlExcNow.Add(-time.Hour), ExpiresAt: ctlExcNow.Add(time.Hour)}
	}
	by := uuid.New()

	ex := newOpen()
	require.NoError(t, ex.Revoke(by, " done ", ctlExcNow))
	assert.Equal(t, ControlExceptionRevoked, ex.StatusAt(ctlExcNow))
	assert.Equal(t, by, *ex.RevokedBy)
	assert.Equal(t, "done", ex.RevokeReason)

	err := ex.Revoke(by, "", ctlExcNow)
	assert.True(t, errors.Is(err, ErrConflict), "revoking twice")

	expired := newOpen()
	err = expired.Revoke(by, "", expired.ExpiresAt)
	assert.True(t, errors.Is(err, ErrConflict), "nothing left to revoke on an expired exception")
	assert.Nil(t, expired.RevokedAt)

	err = newOpen().Revoke(by, strings.Repeat("x", 2001), ctlExcNow)
	assert.True(t, errors.Is(err, ErrValidation))
}
