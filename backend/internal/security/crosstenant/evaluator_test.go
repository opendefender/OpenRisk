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

// TestHarness_DetectsStoredForeignLink proves the own-object evaluator is not
// vacuous: a stored link, a success, a server error and an oracle are each
// rejected, and a clean 400 still passes.
func TestHarness_DetectsStoredForeignLink(t *testing.T) {
	b := "22222222-3333-4444-5555-666666666666"
	clean := OwnBodyObservation{
		ControlStatus: 200, AttackStatus: 400, RandomStatus: 400,
		BeforeHash: "h", AfterHash: "h", AfterBody: `{"category_id":null}`, Foreign: []string{b},
	}
	if f := EvaluateOwnBody(clean); len(f) != 0 {
		t.Fatalf("a clean 400 must pass, got %v", f)
	}
	for name, mod := range map[string]func(*OwnBodyObservation) string{
		"link stored": func(o *OwnBodyObservation) string {
			o.AfterHash, o.AfterBody = "changed", `{"category_id":"`+b+`"}`
			return FailMutation
		},
		"accepted": func(o *OwnBodyObservation) string { o.AttackStatus, o.RandomStatus = 200, 200; return FailLeak },
		"server error": func(o *OwnBodyObservation) string {
			o.AttackStatus, o.RandomStatus = 500, 500
			return FailServer
		},
		"oracle":  func(o *OwnBodyObservation) string { o.AttackStatus, o.RandomStatus = 404, 400; return FailOracle },
		"vacuous": func(o *OwnBodyObservation) string { o.ControlStatus = 400; return FailVacuous },
	} {
		o := clean
		code := mod(&o)
		if f := EvaluateOwnBody(o); !hasFailure(f, code) {
			t.Errorf("%s: want %s, got %v", name, code, f)
		}
	}
}

// TestEvaluate_SuppliedIdsStripLongestFirst: a short supplied id that is a
// substring of a longer supplied id must not cut it, leaving a fragment of B's
// canary that reads as an echo (seen on score-engine/configs, #1521).
func TestEvaluate_SuppliedIdsStripLongestFirst(t *testing.T) {
	long := "xt-145e0dcd-b-scoreconfig-0b038c"
	body := `{"ID":"` + long + `","Name":"attacker"}`
	foreign := []string{"xt-145e0dcd-b-"}
	for _, supplied := range [][]string{{"38", long}, {long, "38"}} {
		if hits := echoes(body, foreign, supplied); len(hits) != 0 {
			t.Fatalf("supplied %v: the supplied id is not an echo, got %v", supplied, hits)
		}
	}
	// A genuine echo next to the supplied id is still caught.
	if hits := echoes(body+` "xt-145e0dcd-b-risk"`, foreign, []string{"38", long}); len(hits) != 1 {
		t.Fatalf("a real canary beside the supplied id must be reported, got %v", hits)
	}
}

// TestHarness_DropsForeignMode: on a route that drops an unresolvable id, a 200
// passes only when it matches the random id, the control's own id was applied,
// and A's object holds nothing of B's.
func TestHarness_DropsForeignMode(t *testing.T) {
	b := "22222222-3333-4444-5555-666666666666"
	clean := OwnBodyObservation{
		ControlStatus: 200, AttackStatus: 200, RandomStatus: 200,
		BeforeHash: "h", AfterHash: "changed", AfterBody: `{"assets":[]}`, Foreign: []string{b},
		DropsForeign: true, ControlApplied: true, LinkChecked: true,
	}
	if f := EvaluateOwnBody(clean); len(f) != 0 {
		t.Fatalf("a dropped foreign id must pass, got %v", f)
	}
	for name, mod := range map[string]func(*OwnBodyObservation) string{
		"link stored": func(o *OwnBodyObservation) string {
			o.AfterBody = `{"assets":[{"id":"` + b + `"}]}`
			return FailMutation
		},
		"field not applied": func(o *OwnBodyObservation) string { o.ControlApplied = false; return FailVacuous },
		// A's read filters assets by tenant, so only the store shows this link.
		"link hidden from the read": func(o *OwnBodyObservation) string { o.ForeignLinkStored = true; return FailMutation },
		"store not queried":         func(o *OwnBodyObservation) string { o.LinkChecked = false; return FailVacuous },
		"oracle":                    func(o *OwnBodyObservation) string { o.RandomStatus = 404; return FailOracle },
	} {
		o := clean
		code := mod(&o)
		if f := EvaluateOwnBody(o); !hasFailure(f, code) {
			t.Errorf("%s: want %s, got %v", name, code, f)
		}
	}
	// Without the mode, the same 200 is a leak.
	o := clean
	o.DropsForeign = false
	if f := EvaluateOwnBody(o); !hasFailure(f, FailLeak) {
		t.Errorf("a 200 outside the mode must be a leak, got %v", f)
	}
}
