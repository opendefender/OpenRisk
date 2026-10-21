// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/authmfa"
)

// D-064 (#879): anyone can keep an address locked; the owner's usual browser
// still gets in, on a small budget of its own. Unknown devices stay locked.
func TestLogin_TrustedDeviceOnALockedAddress(t *testing.T) {
	ctx := context.Background()
	right := "Ancre-Vitrail7-Cobalt"

	locked := func(t *testing.T) (*LoginUseCase, *loginUsers) {
		t.Helper()
		uc, users, _ := newThrottleHarness(t)
		uc.WithAttemptLimits(authmfa.NewMemoryAttemptStore())
		for i := 0; i < LoginMaxFailuresPerAddress; i++ {
			failLogin(t, uc, users.user.Email)
		}
		return uc, users
	}
	deviceOf := func(userID uuid.UUID) *domain.TrustedDevice {
		return &domain.TrustedDevice{UserID: userID, DeviceID: uuid.New(), IssuedAt: time.Now()}
	}
	isLocked := func(err error) bool {
		var l *LoginLockedError
		return errors.As(err, &l)
	}

	t.Run("the account's own device gets in", func(t *testing.T) {
		uc, users := locked(t)
		_, err := uc.Execute(ctx, LoginInput{Email: users.user.Email, Password: right, TrustedDevice: deviceOf(users.user.ID)})
		require.NoError(t, err)

		// …and the address stays locked for everyone else.
		_, err = uc.Execute(ctx, LoginInput{Email: users.user.Email, Password: right})
		assert.True(t, isLocked(err), "a device sign-in must not lift the lock for others: %v", err)
	})

	t.Run("no cookie is still refused", func(t *testing.T) {
		uc, users := locked(t)
		_, err := uc.Execute(ctx, LoginInput{Email: users.user.Email, Password: right})
		assert.True(t, isLocked(err), "got %v", err)
	})

	t.Run("another account's device is no device", func(t *testing.T) {
		uc, users := locked(t)
		_, err := uc.Execute(ctx, LoginInput{Email: users.user.Email, Password: right, TrustedDevice: deviceOf(uuid.New())})
		assert.True(t, isLocked(err), "got %v", err)
	})

	t.Run("a device issued before the sessions were revoked is no device", func(t *testing.T) {
		uc, users := locked(t)
		revoked := time.Now()
		users.user.DevicesRevokedAt = &revoked
		d := deviceOf(users.user.ID)
		d.IssuedAt = revoked.Add(-time.Hour)
		_, err := uc.Execute(ctx, LoginInput{Email: users.user.Email, Password: right, TrustedDevice: d})
		assert.True(t, isLocked(err), "got %v", err)
	})

	t.Run("the device has a budget of its own", func(t *testing.T) {
		uc, users := locked(t)
		d := deviceOf(users.user.ID)
		for i := 1; i <= TrustedDeviceMaxFailures; i++ {
			_, err := uc.Execute(ctx, LoginInput{Email: users.user.Email, Password: "wrong-password-" + uuid.NewString(), TrustedDevice: d})
			require.False(t, isLocked(err), "failure %d is evaluated", i)
		}
		_, err := uc.Execute(ctx, LoginInput{Email: users.user.Email, Password: right, TrustedDevice: d})
		var l *LoginLockedError
		require.True(t, errors.As(err, &l), "got %v", err)
		assert.InDelta(t, TrustedDeviceLockDuration.Seconds(), l.RetryAfter.Seconds(), 1)
	})
}
