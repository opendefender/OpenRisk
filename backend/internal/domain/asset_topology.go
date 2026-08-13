// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"sort"
	"strings"

	"github.com/google/uuid"
)

// TopologyNode is one asset as the topology view needs it: enough to place it,
// colour it and label it, and nothing more. The full asset is fetched only when
// the user opens the detail panel — a 2 000-node graph must not ship 2 000 full
// asset records with their preloaded risks.
type TopologyNode struct {
	ID          uuid.UUID        `json:"id"`
	Name        string           `json:"name"`
	Type        string           `json:"type"`
	Category    AssetCategory    `json:"category,omitempty"`
	Criticality AssetCriticality `json:"criticality"`
	// Zone is the cluster this node belongs to (see ZoneOf).
	Zone string `json:"zone"`
	// InternetExposed drives the second colouring mode. Derived from the typed
	// attributes; false when the asset never declared one.
	InternetExposed bool `json:"internet_exposed"`
	// RiskCount / MaxRiskScore summarise what the register already knows.
	RiskCount    int     `json:"risk_count"`
	MaxRiskScore float64 `json:"max_risk_score"`
	// VulnCount is the number of open vulnerabilities attributed to this asset.
	VulnCount int `json:"vuln_count"`
	// Degree is how many edges touch this node — the layout uses it for sizing,
	// and it is what makes a hub legible without reading labels.
	Degree int `json:"degree"`
}

// TopologyEdge is one directed dependency, folded onto the four-type vocabulary.
type TopologyEdge struct {
	ID     uuid.UUID      `json:"id"`
	Source uuid.UUID      `json:"source"`
	Target uuid.UUID      `json:"target"`
	Type   DependencyType `json:"type"`
	// RawType is the stored type before folding, so the detail panel can say
	// "runs_on" where that is what the operator recorded.
	RawType     DependencyType `json:"raw_type"`
	Description string         `json:"description,omitempty"`
}

// TopologyZone is a cluster the layout groups nodes into.
type TopologyZone struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// AssetTopology is the whole graph for a tenant.
type AssetTopology struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
	Zones []TopologyZone `json:"zones"`
	// Truncated reports that the graph was capped. It is returned rather than
	// silently trimming, because a topology missing nodes without saying so is
	// worse than one that renders slowly.
	Truncated bool `json:"truncated"`
	NodeLimit int  `json:"node_limit,omitempty"`
}

// ZoneOf decides which cluster an asset belongs to.
//
// A declared network zone wins (that is what the operator said), then the cloud
// provider/region for cloud assets, then the typed category, then the free-text
// type. The fallback chain matters: an asset that has told us nothing still gets
// a stable, non-empty cluster instead of drifting through the layout alone.
func ZoneOf(a *Asset) string {
	if a == nil {
		return "unknown"
	}
	if z := attrString(a.Attributes, "network_zone"); z != "" {
		return z
	}
	if a.Category == CategoryCloud {
		if p := attrString(a.Attributes, "provider"); p != "" {
			if r := attrString(a.Attributes, "region"); r != "" {
				return p + "/" + r
			}
			return p
		}
	}
	if env := attrString(a.Attributes, "environment"); env != "" && a.Category != "" {
		return string(a.Category) + ":" + env
	}
	if a.Category != "" {
		return string(a.Category)
	}
	if a.Type != "" {
		return strings.ToLower(a.Type)
	}
	return "unknown"
}

// IsInternetExposed reads the exposure signal out of an asset's typed
// attributes. Several categories express it under different keys, so all of
// them are consulted — an application marked publicly accessible is exposed
// whether it said so as `internet_exposed` or `publicly_accessible`.
func IsInternetExposed(a *Asset) bool {
	if a == nil {
		return false
	}
	for _, key := range []string{"internet_exposed", "publicly_accessible"} {
		if v, ok := a.Attributes[key]; ok {
			if b, ok := v.(bool); ok && b {
				return true
			}
		}
	}
	return false
}

func attrString(attrs AssetAttributes, key string) string {
	if attrs == nil {
		return ""
	}
	if v, ok := attrs[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// BuildTopology assembles the graph from a tenant's assets and dependency edges.
//
// nodeLimit caps how many nodes are returned (0 = no cap). When the cap bites,
// the nodes kept are the ones the user most needs to see — highest criticality,
// then highest degree — and Truncated says so.
func BuildTopology(assets []Asset, deps []AssetDependency, vulnCounts map[uuid.UUID]int, nodeLimit int) AssetTopology {
	degree := make(map[uuid.UUID]int, len(assets))
	for _, d := range deps {
		degree[d.SourceAssetID]++
		degree[d.TargetAssetID]++
	}

	nodes := make([]TopologyNode, 0, len(assets))
	for i := range assets {
		a := &assets[i]
		n := TopologyNode{
			ID:              a.ID,
			Name:            a.Name,
			Type:            a.Type,
			Category:        a.Category,
			Criticality:     a.Criticality,
			Zone:            ZoneOf(a),
			InternetExposed: IsInternetExposed(a),
			RiskCount:       len(a.Risks),
			VulnCount:       vulnCounts[a.ID],
			Degree:          degree[a.ID],
		}
		for _, r := range a.Risks {
			if r != nil && r.Score > n.MaxRiskScore {
				n.MaxRiskScore = r.Score
			}
		}
		nodes = append(nodes, n)
	}

	truncated := false
	if nodeLimit > 0 && len(nodes) > nodeLimit {
		sort.SliceStable(nodes, func(i, j int) bool {
			ci, cj := nodes[i].Criticality.ScoreFactor(), nodes[j].Criticality.ScoreFactor()
			if ci != cj {
				return ci > cj
			}
			return nodes[i].Degree > nodes[j].Degree
		})
		nodes = nodes[:nodeLimit]
		truncated = true
	}

	kept := make(map[uuid.UUID]bool, len(nodes))
	for _, n := range nodes {
		kept[n.ID] = true
	}

	edges := make([]TopologyEdge, 0, len(deps))
	for _, d := range deps {
		// An edge to a node that was capped out would render as a line into
		// nothing. Drop it with the node.
		if !kept[d.SourceAssetID] || !kept[d.TargetAssetID] {
			continue
		}
		edges = append(edges, TopologyEdge{
			ID:          d.ID,
			Source:      d.SourceAssetID,
			Target:      d.TargetAssetID,
			Type:        d.Type.CanonicalTopologyType(),
			RawType:     d.Type,
			Description: d.Description,
		})
	}

	zoneCounts := map[string]int{}
	for _, n := range nodes {
		zoneCounts[n.Zone]++
	}
	zones := make([]TopologyZone, 0, len(zoneCounts))
	for k, c := range zoneCounts {
		zones = append(zones, TopologyZone{Key: k, Label: zoneLabel(k), Count: c})
	}
	sort.Slice(zones, func(i, j int) bool {
		if zones[i].Count != zones[j].Count {
			return zones[i].Count > zones[j].Count
		}
		return zones[i].Key < zones[j].Key
	})

	// Stable node order regardless of the truncation sort, so two renders of the
	// same graph do not reshuffle.
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID.String() < nodes[j].ID.String() })

	return AssetTopology{
		Nodes: nodes, Edges: edges, Zones: zones,
		Truncated: truncated,
		NodeLimit: nodeLimit,
	}
}

func zoneLabel(key string) string {
	if key == "unknown" {
		return "Non classé"
	}
	if i := strings.Index(key, ":"); i > 0 {
		cat := AssetCategory(key[:i])
		if label := DefaultCategoryLabel(cat); label != string(cat) {
			return label + " · " + key[i+1:]
		}
	}
	if label := DefaultCategoryLabel(AssetCategory(key)); label != key {
		return label
	}
	return key
}

// CompromiseChain is the answer to "this asset is compromised — what else is
// reachable from it?".
type CompromiseChain struct {
	OriginID uuid.UUID `json:"origin_id"`
	// Impacted are the assets that depend, directly or transitively, on the
	// origin: what breaks if the origin falls.
	Impacted []ChainHop `json:"impacted"`
	// Reachable are the assets the origin itself depends on or connects to:
	// where an attacker standing on the origin could move next.
	Reachable []ChainHop `json:"reachable"`
	// EdgeIDs is every edge on either path, so the view can highlight the chain
	// without recomputing it.
	EdgeIDs []uuid.UUID `json:"edge_ids"`
}

// ChainHop is one asset on a compromise chain, with how far it sits from the
// origin.
type ChainHop struct {
	AssetID uuid.UUID `json:"asset_id"`
	Depth   int       `json:"depth"`
}

// BuildCompromiseChain walks the dependency graph from an origin asset in both
// directions and returns the full chain.
//
// Direction is the whole point and is easy to get backwards. An edge
// "app --hosted_on--> server" means the APP needs the SERVER. So:
//
//   - if the SERVER is compromised, the APP is impacted → follow edges
//     BACKWARDS (target → source) to find what breaks;
//   - if the APP is compromised, the SERVER is where an attacker moves next →
//     follow edges FORWARDS (source → target) to find what is reachable.
//
// Cycles are normal in a real estate (A connects_to B connects_to A) and are
// handled by the visited set; without it this would not terminate.
func BuildCompromiseChain(origin uuid.UUID, deps []AssetDependency) CompromiseChain {
	type adjEntry struct {
		to     uuid.UUID
		edgeID uuid.UUID
	}
	forward := map[uuid.UUID][]adjEntry{}  // source → targets (what origin depends on)
	backward := map[uuid.UUID][]adjEntry{} // target → sources (what depends on origin)
	for _, d := range deps {
		forward[d.SourceAssetID] = append(forward[d.SourceAssetID], adjEntry{d.TargetAssetID, d.ID})
		backward[d.TargetAssetID] = append(backward[d.TargetAssetID], adjEntry{d.SourceAssetID, d.ID})
	}

	edgeSet := map[uuid.UUID]bool{}
	walk := func(adj map[uuid.UUID][]adjEntry) []ChainHop {
		visited := map[uuid.UUID]bool{origin: true}
		var out []ChainHop
		frontier := []uuid.UUID{origin}
		for depth := 1; len(frontier) > 0; depth++ {
			var next []uuid.UUID
			for _, cur := range frontier {
				for _, e := range adj[cur] {
					edgeSet[e.edgeID] = true
					if visited[e.to] {
						continue
					}
					visited[e.to] = true
					out = append(out, ChainHop{AssetID: e.to, Depth: depth})
					next = append(next, e.to)
				}
			}
			frontier = next
		}
		return out
	}

	// Initialised to empty slices, not left nil: a nil slice marshals to JSON
	// `null`, and the API contract declares these as arrays. An isolated asset
	// would otherwise hand every client a null to guard against.
	chain := CompromiseChain{
		OriginID:  origin,
		Impacted:  []ChainHop{},
		Reachable: []ChainHop{},
	}
	if hops := walk(backward); hops != nil {
		chain.Impacted = hops
	}
	if hops := walk(forward); hops != nil {
		chain.Reachable = hops
	}

	chain.EdgeIDs = make([]uuid.UUID, 0, len(edgeSet))
	for id := range edgeSet {
		chain.EdgeIDs = append(chain.EdgeIDs, id)
	}
	sort.Slice(chain.EdgeIDs, func(i, j int) bool {
		return chain.EdgeIDs[i].String() < chain.EdgeIDs[j].String()
	})
	return chain
}
