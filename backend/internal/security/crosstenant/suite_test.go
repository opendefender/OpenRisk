//go:build crosstenant

// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package crosstenant

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestCrossTenant_Seeds proves every seed kind can be created for both tenants.
// A kind that fails to seed would make its probes fail as VACUOUS, so this names
// the cause once instead of once per probe.
func TestCrossTenant_Seeds(t *testing.T) {
	h := newHarness(t)
	kinds := make([]string, 0, len(seeders))
	for k := range seeders {
		kinds = append(kinds, string(k))
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		for _, p := range []*principal{h.A, h.B} {
			if h.seedShared(p, Kind(k)) == nil {
				t.Errorf("seed %s for tenant %s failed: the create route refused it", k, p.name)
			}
		}
	}
}

// TestCrossTenant runs every probe, one subtest per route, grouped by the first
// path segment after /api/v1.
func TestCrossTenant(t *testing.T) {
	h := newHarness(t)
	h.seedBoth(t)

	groups := map[string][]Probe{}
	for _, p := range allProbes() {
		g := Group(p.Path)
		groups[g] = append(groups[g], p)
	}
	names := make([]string, 0, len(groups))
	for g := range groups {
		names = append(names, g)
	}
	sort.Strings(names)

	for _, g := range names {
		t.Run(g, func(t *testing.T) {
			for _, p := range groups[g] {
				t.Run(p.ID(), func(t *testing.T) {
					o, supplied, err := h.observe(p)
					if err != nil {
						t.Fatalf("suite defect: %v", err)
					}
					o.Allow = p.Allow
					// Ids A supplied in any probe so far are not a leak when a
					// later read shows them: A named them itself, so reading
					// them back is not B's data surfacing. Writes that named a
					// foreign id are refused with 404 since #1520, so this only
					// covers ids a probe was allowed to echo.
					h.supplied = append(h.supplied, supplied...)
					o.Supplied = append([]string(nil), h.supplied...)
					o.Foreign = h.B.foreign(h.run)
					if fails := Evaluate(o); len(fails) > 0 {
						for _, f := range fails {
							t.Errorf("%s: %s", p.ID(), f)
						}
						t.Logf("evidence: attack=%d %.700q | random=%d | control=%d %.200q",
							o.AttackStatus, o.AttackBody, o.RandomStatus, o.ControlStatus, o.ControlBody)
					}
				})
			}
		})
	}
	// Guard silence (the boot log has no tenantguard line) is checked by
	// scripts/ci/cross-tenant.sh, which owns the log.
}

// observe runs one probe and records what happened. The order matters: B's read
// before, A's attack, A's random-id twin, B's read after, and the control last,
// because a destructive control removes the victim.
func (h *harness) observe(p Probe) (Observation, []string, error) {
	o := Observation{Collection: p.Collection, Aggregate: p.Aggregate}
	if p.Collection {
		return h.observeCollection(p, o)
	}

	ids := map[Kind]string{}
	fresh := p.Method != "GET"
	for i := len(p.Kinds) - 1; i >= 0; i-- {
		k := p.Kinds[i]
		if IsLiteral(k) {
			continue
		}
		if _, ok := ids[k]; ok {
			continue
		}
		var s map[Kind]string
		if fresh {
			s = h.seedFresh(h.B, k)
		} else {
			s = h.seedShared(h.B, k)
		}
		if s == nil {
			return o, nil, fmt.Errorf("cannot seed %s for tenant B", k)
		}
		for sk, sid := range s {
			if _, ok := ids[sk]; !ok {
				ids[sk] = sid
			}
		}
	}

	path, body, supplied := h.render(p, ids, false)
	rpath, rbody, _ := h.render(p, ids, true)

	readPath := h.readPath(p, ids)
	if readPath != "" {
		o.Verified = true
		_, before := h.do(h.B, "GET", readPath, "")
		o.BeforeHash = hashBody(before)
	}

	o.AttackStatus, o.AttackBody = h.do(h.A, p.Method, path, body)
	o.RandomStatus, _ = h.do(h.A, p.Method, rpath, rbody)

	if readPath != "" {
		var after string
		o.AfterStatus, after = h.do(h.B, "GET", readPath, "")
		o.AfterHash = hashBody(after)
	}

	o.ControlStatus, o.ControlBody = h.do(h.B, p.Method, path, body)
	return o, supplied, nil
}

func (h *harness) observeCollection(p Probe, o Observation) (Observation, []string, error) {
	var bIDs []string
	path, _, _ := h.render(p, nil, false)
	for _, k := range p.Kinds[min(ParamCount(p.Path), len(p.Kinds)):] {
		for _, pr := range []*principal{h.A, h.B} {
			s := h.seedShared(pr, k)
			if s == nil {
				return o, nil, fmt.Errorf("cannot seed %s for tenant %s", k, pr.name)
			}
			if pr == h.B {
				for _, id := range s {
					bIDs = append(bIDs, id)
				}
				bIDs = append(bIDs, h.canary(h.B, k))
			}
		}
	}
	o.AttackStatus, o.AttackBody = h.do(h.A, "GET", path, "")
	o.ControlStatus, o.ControlBody = h.do(h.B, "GET", path, "")
	for _, id := range bIDs {
		if strings.Contains(o.ControlBody, id) {
			o.ControlHasB = true
		}
	}
	return o, nil, nil
}

// render fills the path params and the body placeholders with B's ids, or with
// random ids of the same shape for the oracle twin.
func (h *harness) render(p Probe, ids map[Kind]string, random bool) (string, string, []string) {
	var supplied []string
	val := func(k Kind) string {
		if IsLiteral(k) {
			return strings.TrimPrefix(string(k), "=")
		}
		if random {
			if Kinds[k].IntID {
				return strconv.Itoa(900000000 + rand.Intn(99999999))
			}
			return newUUID()
		}
		supplied = append(supplied, ids[k])
		return ids[k]
	}
	segs := strings.Split(p.Path, "/")
	n := 0
	for i, s := range segs {
		if strings.HasPrefix(s, ":") {
			segs[i] = val(p.Kinds[n])
			n++
		}
	}
	body := strings.ReplaceAll(strings.ReplaceAll(p.Body, "{{run}}", h.run), "{{in30d}}", time.Now().UTC().AddDate(0, 0, 30).Format(time.RFC3339))
	// Predefined roles are deployment-wide ids, the same for the attack, the
	// random twin and the control (a random one would fail on validation).
	for _, name := range []string{"viewer", "analyst"} {
		if strings.Contains(body, "{{"+name+"}}") {
			body = strings.ReplaceAll(body, "{{"+name+"}}", h.predefinedRole(name))
		}
	}
	for k := range ids {
		if strings.Contains(body, "{{"+string(k)+"}}") {
			body = strings.ReplaceAll(body, "{{"+string(k)+"}}", val(k))
		}
	}
	path := strings.Join(segs, "/")
	if p.Query != "" {
		query := strings.ReplaceAll(p.Query, "{{run}}", h.run)
		for k := range ids {
			query = strings.ReplaceAll(query, "{{"+string(k)+"}}", val(k))
		}
		path += "?" + query
	}
	return path, body, supplied
}

// readPath is B's own read of the probe's target, from the last seeded kind.
func (h *harness) readPath(p Probe, ids map[Kind]string) string {
	for i := len(p.Kinds) - 1; i >= 0; i-- {
		k := p.Kinds[i]
		if IsLiteral(k) || k == KindUser {
			continue
		}
		rp := Kinds[k].ReadPath
		if rp == "" {
			return ""
		}
		for ik, id := range ids {
			rp = strings.ReplaceAll(rp, "{"+string(ik)+"}", id)
		}
		if strings.Contains(rp, "{") {
			return ""
		}
		return rp
	}
	return ""
}

// seedBoth gives both tenants one object of every kind that has no setup state
// before any probe runs, so an aggregate route (a dashboard, a count, a search)
// has real rows of both tenants to mix up. Without it A's dashboards would be
// empty and could not show a foreign row.
func (h *harness) seedBoth(t *testing.T) {
	t.Helper()
	for k := range seeders {
		switch k {
		case KindMitigationInReview, KindSubActionDone, KindIncidentPM:
			continue
		}
		for _, p := range []*principal{h.A, h.B} {
			if h.seedShared(p, k) == nil {
				t.Fatalf("cannot seed %s for tenant %s", k, p.name)
			}
		}
	}
}
