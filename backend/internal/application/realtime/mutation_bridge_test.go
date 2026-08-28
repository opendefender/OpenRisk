// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package realtime

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	rt "github.com/opendefender/openrisk/pkg/realtime"
)

// observe drives the bridge through a REAL Fiber route, because the route
// pattern is where the trailing verb lives and a hand-built context would not
// have one — which is exactly the part that decides whether a transition is
// published as a status change or as a creation.
func observe(t *testing.T, routePattern, requestPath string, ev *domain.AuditEvent) []rt.Envelope {
	t.Helper()
	pub := &capturePublisher{}
	bridge := NewMutationBridge(pub)

	app := fiber.New()
	app.Post(routePattern, func(c *fiber.Ctx) error {
		bridge.ObserveMutation(c, ev)
		return c.SendStatus(fiber.StatusOK)
	})
	app.Patch(routePattern, func(c *fiber.Ctx) error {
		bridge.ObserveMutation(c, ev)
		return c.SendStatus(fiber.StatusOK)
	})
	app.Delete(routePattern, func(c *fiber.Ctx) error {
		bridge.ObserveMutation(c, ev)
		return c.SendStatus(fiber.StatusOK)
	})

	method := fiber.MethodPost
	switch ev.Action {
	case domain.AuditActionUpdate:
		method = fiber.MethodPatch
	case domain.AuditActionDelete:
		method = fiber.MethodDelete
	}
	resp, err := app.Test(httptest.NewRequest(method, requestPath, nil))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	return pub.sent
}

func auditEvent(tenant uuid.UUID, entity, entityID string, action domain.AuditAction) *domain.AuditEvent {
	actor := uuid.New()
	return &domain.AuditEvent{
		ID:            uuid.New(),
		TenantID:      tenant,
		ActorID:       &actor,
		Action:        action,
		EntityType:    entity,
		EntityID:      entityID,
		RequestID:     "req-1",
		ChangedFields: domain.StringList{"status"},
	}
}

// Each domain W0-07 must cover produces its canonical event from an ordinary
// mutation — no publish call in the use case, no possibility of forgetting one.
func TestMutationBridge_MapsEachDomainOntoItsCanonicalEvent(t *testing.T) {
	tenant := uuid.New()
	id := uuid.NewString()

	// parentID is the id that appears IN the path; entityID is the id the row
	// layer reported. For a creation under a sub-resource the two differ — the
	// path names the parent, the event names the thing that was created — and
	// that difference is exactly what tells a creation from an update.
	parent := uuid.NewString()

	cases := []struct {
		name     string
		route    string
		path     string
		entity   string
		entityID string
		action   domain.AuditAction
		want     rt.EventType
		wantAggr string
	}{
		{"risk created", "/risks", "/risks", "risk", id, domain.AuditActionCreate, rt.RiskCreated, rt.AggregateRisk},
		{"risk updated", "/risks/:id", "/risks/" + id, "risk", id, domain.AuditActionUpdate, rt.RiskUpdated, rt.AggregateRisk},
		{"risk deleted", "/risks/:id", "/risks/" + id, "risk", id, domain.AuditActionDelete, rt.RiskDeleted, rt.AggregateRisk},
		{"asset created", "/assets", "/assets", "asset", id, domain.AuditActionCreate, rt.AssetCreated, rt.AggregateAsset},
		{"asset updated", "/assets/:id", "/assets/" + id, "asset", id, domain.AuditActionUpdate, rt.AssetUpdated, rt.AggregateAsset},
		{"asset deleted", "/assets/:id", "/assets/" + id, "asset", id, domain.AuditActionDelete, rt.AssetDeleted, rt.AggregateAsset},
		{"vulnerability updated", "/vulnerabilities/:id", "/vulnerabilities/" + id, "vulnerability", id, domain.AuditActionUpdate, rt.VulnerabilityUpdated, rt.AggregateVulnerability},
		{"vulnerability deleted", "/vulnerabilities/:id", "/vulnerabilities/" + id, "vulnerability", id, domain.AuditActionDelete, rt.VulnerabilityDeleted, rt.AggregateVulnerability},
		{"incident created", "/incidents", "/incidents", "incident", id, domain.AuditActionCreate, rt.IncidentCreated, rt.AggregateIncident},
		{"incident updated", "/incidents/:id", "/incidents/" + id, "incident", id, domain.AuditActionUpdate, rt.IncidentUpdated, rt.AggregateIncident},
		{"incident deleted", "/incidents/:id", "/incidents/" + id, "incident", id, domain.AuditActionDelete, rt.IncidentDeleted, rt.AggregateIncident},
		// A creation under a sub-resource: the path names the framework, the
		// event names the control that was created.
		{"control created", "/compliance/frameworks/:fid/controls", "/compliance/frameworks/" + parent + "/controls", "compliance_control", id, domain.AuditActionCreate, rt.ControlCreated, rt.AggregateControl},
		{"control updated", "/compliance/controls/:id", "/compliance/controls/" + id, "compliance_control", id, domain.AuditActionUpdate, rt.ControlUpdated, rt.AggregateControl},
		{"control deleted", "/compliance/controls/:id", "/compliance/controls/" + id, "control", id, domain.AuditActionDelete, rt.ControlDeleted, rt.AggregateControl},
		{"assessment created", "/compliance/audits", "/compliance/audits", "audit", id, domain.AuditActionCreate, rt.AssessmentCreated, rt.AggregateComplianceAudit},
		{"assessment updated", "/compliance/audits/:id", "/compliance/audits/" + id, "audit", id, domain.AuditActionUpdate, rt.AssessmentUpdated, rt.AggregateComplianceAudit},
		{"assessment deleted", "/compliance/audits/:id", "/compliance/audits/" + id, "compliance_audit", id, domain.AuditActionDelete, rt.AssessmentDeleted, rt.AggregateComplianceAudit},
		{"mitigation created", "/risks/:id/mitigations", "/risks/" + parent + "/mitigations", "mitigation", id, domain.AuditActionCreate, rt.MitigationCreated, rt.AggregateMitigation},
		{"mitigation updated", "/mitigations/:id", "/mitigations/" + id, "mitigation", id, domain.AuditActionUpdate, rt.MitigationUpdated, rt.AggregateMitigation},
		{"mitigation deleted", "/mitigations/:id", "/mitigations/" + id, "mitigation", id, domain.AuditActionDelete, rt.MitigationDeleted, rt.AggregateMitigation},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sent := observe(t, tc.route, tc.path, auditEvent(tenant, tc.entity, tc.entityID, tc.action))
			if len(sent) != 1 {
				t.Fatalf("expected exactly one canonical event, got %d", len(sent))
			}
			got := sent[0]
			if got.Type != tc.want {
				t.Fatalf("got %s, want %s", got.Type, tc.want)
			}
			if got.Aggregate.Type != tc.wantAggr || got.Aggregate.ID != tc.entityID {
				t.Fatalf("aggregate %+v, want %s/%s", got.Aggregate, tc.wantAggr, tc.entityID)
			}
			if got.TenantID != tenant.String() {
				t.Fatalf("tenant %q", got.TenantID)
			}
		})
	}
}

// A lifecycle transition is a status change. Neither the HTTP verb nor the row
// layer knows that — Risk is deliberately not audited at row level — so the
// route's trailing segment is the only place the intent exists.
func TestMutationBridge_ATransitionIsAStatusChangeNotACreation(t *testing.T) {
	tenant := uuid.New()
	id := uuid.NewString()

	sent := observe(t, "/risks/:id/transition", "/risks/"+id+"/transition",
		auditEvent(tenant, "risk", id, domain.AuditActionCreate))
	if len(sent) != 1 || sent[0].Type != rt.RiskStatusChanged {
		t.Fatalf("expected risk.status_changed, got %+v", sent)
	}
	if sent[0].Payload["action"] != "transition" {
		t.Fatalf("the payload should name the action taken, got %v", sent[0].Payload)
	}
}

// A POST to a sub-resource updates the aggregate; publishing `created` would
// tell every consumer a mitigation appeared that has existed for weeks.
func TestMutationBridge_ASubResourcePostIsAnUpdateOfTheAggregate(t *testing.T) {
	tenant := uuid.New()
	id := uuid.NewString()

	sent := observe(t, "/mitigations/:id/sub-actions", "/mitigations/"+id+"/sub-actions",
		auditEvent(tenant, "mitigation", id, domain.AuditActionCreate))
	if len(sent) != 1 || sent[0].Type != rt.MitigationUpdated {
		t.Fatalf("expected mitigation.updated, got %+v", sent)
	}
}

// The event and the audit entry describing the same change must be tied
// together, or "why did this fire?" has no answer.
func TestMutationBridge_CorrelatesWithTheAuditEntry(t *testing.T) {
	tenant := uuid.New()
	id := uuid.NewString()
	ev := auditEvent(tenant, "risk", id, domain.AuditActionUpdate)

	sent := observe(t, "/risks/:id", "/risks/"+id, ev)
	if len(sent) != 1 {
		t.Fatal("expected one event")
	}
	if sent[0].CorrelationID != ev.RequestID {
		t.Fatalf("correlation id %q, want the request id %q", sent[0].CorrelationID, ev.RequestID)
	}
	if sent[0].CausationID != ev.ID.String() {
		t.Fatalf("causation id %q, want the audit entry id %q", sent[0].CausationID, ev.ID)
	}
	if sent[0].ActorID != ev.ActorID.String() {
		t.Fatalf("actor %q", sent[0].ActorID)
	}
	fields, ok := sent[0].Payload["changedFields"].([]string)
	if !ok || len(fields) != 1 || fields[0] != "status" {
		t.Fatalf("changed fields did not survive: %v", sent[0].Payload["changedFields"])
	}
}

// The bridge publishes references, never entities. A consumer that wants the
// object reads it back, which is also the only way it can be sure it holds the
// current state.
func TestMutationBridge_NeverCopiesTheBeforeAfterSnapshots(t *testing.T) {
	tenant := uuid.New()
	id := uuid.NewString()
	ev := auditEvent(tenant, "risk", id, domain.AuditActionUpdate)
	ev.Before = domain.JSONMap{"name": "old", "owner_email": "a@b.c"}
	ev.After = domain.JSONMap{"name": "new", "owner_email": "a@b.c"}

	sent := observe(t, "/risks/:id", "/risks/"+id, ev)
	for _, banned := range []string{"before", "after", "owner_email", "name"} {
		if _, ok := sent[0].Payload[banned]; ok {
			t.Fatalf("%q reached the event payload", banned)
		}
	}
}

// A mutation nobody mapped publishes nothing. The catalog is a closed set, and
// silence is the correct behaviour for an entity that has not been given an
// event on purpose.
func TestMutationBridge_UnmappedMutationsArePublishedAsNothing(t *testing.T) {
	tenant := uuid.New()
	id := uuid.NewString()

	for _, entity := range []string{"automation_rule", "approval_workflow", "delegation", "invitation", "unknown"} {
		sent := observe(t, "/things/:id", "/things/"+id, auditEvent(tenant, entity, id, domain.AuditActionUpdate))
		if len(sent) != 0 {
			t.Fatalf("entity %q published %s, but nothing was mapped for it", entity, sent[0].Type)
		}
	}
}

// Without an aggregate id the event names no subject, and a consumer could do
// nothing with it but refetch everything.
func TestMutationBridge_SkipsAMutationWithNoAggregateID(t *testing.T) {
	tenant := uuid.New()
	ev := auditEvent(tenant, "risk", "", domain.AuditActionCreate)
	if sent := observe(t, "/risks", "/risks", ev); len(sent) != 0 {
		t.Fatalf("expected nothing, got %+v", sent)
	}
}

func TestMutationBridge_NilInputsAreSafe(t *testing.T) {
	var nilBridge *MutationBridge
	nilBridge.ObserveMutation(nil, nil)

	bridge := NewMutationBridge(nil)
	bridge.ObserveMutation(nil, auditEvent(uuid.New(), "risk", uuid.NewString(), domain.AuditActionCreate))
}

// A create whose route has no path parameter and whose entity is not audited at
// row level leaves the trail without an aggregate id. Risk is exactly that
// case — deliberately excluded from the audit plugin — so without this fallback
// risk.created could never fire at all. Found by running the thing against a
// live database rather than by reading it.
func TestMutationBridge_RecoversACreatedIDFromTheResponse(t *testing.T) {
	tenant := uuid.New()
	created := uuid.NewString()

	pub := &capturePublisher{}
	bridge := NewMutationBridge(pub)

	app := fiber.New()
	app.Post("/risks", func(c *fiber.Ctx) error {
		// The handler answers with the record it just made, exactly as the real
		// one does; the observer runs afterwards, on the response path.
		if err := c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": created, "title": "a risk"}); err != nil {
			return err
		}
		bridge.ObserveMutation(c, auditEvent(tenant, "risk", "", domain.AuditActionCreate))
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/risks", nil))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if len(pub.sent) != 1 {
		t.Fatalf("expected risk.created to be published, got %d events", len(pub.sent))
	}
	if pub.sent[0].Type != rt.RiskCreated {
		t.Fatalf("got %s", pub.sent[0].Type)
	}
	if pub.sent[0].Aggregate.ID != created {
		t.Fatalf("aggregate id %q, want the id from the response %q", pub.sent[0].Aggregate.ID, created)
	}
}

// The wrapped shape this API also answers with.
func TestMutationBridge_RecoversACreatedIDFromAWrappedResponse(t *testing.T) {
	tenant := uuid.New()
	created := uuid.NewString()

	pub := &capturePublisher{}
	bridge := NewMutationBridge(pub)

	app := fiber.New()
	app.Post("/incidents", func(c *fiber.Ctx) error {
		if err := c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": fiber.Map{"id": created}}); err != nil {
			return err
		}
		bridge.ObserveMutation(c, auditEvent(tenant, "incident", "", domain.AuditActionCreate))
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/incidents", nil))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if len(pub.sent) != 1 || pub.sent[0].Aggregate.ID != created {
		t.Fatalf("unexpected result: %+v", pub.sent)
	}
}

// The fallback is for creations only. An update or a delete that reached the
// trail without an id is a defect somewhere else, and inventing a subject for
// it from whatever the response happened to contain would publish an event
// about the wrong record.
func TestMutationBridge_DoesNotGuessAnIDForUpdatesOrDeletes(t *testing.T) {
	tenant := uuid.New()
	other := uuid.NewString()

	for _, action := range []domain.AuditAction{domain.AuditActionUpdate, domain.AuditActionDelete} {
		pub := &capturePublisher{}
		bridge := NewMutationBridge(pub)

		app := fiber.New()
		app.Patch("/risks", func(c *fiber.Ctx) error {
			if err := c.JSON(fiber.Map{"id": other}); err != nil {
				return err
			}
			bridge.ObserveMutation(c, auditEvent(tenant, "risk", "", action))
			return nil
		})
		resp, err := app.Test(httptest.NewRequest(fiber.MethodPatch, "/risks", nil))
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()

		if len(pub.sent) != 0 {
			t.Fatalf("action %s published %+v from a response body it should not have read", action, pub.sent)
		}
	}
}

// A response that is not the record — an empty body, a non-JSON body, or one
// with no id — yields nothing rather than a malformed event.
func TestMutationBridge_UnreadableResponsesYieldNoEvent(t *testing.T) {
	tenant := uuid.New()

	for name, write := range map[string]func(*fiber.Ctx) error{
		"empty":    func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) },
		"not json": func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).SendString("created") },
		"no id":    func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "ok"}) },
		"empty id": func(c *fiber.Ctx) error { return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": ""}) },
	} {
		t.Run(name, func(t *testing.T) {
			pub := &capturePublisher{}
			bridge := NewMutationBridge(pub)
			app := fiber.New()
			app.Post("/risks", func(c *fiber.Ctx) error {
				if err := write(c); err != nil {
					return err
				}
				bridge.ObserveMutation(c, auditEvent(tenant, "risk", "", domain.AuditActionCreate))
				return nil
			})
			resp, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/risks", nil))
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if len(pub.sent) != 0 {
				t.Fatalf("published %+v from an unusable response", pub.sent)
			}
		})
	}
}
