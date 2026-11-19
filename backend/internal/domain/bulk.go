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
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Governed bulk operations, generalised beyond the risk register (#582).
//
// #581 made the risk register's bulk actions transactional and audited. This is
// the same guarantee expressed once, as a port a register implements, so the
// next register inherits it instead of copying it — "generalising the pattern to
// six more registers is how one defect becomes seven" is the risk #582 names,
// and a shared engine is the answer to it.
//
// What generalises, and what does not. #581's action set is change_status,
// assign_to, add_tags, remove_tags and delete, and #582 forbids inventing new
// ones. Checked against the models rather than assumed:
//
//	Vulnerability  Status (6 values)          -> change_status, delete
//	Asset          no Status, no Tags,        -> delete only
//	               Owner is free-text
//
// So the two actions that actually generalise today are change_status and
// delete. A register declares which it supports; the engine refuses the rest
// rather than pretending. Adding assign_to or tags means adding the columns
// first, which is a different issue.
//
// The risk register (#599) does carry Tags, so add_tags and remove_tags join
// the set — they are #581's actions, not new ones, and only the risk store
// declares them. assign_to is still absent: the risk register's single-row
// assignment validates the assignee's tenant membership through the ownership
// manager, and a bulk path without that check would be a weaker copy of it.
// ---------------------------------------------------------------------------

// BulkAction is the closed set of generalised bulk actions.
type BulkAction string

const (
	BulkActionChangeStatus BulkAction = "change_status"
	BulkActionDelete       BulkAction = "delete"
	BulkActionAddTags      BulkAction = "add_tags"
	BulkActionRemoveTags   BulkAction = "remove_tags"
)

// MaxBulkItems caps one request, carried over from #581 unchanged. The cap is
// what keeps a synchronous implementation adequate.
const MaxBulkItems = 100

// BulkChange describes one mutation in module-agnostic terms.
type BulkChange struct {
	Action BulkAction `json:"action"`
	// Status is the target value for change_status. Its vocabulary is the
	// module's, so the module validates it — the engine only carries it.
	Status string `json:"status,omitempty"`
	// Tags are the labels add_tags and remove_tags act on.
	Tags []string `json:"tags,omitempty"`
	// Justification is recorded on every audit entry the change produces.
	Justification string `json:"justification,omitempty"`
}

// Validate checks the module-agnostic half. The module checks its own
// vocabulary in ValidateBulkChange.
func (c BulkChange) Validate() error {
	switch c.Action {
	case BulkActionChangeStatus:
		if strings.TrimSpace(c.Status) == "" {
			return NewValidationError("status is required for change_status")
		}
		return nil
	case BulkActionAddTags, BulkActionRemoveTags:
		for _, t := range c.Tags {
			if strings.TrimSpace(t) == "" {
				return NewValidationError("a tag in the selection is empty")
			}
		}
		if len(c.Tags) == 0 {
			return NewValidationError(fmt.Sprintf("tags are required for %s", c.Action))
		}
		return nil
	case BulkActionDelete:
		return nil
	default:
		return NewValidationError(fmt.Sprintf("unknown bulk action: %q", c.Action))
	}
}

// PredictOn computes the snapshot a row WOULD have after this change, without
// touching anything.
//
// This is the single source of truth for "what will change": the preview shows
// its output, and the store applies the same field. Computing the preview from a
// second, parallel implementation is how a preview starts lying about the
// mutation it precedes.
func (c BulkChange) PredictOn(before map[string]interface{}) map[string]interface{} {
	after := make(map[string]interface{}, len(before))
	for k, v := range before {
		after[k] = v
	}
	switch c.Action {
	case BulkActionChangeStatus:
		after["status"] = c.Status
	case BulkActionAddTags:
		after["tags"] = AddTags(snapshotTags(before["tags"]), c.Tags)
	case BulkActionRemoveTags:
		after["tags"] = RemoveTags(snapshotTags(before["tags"]), c.Tags)
	}
	return after
}

// BulkPredictor is implemented by a store whose change is more than a plain
// field write. A risk's status change goes through Risk.SetState, which also
// moves the canonical state and the lifecycle phase; the generic PredictOn
// would show the status alone and the preview would understate the change.
// The store predicts with the same function it applies, so the two cannot
// drift.
type BulkPredictor interface {
	PredictBulk(row BulkRow, change BulkChange) map[string]interface{}
}

// AddTags unions two tag sets and returns a SORTED slice. Sorted rather than
// map-iteration order: Go randomises map iteration, so an unsorted result would
// make the same request look like a different change on every call.
func AddTags(existing, added []string) []string {
	set := make(map[string]struct{}, len(existing)+len(added))
	for _, t := range existing {
		set[t] = struct{}{}
	}
	for _, t := range added {
		set[t] = struct{}{}
	}
	return sortedTagSet(set)
}

// RemoveTags subtracts the given tags. A tag that is already absent is a no-op,
// never an error: the caller has the outcome they asked for.
func RemoveTags(existing, removed []string) []string {
	drop := make(map[string]struct{}, len(removed))
	for _, t := range removed {
		drop[t] = struct{}{}
	}
	keep := make(map[string]struct{}, len(existing))
	for _, t := range existing {
		if _, gone := drop[t]; !gone {
			keep[t] = struct{}{}
		}
	}
	return sortedTagSet(keep)
}

func sortedTagSet(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// snapshotTags reads a snapshot's tags value. A store projects []string; a
// value that went through JSON comes back as []interface{}.
func snapshotTags(v interface{}) []string {
	switch tags := v.(type) {
	case []string:
		return tags
	case []interface{}:
		out := make([]string, 0, len(tags))
		for _, t := range tags {
			if s, ok := t.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// BulkRow is one row as the engine sees it: an id and the narrow snapshot of the
// fields a bulk action may touch. Modules project into this so the engine never
// depends on their concrete types.
type BulkRow struct {
	ID uuid.UUID `json:"id"`
	// Label is what the preview shows a human ("CVE-2021-44228"), never an id.
	Label    string                 `json:"label"`
	Snapshot map[string]interface{} `json:"snapshot"`
}

// BulkMutation is one row's before → after, the unit the audit trail is built
// from. Deliberately the same shape as RiskMutation so both paths journal alike.
type BulkMutation struct {
	ID            uuid.UUID
	Label         string
	Before        map[string]interface{}
	After         map[string]interface{}
	ChangedFields []string
}

// BulkStore is what a register implements to get governed bulk operations.
//
// ABSOLUTE RULE 2 is the whole point of concentrating this in one port: every
// method takes tenantID and MUST put it in the WHERE clause. A bulk endpoint is
// the worst place to drop a tenant filter — one unfiltered request mutates many
// rows across tenants at once rather than leaking a single one.
//
// A row belonging to another tenant, or an id that never existed, must both fail
// the same way: LoadBulk omits it (the engine then reports not-found), and the
// write paths must affect nothing.
type BulkStore interface {
	// EntityType is the audit trail's name for this register ("vulnerability").
	EntityType() string

	// SupportedBulkActions is what this register can actually do. The engine
	// refuses anything absent from it, so a module never half-implements an
	// action.
	SupportedBulkActions() []BulkAction

	// ValidateBulkChange checks the change against the module's own vocabulary
	// (its status enum). Returns ErrValidation.
	ValidateBulkChange(change BulkChange) error

	// LoadBulk reads the named rows, tenant-scoped, WITHOUT locking or mutating.
	// It backs the preview, so it must have no side effects whatsoever.
	// Rows absent or owned by another tenant are simply not returned.
	LoadBulk(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) ([]BulkRow, error)

	// ApplyBulk applies the change to every named row inside ONE transaction.
	// All-or-nothing (D-036): any id that does not resolve fails the batch with
	// ErrNotFound and writes nothing.
	ApplyBulk(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID, change BulkChange) ([]BulkMutation, error)

	// DeleteBulk soft-deletes every named row, all or none.
	DeleteBulk(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) ([]BulkMutation, error)
}

// SupportsBulkAction reports whether a store declares an action.
func SupportsBulkAction(s BulkStore, a BulkAction) bool {
	for _, supported := range s.SupportedBulkActions() {
		if supported == a {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Preview fingerprint
// ---------------------------------------------------------------------------

// BulkFingerprint is a stable digest of the exact rows a preview was computed
// over, in the exact state they were in.
//
// It is what answers #582 criterion 3: the client sends it back with the apply,
// the server recomputes it from a fresh read, and a mismatch means the set moved
// under the user between seeing the preview and confirming it. Applying anyway
// would silently act on a set they never saw.
//
// Sorted by id so the digest does not depend on request order, and computed over
// the same narrow snapshot the preview displayed — not over the whole row, or an
// unrelated column changing elsewhere would invalidate a preview that is still
// perfectly accurate.
func BulkFingerprint(rows []BulkRow) string {
	ordered := make([]BulkRow, len(rows))
	copy(ordered, rows)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].ID.String() < ordered[j].ID.String()
	})

	h := sha256.New()
	for _, r := range ordered {
		h.Write([]byte(r.ID.String()))
		// json.Marshal sorts map keys, so the encoding is stable.
		encoded, err := json.Marshal(r.Snapshot)
		if err != nil {
			// A snapshot that cannot be encoded must not silently produce a
			// digest that ignores it.
			encoded = []byte(fmt.Sprintf("unencodable:%v", r.Snapshot))
		}
		h.Write(encoded)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// BulkChangedFields names the snapshot keys whose value differs, in a stable
// order, so a row a change would not actually alter is visible as such.
func BulkChangedFields(before, after map[string]interface{}) []string {
	keys := make([]string, 0, len(before))
	for k := range before {
		keys = append(keys, k)
	}
	for k := range after {
		if _, seen := before[k]; !seen {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	changed := []string{}
	for _, k := range keys {
		if fmt.Sprintf("%v", before[k]) != fmt.Sprintf("%v", after[k]) {
			changed = append(changed, k)
		}
	}
	return changed
}
