// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package domain

import (
	"testing"

	"github.com/google/uuid"
)

func edge(src, dst uuid.UUID, t DependencyType) AssetDependency {
	return AssetDependency{ID: uuid.New(), SourceAssetID: src, TargetAssetID: dst, Type: t}
}

// The four topology edge types the spec names must all exist, and the legacy
// vocabulary must fold onto them rather than disappearing.
func TestCanonicalTopologyType(t *testing.T) {
	cases := map[DependencyType]DependencyType{
		DepDependsOn:        DepDependsOn,
		DepConnectsTo:       DepConnectsTo,
		DepHostedOn:         DepHostedOn,
		DepProcessesDataOf:  DepProcessesDataOf,
		DepRunsOn:           DepHostedOn,        // legacy alias
		DepHostedBy:         DepHostedOn,        // legacy alias
		DepStoresDataIn:     DepProcessesDataOf, // legacy alias
		DepAuthenticatesVia: DepDependsOn,
		DepBacksUpTo:        DepDependsOn,
		DepManagedBy:        DepDependsOn,
	}
	for in, want := range cases {
		if got := in.CanonicalTopologyType(); got != want {
			t.Errorf("%s folded to %s, want %s", in, got, want)
		}
		if !in.IsValid() {
			t.Errorf("%s should be a valid dependency type", in)
		}
	}
	if len(TopologyEdgeTypes) != 4 {
		t.Errorf("expected 4 topology edge types, got %d", len(TopologyEdgeTypes))
	}
}

func TestZoneOf_FallbackChain(t *testing.T) {
	cases := []struct {
		name  string
		asset Asset
		want  string
	}{
		{"declared network zone wins", Asset{
			Category:   CategoryServer,
			Attributes: AssetAttributes{"network_zone": "dmz", "environment": "production"},
		}, "dmz"},
		{"cloud provider and region", Asset{
			Category:   CategoryCloud,
			Attributes: AssetAttributes{"provider": "aws", "region": "eu-west-3"},
		}, "aws/eu-west-3"},
		{"category and environment", Asset{
			Category:   CategoryApplication,
			Attributes: AssetAttributes{"environment": "production"},
		}, "application:production"},
		{"category alone", Asset{Category: CategoryDatabase}, "database"},
		{"free-text type", Asset{Type: "Laptop"}, "laptop"},
		{"nothing at all", Asset{}, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ZoneOf(&tc.asset); got != tc.want {
				t.Errorf("ZoneOf = %q, want %q", got, tc.want)
			}
		})
	}
}

// Exposure is expressed under different keys by different categories; all of
// them count, or a publicly-accessible bucket would render as internal.
func TestIsInternetExposed(t *testing.T) {
	if !IsInternetExposed(&Asset{Attributes: AssetAttributes{"internet_exposed": true}}) {
		t.Error("internet_exposed=true must count as exposed")
	}
	if !IsInternetExposed(&Asset{Attributes: AssetAttributes{"publicly_accessible": true}}) {
		t.Error("publicly_accessible=true must count as exposed")
	}
	if IsInternetExposed(&Asset{Attributes: AssetAttributes{"internet_exposed": false}}) {
		t.Error("false must not count as exposed")
	}
	if IsInternetExposed(&Asset{}) {
		t.Error("an asset that declared nothing is not exposed")
	}
}

func TestBuildTopology(t *testing.T) {
	a1, a2, a3 := uuid.New(), uuid.New(), uuid.New()
	assets := []Asset{
		{ID: a1, Name: "web-01", Category: CategoryServer, Criticality: CriticalityHigh,
			Attributes: AssetAttributes{"network_zone": "dmz", "internet_exposed": true}},
		{ID: a2, Name: "db-01", Category: CategoryDatabase, Criticality: CriticalityCritical,
			Attributes: AssetAttributes{"network_zone": "interne"}},
		{ID: a3, Name: "app-01", Category: CategoryApplication, Criticality: CriticalityMedium},
	}
	deps := []AssetDependency{
		edge(a3, a1, DepRunsOn),
		edge(a3, a2, DepStoresDataIn),
	}

	topo := BuildTopology(assets, deps, map[uuid.UUID]int{a1: 4}, 0)

	if len(topo.Nodes) != 3 || len(topo.Edges) != 2 {
		t.Fatalf("expected 3 nodes / 2 edges, got %d / %d", len(topo.Nodes), len(topo.Edges))
	}
	byID := map[uuid.UUID]TopologyNode{}
	for _, n := range topo.Nodes {
		byID[n.ID] = n
	}
	if byID[a1].Zone != "dmz" || !byID[a1].InternetExposed {
		t.Errorf("web-01 zone/exposure wrong: %+v", byID[a1])
	}
	if byID[a1].VulnCount != 4 {
		t.Errorf("vuln count not carried: %d", byID[a1].VulnCount)
	}
	if byID[a3].Degree != 2 {
		t.Errorf("app-01 should have degree 2, got %d", byID[a3].Degree)
	}
	// Legacy edge types fold, but keep their stored value for the detail panel.
	for _, e := range topo.Edges {
		if e.Target == a1 && (e.Type != DepHostedOn || e.RawType != DepRunsOn) {
			t.Errorf("runs_on should fold to hosted_on and keep raw: %+v", e)
		}
		if e.Target == a2 && e.Type != DepProcessesDataOf {
			t.Errorf("stores_data_in should fold to processes_data_of: %+v", e)
		}
	}
	if topo.Truncated {
		t.Error("an uncapped build must not report truncation")
	}
	if len(topo.Zones) != 3 {
		t.Errorf("expected 3 zones, got %d: %+v", len(topo.Zones), topo.Zones)
	}
}

// Capping must keep the assets that matter and SAY it capped — a topology
// quietly missing nodes is worse than one that renders slowly.
func TestBuildTopology_TruncatesLoudlyAndKeepsCriticalAssets(t *testing.T) {
	var assets []Asset
	var critical uuid.UUID
	for i := 0; i < 50; i++ {
		id := uuid.New()
		crit := CriticalityLow
		if i == 37 {
			crit = CriticalityCritical
			critical = id
		}
		assets = append(assets, Asset{ID: id, Name: "a", Criticality: crit})
	}
	topo := BuildTopology(assets, nil, nil, 10)

	if !topo.Truncated || topo.NodeLimit != 10 {
		t.Fatalf("truncation must be reported: truncated=%v limit=%d", topo.Truncated, topo.NodeLimit)
	}
	if len(topo.Nodes) != 10 {
		t.Fatalf("expected 10 nodes, got %d", len(topo.Nodes))
	}
	found := false
	for _, n := range topo.Nodes {
		if n.ID == critical {
			found = true
		}
	}
	if !found {
		t.Error("the critical asset must survive truncation")
	}
}

// An edge whose endpoint was capped out would draw a line into nothing.
func TestBuildTopology_DropsEdgesToCappedNodes(t *testing.T) {
	keep, drop := uuid.New(), uuid.New()
	assets := []Asset{
		{ID: keep, Criticality: CriticalityCritical},
		{ID: drop, Criticality: CriticalityLow},
	}
	topo := BuildTopology(assets, []AssetDependency{edge(keep, drop, DepDependsOn)}, nil, 1)
	if len(topo.Edges) != 0 {
		t.Errorf("expected the dangling edge to be dropped, got %+v", topo.Edges)
	}
}

// The direction is the whole point, and the easiest thing to get backwards.
func TestBuildCompromiseChain_Direction(t *testing.T) {
	server, app, browser := uuid.New(), uuid.New(), uuid.New()
	// browser --depends_on--> app --hosted_on--> server
	deps := []AssetDependency{
		edge(app, server, DepHostedOn),
		edge(browser, app, DepDependsOn),
	}

	// The server falls: everything standing on it is impacted, and nothing is
	// reachable from it (it depends on nothing).
	chain := BuildCompromiseChain(server, deps)
	if len(chain.Impacted) != 2 {
		t.Fatalf("expected app and browser impacted, got %+v", chain.Impacted)
	}
	depth := map[uuid.UUID]int{}
	for _, h := range chain.Impacted {
		depth[h.AssetID] = h.Depth
	}
	if depth[app] != 1 || depth[browser] != 2 {
		t.Errorf("depths wrong: app=%d browser=%d", depth[app], depth[browser])
	}
	if len(chain.Reachable) != 0 {
		t.Errorf("nothing is reachable from the server, got %+v", chain.Reachable)
	}

	// The browser falls: the attacker can move onto the app then the server.
	chain = BuildCompromiseChain(browser, deps)
	if len(chain.Reachable) != 2 {
		t.Fatalf("expected app and server reachable, got %+v", chain.Reachable)
	}
	if len(chain.Impacted) != 0 {
		t.Errorf("nothing depends on the browser, got %+v", chain.Impacted)
	}
	if len(chain.EdgeIDs) != 2 {
		t.Errorf("both edges are on the chain, got %d", len(chain.EdgeIDs))
	}
}

// Cycles are normal in a real estate; without a visited set this would hang.
func TestBuildCompromiseChain_TerminatesOnCycles(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	deps := []AssetDependency{
		edge(a, b, DepConnectsTo),
		edge(b, c, DepConnectsTo),
		edge(c, a, DepConnectsTo),
	}
	chain := BuildCompromiseChain(a, deps)
	if len(chain.Reachable) != 2 {
		t.Errorf("expected b and c reachable exactly once each, got %+v", chain.Reachable)
	}
	if len(chain.Impacted) != 2 {
		t.Errorf("expected b and c impacted exactly once each, got %+v", chain.Impacted)
	}
}

// An isolated asset has an empty chain, not a chain containing itself.
func TestBuildCompromiseChain_Isolated(t *testing.T) {
	lonely := uuid.New()
	chain := BuildCompromiseChain(lonely, []AssetDependency{edge(uuid.New(), uuid.New(), DepDependsOn)})
	if len(chain.Impacted) != 0 || len(chain.Reachable) != 0 || len(chain.EdgeIDs) != 0 {
		t.Errorf("expected an empty chain, got %+v", chain)
	}
	if chain.OriginID != lonely {
		t.Error("the origin must be reported back")
	}
}
