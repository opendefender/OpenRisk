// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"strings"

	"github.com/google/uuid"
)

// NotificationTypeMitigationValidated tells whoever did the work on a plan that
// its reviewer approved it (#757). No dedicated preference column: e-mail
// follows the global switch, as scan_complete does.
const NotificationTypeMitigationValidated NotificationType = "mitigation_validated"

// NotificationTypeRiskStateChanged tells a risk's owner that someone else moved
// it to another state that does not end it (#757). The owner hears about every
// change: there is no per-event column, so only the global "pause everything"
// switch silences it. Moves that end the risk are NotificationTypeRiskResolved
// and keep their own switch.
const NotificationTypeRiskStateChanged NotificationType = "risk_state_changed"

// NoticeMessage is the copy of one notification in one language.
//
// Lines are plain sentences. Whoever renders them escapes them: a risk title is
// typed by a user, and an e-mail body is HTML.
type NoticeMessage struct {
	Subject string
	Lines   []string
}

// InAppText is the body the notification bell shows.
func (m NoticeMessage) InAppText() string {
	return strings.Join(m.Lines, " ")
}

// Notice is one event addressed to one member of one tenant, before anyone
// knows which language that member reads.
//
// Compose is called once the recipient is resolved, with their locale ("fr" or
// "en"). The producer words the event; the dispatcher decides who may receive
// it, on which channel, in which language.
type Notice struct {
	TenantID     uuid.UUID
	UserID       uuid.UUID
	Type         NotificationType
	ResourceID   *uuid.UUID
	ResourceType string
	// ResourceTitle names the item in a digest e-mail ("3 changes to «Patch
	// VPN»"). Optional: the digest falls back to the kind of item.
	ResourceTitle string
	Compose       func(locale string) NoticeMessage
	// Deduplicated is set by a producer that already keeps its own ledger of
	// what it sent (the deadline sweep stamps each D-7 / D-1 / "passed"
	// reminder). Such a notice is e-mailed on its own and never folded into a
	// digest, which would hold the D-1 reminder back behind the D-7 (#997).
	Deduplicated bool
}

// NoticeLocale reduces a stored locale ("en-GB", "fr", "") to the two languages
// every notice is written in. French is the default: it is the product's first
// market, and the rest of the notification copy already defaults to it.
func NoticeLocale(raw string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(raw)), "en") {
		return "en"
	}
	return "fr"
}
