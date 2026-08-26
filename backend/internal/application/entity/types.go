// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package entity is the universal entity drawer's server side (W1-02).
//
// The drawer is one UI primitive over eight different business objects. That
// only works if the server answers the same four questions about all of them —
// what is this, what is it connected to, what happened to it, and what may I do
// to it — in one shape. This package is that shape.
//
// It owns no data. Every resolver composes the module repositories that already
// exist, each of which is tenant-scoped, so the drawer inherits RULE #2 rather
// than re-implementing it. Nothing here queries the database directly.
//
// Two rules govern what may appear in these structs:
//
//   - A field is present because a repository returned it. There is no default,
//     no placeholder and no locally computed stand-in. Where a value genuinely
//     does not exist for a type, the field is absent and the client renders the
//     absence (see Score.Available).
//   - A relation is present because the CURRENT tenant can read it. Relations
//     cross module boundaries, which is precisely where a cross-tenant leak
//     hides, so each side is resolved through its own tenant-scoped repository
//     rather than trusted because the parent was.
package entity

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// =============================================================================
// Entity types
// =============================================================================

// Type is the canonical name of a business object the drawer can show.
//
// The mission for W1-02 names eight. Two of them do not exist under that name in
// this domain and are mapped to what is actually stored rather than invented:
//
//	finding — there is no findings table. A scanner's FindingDiscovery is a
//	          transient preview struct; a finding becomes persistent only when it
//	          is upserted as a Vulnerability. TypeFinding is therefore the
//	          scanner/assessment-sourced projection of a vulnerability, and the
//	          resolver refuses an id whose row was not produced by a scan.
//	vendor  — there is no vendors table. domain.CategoryVendor is a first-class
//	          asset category with its own attribute schema, so a vendor IS an
//	          asset, and TypeVendor is the asset resolver restricted to that
//	          category.
//
// Modelling those two as separate tables would have meant inventing storage the
// product does not have, which is the failure mode this wave exists to prevent.
type Type string

const (
	TypeAsset         Type = "asset"
	TypeRisk          Type = "risk"
	TypeVulnerability Type = "vulnerability"
	TypeFinding       Type = "finding"
	TypeControl       Type = "control"
	TypeIncident      Type = "incident"
	TypeVendor        Type = "vendor"
	TypeEvidence      Type = "evidence"
)

// Types is the ordered, complete set of drawer-addressable types.
var Types = []Type{
	TypeAsset, TypeRisk, TypeVulnerability, TypeFinding,
	TypeControl, TypeIncident, TypeVendor, TypeEvidence,
}

// ParseType validates a type coming off the wire. An unknown type is a
// validation error, never a silent fallback: falling back would let a caller
// probe for types by watching which ones answer differently.
func ParseType(s string) (Type, error) {
	t := Type(strings.ToLower(strings.TrimSpace(s)))
	for _, known := range Types {
		if t == known {
			return t, nil
		}
	}
	return "", domain.NewValidationError(fmt.Sprintf("unknown entity type %q", s))
}

// Ref names one entity. It is the value a relation points at and the value a
// timeline event targets.
type Ref struct {
	Type Type   `json:"type"`
	ID   string `json:"id"`
}

// =============================================================================
// Sections — what a type's drawer is made of
// =============================================================================

// Section is one region of the drawer. A type declares which sections it
// supports; the client renders exactly those and no more, so a type never shows
// an empty "Controls" tab that could never have been filled.
type Section string

const (
	SectionSummary   Section = "summary"
	SectionRelations Section = "relations"
	SectionTimeline  Section = "timeline"
	SectionAudit     Section = "audit"
)

// =============================================================================
// Summary
// =============================================================================

// FieldKind tells the client how to render a value without the client having to
// know the entity. It is presentation intent, not a data type.
type FieldKind string

const (
	FieldText     FieldKind = "text"
	FieldDate     FieldKind = "date"
	FieldUser     FieldKind = "user"
	FieldBadge    FieldKind = "badge"
	FieldNumber   FieldKind = "number"
	FieldMoney    FieldKind = "money"
	FieldLink     FieldKind = "link"
	FieldTagList  FieldKind = "tags"
	FieldBoolean  FieldKind = "boolean"
	FieldMultilne FieldKind = "multiline"
)

// Field is one labelled value in the summary. Label is English; the client owns
// translation through a stable Key, which is why both are carried.
type Field struct {
	Key   string    `json:"key"`
	Label string    `json:"label"`
	Value string    `json:"value"`
	Kind  FieldKind `json:"kind"`
	// Tone optionally colours the value (severity, status). Empty means neutral.
	Tone string `json:"tone,omitempty"`
	// Values carries a list when Kind is tags; Value stays empty then.
	Values []string `json:"values,omitempty"`
	// Href deep-links the value when Kind is link.
	Href string `json:"href,omitempty"`
}

// Chip is a short coloured label — a status or a severity.
type Chip struct {
	Value string `json:"value"`
	Label string `json:"label"`
	// Tone maps to the design system's badge intents: critical|high|medium|low|
	// success|info|neutral|warning.
	Tone string `json:"tone"`
}

// Score is a business score the entity actually carries.
//
// Available exists so the client can say "score unavailable" honestly instead of
// rendering a zero. A risk that has never been through the score engine, a
// vulnerability with no CVSS, an evidence artifact that has no notion of a score
// at all: all three are Available=false, and none of them may be shown as 0.
// Nothing in this package computes a score — every value is read from the column
// the owning module wrote (§13).
type Score struct {
	Available bool `json:"available"`
	// Key identifies which score this is: risk_score | smart_score | cvss |
	// priority | criticality | coverage.
	Key   string  `json:"key"`
	Label string  `json:"label"`
	Value float64 `json:"value"`
	Max   float64 `json:"max"`
	// Tone bands the value for colouring.
	Tone string `json:"tone,omitempty"`
	// Basis names the engine that produced it, so a reader can tell a CVSS from
	// a P×I×AC product without guessing from the number.
	Basis string `json:"basis,omitempty"`
	// Unavailable explains WHY there is no score, when there is no score.
	Unavailable string `json:"unavailable,omitempty"`
}

// Actor is a person, resolved for display. Email may be empty when the id could
// not be resolved; the client then shows the short id rather than a blank.
type Actor struct {
	ID    string `json:"id,omitempty"`
	Email string `json:"email,omitempty"`
	Label string `json:"label,omitempty"`
}

// Summary is the normalised head of every drawer.
type Summary struct {
	Type     Type   `json:"type"`
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
	// TypeLabel is the human name of the type ("Risk", "Control").
	TypeLabel string `json:"type_label"`

	Status   *Chip `json:"status,omitempty"`
	Severity *Chip `json:"severity,omitempty"`
	Score    Score `json:"score"`

	Owner  *Actor  `json:"owner,omitempty"`
	Fields []Field `json:"fields"`

	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`

	// URL is the canonical in-app deep link that opens this drawer.
	URL string `json:"url"`
	// Sections the drawer should offer for this entity.
	Sections []Section `json:"sections"`
}

// =============================================================================
// Relations
// =============================================================================

// Relation is one edge, already resolved to something the caller may read.
type Relation struct {
	Ref
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
	Status   *Chip  `json:"status,omitempty"`
	Severity *Chip  `json:"severity,omitempty"`
	// RelationLabel names the edge itself when it carries meaning of its own
	// (an asset dependency's "runs_on", a control mapping's framework).
	RelationLabel string `json:"relation_label,omitempty"`
	// URL opens this relation's own drawer.
	URL string `json:"url"`
}

// RelationGroup is one titled list of edges of a single target type.
//
// A group is present with Denied=true rather than omitted when the caller lacks
// the target type's read permission: hiding it entirely would make "no linked
// vulnerabilities" and "you may not see linked vulnerabilities" look identical,
// and the second one has an action attached (ask an admin).
type RelationGroup struct {
	Key        Section    `json:"-"`
	GroupKey   string     `json:"key"`
	Label      string     `json:"label"`
	TargetType Type       `json:"target_type"`
	Items      []Relation `json:"items"`
	Total      int        `json:"total"`
	// Truncated is true when Total exceeds what Items carries.
	Truncated bool `json:"truncated"`
	// Denied marks a group the caller may not read.
	Denied bool `json:"denied"`
	// Error carries a per-group failure so one broken source degrades one group
	// instead of the whole drawer (§27).
	Error string `json:"error,omitempty"`
}

// =============================================================================
// Actions
// =============================================================================

// ActionKind drives the client's affordance, not its behaviour.
type ActionKind string

const (
	ActionPrimary   ActionKind = "primary"
	ActionSecondary ActionKind = "secondary"
	ActionDanger    ActionKind = "danger"
)

// Action is something the caller may do to this entity.
//
// Only ALLOWED actions are returned. A disabled button that exists to advertise
// a permission the user does not have is exactly the "button that lies" the
// project forbids; if the caller cannot do it, they are not told it is there.
// Method and Path name the real endpoint that performs it, which is what keeps
// this list honest: an action with no route cannot be expressed.
type Action struct {
	Key        string     `json:"key"`
	Label      string     `json:"label"`
	Kind       ActionKind `json:"kind"`
	Method     string     `json:"method"`
	Path       string     `json:"path"`
	Permission string     `json:"permission"`
}

// =============================================================================
// Timeline
// =============================================================================

// TimelineSource names where an event came from. It is carried to the client so
// a reader can tell an HTTP-audited change from one a background worker made,
// and so the drawer can say which journal it is quoting.
type TimelineSource string

const (
	// SourceAudit is the canonical audit trail (audit_events) — every mutating
	// HTTP request, hash-chained, tenant-scoped.
	SourceAudit TimelineSource = "audit"
	// SourceRiskHistory is risk_histories — score and status changes, including
	// the ones the score worker makes outside any request.
	SourceRiskHistory TimelineSource = "risk_history"
	// SourceIncident is an incident's own journal.
	SourceIncident TimelineSource = "incident_timeline"
	// SourceAssetSnapshot is asset_snapshots.
	SourceAssetSnapshot TimelineSource = "asset_snapshot"
)

// Change is one field that moved.
type Change struct {
	Field string `json:"field"`
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
}

// TimelineEvent is one thing that happened, in the user's language.
//
// This is deliberately NOT an audit record (§23). It carries a sentence and at
// most a short list of changed fields; it does not carry before/after snapshots,
// which may hold data the reader of a timeline has no business seeing. The audit
// endpoint carries those, behind its own permission.
type TimelineEvent struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	OccurredAt time.Time      `json:"occurred_at"`
	Actor      *Actor         `json:"actor,omitempty"`
	Target     Ref            `json:"target"`
	Summary    string         `json:"summary"`
	Changes    []Change       `json:"changes,omitempty"`
	Source     TimelineSource `json:"source"`
	// TargetURL deep-links the event's subject — this is what makes the tenant
	// timeline navigable (§36).
	TargetURL string `json:"target_url,omitempty"`
}

// TimelinePage is one cursor-paginated slice, newest first.
//
// Newest-first is the ordering everywhere in this product (the audit trail, the
// risk timeline and the incident journal all order created_at DESC), and §20
// asks for one rule stated once. NextCursor is empty when the caller has reached
// the end.
type TimelinePage struct {
	Events     []TimelineEvent `json:"events"`
	NextCursor string          `json:"next_cursor,omitempty"`
	// Sources lists which journals contributed, so a reader can see that (say)
	// the risk history was consulted and simply had nothing to add.
	Sources []TimelineSource `json:"sources"`
}

// TimelineFilter narrows a timeline query. Zero-value fields are ignored.
type TimelineFilter struct {
	// Kind filters on the event verb (create/update/delete/...).
	Kind string
	// ActorID filters to one actor.
	ActorID *uuid.UUID
	// Since / Until bound the window.
	Since *time.Time
	Until *time.Time
	Limit int
}
