// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package realtime

import (
	"context"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/opendefender/openrisk/internal/domain"
	rt "github.com/opendefender/openrisk/pkg/realtime"
)

// ---------------------------------------------------------------------------
// MutationBridge — canonical domain events from the mutation the audit trail
// already observed.
//
// WHY NOT A PUBLISH CALL PER USE CASE. There are several hundred mutating use
// cases. A publish call added to each is a call somebody eventually forgets,
// and the one they forget is the one that matters. The audit middleware already
// sees every successful POST/PUT/PATCH/DELETE, with the tenant, the actor, the
// resource type and id that actually changed (from the row layer, not from a
// guess at the URL), the changed field names and the request id.
//
// WHY THIS IS STILL A DOMAIN EVENT AND NOT A UI EVENT. What is published is
// decided by an explicit map from (entity, action) onto the catalog. A mutation
// with no entry produces nothing. So the bridge is exhaustive in its coverage
// of the entities it names, and silent about everything else — it can never
// invent `risk-card-refresh`, because the only names it can emit are the
// aggregate-and-action names the catalog defines.
//
// WHAT IT DELIBERATELY DOES NOT DO. It does not copy the before/after snapshots
// into the event. Consumers get the aggregate id and the list of changed field
// NAMES, and read the current state back from the API if they need it — which
// is also the only way they can be sure they are acting on the current state
// rather than on a snapshot that was already stale when it was serialised.
// ---------------------------------------------------------------------------

// EventPublisher is the write side the bridge needs.
type EventPublisher interface {
	Publish(ctx context.Context, env rt.Envelope) (rt.Envelope, error)
}

// MutationBridge turns audit observations into canonical events.
type MutationBridge struct {
	pub EventPublisher
}

// NewMutationBridge wires the bridge.
func NewMutationBridge(pub EventPublisher) *MutationBridge {
	return &MutationBridge{pub: pub}
}

// verbEvents maps (entity, trailing route verb) onto an event type, for the
// actions whose HTTP method understates what happened.
//
// `POST /risks/:id/transition` is a status change, not a creation; the method
// cannot know that and the row layer does not either, because Risk is
// deliberately not audited at the row level. The verb is the only place that
// knowledge exists, so it is read here.
var verbEvents = map[string]map[string]rt.EventType{
	"risk": {
		"transition": rt.RiskStatusChanged,
		"review":     rt.RiskUpdated,
	},
	"vulnerability": {
		"status": rt.VulnerabilityUpdated,
		"ticket": rt.VulnerabilityUpdated,
		"asset":  rt.VulnerabilityUpdated,
	},
}

// entityEvents maps an entity onto its create/update/delete events.
//
// Two names appear for the same aggregate where the row layer and the URL
// disagree: the compliance control is `compliance_control` to the audit plugin
// (its model's own declaration) and `control` to a route parser reading
// /compliance/controls/:id. Both are listed rather than normalised somewhere
// else, because a rename in either place should show up as a missing event in a
// test, not as a silently different one.
var entityEvents = map[string]map[domain.AuditAction]rt.EventType{
	"risk": {
		domain.AuditActionCreate: rt.RiskCreated,
		domain.AuditActionUpdate: rt.RiskUpdated,
		domain.AuditActionDelete: rt.RiskDeleted,
	},
	"asset": {
		domain.AuditActionCreate: rt.AssetCreated,
		domain.AuditActionUpdate: rt.AssetUpdated,
		domain.AuditActionDelete: rt.AssetDeleted,
	},
	"vulnerability": {
		domain.AuditActionUpdate: rt.VulnerabilityUpdated,
		domain.AuditActionDelete: rt.VulnerabilityDeleted,
	},
	"incident": {
		domain.AuditActionCreate: rt.IncidentCreated,
		domain.AuditActionUpdate: rt.IncidentUpdated,
		domain.AuditActionDelete: rt.IncidentDeleted,
	},
	"compliance_control": {
		domain.AuditActionCreate: rt.ControlCreated,
		domain.AuditActionUpdate: rt.ControlUpdated,
		domain.AuditActionDelete: rt.ControlDeleted,
	},
	"control": {
		domain.AuditActionCreate: rt.ControlCreated,
		domain.AuditActionUpdate: rt.ControlUpdated,
		domain.AuditActionDelete: rt.ControlDeleted,
	},
	// The product's "assessment" is the compliance audit; see the catalog note.
	"audit": {
		domain.AuditActionCreate: rt.AssessmentCreated,
		domain.AuditActionUpdate: rt.AssessmentUpdated,
		domain.AuditActionDelete: rt.AssessmentDeleted,
	},
	"compliance_audit": {
		domain.AuditActionCreate: rt.AssessmentCreated,
		domain.AuditActionUpdate: rt.AssessmentUpdated,
		domain.AuditActionDelete: rt.AssessmentDeleted,
	},
	"mitigation": {
		domain.AuditActionCreate: rt.MitigationCreated,
		domain.AuditActionUpdate: rt.MitigationUpdated,
		domain.AuditActionDelete: rt.MitigationDeleted,
	},
}

// ObserveMutation publishes the canonical event for one journaled mutation.
func (b *MutationBridge) ObserveMutation(c *fiber.Ctx, ev *domain.AuditEvent) {
	if b == nil || b.pub == nil || ev == nil {
		return
	}
	env, ok := b.envelopeFor(c, ev)
	if !ok {
		return
	}
	// The request context is already on its way out; using it would cancel the
	// publish mid-flight on a client that disconnected right after its write
	// succeeded, and lose an event for a change that definitely happened.
	if _, err := b.pub.Publish(context.WithoutCancel(c.UserContext()), env); err != nil {
		// Best-effort by design: the business call and the audit entry both
		// succeeded, and failing the response now would report a failure that
		// did not occur. It is logged because a stream that silently stops is
		// the hardest realtime fault to diagnose.
		log.Printf("realtime: could not publish %s for %s %s: %v",
			env.Type, ev.EntityType, ev.EntityID, err)
	}
}

// envelopeFor decides which event, if any, a journaled mutation produces.
func (b *MutationBridge) envelopeFor(c *fiber.Ctx, ev *domain.AuditEvent) (rt.Envelope, bool) {
	if ev.TenantID.String() == "" || ev.EntityID == "" {
		// Without an aggregate id the event names no subject, and a consumer
		// could do nothing with it but refetch everything.
		return rt.Envelope{}, false
	}
	entity := strings.ToLower(strings.TrimSpace(ev.EntityType))

	verb := routeVerb(c)
	var typ rt.EventType
	if byVerb, ok := verbEvents[entity]; ok {
		if t, ok := byVerb[verb]; ok {
			typ = t
		}
	}
	if typ == "" {
		byAction, ok := entityEvents[entity]
		if !ok {
			// An entity nobody mapped is silent. That is the design: the
			// catalog is a closed set, and adding an entity is a deliberate act
			// with a test behind it.
			return rt.Envelope{}, false
		}
		t, ok := byAction[ev.Action]
		if !ok {
			return rt.Envelope{}, false
		}
		// A POST under a sub-resource is ambiguous, and the two readings are
		// both real:
		//
		//   POST /risks/:id/mitigations      creates a mitigation
		//   POST /mitigations/:id/sub-actions  updates the mitigation :id
		//
		// What separates them is whose id was reported. If the aggregate's id is
		// one of the path parameters, the aggregate already existed and this
		// call changed it; publishing `created` would tell every consumer that a
		// mitigation appeared which has existed for weeks. If it is a new id the
		// row layer just produced, it is genuinely a creation.
		if ev.Action == domain.AuditActionCreate && idIsAPathParameter(c, ev.EntityID) {
			if upd, ok := byAction[domain.AuditActionUpdate]; ok {
				t = upd
			}
		}
		typ = t
	}

	desc, ok := rt.Lookup(typ)
	if !ok {
		return rt.Envelope{}, false
	}

	actor := ""
	if ev.ActorID != nil {
		actor = ev.ActorID.String()
	}
	payload := map[string]any{}
	if len(ev.ChangedFields) > 0 {
		payload["changedFields"] = []string(ev.ChangedFields)
	}
	if verb != "" {
		payload["action"] = verb
	}

	return rt.Envelope{
		Type:      typ,
		TenantID:  ev.TenantID.String(),
		ActorID:   actor,
		Aggregate: rt.Aggregate{Type: desc.Aggregate, ID: ev.EntityID},
		// The audit entry and the event that describes the same change share a
		// correlation id, and the event names the entry as its cause. That is
		// what lets "why did this event fire?" be answered by reading the trail,
		// and it is why the realtime stream does not have to become a second,
		// competing record of what happened.
		CorrelationID: ev.RequestID,
		CausationID:   ev.ID.String(),
		Payload:       payload,
	}, true
}

// routeVerb recovers the trailing action segment of a route
// (`/risks/:id/transition` → "transition"), which is where the intent lives for
// the routes whose HTTP method understates it.
func routeVerb(c *fiber.Ctx) string {
	if c == nil {
		return ""
	}
	route := c.Route()
	if route == nil || route.Path == "" {
		return ""
	}
	segs := strings.Split(strings.Trim(route.Path, "/"), "/")
	// A trailing static segment that follows at least one parameter is the verb.
	seenParam := false
	verb := ""
	for _, s := range segs {
		if s == "" {
			continue
		}
		if strings.HasPrefix(s, ":") || strings.HasPrefix(s, "+") || strings.HasPrefix(s, "*") {
			seenParam = true
			verb = ""
			continue
		}
		if seenParam {
			verb = s
		}
	}
	return verb
}

// idIsAPathParameter reports whether the reported aggregate id appears as a
// parameter of the matched route — which means the aggregate was addressed, not
// created, by this call.
func idIsAPathParameter(c *fiber.Ctx, id string) bool {
	if c == nil || id == "" {
		return false
	}
	route := c.Route()
	if route == nil {
		return false
	}
	for _, name := range route.Params {
		if c.Params(name) == id {
			return true
		}
	}
	return false
}
