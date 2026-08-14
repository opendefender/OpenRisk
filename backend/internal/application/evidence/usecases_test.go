// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package evidence

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// Fakes. In-memory rather than mocks with expectations: these tests are about
// what the library ends up holding, not about which methods got called.
// ---------------------------------------------------------------------------

type fakeRepo struct {
	items    map[uuid.UUID]*domain.Evidence
	links    []domain.EvidenceControlLink
	failNext error
}

func newFakeRepo() *fakeRepo { return &fakeRepo{items: map[uuid.UUID]*domain.Evidence{}} }

func (r *fakeRepo) Create(_ context.Context, e *domain.Evidence) error {
	if r.failNext != nil {
		err := r.failNext
		r.failNext = nil
		return err
	}
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	cp := *e
	r.items[e.ID] = &cp
	return nil
}

func (r *fakeRepo) Update(_ context.Context, e *domain.Evidence) error {
	cur, ok := r.items[e.ID]
	if !ok || cur.TenantID != e.TenantID {
		return domain.NewNotFoundError("evidence", e.ID)
	}
	cp := *e
	r.items[e.ID] = &cp
	return nil
}

func (r *fakeRepo) GetByID(_ context.Context, tenantID, id uuid.UUID) (*domain.Evidence, error) {
	e, ok := r.items[id]
	if !ok || e.TenantID != tenantID {
		return nil, nil
	}
	cp := *e
	return &cp, nil
}

func (r *fakeRepo) List(_ context.Context, tenantID uuid.UUID, f domain.EvidenceFilter) ([]domain.Evidence, int64, error) {
	var out []domain.Evidence
	for _, e := range r.items {
		if e.TenantID != tenantID {
			continue
		}
		if f.Type != "" && e.Type != f.Type {
			continue
		}
		if f.Review != "" && e.Review != f.Review {
			continue
		}
		if f.ControlID != nil && !r.linked(e.ID, *f.ControlID) {
			continue
		}
		out = append(out, *e)
	}
	total := int64(len(out))
	if f.Offset > 0 && f.Offset < len(out) {
		out = out[f.Offset:]
	}
	if f.Limit > 0 && f.Limit < len(out) {
		out = out[:f.Limit]
	}
	return out, total, nil
}

func (r *fakeRepo) linked(evID, controlID uuid.UUID) bool {
	for _, l := range r.links {
		if l.EvidenceID == evID && l.ControlID == controlID {
			return true
		}
	}
	return false
}

func (r *fakeRepo) Delete(_ context.Context, tenantID, id uuid.UUID) error {
	e, ok := r.items[id]
	if !ok || e.TenantID != tenantID {
		return domain.NewNotFoundError("evidence", id)
	}
	delete(r.items, id)
	return nil
}

func (r *fakeRepo) Link(_ context.Context, l *domain.EvidenceControlLink) error {
	if r.linked(l.EvidenceID, l.ControlID) {
		return nil // idempotent, like the unique index
	}
	r.links = append(r.links, *l)
	return nil
}

func (r *fakeRepo) Unlink(_ context.Context, tenantID, evID, controlID uuid.UUID) error {
	out := r.links[:0]
	for _, l := range r.links {
		if l.TenantID == tenantID && l.EvidenceID == evID && l.ControlID == controlID {
			continue
		}
		out = append(out, l)
	}
	r.links = out
	return nil
}

func (r *fakeRepo) ListLinks(_ context.Context, tenantID uuid.UUID, ids []uuid.UUID) ([]domain.EvidenceControlLink, error) {
	want := map[uuid.UUID]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []domain.EvidenceControlLink
	for _, l := range r.links {
		if l.TenantID == tenantID && want[l.EvidenceID] {
			out = append(out, l)
		}
	}
	return out, nil
}

func (r *fakeRepo) ListByControl(_ context.Context, tenantID, controlID uuid.UUID) ([]domain.Evidence, error) {
	var out []domain.Evidence
	for _, l := range r.links {
		if l.TenantID != tenantID || l.ControlID != controlID {
			continue
		}
		if e, ok := r.items[l.EvidenceID]; ok {
			out = append(out, *e)
		}
	}
	return out, nil
}

func (r *fakeRepo) CountCoveringByFramework(_ context.Context, tenantID, frameworkID uuid.UUID, now time.Time) (map[uuid.UUID]int, error) {
	out := map[uuid.UUID]int{}
	for _, l := range r.links {
		if l.TenantID != tenantID {
			continue
		}
		e, ok := r.items[l.EvidenceID]
		if !ok || !e.Covers(now) {
			continue
		}
		out[l.ControlID]++
	}
	return out, nil
}

func (r *fakeRepo) ControlsWithCoverage(ctx context.Context, tenantID, frameworkID uuid.UUID, now time.Time) (map[uuid.UUID]bool, error) {
	counts, _ := r.CountCoveringByFramework(ctx, tenantID, frameworkID, now)
	out := map[uuid.UUID]bool{}
	for id, n := range counts {
		out[id] = n > 0
	}
	return out, nil
}

func (r *fakeRepo) ListExpiring(context.Context, time.Time, time.Duration, int) ([]domain.Evidence, error) {
	return nil, nil
}
func (r *fakeRepo) MarkReminded(context.Context, uuid.UUID, time.Time) error { return nil }

type fakeControls struct {
	controls   map[uuid.UUID]domain.ComplianceControl
	frameworks map[uuid.UUID]domain.ComplianceFramework
}

func (f *fakeControls) GetControlByID(_ context.Context, id, tenantID uuid.UUID) (*domain.ComplianceControl, error) {
	c, ok := f.controls[id]
	if !ok || c.TenantID != tenantID {
		return nil, nil
	}
	return &c, nil
}

func (f *fakeControls) ListControlsByFramework(_ context.Context, tenantID, frameworkID uuid.UUID) ([]domain.ComplianceControl, error) {
	var out []domain.ComplianceControl
	for _, c := range f.controls {
		if c.TenantID == tenantID && c.FrameworkID == frameworkID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeControls) GetFrameworkByID(_ context.Context, id, tenantID uuid.UUID) (*domain.ComplianceFramework, error) {
	fw, ok := f.frameworks[id]
	if !ok || fw.TenantID != tenantID {
		return nil, nil
	}
	return &fw, nil
}

func (f *fakeControls) ListFrameworks(_ context.Context, tenantID uuid.UUID) ([]domain.ComplianceFramework, error) {
	var out []domain.ComplianceFramework
	for _, fw := range f.frameworks {
		if fw.TenantID == tenantID {
			out = append(out, fw)
		}
	}
	return out, nil
}

type fakeStorage struct {
	saved   map[string][]byte
	deleted []string
	failing bool
}

func newFakeStorage() *fakeStorage { return &fakeStorage{saved: map[string][]byte{}} }

func (s *fakeStorage) Save(_ context.Context, tenantID uuid.UUID, filename string, content io.Reader) (string, error) {
	b, _ := io.ReadAll(content)
	key := tenantID.String() + "/" + filename
	s.saved[key] = b
	return key, nil
}

func (s *fakeStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	b, ok := s.saved[key]
	if !ok {
		return nil, io.EOF
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (s *fakeStorage) Delete(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	delete(s.saved, key)
	return nil
}

// ---------------------------------------------------------------------------

type fixture struct {
	svc      *Service
	repo     *fakeRepo
	controls *fakeControls
	store    *fakeStorage
	tenant   uuid.UUID
	other    uuid.UUID
	fw       uuid.UUID
	c1, c2   uuid.UUID
	now      time.Time
}

func setup(t *testing.T) *fixture {
	t.Helper()
	tenant, other, fw := uuid.New(), uuid.New(), uuid.New()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

	fc := &fakeControls{
		controls:   map[uuid.UUID]domain.ComplianceControl{},
		frameworks: map[uuid.UUID]domain.ComplianceFramework{fw: {ID: fw, TenantID: tenant, Name: "ISO/IEC 27001", Version: "2022"}},
	}
	c1, c2 := uuid.New(), uuid.New()
	fc.controls[c1] = domain.ComplianceControl{ID: c1, TenantID: tenant, FrameworkID: fw, ReferenceCode: "A.5.1", Name: "Policies", Status: domain.ControlStatusImplemented}
	fc.controls[c2] = domain.ComplianceControl{ID: c2, TenantID: tenant, FrameworkID: fw, ReferenceCode: "A.8.2", Name: "Privileged access", Status: domain.ControlStatusInProgress}

	repo, store := newFakeRepo(), newFakeStorage()
	svc := NewService(repo, fc, store).WithClock(func() time.Time { return now })
	return &fixture{svc: svc, repo: repo, controls: fc, store: store, tenant: tenant, other: other, fw: fw, c1: c1, c2: c2, now: now}
}

func TestCreate_LinksToSeveralControls(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	ev, err := f.svc.Create(ctx, f.tenant, CreateInput{
		Title:      "ISO certificate 2026",
		Type:       "attestation",
		Filename:   "iso.pdf",
		Content:    strings.NewReader("PDF"),
		ValidUntil: evAt(f.now.Add(365 * 24 * time.Hour)),
		ControlIDs: []uuid.UUID{f.c1, f.c2},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(ev.ControlIDs) != 2 {
		t.Fatalf("one artifact should answer both controls, got %d", len(ev.ControlIDs))
	}
	if ev.Status != domain.EvidenceStatusValid {
		t.Fatalf("status should be derived as valid, got %q", ev.Status)
	}
	// The labels come back so the UI need not re-resolve each control.
	if len(ev.Controls) != 2 || ev.Controls[0].FrameworkName != "ISO/IEC 27001" {
		t.Fatalf("linked controls should be labelled with their framework: %+v", ev.Controls)
	}
}

func TestCreate_RejectsControlFromAnotherTenant(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	// A control that exists, but not for this tenant.
	foreign := uuid.New()
	f.controls.controls[foreign] = domain.ComplianceControl{ID: foreign, TenantID: f.other, FrameworkID: uuid.New()}

	_, err := f.svc.Create(ctx, f.tenant, CreateInput{
		Title: "leak", Description: "x", ControlIDs: []uuid.UUID{f.c1, foreign},
	})
	if err == nil {
		t.Fatal("linking to another tenant's control must be refused")
	}
	// And nothing may have been written — a partial create would leave proof
	// attached to some controls and not others, with no signal to the user.
	if len(f.repo.items) != 0 {
		t.Fatalf("refused create must write nothing, found %d rows", len(f.repo.items))
	}
	if len(f.store.saved) != 0 {
		t.Fatal("refused create must not touch storage")
	}
}

func TestCreate_RefusesEmptyEvidence(t *testing.T) {
	f := setup(t)
	// No file, no link, no words: a row asserting a control is covered by nothing.
	_, err := f.svc.Create(context.Background(), f.tenant, CreateInput{Title: "nothing"})
	if err == nil {
		t.Fatal("evidence with no file, link or description must be refused")
	}
}

func TestCreate_RefusesExpiryBeforeCollection(t *testing.T) {
	f := setup(t)
	_, err := f.svc.Create(context.Background(), f.tenant, CreateInput{
		Title: "already dead", Description: "x",
		CollectedAt: evAt(f.now), ValidUntil: evAt(f.now.Add(-time.Hour)),
	})
	if err == nil {
		t.Fatal("proof that expired before it was collected must be refused")
	}
}

func TestCreate_CleansUpStoredFileWhenTheRowFails(t *testing.T) {
	f := setup(t)
	f.repo.failNext = domain.NewInternalError("db down")

	_, err := f.svc.Create(context.Background(), f.tenant, CreateInput{
		Title: "doc", Filename: "a.pdf", Content: strings.NewReader("bytes"),
	})
	if err == nil {
		t.Fatal("expected the create to fail")
	}
	if len(f.store.deleted) != 1 {
		t.Fatal("a failed row must not leave an orphaned blob in storage")
	}
}

// Reuse is the whole point: attach an existing artifact to a second framework's
// control without re-uploading anything.
func TestLink_ReusesOneArtifactAcrossFrameworks(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	ev, err := f.svc.Create(ctx, f.tenant, CreateInput{
		Title: "Pentest", Description: "annual test", ControlIDs: []uuid.UUID{f.c1},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// A control in a different framework.
	fw2 := uuid.New()
	f.controls.frameworks[fw2] = domain.ComplianceFramework{ID: fw2, TenantID: f.tenant, Name: "SOC 2"}
	c3 := uuid.New()
	f.controls.controls[c3] = domain.ComplianceControl{ID: c3, TenantID: f.tenant, FrameworkID: fw2, ReferenceCode: "CC6.1", Name: "Access"}

	got, err := f.svc.Link(ctx, f.tenant, ev.ID, []uuid.UUID{c3}, "same test covers both", uuid.New())
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	if len(got.ControlIDs) != 2 {
		t.Fatalf("artifact should now answer 2 controls, got %d", len(got.ControlIDs))
	}
	if len(f.repo.items) != 1 {
		t.Fatalf("reuse must not duplicate the artifact, found %d", len(f.repo.items))
	}

	// Unlinking one side leaves the artifact and the other link alone.
	if err := f.svc.Unlink(ctx, f.tenant, ev.ID, c3); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	got, _ = f.svc.Get(ctx, f.tenant, ev.ID)
	if len(got.ControlIDs) != 1 {
		t.Fatalf("expected 1 remaining link, got %d", len(got.ControlIDs))
	}
}

func TestGetAndDelete_AreTenantScoped(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	ev, _ := f.svc.Create(ctx, f.tenant, CreateInput{Title: "mine", Description: "x"})

	if _, err := f.svc.Get(ctx, f.other, ev.ID); err == nil {
		t.Fatal("another tenant must not read this artifact")
	}
	if err := f.svc.Delete(ctx, f.other, ev.ID); err == nil {
		t.Fatal("another tenant must not delete this artifact")
	}
	if _, err := f.svc.Get(ctx, f.tenant, ev.ID); err != nil {
		t.Fatalf("owner must still have it: %v", err)
	}
}

func TestReview_RejectionMustSayWhy(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	ev, _ := f.svc.Create(ctx, f.tenant, CreateInput{Title: "doc", Description: "x", ControlIDs: []uuid.UUID{f.c1}})

	if _, err := f.svc.Review(ctx, f.tenant, ev.ID, uuid.New(), "rejected", "   "); err == nil {
		t.Fatal("a rejection with no reason must be refused — nobody could act on it")
	}

	got, err := f.svc.Review(ctx, f.tenant, ev.ID, uuid.New(), "rejected", "screenshot is illegible")
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if got.Status != domain.EvidenceStatusRejected {
		t.Fatalf("status should read rejected, got %q", got.Status)
	}

	// And it must stop counting as coverage immediately.
	counts, _ := f.repo.CountCoveringByFramework(ctx, f.tenant, f.fw, f.now)
	if counts[f.c1] != 0 {
		t.Fatalf("rejected proof must not cover its control, got %d", counts[f.c1])
	}
}

// The tabs above the list must count the whole filtered set, not the page.
func TestList_SummaryCountsBeyondThePage(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	mk := func(title string, collected time.Time, until *time.Time, review string) {
		if _, err := f.svc.Create(ctx, f.tenant, CreateInput{
			Title: title, Description: "x", CollectedAt: &collected, ValidUntil: until, Review: review,
		}); err != nil {
			t.Fatalf("create %s: %v", title, err)
		}
	}
	lastYear := f.now.Add(-365 * 24 * time.Hour)
	mk("valid-1", f.now, nil, "")
	mk("valid-2", f.now, evAt(f.now.Add(200*24*time.Hour)), "")
	mk("expiring", f.now, evAt(f.now.Add(10*24*time.Hour)), "")
	// Collected a year ago and expired yesterday — the only honest way to have an
	// expired artifact, since Create refuses proof that was never valid.
	mk("expired", lastYear, evAt(f.now.Add(-24*time.Hour)), "")
	mk("rejected", f.now, nil, "rejected")

	res, err := f.svc.List(ctx, f.tenant, domain.EvidenceFilter{Limit: 2})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("page should hold 2, got %d", len(res.Items))
	}
	if res.Total != 5 {
		t.Fatalf("total should be 5, got %d", res.Total)
	}
	s := res.Summary
	if s.Valid != 2 || s.Expiring != 1 || s.Expired != 1 || s.Rejected != 1 {
		t.Fatalf("summary must span the filter, not the page: %+v", s)
	}
}

func TestDownload_RefusesEvidenceThatHoldsNoFile(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	// Evidence recorded as a link to another system: there are no bytes to serve,
	// and saying so beats a 500 out of the storage layer.
	ev, _ := f.svc.Create(ctx, f.tenant, CreateInput{Title: "in Confluence", ExternalURL: "https://wiki/x"})

	if _, _, err := f.svc.Download(ctx, f.tenant, ev.ID); err == nil {
		t.Fatal("expected a validation error for evidence with no file")
	}
}

func evAt(t time.Time) *time.Time { return &t }
