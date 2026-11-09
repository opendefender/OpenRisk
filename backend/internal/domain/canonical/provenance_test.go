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

	"github.com/opendefender/openrisk/internal/domain/canonical"
)

func TestCanonical_Provenance_Success(t *testing.T) {
	t.Run("every D3 key", func(t *testing.T) {
		raw := `{
			"method": "scan",
			"actor": {"kind": "system", "id": "nessus-ingest"},
			"via": "vuln_integrations:7f3c",
			"ingest_id": "scan-job-4471",
			"derived_from": [{"type": "Finding", "id": "` + uuid.NewString() + `"}],
			"external_refs": [{"scheme": "cve", "value": "CVE-2024-3094"}],
			"rule": "KEV-listed and internet-facing",
			"confidence_rule": "confidence/1",
			"raw_ref": "vulnerabilities.raw_data"
		}`
		p, err := canonical.ParseProvenance([]byte(raw), canonical.SourceNessus)
		require.NoError(t, err)
		assert.Equal(t, canonical.MethodScan, p.Method)
	})
	t.Run("vendor_questionnaire's ADR 0004 keys", func(t *testing.T) {
		raw := `{
			"channel": "public_link", "template_id": "` + uuid.NewString() + `", "template_version": 3,
			"submitted_with_token_id": "` + uuid.NewString() + `",
			"method": "questionnaire",
			"actor": {"kind": "vendor_contact", "id": "vendor_contact:` + uuid.NewString() + `"},
			"confidence_rule": "confidence/1"
		}`
		p, err := canonical.ParseProvenance([]byte(raw), canonical.SourceVendorQuestionnaire)
		require.NoError(t, err)
		require.NotNil(t, p.TemplateVersion)
		assert.Equal(t, 3, *p.TemplateVersion)
		assert.Equal(t, []string{"channel", "template_id", "template_version", "submitted_with_token_id"},
			canonical.ProvenanceAllowlist(canonical.SourceVendorQuestionnaire))
		assert.Empty(t, canonical.ProvenanceAllowlist(canonical.SourceOpenRiskUser))
	})
	t.Run("a derived record naming its inputs", func(t *testing.T) {
		p := provenanceFor(canonical.SourceOpenRiskDerived)
		require.NoError(t, p.Validate(canonical.SourceOpenRiskDerived))
		p.ExternalRefs = nil
		p.DerivedFrom = []canonical.RecordRef{{Type: canonical.TypeAssessment, ID: uuid.New()}}
		require.NoError(t, p.Validate(canonical.SourceOpenRiskDerived))
	})
}

// TestCanonical_ProvenanceRefusesUnknownKeys is ADR 0008 D3 / #1240 criterion 7.
func TestCanonical_ProvenanceRefusesUnknownKeys(t *testing.T) {
	actor := `"actor": {"kind": "user", "id": "` + userAlice.String() + `"}`
	base := `"method": "manual_entry", ` + actor + `, "confidence_rule": "confidence/1"`
	for name, raw := range map[string]string{
		"a key outside D3":                    `{` + base + `, "ip_address": "203.0.113.7"}`,
		"a user agent":                        `{` + base + `, "user_agent": "Mozilla/5.0"}`,
		"a key nested in actor":               `{"method": "manual_entry", "actor": {"kind": "user", "id": "` + userAlice.String() + `", "email": "a@example.com"}, "confidence_rule": "confidence/1"}`,
		"a key nested in derived_from":        `{` + base + `, "derived_from": [{"type": "Finding", "id": "` + uuid.NewString() + `", "tenant_id": "x"}]}`,
		"a vendor key on a non-vendor source": `{` + base + `, "channel": "public_link"}`,
		"another vendor key":                  `{` + base + `, "template_version": 3}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := canonical.ParseProvenance([]byte(raw), canonical.SourceOpenRiskUser)
			requireRefused(t, err, "provenance")
		})
	}
	t.Run("an unknown key through the envelope's JSON", func(t *testing.T) {
		raw, err := json.Marshal(validEvidence())
		require.NoError(t, err)
		patched := strings.Replace(string(raw), `"confidence_rule":"confidence/1"`, `"confidence_rule":"confidence/1","api_key":"x"`, 1)
		require.NotEqual(t, string(raw), patched)
		var e canonical.Evidence
		requireRefused(t, json.Unmarshal([]byte(patched), &e), "provenance")
	})
	t.Run("a vendor key set in Go on a non-vendor source", func(t *testing.T) {
		p := userProvenance(userAlice)
		p.TemplateID = ptr(uuid.New())
		requireRefused(t, p.Validate(canonical.SourceOpenRiskUser), "provenance.template_id")
	})
}

func TestCanonical_Provenance_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		source canonical.Source
		field  string
		mutate func(*canonical.Provenance)
	}{
		{"no method", canonical.SourceOpenRiskUser, "method", func(p *canonical.Provenance) { p.Method = "" }},
		{"method outside the enum", canonical.SourceOpenRiskUser, "method", func(p *canonical.Provenance) { p.Method = "upload" }},
		{"no actor", canonical.SourceOpenRiskUser, "actor.id", func(p *canonical.Provenance) { p.Actor = canonical.Actor{} }},
		{"actor kind outside the enum", canonical.SourceOpenRiskUser, "actor.kind", func(p *canonical.Provenance) { p.Actor.Kind = "service" }},
		{"a user actor that is not a user id", canonical.SourceOpenRiskUser, "actor.id", func(p *canonical.Provenance) { p.Actor.ID = "alice@example.com" }},
		{"no confidence_rule", canonical.SourceOpenRiskUser, "confidence_rule", func(p *canonical.Provenance) { p.ConfidenceRule = "" }},
		{"an unknown confidence_rule", canonical.SourceOpenRiskUser, "confidence_rule", func(p *canonical.Provenance) { p.ConfidenceRule = "confidence/2" }},
		{"via as a URL", canonical.SourceOpenRiskUser, "via", func(p *canonical.Provenance) { p.Via = ptr("https://jira.example.com") }},
		{"derived_from naming a non-entity", canonical.SourceOpenRiskUser, "derived_from[0].type", func(p *canonical.Provenance) {
			p.DerivedFrom = []canonical.RecordRef{{Type: "Framework", ID: uuid.New()}}
		}},
		{"derived_from with a nil id", canonical.SourceOpenRiskUser, "derived_from[0].id", func(p *canonical.Provenance) {
			p.DerivedFrom = []canonical.RecordRef{{Type: canonical.TypeFinding}}
		}},
		{"external_ref without a value", canonical.SourceOpenRiskUser, "external_refs[0].value", func(p *canonical.Provenance) {
			p.ExternalRefs = []canonical.ExternalRef{{Scheme: "cve"}}
		}},
		{"vendor_questionnaire by a user", canonical.SourceVendorQuestionnaire, "method", func(p *canonical.Provenance) {}},
		{"vendor contact id without its assessment", canonical.SourceVendorQuestionnaire, "actor.id", func(p *canonical.Provenance) {
			*p = provenanceFor(canonical.SourceVendorQuestionnaire)
			p.Actor.ID = "vendor_contact:"
		}},
		{"a source outside the registry", "manual", "source", func(p *canonical.Provenance) {}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := userProvenance(userAlice)
			tc.mutate(&p)
			requireRefused(t, p.Validate(tc.source), tc.field)
		})
	}
	t.Run("a derived record with neither derived_from nor external_refs", func(t *testing.T) {
		for _, s := range []canonical.Source{canonical.SourceOpenRiskDerived, canonical.SourceOpenRiskStarter, canonical.SourceOpenRiskAI} {
			p := provenanceFor(s)
			p.ExternalRefs, p.DerivedFrom = nil, nil
			requireRefused(t, p.Validate(s), "derived_from or external_refs")
		}
	})
}

// TestCanonical_ProvenanceRefusesCredentialShapedValues is ADR 0008 D3 /
// #1240 criterion 7. ABSOLUTE RULE 6.
func TestCanonical_ProvenanceRefusesCredentialShapedValues(t *testing.T) {
	const secret = "s3cr3tVALUE"
	shaped := map[string]string{
		"userinfo":                     "https://admin:" + secret + "@jira.example.com/rest",
		"username only":                "ftp://" + secret + "@files.example.com/",
		"userinfo without //":          "svc:" + secret + "@db.internal",
		"S3 presigned":                 "https://b.s3.amazonaws.com/o.pdf?X-Amz-Algorithm=AWS4&X-Amz-Signature=" + secret,
		"S3 presigned, lower case":     "https://b.s3.amazonaws.com/o.pdf?x-amz-signature=" + secret,
		"GCS signed":                   "https://storage.googleapis.com/b/o?X-Goog-Signature=" + secret,
		"Azure SAS":                    "https://a.blob.core.windows.net/c/o?sv=2022&sig=" + secret,
		"sig in upper case":            "https://a.example.com/o?SIG=" + secret,
		"token":                        "https://a.example.com/o?token=" + secret,
		"access_token":                 "https://a.example.com/o?access_token=" + secret,
		"access_token in the fragment": "https://a.example.com/cb#access_token=" + secret,
		"OAuth code":                   "https://a.example.com/cb?state=1&code=" + secret,
		"api key":                      "https://a.example.com/o?apikey=" + secret,
		"api_key":                      "https://a.example.com/o?api_key=" + secret,
		"key":                          "https://maps.example.com/o?key=" + secret,
		"a *_token variant":            "https://a.example.com/o?session_token=" + secret,
		"encoded parameter name":       "https://a.example.com/o?%74oken=" + secret,
		"query without a scheme":       "/download?signature=" + secret,
		"unparseable URL":              "https://a.example.com/%zz?x=" + secret,
	}

	t.Run("in every provenance string", func(t *testing.T) {
		setters := map[string]func(*canonical.Provenance, string){
			"via":       func(p *canonical.Provenance, v string) { p.Via = &v },
			"ingest_id": func(p *canonical.Provenance, v string) { p.IngestID = &v },
			"rule":      func(p *canonical.Provenance, v string) { p.Rule = &v },
			"raw_ref":   func(p *canonical.Provenance, v string) { p.RawRef = &v },
			"external_refs[0].value": func(p *canonical.Provenance, v string) {
				p.ExternalRefs = []canonical.ExternalRef{{Scheme: "url", Value: v}}
			},
			"actor.id": func(p *canonical.Provenance, v string) { p.Actor = canonical.Actor{Kind: canonical.ActorSystem, ID: v} },
		}
		for field, set := range setters {
			for name, value := range shaped {
				t.Run(field+"/"+name, func(t *testing.T) {
					p := userProvenance(userAlice)
					set(&p, value)
					err := p.Validate(canonical.SourceOpenRiskUser)
					requireRefused(t, err, field)
					assert.NotContains(t, err.Error(), secret, "a refusal must never echo the credential")
				})
			}
		}
		t.Run("channel", func(t *testing.T) {
			p := provenanceFor(canonical.SourceVendorQuestionnaire)
			p.Channel = ptr(shaped["token"])
			requireRefused(t, p.Validate(canonical.SourceVendorQuestionnaire), "channel")
		})
	})

	t.Run("in Evidence.external_url", func(t *testing.T) {
		for name, value := range shaped {
			if !strings.HasPrefix(value, "http") {
				continue
			}
			t.Run(name, func(t *testing.T) {
				e := validEvidence()
				e.ExternalURL = ptr(value)
				err := e.Validate()
				requireRefused(t, err, "external_url")
				assert.NotContains(t, err.Error(), secret)
			})
		}
	})

	t.Run("a string over 255 characters", func(t *testing.T) {
		p := userProvenance(userAlice)
		p.Rule = ptr(strings.Repeat("r", 256))
		requireRefused(t, p.Validate(canonical.SourceOpenRiskUser), "255")
		p.Rule = ptr(strings.Repeat("é", 255))
		require.NoError(t, p.Validate(canonical.SourceOpenRiskUser), "255 characters is the cap, counted in characters")
		p = userProvenance(userAlice)
		p.ExternalRefs = []canonical.ExternalRef{{Scheme: "cve", Value: strings.Repeat("9", 256)}}
		requireRefused(t, p.Validate(canonical.SourceOpenRiskUser), "external_refs[0].value")
	})

	t.Run("ordinary values pass", func(t *testing.T) {
		p := userProvenance(userAlice)
		p.Rule = ptr("KEV-listed, CVSS >= 9.0 and asset is internet-facing")
		p.RawRef = ptr("https://scanner.example.com/reports/4471?page=2#host-12")
		p.ExternalRefs = []canonical.ExternalRef{{Scheme: "cve", Value: "CVE-2024-3094"}}
		require.NoError(t, p.Validate(canonical.SourceOpenRiskUser))
		e := validEvidence()
		e.ExternalURL = ptr("https://sharepoint.example.com/sites/grc/Policy.pdf?web=1")
		require.NoError(t, e.Validate())
	})
}
