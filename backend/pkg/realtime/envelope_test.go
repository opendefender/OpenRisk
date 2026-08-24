// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package realtime

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func validEnvelope() Envelope {
	return Envelope{
		ID:              "01J000000000000000000000",
		EnvelopeVersion: EnvelopeVersion,
		Type:            RiskCreated,
		Version:         1,
		OccurredAt:      time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC),
		TenantID:        "11111111-1111-1111-1111-111111111111",
		ActorID:         "22222222-2222-2222-2222-222222222222",
		Aggregate:       Aggregate{Type: AggregateRisk, ID: "33333333-3333-3333-3333-333333333333"},
		Sequence:        7,
		CorrelationID:   "req-1",
		CausationID:     "audit-1",
		Payload:         map[string]any{"changedFields": []string{"name"}},
	}
}

func TestValidate_AcceptsAWellFormedEnvelope(t *testing.T) {
	if err := validEnvelope().Validate(); err != nil {
		t.Fatalf("expected a valid envelope, got %v", err)
	}
}

func TestValidate_RefusesEachMissingRequiredField(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Envelope)
		want error
	}{
		{"no id", func(e *Envelope) { e.ID = "" }, ErrNoID},
		{"blank id", func(e *Envelope) { e.ID = "   " }, ErrNoID},
		{"no type", func(e *Envelope) { e.Type = "" }, ErrNoType},
		{"no tenant", func(e *Envelope) { e.TenantID = "" }, ErrNoTenant},
		{"no occurredAt", func(e *Envelope) { e.OccurredAt = time.Time{} }, ErrNoOccurredAt},
		{"no aggregate type", func(e *Envelope) { e.Aggregate.Type = "" }, ErrNoAggregateType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := validEnvelope()
			tc.mut(&e)
			err := e.Validate()
			if !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}

// An unregistered type must be refused at publication. The catalog is the
// contract consumers hold; a type that can be published without appearing in it
// makes the catalog a lie.
func TestValidate_RefusesATypeOutsideTheCatalog(t *testing.T) {
	e := validEnvelope()
	e.Type = "risk.invented"
	if err := e.Validate(); !errors.Is(err, ErrUnknownType) {
		t.Fatalf("want ErrUnknownType, got %v", err)
	}
}

func TestValidate_RefusesAnAggregateContradictingTheCatalog(t *testing.T) {
	e := validEnvelope()
	e.Aggregate.Type = AggregateAsset // risk.created is a risk event
	if err := e.Validate(); !errors.Is(err, ErrAggregateMismatch) {
		t.Fatalf("want ErrAggregateMismatch, got %v", err)
	}
}

// Version skew is a contract break, not a warning: a consumer written for v1
// must never silently receive a v2 payload under the same type.
func TestValidate_RefusesAVersionTheCatalogDoesNotPublish(t *testing.T) {
	e := validEnvelope()
	e.Version = 2
	if err := e.Validate(); !errors.Is(err, ErrVersionMismatch) {
		t.Fatalf("want ErrVersionMismatch, got %v", err)
	}
}

func TestValidate_RefusesAForbiddenPayloadField(t *testing.T) {
	for _, field := range []string{"password", "passwordHash", "mfa_secret", "apiToken", "session_id"} {
		e := validEnvelope()
		e.Payload = map[string]any{field: "x"}
		if err := e.Validate(); !errors.Is(err, ErrForbiddenField) {
			t.Fatalf("field %q: want ErrForbiddenField, got %v", field, err)
		}
	}
}

func TestValidate_RefusesAnOversizedPayload(t *testing.T) {
	e := validEnvelope()
	e.Payload = map[string]any{"note": strings.Repeat("x", MaxPayloadBytes+1)}
	if err := e.Validate(); !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("want ErrPayloadTooLarge, got %v", err)
	}
}

func TestSanitizePayload_DropsSecretsAndNestedStructures(t *testing.T) {
	got := SanitizePayload(map[string]any{
		"riskId":        "abc",
		"score":         13.5,
		"kev":           true,
		"changedFields": []string{"status", "severity"},
		"passwordHash":  "$argon2id$...",
		"apiToken":      "tok_live_x",
		"entity":        map[string]any{"email": "a@b.c", "mfaSecret": "JBSWY3DP"},
		"nilField":      nil,
	})

	for _, banned := range []string{"passwordHash", "apiToken", "entity", "nilField"} {
		if _, ok := got[banned]; ok {
			t.Fatalf("%q survived sanitisation", banned)
		}
	}
	for _, kept := range []string{"riskId", "score", "kev", "changedFields"} {
		if _, ok := got[kept]; !ok {
			t.Fatalf("%q was dropped but is a legitimate field", kept)
		}
	}

	// The whole point: whatever survived must be publishable.
	e := validEnvelope()
	e.Payload = got
	if err := e.Validate(); err != nil {
		t.Fatalf("sanitised payload must validate, got %v", err)
	}
}

func TestSanitizePayload_EmptyInAndEmptyOutAreNil(t *testing.T) {
	if got := SanitizePayload(nil); got != nil {
		t.Fatalf("nil in must give nil out, got %v", got)
	}
	if got := SanitizePayload(map[string]any{"secret": "x"}); got != nil {
		t.Fatalf("a payload that is entirely forbidden must give nil, got %v", got)
	}
}

// The wire shape is the contract. This test fails the moment a field is renamed
// or removed, which is exactly when a consumer would break.
func TestEnvelope_WireShapeIsStable(t *testing.T) {
	raw, err := json.Marshal(validEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"id", "envelopeVersion", "type", "version", "occurredAt", "tenantId",
		"actorId", "aggregate", "sequence", "correlationId", "causationId", "payload",
	} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("envelope lost its %q field — that is a breaking contract change", key)
		}
	}
	agg, ok := decoded["aggregate"].(map[string]any)
	if !ok {
		t.Fatal("aggregate must serialise as an object with type and id")
	}
	if agg["type"] != AggregateRisk || agg["id"] == "" {
		t.Fatalf("aggregate lost its shape: %v", agg)
	}
}

// A v1 payload decoded by a consumer that already knows about later fields must
// still work: unknown-to-the-payload fields are simply absent, never an error.
func TestEnvelope_ForwardCompatibleDecoding(t *testing.T) {
	raw, err := json.Marshal(validEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	// A newer consumer's view of the same event, with a field v1 never sent.
	var newer struct {
		Envelope
		FutureField string `json:"futureField"`
	}
	if err := json.Unmarshal(raw, &newer); err != nil {
		t.Fatalf("a newer consumer must decode a v1 event, got %v", err)
	}
	if newer.Type != RiskCreated || newer.FutureField != "" {
		t.Fatalf("unexpected decode: %+v", newer)
	}
}
