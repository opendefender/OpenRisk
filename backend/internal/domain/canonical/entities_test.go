// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain/canonical"
)

func requireRefused(t *testing.T, err error, field string) {
	t.Helper()
	require.Error(t, err)
	require.True(t, errors.Is(err, canonical.ErrValidation), "error must wrap canonical.ErrValidation: %v", err)
	if field != "" {
		require.Contains(t, err.Error(), field, "refused for the wrong reason")
	}
}

// TestCanonical_Entities_Success: every fixture is a valid canonical record.
func TestCanonical_Entities_Success(t *testing.T) {
	for _, tc := range entityCases() {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, tc.build(noChange).Validate())
		})
	}
	require.NoError(t, validEdge().Validate())
	require.NoError(t, validResult().Validate())
}

// TestCanonical_TenantIDRequired is ADR 0008 D9 / #1240 criterion 1: no
// canonical record validates without a tenant. ABSOLUTE RULE 2.
func TestCanonical_TenantIDRequired(t *testing.T) {
	for _, tc := range entityCases() {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.build(func(e *canonical.Envelope) { e.TenantID = uuid.Nil }).Validate()
			requireRefused(t, err, "tenant_id")
		})
	}
	t.Run("Edge", func(t *testing.T) {
		e := validEdge()
		e.TenantID = uuid.Nil
		requireRefused(t, e.Validate(), "tenant_id")
	})
	t.Run("Edge target", func(t *testing.T) {
		e := validEdge()
		e.To.TenantID = uuid.Nil
		requireRefused(t, e.Validate(), "to.tenant_id")
	})
}

// TestCanonical_EnvelopeFields_Refused covers the rest of D2 on every entity.
func TestCanonical_EnvelopeFields_Refused(t *testing.T) {
	breaks := []struct {
		name   string
		field  string
		mutate func(*canonical.Envelope)
	}{
		{"nil id", "id", func(e *canonical.Envelope) { e.ID = uuid.Nil }},
		{"source outside the registry", "source", func(e *canonical.Envelope) { e.Source = "manual" }},
		{"empty source_id", "source_id", func(e *canonical.Envelope) { e.SourceID = " " }},
		{"source_id over 255", "source_id", func(e *canonical.Envelope) { e.SourceID = strings.Repeat("x", 256) }},
		{"source_version over 64", "source_version", func(e *canonical.Envelope) { e.SourceVersion = ptr(strings.Repeat("v", 65)) }},
		{"zero created_at", "created_at", func(e *canonical.Envelope) { e.CreatedAt = time.Time{} }},
		{"zero updated_at", "updated_at", func(e *canonical.Envelope) { e.UpdatedAt = time.Time{} }},
		{"negative confidence", "confidence", func(e *canonical.Envelope) { e.Confidence = -0.01 }},
		{"confidence above 1", "confidence", func(e *canonical.Envelope) { e.Confidence = 1.01 }},
		{"missing provenance", "provenance.method", func(e *canonical.Envelope) { e.Provenance = canonical.Provenance{} }},
		{"edge from another record", "relationships[0].from", func(e *canonical.Envelope) {
			edge := validEdge()
			e.Relationships = []canonical.Edge{edge}
		}},
	}
	for _, tc := range entityCases() {
		for _, b := range breaks {
			t.Run(tc.name+"/"+b.name, func(t *testing.T) {
				requireRefused(t, tc.build(b.mutate).Validate(), b.field)
			})
		}
	}
}

// TestCanonical_EdgeInRelationships_RefusesAnotherTenant: an outgoing edge
// inherits its record's tenant.
func TestCanonical_EdgeInRelationships_RefusesAnotherTenant(t *testing.T) {
	r := validRequirement()
	r.Relationships[0].TenantID = tenantB
	r.Relationships[0].To.TenantID = tenantB
	requireRefused(t, r.Validate(), "relationships[0].tenant_id")
}

// TestCanonical_ObservedAtRequired is ADR 0008 D4 / #1240 criterion 6.
func TestCanonical_ObservedAtRequired(t *testing.T) {
	t.Run("Evidence without observed_at", func(t *testing.T) {
		e := validEvidence()
		e.ObservedAt = nil
		requireRefused(t, e.Validate(), "observed_at")
	})
	t.Run("Finding without observed_at", func(t *testing.T) {
		f := validFinding()
		f.ObservedAt = nil
		requireRefused(t, f.Validate(), "observed_at")
	})
	t.Run("ControlTestResult without observed_at", func(t *testing.T) {
		r := validResult()
		r.ObservedAt = time.Time{}
		requireRefused(t, r.Validate(), "observed_at")
	})
	t.Run("ControlTest carrying a result without observed_at", func(t *testing.T) {
		ct := validControlTest()
		ct.Results[0].ObservedAt = time.Time{}
		requireRefused(t, ct.Validate(), "results[0]")
	})
	for _, tc := range []struct {
		name   string
		record func() validator
	}{
		{"Requirement", func() validator { r := validRequirement(); r.ObservedAt = ptr(observed); return r }},
		{"Control", func() validator { r := validControl(); r.ObservedAt = ptr(observed); return r }},
		{"ControlTest", func() validator { r := validControlTest(); r.ObservedAt = ptr(observed); return r }},
	} {
		t.Run(tc.name+" with observed_at, which D4 says is null", func(t *testing.T) {
			requireRefused(t, tc.record().Validate(), "observed_at")
		})
	}
	t.Run("ControlImplementation and Risk may have it or not", func(t *testing.T) {
		ci := validImplementation()
		require.NoError(t, ci.Validate())
		ci.ObservedAt = ptr(observed)
		require.NoError(t, ci.Validate())
		r := validRisk()
		require.NoError(t, r.Validate())
		r.ObservedAt = nil
		require.NoError(t, r.Validate())
	})
}

func TestCanonical_Requirement_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		field  string
		mutate func(*canonical.Requirement)
	}{
		{"catalogue requirement without citation", "citation", func(r *canonical.Requirement) { r.Citation = nil }},
		{"catalogue requirement without framework key", "framework.key", func(r *canonical.Requirement) { r.Framework.Key = "" }},
		{"catalogue requirement without reference code", "reference_code", func(r *canonical.Requirement) { r.ReferenceCode = "" }},
		{"reference_code over 50", "reference_code", func(r *canonical.Requirement) { r.ReferenceCode = strings.Repeat("A", 51) }},
		{"citation over 255", "citation", func(r *canonical.Requirement) { r.Citation = ptr(strings.Repeat("c", 256)) }},
		{"empty title", "title", func(r *canonical.Requirement) { r.Title = "" }},
		{"title over 255", "title", func(r *canonical.Requirement) { r.Title = strings.Repeat("t", 256) }},
		{"empty statement", "statement", func(r *canonical.Requirement) { r.Statement = "" }},
		{"no framework name", "framework.name", func(r *canonical.Requirement) { r.Framework.Name = "" }},
		{"obligation outside the enum", "obligation", func(r *canonical.Requirement) { r.Obligation = "must" }},
		{"language not a locale", "language", func(r *canonical.Requirement) { r.Language = "French" }},
		{"nil parent", "parent_id", func(r *canonical.Requirement) { r.ParentID = ptr(uuid.Nil) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := validRequirement()
			tc.mutate(&r)
			requireRefused(t, r.Validate(), tc.field)
		})
	}
	t.Run("a tenant-authored requirement needs no citation, key or code", func(t *testing.T) {
		r := validRequirement()
		r.Source, r.Confidence, r.Provenance = canonical.SourceOpenRiskUser, 0.70, userProvenance(userAlice)
		r.Relationships[0].From.ID = r.ID
		r.Citation, r.Framework.Key, r.ReferenceCode = nil, "", ""
		require.NoError(t, r.Validate())
	})
}

func TestCanonical_Control_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		field  string
		mutate func(*canonical.Control)
	}{
		{"not_applicable without justification", "applicability_justification", func(c *canonical.Control) {
			c.Applicability = canonical.NotApplicable
		}},
		{"not_applicable with a blank justification", "applicability_justification", func(c *canonical.Control) {
			c.Applicability, c.ApplicabilityJustification = canonical.NotApplicable, ptr("  ")
		}},
		{"applicability outside the enum", "applicability", func(c *canonical.Control) { c.Applicability = "n/a" }},
		{"not_applicable as a declared status", "declared_status", func(c *canonical.Control) { c.DeclaredStatus = "not_applicable" }},
		{"empty name", "name", func(c *canonical.Control) { c.Name = "" }},
		{"nil owner", "owner_id", func(c *canonical.Control) { c.OwnerID = ptr(uuid.Nil) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validControl()
			tc.mutate(&c)
			requireRefused(t, c.Validate(), tc.field)
		})
	}
	t.Run("not_applicable with a justification", func(t *testing.T) {
		c := validControl()
		c.Applicability, c.ApplicabilityJustification = canonical.NotApplicable, ptr("No card data is processed.")
		require.NoError(t, c.Validate())
	})
}

func TestCanonical_ControlImplementation_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		field  string
		mutate func(*canonical.ControlImplementation)
	}{
		{"no control", "control_id", func(c *canonical.ControlImplementation) { c.ControlID = uuid.Nil }},
		{"implemented without a how", "description", func(c *canonical.ControlImplementation) { c.Description = "" }},
		{"status outside the enum", "status", func(c *canonical.ControlImplementation) { c.Status = "implemented" }},
		{"in_place_since not a date", "in_place_since", func(c *canonical.ControlImplementation) { c.InPlaceSince = ptr(canonical.Date("15/01/2026")) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validImplementation()
			tc.mutate(&c)
			requireRefused(t, c.Validate(), tc.field)
		})
	}
}

func TestCanonical_ControlTest_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		field  string
		mutate func(*canonical.ControlTest)
	}{
		{"manual without procedure", "procedure", func(c *canonical.ControlTest) { c.Procedure = nil }},
		{"automated without collector", "collector", func(c *canonical.ControlTest) { c.Method = canonical.TestAutomated }},
		{"method outside the enum", "method", func(c *canonical.ControlTest) { c.Method = "hybrid" }},
		{"cadence not ISO 8601", "cadence", func(c *canonical.ControlTest) { c.Cadence = ptr("quarterly") }},
		{"cadence without a component", "cadence", func(c *canonical.ControlTest) { c.Cadence = ptr("PT") }},
		{"zero cadence", "cadence", func(c *canonical.ControlTest) { c.Cadence = ptr("P0D") }},
		{"cadence out of order", "cadence", func(c *canonical.ControlTest) { c.Cadence = ptr("P1D2M") }},
		{"no implementation", "implementation_id", func(c *canonical.ControlTest) { c.ImplementationID = uuid.Nil }},
		{"no checks", "checks", func(c *canonical.ControlTest) { c.Checks = "" }},
		{"no pass criteria", "pass_criteria", func(c *canonical.ControlTest) { c.PassCriteria = "" }},
		{"name over 255", "name", func(c *canonical.ControlTest) { c.Name = strings.Repeat("n", 256) }},
		{"duplicate result id", "result_id", func(c *canonical.ControlTest) { c.Results = append(c.Results, c.Results[0]) }},
		{"results not newest last", "recorded_at", func(c *canonical.ControlTest) {
			older := validResult()
			older.RecordedAt = stored.Add(-time.Hour)
			c.Results = append(c.Results, older)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validControlTest()
			tc.mutate(&c)
			requireRefused(t, c.Validate(), tc.field)
		})
	}
	t.Run("automated with a collector and on-demand cadence", func(t *testing.T) {
		c := validControlTest()
		c.Method, c.Procedure, c.Collector, c.Cadence = canonical.TestAutomated, nil, ptr("okta-access-review"), nil
		require.NoError(t, c.Validate())
	})
	t.Run("cadences of ADR 0008 D6", func(t *testing.T) {
		for _, d := range []string{"P30D", "P3M", "P1Y", "P2W", "PT12H", "P1DT6H"} {
			c := validControlTest()
			c.Cadence = ptr(d)
			require.NoError(t, c.Validate(), d)
		}
	})
}

func TestCanonical_ControlTestResult_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		field  string
		mutate func(*canonical.ControlTestResult)
	}{
		{"no result id", "result_id", func(r *canonical.ControlTestResult) { r.ResultID = uuid.Nil }},
		{"outcome outside the enum", "outcome", func(r *canonical.ControlTestResult) { r.Outcome = "partial" }},
		{"no recorded_at", "recorded_at", func(r *canonical.ControlTestResult) { r.RecordedAt = time.Time{} }},
		{"a threat feed as the runner", "source", func(r *canonical.ControlTestResult) { r.Source = canonical.SourceCISAKEV }},
		{"manual run above the user_entered ceiling", "confidence", func(r *canonical.ControlTestResult) { r.Confidence = 0.71 }},
		{"no source_id", "source_id", func(r *canonical.ControlTestResult) { r.SourceID = "" }},
		{"executed_by not a user id", "executed_by.id", func(r *canonical.ControlTestResult) { r.ExecutedBy.ID = "alice" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := validResult()
			tc.mutate(&r)
			requireRefused(t, r.Validate(), tc.field)
		})
	}
	t.Run("a collector run", func(t *testing.T) {
		r := validResult()
		r.Source, r.Confidence = canonical.SourceOpenRiskAgent, 0.80
		r.ExecutedBy = canonical.Actor{Kind: canonical.ActorSystem, ID: "collector:okta"}
		require.NoError(t, r.Validate())
	})
}

func TestCanonical_Evidence_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		field  string
		mutate func(*canonical.Evidence)
	}{
		{"no file, url or description", "at least one of", func(e *canonical.Evidence) {
			e.FileRef, e.ExternalURL, e.Description = nil, nil, ptr(" ")
		}},
		{"kind outside the enum", "kind", func(e *canonical.Evidence) { e.Kind = "screenshot" }},
		{"empty title", "title", func(e *canonical.Evidence) { e.Title = "" }},
		{"filename over 255", "filename", func(e *canonical.Evidence) { e.Filename = ptr(strings.Repeat("f", 256)) }},
		{"verdict outside the enum", "review.verdict", func(e *canonical.Evidence) { e.Review.Verdict = "approved" }},
		{"external_url not http(s)", "external_url", func(e *canonical.Evidence) { e.ExternalURL = ptr("javascript:alert(1)") }},
		{"external_url relative", "external_url", func(e *canonical.Evidence) { e.ExternalURL = ptr("/reviews/q3") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := validEvidence()
			tc.mutate(&e)
			requireRefused(t, e.Validate(), tc.field)
		})
	}
	t.Run("an inline statement is evidence enough", func(t *testing.T) {
		e := validEvidence()
		e.FileRef, e.ExternalURL, e.Description = nil, nil, ptr("MFA is enforced tenant-wide; see the IdP policy.")
		require.NoError(t, e.Validate())
	})
}

func TestCanonical_Risk_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		field  string
		mutate func(*canonical.Risk)
	}{
		{"probability above 1", "probability", func(r *canonical.Risk) { r.Probability = 1.2 }},
		{"impact above 10", "impact", func(r *canonical.Risk) { r.Impact = 11 }},
		{"lifecycle state outside RiskState", "lifecycle_state", func(r *canonical.Risk) { r.LifecycleState = "OPEN" }},
		{"treatment outside the enum", "treatment", func(r *canonical.Risk) { r.Treatment = "ignore" }},
		{"no created_by", "created_by", func(r *canonical.Risk) { r.CreatedBy = uuid.Nil }},
		{"currency not ISO 4217", "quantification.currency", func(r *canonical.Risk) { r.Quantification.Currency = "fcfa" }},
		{"negative SLE", "quantification.sle", func(r *canonical.Risk) { r.Quantification.SLE = ptr(-1.0) }},
		{"mitigation effectiveness above 1", "mitigation_effectiveness", func(r *canonical.Risk) {
			r.Quantification.MitigationEffectiveness = ptr(1.5)
		}},
		{"residual without formula version", "residual.formula_version", func(r *canonical.Risk) {
			r.Residual = &canonical.Residual{Value: 1.2, Band: canonical.CriticalityLow}
		}},
		{"negative review interval", "review_cadence.interval_days", func(r *canonical.Risk) {
			r.ReviewCadence = &canonical.ReviewCadence{IntervalDays: -1}
		}},
		{"category without id", "category.id", func(r *canonical.Risk) { r.Category = &canonical.Category{Name: "Cyber"} }},
		{"empty name", "name", func(r *canonical.Risk) { r.Name = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := validRisk()
			tc.mutate(&r)
			requireRefused(t, r.Validate(), tc.field)
		})
	}
	t.Run("projected scores are validated", func(t *testing.T) {
		r := canonical.WithEngineScores(validRisk(), canonical.RiskScore{Value: 3.2, Criticality: canonical.CriticalityMedium},
			&canonical.SmartScore{Value: 140, Level: canonical.CriticalityHigh, ComputedAt: stored})
		requireRefused(t, r.Validate(), "smart_score.value")
		r = canonical.WithEngineScores(validRisk(), canonical.RiskScore{Value: 3.2, Criticality: "MEDIUM"}, nil)
		requireRefused(t, r.Validate(), "score.criticality")
		r = canonical.WithEngineScores(validRisk(), canonical.RiskScore{Value: 3.2, Criticality: canonical.CriticalityMedium},
			&canonical.SmartScore{Value: 61, Level: canonical.CriticalityHigh, ComputedAt: stored,
				Factors: []canonical.SmartFactor{{Key: "exploitability", Weight: 0.2, Value: 0.9, Contribution: 18}}})
		require.NoError(t, r.Validate())
	})
}

func TestCanonical_Finding_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		field  string
		mutate func(*canonical.Finding)
	}{
		{"kind outside the enum", "kind", func(f *canonical.Finding) { f.Kind = "misconfiguration" }},
		{"title over 512", "title", func(f *canonical.Finding) { f.Title = strings.Repeat("t", 513) }},
		{"severity outside the enum", "severity", func(f *canonical.Finding) { f.Severity = "urgent" }},
		{"status outside the enum", "status", func(f *canonical.Finding) { f.Status = "closed" }},
		{"false_positive still believed", "confidence", func(f *canonical.Finding) { f.Status = canonical.FindingFalsePositive }},
		{"identifier scheme outside the enum", "identifiers[0].scheme", func(f *canonical.Finding) {
			f.Identifiers = []canonical.FindingIdentifier{{Scheme: "ghsa", Value: "GHSA-x"}}
		}},
		{"cvss above 10", "cvss.score", func(f *canonical.Finding) { f.CVSS.Score = 10.5 }},
		{"cvss vector over 128", "cvss.vector", func(f *canonical.Finding) { f.CVSS.Vector = strings.Repeat("v", 129) }},
		{"epss above 1", "exploitability.epss", func(f *canonical.Finding) { f.Exploitability.EPSS = 1.1 }},
		{"exploit maturity outside the enum", "exploit_maturity", func(f *canonical.Finding) {
			f.Exploitability.ExploitMaturity = ptr(canonical.ExploitMaturity("weaponized"))
		}},
		{"no first_observed_at", "first_observed_at", func(f *canonical.Finding) { f.FirstObservedAt = time.Time{} }},
		{"first observed after the latest observation", "first_observed_at", func(f *canonical.Finding) {
			f.FirstObservedAt = observed.Add(time.Hour)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := validFinding()
			tc.mutate(&f)
			requireRefused(t, f.Validate(), tc.field)
		})
	}
	t.Run("false_positive at 0.00", func(t *testing.T) {
		f := validFinding()
		f.Status, f.Confidence = canonical.FindingFalsePositive, 0
		require.NoError(t, f.Validate())
	})
	t.Run("D4's re-observation: observed_at moves, first_observed_at stays", func(t *testing.T) {
		f := validFinding()
		f.ObservedAt = ptr(time.Date(2026, 11, 9, 22, 0, 0, 0, time.UTC))
		require.NoError(t, f.Validate())
	})
}

// TestCanonical_EnvelopeJSON_RoundTrip pins the wire names #542 generates the
// schema from: snake_case, as ADR 0008 spells them.
func TestCanonical_EnvelopeJSON_RoundTrip(t *testing.T) {
	e := validEvidence()
	raw, err := json.Marshal(e)
	require.NoError(t, err)
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &m))
	for _, key := range []string{
		"id", "tenant_id", "source", "source_id", "source_version", "created_at", "updated_at",
		"observed_at", "confidence", "provenance", "relationships",
		"title", "kind", "description", "file_ref", "filename", "external_url", "valid_until",
		"collected_by", "review", "owner_id", "assignee_id", "reviewer_id",
	} {
		assert.Contains(t, m, key)
	}
	assert.Len(t, m, 23, "Evidence carries the 11 envelope fields and D6's 12 own fields, no more")

	var back canonical.Evidence
	require.NoError(t, json.Unmarshal(raw, &back))
	require.NoError(t, back.Validate())
	assert.Equal(t, e.ID, back.ID)
}
