// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import "strings"

// RegisterStatus is the five-value status the register shows and filters on.
//
// The status column carries two live vocabularies, side by side: open,
// in_progress, mitigated, accepted, closed, and the older DRAFT, ACTIVE,
// MITIGATED, ACCEPTED that the create path still writes. A filter that compares
// the raw column drops the legacy rows from the very filter that should show
// them. This is the ONE mapping from the register's value to what is stored; it
// mirrors the frontend's toStatusValue (features/risks/riskViews.ts), which
// reads the same rows.
type RegisterStatus string

const (
	RegisterOpen       RegisterStatus = "open"
	RegisterInProgress RegisterStatus = "in_progress"
	RegisterMitigated  RegisterStatus = "mitigated"
	RegisterAccepted   RegisterStatus = "accepted"
	RegisterClosed     RegisterStatus = "closed"
)

// registerStored lists, lower-cased, the stored values each non-open register
// status covers.
var registerStored = map[RegisterStatus][]string{
	RegisterInProgress: {"in_progress", "progress", "active"},
	RegisterMitigated:  {"mitigated"},
	RegisterAccepted:   {"accepted"},
	RegisterClosed:     {"closed", "done"},
}

// registerOpenKnown are the stored values that are explicitly "open". Open is
// also the catch-all: any stored value no other status claims (and NULL) reads
// as open, exactly as the frontend does.
var registerOpenKnown = []string{"open", "draft"}

// NormalizeStatus maps a stored status, in either vocabulary and any case, to
// the register value. Unknown and empty values are open.
func NormalizeStatus(raw string) RegisterStatus {
	v := strings.ToLower(strings.TrimSpace(raw))
	for status, stored := range registerStored {
		for _, s := range stored {
			if s == v {
				return status
			}
		}
	}
	return RegisterOpen
}

// parseRegisterStatus recognises a filter value: a register value or a stored
// alias of one. ok is false for anything else.
func parseRegisterStatus(raw string) (RegisterStatus, bool) {
	v := strings.ToLower(strings.TrimSpace(raw))
	for _, o := range registerOpenKnown {
		if v == o {
			return RegisterOpen, true
		}
	}
	if _, isRegister := registerStored[RegisterStatus(v)]; isRegister {
		return RegisterStatus(v), true
	}
	for status, stored := range registerStored {
		for _, s := range stored {
			if s == v {
				return status, true
			}
		}
	}
	return "", false
}

// NonOpenStoredValues is every stored value that is NOT open: what "open" is
// the complement of.
func NonOpenStoredValues() []string {
	var out []string
	for _, status := range []RegisterStatus{RegisterInProgress, RegisterMitigated, RegisterAccepted, RegisterClosed} {
		out = append(out, registerStored[status]...)
	}
	return out
}

// StoredStatusValues returns the lower-cased stored values a non-open register
// status covers, or nil for open (which is a complement, see NonOpenStoredValues).
func StoredStatusValues(s RegisterStatus) []string {
	return append([]string(nil), registerStored[s]...)
}

// StatusSelector is a status filter resolved against the stored vocabularies.
// A row matches when its lower-cased status is in In, or when IncludeOpen is
// set and it is not in NonOpen.
type StatusSelector struct {
	In          []string
	IncludeOpen bool
	NonOpen     []string
}

// SelectStatuses resolves a filter's values. A value that is neither a register
// status nor a known alias is kept as the raw stored value, so a filter that
// matched nothing before still matches nothing, and is not silently widened
// into "open".
func SelectStatuses(values []string) StatusSelector {
	sel := StatusSelector{}
	seen := map[string]bool{}
	add := func(s string) {
		if !seen[s] {
			seen[s] = true
			sel.In = append(sel.In, s)
		}
	}
	for _, raw := range values {
		status, ok := parseRegisterStatus(raw)
		switch {
		case !ok:
			add(strings.ToLower(strings.TrimSpace(raw)))
		case status == RegisterOpen:
			sel.IncludeOpen = true
		default:
			for _, s := range registerStored[status] {
				add(s)
			}
		}
	}
	if sel.IncludeOpen {
		sel.NonOpen = NonOpenStoredValues()
	}
	return sel
}
