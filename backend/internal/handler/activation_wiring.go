// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	appactivation "github.com/opendefender/openrisk/internal/application/activation"
)

// Package-level activation recorder for the LEGACY handlers that build their use
// case inline per request instead of receiving it from the DI container in
// main.go (mitigation is the case in point — it constructs its repositories from
// database.DB inside the handler function).
//
// This mirrors the existing SetSSOTokenManager seam. It is deliberately narrow:
// the recording still happens INSIDE the use case, which is where activation
// belongs; this only carries the dependency to a constructor main.go cannot reach.
// A handler retrofitted to proper DI should drop its use of this.
//
// nil is a valid value: the recorder is nil-safe, so an un-wired deployment
// simply records nothing rather than panicking.
var activationRecorder *appactivation.Recorder

// SetActivationRecorder injects the recorder from main.go. Call once at boot,
// before the server starts serving.
func SetActivationRecorder(rec *appactivation.Recorder) {
	activationRecorder = rec
}

// ActivationRecorderInstance returns the injected recorder (possibly nil).
func ActivationRecorderInstance() *appactivation.Recorder {
	return activationRecorder
}
