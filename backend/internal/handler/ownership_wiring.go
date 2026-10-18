// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/opendefender/openrisk/internal/application/mitigation"
	"github.com/opendefender/openrisk/internal/application/ownership"
)

// Package-level ownership service for the LEGACY handlers that build their use
// case inline per request rather than receiving it from the DI container in
// main.go — mitigation and incident are the cases in point.
//
// Same seam as SetActivationRecorder, and for the same reason: main.go cannot
// reach a constructor that lives inside a handler function. Validation and
// notification still happen inside the service, which is where they belong.
//
// nil is a valid value: every method is nil-safe, so an un-wired deployment
// assigns without membership validation and without notifications rather than
// panicking.
var ownershipService *ownership.Service

// SetOwnershipService injects the service from main.go. Call once at boot,
// before the server starts serving.
func SetOwnershipService(s *ownership.Service) { ownershipService = s }

// OwnershipServiceInstance returns the injected service (possibly nil).
func OwnershipServiceInstance() *ownership.Service { return ownershipService }

// Package-level notice dispatcher for the same legacy handlers (#757). The
// mitigation validation tells the plan's assignee it was approved through it.
//
// nil is valid: the use case then tells nobody, which is the state before
// notifications were wired — never a crash.
var noticeDeliverer mitigation.NoticeDeliverer

// SetNoticeDeliverer injects the dispatcher from main.go. Call once at boot.
func SetNoticeDeliverer(d mitigation.NoticeDeliverer) { noticeDeliverer = d }
