// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// SessionRecord is one signed-in device, as shown in Settings → Sessions.
//
// A read model, not a table: the underlying rows are refresh tokens, and this
// is the shape a person can actually reason about — what device, from where,
// when last used, and is it the one I'm holding.
type SessionRecord struct {
	ID uuid.UUID `json:"id"`

	// TokenHash is used internally to identify the caller's own session and is
	// cleared before the record leaves the application layer. It is a
	// credential-equivalent lookup key and must never reach a client.
	TokenHash string `json:"-"`

	// Device is a human label derived from the User-Agent ("Chrome on macOS"),
	// because a raw UA string is not something a user can judge.
	Device    string `json:"device"`
	UserAgent string `json:"user_agent,omitempty"`
	IPAddress string `json:"ip_address,omitempty"`

	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`

	// Current marks the session making the request, so the UI can warn before
	// someone signs out the device they are using.
	Current bool `json:"current"`
}

// DescribeDevice turns a User-Agent into a short human label.
//
// Deliberately coarse. This is not analytics: the only question it has to
// answer is "do I recognise this?", and "Chrome on Windows" answers it while a
// 120-character UA string does not. Order matters in both passes — Edge and
// Opera advertise themselves as Chrome, and Chrome advertises itself as Safari,
// so the most specific brand has to be tested first.
func DescribeDevice(userAgent string) string {
	ua := strings.ToLower(userAgent)
	if strings.TrimSpace(ua) == "" {
		return "Unknown device"
	}

	browser := ""
	switch {
	case strings.Contains(ua, "edg/"), strings.Contains(ua, "edga/"), strings.Contains(ua, "edgios/"):
		browser = "Edge"
	case strings.Contains(ua, "opr/"), strings.Contains(ua, "opera"):
		browser = "Opera"
	case strings.Contains(ua, "firefox/"), strings.Contains(ua, "fxios/"):
		browser = "Firefox"
	case strings.Contains(ua, "chrome/"), strings.Contains(ua, "crios/"):
		browser = "Chrome"
	case strings.Contains(ua, "safari/"):
		browser = "Safari"
	case strings.Contains(ua, "curl/"):
		browser = "curl"
	case strings.Contains(ua, "postman"):
		browser = "Postman"
	case strings.Contains(ua, "openrisk"):
		browser = "OpenRisk agent"
	}

	os := ""
	switch {
	// iPadOS reports "Macintosh" in desktop mode, so iPad is tested before mac.
	case strings.Contains(ua, "iphone"):
		os = "iPhone"
	case strings.Contains(ua, "ipad"):
		os = "iPad"
	case strings.Contains(ua, "android"):
		os = "Android"
	case strings.Contains(ua, "windows"):
		os = "Windows"
	case strings.Contains(ua, "mac os"), strings.Contains(ua, "macintosh"):
		os = "macOS"
	case strings.Contains(ua, "cros"):
		os = "ChromeOS"
	case strings.Contains(ua, "linux"), strings.Contains(ua, "x11"):
		os = "Linux"
	}

	switch {
	case browser != "" && os != "":
		return browser + " on " + os
	case browser != "":
		return browser
	case os != "":
		return os
	default:
		return "Unknown device"
	}
}
