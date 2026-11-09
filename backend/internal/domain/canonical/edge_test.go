// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical_test

import (
	"encoding/json"
	"strings"
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
		requireRefused(t, json.Unmarshal([]byte(`{"note":"x","weight":3}`), &a), "unknown key")
		require.NoError(t, json.Unmarshal([]byte(`{"note":"x"}`), &a))
	})
	t.Run("attribute keys are matched exactly, once", func(t *testing.T) {
		for name, raw := range map[string]string{
			"upper-case spelling":      `{"NOTE":"https://u:p@h/x"}`,
			"two spellings of one key": `{"NOTE":"https://u:p@h/x","note":"ok"}`,
			"duplicate key":            `{"note":"ok","note":"https://u:p@h/x"}`,
			"mixed case":               `{"Rationale":"x"}`,
		} {
			t.Run(name, func(t *testing.T) {
				var a c.EdgeAttributes
				err := json.Unmarshal([]byte(raw), &a)
				requireRefused(t, err, "attributes")
				assert.NotContains(t, err.Error(), "NOTE")
				assert.NotContains(t, err.Error(), "u:p@h")
			})
		}
		var a c.EdgeAttributes
		require.NoError(t, json.Unmarshal([]byte(`{"match_candidates":[{"Asset":"x","asset":"y"}]}`), &a),
			"match_candidates is opaque JSON (A7): its inner keys are not ours to close")
	})
	t.Run("the key walk refuses JSON nested past 64 levels", func(t *testing.T) {
		deep := `{"match_candidates":` + strings.Repeat("[", 100) + strings.Repeat("]", 100) + `}`
		var a c.EdgeAttributes
		requireRefused(t, a.UnmarshalJSON([]byte(deep)), "nested too deeply")
		var p c.Provenance
		requireRefused(t, p.UnmarshalJSON([]byte(`{"derived_from":`+strings.Repeat("[", 100)+strings.Repeat("]", 100)+`}`)), "nested too deeply")
		shallow := `{"match_candidates":` + strings.Repeat("[", 10) + strings.Repeat("]", 10) + `}`
		require.NoError(t, a.UnmarshalJSON([]byte(shallow)))
	})
	t.Run("the vocabulary refusal echoes nothing it was given", func(t *testing.T) {
		e := validEdge()
		e.Verb = "owns\nlevel=INFO msg=forged"
		e.From.Type = "Injected\r\nType"
		err := e.Validate()
		requireRefused(t, err, "vocabulary")
		assert.NotContains(t, err.Error(), "forged")
		assert.NotContains(t, err.Error(), "Injected")
	})
	t.Run("attribute labels are capped and never credential-shaped", func(t *testing.T) {
		long := strings.Repeat("n", 256)
		for name, set := range map[string]func(*c.EdgeAttributes, string){
			"match_method":                     func(a *c.EdgeAttributes, v string) { a.MatchMethod = &v },
			"asset_criticality_at_observation": func(a *c.EdgeAttributes, v string) { a.AssetCriticalityAtObservation = &v },
		} {
			for _, value := range []string{long, "https://h/o?X-Amz-Signature=abc", "svc:pw@db.internal"} {
				e := edgeFor(c.Triple{Verb: c.VerbAffects, From: c.TypeFinding, To: c.TypeAsset})
				set(&e.Attributes, value)
				requireRefused(t, e.Validate(), "attributes."+name)
			}
		}
		e := edgeFor(c.Triple{Verb: c.VerbAffects, From: c.TypeFinding, To: c.TypeAsset})
		e.Attributes.MatchCandidates = json.RawMessage(`"` + strings.Repeat("x", 16<<10) + `"`)
		requireRefused(t, e.Validate(), "match_candidates")
	})
	t.Run("note and rationale are uncapped prose; only URL tokens are checked (A9)", func(t *testing.T) {
		setters := map[string]func(*c.Edge, string){
			"note": func(e *c.Edge, v string) {
				*e = edgeFor(c.Triple{Verb: c.VerbMitigatedBy, From: c.TypeRisk, To: c.TypeControl})
				e.Attributes.Note = &v
			},
			"rationale": func(e *c.Edge, v string) {
				*e = edgeFor(c.Triple{Verb: c.VerbMapsTo, From: c.TypeRequirement, To: c.TypeRequirement})
				e.Attributes.Rationale = &v
			},
		}
		for field, set := range setters {
			for _, prose := range []string{
				strings.Repeat("a", 1000),
				"Note: escalate to alice@corp.com",
				"Voir: la revue trimestrielle, contact rssi@banque.cm",
				"Cf. https://wiki.example.com/grc/access-review?page=2 for the procedure.",
				"see (https://wiki.corp/page?id=42) for context",
				"\"https://wiki.corp/page?id=42\"",
				"rotate creds for svc:pw@db.internal", // no "://": prose, not a URL (A9)
				"tracked in https://jira.corp/x?ticket=OPS-12",
			} {
				var e c.Edge
				set(&e, prose)
				require.NoError(t, e.Validate(), "%s: %q", field, prose[:min(len(prose), 40)])
			}
			for _, secret := range []string{
				"Evidence at https://x/y?X-Amz-Signature=abc, see above.",
				"(https://s3.amazonaws.com/b/o?X-Amz-Signature=abc)",
			} {
				var e c.Edge
				set(&e, secret)
				err := e.Validate()
				requireRefused(t, err, "attributes."+field)
				assert.NotContains(t, err.Error(), "X-Amz")
			}
		}
	})
	t.Run("maps_to from a requirement to itself", func(t *testing.T) {
		e := edgeFor(c.Triple{Verb: c.VerbMapsTo, From: c.TypeRequirement, To: c.TypeRequirement})
		e.To.ID = e.From.ID
		requireRefused(t, e.Validate(), "to.id")
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
