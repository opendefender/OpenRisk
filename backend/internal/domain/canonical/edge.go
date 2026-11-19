// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/google/uuid"
)

// EntityType names a canonical record type, spelled as ADR 0008 writes it.
type EntityType string

// The seven entities ADR 0008 D6 specifies.
const (
	TypeRequirement           EntityType = "Requirement"
	TypeControl               EntityType = "Control"
	TypeControlImplementation EntityType = "ControlImplementation"
	TypeControlTest           EntityType = "ControlTest"
	TypeEvidence              EntityType = "Evidence"
	TypeRisk                  EntityType = "Risk"
	TypeFinding               EntityType = "Finding"
)

// The two entities adopted from ADR 0004 (D8), whose shape this package does
// not define.
const (
	TypeVendor     EntityType = "Vendor"
	TypeAssessment EntityType = "Assessment"
)

// Shape-deferred edge targets (D5): the edge stores their type and id; their
// fields are validated once #1217 fixes them. Deferral never skips the tenant
// check.
const (
	TypeAsset      EntityType = "Asset"
	TypeSystem     EntityType = "System"
	TypeService    EntityType = "Service"
	TypeThreat     EntityType = "Threat"
	TypeAction     EntityType = "Action"
	TypeMitigation EntityType = "Mitigation"
)

// derivableTypes are the record types provenance.derived_from may name: the
// seven specified entities and the two adopted ones (D3's derived table names
// a Finding, an Assessment and a ControlTest as inputs).
var derivableTypes = map[EntityType]bool{
	TypeRequirement: true, TypeControl: true, TypeControlImplementation: true, TypeControlTest: true,
	TypeEvidence: true, TypeRisk: true, TypeFinding: true, TypeVendor: true, TypeAssessment: true,
}

// Verb is a relationship verb from the closed vocabulary of ADR 0008 D5.
type Verb string

// The fifteen verbs of D5.
const (
	VerbAddressedBy   Verb = "addressed_by"
	VerbImplementedBy Verb = "implemented_by"
	VerbTestedBy      Verb = "tested_by"
	VerbEvidencedBy   Verb = "evidenced_by"
	VerbMapsTo        Verb = "maps_to"
	VerbMitigatedBy   Verb = "mitigated_by"
	VerbIndicates     Verb = "indicates"
	VerbViolates      Verb = "violates"
	VerbRaisedBy      Verb = "raised_by"
	VerbAffects       Verb = "affects"
	VerbAppliesTo     Verb = "applies_to"
	VerbCausedBy      Verb = "caused_by"
	VerbRequires      Verb = "requires"
	VerbTreatedBy     Verb = "treated_by"
	VerbAssesses      Verb = "assesses"
)

// IsSymmetric is true for maps_to only: stored once per unordered pair, A→B
// and B→A being the same edge (D5). Every other verb has one direction.
func (v Verb) IsSymmetric() bool { return v == VerbMapsTo }

// Attribute names, as D5 spells them.
const (
	attrCoverage                      = "coverage"
	attrRationale                     = "rationale"
	attrOrigin                        = "origin"
	attrNote                          = "note"
	attrMatchConfidence               = "match_confidence"
	attrMatchMethod                   = "match_method"
	attrMatchAmbiguous                = "match_ambiguous"
	attrMatchCandidates               = "match_candidates"
	attrResolvedBy                    = "resolved_by"
	attrAssetCriticalityAtObservation = "asset_criticality_at_observation"
)

// Triple is one row of the D5 table: a verb from one type to another.
type Triple struct {
	Verb Verb
	From EntityType
	To   EntityType
}

// tripleRule is what a D5 row allows. `result_id`, which D5 lists in the
// Attributes column of the two result-bound rows, is the edge's `qualifier`
// ("result_id where the verb takes one; part of the identity"), so it is the
// qualifier rule here, not an attribute.
type tripleRule struct {
	qualifier bool
	attrs     []string
	required  []string
}

// vocabulary is ADR 0008 D5, row by row. Nothing outside it validates.
var vocabulary = map[Triple]tripleRule{
	{VerbAddressedBy, TypeRequirement, TypeControl}:             {attrs: []string{attrCoverage, attrRationale}},
	{VerbImplementedBy, TypeControl, TypeControlImplementation}: {},
	{VerbTestedBy, TypeControlImplementation, TypeControlTest}:  {},
	{VerbEvidencedBy, TypeControlTest, TypeEvidence}:            {qualifier: true, attrs: []string{attrNote}},
	{VerbEvidencedBy, TypeFinding, TypeEvidence}:                {attrs: []string{attrNote}},
	{VerbEvidencedBy, TypeControl, TypeEvidence}:                {attrs: []string{attrNote}}, // transitional (#553 criterion 6)
	{VerbMapsTo, TypeRequirement, TypeRequirement}:              {attrs: []string{attrCoverage, attrRationale, attrOrigin}, required: []string{attrRationale}},
	{VerbMitigatedBy, TypeRisk, TypeControl}:                    {attrs: []string{attrNote}},
	{VerbIndicates, TypeFinding, TypeRisk}:                      {},
	{VerbViolates, TypeFinding, TypeControl}:                    {},
	{VerbRaisedBy, TypeFinding, TypeControlTest}:                {qualifier: true},
	{VerbAffects, TypeFinding, TypeAsset}:                       {attrs: []string{attrMatchConfidence, attrMatchMethod, attrMatchAmbiguous, attrMatchCandidates, attrResolvedBy, attrAssetCriticalityAtObservation}},
	{VerbAffects, TypeRisk, TypeAsset}:                          {},
	{VerbAppliesTo, TypeControlImplementation, TypeAsset}:       {},
	{VerbAppliesTo, TypeControlImplementation, TypeSystem}:      {},
	{VerbAppliesTo, TypeControlImplementation, TypeService}:     {},
	{VerbCausedBy, TypeFinding, TypeThreat}:                     {},
	{VerbRequires, TypeFinding, TypeAction}:                     {},
	{VerbTreatedBy, TypeRisk, TypeMitigation}:                   {},
	{VerbAssesses, TypeAssessment, TypeVendor}:                  {},
}

// Vocabulary lists every allowed triple, sorted, for #542 to publish.
func Vocabulary() []Triple {
	out := make([]Triple, 0, len(vocabulary))
	for t := range vocabulary {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Verb != b.Verb {
			return a.Verb < b.Verb
		}
		if a.From != b.From {
			return a.From < b.From
		}
		return a.To < b.To
	})
	return out
}

// Coverage is how fully one end answers the other (addressed_by, maps_to).
type Coverage string

// D5's coverage values.
const (
	CoverageFull    Coverage = "full"
	CoveragePartial Coverage = "partial"
)

// CrosswalkOrigin is where a maps_to edge came from.
type CrosswalkOrigin string

// D5's maps_to origins.
const (
	OriginCurated CrosswalkOrigin = "curated"
	OriginManual  CrosswalkOrigin = "manual"
)

// EdgeAttributes is the closed union of every attribute D5 names. Which ones a
// given edge may carry is fixed by its triple; decoding refuses any other key.
type EdgeAttributes struct {
	Coverage  *Coverage        `json:"coverage,omitempty"`
	Rationale *string          `json:"rationale,omitempty"`
	Origin    *CrosswalkOrigin `json:"origin,omitempty"`
	Note      *string          `json:"note,omitempty"`

	// Finding --affects--> Asset: the attribution confidence, kept apart from
	// the record's confidence (ADR 0008 Context 3).
	MatchConfidence               *float64        `json:"match_confidence,omitempty"`
	MatchMethod                   *string         `json:"match_method,omitempty"`
	MatchAmbiguous                *bool           `json:"match_ambiguous,omitempty"`
	MatchCandidates               json.RawMessage `json:"match_candidates,omitempty"`
	ResolvedBy                    *uuid.UUID      `json:"resolved_by,omitempty"`
	AssetCriticalityAtObservation *string         `json:"asset_criticality_at_observation,omitempty"`
}

// UnmarshalJSON refuses any attribute key outside D5, a key spelled other than
// exactly as D5 spells it, and a duplicate key.
func (a *EdgeAttributes) UnmarshalJSON(b []byte) error {
	type plain EdgeAttributes
	var v plain
	if err := decodeStrict("attributes", b, attributesShape, &v); err != nil {
		return err
	}
	*a = EdgeAttributes(v)
	return nil
}

func (a EdgeAttributes) present() []string {
	var out []string
	add := func(set bool, name string) {
		if set {
			out = append(out, name)
		}
	}
	add(a.Coverage != nil, attrCoverage)
	add(a.Rationale != nil, attrRationale)
	add(a.Origin != nil, attrOrigin)
	add(a.Note != nil, attrNote)
	add(a.MatchConfidence != nil, attrMatchConfidence)
	add(a.MatchMethod != nil, attrMatchMethod)
	add(a.MatchAmbiguous != nil, attrMatchAmbiguous)
	add(len(a.MatchCandidates) > 0, attrMatchCandidates)
	add(a.ResolvedBy != nil, attrResolvedBy)
	add(a.AssetCriticalityAtObservation != nil, attrAssetCriticalityAtObservation)
	return out
}

// Caps on edge attributes (A9). The two labels follow provenance's 255 (D3);
// the opaque match_candidates ranking is bounded so an edge cannot carry a
// payload. note and rationale are uncapped prose.
const (
	attributeStringCap   = 255
	matchCandidatesBytes = 16 << 10
)

func (a EdgeAttributes) validateValues() error {
	// match_method and asset_criticality_at_observation are short labels:
	// capped and checked whole, like provenance strings (A9).
	for _, attr := range []struct {
		field string
		value *string
	}{
		{"attributes.match_method", a.MatchMethod},
		{"attributes.asset_criticality_at_observation", a.AssetCriticalityAtObservation},
	} {
		if attr.value == nil {
			continue
		}
		if err := maxLen(attr.field, *attr.value, attributeStringCap); err != nil {
			return err
		}
		if credentialShaped(*attr.value) {
			return invalid(attr.field, "carries a URL with userinfo or a signature/token parameter")
		}
	}
	// note and rationale are tenant prose, text and uncapped (A9). Only the
	// tokens that are URLs are checked, so "escalate to alice@corp.com" is
	// prose, and a pasted presigned link is still refused.
	for _, attr := range []struct {
		field string
		value *string
	}{
		{"attributes.rationale", a.Rationale}, {"attributes.note", a.Note},
	} {
		if attr.value != nil && proseCarriesCredentialURL(*attr.value) {
			return invalid(attr.field, "carries a URL with userinfo or a signature/token parameter")
		}
	}
	if len(a.MatchCandidates) > matchCandidatesBytes {
		return invalid("attributes.match_candidates", "exceeds %d bytes", matchCandidatesBytes)
	}
	if a.Coverage != nil && *a.Coverage != CoverageFull && *a.Coverage != CoveragePartial {
		return invalid("attributes.coverage", "must be full or partial")
	}
	if a.Origin != nil && *a.Origin != OriginCurated && *a.Origin != OriginManual {
		return invalid("attributes.origin", "must be curated or manual")
	}
	if err := optionalRange("attributes.match_confidence", a.MatchConfidence, 0, 1); err != nil {
		return err
	}
	if len(a.MatchCandidates) > 0 && !json.Valid(a.MatchCandidates) {
		return invalid("attributes.match_candidates", "must be valid JSON")
	}
	return optionalID("attributes.resolved_by", a.ResolvedBy)
}

// EdgeTarget is an edge's `to` end. TenantID is filled by persistence when it
// resolves the target, never by a caller, so the pure validator can compare it
// with the edge's own tenant (D5).
type EdgeTarget struct {
	Type     EntityType `json:"type"`
	ID       uuid.UUID  `json:"id"`
	TenantID uuid.UUID  `json:"tenant_id"`
}

// Edge is a relationship record (ADR 0008 D5). It is stored on its source end,
// inherits that end's tenant, and carries its own source and confidence.
type Edge struct {
	ID         uuid.UUID      `json:"id"`
	TenantID   uuid.UUID      `json:"tenant_id"`
	From       RecordRef      `json:"from"`
	Verb       Verb           `json:"verb"`
	To         EdgeTarget     `json:"to"`
	Qualifier  *uuid.UUID     `json:"qualifier"`
	Attributes EdgeAttributes `json:"attributes"`
	Source     Source         `json:"source"`
	SourceID   string         `json:"source_id"`
	Confidence float64        `json:"confidence"`
	Provenance Provenance     `json:"provenance"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// Validate applies D5: both ends in one tenant, a triple of the vocabulary,
// the qualifier and attribute rules of that triple, and D2/D3 on the edge's
// own source, confidence and provenance.
func (e Edge) Validate() error {
	if err := requireID("edge.tenant_id", e.TenantID); err != nil {
		return err
	}
	if err := requireID("edge.to.tenant_id", e.To.TenantID); err != nil {
		return err
	}
	if e.To.TenantID != e.TenantID {
		return invalid("edge.to.tenant_id", "must equal the edge's tenant_id: an edge never crosses tenants")
	}
	if err := requireID("edge.id", e.ID); err != nil {
		return err
	}
	if err := requireID("edge.from.id", e.From.ID); err != nil {
		return err
	}
	if err := requireID("edge.to.id", e.To.ID); err != nil {
		return err
	}
	rule, ok := vocabulary[Triple{e.Verb, e.From.Type, e.To.Type}]
	if !ok {
		// Fixed message: the verb and types are unvalidated input here, and
		// echoing them would carry whatever they hold into logs.
		return invalid("edge", "the (verb, from type, to type) triple is not in the ADR 0008 D5 vocabulary")
	}
	if rule.qualifier {
		if e.Qualifier == nil || *e.Qualifier == uuid.Nil {
			return invalid("edge.qualifier", "%s from %s requires the result_id it is bound to", e.Verb, e.From.Type)
		}
	} else if e.Qualifier != nil {
		return invalid("edge.qualifier", "%s from %s takes no qualifier", e.Verb, e.From.Type)
	}
	allowed := map[string]bool{}
	for _, a := range rule.attrs {
		allowed[a] = true
	}
	for _, a := range e.Attributes.present() {
		if !allowed[a] {
			return invalid("edge.attributes."+a, "is not an attribute of %s from %s", e.Verb, e.From.Type)
		}
	}
	for _, a := range rule.required {
		if a == attrRationale && blankPtr(e.Attributes.Rationale) {
			return invalid("edge.attributes.rationale", "is required on %s", e.Verb)
		}
	}
	if err := within("edge", e.Attributes.validateValues()); err != nil {
		return err
	}
	if e.Verb == VerbMapsTo && e.From.ID == e.To.ID {
		return invalid("edge.to.id", "a requirement does not map to itself")
	}
	if _, err := validateSource("edge.source", e.Source); err != nil {
		return err
	}
	if err := requireText("edge.source_id", e.SourceID, 255); err != nil {
		return err
	}
	if err := validateConfidence("edge.confidence", e.Source, e.Confidence, ReviewContext{ActorID: e.Provenance.Actor.ID}); err != nil {
		return err
	}
	if err := within("edge", e.Provenance.Validate(e.Source)); err != nil {
		return err
	}
	if err := requireTime("edge.created_at", e.CreatedAt); err != nil {
		return err
	}
	return requireTime("edge.updated_at", e.UpdatedAt)
}

// EdgeKey is an edge's identity, `(tenant_id, from, verb, to, qualifier)` (D5).
type EdgeKey struct {
	TenantID  uuid.UUID
	From      RecordRef
	Verb      Verb
	To        RecordRef
	Qualifier uuid.UUID
}

// Key returns the edge's identity. For maps_to, the symmetric verb, the two
// ends are ordered so that A→B and B→A yield the same key: one edge per
// unordered pair, as control_crosswalks already enforces.
func (e Edge) Key() EdgeKey {
	from, to := e.From, RecordRef{Type: e.To.Type, ID: e.To.ID}
	if e.Verb.IsSymmetric() && (to.Type < from.Type || (to.Type == from.Type && to.ID.String() < from.ID.String())) {
		from, to = to, from
	}
	k := EdgeKey{TenantID: e.TenantID, From: from, Verb: e.Verb, To: to}
	if e.Qualifier != nil {
		k.Qualifier = *e.Qualifier
	}
	return k
}
