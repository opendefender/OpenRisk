// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// provenanceStringCap is D3's cap on every string in provenance.
const provenanceStringCap = 255

// Method is how a record reached OpenRisk (D3, closed).
type Method string

// The provenance methods of ADR 0008 D3.
const (
	MethodManualEntry   Method = "manual_entry"
	MethodFileImport    Method = "file_import"
	MethodAPI           Method = "api"
	MethodScan          Method = "scan"
	MethodFeed          Method = "feed"
	MethodQuestionnaire Method = "questionnaire"
	MethodDerivation    Method = "derivation"
	MethodMigration     Method = "migration"
)

var methods = map[Method]bool{
	MethodManualEntry: true, MethodFileImport: true, MethodAPI: true, MethodScan: true,
	MethodFeed: true, MethodQuestionnaire: true, MethodDerivation: true, MethodMigration: true,
}

// ActorKind is who stored a record (D3, closed).
type ActorKind string

// The actor kinds of ADR 0008 D3.
const (
	ActorUser          ActorKind = "user"
	ActorSystem        ActorKind = "system"
	ActorVendorContact ActorKind = "vendor_contact"
)

// vendorContactPrefix is ADR 0004 D4's actor id shape: vendor_contact:<assessment_id>.
const vendorContactPrefix = "vendor_contact:"

// Actor is provenance.actor and ControlTestResult.executed_by: `{kind, id}`.
// `id` is a user id, a worker name or `vendor_contact:<assessment_id>`.
type Actor struct {
	Kind ActorKind `json:"kind"`
	ID   string    `json:"id"`
}

// RecordRef is `{type, id}`: a same-tenant canonical record, as named by
// provenance.derived_from and by an edge's `from` end.
type RecordRef struct {
	Type EntityType `json:"type"`
	ID   uuid.UUID  `json:"id"`
}

// ExternalRef is `{scheme, value}`: a reference to a global catalogue that is
// not a tenant record, e.g. `{cve, CVE-2024-3094}`. D3 gives examples of
// schemes (`cve`, `starter_catalogue`, `ai_proposal`) without closing the set,
// so the scheme is checked for presence and shape, not membership.
type ExternalRef struct {
	Scheme string `json:"scheme"`
	Value  string `json:"value"`
}

// Provenance is how a record reached OpenRisk (ADR 0008 D3). Keys not listed
// are refused: UnmarshalJSON rejects unknown keys, and Validate rejects a
// source-specific key the record's source does not allowlist.
type Provenance struct {
	Method         Method        `json:"method"`
	Actor          Actor         `json:"actor"`
	Via            *string       `json:"via,omitempty"`
	IngestID       *string       `json:"ingest_id,omitempty"`
	DerivedFrom    []RecordRef   `json:"derived_from,omitempty"`
	ExternalRefs   []ExternalRef `json:"external_refs,omitempty"`
	Rule           *string       `json:"rule,omitempty"`
	ConfidenceRule string        `json:"confidence_rule"`
	RawRef         *string       `json:"raw_ref,omitempty"`

	// Source-specific keys. Today only vendor_questionnaire allowlists any:
	// ADR 0004 D3's assessment keys, adopted unchanged by ADR 0008 D8.
	Channel              *string    `json:"channel,omitempty"`
	TemplateID           *uuid.UUID `json:"template_id,omitempty"`
	TemplateVersion      *int       `json:"template_version,omitempty"`
	SubmittedWithTokenID *uuid.UUID `json:"submitted_with_token_id,omitempty"`
}

// sourceSpecificKeys are the provenance keys outside D3's common set, each with
// a probe telling whether a record sets it.
var sourceSpecificKeys = []struct {
	key string
	set func(Provenance) bool
}{
	{"channel", func(p Provenance) bool { return p.Channel != nil }},
	{"template_id", func(p Provenance) bool { return p.TemplateID != nil }},
	{"template_version", func(p Provenance) bool { return p.TemplateVersion != nil }},
	{"submitted_with_token_id", func(p Provenance) bool { return p.SubmittedWithTokenID != nil }},
}

// provenanceAllowlist is the per-source registry of extra provenance keys (D3, D8).
var provenanceAllowlist = map[Source]map[string]bool{
	SourceVendorQuestionnaire: {
		"channel": true, "template_id": true, "template_version": true, "submitted_with_token_id": true,
	},
}

// ProvenanceAllowlist returns the extra keys a source may carry, sorted as
// declared. Empty for every source but vendor_questionnaire.
func ProvenanceAllowlist(source Source) []string {
	var out []string
	for _, k := range sourceSpecificKeys {
		if provenanceAllowlist[source][k.key] {
			out = append(out, k.key)
		}
	}
	return out
}

// UnmarshalJSON decodes provenance strictly: a key spelled other than exactly
// as D3 or a source allowlist spells it, a duplicate key, at any nesting level,
// or trailing data is refused. Whether the record's own source allowlists a
// source-specific key is checked by Validate, which knows the source.
func (p *Provenance) UnmarshalJSON(b []byte) error {
	type plain Provenance
	var v plain
	if err := decodeStrict("provenance", b, provenanceShape, &v); err != nil {
		return err
	}
	*p = Provenance(v)
	return nil
}

// ParseProvenance decodes raw provenance strictly and validates it for source.
// It is the entry point for provenance read from a JSONB column or a feed.
// Every failure, malformed JSON included, satisfies errors.Is(err, ErrValidation).
func ParseProvenance(raw []byte, source Source) (Provenance, error) {
	var p Provenance
	if err := json.Unmarshal(raw, &p); err != nil {
		if !errors.Is(err, ErrValidation) {
			return Provenance{}, invalid("provenance", "malformed JSON")
		}
		return Provenance{}, err
	}
	if err := p.Validate(source); err != nil {
		return Provenance{}, err
	}
	return p, nil
}

// Validate applies D3 (and D8 for vendor_questionnaire) for a record of source.
func (p Provenance) Validate(source Source) error {
	class, err := validateSource("source", source)
	if err != nil {
		return err
	}
	if !methods[p.Method] {
		return invalid("provenance.method", "is required and must be one of the D3 methods")
	}
	if err := within("provenance", p.Actor.validate("actor")); err != nil {
		return err
	}
	if !knownConfidenceRules[p.ConfidenceRule] {
		return invalid("provenance.confidence_rule", "is required and must be %s", ConfidenceRule)
	}
	if err := p.checkStrings(); err != nil {
		return err
	}
	if p.Via != nil && strings.Contains(*p.Via, "://") {
		return invalid("provenance.via", "is a registry key or an id, never a URL")
	}
	for i, ref := range p.DerivedFrom {
		field := fmt.Sprintf("provenance.derived_from[%d]", i)
		if !derivableTypes[ref.Type] {
			return invalid(field+".type", "must name a canonical entity type")
		}
		if err := requireID(field+".id", ref.ID); err != nil {
			return err
		}
	}
	for i, ref := range p.ExternalRefs {
		field := fmt.Sprintf("provenance.external_refs[%d]", i)
		if blank(ref.Scheme) {
			return invalid(field+".scheme", "is required")
		}
		if blank(ref.Value) {
			return invalid(field+".value", "is required")
		}
	}
	if class == ClassDerived && len(p.DerivedFrom) == 0 && len(p.ExternalRefs) == 0 {
		return invalid("provenance", "a derived record must name derived_from or external_refs")
	}
	for _, k := range sourceSpecificKeys {
		if k.set(p) && !provenanceAllowlist[source][k.key] {
			return invalid("provenance."+k.key, "is not allowlisted for source %s", source)
		}
	}
	if err := optionalID("provenance.template_id", p.TemplateID); err != nil {
		return err
	}
	if err := optionalID("provenance.submitted_with_token_id", p.SubmittedWithTokenID); err != nil {
		return err
	}
	if source == SourceVendorQuestionnaire {
		// D8: fixed values for this source.
		if p.Method != MethodQuestionnaire {
			return invalid("provenance.method", "must be %s for %s", MethodQuestionnaire, source)
		}
		if p.Actor.Kind != ActorVendorContact {
			return invalid("provenance.actor.kind", "must be %s for %s", ActorVendorContact, source)
		}
	}
	return nil
}

// checkStrings applies D3's two enforced rules to every string in provenance:
// at most 255 characters, and never a credential-shaped URL.
func (p Provenance) checkStrings() error {
	type named struct {
		field string
		value string
	}
	all := []named{{"provenance.actor.id", p.Actor.ID}, {"provenance.confidence_rule", p.ConfidenceRule}}
	for _, opt := range []struct {
		field string
		value *string
	}{
		{"provenance.via", p.Via}, {"provenance.ingest_id", p.IngestID}, {"provenance.rule", p.Rule},
		{"provenance.raw_ref", p.RawRef}, {"provenance.channel", p.Channel},
	} {
		if opt.value != nil {
			all = append(all, named{opt.field, *opt.value})
		}
	}
	for i, ref := range p.DerivedFrom {
		all = append(all, named{fmt.Sprintf("provenance.derived_from[%d].type", i), string(ref.Type)})
	}
	for i, ref := range p.ExternalRefs {
		all = append(all,
			named{fmt.Sprintf("provenance.external_refs[%d].scheme", i), ref.Scheme},
			named{fmt.Sprintf("provenance.external_refs[%d].value", i), ref.Value})
	}
	for _, s := range all {
		if err := maxLen(s.field, s.value, provenanceStringCap); err != nil {
			return err
		}
		if credentialShaped(s.value) {
			return invalid(s.field, "carries a URL with userinfo or a signature/token parameter")
		}
	}
	return nil
}

// validate checks the `{kind, id}` shape shared by provenance.actor and
// ControlTestResult.executed_by.
func (a Actor) validate(field string) error {
	if err := requireText(field+".id", a.ID, provenanceStringCap); err != nil {
		return err
	}
	if credentialShaped(a.ID) {
		return invalid(field+".id", "carries a URL with userinfo or a signature/token parameter")
	}
	switch a.Kind {
	case ActorUser:
		// Canonical lowercase hyphenated form only. uuid.Parse also accepts
		// upper case, braces, urn:uuid: and bare hex; admitting those would let
		// one user appear under several spellings and defeat every comparison
		// of actor ids, the user_entered uplift's first of all.
		if !isCanonicalUUID(a.ID) {
			return invalid(field+".id", "must be a user id in canonical lowercase form for kind %s", ActorUser)
		}
	case ActorVendorContact:
		rest, ok := strings.CutPrefix(a.ID, vendorContactPrefix)
		if !ok || !isCanonicalUUID(rest) {
			return invalid(field+".id", "must be %s<assessment_id>, the id in canonical lowercase form, for kind %s", vendorContactPrefix, ActorVendorContact)
		}
	case ActorSystem:
		// a worker name; presence and the caps above are the whole rule
	default:
		return invalid(field+".kind", "must be one of user, system, vendor_contact")
	}
	return nil
}

// isCanonicalUUID is true for a non-nil uuid written exactly as uuid.UUID.String
// writes it.
func isCanonicalUUID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id != uuid.Nil && id.String() == s
}
