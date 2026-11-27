// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package crosstenant

import (
	"fmt"
	"strings"
)

// Observation is everything the live harness recorded about one probe. The
// evaluator is a pure function of it, so the rules that decide pass or fail are
// unit-tested without a server (TestHarness_DetectsLeak) and the live suite
// cannot quietly weaken them.
type Observation struct {
	// Collection is a list or aggregate GET with no id in its path: A is expected
	// to get a 2xx with only its own data. There is no random-UUID comparison.
	Collection bool
	// Aggregate marks a collection whose body is numbers or settings rather than
	// rows, so the control cannot show a B id in it. The control then proves the
	// route answers for B, not that B's data is in the body; the report says so.
	Aggregate bool

	ControlStatus int
	ControlBody   string
	// ControlHasB: the control body names one of B's ids or canaries. Required
	// for a non-aggregate collection, otherwise the list was empty and the probe
	// proves nothing.
	ControlHasB bool

	AttackStatus int
	AttackBody   string
	RandomStatus int

	// BeforeHash and AfterHash are B's own read of the target around A's attempt.
	// Verified is false when the kind has no read route.
	Verified              bool
	BeforeHash, AfterHash string
	// AfterStatus is B's read after the attempt; a DELETE must leave it at 200.
	AfterStatus int

	// Allow lists extra pass statuses for the attack, granted per probe with a
	// reason and a register row.
	Allow []int
	// Foreign are B's ids and canaries; no A body may contain one.
	Foreign []string
	// Supplied are the ids the attacker put in the request itself. An error that
	// repeats them is not a leak, so they are removed before the scan.
	Supplied []string
}

// Failure codes. They are stable so a report can quote them.
const (
	FailVacuous  = "VACUOUS"
	FailLeak     = "LEAK"
	FailServer   = "SERVER_ERROR"
	FailHarness  = "HARNESS_401"
	FailLimit    = "RATE_LIMITED"
	FailStatus   = "STATUS_NOT_ALLOWED"
	FailOracle   = "ORACLE"
	FailMutation = "MUTATION"
	FailEcho     = "ECHO"
)

// Evaluate returns the failures of one observation. An empty result is a pass.
func Evaluate(o Observation) []string {
	var out []string
	fail := func(code, format string, a ...any) {
		out = append(out, code+": "+fmt.Sprintf(format, a...))
	}

	// 1. Control. No control, no coverage.
	switch {
	case o.ControlStatus == 429 || o.AttackStatus == 429 || o.RandomStatus == 429:
		fail(FailLimit, "a request was rate limited; the run is invalid")
		return out
	case o.ControlStatus < 200 || o.ControlStatus > 299:
		fail(FailVacuous, "control (B's own token) answered %d, not 2xx", o.ControlStatus)
	case o.Collection && !o.Aggregate && !o.ControlHasB:
		fail(FailVacuous, "control list holds none of B's ids or canaries, so an empty list would pass")
	}

	if o.AttackStatus == 401 || o.RandomStatus == 401 {
		fail(FailHarness, "401 on a request that carries a valid token is a suite defect")
		return out
	}

	if o.Collection {
		if o.AttackStatus < 200 || o.AttackStatus > 299 {
			fail(FailStatus, "collection answered %d for A", o.AttackStatus)
		}
	} else {
		allowed := false
		for _, s := range o.Allow {
			if s == o.AttackStatus {
				allowed = true
			}
		}
		switch {
		case allowed:
			// pass, per probe, with a register row. Only an exact status
			// listed in Allow gets here; the oracle, mutation and echo rules
			// below still apply to it.
		case o.AttackStatus >= 200 && o.AttackStatus <= 299:
			fail(FailLeak, "A got %d for B's id", o.AttackStatus)
		case o.AttackStatus >= 500:
			fail(FailServer, "A got %d for B's id", o.AttackStatus)
		case o.AttackStatus == 404 || o.AttackStatus == 403:
			// pass
		default:
			fail(FailStatus, "A got %d for B's id; only 404 or 403 pass (400/422 mean validation ran before ownership)", o.AttackStatus)
		}
		if o.AttackStatus != o.RandomStatus {
			fail(FailOracle, "A gets %d for B's id but %d for a random id: the difference confirms B's object exists", o.AttackStatus, o.RandomStatus)
		}
	}

	if o.Verified {
		if o.BeforeHash != o.AfterHash {
			fail(FailMutation, "B's object changed after A's attempt")
		}
		if o.AfterStatus != 0 && (o.AfterStatus < 200 || o.AfterStatus > 299) {
			fail(FailMutation, "B's object answers %d after A's attempt", o.AfterStatus)
		}
	}

	if hits := echoes(o.AttackBody, o.Foreign, o.Supplied); len(hits) > 0 {
		fail(FailEcho, "A's response contains B data: %s", strings.Join(hits, ", "))
	}
	return out
}

// echoes returns the foreign strings found in body once the request's own
// values are removed.
func echoes(body string, foreign, supplied []string) []string {
	for _, s := range supplied {
		if s != "" {
			body = strings.ReplaceAll(body, s, "")
		}
	}
	var hits []string
	for _, f := range foreign {
		// A sequential integer id is a substring of almost any body, so an
		// integer cannot be searched for; those kinds rely on their canary.
		if len(f) < 8 {
			continue
		}
		if strings.Contains(body, f) {
			hits = append(hits, f)
		}
	}
	return hits
}

// OwnBodyObservation is what the harness recorded for a probe of the own-object
// shape: A's own object in the path, B's id in the body.
type OwnBodyObservation struct {
	// ControlStatus is the same request with A's own in-tenant ids: it must work.
	ControlStatus int
	// AttackStatus names B's id in the body; RandomStatus names a random one.
	AttackStatus, RandomStatus int
	AttackBody                 string
	// BeforeHash and AfterHash are A's own read of its object around the attempt;
	// AfterBody is the read after, scanned for anything of B's.
	BeforeHash, AfterHash, AfterBody string
	// Foreign are B's ids and canaries; Supplied are the ids the attacker sent.
	Foreign, Supplied []string
}

// EvaluateOwnBody returns the failures of an own-object probe. The refusal may be
// a 404 or 403 as elsewhere, or a typed validation error (400, 422); what it may
// not be is a success, a server error, or an answer that differs from the random
// id, and no link to B's id may be stored on A's object.
func EvaluateOwnBody(o OwnBodyObservation) []string {
	var out []string
	fail := func(code, format string, a ...any) {
		out = append(out, code+": "+fmt.Sprintf(format, a...))
	}
	switch {
	case o.ControlStatus == 429 || o.AttackStatus == 429 || o.RandomStatus == 429:
		fail(FailLimit, "a request was rate limited; the run is invalid")
		return out
	case o.ControlStatus < 200 || o.ControlStatus > 299:
		fail(FailVacuous, "control (A's own ids) answered %d, not 2xx", o.ControlStatus)
	}
	if o.AttackStatus == 401 || o.RandomStatus == 401 {
		fail(FailHarness, "401 on a request that carries a valid token is a suite defect")
		return out
	}
	switch o.AttackStatus {
	case 400, 403, 404, 422:
		// refused
	default:
		switch {
		case o.AttackStatus >= 200 && o.AttackStatus <= 299:
			fail(FailLeak, "A got %d for B's id in the body", o.AttackStatus)
		case o.AttackStatus >= 500:
			fail(FailServer, "A got %d for B's id in the body", o.AttackStatus)
		default:
			fail(FailStatus, "A got %d for B's id in the body; only 400, 403, 404 or 422 pass", o.AttackStatus)
		}
	}
	if o.AttackStatus != o.RandomStatus {
		fail(FailOracle, "A gets %d for B's id but %d for a random id: the difference confirms B's object exists", o.AttackStatus, o.RandomStatus)
	}
	if o.BeforeHash != o.AfterHash {
		fail(FailMutation, "A's own object changed after a request that named B's id")
	}
	if hits := echoes(o.AfterBody, o.Foreign, nil); len(hits) > 0 {
		fail(FailMutation, "A's own object now holds B data: %s", strings.Join(hits, ", "))
	}
	if hits := echoes(o.AttackBody, o.Foreign, o.Supplied); len(hits) > 0 {
		fail(FailEcho, "A's response contains B data: %s", strings.Join(hits, ", "))
	}
	return out
}
