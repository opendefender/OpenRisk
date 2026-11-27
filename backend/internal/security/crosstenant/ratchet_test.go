// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package crosstenant

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/opendefender/openrisk/internal/security/isolation"
	"github.com/opendefender/openrisk/internal/security/routes"
)

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("module root not found")
		}
		dir = parent
	}
}

func allRoutes(t *testing.T) []routes.Route {
	t.Helper()
	all, err := routes.Extract(filepath.Join(moduleRoot(t), "cmd", "server", "main.go"))
	if err != nil {
		t.Fatalf("extract routes: %v", err)
	}
	if len(all) < 100 {
		t.Fatalf("only %d routes extracted: the ratchet would pass vacuously", len(all))
	}
	return all
}

// autoExempt: the isolation registry already says tenant isolation does not
// apply to this route in the way a cross-tenant probe tests it.
func autoExempt(r routes.Route) bool {
	d, ok := isolation.Lookup(r.Path)
	if !ok {
		return false
	}
	switch d.Status {
	case isolation.PublicByDesign, isolation.MachineAuthenticated, isolation.SelfScoped:
		return true
	}
	return false
}

func routeKey(method, path string) string { return method + " " + path }

// needsCoverage is the ratchet's unit: every parameterised route, every
// collection GET, and every unparameterised write.
func needsCoverage(r routes.Route) bool { return !autoExempt(r) }

func probeIndex() map[string]Probe {
	idx := map[string]Probe{}
	for _, p := range allProbes() {
		idx[routeKey(p.Method, p.Path)] = p
	}
	return idx
}

func exemptionIndex() map[string]Exemption {
	idx := map[string]Exemption{}
	for _, e := range Exemptions {
		idx[routeKey(e.Method, e.Path)] = e
	}
	return idx
}

// TestRatchet_EveryRouteHasAProbeOrAnExemption: a route added tomorrow fails
// here until somebody probes it or writes down why not.
func TestRatchet_EveryRouteHasAProbeOrAnExemption(t *testing.T) {
	probes, exempt := probeIndex(), exemptionIndex()
	var missing []string
	for _, r := range allRoutes(t) {
		if !needsCoverage(r) {
			continue
		}
		k := routeKey(r.Method, r.Path)
		if _, ok := probes[k]; ok {
			continue
		}
		if _, ok := exempt[k]; ok {
			continue
		}
		missing = append(missing, k)
	}
	if len(missing) > 0 {
		t.Fatalf("%d routes have neither a probe nor an exemption (add a Probe in probe_*.go, or an Exemption with a reason):\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// TestRatchet_NoStaleEntries: every probe and exemption names a route that is
// still registered, once.
func TestRatchet_NoStaleEntries(t *testing.T) {
	registered := map[string]routes.Route{}
	for _, r := range allRoutes(t) {
		registered[routeKey(r.Method, r.Path)] = r
	}
	seen := map[string]string{}
	for _, p := range allProbes() {
		k := routeKey(p.Method, p.Path)
		if _, ok := registered[k]; !ok {
			t.Errorf("probe %s names a route that is not registered", k)
		}
		if prev, dup := seen[k]; dup {
			t.Errorf("%s is listed twice (%s)", k, prev)
		}
		seen[k] = "probe"
	}
	for _, e := range Exemptions {
		k := e.ID()
		r, ok := registered[k]
		if !ok {
			t.Errorf("exemption %s names a route that is not registered", k)
			continue
		}
		if autoExempt(r) {
			t.Errorf("exemption %s is redundant: the isolation registry already exempts it", k)
		}
		if prev, dup := seen[k]; dup {
			t.Errorf("%s is both a probe/exemption (%s) and an exemption", k, prev)
		}
		seen[k] = "exemption"
	}
}

// TestRatchet_EveryGroupIsCovered: each route group has at least one probe or a
// group-level exemption. Nothing passes without one or the other.
func TestRatchet_EveryGroupIsCovered(t *testing.T) {
	groupProbed := map[string]bool{}
	for _, p := range allProbes() {
		groupProbed[Group(p.Path)] = true
	}
	groupExempt := map[string]bool{}
	for _, g := range GroupExemptions {
		groupExempt[g.Group] = true
		if groupProbed[g.Group] {
			t.Errorf("group %q has probes and a group exemption: drop the exemption", g.Group)
		}
	}
	// A group whose every route is NotApplicable has nothing to probe and needs
	// no group exemption: each route already carries its written reason.
	exempt := exemptionIndex()
	groupHasWork := map[string]bool{}
	for _, r := range allRoutes(t) {
		if !needsCoverage(r) {
			continue
		}
		if e, ok := exempt[routeKey(r.Method, r.Path)]; ok && e.NotApplicable {
			continue
		}
		groupHasWork[Group(r.Path)] = true
	}
	var missing []string
	for g := range groupHasWork {
		if !groupProbed[g] && !groupExempt[g] {
			missing = append(missing, g)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("route groups with no probe and no group exemption: %s", strings.Join(missing, ", "))
	}
}

// TestRatchet_UnparameterisedWritesAreAccountedFor closes the blind spot the
// isolation package names: a write with no id in its path takes its target in the
// body. It needs a body-addressed probe, or a written reason that no id is in
// the body.
func TestRatchet_UnparameterisedWritesAreAccountedFor(t *testing.T) {
	probes, exempt := probeIndex(), exemptionIndex()
	for _, r := range allRoutes(t) {
		if r.Method == "GET" || r.IsParameterised() || !needsCoverage(r) {
			continue
		}
		k := routeKey(r.Method, r.Path)
		if p, ok := probes[k]; ok {
			if len(p.Kinds) == 0 || !strings.Contains(p.Body+p.Query, "{{") {
				t.Errorf("%s: a write probe with no {{kind}} in its body proves nothing about body- or query-addressed ids", k)
			}
			continue
		}
		e, ok := exempt[k]
		if !ok {
			t.Errorf("%s: unparameterised write with no body-addressed probe and no exemption", k)
			continue
		}
		if strings.TrimSpace(e.Reason) == "" {
			t.Errorf("%s: exemption without a written reason", k)
		}
	}
}

// TestRatchet_ProbesAreWellFormed checks the shape rules the live harness
// relies on, so a typo fails here and not as a confusing live failure.
func TestRatchet_ProbesAreWellFormed(t *testing.T) {
	for _, p := range allProbes() {
		id := p.ID()
		n := ParamCount(p.Path)
		if len(p.Kinds) < n {
			t.Errorf("%s: %d path params but %d kinds", id, n, len(p.Kinds))
			continue
		}
		for _, k := range p.Kinds {
			if IsLiteral(k) {
				continue
			}
			if _, ok := Kinds[k]; !ok {
				t.Errorf("%s: kind %q is not in Kinds", id, k)
			}
		}
		for i := 0; i < n; i++ {
			if p.Kinds[i] == "" {
				t.Errorf("%s: empty kind for param %d", id, i)
			}
		}
		if p.Collection {
			if p.Method != "GET" {
				t.Errorf("%s: only a GET can be a collection probe", id)
			}
			if !p.Aggregate && len(p.Kinds) <= n {
				t.Errorf("%s: a list probe needs a kind its control must show; mark it Aggregate if it holds none", id)
			}
		}
		if len(p.Allow) > 0 && (strings.TrimSpace(p.Reason) == "" || p.RegisterID == "") {
			t.Errorf("%s: Allow needs a Reason and a RegisterID", id)
		}
		for _, s := range p.Allow {
			if s == 401 || s == 429 || s >= 500 {
				t.Errorf("%s: %d can never be allowed", id, s)
			}
		}
	}
}

var rrFormat = regexp.MustCompile(`^RR-\d{2}$`)

// TestRatchet_RegisterIDsExist ties the exemptions to the register: every id
// used is defined, defined ids are used, and once the report exists every id is
// in it. The report is written separately (AC2-AC4), so its absence is logged,
// not failed: the check starts to bite the day the file appears.
func TestRatchet_RegisterIDsExist(t *testing.T) {
	used := map[string]string{}
	for _, p := range allProbes() {
		if p.RegisterID != "" {
			used[p.RegisterID] = p.ID()
		}
	}
	for _, e := range Exemptions {
		if strings.TrimSpace(e.Reason) == "" {
			t.Errorf("exemption %s has no reason", e.ID())
		}
		if e.NotApplicable {
			if e.RegisterID != "" {
				t.Errorf("exemption %s is NotApplicable and must not cite a register row", e.ID())
			}
			continue
		}
		if e.RegisterID == "" {
			t.Errorf("exemption %s has no RegisterID", e.ID())
			continue
		}
		used[e.RegisterID] = e.ID()
	}
	for _, g := range GroupExemptions {
		if strings.TrimSpace(g.Reason) == "" || g.RegisterID == "" {
			t.Errorf("group exemption %q needs a reason and a RegisterID", g.Group)
			continue
		}
		used[g.RegisterID] = "group " + g.Group
	}
	for id, where := range used {
		if !rrFormat.MatchString(id) {
			t.Errorf("%s: register id %q is not of the form RR-NN", where, id)
		}
		if _, ok := Register[id]; !ok {
			t.Errorf("%s: register id %s is not defined in Register", where, id)
		}
	}
	for id := range Register {
		if _, ok := used[id]; !ok {
			t.Errorf("register row %s is defined but no probe or exemption uses it", id)
		}
	}

	report := filepath.Join(moduleRoot(t), "..", "docs", "security", "SECURITY_TEST_REPORT.md")
	raw, err := os.ReadFile(report)
	if err != nil {
		t.Logf("%s does not exist yet: register rows are not cross-checked against it", report)
		return
	}
	text := string(raw)
	for id := range used {
		if !strings.Contains(text, id) {
			t.Errorf("%s is used by the suite but absent from SECURITY_TEST_REPORT.md", id)
		}
	}
}

// TestRatchet_IsolationEvidenceNamesARealProbe: a Covered decision that cites
// the live suite must cite a probe that exists.
func TestRatchet_IsolationEvidenceNamesARealProbe(t *testing.T) {
	probes := probeIndex()
	for _, d := range isolation.Decisions() {
		for _, ref := range strings.Fields(strings.ReplaceAll(d.Evidence, "crosstenant:", "\n crosstenant:")) {
			_ = ref
		}
		idx := strings.Index(d.Evidence, "crosstenant:")
		if idx < 0 {
			continue
		}
		name := strings.TrimSpace(d.Evidence[idx+len("crosstenant:"):])
		if i := strings.IndexAny(name, ";\n"); i >= 0 {
			name = name[:i]
		}
		if _, ok := probes[strings.TrimSpace(name)]; !ok {
			t.Errorf("isolation decision %s cites crosstenant probe %q, which does not exist", d.Pattern, name)
		}
	}
}

// TestRatchet_OwnBodyProbesAreWellFormed: an own-object probe names a registered
// route, takes one of A's own objects per path param, and names at least one B
// id in its body (otherwise it would test nothing).
func TestRatchet_OwnBodyProbesAreWellFormed(t *testing.T) {
	registered := map[string]bool{}
	for _, r := range allRoutes(t) {
		registered[routeKey(r.Method, r.Path)] = true
	}
	if len(ownBodyProbes) < 2 {
		t.Fatalf("the own-object shape needs the risk category route and at least one more, have %d", len(ownBodyProbes))
	}
	for _, p := range ownBodyProbes {
		if !registered[routeKey(p.Method, p.Path)] {
			t.Errorf("own-object probe %s names a route that is not registered", p.ID())
		}
		if len(p.Own) != ParamCount(p.Path) {
			t.Errorf("own-object probe %s has %d own kinds for %d path params", p.ID(), len(p.Own), ParamCount(p.Path))
		}
		if len(p.Kinds) == 0 || !strings.Contains(p.Body, "{{") {
			t.Errorf("own-object probe %s names no foreign id in its body", p.ID())
		}
	}
}
