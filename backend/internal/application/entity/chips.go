// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entity

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Tone vocabulary. These strings are the design system's badge intents, and this
// file is the only place a domain value is translated into one — so a critical
// risk and a critical vulnerability are the same red everywhere, and a new
// status cannot pick up a colour by accident somewhere in the client.
const (
	toneCritical = "critical"
	toneHigh     = "high"
	toneMedium   = "medium"
	toneLow      = "low"
	toneSuccess  = "success"
	toneWarning  = "warning"
	toneInfo     = "info"
	toneNeutral  = "neutral"
)

// severityChip maps any of the product's severity/criticality vocabularies onto
// one scale. The vocabularies genuinely differ in case (assets shout CRITICAL,
// vulnerabilities whisper critical), which is why this normalises rather than
// switching on the typed enums.
func severityChip(raw string) *Chip {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return nil
	}
	label, tone := v, toneNeutral
	switch v {
	case "critical":
		label, tone = "Critical", toneCritical
	case "high":
		label, tone = "High", toneHigh
	case "medium":
		label, tone = "Medium", toneMedium
	case "low":
		label, tone = "Low", toneLow
	case "info", "informational":
		label, tone = "Info", toneInfo
	default:
		label = title(v)
	}
	return &Chip{Value: v, Label: label, Tone: tone}
}

// riskStatusChip colours a risk's status.
//
// Risk carries two status vocabularies that never got unified — the lowercase
// open/in_progress/… and the uppercase DRAFT/ACTIVE/… — and a switch with no
// default is how a whole page once went blank on an unexpected value. Anything
// unrecognised falls through to a neutral chip that still shows the raw value:
// the reader learns what the system thinks, and nothing crashes.
func riskStatusChip(raw string) *Chip {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return nil
	}
	switch v {
	case "open":
		return &Chip{Value: v, Label: "Open", Tone: toneWarning}
	case "in_progress", "in-progress", "active":
		return &Chip{Value: v, Label: "In progress", Tone: toneInfo}
	case "mitigated":
		return &Chip{Value: v, Label: "Mitigated", Tone: toneSuccess}
	case "accepted":
		return &Chip{Value: v, Label: "Accepted", Tone: toneNeutral}
	case "closed":
		return &Chip{Value: v, Label: "Closed", Tone: toneNeutral}
	case "draft":
		return &Chip{Value: v, Label: "Draft", Tone: toneNeutral}
	default:
		return &Chip{Value: v, Label: title(v), Tone: toneNeutral}
	}
}

// controlStatusChip colours a compliance control's implementation status.
func controlStatusChip(raw string) *Chip {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return nil
	}
	switch v {
	case "implemented":
		return &Chip{Value: v, Label: "Implemented", Tone: toneSuccess}
	case "in_progress":
		return &Chip{Value: v, Label: "In progress", Tone: toneInfo}
	case "not_implemented":
		return &Chip{Value: v, Label: "Not implemented", Tone: toneHigh}
	case "not_applicable":
		return &Chip{Value: v, Label: "Not applicable", Tone: toneNeutral}
	default:
		return &Chip{Value: v, Label: title(v), Tone: toneNeutral}
	}
}

// vulnStatusChip colours a vulnerability's remediation status.
func vulnStatusChip(raw string) *Chip {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return nil
	}
	switch v {
	case "open":
		return &Chip{Value: v, Label: "Open", Tone: toneHigh}
	case "triaged":
		return &Chip{Value: v, Label: "Triaged", Tone: toneWarning}
	case "in_remediation":
		return &Chip{Value: v, Label: "In remediation", Tone: toneInfo}
	case "remediated":
		return &Chip{Value: v, Label: "Remediated", Tone: toneSuccess}
	case "accepted":
		return &Chip{Value: v, Label: "Risk accepted", Tone: toneNeutral}
	case "false_positive":
		return &Chip{Value: v, Label: "False positive", Tone: toneNeutral}
	default:
		return &Chip{Value: v, Label: title(v), Tone: toneNeutral}
	}
}

// incidentStatusChip colours an incident's lifecycle status.
func incidentStatusChip(raw string) *Chip {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return nil
	}
	switch v {
	case "open":
		return &Chip{Value: v, Label: "Open", Tone: toneCritical}
	case "investigating":
		return &Chip{Value: v, Label: "Investigating", Tone: toneWarning}
	case "resolved":
		return &Chip{Value: v, Label: "Resolved", Tone: toneSuccess}
	case "closed":
		return &Chip{Value: v, Label: "Closed", Tone: toneNeutral}
	default:
		return &Chip{Value: v, Label: title(v), Tone: toneNeutral}
	}
}

// evidenceStatusChip colours an artifact's effective freshness/review state.
func evidenceStatusChip(raw string) *Chip {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return nil
	}
	switch v {
	case "valid":
		return &Chip{Value: v, Label: "Valid", Tone: toneSuccess}
	case "expiring_soon":
		return &Chip{Value: v, Label: "Expiring soon", Tone: toneWarning}
	case "expired":
		return &Chip{Value: v, Label: "Expired", Tone: toneHigh}
	case "rejected":
		return &Chip{Value: v, Label: "Rejected", Tone: toneCritical}
	case "pending":
		return &Chip{Value: v, Label: "Pending review", Tone: toneInfo}
	default:
		return &Chip{Value: v, Label: title(v), Tone: toneNeutral}
	}
}

// mitigationStatusChip colours a mitigation plan's status.
func mitigationStatusChip(raw string) *Chip {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return nil
	}
	switch v {
	case "done", "completed":
		return &Chip{Value: v, Label: "Done", Tone: toneSuccess}
	case "in_progress":
		return &Chip{Value: v, Label: "In progress", Tone: toneInfo}
	case "review":
		return &Chip{Value: v, Label: "In review", Tone: toneWarning}
	case "planned":
		return &Chip{Value: v, Label: "Planned", Tone: toneNeutral}
	default:
		return &Chip{Value: v, Label: title(v), Tone: toneNeutral}
	}
}

// scoreTone bands a 0..max value. Used only for colouring a score the owning
// module computed — never to compute one (§13).
func scoreTone(value, max float64) string {
	if max <= 0 {
		return toneNeutral
	}
	switch ratio := value / max; {
	case ratio >= 0.75:
		return toneCritical
	case ratio >= 0.5:
		return toneHigh
	case ratio >= 0.25:
		return toneMedium
	default:
		return toneLow
	}
}

// unavailableScore is the honest answer when an entity has no score of that kind
// — never a zero (§13).
func unavailableScore(key, label, why string) Score {
	return Score{Available: false, Key: key, Label: label, Unavailable: why}
}

// --- small helpers ---------------------------------------------------------

func title(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	c := t
	return &c
}

// actorFor builds a display actor from an optional id. A nil or zero id means
// "no one recorded", which the client renders as such rather than as a blank.
func actorFor(id *uuid.UUID) *Actor {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	return &Actor{ID: id.String()}
}

// field is the small constructor the resolvers use; it drops empty values so a
// summary never shows a labelled blank.
func field(key, label, value string, kind FieldKind) *Field {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &Field{Key: key, Label: label, Value: value, Kind: kind}
}

// appendField adds a field when it has a value.
func appendField(dst []Field, f *Field) []Field {
	if f == nil {
		return dst
	}
	return append(dst, *f)
}
