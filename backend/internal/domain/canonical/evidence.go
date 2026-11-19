// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical

import (
	"regexp"
	"time"

	"github.com/google/uuid"
)

// EvidenceKind is the nature of the artifact (D6, closed): today's
// domain.EvidenceType, unchanged.
type EvidenceKind string

// D6's evidence kinds, spelled as domain.EvidenceType spells them.
const (
	KindDocument      EvidenceKind = "document"
	KindCapture       EvidenceKind = "capture"
	KindConfiguration EvidenceKind = "configuration"
	KindAttestation   EvidenceKind = "attestation"
	KindLog           EvidenceKind = "log"
)

// ReviewVerdict is the human verdict on an artifact (D6, closed): today's
// domain.EvidenceReview.
type ReviewVerdict string

// D6's review verdicts.
const (
	ReviewPending  ReviewVerdict = "pending"
	ReviewAccepted ReviewVerdict = "accepted"
	ReviewRejected ReviewVerdict = "rejected"
)

// EvidenceReview is `review` on an Evidence: `{verdict, note, reviewed_by,
// reviewed_at}`.
type EvidenceReview struct {
	Verdict    ReviewVerdict `json:"verdict"`
	Note       *string       `json:"note"`
	ReviewedBy *uuid.UUID    `json:"reviewed_by"`
	ReviewedAt *time.Time    `json:"reviewed_at"`
}

// Evidence is a proof artifact, reusable across controls and tests (ADR 0008
// D6). observed_at is required: when the proof was taken, today's collected_at
// (D4). The effective status is derived from review, valid_until and the
// clock, and is not stored. Content hash, observation period, collection
// method, collector identity and signature are #554's (ADR 0008 A11); chain of
// custody stays #1450's. Every #554 field is optional here: a legacy record
// carries none, and the confidence and assurance a method implies are derived,
// never stored.
type Evidence struct {
	Envelope
	Title       string         `json:"title"`
	Kind        EvidenceKind   `json:"kind"`
	Description *string        `json:"description"`
	FileRef     *string        `json:"file_ref"`
	Filename    *string        `json:"filename"`
	ExternalURL *string        `json:"external_url"`
	ValidUntil  *time.Time     `json:"valid_until"`
	CollectedBy *uuid.UUID     `json:"collected_by"`
	Review      EvidenceReview `json:"review"`
	Ownership

	ContentHash        *string           `json:"content_hash"`
	HashAlgorithm      *string           `json:"hash_algorithm"`
	ObservedFrom       *time.Time        `json:"observed_from"`
	ObservedTo         *time.Time        `json:"observed_to"`
	CollectionMethod   *CollectionMethod `json:"collection_method"`
	CollectorKind      *CollectorKind    `json:"collector_kind"`
	CollectorRef       *string           `json:"collector_ref"`
	Signature          *string           `json:"signature"`
	SignatureAlgorithm *string           `json:"signature_algorithm"`
}

// CollectionMethod is how the proof reached the register (A11). It is its own
// axis: the source class is unchanged by it.
type CollectionMethod string

// A11's four collection methods.
const (
	MethodManualUpload     CollectionMethod = "manual_upload"
	MethodAutomatedCollect CollectionMethod = "automated_collector"
	MethodAPIPull          CollectionMethod = "api_pull"
	MethodAttestation      CollectionMethod = "attestation"
)

// CollectorKind is what sort of identity collected the proof (A11).
type CollectorKind string

// A11's three collector kinds.
const (
	CollectorUser      CollectorKind = "user"
	CollectorConnector CollectorKind = "connector"
	CollectorJob       CollectorKind = "job"
)

var (
	sha256Hex    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	collectorRef = regexp.MustCompile(`^[A-Za-z0-9._:@/-]+$`)
)

// validateIntegrity applies A11 to the optional #554 fields.
func (e Evidence) validateIntegrity() error {
	if e.ContentHash != nil {
		if !sha256Hex.MatchString(*e.ContentHash) {
			return invalid("content_hash", "must be 64 lower-case hex characters (SHA-256)")
		}
		if e.HashAlgorithm == nil || *e.HashAlgorithm != "sha256" {
			return invalid("hash_algorithm", "must be sha256 when content_hash is set")
		}
	}
	if err := optionalTime("observed_from", e.ObservedFrom); err != nil {
		return err
	}
	if err := optionalTime("observed_to", e.ObservedTo); err != nil {
		return err
	}
	if (e.ObservedFrom == nil) != (e.ObservedTo == nil) {
		return invalid("observed_from", "and observed_to are both present or both absent")
	}
	if e.ObservedFrom != nil {
		if e.ObservedFrom.After(*e.ObservedTo) {
			return invalid("observed_from", "must not be after observed_to")
		}
		if e.ObservedAt != nil && e.ObservedTo.After(*e.ObservedAt) {
			return invalid("observed_to", "must not be after observed_at")
		}
	}
	if e.CollectionMethod != nil {
		switch *e.CollectionMethod {
		case MethodManualUpload, MethodAttestation:
		case MethodAutomatedCollect, MethodAPIPull:
			if blankPtr(e.CollectorRef) {
				return invalid("collector_ref", "is required for an automated collection method")
			}
		default:
			return invalid("collection_method", "must be manual_upload, automated_collector, api_pull or attestation")
		}
	}
	if e.CollectorKind != nil {
		switch *e.CollectorKind {
		case CollectorUser, CollectorConnector, CollectorJob:
		default:
			return invalid("collector_kind", "must be user, connector or job")
		}
	}
	if !blankPtr(e.CollectorRef) {
		if err := optionalText("collector_ref", e.CollectorRef, 128); err != nil {
			return err
		}
		if !collectorRef.MatchString(*e.CollectorRef) {
			return invalid("collector_ref", "must be an identifier of A-Z a-z 0-9 . _ : @ / - (never a credential)")
		}
	}
	if err := optionalText("signature_algorithm", e.SignatureAlgorithm, 32); err != nil {
		return err
	}
	if !blankPtr(e.Signature) && blankPtr(e.SignatureAlgorithm) {
		return invalid("signature_algorithm", "is required when signature is present")
	}
	return nil
}

// ReviewContext is the review state the user_entered uplift reads for this
// artifact. Use it with DefaultConfidence to compute, never accept, the
// confidence of a user_entered Evidence.
func (e Evidence) ReviewContext() ReviewContext {
	return ReviewContext{
		Verdict:     e.Review.Verdict,
		ReviewedBy:  e.Review.ReviewedBy,
		CollectedBy: e.CollectedBy,
		ActorID:     e.Provenance.Actor.ID,
	}
}

// Validate applies D2, D3, D4 and D6 to an Evidence.
func (e Evidence) Validate() error {
	if _, err := e.Envelope.validate(TypeEvidence, observedRequired, e.ReviewContext()); err != nil {
		return err
	}
	if err := requireText("title", e.Title, 255); err != nil {
		return err
	}
	switch e.Kind {
	case KindDocument, KindCapture, KindConfiguration, KindAttestation, KindLog:
	default:
		return invalid("kind", "must be document, capture, configuration, attestation or log")
	}
	if blankPtr(e.FileRef) && blankPtr(e.ExternalURL) && blankPtr(e.Description) {
		return invalid("evidence", "at least one of file_ref, external_url or description is required")
	}
	if err := optionalText("filename", e.Filename, 255); err != nil {
		return err
	}
	if !blankPtr(e.ExternalURL) {
		if err := validateExternalURL("external_url", *e.ExternalURL); err != nil {
			return err
		}
	}
	if err := optionalTime("valid_until", e.ValidUntil); err != nil {
		return err
	}
	if err := optionalID("collected_by", e.CollectedBy); err != nil {
		return err
	}
	switch e.Review.Verdict {
	case ReviewPending, ReviewAccepted, ReviewRejected:
	default:
		return invalid("review.verdict", "must be pending, accepted or rejected")
	}
	if err := optionalID("review.reviewed_by", e.Review.ReviewedBy); err != nil {
		return err
	}
	if err := optionalTime("review.reviewed_at", e.Review.ReviewedAt); err != nil {
		return err
	}
	if err := e.validateIntegrity(); err != nil {
		return err
	}
	return e.Ownership.validate()
}
