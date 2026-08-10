// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"context"

	"github.com/opendefender/openrisk/pkg/pwpolicy"
)

// MinPasswordLength is the enforced minimum, re-exported so existing callers and
// tests keep a domain-level name for it.
//
// Audit finding F-05: the code enforced 8 characters while the README promised
// 12. The spec settles it at 12 — a security tool that accepts 8 loses
// credibility at the first customer audit.
const MinPasswordLength = pwpolicy.MinLength

// ValidatePassword enforces the password policy.
//
// This is the LOCAL half of the policy: length, character classes, and zxcvbn
// strength. It deliberately does not consult the breach corpus, because it has
// no context to cancel a network call with and is called from paths that must
// not block on a third party.
//
// The full policy — these gates plus HaveIBeenPwned, plus the user's own email
// and name fed to zxcvbn so a password rebuilt from the account scores as the
// giveaway it is — lives in pwpolicy.Policy and is what the reset and
// registration flows run. Both share one implementation, so the rules cannot
// drift between entry points the way 8-vs-12 already had.
//
// The returned error is a typed ValidationError whose message is actionable:
// telling someone their password is "invalid" without saying what is missing
// just produces repeated failed attempts.
func ValidatePassword(password string) error {
	a := pwpolicy.New().Assess(context.Background(), password, nil)
	if a.OK {
		return nil
	}
	return NewValidationError(a.Blocking[0].EN)
}

// AssessPassword returns the full local assessment, for callers that want to
// show a strength meter or list every reason rather than only the first.
func AssessPassword(password string, userInputs []string) pwpolicy.Assessment {
	return pwpolicy.New().Assess(context.Background(), password, userInputs)
}
