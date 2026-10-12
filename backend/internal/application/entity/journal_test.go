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

// The tenant feed read as a journal (#905).

type namedLookup struct {
	fakeLookup
	names map[uuid.UUID]string
}

func (n *namedLookup) NamesByIDs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		if v, ok := n.names[id]; ok {
			out[id] = v
		}
	}
	return out, nil
}

func journalWorld(t *testing.T) *world {
	w := newWorld(t)
	mit := auditEvent(w.tenantA, "mitigation", uuid.NewString(), domain.AuditActionCreate, at(6))
	mit.Summary = "create mitigation Bastion d'administration (PAM)"
	w.audit.events = []domain.AuditEvent{
		auditEvent(w.tenantA, "risk", w.riskA.ID.String(), domain.AuditActionUpdate, at(7)),
		mit,
		auditEvent(w.tenantA, "report", uuid.NewString(), domain.AuditActionCreate, at(5)),
		auditEvent(w.tenantA, "automation_rule", uuid.NewString(), domain.AuditActionUpdate, at(4)),
		auditEvent(w.tenantA, "celebrated", uuid.NewString(), domain.AuditActionCreate, at(3)),
		auditEvent(w.tenantA, "notification", uuid.NewString(), domain.AuditActionUpdate, at(2)),
		auditEvent(w.tenantB, "mitigation", uuid.NewString(), domain.AuditActionCreate, at(8)),
	}
	return w
}

func domainsOf(events []TimelineEvent) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e.Domain)
	}
	return out
}

func TestJournal_Success(t *testing.T) {
	w := journalWorld(t)
	page, err := w.svc.TenantTimeline(context.Background(), w.admin(w.tenantA), "", TimelineFilter{})
	if err != nil {
		t.Fatal(err)
	}
	got := domainsOf(page.Events)
	want := []string{DomainRisk, DomainMitigation, DomainReport, DomainGovernance}
	if len(got) != len(want) {
		t.Fatalf("journal = %v, want %v (technical rows and tenant B stay out)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("journal = %v, want %v", got, want)
		}
	}
	m := page.Events[1]
	if m.Object != "Bastion d'administration (PAM)" || m.TargetURL != "/mitigations" {
		t.Errorf("mitigation entry = object %q link %q", m.Object, m.TargetURL)
	}
}

// The domain filter is pushed into the query, so a filtered page is full.
func TestJournal_FiltersByDomain(t *testing.T) {
	w := journalWorld(t)
	page, err := w.svc.TenantTimeline(context.Background(), w.admin(w.tenantA), "", TimelineFilter{Domain: DomainMitigation})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 1 || page.Events[0].Domain != DomainMitigation {
		t.Fatalf("mitigation filter returned %v", domainsOf(page.Events))
	}
	if !containsStr(w.audit.lastFilter.EntityTypes, "mitigation") {
		t.Errorf("the domain did not reach the query: %v", w.audit.lastFilter.EntityTypes)
	}
}

func TestJournal_NotFound(t *testing.T) {
	w := journalWorld(t)
	_, err := w.svc.TenantTimeline(context.Background(), w.admin(w.tenantA), "", TimelineFilter{Domain: "payroll"})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("an unknown domain must be a validation error, got %v", err)
	}
}

// Visibility follows the page each object belongs to.
func TestJournal_Unauthorized(t *testing.T) {
	w := journalWorld(t)

	risksOnly, err := w.svc.TenantTimeline(context.Background(), callerIn(w.tenantA, "risks:read"), "", TimelineFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if got := domainsOf(risksOnly.Events); len(got) != 1 || got[0] != DomainRisk {
		t.Fatalf("risks:read saw %v", got)
	}

	withMitigations, err := w.svc.TenantTimeline(context.Background(), callerIn(w.tenantA, "risks:read", "mitigations:read"), "", TimelineFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if got := domainsOf(withMitigations.Events); len(got) != 2 || got[1] != DomainMitigation {
		t.Fatalf("mitigations:read saw %v", got)
	}

	if _, err := w.svc.TenantTimeline(context.Background(), Caller{}, "", TimelineFilter{}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("a caller without a session must be refused, got %v", err)
	}
}

// Two writers record one mutation; the journal shows it once, from the row
// that carries the record.
func TestJournal_MergesTheTwoWriters(t *testing.T) {
	w := newWorld(t)
	httpRow := auditEvent(w.tenantA, "incident", "5", domain.AuditActionCreate, at(3))
	httpRow.Source = "http"
	httpRow.Summary = "create incident 5"
	actor := uuid.New()
	httpRow.ActorID = &actor
	modelRow := auditEvent(w.tenantA, "incident", "5", domain.AuditActionCreate, at(3).Add(-40*time.Millisecond))
	modelRow.Source = "gorm"
	modelRow.Summary = "create incident Indisponibilité du canal USSD"
	modelRow.After = domain.JSONMap{"title": "Indisponibilité du canal USSD"}
	later := auditEvent(w.tenantA, "incident", "5", domain.AuditActionCreate, at(9))
	later.Source = "http"
	w.audit.events = []domain.AuditEvent{httpRow, modelRow, later}

	page, err := w.svc.TenantTimeline(context.Background(), w.admin(w.tenantA), "", TimelineFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 2 {
		t.Fatalf("got %v, want the model row and the unrelated later row", summariesOf(page.Events))
	}
	for _, e := range page.Events {
		if e.Summary == "create incident 5" && e.OccurredAt.Equal(at(3)) {
			t.Fatal("the HTTP twin survived")
		}
		if e.Object == "Indisponibilité du canal USSD" && (e.Actor == nil || e.Actor.ID != actor.String()) {
			t.Fatalf("the person who acted was lost in the merge: %+v", e.Actor)
		}
	}
}

func TestObjectLabel(t *testing.T) {
	cases := []struct {
		name string
		e    domain.AuditEvent
		want string
	}{
		{"title in the snapshot", domain.AuditEvent{After: domain.JSONMap{"title": "Rançongiciel"}, Summary: "update risk x"}, "Rançongiciel"},
		{"name before deletion", domain.AuditEvent{Before: domain.JSONMap{"name": "Base clients"}}, "Base clients"},
		{"quoted in the summary", domain.AuditEvent{Action: "defer", EntityType: "approval_request", Summary: `defer "Budget du PRA" to the next committee (2 records affected)`}, "Budget du PRA"},
		{"the request, not the step", domain.AuditEvent{Action: "approve", EntityType: "approval_request", Summary: `approve step "Comité des risques" of "Prolongation de l'exception" → approved (2 records affected)`}, "Prolongation de l'exception"},
		{"submitted for approval", domain.AuditEvent{Action: "submit", EntityType: "approval_request", Summary: "submitted risk_acceptance for approval: Acceptation du risque R-0115 (2 records affected)"}, "Acceptation du risque R-0115"},
		{"writer phrasing", domain.AuditEvent{Action: "create", EntityType: "compliance_control", Summary: "create compliance_control Policies (94 records affected)"}, "Policies"},
		{"a bare id is no name", domain.AuditEvent{Action: "create", EntityType: "incident", Summary: "create incident 5"}, ""},
		{"a uuid is no name", domain.AuditEvent{Action: "create", EntityType: "board", Summary: "create board f590a0bb-85db-4a23-99e9-d083d2acf495"}, ""},
	}
	for _, c := range cases {
		if got := objectLabel(c.e); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestJournal_ActorsReadByName(t *testing.T) {
	w := newWorld(t)
	actor := uuid.New()
	ev := auditEvent(w.tenantA, "risk", w.riskA.ID.String(), domain.AuditActionUpdate, at(1))
	ev.ActorID = &actor
	w.audit.events = []domain.AuditEvent{ev}
	lookup := &namedLookup{
		fakeLookup: fakeLookup{emails: map[uuid.UUID]string{actor: "fatou@a.example"}},
		names:      map[uuid.UUID]string{actor: "Fatou Ndiaye"},
	}
	w.svc = w.svc.WithTimeline(NewTimelineService(w.audit).WithUserLookup(lookup))

	page, err := w.svc.TenantTimeline(context.Background(), w.admin(w.tenantA), "", TimelineFilter{})
	if err != nil {
		t.Fatal(err)
	}
	a := page.Events[0].Actor
	if a == nil || a.Label != "Fatou Ndiaye" || a.Email != "fatou@a.example" {
		t.Fatalf("actor = %+v", a)
	}
}
