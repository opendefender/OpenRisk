// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical

import (
	"math"
	"sort"

	"github.com/google/uuid"
)

// Source is a flat identifier from the source registry (ADR 0008 D3). Each
// identifier belongs to exactly one SourceClass; the class, not the identifier,
// fixes the confidence semantics.
type Source string

// The registry, exactly the identifiers of ADR 0008 D3. Adding one is an
// amendment of the ADR.
const (
	// user_entered
	SourceOpenRiskUser       Source = "openrisk_user"
	SourceOpenRiskImportFile Source = "openrisk_import_file"

	// automated_scanner
	SourceNessus          Source = "nessus"
	SourceOpenVAS         Source = "openvas"
	SourceQualys          Source = "qualys"
	SourceMSDefender      Source = "ms_defender"
	SourceAWSInspector    Source = "aws_inspector"
	SourceAzureDefender   Source = "azure_defender"
	SourceCrowdStrike     Source = "crowdstrike"
	SourceOpenRiskScanner Source = "openrisk_scanner"
	SourceOpenRiskAgent   Source = "openrisk_agent"

	// threat_intelligence
	SourceNVD         Source = "nvd"
	SourceCISAKEV     Source = "cisa_kev"
	SourceMITREATTACK Source = "mitre_attack"
	SourceOpenCTI     Source = "opencti"

	// connector_import ("future #543 contracts" join here by amendment)
	SourceJira       Source = "jira"
	SourceServiceNow Source = "servicenow"

	// third_party_attestation (ADR 0004, adopted by D8)
	SourceVendorQuestionnaire Source = "vendor_questionnaire"

	// regulatory_catalogue
	SourceOpenRiskCatalogue Source = "openrisk_catalogue"

	// derived
	SourceOpenRiskDerived Source = "openrisk_derived"
	SourceOpenRiskStarter Source = "openrisk_starter"
	SourceOpenRiskAI      Source = "openrisk_ai"
)

// SourceClass fixes what a record's confidence means.
type SourceClass string

// The seven source classes of ADR 0008 D3.
const (
	ClassUserEntered           SourceClass = "user_entered"
	ClassAutomatedScanner      SourceClass = "automated_scanner"
	ClassThreatIntelligence    SourceClass = "threat_intelligence"
	ClassConnectorImport       SourceClass = "connector_import"
	ClassThirdPartyAttestation SourceClass = "third_party_attestation"
	ClassRegulatoryCatalogue   SourceClass = "regulatory_catalogue"
	ClassDerived               SourceClass = "derived"
)

// ConfidenceRule is the version of the confidence rule this package applies,
// written into provenance.confidence_rule. A rule change is a new version,
// never a silent recomputation (ADR 0008 D3).
const ConfidenceRule = "confidence/1"

// knownConfidenceRules is the closed set provenance.confidence_rule may name.
var knownConfidenceRules = map[string]bool{ConfidenceRule: true}

var registry = map[Source]SourceClass{
	SourceOpenRiskUser:        ClassUserEntered,
	SourceOpenRiskImportFile:  ClassUserEntered,
	SourceNessus:              ClassAutomatedScanner,
	SourceOpenVAS:             ClassAutomatedScanner,
	SourceQualys:              ClassAutomatedScanner,
	SourceMSDefender:          ClassAutomatedScanner,
	SourceAWSInspector:        ClassAutomatedScanner,
	SourceAzureDefender:       ClassAutomatedScanner,
	SourceCrowdStrike:         ClassAutomatedScanner,
	SourceOpenRiskScanner:     ClassAutomatedScanner,
	SourceOpenRiskAgent:       ClassAutomatedScanner,
	SourceNVD:                 ClassThreatIntelligence,
	SourceCISAKEV:             ClassThreatIntelligence,
	SourceMITREATTACK:         ClassThreatIntelligence,
	SourceOpenCTI:             ClassThreatIntelligence,
	SourceJira:                ClassConnectorImport,
	SourceServiceNow:          ClassConnectorImport,
	SourceVendorQuestionnaire: ClassThirdPartyAttestation,
	SourceOpenRiskCatalogue:   ClassRegulatoryCatalogue,
	SourceOpenRiskDerived:     ClassDerived,
	SourceOpenRiskStarter:     ClassDerived,
	SourceOpenRiskAI:          ClassDerived,
}

// Class returns the source's class, and false for an identifier outside the
// registry.
func (s Source) Class() (SourceClass, bool) {
	c, ok := registry[s]
	return c, ok
}

// Sources lists every registry identifier, sorted, for #542 to publish.
func Sources() []Source {
	out := make([]Source, 0, len(registry))
	for s := range registry {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// SourceClasses lists the seven classes in ADR order.
func SourceClasses() []SourceClass {
	return []SourceClass{
		ClassUserEntered, ClassAutomatedScanner, ClassThreatIntelligence,
		ClassConnectorImport, ClassThirdPartyAttestation, ClassRegulatoryCatalogue,
		ClassDerived,
	}
}

// classRule is a class's default and ceiling under confidence/1.
//
// The ceilings, read from ADR 0008 D3's "Rises to" column:
//   - user_entered: 0.70, or 0.90 with a qualifying second-person review
//     (ReviewContext.QualifiesForUplift). The ADR's test pins "> 0.90 refused".
//   - automated_scanner, threat_intelligence, regulatory_catalogue: the ADR
//     gives no "rises to", so the ceiling is the default (0.80, 0.40, 0.90).
//   - connector_import: the source's own value, "capped at 0.80".
//   - third_party_attestation: 0.50 is "a cap", nothing raises it in v1.
//   - derived: "the minimum of its inputs", which a pure validator cannot see.
//     The ceiling is therefore the highest ceiling of any non-derived class,
//     0.90: a minimum of inputs can never exceed it, and openrisk_ai "may not
//     raise it above user_entered". No default exists for this class.
//
// No default and no ceiling reaches 1.00, which is reserved (D3).
type classRule struct {
	def        float64
	hasDefault bool
	ceiling    float64
}

const userEnteredReviewedCeiling = 0.90

var classRules = map[SourceClass]classRule{
	ClassUserEntered:           {def: 0.70, hasDefault: true, ceiling: 0.70},
	ClassAutomatedScanner:      {def: 0.80, hasDefault: true, ceiling: 0.80},
	ClassThreatIntelligence:    {def: 0.40, hasDefault: true, ceiling: 0.40},
	ClassConnectorImport:       {def: 0.60, hasDefault: true, ceiling: 0.80},
	ClassThirdPartyAttestation: {def: 0.50, hasDefault: true, ceiling: 0.50},
	ClassRegulatoryCatalogue:   {def: 0.90, hasDefault: true, ceiling: 0.90},
	ClassDerived:               {hasDefault: false, ceiling: 0.90},
}

// ReviewContext is what the user_entered uplift reads (ADR 0008 D3): a review
// verdict, who reviewed, who collected and who stored. The zero value never
// qualifies, which is the answer for every entity that carries no review.
type ReviewContext struct {
	Verdict     ReviewVerdict
	ReviewedBy  *uuid.UUID
	CollectedBy *uuid.UUID
	// ActorID is provenance.actor.id. Envelope validation fills it.
	ActorID string
}

// QualifiesForUplift is D3's rule, all four conditions at once: the verdict is
// accepted, a reviewer is set, and the reviewer is neither the collector nor
// the actor who stored the record. Today's default "accepted with no reviewer"
// does not qualify.
func (rc ReviewContext) QualifiesForUplift() bool {
	if rc.Verdict != ReviewAccepted || rc.ReviewedBy == nil || *rc.ReviewedBy == uuid.Nil {
		return false
	}
	if rc.CollectedBy != nil && *rc.CollectedBy == *rc.ReviewedBy {
		return false
	}
	return rc.ActorID != rc.ReviewedBy.String()
}

// DefaultConfidence is the confidence a use case writes for a record of this
// source under confidence/1. A user_entered record's confidence is computed
// from its review state, never supplied (D2). A derived record has no default:
// use DerivedConfidence over its inputs.
func DefaultConfidence(source Source, rc ReviewContext) (float64, error) {
	class, ok := source.Class()
	if !ok {
		return 0, invalid("source", "is not in the source registry")
	}
	rule := classRules[class]
	if !rule.hasDefault {
		return 0, invalid("source", "class %s has no default: its confidence is the minimum of its inputs", class)
	}
	if class == ClassUserEntered && rc.QualifiesForUplift() {
		return userEnteredReviewedCeiling, nil
	}
	return rule.def, nil
}

// ConfidenceCeiling is the highest confidence a record of this source may carry.
func ConfidenceCeiling(source Source, rc ReviewContext) (float64, error) {
	class, ok := source.Class()
	if !ok {
		return 0, invalid("source", "is not in the source registry")
	}
	if class == ClassUserEntered && rc.QualifiesForUplift() {
		return userEnteredReviewedCeiling, nil
	}
	return classRules[class].ceiling, nil
}

// DerivedConfidence is the derived class's rule: never surer than its weakest
// input. It refuses an empty input list and any input outside [0, 1].
func DerivedConfidence(inputs ...float64) (float64, error) {
	if len(inputs) == 0 {
		return 0, invalid("derived_from", "a derived confidence needs at least one input")
	}
	lowest := math.Inf(1)
	for _, c := range inputs {
		if err := inRange("confidence", c, 0, 1); err != nil {
			return 0, err
		}
		lowest = math.Min(lowest, c)
	}
	return lowest, nil
}

// validateConfidence checks [0, 1] and the source's ceiling.
func validateConfidence(field string, source Source, confidence float64, rc ReviewContext) error {
	if err := inRange(field, confidence, 0, 1); err != nil {
		return err
	}
	ceiling, err := ConfidenceCeiling(source, rc)
	if err != nil {
		return err
	}
	if confidence > ceiling {
		return invalid(field, "exceeds the %s ceiling of %s for source %s", ConfidenceRule, fmtNum(ceiling), source)
	}
	return nil
}

func validateSource(field string, s Source) (SourceClass, error) {
	class, ok := s.Class()
	if !ok {
		return "", invalid(field, "is not in the source registry")
	}
	return class, nil
}
