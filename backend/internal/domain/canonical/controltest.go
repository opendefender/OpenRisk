// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// This file is not named control_test.go on purpose: that suffix would make it
// a test file and drop ControlTest from the package.

package canonical

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// TestMethod is how a control test runs (D6, closed).
type TestMethod string

// D6's test methods.
const (
	TestAutomated TestMethod = "automated"
	TestManual    TestMethod = "manual"
)

// Outcome is a test run's result (D6, closed). inconclusive never counts as a
// pass.
type Outcome string

// D6's outcomes.
const (
	OutcomePass         Outcome = "pass"
	OutcomeFail         Outcome = "fail"
	OutcomeInconclusive Outcome = "inconclusive"
)

// ControlTest is a test of one implementation: what makes a control's status
// derivable rather than declared (ADR 0008 D1, D6). The definition is not
// observed, so its observed_at is null; each result carries its own (D4).
type ControlTest struct {
	Envelope
	ImplementationID uuid.UUID               `json:"implementation_id"`
	Name             string                  `json:"name"`
	Checks           string                  `json:"checks"`
	Method           TestMethod              `json:"method"`
	Procedure        *string                 `json:"procedure"`
	Collector        *string                 `json:"collector"`
	Cadence          *string                 `json:"cadence"`
	PassCriteria     string                  `json:"pass_criteria"`
	Results          List[ControlTestResult] `json:"results"`
}

// ControlTestResult is one run of a ControlTest: a sub-record, not an entity
// (D1). It has no tenant_id of its own; it is reached only through its test,
// whose envelope carries the tenant. Append-only, newest last.
type ControlTestResult struct {
	ResultID   uuid.UUID `json:"result_id"`
	Outcome    Outcome   `json:"outcome"`
	ObservedAt time.Time `json:"observed_at"`
	RecordedAt time.Time `json:"recorded_at"`
	Source     Source    `json:"source"`
	SourceID   string    `json:"source_id"`
	Confidence float64   `json:"confidence"`
	ExecutedBy Actor     `json:"executed_by"`
	Note       *string   `json:"note"`
}

// resultClasses are the classes D6 names for a result: a manual run is
// user_entered, a collector run is automated_scanner or connector_import.
var resultClasses = map[SourceClass]bool{
	ClassUserEntered: true, ClassAutomatedScanner: true, ClassConnectorImport: true,
}

// Validate applies D2, D3, D4 and D6 to a ControlTest and each of its results.
func (t ControlTest) Validate() error {
	if _, err := t.Envelope.validate(TypeControlTest, observedForbidden, ReviewContext{}); err != nil {
		return err
	}
	if err := requireID("implementation_id", t.ImplementationID); err != nil {
		return err
	}
	if err := requireText("name", t.Name, 255); err != nil {
		return err
	}
	if err := requireText("checks", t.Checks, 0); err != nil {
		return err
	}
	switch t.Method {
	case TestManual:
		if blankPtr(t.Procedure) {
			return invalid("procedure", "is required when the method is manual")
		}
	case TestAutomated:
		if blankPtr(t.Collector) {
			return invalid("collector", "is required when the method is automated")
		}
	default:
		return invalid("method", "must be automated or manual")
	}
	if t.Cadence != nil && !isISO8601Duration(*t.Cadence) {
		return invalid("cadence", "must be a non-zero ISO 8601 duration such as P30D or P3M, or null")
	}
	if err := requireText("pass_criteria", t.PassCriteria, 0); err != nil {
		return err
	}
	seen := make(map[uuid.UUID]bool, len(t.Results))
	for i, r := range t.Results {
		field := fmt.Sprintf("results[%d]", i)
		if err := within(field, r.Validate()); err != nil {
			return err
		}
		if seen[r.ResultID] {
			return invalid(field+".result_id", "is not unique within the test")
		}
		seen[r.ResultID] = true
		if i > 0 && r.RecordedAt.Before(t.Results[i-1].RecordedAt) {
			return invalid(field+".recorded_at", "results are append-only, newest last")
		}
	}
	// An outgoing evidenced_by names, as its qualifier, a result of this test
	// (D5, A3): evidence backs a run that exists, not any uuid.
	for i, edge := range t.Relationships {
		if edge.Verb == VerbEvidencedBy && edge.Qualifier != nil && !seen[*edge.Qualifier] {
			return invalid(fmt.Sprintf("relationships[%d].qualifier", i), "must be the result_id of one of this test's results")
		}
	}
	return nil
}

// Validate applies D3, D4 and D6 to one result.
func (r ControlTestResult) Validate() error {
	if err := requireID("result_id", r.ResultID); err != nil {
		return err
	}
	switch r.Outcome {
	case OutcomePass, OutcomeFail, OutcomeInconclusive:
	default:
		return invalid("outcome", "must be pass, fail or inconclusive")
	}
	if err := requireTime("observed_at", r.ObservedAt); err != nil {
		return err
	}
	if err := requireTime("recorded_at", r.RecordedAt); err != nil {
		return err
	}
	class, err := validateSource("source", r.Source)
	if err != nil {
		return err
	}
	if !resultClasses[class] {
		return invalid("source", "a test result is user_entered, automated_scanner or connector_import")
	}
	if err := requireText("source_id", r.SourceID, 255); err != nil {
		return err
	}
	if err := validateConfidence("confidence", r.Source, r.Confidence, ReviewContext{ActorID: r.ExecutedBy.ID}); err != nil {
		return err
	}
	return r.ExecutedBy.validate("executed_by")
}
