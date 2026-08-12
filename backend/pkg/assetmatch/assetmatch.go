// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package assetmatch correlates an external finding with an asset in the
// inventory, and says how sure it is.
//
// This is the join every vulnerability scanner leaves to its customer. Nessus
// reports "WEB-01.corp.local", AWS Inspector reports an ARN, CrowdStrike reports
// an agent id, and the inventory calls the same machine "web-01". Getting that
// join wrong is not a cosmetic problem: a vulnerability attributed to the wrong
// asset inherits the wrong business criticality, and therefore the wrong
// priority — it is a scoring error dressed as a data error.
//
// So this package does three things the naive `strings.ToLower(name) == name`
// version does not:
//
//   - it matches on FOUR signals (cloud resource id, IP, hostname, CPE) with
//     different confidence, because they are not equally trustworthy;
//   - it returns a CONFIDENCE and the reason, so the UI can show why;
//   - it reports AMBIGUITY rather than silently picking the first hit. Two
//     assets answering to the same hostname is a real situation (a rebuild, a
//     stale record), and picking one at random hides exactly the inventory
//     problem the user needs to fix.
//
// Pure: no I/O, no database, no domain imports. Deterministic — the same inputs
// always produce the same ranking, including the tie-break.
package assetmatch

import (
	"net"
	"sort"
	"strings"
)

// Method names how a candidate was matched. Ordered from most to least
// trustworthy.
type Method string

const (
	// MethodCloudID — the finding carried a cloud resource id (ARN, Azure
	// resource id, GCP self-link) that the asset declares. Provider-issued and
	// globally unique: as close to certain as this gets.
	MethodCloudID Method = "cloud_id"
	// MethodExternalID — the finding's asset id equals the asset's external id
	// from the same tool. Authoritative within that tool.
	MethodExternalID Method = "external_id"
	// MethodIP — an exact IP match. Strong, but IPs are reassigned and a NAT
	// address can be shared, so it is not certainty.
	MethodIP Method = "ip"
	// MethodHostnameExact — the full hostname matched, case-insensitively.
	MethodHostnameExact Method = "hostname_exact"
	// MethodHostnameShort — the short name matched after stripping the domain
	// ("web-01.corp.local" vs "web-01"). Common and usually right, but short
	// names are only unique within a domain.
	MethodHostnameShort Method = "hostname_short"
	// MethodName — the finding's asset label matched the asset's display name.
	// Weakest: display names are free text.
	MethodName Method = "name"
	// MethodCPE — the finding's CPE matched one the asset declares. This says
	// "this software runs here", not "this is that machine", so on its own it is
	// a weak signal — but it CORROBORATES another one strongly.
	MethodCPE Method = "cpe"
)

// baseConfidence is the score a single signal contributes.
var baseConfidence = map[Method]float64{
	MethodCloudID:       0.99,
	MethodExternalID:    0.95,
	MethodIP:            0.85,
	MethodHostnameExact: 0.80,
	MethodHostnameShort: 0.65,
	MethodCPE:           0.35,
	MethodName:          0.45,
}

// Finding is the identity an external tool gave a finding.
type Finding struct {
	// AssetExternalID is the tool's own id for the affected asset.
	AssetExternalID string
	// AssetName is whatever label the tool reported — often a hostname, often
	// an FQDN, sometimes a display name, sometimes a resource id.
	AssetName string
	Hostname  string
	IPs       []string
	CloudID   string
	CPEs      []string
}

// Candidate is one asset the correlator may attribute a finding to.
type Candidate struct {
	// ID is opaque here (the caller's uuid, stringified) — this package does not
	// import the domain.
	ID              string
	Name            string
	ExternalID      string
	Hostnames       []string
	IPs             []string
	CloudResourceID string
	CPEs            []string
}

// Match is one scored candidate.
type Match struct {
	AssetID string `json:"asset_id"`
	// Confidence in [0,1].
	Confidence float64 `json:"confidence"`
	// Methods are every signal that agreed, strongest first.
	Methods []Method `json:"methods"`
	// Reason is a one-line, human-readable explanation.
	Reason string `json:"reason"`
}

// Result is the outcome of correlating one finding.
type Result struct {
	// Best is the winning candidate, or nil when nothing matched.
	Best *Match `json:"best,omitempty"`
	// Candidates are all scored matches, best first (Best included).
	Candidates []Match `json:"candidates,omitempty"`
	// Ambiguous is true when more than one candidate matched at a confidence
	// close enough to the winner that the choice is not safe to make
	// automatically. The caller should attribute nothing and ask a human.
	Ambiguous bool `json:"ambiguous"`
}

// AmbiguityMargin is how close the runner-up must be to the winner for the
// result to count as ambiguous.
//
// It is deliberately generous. The cost of a wrong automatic attribution (a
// vulnerability silently scored against another machine's criticality) is much
// higher than the cost of asking someone to resolve it on the unassigned screen.
const AmbiguityMargin = 0.10

// AutoAssignThreshold is the confidence at or above which a caller may attribute
// a finding without human review. Below it, the finding belongs on the
// "Unassigned vulnerabilities" screen with its candidates attached.
const AutoAssignThreshold = 0.80

// Correlate scores a finding against every candidate and returns the ranking.
func Correlate(f Finding, candidates []Candidate) Result {
	fp := normaliseFinding(f)

	matches := make([]Match, 0, 4)
	for _, c := range candidates {
		if m, ok := score(fp, c); ok {
			matches = append(matches, m)
		}
	}
	if len(matches) == 0 {
		return Result{}
	}

	// Deterministic ordering: confidence desc, then asset id — without the
	// tie-break, two equally-scored candidates would swap between identical runs
	// and the "best" match would depend on map iteration order.
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Confidence != matches[j].Confidence {
			return matches[i].Confidence > matches[j].Confidence
		}
		return matches[i].AssetID < matches[j].AssetID
	})

	res := Result{Candidates: matches}
	best := matches[0]
	res.Best = &best

	if len(matches) > 1 && best.Confidence-matches[1].Confidence < AmbiguityMargin {
		res.Ambiguous = true
	}
	return res
}

// normalisedFinding is the finding with every signal lower-cased and de-duped.
type normalisedFinding struct {
	externalID string
	name       string
	hostnames  map[string]bool // full forms
	shortNames map[string]bool // domain stripped
	ips        map[string]bool
	cloudID    string
	cpes       map[string]bool
}

func normaliseFinding(f Finding) normalisedFinding {
	out := normalisedFinding{
		externalID: strings.ToLower(strings.TrimSpace(f.AssetExternalID)),
		name:       strings.ToLower(strings.TrimSpace(f.AssetName)),
		cloudID:    strings.ToLower(strings.TrimSpace(f.CloudID)),
		hostnames:  map[string]bool{},
		shortNames: map[string]bool{},
		ips:        map[string]bool{},
		cpes:       map[string]bool{},
	}

	// The tool's asset label is treated as a hostname candidate as well as a
	// name: most scanners put an FQDN there, and refusing to try it would throw
	// away the single most common way this join actually succeeds.
	for _, h := range append([]string{f.Hostname, f.AssetName}, nil...) {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" {
			continue
		}
		out.hostnames[h] = true
		out.shortNames[shortHost(h)] = true
	}
	for _, ip := range f.IPs {
		if ip = strings.TrimSpace(ip); ip != "" {
			out.ips[ip] = true
		}
	}
	// An asset label that parses as an IP is one — several tools report the
	// address when they never resolved a name.
	if looksLikeIP(out.name) {
		out.ips[out.name] = true
	}
	for _, cpe := range f.CPEs {
		if cpe = strings.ToLower(strings.TrimSpace(cpe)); cpe != "" {
			out.cpes[cpeProduct(cpe)] = true
		}
	}
	return out
}

// score evaluates one candidate. Returns ok=false when no signal agreed.
func score(f normalisedFinding, c Candidate) (Match, bool) {
	var methods []Method

	if f.cloudID != "" && strings.EqualFold(f.cloudID, c.CloudResourceID) {
		methods = append(methods, MethodCloudID)
	}
	if f.externalID != "" && c.ExternalID != "" && strings.EqualFold(f.externalID, c.ExternalID) {
		methods = append(methods, MethodExternalID)
	}
	for _, ip := range c.IPs {
		if f.ips[strings.TrimSpace(ip)] {
			methods = append(methods, MethodIP)
			break
		}
	}

	exact, short := false, false
	for _, h := range c.Hostnames {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" {
			continue
		}
		if f.hostnames[h] {
			exact = true
			break
		}
		if f.shortNames[shortHost(h)] {
			short = true
		}
	}
	// The asset's display name is also worth trying as a hostname: inventories
	// routinely name a server after its hostname without filling the field.
	if !exact && c.Name != "" {
		n := strings.ToLower(strings.TrimSpace(c.Name))
		if f.hostnames[n] {
			exact = true
		} else if f.shortNames[shortHost(n)] {
			short = true
		}
	}
	switch {
	case exact:
		methods = append(methods, MethodHostnameExact)
	case short:
		methods = append(methods, MethodHostnameShort)
	}

	if !exact && !short && f.name != "" && strings.EqualFold(f.name, c.Name) {
		methods = append(methods, MethodName)
	}

	if len(f.cpes) > 0 {
		for _, cpe := range c.CPEs {
			if f.cpes[cpeProduct(strings.ToLower(strings.TrimSpace(cpe)))] {
				methods = append(methods, MethodCPE)
				break
			}
		}
	}

	if len(methods) == 0 {
		return Match{}, false
	}

	sort.Slice(methods, func(i, j int) bool {
		return baseConfidence[methods[i]] > baseConfidence[methods[j]]
	})

	// Confidence combination: the strongest signal sets the floor, and each
	// additional agreeing signal closes part of the remaining gap to certainty.
	// Independent signals agreeing IS the evidence — a hostname match alone can
	// be a stale record, the same hostname plus the same IP plus the same CPE
	// is the machine. Summing instead would let three weak signals outrank a
	// cloud resource id, which is never right.
	conf := baseConfidence[methods[0]]
	for _, m := range methods[1:] {
		conf += (1 - conf) * baseConfidence[m] * 0.6
	}
	if conf > 0.99 {
		conf = 0.99 // never claim certainty; a human can always be right instead
	}

	return Match{
		AssetID:    c.ID,
		Confidence: round2(conf),
		Methods:    methods,
		Reason:     reasonFor(methods, c),
	}, true
}

func reasonFor(methods []Method, c Candidate) string {
	label := c.Name
	if label == "" {
		label = c.ID
	}
	parts := make([]string, 0, len(methods))
	for _, m := range methods {
		switch m {
		case MethodCloudID:
			parts = append(parts, "identifiant de ressource cloud identique")
		case MethodExternalID:
			parts = append(parts, "identifiant externe identique")
		case MethodIP:
			parts = append(parts, "adresse IP identique")
		case MethodHostnameExact:
			parts = append(parts, "nom d'hôte identique")
		case MethodHostnameShort:
			parts = append(parts, "nom d'hôte court identique")
		case MethodName:
			parts = append(parts, "nom d'actif identique")
		case MethodCPE:
			parts = append(parts, "CPE en commun")
		}
	}
	return label + " — " + strings.Join(parts, ", ")
}

// shortHost strips the DNS domain: "web-01.corp.local" → "web-01".
func shortHost(h string) string {
	if i := strings.Index(h, "."); i > 0 {
		// Do not strip anything from something that is actually an IP.
		if !looksLikeIP(h) {
			return h[:i]
		}
	}
	return h
}

// looksLikeIP defers to net.ParseIP rather than counting dots and colons.
// Hand-rolling it gets IPv6 wrong: "fe80::1" leads with hex letters, so any
// character-by-character check that has not already seen a colon rejects a
// perfectly valid address.
func looksLikeIP(s string) bool {
	return net.ParseIP(strings.TrimSpace(s)) != nil
}

// cpeProduct reduces a CPE to vendor:product, dropping the version.
//
// Version is intentionally discarded: a finding says "log4j 2.14.1 is
// vulnerable" while the inventory may record 2.14.0 or no version at all.
// Matching on the exact version would fail precisely when the asset is the one
// at risk, which is backwards.
func cpeProduct(cpe string) string {
	if cpe == "" {
		return ""
	}
	parts := strings.Split(cpe, ":")
	// cpe:2.3:a:vendor:product:version:...
	if len(parts) >= 5 && parts[0] == "cpe" {
		return parts[2] + ":" + parts[3] + ":" + parts[4]
	}
	return cpe
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
