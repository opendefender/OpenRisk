// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"strings"
	"testing"
)

// The lifecycle, as a cross product: every pair is either explicitly allowed or
// explicitly refused. Written this way because the dangerous transitions are the
// ones nobody thought to write a test for.
func TestReportLifecycle_Transitions(t *testing.T) {
	all := []ReportLifecycle{
		ReportLifecycleDraft, ReportLifecycleInReview,
		ReportLifecycleApproved, ReportLifecyclePublished,
	}
	allowed := map[ReportLifecycle]map[ReportLifecycle]bool{
		ReportLifecycleDraft:     {ReportLifecycleInReview: true, ReportLifecycleApproved: true},
		ReportLifecycleInReview:  {ReportLifecycleDraft: true, ReportLifecycleApproved: true},
		ReportLifecycleApproved:  {ReportLifecyclePublished: true, ReportLifecycleDraft: true},
		ReportLifecyclePublished: {},
	}

	for _, from := range all {
		for _, to := range all {
			err := from.CanTransitionTo(to)
			want := allowed[from][to]
			if want && err != nil {
				t.Errorf("%s -> %s should be allowed: %v", from, to, err)
			}
			if !want && err == nil {
				t.Errorf("%s -> %s should be refused", from, to)
			}
		}
	}
}

// A published report is frozen. Editing a document people already hold is how
// two versions of "the" report end up in circulation, so the refusal has to say
// what to do instead.
func TestReportLifecycle_PublishedIsFrozenAndSaysWhy(t *testing.T) {
	err := ReportLifecyclePublished.CanTransitionTo(ReportLifecycleDraft)
	if err == nil {
		t.Fatal("published must not go back to draft")
	}
	if !strings.Contains(err.Error(), "new version") {
		t.Fatalf("the refusal should point at generating a new version, got %q", err.Error())
	}
	if ReportLifecyclePublished.Editable() {
		t.Fatal("a published report must not be editable")
	}
	for _, s := range []ReportLifecycle{ReportLifecycleDraft, ReportLifecycleInReview, ReportLifecycleApproved} {
		if !s.Editable() {
			t.Fatalf("%s should still be editable", s)
		}
	}
}

func TestReportLifecycle_NoOpIsRefused(t *testing.T) {
	if err := ReportLifecycleDraft.CanTransitionTo(ReportLifecycleDraft); err == nil {
		t.Fatal("moving to the state it is already in must be refused, not silently accepted")
	}
}

// The hash is the difference between a report and a PDF that says it is a
// report: it must change when a single byte does.
func TestReport_IntegrityHash(t *testing.T) {
	body := []byte("%PDF-1.7 ... compliance report ...")
	r := &Report{Artifact: body, ContentHash: ComputeContentHash(body)}

	if !r.VerifyIntegrity() {
		t.Fatal("a freshly hashed artifact must verify")
	}
	if len(r.ContentHash) != 64 {
		t.Fatalf("expected a hex sha-256, got %d chars", len(r.ContentHash))
	}
	// The printed value is the CONTENT fingerprint, not the file hash: a file
	// cannot contain the hash of itself.
	r.ContentFingerprint = ComputeContentHash([]byte("the data that went into the report"))
	if len(r.ShortFingerprint()) != 16 {
		t.Fatalf("the printed fingerprint should be 16 chars, got %d", len(r.ShortFingerprint()))
	}
	if r.ContentFingerprint == r.ContentHash {
		t.Fatal("the fingerprint and the file hash answer different questions and must not be the same number")
	}

	// One byte changed anywhere must break it.
	r.Artifact = append([]byte{}, body...)
	r.Artifact[5] = 'X'
	if r.VerifyIntegrity() {
		t.Fatal("a tampered artifact must not verify")
	}

	// Nothing to verify is not the same as verified.
	if (&Report{}).VerifyIntegrity() {
		t.Fatal("an empty report must not report itself as intact")
	}
	if (&Report{Artifact: body}).VerifyIntegrity() {
		t.Fatal("an artifact with no recorded hash must not verify")
	}
}

func TestParseReportFormat(t *testing.T) {
	// Empty defaults to PDF: the common case is the document you file.
	if got, err := ParseReportFormat(""); err != nil || got != ReportFormatPDF {
		t.Fatalf("empty should default to pdf, got %q err=%v", got, err)
	}
	for _, s := range []string{"pdf", "docx", "xlsx", "PDF", " DOCX "} {
		if _, err := ParseReportFormat(s); err != nil {
			t.Fatalf("%q should be valid: %v", s, err)
		}
	}
	if _, err := ParseReportFormat("csv"); err == nil {
		t.Fatal("an unsupported format must be refused, not silently rendered as pdf")
	}

	if ReportFormatXLSX.Extension() != ".xlsx" {
		t.Fatal("extension should carry the dot")
	}
	if ReportFormatDOCX.ContentType() == ReportFormatPDF.ContentType() {
		t.Fatal("each format needs its own content type or browsers will mislabel the download")
	}
}

// The document's language is independent of the interface's. A French-speaking
// officer producing an English report for a foreign regulator must not have to
// switch the whole product.
func TestParseReportLocale(t *testing.T) {
	if got, _ := ParseReportLocale(""); got != ReportLocaleFR {
		t.Fatalf("empty should default to fr, got %q", got)
	}
	for _, s := range []string{"fr", "en", "EN"} {
		if _, err := ParseReportLocale(s); err != nil {
			t.Fatalf("%q should be valid: %v", s, err)
		}
	}
	if _, err := ParseReportLocale("de"); err == nil {
		t.Fatal("an unsupported language must be refused rather than silently rendered in French")
	}
}

func TestReportType_Catalogue(t *testing.T) {
	types := AllReportTypes()
	if len(types) != 6 {
		t.Fatalf("expected the six report types the configurator offers, got %d", len(types))
	}
	seen := map[ReportType]bool{}
	for _, k := range types {
		if seen[k] {
			t.Fatalf("duplicate report type %q", k)
		}
		seen[k] = true
		if !k.Valid() {
			t.Fatalf("%q is listed but not valid", k)
		}
	}
	if ReportType("something_else").Valid() {
		t.Fatal("an unknown type must not validate")
	}
}

func TestReportRunState_Terminal(t *testing.T) {
	if ReportRunQueued.Terminal() || ReportRunRunning.Terminal() {
		t.Fatal("a job still in flight is not terminal")
	}
	if !ReportRunSucceeded.Terminal() || !ReportRunFailed.Terminal() {
		t.Fatal("succeeded and failed both end the run — a client polling must be able to stop")
	}
}
