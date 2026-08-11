// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Tamper-evident audit trail — hash chaining.
//
// Every AuditEvent carries the hash of the previous event of the SAME tenant.
// Rewriting, deleting or reordering any entry breaks the chain from that point
// on, and VerifyAuditChain reports exactly where. Append-only storage keeps
// honest writers honest; the chain makes a dishonest one detectable.
//
// Genesis: the first event of a tenant links to GenesisHash ("").
// Retention pruning does not silently cut the chain — it writes an
// AuditChainSeal recording the hash the surviving head must link back to.
// ---------------------------------------------------------------------------

// GenesisHash is the PrevHash of the very first event of a tenant.
const GenesisHash = ""

// AuditSource says which writer produced an entry.
const (
	// AuditSourceHTTP — the request middleware (one entry per mutating API call).
	AuditSourceHTTP = "http"
	// AuditSourceGorm — the GORM plugin, for mutations outside an HTTP request
	// (background workers, schedulers, migrations).
	AuditSourceGorm = "gorm"
	// AuditSourceExplicit — an application-layer Recorder call for a high-value
	// domain action that has no single row mutation (approve, export, delegate).
	AuditSourceExplicit = "explicit"
)

// canonicalJSON renders v with sorted object keys (encoding/json sorts map keys),
// so the same logical value always hashes to the same bytes.
func canonicalJSON(v interface{}) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	s := string(b)
	if s == "null" {
		return ""
	}
	return s
}

// CanonicalPayload is the exact, ordered byte sequence an event's hash covers.
// Field order is fixed here and must never be reordered: doing so would
// invalidate every previously computed hash.
func (e *AuditEvent) CanonicalPayload() []byte {
	var b strings.Builder
	write := func(k, v string) {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(v)
		b.WriteByte('\n')
	}
	actor := ""
	if e.ActorID != nil {
		actor = e.ActorID.String()
	}
	write("prev", e.PrevHash)
	write("seq", strconv.FormatInt(e.Sequence, 10))
	write("id", e.ID.String())
	write("tenant", e.TenantID.String())
	write("actor", actor)
	write("action", string(e.Action))
	write("entity_type", e.EntityType)
	write("entity_id", e.EntityID)
	write("summary", e.Summary)
	write("before", canonicalJSON(e.Before))
	write("after", canonicalJSON(e.After))
	write("changed", canonicalJSON([]string(e.ChangedFields)))
	write("ip", e.IPAddress)
	write("ua", e.UserAgent)
	write("request_id", e.RequestID)
	write("method", e.Method)
	write("path", e.Path)
	write("status", strconv.Itoa(e.StatusCode))
	write("source", e.Source)
	write("at", e.CreatedAt.UTC().Format(time.RFC3339Nano))
	return []byte(b.String())
}

// ComputeHash returns the SHA-256 of the canonical payload, hex-encoded.
func (e *AuditEvent) ComputeHash() string {
	sum := sha256.Sum256(e.CanonicalPayload())
	return hex.EncodeToString(sum[:])
}

// SealChain stamps Sequence/PrevHash and computes Hash. Callers must set
// CreatedAt first — the timestamp is part of what the hash covers, so letting
// the database default fill it in afterwards would produce an unverifiable row.
func (e *AuditEvent) SealChain(sequence int64, prevHash string) {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	e.CreatedAt = e.CreatedAt.UTC()
	e.Sequence = sequence
	e.PrevHash = prevHash
	e.Hash = e.ComputeHash()
}

// VerifyHash reports whether the stored hash still matches the stored content.
func (e *AuditEvent) VerifyHash() bool {
	return e.Hash != "" && e.Hash == e.ComputeHash()
}

// =============================================================================
// Chain seals — how retention pruning stays honest
// =============================================================================

// AuditChainSeal records that a contiguous range of a tenant's audit events was
// removed by retention. It preserves the hash the next surviving event links
// back to, so verification can cross the gap knowingly instead of reporting a
// break it cannot explain. Seals themselves are append-only.
type AuditChainSeal struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID     uuid.UUID `gorm:"type:uuid;index;not null" json:"tenant_id"`
	Reason       string    `gorm:"type:varchar(64)" json:"reason"`
	FromSequence int64     `json:"from_sequence"`
	ToSequence   int64     `json:"to_sequence"`
	PrunedCount  int64     `json:"pruned_count"`
	// LastHash is the hash of the last pruned event — the value the first
	// surviving event's prev_hash must equal.
	LastHash  string    `gorm:"type:varchar(64)" json:"last_hash"`
	CreatedAt time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

func (AuditChainSeal) TableName() string { return "audit_chain_seals" }

// =============================================================================
// Retention policy
// =============================================================================

// AuditRetentionPolicy is the tenant's configurable retention window. Zero days
// means "keep forever" — the safe default for a compliance product, chosen
// explicitly rather than inherited from an empty column.
type AuditRetentionPolicy struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID      uuid.UUID  `gorm:"type:uuid;uniqueIndex;not null" json:"tenant_id"`
	RetentionDays int        `gorm:"default:0" json:"retention_days"`
	LastPrunedAt  *time.Time `json:"last_pruned_at,omitempty"`
	UpdatedBy     *uuid.UUID `gorm:"type:uuid" json:"updated_by,omitempty"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (AuditRetentionPolicy) TableName() string { return "audit_retention_policies" }

// Validate keeps the window inside a sane range. The upper bound is 10 years.
func (p *AuditRetentionPolicy) Validate() error {
	if p.RetentionDays < 0 {
		return NewValidationError("retention_days cannot be negative")
	}
	if p.RetentionDays > 3650 {
		return NewValidationError("retention_days cannot exceed 3650 (10 years)")
	}
	// A window shorter than a month is almost always a mistake in a GRC tool and
	// would destroy evidence a regulator may ask for.
	if p.RetentionDays > 0 && p.RetentionDays < 30 {
		return NewValidationError("retention_days must be 0 (keep forever) or at least 30")
	}
	return nil
}

// =============================================================================
// Verification report
// =============================================================================

// Break kinds reported by chain verification.
const (
	BreakHashMismatch = "hash_mismatch"  // stored hash ≠ recomputed hash → content altered
	BreakPrevMismatch = "prev_mismatch"  // link to the previous entry is wrong → entry removed/inserted
	BreakSequenceGap  = "sequence_gap"   // a sequence number is missing and no seal explains it
	BreakUnsealedHead = "unsealed_head"  // the chain does not start at genesis and no seal covers the gap
	BreakDuplicateSeq = "duplicate_sequence"
)

// AuditChainBreak locates one detected alteration.
type AuditChainBreak struct {
	Sequence int64  `json:"sequence"`
	EventID  string `json:"event_id,omitempty"`
	Kind     string `json:"kind"`
	Detail   string `json:"detail"`
}

// AuditChainReport is the outcome of verifying a tenant's audit chain.
type AuditChainReport struct {
	TenantID      uuid.UUID         `json:"tenant_id"`
	Valid         bool              `json:"valid"`
	TotalEvents   int64             `json:"total_events"`
	Verified      int64             `json:"verified"`
	FirstSequence int64             `json:"first_sequence"`
	LastSequence  int64             `json:"last_sequence"`
	HeadHash      string            `json:"head_hash"`
	Seals         int               `json:"seals"`
	Breaks        []AuditChainBreak `json:"breaks"`
	CheckedAt     time.Time         `json:"checked_at"`
	DurationMS    int64             `json:"duration_ms"`
}

// VerifyAuditChain replays a tenant's events in sequence order and reports every
// detected alteration. It is a pure function of the rows handed to it, so it is
// trivially testable and identical whether the rows came from Postgres, a CSV
// export or an offline archive.
//
// events must be ordered by Sequence ascending; seals may arrive in any order.
func VerifyAuditChain(tenantID uuid.UUID, events []AuditEvent, seals []AuditChainSeal) AuditChainReport {
	start := time.Now()
	rep := AuditChainReport{
		TenantID:  tenantID,
		Valid:     true,
		Seals:     len(seals),
		Breaks:    []AuditChainBreak{},
		CheckedAt: start.UTC(),
	}
	rep.TotalEvents = int64(len(events))
	if len(events) == 0 {
		rep.DurationMS = time.Since(start).Milliseconds()
		return rep
	}

	// sealAfter[seq] = hash the event with sequence seq+1 must link to, when a
	// seal explains the gap before it.
	sealAfter := make(map[int64]string, len(seals))
	for _, s := range seals {
		sealAfter[s.ToSequence] = s.LastHash
	}

	prevHash := GenesisHash
	var prevSeq int64
	for i := range events {
		ev := events[i]
		expectPrev := prevHash

		switch {
		case i == 0 && ev.Sequence > 1:
			// Chain does not start at genesis: a seal must vouch for the gap.
			if h, ok := sealAfter[ev.Sequence-1]; ok {
				expectPrev = h
			} else {
				rep.Breaks = append(rep.Breaks, AuditChainBreak{
					Sequence: ev.Sequence, EventID: ev.ID.String(), Kind: BreakUnsealedHead,
					Detail: "chain starts at sequence " + strconv.FormatInt(ev.Sequence, 10) + " with no retention seal covering the earlier entries",
				})
				rep.Valid = false
				expectPrev = ev.PrevHash // don't cascade this one break into every later entry
			}
		case i > 0 && ev.Sequence == prevSeq:
			rep.Breaks = append(rep.Breaks, AuditChainBreak{
				Sequence: ev.Sequence, EventID: ev.ID.String(), Kind: BreakDuplicateSeq,
				Detail: "two entries share sequence " + strconv.FormatInt(ev.Sequence, 10),
			})
			rep.Valid = false
		case i > 0 && ev.Sequence != prevSeq+1:
			if h, ok := sealAfter[ev.Sequence-1]; ok {
				expectPrev = h
			} else {
				rep.Breaks = append(rep.Breaks, AuditChainBreak{
					Sequence: ev.Sequence, EventID: ev.ID.String(), Kind: BreakSequenceGap,
					Detail: "sequence jumps from " + strconv.FormatInt(prevSeq, 10) + " to " + strconv.FormatInt(ev.Sequence, 10) + " with no retention seal",
				})
				rep.Valid = false
				expectPrev = ev.PrevHash
			}
		}

		if ev.PrevHash != expectPrev {
			rep.Breaks = append(rep.Breaks, AuditChainBreak{
				Sequence: ev.Sequence, EventID: ev.ID.String(), Kind: BreakPrevMismatch,
				Detail: "entry links to " + shortHash(ev.PrevHash) + " but the preceding entry hashes to " + shortHash(expectPrev),
			})
			rep.Valid = false
		}
		if !ev.VerifyHash() {
			rep.Breaks = append(rep.Breaks, AuditChainBreak{
				Sequence: ev.Sequence, EventID: ev.ID.String(), Kind: BreakHashMismatch,
				Detail: "stored hash " + shortHash(ev.Hash) + " does not match the entry's content (recomputed " + shortHash(ev.ComputeHash()) + ")",
			})
			rep.Valid = false
		} else {
			rep.Verified++
		}

		prevHash = ev.Hash
		prevSeq = ev.Sequence
	}

	rep.FirstSequence = events[0].Sequence
	rep.LastSequence = events[len(events)-1].Sequence
	rep.HeadHash = events[len(events)-1].Hash
	rep.DurationMS = time.Since(start).Milliseconds()
	return rep
}

func shortHash(h string) string {
	if h == "" {
		return "(genesis)"
	}
	if len(h) <= 12 {
		return h
	}
	return h[:12] + "…"
}

// =============================================================================
// Ports
// =============================================================================

// AuditChainRepository extends the append-only trail with the operations the
// tamper-evidence story needs: full ordered reads for verification and export,
// and retention pruning that seals what it removes.
type AuditChainRepository interface {
	AuditEventRepository
	// ListAll returns every matching event ordered by sequence ASC (no paging) —
	// used by verification and export.
	ListAll(ctx context.Context, tenantID uuid.UUID, f AuditEventFilter) ([]AuditEvent, error)
	// ListSeals returns the tenant's retention seals.
	ListSeals(ctx context.Context, tenantID uuid.UUID) ([]AuditChainSeal, error)
	// Prune deletes events strictly older than `before`, writing a seal that
	// preserves the link for the surviving head. Returns the seal (nil if
	// nothing was eligible).
	Prune(ctx context.Context, tenantID uuid.UUID, before time.Time) (*AuditChainSeal, error)
}

// AuditRetentionRepository stores the per-tenant retention window.
type AuditRetentionRepository interface {
	Get(ctx context.Context, tenantID uuid.UUID) (*AuditRetentionPolicy, error)
	Upsert(ctx context.Context, p *AuditRetentionPolicy) error
	// ListWithRetention returns every tenant policy with a finite window, for the
	// cross-tenant pruning sweep.
	ListWithRetention(ctx context.Context) ([]AuditRetentionPolicy, error)
}
