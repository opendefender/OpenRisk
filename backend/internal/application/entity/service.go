// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entity

import (
	"context"

	"github.com/opendefender/openrisk/internal/domain"
)

// Service is the drawer's single entry point. The HTTP layer parses and
// authenticates; every authorisation decision below this line is made here or in
// a resolver, never in a handler.
type Service struct {
	registry *Registry
	timeline *TimelineService
}

func NewService(registry *Registry) *Service {
	return &Service{registry: registry}
}

// WithTimeline attaches the timeline reader. Optional and nil-safe: without it
// the timeline and audit sections answer a typed "unavailable" rather than
// failing the whole drawer, which is the same degradation contract the rest of
// the platform uses for optional ports.
func (s *Service) WithTimeline(t *TimelineService) *Service {
	s.timeline = t
	return s
}

// EntityView is everything the drawer needs to render its head: identity,
// summary, and the actions this caller may take.
//
// Relations and timeline are deliberately NOT in here. They are separate reads
// because they are separately slow, separately permissioned and separately
// allowed to fail — folding them in would mean one failing relation query blanks
// the whole drawer (§27), and it would make opening a drawer cost every query
// the entity could possibly need.
type EntityView struct {
	Summary  *Summary   `json:"summary"`
	Actions  []Action   `json:"actions"`
	Sections []Section  `json:"sections"`
	Type     Descriptor `json:"-"`
}

// Get loads one entity for one caller.
//
// Order matters. The permission gate runs BEFORE the lookup: checking existence
// first and permission second would make a forbidden-but-real id answer
// differently from a fabricated one, which is an enumeration oracle.
func (s *Service) Get(ctx context.Context, c Caller, t Type, id string) (*EntityView, error) {
	desc, res, err := s.access(c, t)
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, domain.NewValidationError("entity id is required")
	}

	summary, err := res.Summary(ctx, c, id)
	if err != nil {
		return nil, err
	}
	if summary == nil {
		return nil, domain.NewNotFoundError(string(t), id)
	}
	summary.Type = t
	summary.TypeLabel = desc.Label
	summary.Sections = s.sectionsFor(c, desc)
	if summary.URL == "" {
		summary.URL = DeepLink(t, summary.ID)
	}

	return &EntityView{
		Summary:  summary,
		Actions:  res.Actions(ctx, c, id),
		Sections: summary.Sections,
		Type:     desc,
	}, nil
}

// Relations loads one entity's edges.
//
// It re-runs the entry gate and re-loads the summary first. That second load
// looks redundant against a client that just called Get — it is not: this is a
// separate HTTP request, and a request that names an entity must prove the
// caller may read THAT entity, in THIS tenant, at the time of the call. Trusting
// that a previous call succeeded is exactly how a relation endpoint becomes an
// IDOR.
func (s *Service) Relations(ctx context.Context, c Caller, t Type, id string) ([]RelationGroup, error) {
	_, res, err := s.access(c, t)
	if err != nil {
		return nil, err
	}
	if _, err := res.Summary(ctx, c, id); err != nil {
		return nil, err
	}

	groups, err := res.Relations(ctx, c, id)
	if err != nil {
		return nil, err
	}

	// Filter by the TARGET type's permission. A caller who may read risks but not
	// assets sees the risk, sees that it has linked assets, and is told they are
	// not visible to them — rather than being shown them, or being shown a list
	// that silently pretends to be complete.
	out := make([]RelationGroup, 0, len(groups))
	for _, g := range groups {
		if !c.CanRead(g.TargetType) {
			g.Items = nil
			g.Total = 0
			g.Truncated = false
			g.Denied = true
		}
		out = append(out, g)
	}
	return out, nil
}

// Timeline reads one entity's history.
func (s *Service) Timeline(ctx context.Context, c Caller, t Type, id string, cursor string, f TimelineFilter) (*TimelinePage, error) {
	_, res, err := s.access(c, t)
	if err != nil {
		return nil, err
	}
	if _, err := res.Summary(ctx, c, id); err != nil {
		return nil, err
	}
	if s.timeline == nil {
		return nil, domain.NewValidationError("timeline is not available on this deployment")
	}
	return s.timeline.ForEntity(ctx, c, Ref{Type: t, ID: id}, cursor, f)
}

// Audit reads one entity's raw audit records.
//
// Separate from Timeline and separately gated (§23). The timeline says "severity
// changed from Medium to High"; the audit trail carries the full before/after
// snapshot, the actor's IP and the request id. The first is context, the second
// is evidence, and they do not carry the same disclosure risk.
func (s *Service) Audit(ctx context.Context, c Caller, t Type, id string, limit, offset int) (*AuditPage, error) {
	_, res, err := s.access(c, t)
	if err != nil {
		return nil, err
	}
	if _, err := res.Summary(ctx, c, id); err != nil {
		return nil, err
	}
	if !CanReadAudit(c) {
		return nil, domain.NewForbiddenError("the audit trail requires the governance audit permission")
	}
	if s.timeline == nil {
		return nil, domain.NewValidationError("the audit trail is not available on this deployment")
	}
	return s.timeline.AuditForEntity(ctx, c, Ref{Type: t, ID: id}, limit, offset)
}

// TenantTimeline reads the tenant-wide activity feed.
func (s *Service) TenantTimeline(ctx context.Context, c Caller, cursor string, f TimelineFilter) (*TimelinePage, error) {
	if !c.Valid() {
		return nil, domain.NewForbiddenError("authentication required")
	}
	if s.timeline == nil {
		return nil, domain.NewValidationError("timeline is not available on this deployment")
	}
	return s.timeline.ForTenant(ctx, c, cursor, f)
}

// Catalogue returns the descriptors of the types this deployment actually
// resolves, annotated with whether the caller may read each. The client uses it
// to know which relation chips are worth rendering as links.
type CatalogueEntry struct {
	Type       Type      `json:"type"`
	Label      string    `json:"label"`
	ListPath   string    `json:"list_path"`
	Permission string    `json:"permission"`
	Readable   bool      `json:"readable"`
	Sections   []Section `json:"sections"`
}

func (s *Service) Catalogue(c Caller) []CatalogueEntry {
	out := make([]CatalogueEntry, 0, len(Types))
	for _, t := range s.registry.Supported() {
		d := descriptors[t]
		out = append(out, CatalogueEntry{
			Type: t, Label: d.Label, ListPath: d.ListPath,
			Permission: d.ReadPermission,
			Readable:   c.CanRead(t),
			Sections:   s.sectionsFor(c, d),
		})
	}
	return out
}

// access is the one gate: caller valid, type known, type readable, resolver
// wired. Everything public above starts here.
func (s *Service) access(c Caller, t Type) (Descriptor, Resolver, error) {
	if !c.Valid() {
		return Descriptor{}, nil, domain.NewForbiddenError("authentication required")
	}
	desc, ok := DescriptorFor(t)
	if !ok {
		return Descriptor{}, nil, domain.NewValidationError("unknown entity type")
	}
	if !c.Can(desc.ReadPermission) {
		return Descriptor{}, nil, domain.NewForbiddenError("missing permission " + desc.ReadPermission)
	}
	res, err := s.registry.Resolver(t)
	if err != nil {
		return Descriptor{}, nil, err
	}
	return desc, res, nil
}

// sectionsFor removes sections this caller cannot use — today, the audit tab for
// a caller without the governance audit permission. A tab that always answers
// 403 is worse than no tab.
func (s *Service) sectionsFor(c Caller, d Descriptor) []Section {
	out := make([]Section, 0, len(d.Sections))
	for _, sec := range d.Sections {
		if sec == SectionAudit && !CanReadAudit(c) {
			continue
		}
		out = append(out, sec)
	}
	return out
}

// AuditPermission is the permission the raw audit trail requires. It is the same
// gate the governance module's own audit routes sit behind, expressed as a
// permission so a business role can hold it without being an org admin.
const AuditPermission = "governance:audit:read"

// CanReadAudit reports whether a caller may see raw audit records. Admins hold
// it through the `*` wildcard, which is how the governance screens already work.
func CanReadAudit(c Caller) bool {
	return c.Can(AuditPermission)
}
