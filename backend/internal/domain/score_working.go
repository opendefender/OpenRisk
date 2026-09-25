// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// ScoreWorking (#486) — a risk score with its working shown.
//
// The design guide's promise is that a person can check the arithmetic. That
// takes three things, and this read model carries all three:
//
//  1. the terms of the frozen formula and the result they produce;
//  2. for each term, the audit entry that last set it — who, when, and the
//     entry's position and hash in the tenant's chain;
//  3. whether each cited entry still hashes to what was sealed, so the reader
//     knows the record behind the number has not been edited since.
//
// Nothing is estimated. A term with no recorded origin (a risk created before
// #486 journalled risk fields) says so with a nil Source; it is never given a
// plausible-looking one.
// ---------------------------------------------------------------------------

// ScoreWorkingFormula names the frozen Score Engine formula (CLAUDE.md).
const ScoreWorkingFormula = "probability × impact × asset_criticality"

// ScoreWorkingSource is one audit entry cited as the origin of a value.
type ScoreWorkingSource struct {
	EventID    string      `json:"event_id"`
	Sequence   int64       `json:"sequence"`
	Hash       string      `json:"hash"`
	PrevHash   string      `json:"prev_hash"`
	HashValid  bool        `json:"hash_valid"`
	Action     AuditAction `json:"action"`
	Source     string      `json:"source,omitempty"`
	ActorID    *uuid.UUID  `json:"actor_id,omitempty"`
	ActorEmail string      `json:"actor_email,omitempty"`
	ActorType  string      `json:"actor_type,omitempty"`
	ActorLabel string      `json:"actor_label,omitempty"`
	At         time.Time   `json:"at"`
	Summary    string      `json:"summary,omitempty"`
	// Value is what the entry set the field to. When it differs from the
	// current value, something changed the field without leaving an entry.
	Value interface{} `json:"value"`
}

// ScoreWorkingTerm is one factor of the formula.
type ScoreWorkingTerm struct {
	Key    string              `json:"key"` // probability | impact | asset_criticality
	Value  float64             `json:"value"`
	Min    float64             `json:"min"`
	Max    float64             `json:"max"`
	Source *ScoreWorkingSource `json:"source"`
}

// ScoreWorkingAsset is one linked asset contributing to asset_criticality.
type ScoreWorkingAsset struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Criticality string              `json:"criticality"`
	Factor      float64             `json:"factor"`
	Source      *ScoreWorkingSource `json:"source"`
}

// ScoreWorking is the whole explanation of one risk's score.
type ScoreWorking struct {
	RiskID  string             `json:"risk_id"`
	Formula string             `json:"formula"`
	Terms   []ScoreWorkingTerm `json:"terms"`
	// Assets are averaged into asset_criticality. Empty with
	// AssetCriticalityDefaulted=true means no asset is linked and the engine's
	// documented default (medium) applied.
	Assets                    []ScoreWorkingAsset `json:"assets"`
	AssetCriticalityDefaulted bool                `json:"asset_criticality_defaulted"`

	// Computed is what the Score Engine returns for the terms above, now.
	Computed    float64 `json:"computed"`
	Criticality string  `json:"criticality"`
	Explanation string  `json:"explanation"`
	// Stored is the score the register shows. Consistent says whether it
	// equals Computed at the stored precision; when it does not, the reader is
	// told rather than shown a number that cannot be re-derived.
	Stored            float64 `json:"stored"`
	StoredCriticality string  `json:"stored_criticality"`
	Consistent        bool    `json:"consistent"`

	// ScoreSource is the entry that last wrote the stored score.
	ScoreSource *ScoreWorkingSource `json:"score_source"`
	// SourcesVisible is false when the caller may not read the audit trail:
	// the arithmetic is still shown, the provenance is withheld, and the
	// reader is told it was withheld rather than that none exists.
	SourcesVisible bool `json:"sources_visible"`
}

// ScoreWorkingSourceOf projects an audit entry into a citation of `field`.
func ScoreWorkingSourceOf(e *AuditEvent, field string) *ScoreWorkingSource {
	if e == nil {
		return nil
	}
	var v interface{}
	if e.After != nil {
		v = e.After[field]
	}
	return &ScoreWorkingSource{
		EventID:    e.ID.String(),
		Sequence:   e.Sequence,
		Hash:       e.Hash,
		PrevHash:   e.PrevHash,
		HashValid:  e.VerifyHash(),
		Action:     e.Action,
		Source:     e.Source,
		ActorID:    e.ActorID,
		ActorEmail: e.ActorEmail,
		ActorType:  e.ActorType,
		ActorLabel: e.ActorLabel,
		At:         e.CreatedAt,
		Summary:    e.Summary,
		Value:      v,
	}
}

// LatestFieldSource returns the most recent entry that set `field`: a create
// whose after-image carries it, or any entry that lists it as changed. events
// must be ordered newest first, as the audit repository returns them. Nil when
// no entry records the field — which is an answer, not an error.
func LatestFieldSource(events []AuditEvent, field string) *AuditEvent {
	for i := range events {
		e := &events[i]
		if e.After == nil {
			continue
		}
		if _, ok := e.After[field]; !ok {
			continue
		}
		if e.Action == AuditActionCreate {
			return e
		}
		for _, f := range e.ChangedFields {
			if f == field {
				return e
			}
		}
	}
	return nil
}

// ScoreEngineTerms is what the Score Engine multiplied, as it multiplied it.
type ScoreEngineTerms struct {
	Probability      float64
	Impact           float64
	AssetCriticality float64
	Score            float64
	Criticality      string
	Explanation      string
	// TriggeredBy is the event's triggered_by: a user id, or "system" when an
	// asset's criticality change fanned out to its risks.
	TriggeredBy string
}

// ScoreEngineAuditEvent builds the entry the Score Engine worker appends when
// it moves a risk's score. The engine is a job, and is attributed as one by
// name; the after-image carries every term so the entry alone lets a reader
// redo the multiplication.
func ScoreEngineAuditEvent(tenantID, riskID uuid.UUID, oldScore float64, t ScoreEngineTerms) *AuditEvent {
	after := JSONMap{
		"probability":       t.Probability,
		"impact":            t.Impact,
		"asset_criticality": t.AssetCriticality,
		"score":             t.Score,
		"criticality":       t.Criticality,
		"formula":           ScoreWorkingFormula,
	}
	if t.TriggeredBy != "" {
		after["triggered_by"] = t.TriggeredBy
	}
	return &AuditEvent{
		ID:            uuid.New(),
		TenantID:      tenantID,
		ActorType:     AuditActorJob,
		ActorLabel:    AuditJobScoreEngine,
		Action:        AuditActionUpdate,
		EntityType:    "risk",
		EntityID:      riskID.String(),
		Summary:       "score recomputed by the Score Engine: " + t.Explanation,
		Before:        JSONMap{"score": oldScore},
		After:         after,
		ChangedFields: StringList{"score"},
		Source:        AuditSourceExplicit,
	}
}
