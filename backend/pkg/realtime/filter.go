// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package realtime

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// MaxFilterEntries caps how many types or aggregates one subscription may name.
//
// A filter list is parsed and held for the life of a connection, so an
// unbounded one is a memory write a client controls. The cap is well above the
// whole catalog, so it can only ever refuse abuse.
const MaxFilterEntries = 64

var (
	// ErrFilterTooLarge is returned when a subscription names more entries than
	// MaxFilterEntries.
	ErrFilterTooLarge = fmt.Errorf("realtime: subscription filter exceeds %d entries", MaxFilterEntries)
	// ErrUnknownFilterType is returned when a subscription names an event type
	// that is not in the catalog.
	ErrUnknownFilterType = errors.New("realtime: subscription names an unknown event type")
	// ErrUnknownAggregate is returned when a subscription names an aggregate
	// that no catalog entry declares.
	ErrUnknownAggregate = errors.New("realtime: subscription names an unknown aggregate")
)

// Filter is a subscriber's narrowing request.
//
// It is a NARROWING request and nothing else. The stream is scoped to one tenant
// before a Filter is ever consulted, and Match is never the thing that keeps
// tenant A's events away from tenant B — the hub is. Getting that order wrong is
// how filtering becomes an isolation bypass: a subscriber that could widen its
// own filter would be one bug away from another tenant's data. Here, the widest
// filter expressible is "everything this tenant publishes", because that is the
// only set the subscription was ever handed.
//
// A zero Filter matches every event in the tenant's stream.
type Filter struct {
	Types      []EventType
	Aggregates []string
}

// ParseFilter builds a Filter from the raw query values a client sent.
//
// Both arguments are comma-separated lists; empty means "no narrowing on this
// dimension". An unknown name is an error rather than an ignored token: a client
// that subscribes to `risk.creted` and silently receives nothing has a bug that
// takes hours to find, while a 400 naming the token takes seconds.
func ParseFilter(types, aggregates string) (Filter, error) {
	var f Filter

	for _, raw := range splitList(types) {
		t := EventType(raw)
		if !IsRegistered(t) {
			return Filter{}, fmt.Errorf("%w: %q", ErrUnknownFilterType, raw)
		}
		f.Types = append(f.Types, t)
	}
	known := map[string]struct{}{}
	for _, a := range Aggregates() {
		known[a] = struct{}{}
	}
	for _, raw := range splitList(aggregates) {
		if _, ok := known[raw]; !ok {
			return Filter{}, fmt.Errorf("%w: %q", ErrUnknownAggregate, raw)
		}
		f.Aggregates = append(f.Aggregates, raw)
	}
	if len(f.Types)+len(f.Aggregates) > MaxFilterEntries {
		return Filter{}, ErrFilterTooLarge
	}

	sort.Slice(f.Types, func(i, j int) bool { return f.Types[i] < f.Types[j] })
	sort.Strings(f.Aggregates)
	return f, nil
}

func splitList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

// Match reports whether an envelope passes the filter.
//
// Types and Aggregates are OR-ed within themselves and AND-ed with each other,
// which is the only reading that makes both useful: "risk and asset events" is
// an aggregate list, "specifically these two risk events" is a type list, and
// asking for both means asking for their intersection.
func (f Filter) Match(e Envelope) bool {
	if len(f.Types) > 0 && !containsType(f.Types, e.Type) {
		return false
	}
	if len(f.Aggregates) > 0 && !containsString(f.Aggregates, e.Aggregate.Type) {
		return false
	}
	return true
}

// IsEmpty reports whether the filter narrows nothing.
func (f Filter) IsEmpty() bool { return len(f.Types) == 0 && len(f.Aggregates) == 0 }

// Describe renders the filter for a log line.
func (f Filter) Describe() string {
	if f.IsEmpty() {
		return "all"
	}
	var b strings.Builder
	if len(f.Types) > 0 {
		b.WriteString("types=")
		for i, t := range f.Types {
			if i > 0 {
				b.WriteByte('|')
			}
			b.WriteString(string(t))
		}
	}
	if len(f.Aggregates) > 0 {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString("aggregates=" + strings.Join(f.Aggregates, "|"))
	}
	return b.String()
}

func containsType(list []EventType, v EventType) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func containsString(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
