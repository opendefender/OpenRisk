// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package evidence

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

type fakeAudit struct {
	events []domain.AuditEvent
	err    error
}

func (a *fakeAudit) Append(_ context.Context, e *domain.AuditEvent) error {
	if a.err != nil {
		return a.err
	}
	a.events = append(a.events, *e)
	return nil
}

func uploaded(t *testing.T, f *fixture, in CreateInput) *domain.Evidence {
	t.Helper()
	if in.Title == "" {
		in.Title = "doc"
	}
	if in.Content == nil {
		in.Content = bytes.NewReader(pdfSample)
		in.Filename = "a.pdf"
	}
	ev, err := f.svc.Create(context.Background(), f.tenant, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return ev
}

// AC1: the hash is the SHA-256 of the saved bytes, computed here.
func TestCreate_StoresTheContentHashOfTheSavedBytes(t *testing.T) {
	f := setup(t)
	ev := uploaded(t, f, CreateInput{})
	if ev.ContentHash != domain.ContentHashOf(pdfSample) || ev.HashAlgorithm != "sha256" {
		t.Fatalf("hash %q alg %q", ev.ContentHash, ev.HashAlgorithm)
	}
	if !bytes.Equal(f.store.saved[ev.FileRef], pdfSample) || ev.ContentHash != domain.ContentHashOf(f.store.saved[ev.FileRef]) {
		t.Fatal("hash must cover exactly the stored bytes")
	}
	if ev.Integrity != domain.IntegrityHashed {
		t.Fatalf("integrity %q", ev.Integrity)
	}
	// Persisted, not just decorated.
	if f.repo.items[ev.ID].ContentHash != ev.ContentHash {
		t.Fatal("hash not persisted")
	}
}

func TestDownload_VerifiedWhenTheHashMatches(t *testing.T) {
	f := setup(t)
	ev := uploaded(t, f, CreateInput{})
	_, rc, verified, err := f.svc.Download(context.Background(), f.tenant, uuid.New(), ev.ID)
	if err != nil || !verified {
		t.Fatalf("verified=%v err=%v", verified, err)
	}
	defer rc.Close()
}

// AC2: tampered blob -> no content, typed integrity error, audit event.
func TestDownload_RefusesTamperedBlobAndRecordsAnAlert(t *testing.T) {
	f := setup(t)
	audit := &fakeAudit{}
	f.svc.WithIntegrityAudit(audit)
	ev := uploaded(t, f, CreateInput{})
	actor := uuid.New()
	f.store.saved[ev.FileRef] = append([]byte("tampered"), pdfSample...)

	got, rc, verified, err := f.svc.Download(context.Background(), f.tenant, actor, ev.ID)
	if got != nil || rc != nil || verified {
		t.Fatal("no content may be returned for a tampered blob")
	}
	if !errors.Is(err, domain.ErrConflict) || domain.ReasonFromError(err) != domain.ReasonEvidenceIntegrity {
		t.Fatalf("want the typed integrity error, got %v", err)
	}
	if len(audit.events) != 1 {
		t.Fatalf("want one alert, got %d", len(audit.events))
	}
	e := audit.events[0]
	if e.TenantID != f.tenant || e.Action != domain.AuditActionIntegrityAlert || e.EntityType != "evidence" ||
		e.EntityID != ev.ID.String() || e.ActorID == nil || *e.ActorID != actor {
		t.Fatalf("alert: %+v", e)
	}
	if strings.Contains(e.Summary, "tampered") {
		t.Fatal("no content in the alert")
	}
}

func TestDownload_StillRefusesWhenTheAlertCannotBeRecorded(t *testing.T) {
	f := setup(t)
	f.svc.WithIntegrityAudit(&fakeAudit{err: errors.New("chain down")})
	ev := uploaded(t, f, CreateInput{})
	f.store.saved[ev.FileRef] = []byte("tampered")
	_, rc, _, err := f.svc.Download(context.Background(), f.tenant, uuid.New(), ev.ID)
	if rc != nil || !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("must refuse with the integrity error, got %v", err)
	}
}

// AC9: a legacy row with no hash is served, flagged unverified.
func TestDownload_LegacyRowWithoutHashIsServedUnverified(t *testing.T) {
	f := setup(t)
	ev := uploaded(t, f, CreateInput{})
	f.repo.items[ev.ID].ContentHash, f.repo.items[ev.ID].HashAlgorithm = "", ""
	_, rc, verified, err := f.svc.Download(context.Background(), f.tenant, uuid.Nil, ev.ID)
	if err != nil || rc == nil || verified {
		t.Fatalf("legacy: err=%v rc=%v verified=%v", err, rc, verified)
	}
	got, _ := f.svc.Get(context.Background(), f.tenant, ev.ID)
	if got.Integrity != domain.IntegrityUnhashed {
		t.Fatalf("Get must show the row as unhashed, got %q", got.Integrity)
	}
}

func TestDownload_Unauthorized_WithoutTenant(t *testing.T) {
	f := setup(t)
	_, rc, _, err := f.svc.Download(context.Background(), uuid.Nil, uuid.Nil, uuid.New())
	if rc != nil || !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("nil tenant must be fail-closed unauthorized, got %v", err)
	}
}

// AC8 at the use-case level: every path says "not found" for another tenant's
// evidence, and listing shows nothing.
func TestEvidence_AnotherTenantReachesNothing(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	ev := uploaded(t, f, CreateInput{ControlIDs: []uuid.UUID{f.c1}})

	if _, err := f.svc.Get(ctx, f.other, ev.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get: %v", err)
	}
	if _, _, _, err := f.svc.Download(ctx, f.other, uuid.New(), ev.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Download: %v", err)
	}
	title := "x"
	if _, err := f.svc.Update(ctx, f.other, ev.ID, UpdateInput{Title: &title}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Update: %v", err)
	}
	if err := f.svc.Delete(ctx, f.other, ev.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Delete: %v", err)
	}
	if res, err := f.svc.List(ctx, f.other, domain.EvidenceFilter{}); err != nil || len(res.Items) != 0 {
		t.Errorf("tenant-wide List must be empty, got %v %v", res, err)
	}
	if _, err := f.svc.ListByControl(ctx, f.other, f.c1); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("control-scoped List: %v", err)
	}
}

// AC4.
func TestCreate_RejectsAnIncoherentObservationPeriod(t *testing.T) {
	f := setup(t)
	c := f.now.Add(-24 * time.Hour)
	later, earlier := c.Add(time.Hour), c.Add(-time.Hour)
	cases := map[string]CreateInput{
		"from after to":         {ObservedFrom: &c, ObservedTo: &earlier},
		"to after collected_at": {ObservedFrom: &c, ObservedTo: &later},
	}
	for name, in := range cases {
		in.Title, in.Description, in.CollectedAt = "x", "statement", &c
		if _, err := f.svc.Create(context.Background(), f.tenant, in); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("%s: want ErrValidation, got %v", name, err)
		}
	}
}

func TestCreate_ObservationDefaultsToCollectedAtAndIsStored(t *testing.T) {
	f := setup(t)
	c := f.now.Add(-24 * time.Hour)
	ev, err := f.svc.Create(context.Background(), f.tenant, CreateInput{Title: "x", Description: "s", CollectedAt: &c})
	if err != nil {
		t.Fatal(err)
	}
	st := f.repo.items[ev.ID]
	if st.ObservedFrom == nil || st.ObservedTo == nil || !st.ObservedFrom.Equal(c) || !st.ObservedTo.Equal(c) {
		t.Fatalf("stored observation: %v %v", st.ObservedFrom, st.ObservedTo)
	}
}

// AC5.
func TestCreate_RejectsAMethodOutsideTheFour(t *testing.T) {
	f := setup(t)
	_, err := f.svc.Create(context.Background(), f.tenant, CreateInput{Title: "x", Description: "s", CollectionMethod: "carrier_pigeon"})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestCreate_DerivesTheMethodFromTheSource(t *testing.T) {
	f := setup(t)
	for src, want := range map[string]domain.CollectionMethod{
		"": domain.CollectionManualUpload, "manual": domain.CollectionManualUpload,
		"integration": domain.CollectionAPIPull, "scanner": domain.CollectionAutomatedCollect,
		"automation": domain.CollectionAutomatedCollect,
	} {
		in := CreateInput{Title: "x", Description: "s", Source: src}
		if want == domain.CollectionAPIPull || want == domain.CollectionAutomatedCollect {
			in.CollectorRef = "collector-1"
		}
		ev, err := f.svc.Create(context.Background(), f.tenant, in)
		if err != nil {
			t.Fatalf("source %q: %v", src, err)
		}
		if ev.CollectionMethod != want {
			t.Errorf("source %q: %q, want %q", src, ev.CollectionMethod, want)
		}
	}
}

// AC7.
func TestCreate_AutomatedSourceNeedsACollectorRef(t *testing.T) {
	f := setup(t)
	for _, src := range []string{"integration", "scanner", "automation"} {
		_, err := f.svc.Create(context.Background(), f.tenant, CreateInput{Title: "x", Description: "s", Source: src})
		if !errors.Is(err, domain.ErrValidation) {
			t.Errorf("%s without collector_ref: want ErrValidation, got %v", src, err)
		}
	}
	// Explicit automated method on a manual source needs one too.
	_, err := f.svc.Create(context.Background(), f.tenant, CreateInput{Title: "x", Description: "s", CollectionMethod: "automated_collector"})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("explicit method: %v", err)
	}
}

func TestCreate_ManualCollectorDefaultsToTheCollectingUser(t *testing.T) {
	f := setup(t)
	u := uuid.New()
	ev := uploaded(t, f, CreateInput{CollectedBy: u})
	if ev.CollectorKind != domain.CollectorUser || ev.CollectorRef != u.String() {
		t.Fatalf("collector: %q %q", ev.CollectorKind, ev.CollectorRef)
	}
}

// AC3 end to end: the record shows the signature as absent, and assurance is
// lower than for the same evidence with one. Never verified, only stored.
func TestCreate_SignatureAbsentLowersAssurance(t *testing.T) {
	f := setup(t)
	unsigned := uploaded(t, f, CreateInput{})
	sig := base64.StdEncoding.EncodeToString([]byte("detached-signature"))
	signed := uploaded(t, f, CreateInput{Signature: sig, SignatureAlgorithm: "ed25519"})
	if unsigned.Signed || !signed.Signed {
		t.Fatalf("signed flags: %v %v", unsigned.Signed, signed.Signed)
	}
	if domain.AssuranceRank(unsigned.Assurance) >= domain.AssuranceRank(signed.Assurance) {
		t.Fatalf("assurance unsigned %s signed %s", unsigned.Assurance, signed.Assurance)
	}
	if f.repo.items[signed.ID].Signature != sig {
		t.Fatal("signature stored as sent")
	}
	_, err := f.svc.Create(context.Background(), f.tenant, CreateInput{Title: "x", Description: "s", Signature: sig})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("signature without algorithm: %v", err)
	}
}

// AC6 via the read model: the derived confidence follows the table.
func TestRead_ExposesDerivedConfidenceFromTheMethodTable(t *testing.T) {
	f := setup(t)
	manual := uploaded(t, f, CreateInput{})
	auto := uploaded(t, f, CreateInput{CollectionMethod: "automated_collector", CollectorRef: "scanner-7"})
	if manual.Confidence != 0.70 || auto.Confidence != 0.80 || manual.Confidence >= auto.Confidence {
		t.Fatalf("confidence manual %v auto %v", manual.Confidence, auto.Confidence)
	}
	if auto.CollectorKind != domain.CollectorJob {
		t.Fatalf("automated kind default: %q", auto.CollectorKind)
	}
}

// Update re-checks the ordering when collected_at moves.
func TestUpdate_RechecksObservationAgainstCollectedAt(t *testing.T) {
	f := setup(t)
	c := f.now.Add(-48 * time.Hour)
	ev, err := f.svc.Create(context.Background(), f.tenant, CreateInput{Title: "x", Description: "s", CollectedAt: &c})
	if err != nil {
		t.Fatal(err)
	}
	earlier := c.Add(-24 * time.Hour)
	if _, err := f.svc.Update(context.Background(), f.tenant, ev.ID, UpdateInput{CollectedAt: &earlier}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("moving collected_at before observed_to: want ErrValidation, got %v", err)
	}
	from, to := earlier.Add(-time.Hour), earlier
	if _, err := f.svc.Update(context.Background(), f.tenant, ev.ID, UpdateInput{CollectedAt: &earlier, ObservedFrom: &from, ObservedTo: &to}); err != nil {
		t.Fatalf("moving both together: %v", err)
	}
	bad := earlier.Add(time.Hour)
	if _, err := f.svc.Update(context.Background(), f.tenant, ev.ID, UpdateInput{ObservedTo: &bad}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("observed_to after collected_at: %v", err)
	}
}

// A legacy row (no observation period yet) can still be edited.
func TestUpdate_LegacyRowWithoutObservationGetsThePointInTimeDefault(t *testing.T) {
	f := setup(t)
	ev := uploaded(t, f, CreateInput{})
	f.repo.items[ev.ID].ObservedFrom, f.repo.items[ev.ID].ObservedTo = nil, nil
	title := "renamed"
	got, err := f.svc.Update(context.Background(), f.tenant, ev.ID, UpdateInput{Title: &title})
	if err != nil || got.ObservedFrom == nil || !got.ObservedFrom.Equal(got.CollectedAt) {
		t.Fatalf("legacy update: %v %+v", err, got)
	}
}
