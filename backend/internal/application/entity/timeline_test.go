// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

func at(min int) time.Time {
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	return base.Add(time.Duration(min) * time.Minute)
}

// ---------------------------------------------------------------------------
// Ordering (§20)
// ---------------------------------------------------------------------------

func TestTimeline_IsNewestFirst(t *testing.T) {
	w := newWorld(t)
	id := w.riskA.ID.String()
	w.audit.events = []domain.AuditEvent{
		auditEvent(w.tenantA, "risk", id, domain.AuditActionCreate, at(0)),
		auditEvent(w.tenantA, "risk", id, domain.AuditActionUpdate, at(10)),
		auditEvent(w.tenantA, "risk", id, domain.AuditActionUpdate, at(5)),
	}

	page, err := w.svc.Timeline(context.Background(), w.admin(w.tenantA), TypeRisk, id, "", TimelineFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 3 {
		t.Fatalf("got %d events, want 3", len(page.Events))
	}
	for i := 1; i < len(page.Events); i++ {
		if page.Events[i].OccurredAt.After(page.Events[i-1].OccurredAt) {
			t.Fatalf("event %d is newer than event %d — the feed is not newest-first", i, i-1)
		}
	}
}

// ---------------------------------------------------------------------------
// Merging (§18)
// ---------------------------------------------------------------------------

// The score worker's history entries and the audit trail's HTTP entries appear
// in ONE ordered stream. A risk timeline made of the trail alone would be
// missing every score movement, which is the most interesting thing that
// happens to a risk.
func TestTimeline_MergesTheSupplementaryJournal(t *testing.T) {
	w := newWorld(t)
	id := w.riskA.ID.String()
	w.audit.events = []domain.AuditEvent{
		auditEvent(w.tenantA, "risk", id, domain.AuditActionUpdate, at(10)),
		auditEvent(w.tenantA, "risk", id, domain.AuditActionCreate, at(0)),
	}
	history := &fakeSource{
		name: SourceRiskHistory,
		events: []TimelineEvent{
			{ID: "risk_history:1", Kind: "update", OccurredAt: at(5),
				Summary: "Score recorded at 25.65", Source: SourceRiskHistory},
		},
	}
	w.svc = w.svc.WithTimeline(NewTimelineService(w.audit).WithSource(TypeRisk, history))

	page, err := w.svc.Timeline(context.Background(), w.admin(w.tenantA), TypeRisk, id, "", TimelineFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 3 {
		t.Fatalf("got %d events, want the trail's 2 plus the journal's 1", len(page.Events))
	}
	if page.Events[1].Source != SourceRiskHistory {
		t.Errorf("the journal entry did not interleave by time: order = %v", sourcesOf(page.Events))
	}
	if !containsSource(page.Sources, SourceRiskHistory) {
		t.Error("the page does not say the journal was read")
	}
	// The journal must have been queried with the CALLER's tenant.
	if history.lastTenant != w.tenantA {
		t.Errorf("journal queried tenant %s, want %s", history.lastTenant, w.tenantA)
	}
}

// A supplementary journal that fails degrades that source only. The canonical
// trail is still worth showing.
func TestTimeline_FailingJournalDegradesToTheTrail(t *testing.T) {
	w := newWorld(t)
	id := w.riskA.ID.String()
	w.audit.events = []domain.AuditEvent{
		auditEvent(w.tenantA, "risk", id, domain.AuditActionUpdate, at(10)),
	}
	broken := &fakeSource{name: SourceRiskHistory, err: errors.New("journal unavailable")}
	w.svc = w.svc.WithTimeline(NewTimelineService(w.audit).WithSource(TypeRisk, broken))

	page, err := w.svc.Timeline(context.Background(), w.admin(w.tenantA), TypeRisk, id, "", TimelineFilter{})
	if err != nil {
		t.Fatalf("a failing journal failed the whole timeline: %v", err)
	}
	if len(page.Events) != 1 {
		t.Fatalf("got %d events, want the trail's 1", len(page.Events))
	}
	if containsSource(page.Sources, SourceRiskHistory) {
		t.Error("the page claims to have read a journal that failed")
	}
}

// ---------------------------------------------------------------------------
// Pagination (§21)
// ---------------------------------------------------------------------------

func TestTimeline_CursorPagesWithoutSkippingOrRepeating(t *testing.T) {
	w := newWorld(t)
	id := w.riskA.ID.String()
	for i := 0; i < 7; i++ {
		w.audit.events = append(w.audit.events,
			auditEvent(w.tenantA, "risk", id, domain.AuditActionUpdate, at(i)))
	}

	seen := map[string]int{}
	cursor := ""
	pages := 0
	for {
		page, err := w.svc.Timeline(context.Background(), w.admin(w.tenantA), TypeRisk, id, cursor, TimelineFilter{Limit: 3})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range page.Events {
			seen[e.ID]++
		}
		pages++
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
		if pages > 10 {
			t.Fatal("pagination did not terminate")
		}
	}
	if len(seen) != 7 {
		t.Fatalf("saw %d distinct events across %d pages, want 7", len(seen), pages)
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("event %s returned %d times", id, n)
		}
	}
}

// Several events can share a timestamp — an import writes twenty rows in the
// same millisecond. A time-only cursor would skip or repeat them.
func TestTimeline_CursorHandlesIdenticalTimestamps(t *testing.T) {
	w := newWorld(t)
	id := w.riskA.ID.String()
	same := at(3)
	for i := 0; i < 5; i++ {
		w.audit.events = append(w.audit.events,
			auditEvent(w.tenantA, "risk", id, domain.AuditActionUpdate, same))
	}

	seen := map[string]int{}
	cursor := ""
	for i := 0; i < 10; i++ {
		page, err := w.svc.Timeline(context.Background(), w.admin(w.tenantA), TypeRisk, id, cursor, TimelineFilter{Limit: 2})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range page.Events {
			seen[e.ID]++
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if len(seen) != 5 {
		t.Fatalf("saw %d of 5 same-timestamp events", len(seen))
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("event %s returned %d times across pages", id, n)
		}
	}
}

func TestTimeline_RejectsAMalformedCursor(t *testing.T) {
	w := newWorld(t)
	_, err := w.svc.Timeline(context.Background(), w.admin(w.tenantA), TypeRisk, w.riskA.ID.String(), "!!!not-base64!!!", TimelineFilter{})
	if err == nil {
		t.Fatal("a malformed cursor was accepted")
	}
	if got := domain.HTTPStatusFromError(err); got != 400 {
		t.Fatalf("malformed cursor answered %d, want 400", got)
	}
}

func TestCursor_RoundTrips(t *testing.T) {
	want := cursor{at: at(42).UTC(), id: "abc-123"}
	got, err := decodeCursor(encodeCursor(want))
	if err != nil {
		t.Fatal(err)
	}
	if !got.at.Equal(want.at) || got.id != want.id {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}

// ---------------------------------------------------------------------------
// Empty and missing
// ---------------------------------------------------------------------------

func TestTimeline_EmptyIsAnEmptyPageNotAnError(t *testing.T) {
	w := newWorld(t)
	page, err := w.svc.Timeline(context.Background(), w.admin(w.tenantA), TypeRisk, w.riskA.ID.String(), "", TimelineFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 0 {
		t.Fatalf("got %d events, want none", len(page.Events))
	}
	if page.NextCursor != "" {
		t.Error("an empty page offered a next cursor")
	}
	if page.Events == nil {
		t.Error("events is nil rather than an empty list; the client would have to guard for both")
	}
}

func TestTimeline_UnavailableWhenNotWired(t *testing.T) {
	w := newWorld(t)
	svc := NewService(NewRegistry().Register(TypeRisk, NewRiskResolver(newFakeRisks(), newFakeRelations())))
	_ = svc
	// Service with a resolver but no timeline reader.
	risks := newFakeRisks()
	r := risks.add(&domain.Risk{TenantID: w.tenantA, Name: "r"})
	bare := NewService(NewRegistry().Register(TypeRisk, NewRiskResolver(risks, newFakeRelations())))
	_, err := bare.Timeline(context.Background(), w.admin(w.tenantA), TypeRisk, r.ID.String(), "", TimelineFilter{})
	if err == nil {
		t.Fatal("a timeline read with no reader wired succeeded")
	}
	if domain.HTTPStatusFromError(err) == 500 {
		t.Error("an unwired timeline crashed rather than answering a typed error")
	}
}

// ---------------------------------------------------------------------------
// Tenant isolation
// ---------------------------------------------------------------------------

func TestTimeline_CrossTenantIsNotFound(t *testing.T) {
	w := newWorld(t)
	_, err := w.svc.Timeline(context.Background(), w.admin(w.tenantA), TypeRisk, w.riskB.ID.String(), "", TimelineFilter{})
	if got := domain.HTTPStatusFromError(err); got != 404 {
		t.Fatalf("timeline for another tenant's risk answered %d, want 404", got)
	}
}

// Even for an id the caller CAN read, the trail query must carry their tenant.
func TestTimeline_QueriesTheCallersTenantOnly(t *testing.T) {
	w := newWorld(t)
	id := w.riskA.ID.String()
	w.audit.events = []domain.AuditEvent{
		auditEvent(w.tenantA, "risk", id, domain.AuditActionUpdate, at(1)),
		// Same entity id planted under the other tenant.
		auditEvent(w.tenantB, "risk", id, domain.AuditActionDelete, at(2)),
	}
	page, err := w.svc.Timeline(context.Background(), w.admin(w.tenantA), TypeRisk, id, "", TimelineFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 1 {
		t.Fatalf("got %d events, want only the caller's tenant's one", len(page.Events))
	}
	if page.Events[0].Kind != string(domain.AuditActionUpdate) {
		t.Errorf("the other tenant's event surfaced: %+v", page.Events[0])
	}
}

// ---------------------------------------------------------------------------
// Timeline is not the audit trail (§23)
// ---------------------------------------------------------------------------

// The timeline says a field moved. It must not carry the before/after snapshot,
// which can hold values a timeline reader is not cleared for.
func TestTimeline_CarriesChangedFieldsButNotSnapshots(t *testing.T) {
	w := newWorld(t)
	id := w.riskA.ID.String()
	ev := auditEvent(w.tenantA, "risk", id, domain.AuditActionUpdate, at(1))
	ev.ChangedFields = domain.StringList{"criticality"}
	ev.Before = domain.JSONMap{"criticality": "medium", "internal_note": "board escalation pending"}
	ev.After = domain.JSONMap{"criticality": "high"}
	w.audit.events = []domain.AuditEvent{ev}

	page, err := w.svc.Timeline(context.Background(), w.admin(w.tenantA), TypeRisk, id, "", TimelineFilter{})
	if err != nil {
		t.Fatal(err)
	}
	got := page.Events[0]
	if len(got.Changes) != 1 || got.Changes[0].Field != "criticality" {
		t.Fatalf("changed fields = %+v, want the field name", got.Changes)
	}
	if got.Changes[0].From != "" || got.Changes[0].To != "" {
		t.Errorf("the timeline carried a before/after value: %+v — that belongs to the audit tab", got.Changes[0])
	}

	// The audit endpoint, behind its own permission, DOES carry them.
	audit, err := w.svc.Audit(context.Background(), callerIn(w.tenantA, "*"), TypeRisk, id, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(audit.Events) != 1 || audit.Events[0].Before == nil {
		t.Fatal("the audit trail dropped the before snapshot it exists to carry")
	}
}

// ---------------------------------------------------------------------------
// Tenant-wide feed (§35)
// ---------------------------------------------------------------------------

func TestTenantTimeline_FiltersByPermission(t *testing.T) {
	w := newWorld(t)
	w.audit.events = []domain.AuditEvent{
		auditEvent(w.tenantA, "risk", w.riskA.ID.String(), domain.AuditActionUpdate, at(3)),
		auditEvent(w.tenantA, "asset", w.assetA.ID.String(), domain.AuditActionUpdate, at(2)),
		auditEvent(w.tenantA, "automation_rule", uuid.New().String(), domain.AuditActionUpdate, at(1)),
	}

	// A caller who may read risks only.
	page, err := w.svc.TenantTimeline(context.Background(), callerIn(w.tenantA, "risks:read"), "", TimelineFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 1 {
		t.Fatalf("got %d events, want only the risk one: %v", len(page.Events), summariesOf(page.Events))
	}
	if page.Events[0].Target.Type != TypeRisk {
		t.Errorf("wrong event survived: %+v", page.Events[0])
	}

	// A governance auditor sees the lot, including entities with no drawer.
	all, err := w.svc.TenantTimeline(context.Background(), callerIn(w.tenantA, AuditPermission), "", TimelineFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Events) != 3 {
		t.Fatalf("an audit-permission holder saw %d of 3 events", len(all.Events))
	}
}

func TestTenantTimeline_NeverCrossesTenants(t *testing.T) {
	w := newWorld(t)
	w.audit.events = []domain.AuditEvent{
		auditEvent(w.tenantA, "risk", w.riskA.ID.String(), domain.AuditActionUpdate, at(2)),
		auditEvent(w.tenantB, "risk", w.riskB.ID.String(), domain.AuditActionUpdate, at(3)),
	}
	page, err := w.svc.TenantTimeline(context.Background(), w.admin(w.tenantA), "", TimelineFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 1 {
		t.Fatalf("got %d events, want only tenant A's", len(page.Events))
	}
	if page.Events[0].Target.ID != w.riskA.ID.String() {
		t.Error("another tenant's event appeared in the feed")
	}
}

// Every event in the feed deep-links to its subject, which is what makes the
// feed navigable (§36).
func TestTenantTimeline_EventsDeepLinkToTheirTarget(t *testing.T) {
	w := newWorld(t)
	w.audit.events = []domain.AuditEvent{
		auditEvent(w.tenantA, "risk", w.riskA.ID.String(), domain.AuditActionUpdate, at(1)),
	}
	page, err := w.svc.TenantTimeline(context.Background(), w.admin(w.tenantA), "", TimelineFilter{})
	if err != nil {
		t.Fatal(err)
	}
	want := "/risks?drawer=risk&entity=" + w.riskA.ID.String()
	if page.Events[0].TargetURL != want {
		t.Errorf("target URL = %q, want %q", page.Events[0].TargetURL, want)
	}
}

func TestTenantTimeline_RefusesATenantlessCaller(t *testing.T) {
	w := newWorld(t)
	c := Caller{UserID: uuid.New(), Perms: perms("*")}
	if _, err := w.svc.TenantTimeline(context.Background(), c, "", TimelineFilter{}); err == nil {
		t.Fatal("a tenant-less caller read the tenant feed")
	}
}

// ---------------------------------------------------------------------------
// Filtering (§22)
// ---------------------------------------------------------------------------

func TestTimeline_FiltersByKind(t *testing.T) {
	w := newWorld(t)
	id := w.riskA.ID.String()
	w.audit.events = []domain.AuditEvent{
		auditEvent(w.tenantA, "risk", id, domain.AuditActionCreate, at(1)),
		auditEvent(w.tenantA, "risk", id, domain.AuditActionUpdate, at(2)),
	}
	page, err := w.svc.Timeline(context.Background(), w.admin(w.tenantA), TypeRisk, id, "",
		TimelineFilter{Kind: string(domain.AuditActionCreate)})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 1 || page.Events[0].Kind != "create" {
		t.Fatalf("kind filter returned %v", summariesOf(page.Events))
	}
}

func TestTimeline_FiltersByWindow(t *testing.T) {
	w := newWorld(t)
	id := w.riskA.ID.String()
	w.audit.events = []domain.AuditEvent{
		auditEvent(w.tenantA, "risk", id, domain.AuditActionUpdate, at(1)),
		auditEvent(w.tenantA, "risk", id, domain.AuditActionUpdate, at(30)),
	}
	since := at(10)
	page, err := w.svc.Timeline(context.Background(), w.admin(w.tenantA), TypeRisk, id, "",
		TimelineFilter{Since: &since})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 1 {
		t.Fatalf("since filter returned %d events, want 1", len(page.Events))
	}
}

// ---------------------------------------------------------------------------
// Actor resolution
// ---------------------------------------------------------------------------

func TestTimeline_ResolvesActorEmailsInOneLookup(t *testing.T) {
	w := newWorld(t)
	id := w.riskA.ID.String()
	actor := uuid.New()
	for i := 0; i < 3; i++ {
		ev := auditEvent(w.tenantA, "risk", id, domain.AuditActionUpdate, at(i))
		a := actor
		ev.ActorID = &a
		w.audit.events = append(w.audit.events, ev)
	}
	lookup := &fakeLookup{emails: map[uuid.UUID]string{actor: "alice@a.example"}}
	w.svc = w.svc.WithTimeline(NewTimelineService(w.audit).WithUserLookup(lookup))

	page, err := w.svc.Timeline(context.Background(), w.admin(w.tenantA), TypeRisk, id, "", TimelineFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range page.Events {
		if e.Actor == nil || e.Actor.Email != "alice@a.example" {
			t.Fatalf("actor not resolved: %+v", e.Actor)
		}
	}
	if lookup.calls != 1 {
		t.Errorf("actor lookup ran %d times for one page — that is an N+1", lookup.calls)
	}
}

// A lookup failure is a display problem, never a reason to fail a read.
func TestTimeline_LookupFailureDoesNotFailTheRead(t *testing.T) {
	w := newWorld(t)
	id := w.riskA.ID.String()
	ev := auditEvent(w.tenantA, "risk", id, domain.AuditActionUpdate, at(1))
	actor := uuid.New()
	ev.ActorID = &actor
	w.audit.events = []domain.AuditEvent{ev}

	w.svc = w.svc.WithTimeline(NewTimelineService(w.audit).
		WithUserLookup(&fakeLookup{err: errors.New("user store down")}))

	page, err := w.svc.Timeline(context.Background(), w.admin(w.tenantA), TypeRisk, id, "", TimelineFilter{})
	if err != nil {
		t.Fatalf("an actor lookup failure failed the read: %v", err)
	}
	if len(page.Events) != 1 {
		t.Fatal("events lost")
	}
	if page.Events[0].Actor == nil || page.Events[0].Actor.ID != actor.String() {
		t.Error("the actor id was dropped along with the failed email lookup")
	}
}

// ---------------------------------------------------------------------------
// Entity-type aliasing
// ---------------------------------------------------------------------------

func TestAuditTypes_CoverBothWritersNames(t *testing.T) {
	got := auditTypes(TypeControl)
	if !containsStr(got, "compliance_control") || !containsStr(got, "control") {
		t.Fatalf("control aliases = %v, want both writers' names", got)
	}
	if a, b := auditTypes(TypeVendor), auditTypes(TypeAsset); len(a) != 1 || a[0] != b[0] {
		t.Errorf("a vendor's trail must be read as an asset's: %v vs %v", a, b)
	}
	if got := auditTypes(TypeFinding); len(got) != 1 || got[0] != "vulnerability" {
		t.Errorf("finding aliases = %v, want vulnerability", got)
	}
}

func TestTypeForAuditEntity_IsTheInverse(t *testing.T) {
	for _, name := range []string{"asset", "risk", "vulnerability", "compliance_control", "incident", "evidence"} {
		if _, ok := typeForAuditEntity(name); !ok {
			t.Errorf("%q does not map back to a drawer type", name)
		}
	}
	if _, ok := typeForAuditEntity("automation_rule"); ok {
		t.Error("an entity with no drawer mapped to one")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func sourcesOf(events []TimelineEvent) []TimelineSource {
	out := make([]TimelineSource, 0, len(events))
	for _, e := range events {
		out = append(out, e.Source)
	}
	return out
}

func summariesOf(events []TimelineEvent) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e.Summary)
	}
	return out
}

func containsSource(list []TimelineSource, want TimelineSource) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
