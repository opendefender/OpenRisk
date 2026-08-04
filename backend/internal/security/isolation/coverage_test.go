// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package isolation

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/opendefender/openrisk/internal/security/routes"
)

func routerPath(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "cmd", "server", "main.go")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("module root not found")
		}
		dir = parent
	}
}

func parameterisedRoutes(t *testing.T) []routes.Route {
	t.Helper()

	all, err := routes.Extract(routerPath(t))
	if err != nil {
		t.Fatalf("extract routes: %v", err)
	}
	var out []routes.Route
	for _, r := range all {
		if r.IsParameterised() {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		t.Fatal("no parameterised routes extracted — the gate would pass vacuously")
	}
	return out
}

// TestEveryParameterisedRouteHasAnIsolationDecision is the gate.
//
// Every route that takes an ID lets a caller name a resource they may not own.
// This fails the build when such a route has no recorded decision, which is what
// stops isolation coverage from drifting behind the router — the failure mode
// that let six real cross-tenant leaks ship before the July sweep.
//
// It asserts that a decision was MADE, not that isolation works. Proving the
// latter needs the live two-tenant probe (see TestCrossTenantProbe), which
// requires a database.
func TestEveryParameterisedRouteHasAnIsolationDecision(t *testing.T) {
	var undecided []routes.Route
	for _, r := range parameterisedRoutes(t) {
		if _, ok := Lookup(r.Path); !ok {
			undecided = append(undecided, r)
		}
	}

	if len(undecided) > 0 {
		var b strings.Builder
		b.WriteString("routes with no tenant-isolation decision:\n\n")
		for _, r := range undecided {
			b.WriteString("  " + r.String() + "\n")
			b.WriteString("      registered at cmd/server/main.go:" + itoa(r.Line) + "\n")
			b.WriteString("      pattern to declare: " + Normalise(r.Path) + "\n")
		}
		b.WriteString("\nAdd an entry to decisions in internal/security/isolation/registry.go.\n")
		b.WriteString("If the route reads or writes tenant data, it needs a cross-tenant test\n")
		b.WriteString("(Covered). If it genuinely cannot leak, record why.\n")
		t.Fatal(b.String())
	}
}

// TestEveryDecisionCarriesEvidence stops the registry degrading into a list of
// bare assertions. A decision without a reason cannot be reviewed.
func TestEveryDecisionCarriesEvidence(t *testing.T) {
	for _, d := range Decisions() {
		if strings.TrimSpace(d.Evidence) == "" {
			t.Errorf("%s (%s) has no evidence", d.Pattern, d.Status)
		}
		if len(d.Evidence) < 20 {
			t.Errorf("%s: evidence %q is too terse to review", d.Pattern, d.Evidence)
		}
	}
}

// TestNoStaleDecisions catches the opposite drift: an entry for a route that no
// longer exists is misleading, and worse, it silently keeps matching a prefix.
func TestNoStaleDecisions(t *testing.T) {
	live := parameterisedRoutes(t)

	for _, d := range Decisions() {
		var used bool
		for _, r := range live {
			if matches(d.Pattern, Normalise(r.Path)) {
				used = true
				break
			}
		}
		if !used {
			t.Errorf("decision %q matches no live route — remove it or fix the pattern", d.Pattern)
		}
	}
}

// TestPendingGapsAreVisible does not fail on pending entries; it reports them.
// Pending is deliberate, acknowledged debt, and the report is what keeps it from
// being forgotten. Turning it into a failure now would only invite someone to
// relabel gaps as covered to get a green build.
func TestPendingGapsAreVisible(t *testing.T) {
	counts := map[Status]int{}
	var pending []string

	for _, r := range parameterisedRoutes(t) {
		d, ok := Lookup(r.Path)
		if !ok {
			continue // already reported by the gate above
		}
		counts[d.Status]++
		if d.Status == Pending {
			pending = append(pending, r.String())
		}
	}

	sort.Strings(pending)
	t.Logf("isolation decisions across %d parameterised routes:", len(parameterisedRoutes(t)))
	for _, s := range []Status{Covered, Pending, PublicByDesign, MachineAuthenticated, SelfScoped, Unreachable} {
		t.Logf("  %-22s %d", s, counts[s])
	}
	if len(pending) > 0 {
		t.Logf("\n%d routes still need a cross-tenant test:", len(pending))
		for _, p := range pending {
			t.Logf("  %s", p)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
