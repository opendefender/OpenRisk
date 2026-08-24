// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package realtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/opendefender/openrisk/pkg/events"
	rt "github.com/opendefender/openrisk/pkg/realtime"
)

// capturePublisher records what the relay tried to publish.
type capturePublisher struct{ sent []rt.Envelope }

func (p *capturePublisher) Publish(_ context.Context, env rt.Envelope) (rt.Envelope, error) {
	p.sent = append(p.sent, env)
	return env, nil
}

func msgOf(t *testing.T, channel string, payload any) *redis.Message {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return &redis.Message{Channel: channel, Payload: string(raw)}
}

func TestDomainRelay_TranslatesEachInternalChannel(t *testing.T) {
	tenant := uuid.New()
	risk, asset, vuln, plan := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	cases := []struct {
		name    string
		msg     *redis.Message
		want    rt.EventType
		aggID   string
		payload map[string]any
	}{
		{
			name: "risk score recomputed by the worker",
			msg: msgOf(t, events.RiskScoreUpdated, events.RiskScoreUpdatedEvent{
				RiskID: risk.String(), TenantID: tenant.String(),
				NewScore: 13.5, OldScore: 8, Delta: 5.5, Criticality: "critical",
				CalculatedAt: "2026-08-24T10:00:00Z",
			}),
			want:  rt.RiskScoreChanged,
			aggID: risk.String(),
			payload: map[string]any{
				"newScore": 13.5, "oldScore": float64(8), "delta": 5.5, "criticality": "critical",
			},
		},
		{
			name: "vulnerability arriving through an unauthenticated ingest path",
			msg: msgOf(t, events.VulnerabilityDetected, events.VulnerabilityDetectedEvent{
				VulnerabilityID: vuln.String(), TenantID: tenant.String(),
				CVEID: "CVE-2021-44228", Severity: "critical", CVSS: 10, KEV: true,
				PriorityTier: "P1", AssetID: asset.String(), Source: "nessus",
				TriggeredBy: "system",
			}),
			want:  rt.VulnerabilityDetected,
			aggID: vuln.String(),
			payload: map[string]any{
				"cveId": "CVE-2021-44228", "severity": "critical", "cvss": float64(10),
				"kev": true, "priorityTier": "P1", "assetId": asset.String(), "source": "nessus",
			},
		},
		{
			name: "asset criticality change",
			msg: msgOf(t, events.AssetCriticalityChanged, events.AssetCriticalityChangedEvent{
				AssetID: asset.String(), TenantID: tenant.String(),
				OldCriticality: "MEDIUM", NewCriticality: "CRITICAL",
				ChangedBy: uuid.New().String(), ChangedAt: "2026-08-24T10:00:00Z",
			}),
			want:    rt.AssetCriticalityChanged,
			aggID:   asset.String(),
			payload: map[string]any{"oldCriticality": "MEDIUM", "newCriticality": "CRITICAL"},
		},
		{
			name: "scanner auto-completing a sub-action",
			msg: msgOf(t, mitigationAutoCompletedChannel, map[string]any{
				"tenant_id": tenant.String(), "plan_id": plan.String(),
				"sub_action_id": "sub-1", "scanner_job_id": "job-1",
				"evidence": "https://internal/scan/job-1",
			}),
			want:  rt.MitigationAutoCompleted,
			aggID: plan.String(),
			payload: map[string]any{
				"planId": plan.String(), "subActionId": "sub-1", "scannerJobId": "job-1",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pub := &capturePublisher{}
			relay := NewDomainEventRelay(nil, pub)
			relay.handle(context.Background(), tc.msg)

			if len(pub.sent) != 1 {
				t.Fatalf("expected one canonical event, got %d", len(pub.sent))
			}
			got := pub.sent[0]
			if got.Type != tc.want {
				t.Fatalf("got %s, want %s", got.Type, tc.want)
			}
			if got.TenantID != tenant.String() {
				t.Fatalf("tenant lost in translation: %q", got.TenantID)
			}
			if got.Aggregate.ID != tc.aggID {
				t.Fatalf("aggregate id %q, want %q", got.Aggregate.ID, tc.aggID)
			}
			for k, want := range tc.payload {
				if got.Payload[k] != want {
					t.Errorf("payload[%q] = %v, want %v", k, got.Payload[k], want)
				}
			}

			// Only what the catalog declares may travel.
			desc, ok := rt.Lookup(got.Type)
			if !ok {
				t.Fatalf("%s is not in the catalog", got.Type)
			}
			declared := map[string]bool{}
			for _, f := range desc.PayloadFields {
				declared[f] = true
			}
			for k := range got.Payload {
				if !declared[k] {
					t.Errorf("payload carries %q, which %s does not declare — that is an undocumented contract change", k, got.Type)
				}
			}
		})
	}
}

// The internal auto-completion payload carries an `evidence` field that may hold
// an internal URL. The relay reads only the declared fields, so it never
// forwards it.
func TestDomainRelay_DoesNotForwardUndeclaredInternalFields(t *testing.T) {
	pub := &capturePublisher{}
	relay := NewDomainEventRelay(nil, pub)
	tenant, plan := uuid.New(), uuid.New()

	relay.handle(context.Background(), msgOf(t, mitigationAutoCompletedChannel, map[string]any{
		"tenant_id": tenant.String(), "plan_id": plan.String(),
		"sub_action_id": "sub-1", "scanner_job_id": "job-1",
		"evidence":     "https://internal-scanner.local/raw?token=abc",
		"internalNote": "do not publish",
	}))

	if len(pub.sent) != 1 {
		t.Fatalf("expected one event, got %d", len(pub.sent))
	}
	for _, banned := range []string{"evidence", "internalNote"} {
		if _, ok := pub.sent[0].Payload[banned]; ok {
			t.Fatalf("%q was forwarded onto a public contract", banned)
		}
	}
}

// "system" is not a user id, and putting it in actorId would give consumers a
// value no lookup can resolve.
func TestDomainRelay_SystemIsNotAnActor(t *testing.T) {
	pub := &capturePublisher{}
	relay := NewDomainEventRelay(nil, pub)
	tenant, vuln := uuid.New(), uuid.New()

	relay.handle(context.Background(), msgOf(t, events.VulnerabilityDetected, events.VulnerabilityDetectedEvent{
		VulnerabilityID: vuln.String(), TenantID: tenant.String(), TriggeredBy: "system",
	}))
	if pub.sent[0].ActorID != "" {
		t.Fatalf("actorId = %q, want empty", pub.sent[0].ActorID)
	}

	human := uuid.New().String()
	relay.handle(context.Background(), msgOf(t, events.VulnerabilityDetected, events.VulnerabilityDetectedEvent{
		VulnerabilityID: vuln.String(), TenantID: tenant.String(), TriggeredBy: human,
	}))
	if pub.sent[1].ActorID != human {
		t.Fatalf("a real actor was dropped: %q", pub.sent[1].ActorID)
	}
}

func TestDomainRelay_IgnoresMalformedAndUnknownMessages(t *testing.T) {
	pub := &capturePublisher{}
	relay := NewDomainEventRelay(nil, pub)

	relay.handle(context.Background(), nil)
	relay.handle(context.Background(), &redis.Message{Channel: events.RiskScoreUpdated, Payload: "{broken"})
	relay.handle(context.Background(), &redis.Message{Channel: "some.other.channel", Payload: "{}"})
	// A payload with no tenant cannot be routed to anyone.
	relay.handle(context.Background(), msgOf(t, events.RiskScoreUpdated, events.RiskScoreUpdatedEvent{RiskID: "r1"}))
	// A payload with a tenant but no aggregate names no subject.
	relay.handle(context.Background(), msgOf(t, events.RiskScoreUpdated, events.RiskScoreUpdatedEvent{TenantID: uuid.NewString()}))

	if len(pub.sent) != 0 {
		t.Fatalf("expected nothing to be published, got %d events", len(pub.sent))
	}
}
