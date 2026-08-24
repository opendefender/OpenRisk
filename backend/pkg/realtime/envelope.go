// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package realtime holds the canonical event contract: the envelope every
// OpenRisk domain event is wrapped in, the catalog of event types that may be
// published, and the pure rules that decide whether an envelope is well formed,
// what a subscriber is allowed to ask for, and what may never appear in a
// payload.
//
// Everything here is stdlib-only and free of any dependency on the database,
// Fiber, Redis or `internal/`. That is deliberate: the contract is the part of
// the system that consumers hold on to, so it has to be testable without
// standing anything up, and it must not drift with the transport that happens to
// carry it today.
package realtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// EnvelopeVersion is the version of the ENVELOPE itself (the outer contract),
// distinct from the per-event-type payload version carried in Envelope.Version.
//
// A consumer reads this first: it says how to find the other fields. It changes
// only when the envelope's own shape changes, which is expected to be roughly
// never.
const EnvelopeVersion = 1

// MaxPayloadBytes caps a single event's JSON payload.
//
// Events carry references and changed-field names, not entities (see
// SanitizePayload and the payload-minimisation rule in the contract doc), so 16
// KiB is generous for anything legitimate. The cap exists because an unbounded
// payload is an unbounded write into every connected client's buffer: one
// oversized event would evict every other tenant's buffered events at once.
const MaxPayloadBytes = 16 * 1024

// Errors returned by Validate. They are sentinels so callers can branch — the
// publisher counts them by kind, and the contract test asserts each one fires.
var (
	ErrNoID              = errors.New("realtime: event id is required")
	ErrNoType            = errors.New("realtime: event type is required")
	ErrUnknownType       = errors.New("realtime: event type is not in the catalog")
	ErrNoTenant          = errors.New("realtime: tenant id is required")
	ErrNoOccurredAt      = errors.New("realtime: occurredAt is required")
	ErrNoAggregateType   = errors.New("realtime: aggregate type is required")
	ErrAggregateMismatch = errors.New("realtime: aggregate type does not match the catalog entry for this event type")
	ErrVersionMismatch   = errors.New("realtime: event version is not published for this type")
	ErrPayloadTooLarge   = fmt.Errorf("realtime: payload exceeds %d bytes", MaxPayloadBytes)
	ErrForbiddenField    = errors.New("realtime: payload carries a forbidden field")
)

// Aggregate names the domain object an event is about.
//
// Type is the aggregate's stable name ("risk", "asset"), NOT a table name and
// not a URL segment: the URL may change, and a consumer keying off it would
// break silently.
type Aggregate struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// Envelope is the canonical wrapper around every published domain event.
//
// Field-by-field, and why each is here:
//
//	ID             — unique, server-minted. The deduplication key. A client that
//	                 sees the same ID twice has seen the same event twice, full
//	                 stop, whatever route it arrived by.
//	EnvelopeVersion— the outer contract version; how to read the rest.
//	Type           — "<aggregate>.<action>", drawn from the catalog. A type not
//	                 in the catalog cannot be published.
//	Version        — the PAYLOAD schema version for this type. Incremented when
//	                 the payload shape changes incompatibly.
//	OccurredAt     — when the business change happened, UTC. Not when it was
//	                 delivered: delivery time is the client's own business.
//	TenantID       — the isolation boundary. Never taken from client input.
//	ActorID        — who caused it; empty for system-originated events.
//	Aggregate      — what it is about.
//	Sequence       — per-tenant monotonic position, assigned by the durable log.
//	                 Zero until the log assigns it; that is how "not yet
//	                 persisted" is expressed without a second field.
//	CorrelationID  — ties every artefact of one request together (the audit entry
//	                 for the same mutation carries the same value).
//	CausationID    — what directly caused this event: the audit entry's id for a
//	                 mutation, or the upstream event id for a derived event.
//	Payload        — minimised, see SanitizePayload.
type Envelope struct {
	ID              string         `json:"id"`
	EnvelopeVersion int            `json:"envelopeVersion"`
	Type            EventType      `json:"type"`
	Version         int            `json:"version"`
	OccurredAt      time.Time      `json:"occurredAt"`
	TenantID        string         `json:"tenantId"`
	ActorID         string         `json:"actorId,omitempty"`
	Aggregate       Aggregate      `json:"aggregate"`
	Sequence        int64          `json:"sequence,omitempty"`
	CorrelationID   string         `json:"correlationId,omitempty"`
	CausationID     string         `json:"causationId,omitempty"`
	Payload         map[string]any `json:"payload,omitempty"`
}

// Validate reports whether an envelope may be published.
//
// It is intentionally strict about the catalog: an unregistered type, or a
// version the catalog does not publish, is refused rather than passed through.
// The alternative — publish whatever a caller invents — turns the catalog into
// documentation that lies, and consumers written against it break at the worst
// possible time.
func (e Envelope) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return ErrNoID
	}
	if e.Type == "" {
		return ErrNoType
	}
	desc, ok := Lookup(e.Type)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownType, e.Type)
	}
	if e.TenantID == "" {
		return ErrNoTenant
	}
	if e.OccurredAt.IsZero() {
		return ErrNoOccurredAt
	}
	if e.Aggregate.Type == "" {
		return ErrNoAggregateType
	}
	if e.Aggregate.Type != desc.Aggregate {
		return fmt.Errorf("%w: %q declares aggregate %q, envelope carries %q",
			ErrAggregateMismatch, e.Type, desc.Aggregate, e.Aggregate.Type)
	}
	if e.Version != desc.Version {
		return fmt.Errorf("%w: %q is published at version %d, envelope carries %d",
			ErrVersionMismatch, e.Type, desc.Version, e.Version)
	}
	if err := validatePayload(e.Payload); err != nil {
		return err
	}
	return nil
}

func validatePayload(payload map[string]any) error {
	if len(payload) == 0 {
		return nil
	}
	for k := range payload {
		if IsForbiddenField(k) {
			return fmt.Errorf("%w: %q", ErrForbiddenField, k)
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("realtime: payload is not serialisable: %w", err)
	}
	if len(raw) > MaxPayloadBytes {
		return ErrPayloadTooLarge
	}
	return nil
}

// forbiddenFragments are substrings that must never name a payload field.
//
// This is a backstop, not the primary defence: the primary defence is that
// events carry references and changed-field NAMES rather than entities. But
// "primary defence" means "until someone adds a field in a hurry", and an event
// stream is exactly the wrong place to discover that a password hash rides
// along. Matching is on the lower-cased field name containing the fragment, so
// `passwordHash`, `mfa_secret` and `apiToken` are all caught.
var forbiddenFragments = []string{
	"password", "passwd", "secret", "token", "credential", "apikey", "api_key",
	"private_key", "privatekey", "mfa", "otp", "backup_code", "backupcode",
	"authorization", "session_id", "sessionid", "cookie", "salt", "hash",
	"ssn", "card_number", "cardnumber", "cvv", "iban",
}

// IsForbiddenField reports whether a payload field name is one an event may
// never carry.
func IsForbiddenField(name string) bool {
	l := strings.ToLower(strings.TrimSpace(name))
	if l == "" {
		return false
	}
	for _, frag := range forbiddenFragments {
		if strings.Contains(l, frag) {
			return true
		}
	}
	return false
}

// SanitizePayload drops forbidden fields and anything that is not a scalar,
// returning the payload an event may actually carry.
//
// The scalar rule is the payload-minimisation rule made mechanical: a nested
// object is how a whole entity ends up on the wire by accident. Consumers that
// need the entity read it back from the API — which is also the only way they
// can be sure they are reading the CURRENT state rather than a snapshot that was
// already stale when it was serialised.
//
// String slices survive because "which fields changed" is a list of names, and
// that list is the single most useful thing an event can carry.
func SanitizePayload(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if IsForbiddenField(k) {
			continue
		}
		switch val := v.(type) {
		case nil:
			continue
		case string, bool,
			int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64,
			float32, float64:
			out[k] = val
		case []string:
			out[k] = val
		default:
			// Anything structured is dropped rather than truncated: a partially
			// serialised object is worse than an absent one, because it looks
			// like data.
			continue
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
