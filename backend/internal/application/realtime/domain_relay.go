// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package realtime

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/opendefender/openrisk/pkg/events"
	rt "github.com/opendefender/openrisk/pkg/realtime"
)

// ---------------------------------------------------------------------------
// DomainEventRelay — canonical events for the changes that never pass through
// an authenticated HTTP mutation.
//
// Three of the events W0-07 has to publish are produced by background work, not
// by a user pressing a button:
//
//   - a vulnerability arrives through a scanner webhook or a live pull, on a
//     path with no session at all;
//   - a risk score is recomputed by the score worker, minutes after the change
//     that triggered it;
//   - the scanner stops detecting a finding and auto-completes a sub-action.
//
// Each already publishes on an internal Redis channel that predates this wave.
// This relay subscribes to those channels and republishes them through the
// canonical publisher, so they land in the durable log with a sequence and reach
// clients as the same envelope as everything else. The internal channels are
// left exactly as they are: their existing consumers (the score worker, the
// automation engine) keep working, and nothing about their contract changes.
// ---------------------------------------------------------------------------

// mitigationAutoCompletedChannel is the scanner's auto-completion channel. It is
// a literal here because it is a literal at both existing ends
// (infrastructure/redis and infrastructure/scanmitigation) and inventing a
// constant on only one side would not make them agree.
const mitigationAutoCompletedChannel = "mitigation.auto_completed"

// Subscriber is the read side of the internal channels.
type Subscriber interface {
	Subscribe(ctx context.Context, channels ...string) *redis.PubSub
}

// DomainEventRelay republishes internal domain events as canonical ones.
type DomainEventRelay struct {
	sub Subscriber
	pub EventPublisher
}

// NewDomainEventRelay wires the relay.
func NewDomainEventRelay(sub Subscriber, pub EventPublisher) *DomainEventRelay {
	return &DomainEventRelay{sub: sub, pub: pub}
}

// Start consumes the internal channels until the context is cancelled.
// Blocking; run it in a goroutine.
func (r *DomainEventRelay) Start(ctx context.Context) {
	if r == nil || r.sub == nil || r.pub == nil {
		return
	}
	channels := []string{
		events.RiskScoreUpdated,
		events.VulnerabilityDetected,
		events.AssetCriticalityChanged,
		mitigationAutoCompletedChannel,
	}
	pubsub := r.sub.Subscribe(ctx, channels...)
	defer func() { _ = pubsub.Close() }()

	log.Printf("Realtime: domain relay subscribed to %v", channels)
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			r.handle(ctx, msg)
		}
	}
}

func (r *DomainEventRelay) handle(ctx context.Context, msg *redis.Message) {
	if msg == nil {
		return
	}
	env, ok := r.translate(msg.Channel, msg.Payload)
	if !ok {
		return
	}
	if _, err := r.pub.Publish(ctx, env); err != nil {
		log.Printf("realtime: domain relay could not publish %s from %s: %v", env.Type, msg.Channel, err)
	}
}

// translate converts one internal payload into a canonical envelope.
//
// Each case reads only the fields the catalog says the event may carry. That is
// deliberate rather than convenient: forwarding the whole internal payload would
// make every future field added to an internal struct silently part of a public
// contract, and would eventually put something on the wire that should not be
// there.
func (r *DomainEventRelay) translate(channel, payload string) (rt.Envelope, bool) {
	switch channel {
	case events.RiskScoreUpdated:
		var e events.RiskScoreUpdatedEvent
		if err := json.Unmarshal([]byte(payload), &e); err != nil || e.TenantID == "" || e.RiskID == "" {
			return rt.Envelope{}, false
		}
		occurred, _ := time.Parse(time.RFC3339, e.CalculatedAt)
		return rt.Envelope{
			Type:       rt.RiskScoreChanged,
			TenantID:   e.TenantID,
			OccurredAt: occurred,
			Aggregate:  rt.Aggregate{Type: rt.AggregateRisk, ID: e.RiskID},
			Payload: map[string]any{
				"newScore":    e.NewScore,
				"oldScore":    e.OldScore,
				"delta":       e.Delta,
				"criticality": e.Criticality,
			},
		}, true

	case events.VulnerabilityDetected:
		var e events.VulnerabilityDetectedEvent
		if err := json.Unmarshal([]byte(payload), &e); err != nil || e.TenantID == "" || e.VulnerabilityID == "" {
			return rt.Envelope{}, false
		}
		env := rt.Envelope{
			Type:      rt.VulnerabilityDetected,
			TenantID:  e.TenantID,
			Aggregate: rt.Aggregate{Type: rt.AggregateVulnerability, ID: e.VulnerabilityID},
			Payload: map[string]any{
				"cveId":        e.CVEID,
				"severity":     e.Severity,
				"cvss":         e.CVSS,
				"kev":          e.KEV,
				"priorityTier": e.PriorityTier,
				"assetId":      e.AssetID,
				"source":       e.Source,
			},
		}
		// "system" is not a user id; carrying it as one would put a value in
		// actorId that no lookup can resolve.
		if e.TriggeredBy != "" && e.TriggeredBy != "system" {
			env.ActorID = e.TriggeredBy
		}
		return env, true

	case events.AssetCriticalityChanged:
		var e events.AssetCriticalityChangedEvent
		if err := json.Unmarshal([]byte(payload), &e); err != nil || e.TenantID == "" || e.AssetID == "" {
			return rt.Envelope{}, false
		}
		occurred, _ := time.Parse(time.RFC3339, e.ChangedAt)
		return rt.Envelope{
			Type:       rt.AssetCriticalityChanged,
			TenantID:   e.TenantID,
			ActorID:    e.ChangedBy,
			OccurredAt: occurred,
			Aggregate:  rt.Aggregate{Type: rt.AggregateAsset, ID: e.AssetID},
			Payload: map[string]any{
				"oldCriticality": e.OldCriticality,
				"newCriticality": e.NewCriticality,
			},
		}, true

	case mitigationAutoCompletedChannel:
		var e struct {
			TenantID     string `json:"tenant_id"`
			PlanID       string `json:"plan_id"`
			SubActionID  string `json:"sub_action_id"`
			ScannerJobID string `json:"scanner_job_id"`
		}
		if err := json.Unmarshal([]byte(payload), &e); err != nil || e.TenantID == "" || e.PlanID == "" {
			return rt.Envelope{}, false
		}
		return rt.Envelope{
			Type:      rt.MitigationAutoCompleted,
			TenantID:  e.TenantID,
			Aggregate: rt.Aggregate{Type: rt.AggregateMitigation, ID: e.PlanID},
			Payload: map[string]any{
				"planId":       e.PlanID,
				"subActionId":  e.SubActionID,
				"scannerJobId": e.ScannerJobID,
			},
		}, true
	}
	return rt.Envelope{}, false
}
