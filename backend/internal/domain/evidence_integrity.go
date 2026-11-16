// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// HashAlgorithmSHA256 is the only content-hash algorithm; it is stored beside
// the hash so a later algorithm can coexist with it.
const HashAlgorithmSHA256 = "sha256"

// ContentHashOf returns the lower-case hex SHA-256 of b.
func ContentHashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// CollectionMethod is how a proof artifact reached the register. It is a
// different axis from EvidenceSource and from the canonical source class: the
// source says what system it came from, the method says by what act it was
// collected (ADR 0008 A11).
type CollectionMethod string

const (
	CollectionManualUpload     CollectionMethod = "manual_upload"
	CollectionAutomatedCollect CollectionMethod = "automated_collector"
	CollectionAPIPull          CollectionMethod = "api_pull"
	CollectionAttestation      CollectionMethod = "attestation"
)

// CollectionMethods lists the closed vocabulary.
func CollectionMethods() []CollectionMethod {
	return []CollectionMethod{CollectionManualUpload, CollectionAutomatedCollect, CollectionAPIPull, CollectionAttestation}
}

// CollectionMethodForSource maps a legacy source to a method. It is the mapping
// the migration backfill uses for rows that predate #554. It is NOT applied to
// a human (HTTP) request, which derives manual_upload when it omits the method:
// a source label sent by a client must not imply an automated collection. An
// attestation is only ever explicit: no source implies one.
func CollectionMethodForSource(s EvidenceSource) CollectionMethod {
	switch s {
	case EvidenceSourceIntegration:
		return CollectionAPIPull
	case EvidenceSourceScanner, EvidenceSourceAutomation:
		return CollectionAutomatedCollect
	default:
		return CollectionManualUpload
	}
}

// ParseCollectionMethod validates a method (empty is not accepted here: the
// caller derives the default from the source first).
func ParseCollectionMethod(s string) (CollectionMethod, error) {
	switch m := CollectionMethod(s); m {
	case CollectionManualUpload, CollectionAutomatedCollect, CollectionAPIPull, CollectionAttestation:
		return m, nil
	default:
		return "", NewValidationError(fmt.Sprintf("invalid collection method: %q", s))
	}
}

// IsAutomated reports whether a machine collected the proof; such a record must
// name its collector (collector_ref).
func (m CollectionMethod) IsAutomated() bool {
	return m == CollectionAutomatedCollect || m == CollectionAPIPull
}

// CollectorKind says what sort of identity collected the proof.
type CollectorKind string

const (
	CollectorUser      CollectorKind = "user"
	CollectorConnector CollectorKind = "connector"
	CollectorJob       CollectorKind = "job"
)

// ParseCollectorKind validates a kind.
func ParseCollectorKind(s string) (CollectorKind, error) {
	switch k := CollectorKind(s); k {
	case CollectorUser, CollectorConnector, CollectorJob:
		return k, nil
	default:
		return "", NewValidationError(fmt.Sprintf("invalid collector kind: %q", s))
	}
}

// defaultCollectorKind is the kind assumed for an automated record that names a
// collector but not its kind.
func defaultCollectorKind(m CollectionMethod) CollectorKind {
	if m == CollectionAPIPull {
		return CollectorConnector
	}
	return CollectorJob
}

// collectorRefPattern is the shape of collector_ref: an identifier, never a
// credential. No whitespace, no quotes, no query-string characters.
var collectorRefPattern = regexp.MustCompile(`^[A-Za-z0-9._:@/-]{1,128}$`)

// Signature limits: base64 on the wire, at most 8 KiB once decoded.
const (
	MaxSignatureBytes        = 8 * 1024
	MaxSignatureAlgorithmLen = 32
)

// EvidenceIntegrity is the derived, read-only state of the content hash.
type EvidenceIntegrity string

const (
	// IntegrityHashed: a hash was taken at ingest. The match itself is only
	// established by a download, which reports it in a response header.
	IntegrityHashed EvidenceIntegrity = "hashed"
	// IntegrityUnhashed: the row holds (or claims) a file but no hash — a legacy
	// row, or one whose hash was blanked. Served, flagged unverified.
	IntegrityUnhashed EvidenceIntegrity = "unhashed"
	// IntegrityNone: the row holds no file (a link or a statement).
	IntegrityNone EvidenceIntegrity = "none"
)

// EvidenceAssurance orders how much an auditor can lean on a collection act.
type EvidenceAssurance string

const (
	AssuranceHigh        EvidenceAssurance = "high"
	AssuranceSubstantial EvidenceAssurance = "substantial"
	AssuranceModerate    EvidenceAssurance = "moderate"
	AssuranceLow         EvidenceAssurance = "low"
	AssuranceMinimal     EvidenceAssurance = "minimal"
)

var assuranceOrder = []EvidenceAssurance{AssuranceMinimal, AssuranceLow, AssuranceModerate, AssuranceSubstantial, AssuranceHigh}

// AssuranceRank is 0 (minimal) to 4 (high); unknown reads as minimal.
func AssuranceRank(a EvidenceAssurance) int {
	for i, v := range assuranceOrder {
		if v == a {
			return i
		}
	}
	return 0
}

// methodProfile is one row of the documented table (ADR 0008 A11, D-115).
// Confidence is the BASE, unreviewed value: it equals the confidence/1 default
// of the canonical class the method is closest to, and so never exceeds that
// class's ceiling. Review uplift lives in the canonical envelope confidence and
// is not part of this table.
type methodProfile struct {
	confidence float64
	assurance  EvidenceAssurance
}

var methodProfiles = map[CollectionMethod]methodProfile{
	CollectionAutomatedCollect: {0.80, AssuranceHigh},        // automated_scanner default
	CollectionAPIPull:          {0.60, AssuranceHigh},        // connector_import default
	CollectionAttestation:      {0.50, AssuranceSubstantial}, // third_party_attestation default
	CollectionManualUpload:     {0.70, AssuranceModerate},    // user_entered default
}

// BaseConfidence is the method's unreviewed confidence; 0 for an unknown method.
func (m CollectionMethod) BaseConfidence() float64 { return methodProfiles[m].confidence }

// BaseAssurance is the method's assurance before the signature adjustment; an
// unknown method reads as low.
func (m CollectionMethod) BaseAssurance() EvidenceAssurance {
	if p, ok := methodProfiles[m]; ok {
		return p.assurance
	}
	return AssuranceLow
}

// AssuranceFor derives assurance. The method's base level is reached ONLY with
// a verified signature. A signature that is present but unverified sits one
// level below base; no signature sits two levels below (floor minimal). Nothing
// verifies a signature yet, so signatureVerified is always false in practice and
// `high` is unreachable today.
func AssuranceFor(m CollectionMethod, signaturePresent, signatureVerified bool) EvidenceAssurance {
	r := AssuranceRank(m.BaseAssurance())
	switch {
	case signatureVerified:
	case signaturePresent:
		r--
	default:
		r -= 2
	}
	if r < 0 {
		r = 0
	}
	return assuranceOrder[r]
}

// DeriveIntegrity fills the computed integrity, signature flags, assurance and confidence
// fields. Pure: nothing is written to the database.
func (e *Evidence) DeriveIntegrity() {
	switch {
	case e.ContentHash != "":
		e.Integrity = IntegrityHashed
	case e.FileRef != "":
		e.Integrity = IntegrityUnhashed
	default:
		e.Integrity = IntegrityNone
	}
	e.SignaturePresent = e.Signature != ""
	// No verifier exists yet: a stored signature is never reported as verified.
	e.SignatureVerified = false
	e.Assurance = AssuranceFor(e.CollectionMethod, e.SignaturePresent, e.SignatureVerified)
	e.Confidence = e.CollectionMethod.BaseConfidence()
}

// ApplyRecordDefaults fills what a client may omit: the collection method (from
// the source), the collector (the collecting user, for a human method) and a
// point-in-time observation period (both bounds = collected_at).
func (e *Evidence) ApplyRecordDefaults() {
	if e.CollectionMethod == "" {
		e.CollectionMethod = CollectionMethodForSource(e.Source)
	}
	if e.CollectorRef == "" && !e.CollectionMethod.IsAutomated() && e.CollectedBy != nil {
		e.CollectorKind = CollectorUser
		e.CollectorRef = e.CollectedBy.String()
	}
	if e.CollectorRef != "" && e.CollectorKind == "" {
		e.CollectorKind = defaultCollectorKind(e.CollectionMethod)
		if !e.CollectionMethod.IsAutomated() {
			e.CollectorKind = CollectorUser
		}
	}
	if e.ObservedFrom == nil && e.ObservedTo == nil {
		e.ObservedFrom, e.ObservedTo = PointInTime(e.CollectedAt)
	} else if e.ObservedFrom == nil {
		t := *e.ObservedTo
		e.ObservedFrom = &t
	} else if e.ObservedTo == nil {
		t := *e.ObservedFrom
		e.ObservedTo = &t
	}
}

// ValidateObservation enforces ObservedFrom <= ObservedTo <= CollectedAt.
func (e *Evidence) ValidateObservation() error {
	if e.ObservedFrom == nil || e.ObservedTo == nil {
		return NewValidationError("observed_from and observed_to are required")
	}
	if e.ObservedFrom.After(*e.ObservedTo) {
		return NewValidationError("observed_from must not be after observed_to")
	}
	if e.ObservedTo.After(e.CollectedAt) {
		return NewValidationError("observed_to must not be after collected_at")
	}
	return nil
}

// ValidateCollection checks the collection method, the collector identity and
// the signature fields.
func (e *Evidence) ValidateCollection() error {
	if _, err := ParseCollectionMethod(string(e.CollectionMethod)); err != nil {
		return err
	}
	if e.CollectionMethod.IsAutomated() && strings.TrimSpace(e.CollectorRef) == "" {
		return NewValidationError("collector_ref is required for an automated collection method")
	}
	switch {
	case e.CollectorRef != "":
		if !collectorRefPattern.MatchString(e.CollectorRef) {
			return NewValidationError("collector_ref must be 1-128 characters of A-Z a-z 0-9 . _ : @ / - (an identifier, never a credential)")
		}
		if _, err := ParseCollectorKind(string(e.CollectorKind)); err != nil {
			return err
		}
	case e.CollectorKind != "":
		// A kind without a ref names nobody: rejected explicitly.
		return NewValidationError("collector_kind requires collector_ref")
	}
	if e.Signature != "" {
		if len(e.Signature) > base64.StdEncoding.EncodedLen(MaxSignatureBytes) {
			return NewValidationError("signature is limited to 8 KiB")
		}
		raw, err := base64.StdEncoding.DecodeString(e.Signature)
		if err != nil || len(raw) == 0 {
			return NewValidationError("signature must be base64")
		}
		if len(raw) > MaxSignatureBytes {
			return NewValidationError("signature is limited to 8 KiB")
		}
	}
	if e.Signature != "" && e.SignatureAlgorithm == "" {
		return NewValidationError("signature_algorithm is required when a signature is present")
	}
	if len(e.SignatureAlgorithm) > MaxSignatureAlgorithmLen {
		return NewValidationError("signature_algorithm is limited to 32 characters")
	}
	return nil
}

// ValidateRecord runs the whole integrity-record validation.
func (e *Evidence) ValidateRecord() error {
	if err := e.ValidateCollection(); err != nil {
		return err
	}
	return e.ValidateObservation()
}

// PointInTime is the default observation period: both bounds at the instant.
func PointInTime(t time.Time) (from, to *time.Time) {
	a, b := t, t
	return &a, &b
}
