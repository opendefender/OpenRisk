// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

var allEvidenceSources = []domain.EvidenceSource{
	domain.EvidenceSourceManual, domain.EvidenceSourceIntegration,
	domain.EvidenceSourceScanner, domain.EvidenceSourceAutomation,
}

// AC5: every legacy source maps to a method of the closed vocabulary. The same
// mapping is written in migrations/0088 and prepare.go; the Postgres test in
// internal/infrastructure/database asserts those agree with this function.
func TestCollectionMethodForSource_MapsEverySource(t *testing.T) {
	want := map[domain.EvidenceSource]domain.CollectionMethod{
		domain.EvidenceSourceManual:      domain.CollectionManualUpload,
		domain.EvidenceSourceIntegration: domain.CollectionAPIPull,
		domain.EvidenceSourceScanner:     domain.CollectionAutomatedCollect,
		domain.EvidenceSourceAutomation:  domain.CollectionAutomatedCollect,
	}
	for _, src := range allEvidenceSources {
		got := domain.CollectionMethodForSource(src)
		if got != want[src] {
			t.Errorf("source %q: method %q, want %q", src, got, want[src])
		}
		if _, err := domain.ParseCollectionMethod(string(got)); err != nil {
			t.Errorf("source %q maps to %q, outside the vocabulary: %v", src, got, err)
		}
	}
	if len(want) != len(allEvidenceSources) {
		t.Fatal("test table out of step with the sources")
	}
}

func TestParseCollectionMethod_RejectsOutsideTheFour(t *testing.T) {
	for _, m := range domain.CollectionMethods() {
		if _, err := domain.ParseCollectionMethod(string(m)); err != nil {
			t.Errorf("%q must parse: %v", m, err)
		}
	}
	for _, bad := range []string{"", "manual", "Manual_Upload", "carrier_pigeon"} {
		_, err := domain.ParseCollectionMethod(bad)
		if !errors.Is(err, domain.ErrValidation) {
			t.Errorf("%q: want ErrValidation, got %v", bad, err)
		}
	}
}

// AC6: the confidence table. Each method's base confidence is the confidence/1
// default of its reference canonical class (so it never exceeds that class's
// ceiling) and manual_upload is strictly lower than automated_collector.
func TestMethodConfidence_FollowsTheDocumentedTable(t *testing.T) {
	ref := map[domain.CollectionMethod]canonical.Source{
		domain.CollectionManualUpload:     canonical.SourceOpenRiskUser,
		domain.CollectionAutomatedCollect: canonical.SourceOpenRiskScanner,
		domain.CollectionAPIPull:          canonical.SourceJira,
		domain.CollectionAttestation:      canonical.SourceVendorQuestionnaire,
	}
	for m, src := range ref {
		def, err := canonical.DefaultConfidence(src, canonical.ReviewContext{})
		if err != nil {
			t.Fatal(err)
		}
		ceil, err := canonical.ConfidenceCeiling(src, canonical.ReviewContext{})
		if err != nil {
			t.Fatal(err)
		}
		if got := m.BaseConfidence(); got != def || got > ceil {
			t.Errorf("%s: base confidence %v, class default %v, ceiling %v", m, got, def, ceil)
		}
	}
	want := map[domain.CollectionMethod]float64{
		domain.CollectionAutomatedCollect: 0.80, domain.CollectionAPIPull: 0.60,
		domain.CollectionAttestation: 0.50, domain.CollectionManualUpload: 0.70,
	}
	for m, c := range want {
		if m.BaseConfidence() != c {
			t.Errorf("%s: %v, documented %v", m, m.BaseConfidence(), c)
		}
	}
	if domain.CollectionManualUpload.BaseConfidence() >= domain.CollectionAutomatedCollect.BaseConfidence() {
		t.Fatal("manual_upload must be strictly lower than automated_collector")
	}
}

// AC3: an unsigned record has lower assurance than the same record signed.
func TestAssurance_UnsignedIsLowerThanSigned(t *testing.T) {
	want := map[domain.CollectionMethod][2]domain.EvidenceAssurance{ // {signed, unsigned}
		domain.CollectionAutomatedCollect: {domain.AssuranceHigh, domain.AssuranceSubstantial},
		domain.CollectionAPIPull:          {domain.AssuranceHigh, domain.AssuranceSubstantial},
		domain.CollectionAttestation:      {domain.AssuranceSubstantial, domain.AssuranceModerate},
		domain.CollectionManualUpload:     {domain.AssuranceModerate, domain.AssuranceLow},
	}
	for m, w := range want {
		signed, unsigned := domain.AssuranceFor(m, true), domain.AssuranceFor(m, false)
		if signed != w[0] || unsigned != w[1] {
			t.Errorf("%s: signed %s unsigned %s, want %v", m, signed, unsigned, w)
		}
		if domain.AssuranceRank(unsigned) >= domain.AssuranceRank(signed) {
			t.Errorf("%s: unsigned must rank below signed", m)
		}
	}
	// Floor: an unknown method is already low and stays low.
	if domain.AssuranceFor("", false) != domain.AssuranceLow {
		t.Fatal("assurance never drops below low")
	}
}

func TestDeriveIntegrity_ShowsSignatureAbsentAndHashState(t *testing.T) {
	e := &domain.Evidence{CollectionMethod: domain.CollectionManualUpload, FileRef: "k"}
	e.DeriveIntegrity()
	if e.Signed || e.Integrity != domain.IntegrityUnhashed {
		t.Fatalf("legacy file row: %+v", e)
	}
	e.ContentHash, e.Signature = strings.Repeat("a", 64), "c2ln"
	e.DeriveIntegrity()
	if !e.Signed || e.Integrity != domain.IntegrityHashed || e.Assurance != domain.AssuranceModerate {
		t.Fatalf("hashed and signed: %+v", e)
	}
	n := &domain.Evidence{}
	n.DeriveIntegrity()
	if n.Integrity != domain.IntegrityNone {
		t.Fatalf("no file: %q", n.Integrity)
	}
}

func TestContentHashOf_IsSHA256Hex(t *testing.T) {
	sum := sha256.Sum256([]byte("abc"))
	if got := domain.ContentHashOf([]byte("abc")); got != hex.EncodeToString(sum[:]) || len(got) != 64 {
		t.Fatalf("got %q", got)
	}
}

// AC4: observed_from <= observed_to <= collected_at.
func TestValidateObservation(t *testing.T) {
	c := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	at := func(h int) *time.Time { v := c.Add(time.Duration(h) * time.Hour); return &v }
	cases := []struct {
		name     string
		from, to *time.Time
		ok       bool
	}{
		{"point in time", at(0), at(0), true},
		{"a period before collection", at(-48), at(-1), true},
		{"from after to", at(-1), at(-2), false},
		{"to after collected_at", at(-1), at(1), false},
		{"missing bound", nil, at(0), false},
	}
	for _, tc := range cases {
		e := &domain.Evidence{CollectedAt: c, ObservedFrom: tc.from, ObservedTo: tc.to}
		err := e.ValidateObservation()
		if tc.ok && err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
		if !tc.ok && !errors.Is(err, domain.ErrValidation) {
			t.Errorf("%s: want ErrValidation, got %v", tc.name, err)
		}
	}
}

// AC7 and the collector identity rules.
func TestValidateCollection_CollectorRules(t *testing.T) {
	sig := base64.StdEncoding.EncodeToString([]byte("sig"))
	big := base64.StdEncoding.EncodeToString(make([]byte, domain.MaxSignatureBytes+1))
	cases := []struct {
		name string
		e    domain.Evidence
		ok   bool
	}{
		{"manual, no collector", domain.Evidence{CollectionMethod: domain.CollectionManualUpload}, true},
		{"automated without ref", domain.Evidence{CollectionMethod: domain.CollectionAutomatedCollect}, false},
		{"api_pull without ref", domain.Evidence{CollectionMethod: domain.CollectionAPIPull, CollectorKind: domain.CollectorConnector}, false},
		{"automated with ref", domain.Evidence{CollectionMethod: domain.CollectionAPIPull, CollectorKind: domain.CollectorConnector, CollectorRef: "okta:prod/users"}, true},
		{"ref with a space", domain.Evidence{CollectionMethod: domain.CollectionManualUpload, CollectorKind: domain.CollectorUser, CollectorRef: "a b"}, false},
		{"ref with a quote", domain.Evidence{CollectionMethod: domain.CollectionManualUpload, CollectorKind: domain.CollectorUser, CollectorRef: `a"b`}, false},
		{"ref over 128", domain.Evidence{CollectionMethod: domain.CollectionManualUpload, CollectorKind: domain.CollectorUser, CollectorRef: strings.Repeat("a", 129)}, false},
		{"kind outside the three", domain.Evidence{CollectionMethod: domain.CollectionManualUpload, CollectorKind: "robot", CollectorRef: "x"}, false},
		{"signature without algorithm", domain.Evidence{CollectionMethod: domain.CollectionManualUpload, Signature: sig}, false},
		{"signature with algorithm", domain.Evidence{CollectionMethod: domain.CollectionManualUpload, Signature: sig, SignatureAlgorithm: "ed25519"}, true},
		{"signature not base64", domain.Evidence{CollectionMethod: domain.CollectionManualUpload, Signature: "***", SignatureAlgorithm: "x"}, false},
		{"signature over 8 KiB", domain.Evidence{CollectionMethod: domain.CollectionManualUpload, Signature: big, SignatureAlgorithm: "x"}, false},
		{"algorithm over 32", domain.Evidence{CollectionMethod: domain.CollectionManualUpload, SignatureAlgorithm: strings.Repeat("a", 33)}, false},
	}
	for _, tc := range cases {
		err := tc.e.ValidateCollection()
		if tc.ok && err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
		if !tc.ok && !errors.Is(err, domain.ErrValidation) {
			t.Errorf("%s: want ErrValidation, got %v", tc.name, err)
		}
	}
}

func TestApplyRecordDefaults(t *testing.T) {
	user := uuid.New()
	c := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	e := &domain.Evidence{Source: domain.EvidenceSourceIntegration, CollectedAt: c, CollectedBy: &user, CollectorRef: "okta"}
	e.ApplyRecordDefaults()
	if e.CollectionMethod != domain.CollectionAPIPull || e.CollectorKind != domain.CollectorConnector || e.CollectorRef != "okta" {
		t.Fatalf("integration: %+v", e)
	}
	if e.ObservedFrom == nil || !e.ObservedFrom.Equal(c) || !e.ObservedTo.Equal(c) {
		t.Fatalf("observation must default to collected_at: %v %v", e.ObservedFrom, e.ObservedTo)
	}
	m := &domain.Evidence{Source: domain.EvidenceSourceManual, CollectedAt: c, CollectedBy: &user}
	m.ApplyRecordDefaults()
	if m.CollectionMethod != domain.CollectionManualUpload || m.CollectorKind != domain.CollectorUser || m.CollectorRef != user.String() {
		t.Fatalf("manual: %+v", m)
	}
}

func TestNewEvidenceIntegrityError_IsATypedConflict(t *testing.T) {
	err := domain.NewEvidenceIntegrityError(uuid.New())
	if !errors.Is(err, domain.ErrConflict) || domain.HTTPStatusFromError(err) != http.StatusConflict {
		t.Fatalf("want ErrConflict/409, got %v / %d", err, domain.HTTPStatusFromError(err))
	}
	if domain.ReasonFromError(err) != domain.ReasonEvidenceIntegrity || domain.ReasonFromError(errors.New("x")) != "" {
		t.Fatal("stable reason code")
	}
	if strings.Contains(domain.MessageFromError(err), "sha") {
		t.Fatal("message must carry no hash")
	}
}

func TestAuditActionIntegrityAlert_FitsTheColumn(t *testing.T) {
	if n := len(domain.AuditActionIntegrityAlert); n > 24 || n == 0 {
		t.Fatalf("action length %d does not fit varchar(24)", n)
	}
}
