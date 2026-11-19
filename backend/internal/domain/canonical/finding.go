// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical

import (
	"encoding/json"
	"fmt"
	"time"
)

// FindingKind is what a finding is (D6, closed). Other kinds are added by
// amendment when a module records them.
type FindingKind string

// D6's finding kinds.
const (
	FindingVulnerability FindingKind = "vulnerability"
	FindingControlGap    FindingKind = "control_gap"
)

// Severity is today's domain.VulnSeverity, spelled identically (D6).
type Severity string

// D6's severities.
const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// FindingStatus is today's domain.VulnStatus, spelled identically (D6).
type FindingStatus string

// D6's finding statuses. false_positive sets confidence to 0.00 (D3).
const (
	FindingOpen          FindingStatus = "open"
	FindingTriaged       FindingStatus = "triaged"
	FindingInRemediation FindingStatus = "in_remediation"
	FindingRemediated    FindingStatus = "remediated"
	FindingAccepted      FindingStatus = "accepted"
	FindingFalsePositive FindingStatus = "false_positive"
)

// IdentifierScheme is a finding identifier's scheme (D6, closed).
type IdentifierScheme string

// D6's identifier schemes.
const (
	SchemeCVE            IdentifierScheme = "cve"
	SchemeCWE            IdentifierScheme = "cwe"
	SchemeVendorAdvisory IdentifierScheme = "vendor_advisory"
)

// FindingIdentifier is one `{scheme, value}` of `identifiers`.
type FindingIdentifier struct {
	Scheme IdentifierScheme `json:"scheme"`
	Value  string           `json:"value"`
}

// CVSS is the technical severity as the source scored it. Version is parsed
// from the vector prefix and null when absent (D7).
type CVSS struct {
	Score   float64 `json:"score"`
	Vector  string  `json:"vector"`
	Version *string `json:"version"`
}

// ExploitMaturity is `exploitability.exploit_maturity` (D6, closed).
type ExploitMaturity string

// D6's exploit maturities.
const (
	MaturityNone       ExploitMaturity = "none"
	MaturityPoC        ExploitMaturity = "poc"
	MaturityFunctional ExploitMaturity = "functional"
	MaturityHigh       ExploitMaturity = "high"
)

// Exploitability is what makes a severe finding urgent. ExploitMaturity is a
// pointer because today's column may be empty, and an empty value is not one
// of the four.
type Exploitability struct {
	EPSS             float64          `json:"epss"`
	KEV              bool             `json:"kev"`
	ExploitAvailable bool             `json:"exploit_available"`
	ExploitMaturity  *ExploitMaturity `json:"exploit_maturity"`
}

// FindingPriority is `priority`: `{score, tier, version}` from pkg/vulnprio,
// read-only like Risk.score.
type FindingPriority struct {
	Score   float64 `json:"score"`
	Tier    string  `json:"tier"`
	Version string  `json:"version"`
}

// Finding is an observed fact that something is wrong (ADR 0008 D6).
// observed_at is required and is the most recent observation; first_observed_at
// is the first (D4).
//
// priority is read-only, like Risk.score: unexported, readable through
// Priority, never decoded from JSON, set only by WithVulnPriority.
type Finding struct {
	Envelope
	Kind            FindingKind             `json:"kind"`
	Title           string                  `json:"title"`
	Description     *string                 `json:"description"`
	Severity        Severity                `json:"severity"`
	Status          FindingStatus           `json:"status"`
	Identifiers     List[FindingIdentifier] `json:"identifiers"`
	CVSS            *CVSS                   `json:"cvss"`
	Exploitability  *Exploitability         `json:"exploitability"`
	priority        *FindingPriority        // read-only, see WithVulnPriority
	FirstObservedAt time.Time               `json:"first_observed_at"`
	SLADueAt        *time.Time              `json:"sla_due_at"`
	RemediatedAt    *time.Time              `json:"remediated_at"`
	RemediationHint *string                 `json:"remediation_hint"`
}

// WithVulnPriority is the single permitted entry point for Finding.priority,
// for the read projection only (ADR 0008 D6, amendment A4). It returns a copy
// of f carrying the priority pkg/vulnprio computed and persistence stored: it
// copies, it never computes, and nothing that writes a canonical Finding may
// call it.
func WithVulnPriority(f Finding, p FindingPriority) Finding {
	c := p
	f.priority = &c
	return f
}

// Priority returns the priority the record carries, and false when it has none.
func (f Finding) Priority() (FindingPriority, bool) {
	if f.priority == nil {
		return FindingPriority{}, false
	}
	return *f.priority, true
}

// MarshalJSON publishes priority beside the other fields; a decoded Finding
// never carries one.
func (f Finding) MarshalJSON() ([]byte, error) {
	type plain Finding
	return json.Marshal(struct {
		plain
		Priority *FindingPriority `json:"priority"`
	}{plain(f), f.priority})
}

// Validate applies D2, D3, D4 and D6 to a Finding.
func (f Finding) Validate() error {
	if _, err := f.Envelope.validate(TypeFinding, observedRequired, ReviewContext{}); err != nil {
		return err
	}
	switch f.Kind {
	case FindingVulnerability, FindingControlGap:
	default:
		return invalid("kind", "must be vulnerability or control_gap")
	}
	if err := requireText("title", f.Title, 512); err != nil {
		return err
	}
	switch f.Severity {
	case SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo:
	default:
		return invalid("severity", "must be critical, high, medium, low or info")
	}
	switch f.Status {
	case FindingOpen, FindingTriaged, FindingInRemediation, FindingRemediated, FindingAccepted:
	case FindingFalsePositive:
		if f.Confidence != 0 {
			return invalid("confidence", "a false_positive finding is recorded and believed false: 0.00 (D3)")
		}
	default:
		return invalid("status", "must be open, triaged, in_remediation, remediated, accepted or false_positive")
	}
	for i, id := range f.Identifiers {
		field := fmt.Sprintf("identifiers[%d]", i)
		switch id.Scheme {
		case SchemeCVE, SchemeCWE, SchemeVendorAdvisory:
		default:
			return invalid(field+".scheme", "must be cve, cwe or vendor_advisory")
		}
		if blank(id.Value) {
			return invalid(field+".value", "is required")
		}
	}
	if c := f.CVSS; c != nil {
		if err := inRange("cvss.score", c.Score, 0, 10); err != nil {
			return err
		}
		if err := maxLen("cvss.vector", c.Vector, 128); err != nil {
			return err
		}
	}
	if x := f.Exploitability; x != nil {
		if err := inRange("exploitability.epss", x.EPSS, 0, 1); err != nil {
			return err
		}
		if m := x.ExploitMaturity; m != nil {
			switch *m {
			case MaturityNone, MaturityPoC, MaturityFunctional, MaturityHigh:
			default:
				return invalid("exploitability.exploit_maturity", "must be none, poc, functional or high")
			}
		}
	}
	if f.priority != nil {
		if err := inRange("priority.score", f.priority.Score, 0, 100); err != nil {
			return err
		}
	}
	if err := requireTime("first_observed_at", f.FirstObservedAt); err != nil {
		return err
	}
	if f.FirstObservedAt.After(*f.ObservedAt) {
		return invalid("first_observed_at", "must not be after observed_at, the most recent observation (D4)")
	}
	if err := optionalTime("sla_due_at", f.SLADueAt); err != nil {
		return err
	}
	return optionalTime("remediated_at", f.RemediatedAt)
}
