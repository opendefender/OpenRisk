// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical_test

import (
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// Fixtures: one valid record of each type. Every test starts from one of these
// and breaks exactly one thing, so a refusal is attributable to that thing.

var (
	tenantA   = uuid.MustParse("a0000000-0000-4000-8000-00000000000a")
	tenantB   = uuid.MustParse("b0000000-0000-4000-8000-00000000000b")
	userAlice = uuid.MustParse("a1100000-0000-4000-8000-000000000001")
	userBob   = uuid.MustParse("b0b00000-0000-4000-8000-000000000002")

	// ADR 0008 D4's worked example: observed Monday 22:00, stored Thursday 09:00.
	observed = time.Date(2026, 11, 2, 22, 0, 0, 0, time.UTC)
	stored   = time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
)

func ptr[T any](v T) *T { return &v }

type validator interface{ Validate() error }

// userProvenance is a person with an account entering a record by hand.
func userProvenance(actor uuid.UUID) canonical.Provenance {
	return canonical.Provenance{
		Method:         canonical.MethodManualEntry,
		Actor:          canonical.Actor{Kind: canonical.ActorUser, ID: actor.String()},
		ConfidenceRule: canonical.ConfidenceRule,
	}
}

// provenanceFor is a valid provenance for any registry source, satisfying the
// source-specific rules (vendor_questionnaire's fixed values, derived inputs).
func provenanceFor(source canonical.Source) canonical.Provenance {
	class, _ := source.Class()
	switch {
	case source == canonical.SourceVendorQuestionnaire:
		return canonical.Provenance{
			Method:         canonical.MethodQuestionnaire,
			Actor:          canonical.Actor{Kind: canonical.ActorVendorContact, ID: "vendor_contact:" + uuid.NewString()},
			ConfidenceRule: canonical.ConfidenceRule,
		}
	case class == canonical.ClassDerived:
		return canonical.Provenance{
			Method:         canonical.MethodDerivation,
			Actor:          canonical.Actor{Kind: canonical.ActorSystem, ID: "risk-proposer"},
			ExternalRefs:   []canonical.ExternalRef{{Scheme: "cve", Value: "CVE-2024-3094"}},
			ConfidenceRule: canonical.ConfidenceRule,
		}
	case class == canonical.ClassUserEntered:
		return userProvenance(userAlice)
	default:
		return canonical.Provenance{
			Method:         canonical.MethodScan,
			Actor:          canonical.Actor{Kind: canonical.ActorSystem, ID: "ingest-worker"},
			Via:            ptr("vuln_integrations:7f3c"),
			ConfidenceRule: canonical.ConfidenceRule,
		}
	}
}

func envelope(source canonical.Source, confidence float64, observedAt *time.Time) canonical.Envelope {
	id := uuid.New()
	return canonical.Envelope{
		ID:         id,
		TenantID:   tenantA,
		Source:     source,
		SourceID:   id.String(),
		CreatedAt:  stored,
		UpdatedAt:  stored,
		ObservedAt: observedAt,
		Confidence: confidence,
		Provenance: provenanceFor(source),
	}
}

func validEdge() canonical.Edge {
	return canonical.Edge{
		ID:         uuid.New(),
		TenantID:   tenantA,
		From:       canonical.RecordRef{Type: canonical.TypeRequirement, ID: uuid.New()},
		Verb:       canonical.VerbAddressedBy,
		To:         canonical.EdgeTarget{Type: canonical.TypeControl, ID: uuid.New(), TenantID: tenantA},
		Attributes: canonical.EdgeAttributes{Coverage: ptr(canonical.CoverageFull)},
		Source:     canonical.SourceOpenRiskUser,
		SourceID:   uuid.NewString(),
		Confidence: 0.70,
		Provenance: userProvenance(userAlice),
		CreatedAt:  stored,
		UpdatedAt:  stored,
	}
}

func validRequirement() canonical.Requirement {
	env := envelope(canonical.SourceOpenRiskCatalogue, 0.90, nil)
	env.SourceVersion = ptr("2022")
	edge := validEdge()
	edge.From = canonical.RecordRef{Type: canonical.TypeRequirement, ID: env.ID}
	env.Relationships = []canonical.Edge{edge}
	return canonical.Requirement{
		Envelope:      env,
		Framework:     canonical.Framework{Key: "iso27001-2022", Name: "ISO/IEC 27001", Version: "2022"},
		ReferenceCode: "A.5.15",
		Title:         "Access control",
		Statement:     "Rules to control physical and logical access shall be established.",
		Citation:      ptr("ISO/IEC 27001:2022 Annex A 5.15"),
		Obligation:    canonical.ObligationShall,
		Language:      "en",
	}
}

func validControl() canonical.Control {
	return canonical.Control{
		Envelope:       envelope(canonical.SourceOpenRiskUser, 0.70, nil),
		Name:           "Quarterly access review",
		Ownership:      canonical.Ownership{OwnerID: ptr(userAlice)},
		Applicability:  canonical.Applicable,
		DeclaredStatus: canonical.DeclaredInProgress,
		Tags:           []string{"iam"},
	}
}

func validImplementation() canonical.ControlImplementation {
	return canonical.ControlImplementation{
		Envelope:     envelope(canonical.SourceOpenRiskUser, 0.70, nil),
		ControlID:    uuid.New(),
		Description:  "Access reviews on the core banking system, exported from the IdP.",
		Status:       canonical.ImplementationInPlace,
		InPlaceSince: ptr(canonical.Date("2026-01-15")),
	}
}

func validResult() canonical.ControlTestResult {
	return canonical.ControlTestResult{
		ResultID:   uuid.New(),
		Outcome:    canonical.OutcomePass,
		ObservedAt: observed,
		RecordedAt: stored,
		Source:     canonical.SourceOpenRiskUser,
		SourceID:   uuid.NewString(),
		Confidence: 0.70,
		ExecutedBy: canonical.Actor{Kind: canonical.ActorUser, ID: userAlice.String()},
	}
}

func validControlTest() canonical.ControlTest {
	return canonical.ControlTest{
		Envelope:         envelope(canonical.SourceOpenRiskUser, 0.70, nil),
		ImplementationID: uuid.New(),
		Name:             "Review sample of 25 accounts",
		Checks:           "Every sampled account has a documented review this quarter.",
		Method:           canonical.TestManual,
		Procedure:        ptr("Export accounts, sample 25, check each review record."),
		Cadence:          ptr("P3M"),
		PassCriteria:     "25 of 25 reviewed.",
		Results:          []canonical.ControlTestResult{validResult()},
	}
}

func validEvidence() canonical.Evidence {
	return canonical.Evidence{
		Envelope:    envelope(canonical.SourceOpenRiskUser, 0.70, ptr(observed)),
		Title:       "Q3 access review export",
		Kind:        canonical.KindDocument,
		FileRef:     ptr("tenants/a/evidence/q3.pdf"),
		Filename:    ptr("q3.pdf"),
		ExternalURL: ptr("https://docs.example.com/reviews/q3"),
		CollectedBy: ptr(userAlice),
		Review:      canonical.EvidenceReview{Verdict: canonical.ReviewPending},
	}
}

func validRisk() canonical.Risk {
	return canonical.Risk{
		Envelope:       envelope(canonical.SourceOpenRiskUser, 0.70, ptr(observed)),
		Name:           "Ransomware on the core banking system",
		Probability:    0.4,
		Impact:         8,
		LifecycleState: canonical.StateAssessed,
		Treatment:      canonical.TreatmentMitigate,
		Tags:           []string{},
		Quantification: &canonical.Quantification{Currency: "XAF", SLE: ptr(1.5e8), ARO: ptr(0.5)},
		CreatedBy:      userAlice,
	}
}

func validFinding() canonical.Finding {
	return canonical.Finding{
		Envelope:        envelope(canonical.SourceNessus, 0.80, ptr(observed)),
		Kind:            canonical.FindingVulnerability,
		Title:           "xz-utils backdoor",
		Severity:        canonical.SeverityCritical,
		Status:          canonical.FindingOpen,
		Identifiers:     []canonical.FindingIdentifier{{Scheme: canonical.SchemeCVE, Value: "CVE-2024-3094"}},
		CVSS:            &canonical.CVSS{Score: 10, Vector: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H", Version: ptr("3.1")},
		Exploitability:  &canonical.Exploitability{EPSS: 0.9, KEV: true, ExploitMaturity: ptr(canonical.MaturityFunctional)},
		FirstObservedAt: observed,
	}
}

// entityCase builds one valid entity and lets a test break its envelope.
type entityCase struct {
	name  string
	build func(mutate func(*canonical.Envelope)) validator
}

func entityCases() []entityCase {
	return []entityCase{
		{"Requirement", func(m func(*canonical.Envelope)) validator { r := validRequirement(); m(&r.Envelope); return r }},
		{"Control", func(m func(*canonical.Envelope)) validator { r := validControl(); m(&r.Envelope); return r }},
		{"ControlImplementation", func(m func(*canonical.Envelope)) validator { r := validImplementation(); m(&r.Envelope); return r }},
		{"ControlTest", func(m func(*canonical.Envelope)) validator { r := validControlTest(); m(&r.Envelope); return r }},
		{"Evidence", func(m func(*canonical.Envelope)) validator { r := validEvidence(); m(&r.Envelope); return r }},
		{"Risk", func(m func(*canonical.Envelope)) validator { r := validRisk(); m(&r.Envelope); return r }},
		{"Finding", func(m func(*canonical.Envelope)) validator { r := validFinding(); m(&r.Envelope); return r }},
	}
}

func noChange(*canonical.Envelope) {}
