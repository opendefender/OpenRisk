// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// controlFrom is a valid Control of any source at a given confidence.
func controlFrom(source canonical.Source, confidence float64) canonical.Control {
	ctl := validControl()
	ctl.Source, ctl.Confidence, ctl.Provenance = source, confidence, provenanceFor(source)
	return ctl
}

// reviewedEvidence is a user_entered Evidence collected by Alice, stored by
// Alice, and accepted by reviewer.
func reviewedEvidence(reviewer *uuid.UUID, verdict canonical.ReviewVerdict, confidence float64) canonical.Evidence {
	e := validEvidence()
	e.CollectedBy = ptr(userAlice)
	e.Provenance = userProvenance(userAlice)
	e.Review = canonical.EvidenceReview{Verdict: verdict, ReviewedBy: reviewer, ReviewedAt: ptr(stored)}
	e.Confidence = confidence
	return e
}

// TestCanonical_ConfidenceCeilingPerClass is ADR 0008 D3 / #1240 criterion 5.
func TestCanonical_ConfidenceCeilingPerClass(t *testing.T) {
	ceilings := []struct {
		class   canonical.SourceClass
		source  canonical.Source
		def     float64
		ceiling float64
	}{
		{canonical.ClassUserEntered, canonical.SourceOpenRiskUser, 0.70, 0.70},
		{canonical.ClassAutomatedScanner, canonical.SourceNessus, 0.80, 0.80},
		{canonical.ClassThreatIntelligence, canonical.SourceCISAKEV, 0.40, 0.40},
		{canonical.ClassConnectorImport, canonical.SourceJira, 0.60, 0.80},
		{canonical.ClassThirdPartyAttestation, canonical.SourceVendorQuestionnaire, 0.50, 0.50},
		{canonical.ClassRegulatoryCatalogue, canonical.SourceOpenRiskCatalogue, 0.90, 0.90},
		{canonical.ClassDerived, canonical.SourceOpenRiskDerived, 0, 0.90},
	}
	require.Len(t, ceilings, len(canonical.SourceClasses()), "every class has a row")

	for _, tc := range ceilings {
		t.Run(string(tc.class), func(t *testing.T) {
			class, ok := tc.source.Class()
			require.True(t, ok)
			require.Equal(t, tc.class, class)

			got, err := canonical.ConfidenceCeiling(tc.source, canonical.ReviewContext{})
			require.NoError(t, err)
			assert.Equal(t, tc.ceiling, got)

			require.NoError(t, controlFrom(tc.source, tc.ceiling).Validate(), "at the ceiling")
			require.NoError(t, controlFrom(tc.source, 0).Validate(), "0.00 is recorded and believed false")
			requireRefused(t, controlFrom(tc.source, tc.ceiling+0.01).Validate(), "ceiling")
		})
	}

	t.Run("the three ceilings the ADR names", func(t *testing.T) {
		// D3: "rejects a user_entered record above 0.90, a connector_import
		// record above 0.80 and a third_party_attestation record above 0.50",
		// whatever the review state.
		qualifying := reviewedEvidence(ptr(userBob), canonical.ReviewAccepted, 0.91)
		requireRefused(t, qualifying.Validate(), "ceiling")
		requireRefused(t, controlFrom(canonical.SourceServiceNow, 0.81).Validate(), "ceiling")
		requireRefused(t, controlFrom(canonical.SourceVendorQuestionnaire, 0.51).Validate(), "ceiling")
	})

	t.Run("no default and no ceiling is 1.00", func(t *testing.T) {
		for _, s := range canonical.Sources() {
			uplift := canonical.ReviewContext{Verdict: canonical.ReviewAccepted, ReviewedBy: ptr(userBob)}
			for _, rc := range []canonical.ReviewContext{{}, uplift} {
				ceiling, err := canonical.ConfidenceCeiling(s, rc)
				require.NoError(t, err)
				assert.Less(t, ceiling, 1.0, s)
				class, _ := s.Class()
				if class == canonical.ClassDerived {
					_, err := canonical.DefaultConfidence(s, rc)
					requireRefused(t, err, "minimum of its inputs")
					continue
				}
				def, err := canonical.DefaultConfidence(s, rc)
				require.NoError(t, err)
				assert.Less(t, def, 1.0, s)
				assert.LessOrEqual(t, def, ceiling, s)
			}
		}
		require.NoError(t, controlFrom(canonical.SourceOpenRiskCatalogue, 0.90).Validate())
		requireRefused(t, controlFrom(canonical.SourceOpenRiskCatalogue, 1.00).Validate(), "ceiling")
	})

	t.Run("defaults of confidence/1", func(t *testing.T) {
		for _, tc := range ceilings {
			if tc.class == canonical.ClassDerived {
				continue
			}
			got, err := canonical.DefaultConfidence(tc.source, canonical.ReviewContext{})
			require.NoError(t, err)
			assert.Equal(t, tc.def, got, tc.class)
		}
		_, err := canonical.DefaultConfidence("manual", canonical.ReviewContext{})
		requireRefused(t, err, "registry")
	})

	t.Run("derived is the minimum of its inputs", func(t *testing.T) {
		got, err := canonical.DerivedConfidence(0.80, 0.40, 0.70)
		require.NoError(t, err)
		assert.Equal(t, 0.40, got)
		_, err = canonical.DerivedConfidence()
		requireRefused(t, err, "")
		_, err = canonical.DerivedConfidence(0.5, 1.5)
		requireRefused(t, err, "")
	})

	t.Run("user_entered rises to 0.90 only on a second person's acceptance", func(t *testing.T) {
		ok := reviewedEvidence(ptr(userBob), canonical.ReviewAccepted, 0.90)
		require.NoError(t, ok.Validate())
		def, err := canonical.DefaultConfidence(ok.Source, ok.ReviewContext())
		require.NoError(t, err)
		assert.Equal(t, 0.90, def, "the uplift is computed, not supplied")

		refusals := map[string]canonical.Evidence{
			"accepted with no reviewer (today's default)": reviewedEvidence(nil, canonical.ReviewAccepted, 0.90),
			"reviewed by the collector":                   reviewedEvidence(ptr(userAlice), canonical.ReviewAccepted, 0.90),
			"pending":                                     reviewedEvidence(ptr(userBob), canonical.ReviewPending, 0.90),
			"rejected":                                    reviewedEvidence(ptr(userBob), canonical.ReviewRejected, 0.90),
		}
		byActor := reviewedEvidence(ptr(userBob), canonical.ReviewAccepted, 0.90)
		byActor.CollectedBy = ptr(uuid.New())
		byActor.Provenance = userProvenance(userBob)
		refusals["reviewed by the actor who stored it"] = byActor

		for name, e := range refusals {
			t.Run(name, func(t *testing.T) {
				requireRefused(t, e.Validate(), "ceiling")
				def, err := canonical.DefaultConfidence(e.Source, e.ReviewContext())
				require.NoError(t, err)
				assert.Equal(t, 0.70, def)
			})
		}
	})

	t.Run("only user_entered rises", func(t *testing.T) {
		e := reviewedEvidence(ptr(userBob), canonical.ReviewAccepted, 0.81)
		e.Source, e.Provenance = canonical.SourceOpenRiskScanner, provenanceFor(canonical.SourceOpenRiskScanner)
		requireRefused(t, e.Validate(), "ceiling")
	})
}
