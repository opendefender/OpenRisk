// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package evidence is the evidence library: reusable proof artifacts, their
// links to controls, their expiry, and the "what am I missing" view.
//
// It sits beside application/compliance rather than inside it because evidence
// outlives any one framework. The same certificate answers ISO, SOC 2 and a
// customer questionnaire; owning it from the compliance package would make it a
// property of whichever framework happened to reference it first.
package evidence

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/storage"
)

// ControlLookup is the narrow slice of the compliance repository this package
// needs: enough to check a control belongs to the tenant before linking to it,
// and to label links for display.
//
// A narrow port rather than domain.ComplianceRepository so the library cannot
// quietly grow the ability to mutate the control register.
type ControlLookup interface {
	GetControlByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.ComplianceControl, error)
	ListControlsByFramework(ctx context.Context, tenantID uuid.UUID, frameworkID uuid.UUID) ([]domain.ComplianceControl, error)
	GetFrameworkByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.ComplianceFramework, error)
	ListFrameworks(ctx context.Context, tenantID uuid.UUID) ([]domain.ComplianceFramework, error)
}

// UserLookup resolves actor ids to emails for display. Optional: a nil lookup
// degrades to showing the raw id, never to failing the read.
type UserLookup interface {
	EmailsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// Service is the evidence library's use cases.
//
// One struct rather than a file per verb: these operations share the decoration
// pipeline (derive status, materialise links, resolve actors) and splitting them
// would mean duplicating it five times or inventing a sixth type to hold it.
type Service struct {
	repo     domain.EvidenceRepository
	controls ControlLookup
	storage  storage.Storage
	users    UserLookup
	// now is injectable so expiry behaviour is testable without sleeping.
	now func() time.Time
}

func NewService(repo domain.EvidenceRepository, controls ControlLookup, store storage.Storage) *Service {
	return &Service{repo: repo, controls: controls, storage: store, now: time.Now}
}

// WithUserLookup is optional and nil-safe.
func (s *Service) WithUserLookup(u UserLookup) *Service {
	if u != nil {
		s.users = u
	}
	return s
}

// WithClock overrides the clock (tests).
func (s *Service) WithClock(f func() time.Time) *Service {
	if f != nil {
		s.now = f
	}
	return s
}

// =============================================================================
// Create
// =============================================================================

// CreateInput describes a new artifact. Content is optional: evidence can be a
// file, or a reference to one held in another system of record.
type CreateInput struct {
	Title        string
	Type         string
	Description  string
	Source       string
	SourceDetail string
	ExternalURL  string

	Filename string
	Content  io.Reader

	CollectedAt *time.Time
	ValidUntil  *time.Time
	Review      string

	// ControlIDs the artifact answers. Every one is checked against the tenant
	// before anything is written.
	ControlIDs []uuid.UUID

	CollectedBy uuid.UUID
}

func (s *Service) Create(ctx context.Context, tenantID uuid.UUID, in CreateInput) (*domain.Evidence, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewValidationError("tenant is required")
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = strings.TrimSpace(in.Filename)
	}
	if title == "" {
		return nil, domain.NewValidationError("a title or a file is required")
	}
	// Evidence that is neither a file, nor a link, nor a written statement is not
	// evidence — it is a row claiming a control is covered by nothing.
	if in.Content == nil && strings.TrimSpace(in.ExternalURL) == "" && strings.TrimSpace(in.Description) == "" {
		return nil, domain.NewValidationError("evidence needs a file, a link, or a description")
	}

	evType, err := domain.ParseEvidenceType(in.Type)
	if err != nil {
		return nil, err
	}
	source, err := domain.ParseEvidenceSource(in.Source)
	if err != nil {
		return nil, err
	}
	review, err := domain.ParseEvidenceReview(in.Review)
	if err != nil {
		return nil, err
	}

	collectedAt := s.now()
	if in.CollectedAt != nil {
		collectedAt = *in.CollectedAt
	}
	if in.ValidUntil != nil && !in.ValidUntil.After(collectedAt) {
		// An expiry at or before collection describes proof that was never valid.
		// Better to refuse than to file something that reads as expired the moment
		// it lands and quietly stops covering anything.
		return nil, domain.NewValidationError("valid_until must be after collected_at")
	}

	// Verify every control BEFORE touching storage: a tenant must never attach
	// proof to a control it cannot see, and a half-written upload is worse than a
	// refused one.
	if err := s.assertControlsOwned(ctx, tenantID, in.ControlIDs); err != nil {
		return nil, err
	}

	ev := &domain.Evidence{
		ID:           uuid.New(),
		TenantID:     tenantID,
		Title:        title,
		Type:         evType,
		Description:  in.Description,
		ExternalURL:  strings.TrimSpace(in.ExternalURL),
		Filename:     in.Filename,
		CollectedAt:  collectedAt,
		ValidUntil:   in.ValidUntil,
		Review:       review,
		Source:       source,
		SourceDetail: in.SourceDetail,
	}
	if in.CollectedBy != uuid.Nil {
		ev.CollectedBy = &in.CollectedBy
		ev.Ownership.OwnerID = &in.CollectedBy
	}

	if in.Content != nil {
		key, err := s.storage.Save(ctx, tenantID, in.Filename, in.Content)
		if err != nil {
			return nil, domain.NewInternalError("failed to store evidence file: " + err.Error())
		}
		ev.FileRef = key
	}

	if err := s.repo.Create(ctx, ev); err != nil {
		if ev.FileRef != "" {
			// Never leave an orphaned blob behind a failed row.
			_ = s.storage.Delete(ctx, ev.FileRef)
		}
		return nil, err
	}

	for _, cid := range in.ControlIDs {
		link := &domain.EvidenceControlLink{
			ID: uuid.New(), TenantID: tenantID, EvidenceID: ev.ID, ControlID: cid,
		}
		if in.CollectedBy != uuid.Nil {
			link.LinkedBy = &in.CollectedBy
		}
		if err := s.repo.Link(ctx, link); err != nil {
			return nil, err
		}
	}

	return s.decorateOne(ctx, tenantID, ev)
}

// =============================================================================
// Read
// =============================================================================

func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (*domain.Evidence, error) {
	ev, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if ev == nil {
		return nil, domain.NewNotFoundError("evidence", id)
	}
	return s.decorateOne(ctx, tenantID, ev)
}

// ListResult is a page of the library.
type ListResult struct {
	Items []domain.Evidence `json:"items"`
	Total int64             `json:"total"`
	// Counts by effective status across the WHOLE filtered set, not just this
	// page — the tabs above a list must not change meaning when you paginate.
	Summary StatusSummary `json:"summary"`
}

// StatusSummary counts artifacts by effective status.
type StatusSummary struct {
	Valid    int `json:"valid"`
	Expiring int `json:"expiring_soon"`
	Expired  int `json:"expired"`
	Rejected int `json:"rejected"`
	Pending  int `json:"pending"`
}

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, f domain.EvidenceFilter) (*ListResult, error) {
	items, total, err := s.repo.List(ctx, tenantID, f)
	if err != nil {
		return nil, err
	}
	if err := s.decorateMany(ctx, tenantID, items); err != nil {
		return nil, err
	}

	// The summary spans the filter, not the page. Counting it means a second read
	// without limit/offset; that is one query against a table sized by a tenant's
	// document count, and it is the difference between tabs that tell the truth
	// and tabs that say "3 expired" because three happened to land on page one.
	summaryFilter := f
	summaryFilter.Limit, summaryFilter.Offset = 0, 0
	all, _, err := s.repo.List(ctx, tenantID, summaryFilter)
	if err != nil {
		return nil, err
	}
	now := s.now()
	var sum StatusSummary
	for i := range all {
		switch all[i].EffectiveStatus(now) {
		case domain.EvidenceStatusValid:
			sum.Valid++
		case domain.EvidenceStatusExpiring:
			sum.Expiring++
		case domain.EvidenceStatusExpired:
			sum.Expired++
		case domain.EvidenceStatusRejected:
			sum.Rejected++
		case domain.EvidenceStatusPending:
			sum.Pending++
		}
	}

	return &ListResult{Items: items, Total: total, Summary: sum}, nil
}

// ListByControl returns the artifacts attached to one control.
func (s *Service) ListByControl(ctx context.Context, tenantID, controlID uuid.UUID) ([]domain.Evidence, error) {
	control, err := s.controls.GetControlByID(ctx, controlID, tenantID)
	if err != nil {
		return nil, err
	}
	if control == nil {
		return nil, domain.NewNotFoundError("control", controlID)
	}
	items, err := s.repo.ListByControl(ctx, tenantID, controlID)
	if err != nil {
		return nil, err
	}
	if err := s.decorateMany(ctx, tenantID, items); err != nil {
		return nil, err
	}
	return items, nil
}

// Download streams the artifact's bytes. The only path to file content: it
// re-verifies tenant ownership on every call rather than trusting a key.
func (s *Service) Download(ctx context.Context, tenantID, id uuid.UUID) (*domain.Evidence, io.ReadCloser, error) {
	ev, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	if ev == nil {
		return nil, nil, domain.NewNotFoundError("evidence", id)
	}
	if ev.FileRef == "" {
		// A link or a written statement has no bytes to serve. Saying so is better
		// than a 500 from a storage layer asked for an empty key.
		return nil, nil, domain.NewValidationError("this evidence holds no file (it is a link or a statement)")
	}
	content, err := s.storage.Open(ctx, ev.FileRef)
	if err != nil {
		return nil, nil, domain.NewInternalError("evidence file is missing from storage: " + err.Error())
	}
	return ev, content, nil
}

// =============================================================================
// Update / review / delete
// =============================================================================

// UpdateInput patches metadata. Pointer fields are tri-state: nil means "leave
// alone", which is what keeps a form that does not render a field from silently
// clearing it.
type UpdateInput struct {
	Title       *string
	Type        *string
	Description *string
	ExternalURL *string
	CollectedAt *time.Time
	// ValidUntil is doubly-optional: a nil pointer leaves it, a non-nil pointer to
	// nil clears the expiry.
	ValidUntil   **time.Time
	SourceDetail *string
}

func (s *Service) Update(ctx context.Context, tenantID, id uuid.UUID, in UpdateInput) (*domain.Evidence, error) {
	ev, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if ev == nil {
		return nil, domain.NewNotFoundError("evidence", id)
	}

	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if t == "" {
			return nil, domain.NewValidationError("title cannot be empty")
		}
		ev.Title = t
	}
	if in.Type != nil {
		t, err := domain.ParseEvidenceType(*in.Type)
		if err != nil {
			return nil, err
		}
		ev.Type = t
	}
	if in.Description != nil {
		ev.Description = *in.Description
	}
	if in.ExternalURL != nil {
		ev.ExternalURL = strings.TrimSpace(*in.ExternalURL)
	}
	if in.SourceDetail != nil {
		ev.SourceDetail = *in.SourceDetail
	}
	if in.CollectedAt != nil {
		ev.CollectedAt = *in.CollectedAt
	}
	if in.ValidUntil != nil {
		ev.ValidUntil = *in.ValidUntil
	}
	if ev.ValidUntil != nil && !ev.ValidUntil.After(ev.CollectedAt) {
		return nil, domain.NewValidationError("valid_until must be after collected_at")
	}

	if err := s.repo.Update(ctx, ev); err != nil {
		return nil, err
	}
	return s.decorateOne(ctx, tenantID, ev)
}

// Review records a human verdict. Rejecting requires a reason, because a
// rejection with no reason is an artifact nobody can fix.
func (s *Service) Review(ctx context.Context, tenantID, id, reviewer uuid.UUID, verdict, note string) (*domain.Evidence, error) {
	r, err := domain.ParseEvidenceReview(verdict)
	if err != nil {
		return nil, err
	}
	if r == domain.EvidenceReviewRejected && strings.TrimSpace(note) == "" {
		return nil, domain.NewValidationError("a rejection must say why")
	}

	ev, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if ev == nil {
		return nil, domain.NewNotFoundError("evidence", id)
	}

	now := s.now()
	ev.Review = r
	ev.ReviewNote = note
	ev.ReviewedAt = &now
	if reviewer != uuid.Nil {
		ev.ReviewedBy = &reviewer
	}
	if err := s.repo.Update(ctx, ev); err != nil {
		return nil, err
	}
	return s.decorateOne(ctx, tenantID, ev)
}

// Delete removes the artifact, its links and its bytes.
func (s *Service) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	ev, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if ev == nil {
		return domain.NewNotFoundError("evidence", id)
	}
	if err := s.repo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	if ev.FileRef != "" {
		// Best-effort: the row is gone, and a blob that outlives it is a storage
		// cost, not a correctness problem. Failing the request here would tell the
		// user the delete did not happen when it did.
		_ = s.storage.Delete(ctx, ev.FileRef)
	}
	return nil
}

// =============================================================================
// Linking
// =============================================================================

// Link attaches an existing artifact to controls — the reuse path. This is the
// operation that turns a document store into a library: proving a second
// framework's control costs one click, not one re-upload.
func (s *Service) Link(ctx context.Context, tenantID, evidenceID uuid.UUID, controlIDs []uuid.UUID, note string, actor uuid.UUID) (*domain.Evidence, error) {
	ev, err := s.repo.GetByID(ctx, tenantID, evidenceID)
	if err != nil {
		return nil, err
	}
	if ev == nil {
		return nil, domain.NewNotFoundError("evidence", evidenceID)
	}
	if len(controlIDs) == 0 {
		return nil, domain.NewValidationError("at least one control is required")
	}
	if err := s.assertControlsOwned(ctx, tenantID, controlIDs); err != nil {
		return nil, err
	}
	for _, cid := range controlIDs {
		link := &domain.EvidenceControlLink{
			ID: uuid.New(), TenantID: tenantID, EvidenceID: evidenceID, ControlID: cid, Note: note,
		}
		if actor != uuid.Nil {
			link.LinkedBy = &actor
		}
		if err := s.repo.Link(ctx, link); err != nil {
			return nil, err
		}
	}
	return s.decorateOne(ctx, tenantID, ev)
}

func (s *Service) Unlink(ctx context.Context, tenantID, evidenceID, controlID uuid.UUID) error {
	ev, err := s.repo.GetByID(ctx, tenantID, evidenceID)
	if err != nil {
		return err
	}
	if ev == nil {
		return domain.NewNotFoundError("evidence", evidenceID)
	}
	return s.repo.Unlink(ctx, tenantID, evidenceID, controlID)
}

// assertControlsOwned refuses the whole operation if any control is not the
// tenant's. All-or-nothing: a partial link would leave the user believing they
// attached proof to controls they did not.
func (s *Service) assertControlsOwned(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) error {
	for _, cid := range ids {
		if cid == uuid.Nil {
			return domain.NewValidationError("invalid control id")
		}
		c, err := s.controls.GetControlByID(ctx, cid, tenantID)
		if err != nil {
			return err
		}
		if c == nil {
			return domain.NewNotFoundError("control", cid)
		}
	}
	return nil
}

// =============================================================================
// Decoration
// =============================================================================

func (s *Service) decorateOne(ctx context.Context, tenantID uuid.UUID, ev *domain.Evidence) (*domain.Evidence, error) {
	items := []domain.Evidence{*ev}
	if err := s.decorateMany(ctx, tenantID, items); err != nil {
		return nil, err
	}
	return &items[0], nil
}

// decorateMany fills status, links and actor emails for a page of artifacts in a
// bounded number of queries: one for links, one for the controls they name, one
// for the actors. Never one per row.
func (s *Service) decorateMany(ctx context.Context, tenantID uuid.UUID, items []domain.Evidence) error {
	now := s.now()
	if len(items) == 0 {
		return nil
	}

	ids := make([]uuid.UUID, 0, len(items))
	for i := range items {
		ids = append(ids, items[i].ID)
	}
	links, err := s.repo.ListLinks(ctx, tenantID, ids)
	if err != nil {
		return err
	}

	byEvidence := make(map[uuid.UUID][]uuid.UUID, len(items))
	controlIDs := make(map[uuid.UUID]bool)
	for _, l := range links {
		byEvidence[l.EvidenceID] = append(byEvidence[l.EvidenceID], l.ControlID)
		controlIDs[l.ControlID] = true
	}

	// Label the linked controls. Resolved through the control lookup one id at a
	// time, but only for the DISTINCT controls this page references — a page of
	// twenty artifacts sharing three controls costs three lookups, not sixty.
	refs := make(map[uuid.UUID]domain.EvidenceControlRef, len(controlIDs))
	frameworkNames := map[uuid.UUID]string{}
	for cid := range controlIDs {
		c, err := s.controls.GetControlByID(ctx, cid, tenantID)
		if err != nil || c == nil {
			// A link to a control that has since been deleted: skip the label rather
			// than fail the read. The link is cleaned up when the control's framework
			// is deleted; until then the artifact is still perfectly readable.
			continue
		}
		fwName, ok := frameworkNames[c.FrameworkID]
		if !ok {
			if fw, err := s.controls.GetFrameworkByID(ctx, c.FrameworkID, tenantID); err == nil && fw != nil {
				fwName = fw.Name
			}
			frameworkNames[c.FrameworkID] = fwName
		}
		refs[cid] = domain.EvidenceControlRef{
			ControlID: c.ID, ReferenceCode: c.ReferenceCode, Name: c.Name,
			FrameworkID: c.FrameworkID, FrameworkName: fwName,
		}
	}

	actorIDs := make([]uuid.UUID, 0, len(items))
	for i := range items {
		if items[i].CollectedBy != nil {
			actorIDs = append(actorIDs, *items[i].CollectedBy)
		}
	}
	emails := map[uuid.UUID]string{}
	if s.users != nil && len(actorIDs) > 0 {
		if m, err := s.users.EmailsByIDs(ctx, actorIDs); err == nil {
			emails = m
		}
		// A failed lookup degrades to the raw id. Being unable to name the person
		// is not a reason to refuse to show the proof.
	}

	for i := range items {
		ev := &items[i]
		ev.ControlIDs = byEvidence[ev.ID]
		if ev.ControlIDs == nil {
			ev.ControlIDs = []uuid.UUID{}
		}
		ev.Controls = make([]domain.EvidenceControlRef, 0, len(ev.ControlIDs))
		for _, cid := range ev.ControlIDs {
			if r, ok := refs[cid]; ok {
				ev.Controls = append(ev.Controls, r)
			}
		}
		if ev.CollectedBy != nil {
			ev.CollectedByEmail = emails[*ev.CollectedBy]
		}
		ev.Decorate(now)
	}
	return nil
}
