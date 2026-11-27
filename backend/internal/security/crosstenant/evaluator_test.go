// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package crosstenant

import (
	"strings"
	"testing"
)

// pass is a clean parameterised probe: 404 for B's id, 404 for a random id, an
// unchanged object, a B control that worked.
func pass() Observation {
	return Observation{
		ControlStatus: 200, AttackStatus: 404, RandomStatus: 404,
		Verified: true, BeforeHash: "h", AfterHash: "h", AfterStatus: 200,
		Foreign: []string{"b-canary-0123456789", "11111111-2222-3333-4444-555555555555"},
	}
}

func hasFailure(fails []string, code string) bool {
	for _, f := range fails {
		if strings.HasPrefix(f, code+":") {
			return true
		}
	}
	return false
}

// TestHarness_DetectsLeak proves the gate is not vacuous: every shape of leak the
// suite exists to catch is fed to the evaluator as a recorded response, and each
// must be rejected. A clean observation must still pass, otherwise rejecting
// everything would look like success.
func TestHarness_DetectsLeak(t *testing.T) {
	if f := Evaluate(pass()); len(f) != 0 {
		t.Fatalf("a clean 404 probe must pass, got %v", f)
	}
	cleanList := Observation{
		Collection: true, ControlStatus: 200, ControlHasB: true, AttackStatus: 200,
		AttackBody: `{"items":[{"id":"own"}]}`, Foreign: []string{"b-canary-0123456789"},
	}
	if f := Evaluate(cleanList); len(f) != 0 {
		t.Fatalf("a clean list must pass, got %v", f)
	}

	cases := []struct {
		name string
		obs  func() Observation
		want string
	}{
		{"200 with B's canary in the body", func() Observation {
			o := pass()
			o.AttackStatus, o.RandomStatus = 200, 200
			o.AttackBody = `{"title":"b-canary-0123456789"}`
			return o
		}, FailLeak},
		{"200 for B's id even with an empty body", func() Observation {
			o := pass()
			o.AttackStatus, o.RandomStatus = 200, 200
			return o
		}, FailLeak},
		{"403 for B's id but 404 for a random id (existence oracle)", func() Observation {
			o := pass()
			o.AttackStatus, o.RandomStatus = 403, 404
			return o
		}, FailOracle},
		{"422 that is not in Allow", func() Observation {
			o := pass()
			o.AttackStatus, o.RandomStatus = 422, 422
			return o
		}, FailStatus},
		{"400 that is not in Allow", func() Observation {
			o := pass()
			o.AttackStatus, o.RandomStatus = 400, 400
			return o
		}, FailStatus},
		{"500 on a foreign id", func() Observation {
			o := pass()
			o.AttackStatus, o.RandomStatus = 500, 500
			return o
		}, FailServer},
		{"failed control (B's own token got 404)", func() Observation {
			o := pass()
			o.ControlStatus = 404
			return o
		}, FailVacuous},
		{"failed control (B's own token got 400)", func() Observation {
			o := pass()
			o.ControlStatus = 400
			return o
		}, FailVacuous},
		{"empty control list", func() Observation {
			o := cleanList
			o.ControlHasB = false
			return o
		}, FailVacuous},
		{"B's object changed after A's attempt", func() Observation {
			o := pass()
			o.AfterHash = "other"
			return o
		}, FailMutation},
		{"B's object gone after A's DELETE", func() Observation {
			o := pass()
			o.AfterStatus = 404
			return o
		}, FailMutation},
		{"B's id echoed in a list A can read", func() Observation {
			o := cleanList
			o.AttackBody = `{"items":[{"id":"11111111-2222-3333-4444-555555555555"}]}`
			o.Foreign = []string{"11111111-2222-3333-4444-555555555555"}
			return o
		}, FailEcho},
		{"B's canary in a collection body", func() Observation {
			o := cleanList
			o.AttackBody = `{"items":[{"title":"b-canary-0123456789"}]}`
			return o
		}, FailEcho},
		{"collection answering 500", func() Observation {
			o := cleanList
			o.AttackStatus = 500
			return o
		}, FailStatus},
		{"401 on a request that carries a token", func() Observation {
			o := pass()
			o.AttackStatus = 401
			return o
		}, FailHarness},
		{"rate limited", func() Observation {
			o := pass()
			o.AttackStatus = 429
			return o
		}, FailLimit},
		{"an Allow does not excuse the oracle", func() Observation {
			o := pass()
			o.AttackStatus, o.RandomStatus = 400, 404
			o.Allow = []int{400}
			return o
		}, FailOracle},
		{"an Allow does not excuse an echo", func() Observation {
			o := pass()
			o.AttackStatus, o.RandomStatus = 400, 400
			o.Allow = []int{400}
			o.AttackBody = `{"error":"b-canary-0123456789"}`
			return o
		}, FailEcho},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fails := Evaluate(tc.obs())
			if !hasFailure(fails, tc.want) {
				t.Fatalf("the evaluator accepted a leak it must reject: want %s, got %v", tc.want, fails)
			}
		})
	}
}

// TestEvaluate_AllowedStatusPasses: a status listed in Allow passes only when
// the oracle, mutation and echo rules also hold, so Allow is a narrow tool.
func TestEvaluate_AllowedStatusPasses(t *testing.T) {
	o := pass()
	o.AttackStatus, o.RandomStatus = 400, 400
	o.Allow = []int{400}
	if f := Evaluate(o); len(f) != 0 {
		t.Fatalf("an allowed 400 with every other rule satisfied must pass, got %v", f)
	}
	o.AttackStatus, o.RandomStatus = 201, 201
	o.Allow = []int{201}
	if f := Evaluate(o); len(f) != 0 {
		t.Fatalf("an allowed 201 with every other rule satisfied must pass, got %v", f)
	}
}

// TestEvaluate_RequestsOwnIdsAreNotAnEcho: an error that repeats the id the
// attacker supplied is not a leak.
func TestEvaluate_RequestsOwnIdsAreNotAnEcho(t *testing.T) {
	o := pass()
	o.AttackBody = `{"error":"risk 11111111-2222-3333-4444-555555555555 not found"}`
	o.Supplied = []string{"11111111-2222-3333-4444-555555555555"}
	if f := Evaluate(o); len(f) != 0 {
		t.Fatalf("an echo of the supplied id must not fail, got %v", f)
	}
}
