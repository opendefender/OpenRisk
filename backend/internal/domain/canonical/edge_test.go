// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	c "github.com/opendefender/openrisk/internal/domain/canonical"
)

// d5 is ADR 0008 D5's table, transcribed by hand from the ADR. The package's
// vocabulary must equal it exactly: one row more or one row less fails.
var d5 = []c.Triple{
	{Verb: c.VerbAddressedBy, From: c.TypeRequirement, To: c.TypeControl},
	{Verb: c.VerbImplementedBy, From: c.TypeControl, To: c.TypeControlImplementation},
	{Verb: c.VerbTestedBy, From: c.TypeControlImplementation, To: c.TypeControlTest},
	{Verb: c.VerbEvidencedBy, From: c.TypeControlTest, To: c.TypeEvidence},
	{Verb: c.VerbEvidencedBy, From: c.TypeFinding, To: c.TypeEvidence},
	{Verb: c.VerbEvidencedBy, From: c.TypeControl, To: c.TypeEvidence},
	{Verb: c.VerbMapsTo, From: c.TypeRequirement, To: c.TypeRequirement},
	{Verb: c.VerbMitigatedBy, From: c.TypeRisk, To: c.TypeControl},
	{Verb: c.VerbIndicates, From: c.TypeFinding, To: c.TypeRisk},
	{Verb: c.VerbViolates, From: c.TypeFinding, To: c.TypeControl},
	{Verb: c.VerbRaisedBy, From: c.TypeFinding, To: c.TypeControlTest},
	{Verb: c.VerbAffects, From: c.TypeFinding, To: c.TypeAsset},
	{Verb: c.VerbAffects, From: c.TypeRisk, To: c.TypeAsset},
	{Verb: c.VerbAppliesTo, From: c.TypeControlImplementation, To: c.TypeAsset},
	{Verb: c.VerbAppliesTo, From: c.TypeControlImplementation, To: c.TypeSystem},
	{Verb: c.VerbAppliesTo, From: c.TypeControlImplementation, To: c.TypeService},
	{Verb: c.VerbCausedBy, From: c.TypeFinding, To: c.TypeThreat},
	{Verb: c.VerbRequires, From: c.TypeFinding, To: c.TypeAction},
	{Verb: c.VerbTreatedBy, From: c.TypeRisk, To: c.TypeMitigation},
	{Verb: c.VerbAssesses, From: c.TypeAssessment, To: c.TypeVendor},
}

// edgeFor builds a valid edge for a D5 row, with the qualifier and the
// attributes that row requires.
func edgeFor(tr c.Triple) c.Edge {
	e := validEdge()
	e.Verb = tr.Verb
	e.From.Type = tr.From
	e.To.Type = tr.To
	e.Attributes = c.EdgeAttributes{}
	if (tr.Verb == c.VerbEvidencedBy && tr.From == c.TypeControlTest) || tr.Verb == c.VerbRaisedBy {
		e.Qualifier = ptr(uuid.New())
	}
	if tr.Verb == c.VerbMapsTo {
		e.Attributes.Rationale = ptr("Both ask for an access control policy.")
	}
	return e
}

// TestCanonical_Edge_Success: every row of D5, shape-deferred targets included,
// validates.
func TestCanonical_Edge_Success(t *testing.T) {
	for _, tr := range d5 {
		t.Run(string(tr.Verb)+"/"+string(tr.From)+"->"+string(tr.To), func(t *testing.T) {
			require.NoError(t, edgeFor(tr).Validate())
		})
	}
	t.Run("affects carries the attribution attributes", func(t *testing.T) {
		e := edgeFor(c.Triple{Verb: c.VerbAffects, From: c.TypeFinding, To: c.TypeAsset})
		e.Source, e.Confidence, e.Provenance = c.SourceNessus, 0.80, provenanceFor(c.SourceNessus)
		e.Attributes = c.EdgeAttributes{
			MatchConfidence: ptr(0.92), MatchMethod: ptr("hostname+ip"), MatchAmbiguous: ptr(false),
			MatchCandidates: json.RawMessage(`[{"asset_id":"x","score":0.92}]`), ResolvedBy: ptr(userBob),
			AssetCriticalityAtObservation: ptr("HIGH"),
		}
		require.NoError(t, e.Validate())
	})
}

// TestCanonical_EdgeRefusesAnotherTenant is ADR 0008 D5 / #1240 criterion 3.
func TestCanonical_EdgeRefusesAnotherTenant(t *testing.T) {
	for _, tr := range d5 {
		t.Run(string(tr.Verb)+"/"+string(tr.From)+"->"+string(tr.To), func(t *testing.T) {
			e := edgeFor(tr)
			e.To.TenantID = tenantB
			requireRefused(t, e.Validate(), "to.tenant_id")
		})
	}
	t.Run("an unresolved target is not trusted", func(t *testing.T) {
		e := validEdge()
		e.To.TenantID = uuid.Nil
		requireRefused(t, e.Validate(), "to.tenant_id")
	})
}

// TestCanonical_EdgeVocabularyIsClosed is ADR 0008 D5 / #1240 criterion 4.
func TestCanonical_EdgeVocabularyIsClosed(t *testing.T) {
	t.Run("the vocabulary is exactly D5", func(t *testing.T) {
		assert.ElementsMatch(t, d5, c.Vocabulary())
	})
	t.Run("a verb outside D5", func(t *testing.T) {
		for _, verb := range []c.Verb{"owns", "addresses", "evidences", "depends_on", "managed_by", ""} {
			e := validEdge()
			e.Verb = verb
			requireRefused(t, e.Validate(), "vocabulary")
		}
	})
	t.Run("a known verb on a triple outside D5", func(t *testing.T) {
		for _, tr := range []c.Triple{
			{Verb: c.VerbAddressedBy, From: c.TypeControl, To: c.TypeRequirement}, // the inverse is a reading, not a stored verb
			{Verb: c.VerbEvidencedBy, From: c.TypeRisk, To: c.TypeEvidence},
			{Verb: c.VerbMapsTo, From: c.TypeControl, To: c.TypeControl}, // maps_to joins requirements, not controls
			{Verb: c.VerbAffects, From: c.TypeEvidence, To: c.TypeAsset},
			{Verb: c.VerbAssesses, From: c.TypeVendor, To: c.TypeAssessment},
			{Verb: c.VerbIndicates, From: c.TypeFinding, To: "Incident"},
		} {
			e := edgeFor(c.Triple{Verb: c.VerbIndicates, From: c.TypeFinding, To: c.TypeRisk})
			e.Verb, e.From.Type, e.To.Type = tr.Verb, tr.From, tr.To
			requireRefused(t, e.Validate(), "vocabulary")
		}
	})
	t.Run("ControlTest evidenced_by Evidence without its result", func(t *testing.T) {
		e := edgeFor(c.Triple{Verb: c.VerbEvidencedBy, From: c.TypeControlTest, To: c.TypeEvidence})
		e.Qualifier = nil
		requireRefused(t, e.Validate(), "qualifier")
		e.Qualifier = ptr(uuid.Nil)
		requireRefused(t, e.Validate(), "qualifier")
	})
	t.Run("Finding raised_by ControlTest without its result", func(t *testing.T) {
		e := edgeFor(c.Triple{Verb: c.VerbRaisedBy, From: c.TypeFinding, To: c.TypeControlTest})
		e.Qualifier = nil
		requireRefused(t, e.Validate(), "qualifier")
	})
	t.Run("a qualifier where the verb takes none", func(t *testing.T) {
		for _, tr := range d5 {
			e := edgeFor(tr)
			if e.Qualifier != nil {
				continue
			}
			e.Qualifier = ptr(uuid.New())
			requireRefused(t, e.Validate(), "qualifier")
		}
	})
	t.Run("an attribute the triple does not carry", func(t *testing.T) {
		e := edgeFor(c.Triple{Verb: c.VerbMitigatedBy, From: c.TypeRisk, To: c.TypeControl})
		e.Attributes.MatchConfidence = ptr(0.5)
		requireRefused(t, e.Validate(), "attributes.match_confidence")
		e = edgeFor(c.Triple{Verb: c.VerbImplementedBy, From: c.TypeControl, To: c.TypeControlImplementation})
		e.Attributes.Note = ptr("—")
		requireRefused(t, e.Validate(), "attributes.note")
	})
	t.Run("maps_to without rationale", func(t *testing.T) {
		e := edgeFor(c.Triple{Verb: c.VerbMapsTo, From: c.TypeRequirement, To: c.TypeRequirement})
		e.Attributes.Rationale = nil
		requireRefused(t, e.Validate(), "rationale")
	})
	t.Run("attribute values are closed", func(t *testing.T) {
		e := validEdge()
		e.Attributes.Coverage = ptr(c.Coverage("most"))
		requireRefused(t, e.Validate(), "coverage")
		e = edgeFor(c.Triple{Verb: c.VerbMapsTo, From: c.TypeRequirement, To: c.TypeRequirement})
		e.Attributes.Origin = ptr(c.CrosswalkOrigin("ai"))
		requireRefused(t, e.Validate(), "origin")
	})
	t.Run("an attribute key outside D5, from JSON", func(t *testing.T) {
		var a c.EdgeAttributes
		requireRefused(t, json.Unmarshal([]byte(`{"note":"x","weight":3}`), &a), "attributes")
		require.NoError(t, json.Unmarshal([]byte(`{"note":"x"}`), &a))
	})
	t.Run("an edge's own source and confidence follow D3", func(t *testing.T) {
		e := validEdge()
		e.Confidence = 0.71 // user_entered, no review
		requireRefused(t, e.Validate(), "confidence")
		e = validEdge()
		e.Source = "manual"
		requireRefused(t, e.Validate(), "source")
	})
}

// TestCanonical_MapsToIsSymmetric: A→B and B→A are one edge; every other verb
// keeps its direction.
func TestCanonical_MapsToIsSymmetric(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	forward := edgeFor(c.Triple{Verb: c.VerbMapsTo, From: c.TypeRequirement, To: c.TypeRequirement})
	forward.From.ID, forward.To.ID = a, b
	backward := forward
	backward.From.ID, backward.To.ID = b, a
	assert.Equal(t, forward.Key(), backward.Key())
	assert.True(t, c.VerbMapsTo.IsSymmetric())

	for _, tr := range d5 {
		if tr.Verb != c.VerbMapsTo {
			assert.False(t, tr.Verb.IsSymmetric(), tr.Verb)
		}
	}
	directed := validEdge()
	flipped := directed
	flipped.From.ID, flipped.To.ID = directed.To.ID, directed.From.ID
	assert.NotEqual(t, directed.Key(), flipped.Key())

	t.Run("the qualifier is part of the identity", func(t *testing.T) {
		e1 := edgeFor(c.Triple{Verb: c.VerbEvidencedBy, From: c.TypeControlTest, To: c.TypeEvidence})
		e2 := e1
		e2.Qualifier = ptr(uuid.New())
		assert.NotEqual(t, e1.Key(), e2.Key())
	})
}
