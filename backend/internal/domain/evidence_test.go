// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"testing"
	"time"
)

func evTime(t time.Time) *time.Time { return &t }

func TestEvidence_EffectiveStatus(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name   string
		ev     Evidence
		expect EvidenceStatus
	}{
		{
			name:   "no expiry is simply valid",
			ev:     Evidence{Review: EvidenceReviewAccepted},
			expect: EvidenceStatusValid,
		},
		{
			name:   "expiry far away is valid",
			ev:     Evidence{Review: EvidenceReviewAccepted, ValidUntil: evTime(now.Add(90 * 24 * time.Hour))},
			expect: EvidenceStatusValid,
		},
		{
			name:   "inside the window is expiring soon",
			ev:     Evidence{Review: EvidenceReviewAccepted, ValidUntil: evTime(now.Add(10 * 24 * time.Hour))},
			expect: EvidenceStatusExpiring,
		},
		{
			name:   "past expiry is expired",
			ev:     Evidence{Review: EvidenceReviewAccepted, ValidUntil: evTime(now.Add(-time.Hour))},
			expect: EvidenceStatusExpired,
		},
		{
			name:   "expiring exactly now is expired, not expiring",
			ev:     Evidence{Review: EvidenceReviewAccepted, ValidUntil: evTime(now)},
			expect: EvidenceStatusExpired,
		},
		{
			// The verdict outranks the calendar: a rejected artifact uploaded this
			// morning is not proof of anything.
			name:   "rejected outranks a fresh date",
			ev:     Evidence{Review: EvidenceReviewRejected, ValidUntil: evTime(now.Add(365 * 24 * time.Hour))},
			expect: EvidenceStatusRejected,
		},
		{
			name:   "rejected outranks expiry too",
			ev:     Evidence{Review: EvidenceReviewRejected, ValidUntil: evTime(now.Add(-time.Hour))},
			expect: EvidenceStatusRejected,
		},
		{
			name:   "pending review is not yet proof",
			ev:     Evidence{Review: EvidenceReviewPending},
			expect: EvidenceStatusPending,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.ev.EffectiveStatus(now); got != tc.expect {
				t.Fatalf("EffectiveStatus = %q, want %q", got, tc.expect)
			}
		})
	}
}

// Covers is the predicate the whole module answers to: only currently-good proof
// substantiates a control. If this ever returns true for expired or rejected
// evidence, every coverage number in the product overstates reality.
func TestEvidence_Covers(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name   string
		ev     Evidence
		covers bool
	}{
		{"valid covers", Evidence{Review: EvidenceReviewAccepted}, true},
		{"expiring still covers", Evidence{Review: EvidenceReviewAccepted, ValidUntil: evTime(now.Add(5 * 24 * time.Hour))}, true},
		{"expired does not cover", Evidence{Review: EvidenceReviewAccepted, ValidUntil: evTime(now.Add(-24 * time.Hour))}, false},
		{"rejected does not cover", Evidence{Review: EvidenceReviewRejected}, false},
		{"pending does not cover", Evidence{Review: EvidenceReviewPending}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.ev.Covers(now); got != tc.covers {
				t.Fatalf("Covers = %v, want %v", got, tc.covers)
			}
		})
	}
}

func TestEvidence_DaysUntil(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

	if d := (&Evidence{}).DaysUntil(now); d != nil {
		t.Fatalf("no expiry should yield nil, got %v", *d)
	}
	ev := Evidence{ValidUntil: evTime(now.Add(10 * 24 * time.Hour))}
	if d := ev.DaysUntil(now); d == nil || *d != 10 {
		t.Fatalf("want 10 days, got %v", d)
	}
	past := Evidence{ValidUntil: evTime(now.Add(-3 * 24 * time.Hour))}
	if d := past.DaysUntil(now); d == nil || *d != -3 {
		t.Fatalf("want -3 days, got %v", d)
	}
}

func TestParseEvidenceType(t *testing.T) {
	// Empty defaults rather than failing: the common upload path should not force
	// a classification the uploader may not have.
	if got, err := ParseEvidenceType(""); err != nil || got != EvidenceTypeDocument {
		t.Fatalf("empty should default to document, got %q err=%v", got, err)
	}
	for _, s := range []string{"document", "capture", "configuration", "attestation", "log"} {
		if _, err := ParseEvidenceType(s); err != nil {
			t.Fatalf("%q should be valid: %v", s, err)
		}
	}
	if _, err := ParseEvidenceType("screenshot"); err == nil {
		t.Fatal("unknown type must be rejected")
	}
}

func TestParseEvidenceSourceAndReview(t *testing.T) {
	if got, _ := ParseEvidenceSource(""); got != EvidenceSourceManual {
		t.Fatalf("empty source should default to manual, got %q", got)
	}
	if _, err := ParseEvidenceSource("telepathy"); err == nil {
		t.Fatal("unknown source must be rejected")
	}
	if got, _ := ParseEvidenceReview(""); got != EvidenceReviewAccepted {
		t.Fatalf("empty review should default to accepted, got %q", got)
	}
	if _, err := ParseEvidenceReview("maybe"); err == nil {
		t.Fatal("unknown review must be rejected")
	}
}
