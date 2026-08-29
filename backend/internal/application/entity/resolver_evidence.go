// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entity

import (
	"context"
	"strconv"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// EvidenceReader is the narrow port over the evidence repository.
type EvidenceReader interface {
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Evidence, error)
}

// EvidenceResolver serves the evidence drawer.
type EvidenceResolver struct {
	evidence  EvidenceReader
	relations RelationReader
}

func NewEvidenceResolver(evidence EvidenceReader, relations RelationReader) *EvidenceResolver {
	return &EvidenceResolver{evidence: evidence, relations: relations}
}

func (r *EvidenceResolver) load(ctx context.Context, c Caller, id string) (*domain.Evidence, uuid.UUID, error) {
	eid, err := uuid.Parse(id)
	if err != nil {
		return nil, uuid.Nil, domain.NewNotFoundError("evidence", id)
	}
	if r.evidence == nil {
		return nil, uuid.Nil, domain.NewNotFoundError("evidence", id)
	}
	// Note the argument order: this repository takes (tenant, id) while the
	// others take (id, tenant). Both are tenant-scoped; the inconsistency is the
	// module's, and it is called out here because getting it backwards would
	// compile and silently query the wrong column pair.
	ev, err := r.evidence.GetByID(ctx, c.TenantID, eid)
	if err != nil {
		return nil, uuid.Nil, err
	}
	if ev == nil {
		return nil, uuid.Nil, domain.NewNotFoundError("evidence", id)
	}
	return ev, eid, nil
}

func (r *EvidenceResolver) Summary(ctx context.Context, c Caller, id string) (*Summary, error) {
	ev, _, err := r.load(ctx, c, id)
	if err != nil {
		return nil, err
	}

	title := ev.Title
	if title == "" {
		title = ev.Filename
	}

	s := &Summary{
		ID:        ev.ID.String(),
		Title:     title,
		Subtitle:  ev.Description,
		Status:    evidenceStatusChip(string(ev.Status)),
		CreatedAt: timePtr(ev.CreatedAt),
		UpdatedAt: timePtr(ev.UpdatedAt),
		// Evidence has no score. Its worth is its review verdict and its
		// freshness, both of which are states, not measurements.
		Score: unavailableScore("review", "Score", "evidence carries a review verdict and a freshness state, not a score"),
	}

	s.Owner = actorFor(ev.OwnerID)
	if s.Owner == nil {
		s.Owner = actorFor(ev.CollectedBy)
		if s.Owner != nil && ev.CollectedByEmail != "" {
			s.Owner.Email = ev.CollectedByEmail
			s.Owner.Label = ev.CollectedByEmail
		}
	}

	s.Fields = appendField(s.Fields, field("evidence_type", "Artifact type", titleOf(string(ev.Type)), FieldBadge))
	s.Fields = appendField(s.Fields, field("review", "Review", titleOf(string(ev.Review)), FieldBadge))
	s.Fields = appendField(s.Fields, field("review_note", "Review note", ev.ReviewNote, FieldMultilne))
	s.Fields = appendField(s.Fields, field("source", "Provenance", titleOf(string(ev.Source)), FieldBadge))
	s.Fields = appendField(s.Fields, field("source_detail", "Provenance detail", ev.SourceDetail, FieldText))
	s.Fields = appendField(s.Fields, field("filename", "File", ev.Filename, FieldText))
	if ev.ExternalURL != "" {
		// Kept distinct from a held file so the client never offers a download
		// for bytes the product does not have.
		s.Fields = append(s.Fields, Field{
			Key: "external_url", Label: "System of record", Kind: FieldLink,
			Value: ev.ExternalURL, Href: ev.ExternalURL,
		})
	}
	s.Fields = append(s.Fields, Field{
		Key: "collected_at", Label: "Collected", Kind: FieldDate,
		Value: ev.CollectedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
	if ev.ValidUntil != nil {
		s.Fields = append(s.Fields, Field{
			Key: "valid_until", Label: "Valid until", Kind: FieldDate,
			Value: ev.ValidUntil.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	if ev.DaysUntilExpiry != nil {
		s.Fields = append(s.Fields, Field{
			Key: "days_until_expiry", Label: "Days until expiry", Kind: FieldNumber,
			Value: strconv.Itoa(*ev.DaysUntilExpiry),
		})
	}
	return s, nil
}

func (r *EvidenceResolver) Relations(ctx context.Context, c Caller, id string) ([]RelationGroup, error) {
	_, eid, err := r.load(ctx, c, id)
	if err != nil {
		return nil, err
	}
	if r.relations == nil {
		return []RelationGroup{}, nil
	}
	ctrls, total, cErr := r.relations.ControlsForEvidence(ctx, c.TenantID, eid, relationCap)
	return []RelationGroup{
		group("controls", "Controls answered", TypeControl, ctrls, total, cErr, controlChips),
	}, nil
}

func (r *EvidenceResolver) Actions(ctx context.Context, c Caller, id string) []Action {
	base := "/api/v1/evidence/" + id
	out := []Action{}
	if c.Can("compliance:evidences:read") {
		// Download is a read, and it is the single most-used action on an
		// artifact — an evidence drawer that cannot open the proof is a viewer
		// of metadata. Offered only when there is something to download; the
		// client hides it for statement-only evidence.
		out = append(out, Action{
			Key: "download", Label: "Download", Kind: ActionPrimary,
			Method: "GET", Path: base + "/download", Permission: "compliance:evidences:read",
		})
	}
	if c.Can("compliance:evidences:create") {
		out = append(out,
			Action{Key: "edit", Label: "Edit", Kind: ActionSecondary,
				Method: "PATCH", Path: base, Permission: "compliance:evidences:create"},
			Action{Key: "link_control", Label: "Link a control", Kind: ActionSecondary,
				Method: "POST", Path: base + "/links", Permission: "compliance:evidences:create"},
			Action{Key: "review", Label: "Record review", Kind: ActionSecondary,
				Method: "POST", Path: base + "/review", Permission: "compliance:evidences:create"},
		)
	}
	if c.Can("compliance:evidences:delete") {
		out = append(out, Action{
			Key: "delete", Label: "Delete", Kind: ActionDanger,
			Method: "DELETE", Path: base, Permission: "compliance:evidences:delete",
		})
	}
	return out
}

// titleOf is `title` under another name — the evidence resolver shadows the
// package helper with a local variable called `title`.
func titleOf(s string) string { return title(s) }
