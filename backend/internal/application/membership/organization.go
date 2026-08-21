// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package membership

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// OrganizationView is the tenant's own profile, as the Settings screen shows
// it. Every field is read from the organizations row or computed from the
// tenant's memberships — none of it is inferred from the browser, and none of
// it is a constant.
//
// Settings used to render the organization name out of a login response, the
// time zone out of Intl.DateTimeFormat() (the VIEWER's zone, not the
// organization's), and nothing else at all. A field appears here only if the
// backend genuinely holds it.
type OrganizationView struct {
	ID        uuid.UUID      `json:"id"`
	Name      string         `json:"name"`
	Slug      string         `json:"slug"`
	LogoURL   string         `json:"logo_url,omitempty"`
	Industry  string         `json:"industry,omitempty"`
	Size      domain.OrgSize `json:"size,omitempty"`
	Plan      domain.OrgPlan `json:"plan"`
	IsActive  bool           `json:"is_active"`
	OwnerID   uuid.UUID      `json:"owner_id"`
	OwnerName string         `json:"owner_name,omitempty"`
	Timezone  string         `json:"timezone,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	// Counts are the live membership numbers, so the profile and the members
	// screen can never disagree about how many people are in the organization.
	Counts domain.OrganizationCounts `json:"counts"`
	// CanEdit tells the UI whether to render the profile as a form or as
	// read-only, using the server's answer rather than its own guess.
	CanEdit bool `json:"can_edit"`
}

// GetOrganization returns the caller's own organization profile with its live
// membership counts.
func (s *Service) GetOrganization(ctx context.Context, tenantID uuid.UUID, canEdit bool) (*OrganizationView, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	if s.orgs == nil {
		return nil, domain.NewInternalError("organization directory unavailable")
	}
	org, err := s.orgs.GetByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, domain.NewNotFoundError("organization", tenantID)
	}

	view := &OrganizationView{
		ID: org.ID, Name: org.Name, Slug: org.Slug, LogoURL: org.LogoURL,
		Industry: org.Industry, Size: org.Size, Plan: org.Plan,
		IsActive: org.IsActive, OwnerID: org.OwnerID,
		CreatedAt: org.CreatedAt, UpdatedAt: org.UpdatedAt,
		CanEdit: canEdit,
	}
	// Timezone lives in the settings jsonb rather than in a column. It is shown
	// only when the organization actually set one — an empty value lets the UI
	// say "not set" instead of presenting the viewer's own zone as the
	// organization's.
	if tz, ok := org.GetSettings()["timezone"].(string); ok {
		view.Timezone = strings.TrimSpace(tz)
	}
	if counts, err := s.repo.Counts(ctx, tenantID); err == nil {
		view.Counts = counts
	}
	if owner, err := s.users.GetByID(ctx, org.OwnerID); err == nil && owner != nil {
		view.OwnerName = owner.FullName
		if view.OwnerName == "" {
			view.OwnerName = owner.Email
		}
	}
	return view, nil
}

// Counts returns the tenant's membership headline on its own. This is what
// feeds the sidebar: one small, tenant-scoped, cacheable call rather than a
// full member listing the sidebar would then count client-side.
func (s *Service) Counts(ctx context.Context, tenantID uuid.UUID) (domain.OrganizationCounts, error) {
	if tenantID == uuid.Nil {
		return domain.OrganizationCounts{}, domain.NewUnauthorizedError("no organization in context")
	}
	return s.repo.Counts(ctx, tenantID)
}

// ---------------------------------------------------------------------------
// Membership audit history
// ---------------------------------------------------------------------------

// membershipEntityTypes are the audit entity types this history covers. It is
// an allowlist rather than a filter over everything: the membership history
// must not become a side channel onto the tenant's whole audit trail.
var membershipEntityTypes = []string{"organization_member", "invitation"}

// AuditEntryView is one row of the membership audit history. It is a
// projection, not the stored event: the hash-chain fields and the raw HTTP
// envelope belong to the governance trail, not to a member-management screen.
type AuditEntryView struct {
	ID         uuid.UUID          `json:"id"`
	At         time.Time          `json:"at"`
	ActorID    *uuid.UUID         `json:"actor_id,omitempty"`
	ActorEmail string             `json:"actor_email,omitempty"`
	Action     domain.AuditAction `json:"action"`
	EntityType string             `json:"entity_type"`
	EntityID   string             `json:"entity_id"`
	Summary    string             `json:"summary"`
	Before     domain.JSONMap     `json:"before,omitempty"`
	After      domain.JSONMap     `json:"after,omitempty"`
	IPAddress  string             `json:"ip_address,omitempty"`
}

// MembershipAudit returns the membership slice of the tenant's audit trail.
//
// Tenant scope comes from the repository, which filters every read by tenant.
// The entity-type allowlist is applied here, and a caller-supplied entity type
// is intersected with it rather than trusted — otherwise ?entity_type=risk
// would turn this endpoint into a general-purpose trail reader for anyone
// holding organization:audit:read.
func (s *Service) MembershipAudit(ctx context.Context, tenantID uuid.UUID, f domain.AuditEventFilter) (*Page[AuditEntryView], error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	if s.reader == nil {
		return nil, domain.NewInternalError("audit trail unavailable")
	}
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}

	requested := membershipEntityTypes
	if f.EntityType != "" {
		allowed := false
		for _, t := range membershipEntityTypes {
			if t == f.EntityType {
				allowed = true
			}
		}
		if !allowed {
			return nil, domain.NewValidationError("entity_type must be one of: organization_member, invitation")
		}
		requested = []string{f.EntityType}
	}

	// The repository filter takes one entity type, so the two are queried and
	// merged. Two small indexed reads beat widening the port for every caller.
	var (
		merged []domain.AuditEvent
		total  int64
	)
	for _, et := range requested {
		q := f
		q.EntityType = et
		// Each slice is fetched up to the page size and the merged result is
		// trimmed, so a page is never short because one entity type dominates.
		rows, n, err := s.reader.List(ctx, tenantID, q)
		if err != nil {
			return nil, err
		}
		merged = append(merged, rows...)
		total += n
	}
	sortEventsNewestFirst(merged)
	if len(merged) > f.Limit {
		merged = merged[:f.Limit]
	}
	s.resolveActorEmails(ctx, merged)

	items := make([]AuditEntryView, 0, len(merged))
	for i := range merged {
		e := merged[i]
		items = append(items, AuditEntryView{
			ID: e.ID, At: e.CreatedAt, ActorID: e.ActorID, ActorEmail: e.ActorEmail,
			Action: e.Action, EntityType: e.EntityType, EntityID: e.EntityID,
			Summary: e.Summary, Before: e.Before, After: e.After, IPAddress: e.IPAddress,
		})
	}
	return &Page[AuditEntryView]{Items: items, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func sortEventsNewestFirst(rows []domain.AuditEvent) {
	// Insertion sort: a page is at most 400 rows before trimming, and this keeps
	// the file free of a sort import for a list that small.
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j].CreatedAt.After(rows[j-1].CreatedAt); j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

// resolveActorEmails turns actor ids into addresses in one query, so a journal
// shows who did something rather than a UUID.
func (s *Service) resolveActorEmails(ctx context.Context, rows []domain.AuditEvent) {
	if s.users == nil || len(rows) == 0 {
		return
	}
	seen := map[uuid.UUID]struct{}{}
	ids := make([]uuid.UUID, 0, len(rows))
	for i := range rows {
		if rows[i].ActorID == nil {
			continue
		}
		if _, ok := seen[*rows[i].ActorID]; ok {
			continue
		}
		seen[*rows[i].ActorID] = struct{}{}
		ids = append(ids, *rows[i].ActorID)
	}
	if len(ids) == 0 {
		return
	}
	byID, err := s.users.EmailsByIDs(ctx, ids)
	if err != nil {
		return
	}
	for i := range rows {
		if rows[i].ActorID != nil {
			rows[i].ActorEmail = byID[*rows[i].ActorID]
		}
	}
}
