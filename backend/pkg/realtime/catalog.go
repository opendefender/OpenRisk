// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package realtime

import "sort"

// EventType is a canonical domain event name, always `<aggregate>.<action>`.
//
// The name describes a change to the BUSINESS, never a change to a screen.
// `risk.status_changed` is an event; `risk-card-refresh` is a rendering concern
// that happens to be triggered by one. The distinction matters because a second
// consumer — automation, analytics, an export — subscribes to the first and
// could never subscribe to the second.
type EventType string

// Risk events.
const (
	RiskCreated       EventType = "risk.created"
	RiskUpdated       EventType = "risk.updated"
	RiskDeleted       EventType = "risk.deleted"
	RiskStatusChanged EventType = "risk.status_changed"
	RiskScoreChanged  EventType = "risk.score_changed"
)

// Asset events.
const (
	AssetCreated            EventType = "asset.created"
	AssetUpdated            EventType = "asset.updated"
	AssetDeleted            EventType = "asset.deleted"
	AssetCriticalityChanged EventType = "asset.criticality_changed"
)

// Vulnerability events.
const (
	VulnerabilityDetected EventType = "vulnerability.detected"
	VulnerabilityUpdated  EventType = "vulnerability.updated"
	VulnerabilityDeleted  EventType = "vulnerability.deleted"
)

// Incident events.
const (
	IncidentCreated EventType = "incident.created"
	IncidentUpdated EventType = "incident.updated"
	IncidentDeleted EventType = "incident.deleted"
)

// Control events (compliance controls).
const (
	ControlCreated EventType = "control.created"
	ControlUpdated EventType = "control.updated"
	ControlDeleted EventType = "control.deleted"
)

// Assessment events.
//
// OpenRisk has no aggregate literally named "assessment": the thing the product
// calls an assessment is `ComplianceAudit` (its `internal` type is documented in
// the domain as a self-assessment). Rather than invent an aggregate with no
// mutations behind it, these events are the compliance audit's, named for what
// the domain calls them. The aggregate field says `compliance_audit` so a
// consumer can always find the record.
const (
	AssessmentCreated EventType = "assessment.created"
	AssessmentUpdated EventType = "assessment.updated"
	AssessmentDeleted EventType = "assessment.deleted"
)

// Mitigation events.
const (
	MitigationCreated       EventType = "mitigation.created"
	MitigationUpdated       EventType = "mitigation.updated"
	MitigationDeleted       EventType = "mitigation.deleted"
	MitigationAutoCompleted EventType = "mitigation.auto_completed"
)

// Aggregate names. Stable identifiers for the domain object, independent of the
// table it lives in and the URL it is served at.
const (
	AggregateRisk            = "risk"
	AggregateAsset           = "asset"
	AggregateVulnerability   = "vulnerability"
	AggregateIncident        = "incident"
	AggregateControl         = "control"
	AggregateComplianceAudit = "compliance_audit"
	AggregateMitigation      = "mitigation"
)

// Origin says where an event is produced, which is what tells an operator where
// to look when one stops arriving.
type Origin string

const (
	// OriginMutation: derived from a successful authenticated API mutation, by
	// the same observation that writes the audit entry.
	OriginMutation Origin = "mutation"
	// OriginDomain: published by a background worker or an unauthenticated
	// ingest path, relayed from an internal domain channel.
	OriginDomain Origin = "domain"
)

// Descriptor is one entry of the catalog: everything a consumer needs to decide
// whether it cares about an event type, and everything an operator needs to
// know where it comes from.
type Descriptor struct {
	Type EventType `json:"type"`
	// Aggregate the event is about. Validate() refuses an envelope whose
	// aggregate contradicts this.
	Aggregate string `json:"aggregate"`
	// Version is the CURRENT published payload version for this type.
	Version int `json:"version"`
	// Origin is the publication path.
	Origin Origin `json:"origin"`
	// Trigger, in one line, is the business change that produces it.
	Trigger string `json:"trigger"`
	// PayloadFields lists the field names the payload may carry. It is the
	// compatibility contract: adding a name here is additive, removing or
	// renaming one is a breaking change and requires a version bump. The
	// contract test asserts no forbidden field ever appears in this list.
	PayloadFields []string `json:"payloadFields"`

	// Permission is the permission string a subscriber must hold to receive
	// this event. Derived from the aggregate at init, so a new event type is
	// authorized the moment it is registered and cannot be added with the
	// question left open. Filled in by init(); never written by hand.
	Permission string `json:"permission"`
}

// aggregatePermission maps each aggregate onto the permission that already
// governs reading it through the API.
//
// This is the rule that keeps the stream from becoming a way around the
// permission model: opening a stream requires StreamPermission, but WHAT
// travels down it is decided per event by the same strings that gate the REST
// endpoints. A viewer holding only risks:read sees risk events and nothing
// else — not because the client asked for that, but because the server will not
// send anything more.
var aggregatePermission = map[string]string{
	AggregateRisk:            "risks:read",
	AggregateAsset:           "assets:read",
	AggregateVulnerability:   "vulnerabilities:read",
	AggregateIncident:        "incidents:read",
	AggregateControl:         "compliance:controls:read",
	AggregateComplianceAudit: "compliance:audits:read",
	AggregateMitigation:      "mitigations:read",
}

// StreamPermission is the permission required to open a stream at all.
//
// Holding it is necessary and never sufficient: it says "this identity may hold
// an event stream", while the per-aggregate permissions above say what may
// travel on it.
const StreamPermission = "events:read"

// PermissionForAggregate returns the permission required to receive an
// aggregate's events, and whether one is defined. An aggregate with no entry is
// a defect, not a wildcard — see the catalog test, which fails rather than let
// an unmapped aggregate default to visible.
func PermissionForAggregate(aggregate string) (string, bool) {
	p, ok := aggregatePermission[aggregate]
	return p, ok
}

// init stamps each descriptor with the permission its aggregate requires.
func init() {
	for typ, d := range catalog {
		if perm, ok := aggregatePermission[d.Aggregate]; ok {
			d.Permission = perm
			catalog[typ] = d
		}
	}
}

// catalog is the single source of truth for what may be published.
//
// An event type absent from here cannot be published (Validate refuses it) and
// cannot be subscribed to (Filter refuses it). That is the point: the catalog is
// not documentation that describes the code, it is the code.
var catalog = map[EventType]Descriptor{
	// ---- Risk -------------------------------------------------------------
	RiskCreated: {RiskCreated, AggregateRisk, 1, OriginMutation,
		"A risk is created through the API.",
		[]string{"changedFields", "action", "path"}, ""},
	RiskUpdated: {RiskUpdated, AggregateRisk, 1, OriginMutation,
		"A risk is updated through the API.",
		[]string{"changedFields", "action", "path"}, ""},
	RiskDeleted: {RiskDeleted, AggregateRisk, 1, OriginMutation,
		"A risk is deleted through the API.",
		[]string{"changedFields", "action", "path"}, ""},
	RiskStatusChanged: {RiskStatusChanged, AggregateRisk, 1, OriginMutation,
		"A risk lifecycle transition is accepted (POST /risks/:id/transition).",
		[]string{"changedFields", "action", "path"}, ""},
	RiskScoreChanged: {RiskScoreChanged, AggregateRisk, 1, OriginDomain,
		"The score worker recomputes a risk score (relayed from risk.score_updated).",
		[]string{"newScore", "oldScore", "delta", "criticality"}, ""},

	// ---- Asset ------------------------------------------------------------
	AssetCreated: {AssetCreated, AggregateAsset, 1, OriginMutation,
		"An asset is created through the API.",
		[]string{"changedFields", "action", "path"}, ""},
	AssetUpdated: {AssetUpdated, AggregateAsset, 1, OriginMutation,
		"An asset is updated through the API.",
		[]string{"changedFields", "action", "path"}, ""},
	AssetDeleted: {AssetDeleted, AggregateAsset, 1, OriginMutation,
		"An asset is deleted through the API.",
		[]string{"changedFields", "action", "path"}, ""},
	AssetCriticalityChanged: {AssetCriticalityChanged, AggregateAsset, 1, OriginDomain,
		"An asset's criticality changes (relayed from asset.criticality_changed).",
		[]string{"oldCriticality", "newCriticality"}, ""},

	// ---- Vulnerability ----------------------------------------------------
	VulnerabilityDetected: {VulnerabilityDetected, AggregateVulnerability, 1, OriginDomain,
		"Ingest records a vulnerability the tenant did not have (relayed from vulnerability.detected).",
		[]string{"cveId", "severity", "cvss", "kev", "priorityTier", "assetId", "source"}, ""},
	VulnerabilityUpdated: {VulnerabilityUpdated, AggregateVulnerability, 1, OriginMutation,
		"A vulnerability's remediation status is changed through the API.",
		[]string{"changedFields", "action", "path"}, ""},
	VulnerabilityDeleted: {VulnerabilityDeleted, AggregateVulnerability, 1, OriginMutation,
		"A vulnerability is deleted through the API.",
		[]string{"changedFields", "action", "path"}, ""},

	// ---- Incident ---------------------------------------------------------
	IncidentCreated: {IncidentCreated, AggregateIncident, 1, OriginMutation,
		"An incident is declared through the API.",
		[]string{"changedFields", "action", "path"}, ""},
	IncidentUpdated: {IncidentUpdated, AggregateIncident, 1, OriginMutation,
		"An incident is updated through the API, including status and severity changes.",
		[]string{"changedFields", "action", "path"}, ""},
	IncidentDeleted: {IncidentDeleted, AggregateIncident, 1, OriginMutation,
		"An incident is deleted through the API.",
		[]string{"changedFields", "action", "path"}, ""},

	// ---- Control ----------------------------------------------------------
	ControlCreated: {ControlCreated, AggregateControl, 1, OriginMutation,
		"A compliance control is created through the API.",
		[]string{"changedFields", "action", "path"}, ""},
	ControlUpdated: {ControlUpdated, AggregateControl, 1, OriginMutation,
		"A compliance control is updated through the API, including its implementation status.",
		[]string{"changedFields", "action", "path"}, ""},
	ControlDeleted: {ControlDeleted, AggregateControl, 1, OriginMutation,
		"A compliance control is deleted through the API.",
		[]string{"changedFields", "action", "path"}, ""},

	// ---- Assessment (compliance audit) ------------------------------------
	AssessmentCreated: {AssessmentCreated, AggregateComplianceAudit, 1, OriginMutation,
		"A compliance audit (assessment) is planned through the API.",
		[]string{"changedFields", "action", "path"}, ""},
	AssessmentUpdated: {AssessmentUpdated, AggregateComplianceAudit, 1, OriginMutation,
		"A compliance audit is updated through the API; completion is an update carrying status in changedFields.",
		[]string{"changedFields", "action", "path"}, ""},
	AssessmentDeleted: {AssessmentDeleted, AggregateComplianceAudit, 1, OriginMutation,
		"A compliance audit is deleted through the API.",
		[]string{"changedFields", "action", "path"}, ""},

	// ---- Mitigation -------------------------------------------------------
	MitigationCreated: {MitigationCreated, AggregateMitigation, 1, OriginMutation,
		"A mitigation plan is created through the API.",
		[]string{"changedFields", "action", "path"}, ""},
	MitigationUpdated: {MitigationUpdated, AggregateMitigation, 1, OriginMutation,
		"A mitigation plan or one of its sub-actions is updated through the API.",
		[]string{"changedFields", "action", "path"}, ""},
	MitigationDeleted: {MitigationDeleted, AggregateMitigation, 1, OriginMutation,
		"A mitigation plan is deleted through the API.",
		[]string{"changedFields", "action", "path"}, ""},
	MitigationAutoCompleted: {MitigationAutoCompleted, AggregateMitigation, 1, OriginDomain,
		"The scanner can no longer detect a finding and auto-completes a sub-action (relayed from mitigation.auto_completed).",
		[]string{"planId", "subActionId", "scannerJobId"}, ""},
}

// Lookup returns the catalog entry for a type.
func Lookup(t EventType) (Descriptor, bool) {
	d, ok := catalog[t]
	return d, ok
}

// IsRegistered reports whether a type may be published and subscribed to.
func IsRegistered(t EventType) bool {
	_, ok := catalog[t]
	return ok
}

// Catalog returns every descriptor, sorted by type, for the contract endpoint
// and the documentation generator.
func Catalog() []Descriptor {
	out := make([]Descriptor, 0, len(catalog))
	for _, d := range catalog {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

// Aggregates returns the distinct aggregate names, sorted.
func Aggregates() []string {
	seen := map[string]struct{}{}
	for _, d := range catalog {
		seen[d.Aggregate] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for a := range seen {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

// AllowedAggregates returns the aggregates a caller may receive, given a
// predicate that reports whether the caller holds a permission string.
//
// The predicate is supplied by the caller rather than the permission model
// being imported here, so this package stays free of `internal/` — and so the
// wildcard semantics used everywhere else in the API ("*" and "risks:*") are
// applied by the one implementation that already knows them, instead of being
// re-derived here where they could drift.
func AllowedAggregates(holds func(permission string) bool) []string {
	if holds == nil {
		return nil
	}
	var out []string
	for _, agg := range Aggregates() {
		perm, ok := aggregatePermission[agg]
		if !ok {
			// An aggregate with no permission mapping is withheld. Failing
			// closed here means forgetting the mapping costs visibility, not
			// confidentiality.
			continue
		}
		if holds(perm) {
			out = append(out, agg)
		}
	}
	sort.Strings(out)
	return out
}

// Restrict narrows a client's requested filter to what it is allowed to see.
//
// Order matters and is the whole point: the caller's request can only ever
// SUBTRACT from the allowed set. Asking for an aggregate that is not permitted
// yields nothing rather than an error, because a client legitimately asking for
// "risks and assets" while holding only risks should get its risks — but it can
// never widen its way to the assets.
//
// The second return value reports whether anything at all can be delivered; a
// subscription that could never deliver anything is refused rather than left
// open as a stream that mysteriously stays silent.
func Restrict(requested Filter, allowedAggregates []string) (Filter, bool) {
	allowed := map[string]struct{}{}
	for _, a := range allowedAggregates {
		allowed[a] = struct{}{}
	}
	if len(allowed) == 0 {
		return Filter{}, false
	}

	out := Filter{}
	if len(requested.Types) > 0 {
		for _, t := range requested.Types {
			d, ok := catalog[t]
			if !ok {
				continue
			}
			if _, permitted := allowed[d.Aggregate]; permitted {
				out.Types = append(out.Types, t)
			}
		}
		if len(out.Types) == 0 {
			return Filter{}, false
		}
	}

	if len(requested.Aggregates) > 0 {
		for _, a := range requested.Aggregates {
			if _, permitted := allowed[a]; permitted {
				out.Aggregates = append(out.Aggregates, a)
			}
		}
		if len(out.Aggregates) == 0 {
			return Filter{}, false
		}
	} else if len(out.Types) == 0 {
		// No narrowing was requested, so the permitted set IS the filter. This
		// is what makes an unfiltered subscription safe: it is never "give me
		// everything", it is "give me everything I am allowed to see".
		out.Aggregates = append(out.Aggregates, allowedAggregates...)
	}

	sort.Slice(out.Types, func(i, j int) bool { return out.Types[i] < out.Types[j] })
	sort.Strings(out.Aggregates)
	return out, true
}
