// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package ownership

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// TransferEntity names a kind of entity whose owner can be transferred.
type TransferEntity string

const (
	TransferRisk       TransferEntity = "risk"
	TransferMitigation TransferEntity = "mitigation"
	TransferIncident   TransferEntity = "incident"
)

// OwnerStore reads and writes the owner slot of one kind of entity. Every method
// is scoped to the tenant: an id that exists only in another organisation must
// read back as not found, never as someone else's row.
//
// The id is a string because the entities do not share a key type: risks and
// mitigations are uuid-keyed, incidents still use a sequential integer. Each
// store parses its own format and reports a malformed id as not found.
type OwnerStore interface {
	// CurrentOwner returns the entity's owner (nil when it has none) and its
	// title for notification and audit copy. Returns a not-found AppError when
	// the id does not exist in the tenant.
	CurrentOwner(ctx context.Context, tenantID uuid.UUID, id string) (owner *uuid.UUID, title string, err error)
	// SetOwner writes the owner slot, and only that slot. Returns a not-found
	// AppError when no row of the tenant matched.
	SetOwner(ctx context.Context, tenantID uuid.UUID, id string, owner uuid.UUID) error
}

// AuditRecorder is the narrow slice of governance.AuditRecorder this use case
// needs. Optional: without it the transfer still happens, it is just not
// journalled explicitly (the HTTP audit middleware still records the request).
type AuditRecorder interface {
	Record(ctx context.Context, ev domain.AuditEvent)
}

// TransferOwnershipInput is one transfer request.
type TransferOwnershipInput struct {
	Entity     TransferEntity
	ID         string
	NewOwnerID uuid.UUID
	// Actor is the authenticated member performing the transfer. Required: an
	// ownership change nobody can be held to account for is exactly what this
	// use case exists to prevent.
	Actor uuid.UUID
	// Locale drives the notification wording ("fr" | "en").
	Locale string
}

// TransferOwnershipResult reports what changed.
type TransferOwnershipResult struct {
	EntityType      TransferEntity `json:"entity_type"`
	EntityID        string         `json:"entity_id"`
	PreviousOwnerID *uuid.UUID     `json:"previous_owner_id"`
	OwnerID         uuid.UUID      `json:"owner_id"`
	TransferredAt   time.Time      `json:"transferred_at"`
}

// TransferOwnershipUseCase hands the owner slot of a risk, mitigation or
// incident to another active member of the same tenant, records who did it and
// who it moved from and to, and tells the new owner.
//
// It is deliberately narrower than the PATCH endpoints, which can also move an
// owner as a side effect of a form save: here the owner is the only thing that
// changes, the previous owner is captured before the write, and the audit entry
// says "transfer", so a reviewer reading the trail sees a transfer rather than
// one field among twenty in an update.
type TransferOwnershipUseCase struct {
	stores  map[TransferEntity]OwnerStore
	members *Service
	audit   AuditRecorder
	now     func() time.Time
}

// NewTransferOwnershipUseCase builds the use case. members supplies the
// membership check, the email lookup and the notifier; it may be nil in tests,
// in which case the new owner is not membership-checked.
func NewTransferOwnershipUseCase(members *Service, stores map[TransferEntity]OwnerStore) *TransferOwnershipUseCase {
	return &TransferOwnershipUseCase{stores: stores, members: members, now: time.Now}
}

// WithAudit attaches the audit recorder. Nil-safe.
func (uc *TransferOwnershipUseCase) WithAudit(a AuditRecorder) *TransferOwnershipUseCase {
	uc.audit = a
	return uc
}

// Execute performs the transfer.
func (uc *TransferOwnershipUseCase) Execute(ctx context.Context, tenantID uuid.UUID, in TransferOwnershipInput) (*TransferOwnershipResult, error) {
	if tenantID == uuid.Nil || in.Actor == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no tenant or user in session")
	}
	store, ok := uc.stores[in.Entity]
	if !ok || store == nil {
		return nil, domain.NewValidationError(fmt.Sprintf("ownership of %q cannot be transferred", in.Entity))
	}
	if in.NewOwnerID == uuid.Nil {
		return nil, domain.NewValidationError("new_owner_id is required")
	}

	previous, title, err := store.CurrentOwner(ctx, tenantID, in.ID)
	if err != nil {
		return nil, fmt.Errorf("ownership.TransferOwnership: %w", err)
	}
	if previous != nil && *previous == in.NewOwnerID {
		return nil, &domain.AppError{
			Err:     domain.ErrConflict,
			Message: "this user already owns it",
			Code:    http.StatusConflict,
		}
	}

	// Same guard as every other assignment path: an id from another
	// organisation, or a deactivated account, is rejected before the write.
	if uc.members != nil {
		if err := uc.members.Validate(ctx, tenantID, domain.OwnershipPatch{Owner: domain.Assign(in.NewOwnerID)}); err != nil {
			return nil, fmt.Errorf("ownership.TransferOwnership: %w", err)
		}
	}

	if err := store.SetOwner(ctx, tenantID, in.ID, in.NewOwnerID); err != nil {
		return nil, fmt.Errorf("ownership.TransferOwnership: %w", err)
	}

	result := &TransferOwnershipResult{
		EntityType:      in.Entity,
		EntityID:        in.ID,
		PreviousOwnerID: previous,
		OwnerID:         in.NewOwnerID,
		TransferredAt:   uc.now().UTC(),
	}

	// Journal and notify only once the write has succeeded: nobody should be
	// told they own something that was not saved. Both are best-effort.
	uc.record(ctx, tenantID, in, previous, title)
	if uc.members != nil {
		newOwner := in.NewOwnerID
		uc.members.Notify(ctx, tenantID, []domain.OwnershipChange{{
			Role:  domain.RoleOwner,
			From:  previous,
			To:    &newOwner,
			Actor: in.Actor,
		}}, domain.OwnershipSubject{
			ResourceType: string(in.Entity),
			ResourceID:   uuidOrNil(in.ID),
			Title:        title,
			Locale:       in.Locale,
		})
	}
	return result, nil
}

// record writes the audit entry: who moved which entity from whom to whom.
func (uc *TransferOwnershipUseCase) record(ctx context.Context, tenantID uuid.UUID, in TransferOwnershipInput, previous *uuid.UUID, title string) {
	if uc.audit == nil {
		return
	}
	emails := uc.emailsOf(ctx, previous, in.NewOwnerID)
	from := "nobody"
	var before interface{}
	if previous != nil {
		from = labelOf(*previous, emails)
		before = previous.String()
	}
	to := labelOf(in.NewOwnerID, emails)

	name := strings.TrimSpace(title)
	if name == "" {
		name = in.ID
	}
	actor := in.Actor
	uc.audit.Record(ctx, domain.AuditEvent{
		TenantID:      tenantID,
		ActorID:       &actor,
		Action:        domain.AuditActionTransfer,
		EntityType:    string(in.Entity),
		EntityID:      in.ID,
		Summary:       fmt.Sprintf("transferred ownership of %s %q from %s to %s", in.Entity, name, from, to),
		Before:        domain.JSONMap{"owner_id": before},
		After:         domain.JSONMap{"owner_id": in.NewOwnerID.String()},
		ChangedFields: domain.StringList{"owner_id"},
	})
}

// emailsOf resolves the two owners' addresses for a readable summary.
// Degrades to bare ids when the lookup is unavailable or fails.
func (uc *TransferOwnershipUseCase) emailsOf(ctx context.Context, previous *uuid.UUID, next uuid.UUID) map[uuid.UUID]string {
	if uc.members == nil || uc.members.users == nil {
		return nil
	}
	ids := []uuid.UUID{next}
	if previous != nil {
		ids = append(ids, *previous)
	}
	emails, err := uc.members.users.EmailsByIDs(ctx, ids)
	if err != nil {
		return nil
	}
	return emails
}

func labelOf(id uuid.UUID, emails map[uuid.UUID]string) string {
	if e := emails[id]; e != "" {
		return e
	}
	return id.String()
}

// uuidOrNil returns the parsed uuid, or uuid.Nil for integer-keyed entities
// (incidents): the notification is then sent without a resource link.
func uuidOrNil(id string) uuid.UUID {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil
	}
	return parsed
}
