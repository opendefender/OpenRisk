// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical

import (
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
// clock, and is not stored. Content hash, observation period and chain of
// custody are #554's, added by amendment there.
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
	return e.Ownership.validate()
}
