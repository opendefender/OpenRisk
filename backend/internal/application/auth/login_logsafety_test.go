// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// brokenAttempts fails every call, which makes login log from each of its
// degraded paths.
type brokenAttempts struct{}

func (brokenAttempts) Increment(context.Context, string, time.Duration) (int64, error) {
	return 0, errors.New("store down")
}
func (brokenAttempts) Lock(context.Context, string, time.Duration) (bool, error) {
	return false, errors.New("store down")
}
func (brokenAttempts) LockedFor(context.Context, string) (time.Duration, error) {
	return 0, errors.New("store down")
}
func (brokenAttempts) Delete(context.Context, string) error { return errors.New("store down") }

// Rule 6 at run time (#1317): a failed sign-in whose degraded paths log must
// not put the password, the address or the stored hash in the log.
func TestLogin_FailedSignInLogsNoSecret(t *testing.T) {
	uc, _, _, _ := newLoginHarness(t, domain.RoleUser)
	uc.WithAttemptLimits(brokenAttempts{})

	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	const typed = "Wrong-Typed-Passphrase-4711"
	_, err := uc.Execute(context.Background(), LoginInput{Email: "admin@opendefender.io", Password: typed})
	require.Error(t, err)

	out := buf.String()
	require.Contains(t, out, "login:", "the degraded path must have logged, or this test proves nothing")
	assert.NotContains(t, out, typed)
	assert.NotContains(t, out, "Ancre-Vitrail7-Cobalt", "the real password")
	assert.NotContains(t, out, "hashed:", "the stored hash")
}
