// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Criticality is a score band (D6): `low · medium · high · critical`, as
// pkg/scoring's engine and SmartScore spell them. The canonical model carries
// the band; it never computes it.
type Criticality string

// D6's bands.
const (
	CriticalityLow      Criticality = "low"
	CriticalityMedium   Criticality = "medium"
	CriticalityHigh     Criticality = "high"
	CriticalityCritical Criticality = "critical"
)

func (c Criticality) valid() bool {
	switch c {
	case CriticalityLow, CriticalityMedium, CriticalityHigh, CriticalityCritical:
		return true
	}
	return false
}

// RiskState is today's domain.RiskState, spelled identically (D6).
type RiskState string

// The lifecycle states of internal/domain/risk_state.go.
const (
	StateDraft            RiskState = "draft"
	StateIdentified       RiskState = "identified"
	StateAssessed         RiskState = "assessed"
	StateTreatmentPlanned RiskState = "treatment_planned"
	StateInTreatment      RiskState = "in_treatment"
	StateResidualAccepted RiskState = "residual_accepted"
	StateMitigated        RiskState = "mitigated"
	StateClosed           RiskState = "closed"
	StateReopened         RiskState = "reopened"
)

var riskStates = map[RiskState]bool{
	StateDraft: true, StateIdentified: true, StateAssessed: true, StateTreatmentPlanned: true,
	StateInTreatment: true, StateResidualAccepted: true, StateMitigated: true, StateClosed: true,
	StateReopened: true,
}

// Treatment is the ISO 27005 treatment option (D6, closed).
type Treatment string

// D6's treatments, spelled as domain.RiskTreatment spells them.
const (
	TreatmentAccept   Treatment = "accept"
	TreatmentMitigate Treatment = "mitigate"
	TreatmentTransfer Treatment = "transfer"
	TreatmentAvoid    Treatment = "avoid"
)

// RiskScore is `score`: `{value, criticality}`, as computed by
// pkg/scoring/engine.go. No engine version is carried: none is persisted and
// the formula is frozen (D6).
type RiskScore struct {
	Value       float64     `json:"value"`
	Criticality Criticality `json:"criticality"`
}

// Residual is ADR 0003's residual, `residual-v1`.
type Residual struct {
	Value          float64     `json:"value"`
	Band           Criticality `json:"band"`
	FormulaVersion string      `json:"formula_version"`
}

// SmartFactor is one line of a SmartScore breakdown, as pkg/scoring's
// FactorScore carries it.
type SmartFactor struct {
	Key          string  `json:"key"`
	Label        string  `json:"label"`
	Weight       float64 `json:"weight"`
	Value        float64 `json:"value"`
	Contribution float64 `json:"contribution"`
	Detail       string  `json:"detail"`
}

// SmartScore is `smart_score`: SmartScore from pkg/scoring/smart.go, additive,
// read-only like `score`.
type SmartScore struct {
	Value      float64           `json:"value"`
	Level      Criticality       `json:"level"`
	Factors    List[SmartFactor] `json:"factors"`
	ComputedAt time.Time         `json:"computed_at"`
}

// Acceptance is present only in or after residual_accepted (#811).
type Acceptance struct {
	AcceptedAt *time.Time `json:"accepted_at"`
	ApprovalID *uuid.UUID `json:"approval_id"`
	ApprovedBy *uuid.UUID `json:"approved_by"`
	ExpiresAt  *time.Time `json:"expires_at"`
	ExpiredAt  *time.Time `json:"expired_at"`
}

// ReviewCadence is a risk's review reminders.
type ReviewCadence struct {
	IntervalDays   int        `json:"interval_days"`
	NextReviewAt   *time.Time `json:"next_review_at"`
	LastReviewedAt *time.Time `json:"last_reviewed_at"`
}

// Category is the tenant's controlled taxonomy entry. RiskCategory is a tenant
// vocabulary, not a canonical entity (D6).
type Category struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Quantification is the CRQ inputs. Amounts carry an ISO 4217 currency, XAF
// for every value stored today; conversion for display is the presenter's job.
type Quantification struct {
	Currency                string   `json:"currency"`
	SLE                     *float64 `json:"sle"`
	ARO                     *float64 `json:"aro"`
	DowntimeHours           *float64 `json:"downtime_hours"`
	HourlyDowntimeCost      *float64 `json:"hourly_downtime_cost"`
	DataLossCost            *float64 `json:"data_loss_cost"`
	Fines                   *float64 `json:"fines"`
	OtherDirectCost         *float64 `json:"other_direct_cost"`
	RemediationCost         *float64 `json:"remediation_cost"`
	MitigationEffectiveness *float64 `json:"mitigation_effectiveness"`
}

// Risk is a business risk with its frozen score (ADR 0008 D6). observed_at is
// the last assessment date, nullable (D4).
//
// score and smart_score are read-only in the contract: they are unexported,
// readable through Score and SmartScore, never decoded from JSON, and set in
// exactly one place, WithEngineScores, which the read projection calls with
// what pkg/scoring already computed. Nothing in this package computes them
// (TestCanonical_RiskScoreIsReadOnly).
type Risk struct {
	Envelope
	Name           string                     `json:"name"`
	Description    *string                    `json:"description"`
	Probability    float64                    `json:"probability"`
	Impact         float64                    `json:"impact"`
	score          *RiskScore                 // read-only, see WithEngineScores
	Residual       *Residual                  `json:"residual"`
	smartScore     *SmartScore                // read-only, see WithEngineScores
	LifecycleState RiskState                  `json:"lifecycle_state"`
	Treatment      Treatment                  `json:"treatment"`
	Acceptance     *Acceptance                `json:"acceptance"`
	ReviewCadence  *ReviewCadence             `json:"review_cadence"`
	Category       *Category                  `json:"category"`
	Tags           List[string]               `json:"tags"`
	Quantification *Quantification            `json:"quantification"`
	CustomFields   map[string]json.RawMessage `json:"custom_fields"`
	Ownership
	CreatedBy uuid.UUID `json:"created_by"`
}

// WithEngineScores is the single permitted entry point for Risk.score and
// Risk.smart_score, for the read projection only (ADR 0008 D6, amendment A4).
// It returns a copy of r carrying the score and SmartScore that pkg/scoring
// computed and persistence stored: it copies, it never computes, and nothing
// that writes a canonical Risk may call it.
func WithEngineScores(r Risk, score RiskScore, smart *SmartScore) Risk {
	s := score
	r.score = &s
	r.smartScore = nil
	if smart != nil {
		c := *smart
		c.Factors = append([]SmartFactor(nil), smart.Factors...)
		r.smartScore = &c
	}
	return r
}

// Score returns the engine score the record carries, and false when the
// projection attached none.
func (r Risk) Score() (RiskScore, bool) {
	if r.score == nil {
		return RiskScore{}, false
	}
	return *r.score, true
}

// SmartScore returns a copy of the SmartScore the record carries, and false
// when it has none.
func (r Risk) SmartScore() (SmartScore, bool) {
	if r.smartScore == nil {
		return SmartScore{}, false
	}
	c := *r.smartScore
	c.Factors = append([]SmartFactor(nil), r.smartScore.Factors...)
	return c, true
}

// MarshalJSON publishes score and smart_score beside the other fields. There is
// no UnmarshalJSON counterpart on purpose: a decoded Risk never carries a
// score from its input, because the fields are unexported.
func (r Risk) MarshalJSON() ([]byte, error) {
	type plain Risk
	return json.Marshal(struct {
		plain
		Score      *RiskScore  `json:"score"`
		SmartScore *SmartScore `json:"smart_score"`
	}{plain(r), r.score, r.smartScore})
}

// Validate applies D2, D3, D4 and D6 to a Risk.
//
// score is validated when present but not required: a Risk being written has
// no score of its own (persistence obtains it from the engine), and a decoded
// Risk never carries one. The read projection always attaches it.
func (r Risk) Validate() error {
	if _, err := r.Envelope.validate(TypeRisk, observedOptional, ReviewContext{}); err != nil {
		return err
	}
	if err := requireText("name", r.Name, 255); err != nil {
		return err
	}
	if err := inRange("probability", r.Probability, 0, 1); err != nil {
		return err
	}
	if err := inRange("impact", r.Impact, 0, 10); err != nil {
		return err
	}
	if r.score != nil {
		if err := nonNegative("score.value", &r.score.Value); err != nil {
			return err
		}
		if !r.score.Criticality.valid() {
			return invalid("score.criticality", "must be low, medium, high or critical")
		}
	}
	if r.Residual != nil {
		if err := nonNegative("residual.value", &r.Residual.Value); err != nil {
			return err
		}
		if !r.Residual.Band.valid() {
			return invalid("residual.band", "must be low, medium, high or critical")
		}
		if err := requireText("residual.formula_version", r.Residual.FormulaVersion, 0); err != nil {
			return err
		}
	}
	if r.smartScore != nil {
		if err := r.smartScore.validate(); err != nil {
			return err
		}
	}
	if !riskStates[r.LifecycleState] {
		return invalid("lifecycle_state", "must be one of today's RiskState values")
	}
	switch r.Treatment {
	case TreatmentAccept, TreatmentMitigate, TreatmentTransfer, TreatmentAvoid:
	default:
		return invalid("treatment", "must be accept, mitigate, transfer or avoid")
	}
	if a := r.Acceptance; a != nil {
		if err := optionalTime("acceptance.accepted_at", a.AcceptedAt); err != nil {
			return err
		}
		if err := optionalTime("acceptance.expires_at", a.ExpiresAt); err != nil {
			return err
		}
		if err := optionalTime("acceptance.expired_at", a.ExpiredAt); err != nil {
			return err
		}
		if err := optionalID("acceptance.approval_id", a.ApprovalID); err != nil {
			return err
		}
		if err := optionalID("acceptance.approved_by", a.ApprovedBy); err != nil {
			return err
		}
	}
	if rc := r.ReviewCadence; rc != nil && rc.IntervalDays < 0 {
		return invalid("review_cadence.interval_days", "must not be negative")
	}
	if c := r.Category; c != nil {
		if err := requireID("category.id", c.ID); err != nil {
			return err
		}
	}
	if q := r.Quantification; q != nil {
		if err := q.validate(); err != nil {
			return err
		}
	}
	if err := r.Ownership.validate(); err != nil {
		return err
	}
	return requireID("created_by", r.CreatedBy)
}

func (s SmartScore) validate() error {
	if err := inRange("smart_score.value", s.Value, 0, 100); err != nil {
		return err
	}
	if !s.Level.valid() {
		return invalid("smart_score.level", "must be low, medium, high or critical")
	}
	for i, f := range s.Factors {
		if blank(f.Key) {
			return invalid(fmt.Sprintf("smart_score.factors[%d].key", i), "is required")
		}
	}
	return requireTime("smart_score.computed_at", s.ComputedAt)
}

func (q Quantification) validate() error {
	if len(q.Currency) != 3 || q.Currency[0] < 'A' || q.Currency[0] > 'Z' ||
		q.Currency[1] < 'A' || q.Currency[1] > 'Z' || q.Currency[2] < 'A' || q.Currency[2] > 'Z' {
		return invalid("quantification.currency", "must be an ISO 4217 code such as XAF")
	}
	for _, amount := range []struct {
		field string
		value *float64
	}{
		{"quantification.sle", q.SLE}, {"quantification.aro", q.ARO},
		{"quantification.downtime_hours", q.DowntimeHours},
		{"quantification.hourly_downtime_cost", q.HourlyDowntimeCost},
		{"quantification.data_loss_cost", q.DataLossCost}, {"quantification.fines", q.Fines},
		{"quantification.other_direct_cost", q.OtherDirectCost},
		{"quantification.remediation_cost", q.RemediationCost},
	} {
		if err := nonNegative(amount.field, amount.value); err != nil {
			return err
		}
	}
	return optionalRange("quantification.mitigation_effectiveness", q.MitigationEffectiveness, 0, 1)
}
