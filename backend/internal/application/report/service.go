// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package report

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/opendefender/openrisk/internal/domain"
)

// UserLookup resolves actors to emails for display. Optional and nil-safe.
type UserLookup interface {
	EmailsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// Service owns the report lifecycle: request, list, read, download, review,
// version and delete. Generation itself happens in the worker.
type Service struct {
	repo    domain.ReportRepository
	sources Sources
	users   UserLookup
	now     func() time.Time
}

func NewService(repo domain.ReportRepository, sources Sources) *Service {
	return &Service{repo: repo, sources: sources, now: time.Now}
}

func (s *Service) WithUserLookup(u UserLookup) *Service {
	if u != nil {
		s.users = u
	}
	return s
}

func (s *Service) WithClock(f func() time.Time) *Service {
	if f != nil {
		s.now = f
	}
	return s
}

// CreateInput is the configurator's answers.
type CreateInput struct {
	Type   string
	Format string
	Locale string
	// Period, scope and recipients — the configurator's four other questions.
	From        *time.Time
	To          *time.Time
	FrameworkID *uuid.UUID
	AuditID     *uuid.UUID
	Recipients  []string
	// Supersedes marks this as a new version of an existing report.
	Supersedes  *uuid.UUID
	RequestedBy uuid.UUID
}

// Create records the request and QUEUES it. It does not render.
//
// Asynchronous because a register of several hundred controls with its evidence
// takes long enough that a synchronous request holds a connection open past the
// point a proxy or a browser will wait — and because the user should be able to
// leave the page. The report has an address of its own the moment it is
// requested, so leaving is safe.
func (s *Service) Create(ctx context.Context, tenantID uuid.UUID, in CreateInput) (*domain.Report, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewValidationError("tenant is required")
	}

	reportType := domain.ReportType(strings.TrimSpace(in.Type))
	if !reportType.Valid() {
		return nil, domain.NewValidationError(fmt.Sprintf("unknown report type: %q", in.Type))
	}
	tpl, err := TemplateFor(reportType)
	if err != nil {
		return nil, err
	}
	format, err := domain.ParseReportFormat(in.Format)
	if err != nil {
		return nil, err
	}
	if !tpl.Supports(format) {
		// Refused rather than silently downgraded to PDF: someone who asked for a
		// spreadsheet wants to filter it, and handing them a PDF that says
		// "spreadsheet" in the filename wastes their time twice.
		return nil, domain.NewValidationError(fmt.Sprintf(
			"the %s report is not produced as %s (available: %s)",
			tpl.Key, format, formatList(tpl.Formats)))
	}
	locale, err := domain.ParseReportLocale(in.Locale)
	if err != nil {
		return nil, err
	}

	if in.From != nil && in.To != nil && in.To.Before(*in.From) {
		return nil, domain.NewValidationError("the period ends before it starts")
	}

	params := map[string]any{}
	if in.From != nil {
		params["from"] = in.From.Format(time.RFC3339)
	}
	if in.To != nil {
		params["to"] = in.To.Format(time.RFC3339)
	}
	if in.FrameworkID != nil {
		params["framework_id"] = in.FrameworkID.String()
	}
	if in.AuditID != nil {
		params["audit_id"] = in.AuditID.String()
	}
	if len(in.Recipients) > 0 {
		params["recipients"] = in.Recipients
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return nil, domain.NewValidationError("invalid report parameters")
	}

	rep := &domain.Report{
		ID:              uuid.New(),
		TenantID:        tenantID,
		Type:            reportType,
		Format:          format,
		Locale:          locale,
		TemplateKey:     tpl.Key,
		TemplateVersion: tpl.Version,
		Params:          datatypes.JSON(encoded),
		Title:           tpl.Title(locale),
		RunState:        domain.ReportRunQueued,
		Lifecycle:       domain.ReportLifecycleDraft,
		Version:         1,
		RequestedBy:     in.RequestedBy,
	}

	// A new version of an existing report inherits its lineage.
	if in.Supersedes != nil {
		prev, err := s.repo.GetByID(ctx, tenantID, *in.Supersedes)
		if err != nil {
			return nil, err
		}
		rep.Supersedes = &prev.ID
		rep.Version = prev.Version + 1
	}

	if err := s.repo.Create(ctx, rep); err != nil {
		return nil, err
	}
	return rep, nil
}

func formatList(fs []domain.ReportFormat) string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = string(f)
	}
	return strings.Join(out, ", ")
}

// Get returns one report with its comments and resolved actors.
func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (*domain.Report, error) {
	rep, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	comments, err := s.repo.ListComments(ctx, tenantID, id)
	if err == nil {
		rep.Comments = comments
	}
	s.resolveActors(ctx, rep)
	return rep, nil
}

// List returns a page, newest first by default.
func (s *Service) List(ctx context.Context, tenantID uuid.UUID, f domain.ReportFilter) ([]domain.Report, int64, error) {
	items, total, err := s.repo.List(ctx, tenantID, f)
	if err != nil {
		return nil, 0, err
	}
	for i := range items {
		s.resolveActors(ctx, &items[i])
	}
	return items, total, nil
}

// Download returns the stored bytes and re-verifies the integrity hash.
//
// Verified on the way out, not only at generation: the point of the hash is that
// someone can trust the file, and a report whose bytes no longer match what was
// recorded must not be served as if nothing happened.
func (s *Service) Download(ctx context.Context, tenantID, id uuid.UUID) (*domain.Report, error) {
	rep, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if rep.RunState != domain.ReportRunSucceeded || len(rep.Artifact) == 0 {
		return nil, domain.NewValidationError(fmt.Sprintf(
			"the document is not ready (state: %s)", rep.RunState))
	}
	if !rep.VerifyIntegrity() {
		return nil, domain.NewInternalError(
			"the stored document no longer matches its integrity hash and will not be served")
	}
	return rep, nil
}

// Verify recomputes the hash and reports the verdict, without downloading.
type VerifyResult struct {
	ReportID    uuid.UUID `json:"report_id"`
	ContentHash string    `json:"content_hash"`
	Recomputed  string    `json:"recomputed_hash"`
	Intact      bool      `json:"intact"`
	SizeBytes   int       `json:"size_bytes"`
	Message     string    `json:"message"`
}

func (s *Service) Verify(ctx context.Context, tenantID, id uuid.UUID) (*VerifyResult, error) {
	rep, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	res := &VerifyResult{
		ReportID:    rep.ID,
		ContentHash: rep.ContentHash,
		SizeBytes:   len(rep.Artifact),
	}
	if len(rep.Artifact) == 0 {
		res.Message = "no document to verify: this report has not been generated"
		return res, nil
	}
	res.Recomputed = domain.ComputeContentHash(rep.Artifact)
	res.Intact = res.Recomputed == rep.ContentHash
	if res.Intact {
		res.Message = "the stored document matches the hash recorded when it was generated"
	} else {
		res.Message = "the stored document does NOT match its recorded hash"
	}
	return res, nil
}

// TransitionInput moves a report through its lifecycle.
type TransitionInput struct {
	To      string
	Comment string
	Actor   uuid.UUID
}

// Transition applies a lifecycle move, recording who did it and what they said.
func (s *Service) Transition(ctx context.Context, tenantID, id uuid.UUID, in TransitionInput) (*domain.Report, error) {
	next, err := domain.ParseReportLifecycle(in.To)
	if err != nil {
		return nil, err
	}
	rep, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := rep.Lifecycle.CanTransitionTo(next); err != nil {
		return nil, err
	}
	// Approving a document that failed to render, or has not rendered yet, would
	// be approving nothing.
	if (next == domain.ReportLifecycleApproved || next == domain.ReportLifecyclePublished) &&
		rep.RunState != domain.ReportRunSucceeded {
		return nil, domain.NewValidationError(
			"this report has no document yet — it cannot be approved or published")
	}
	// Sending something back is a judgement, and the person who has to act on it
	// needs to know why.
	if next == domain.ReportLifecycleDraft && strings.TrimSpace(in.Comment) == "" {
		return nil, domain.NewValidationError(
			"say why you are sending it back: the author cannot act on a bare rejection")
	}

	now := s.now()
	rep.Lifecycle = next
	switch next {
	case domain.ReportLifecycleApproved:
		rep.ApprovedAt = &now
		if in.Actor != uuid.Nil {
			rep.ApprovedBy = &in.Actor
		}
	case domain.ReportLifecyclePublished:
		rep.PublishedAt = &now
	case domain.ReportLifecycleDraft:
		// Withdrawn: the previous approval no longer stands, and leaving the
		// approver's name on it would misstate who is answerable for the document
		// in its current state.
		rep.ApprovedAt, rep.ApprovedBy = nil, nil
	}

	if err := s.repo.Update(ctx, rep); err != nil {
		return nil, err
	}

	if strings.TrimSpace(in.Comment) != "" {
		_ = s.repo.AddComment(ctx, &domain.ReportComment{
			ID: uuid.New(), TenantID: tenantID, ReportID: rep.ID,
			AuthorID: in.Actor, Body: strings.TrimSpace(in.Comment), Transition: next,
		})
	}

	return s.Get(ctx, tenantID, id)
}

// Comment adds a remark without moving the lifecycle.
func (s *Service) Comment(ctx context.Context, tenantID, id, actor uuid.UUID, body string) (*domain.ReportComment, error) {
	if strings.TrimSpace(body) == "" {
		return nil, domain.NewValidationError("a comment needs something in it")
	}
	if _, err := s.repo.GetByID(ctx, tenantID, id); err != nil {
		return nil, err
	}
	c := &domain.ReportComment{
		ID: uuid.New(), TenantID: tenantID, ReportID: id,
		AuthorID: actor, Body: strings.TrimSpace(body),
	}
	if err := s.repo.AddComment(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Delete removes a report. Published ones are refused.
func (s *Service) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	rep, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if !rep.Lifecycle.Editable() {
		return domain.NewValidationError(
			"a published report cannot be deleted — people already hold it; withdraw it by publishing a new version")
	}
	return s.repo.Delete(ctx, tenantID, id)
}

// Versions returns the lineage, newest first.
func (s *Service) Versions(ctx context.Context, tenantID, id uuid.UUID) ([]domain.Report, error) {
	chain, err := s.repo.Lineage(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	for i := range chain {
		s.resolveActors(ctx, &chain[i])
	}
	return chain, nil
}

// VersionDiff is what changed between two versions of a report.
//
// Deliberately a comparison of the REQUEST and the RESULT, not of the rendered
// bytes. Two PDFs differ in every byte the moment a date changes, so a byte diff
// answers nothing; what a reviewer asks is "same scope? same period? did the
// figures move?".
type VersionDiff struct {
	From ReportSummary `json:"from"`
	To   ReportSummary `json:"to"`
	// Changes lists the differences in plain language.
	Changes []string `json:"changes"`
	// SameDocument is true when the two artifacts are byte-identical — which
	// means regenerating produced exactly the same document, and the underlying
	// data has not moved.
	SameDocument bool `json:"same_document"`
}

// ReportSummary is the comparable face of a report.
type ReportSummary struct {
	ID          uuid.UUID              `json:"id"`
	Version     int                    `json:"version"`
	CreatedAt   time.Time              `json:"created_at"`
	Format      domain.ReportFormat    `json:"format"`
	Locale      domain.ReportLocale    `json:"locale"`
	Template    string                 `json:"template"`
	Lifecycle   domain.ReportLifecycle `json:"lifecycle"`
	ContentHash string                 `json:"content_hash"`
	SizeBytes   int                    `json:"size_bytes"`
}

// CompareVersions diffs two reports of the same lineage.
func (s *Service) CompareVersions(ctx context.Context, tenantID, aID, bID uuid.UUID) (*VersionDiff, error) {
	a, err := s.repo.GetByID(ctx, tenantID, aID)
	if err != nil {
		return nil, err
	}
	b, err := s.repo.GetByID(ctx, tenantID, bID)
	if err != nil {
		return nil, err
	}
	if a.Type != b.Type {
		return nil, domain.NewValidationError("these are different kinds of report; there is nothing to compare")
	}

	diff := &VersionDiff{
		From:         summarise(a),
		To:           summarise(b),
		SameDocument: a.ContentHash != "" && a.ContentHash == b.ContentHash,
	}

	if a.Locale != b.Locale {
		diff.Changes = append(diff.Changes, fmt.Sprintf("language: %s → %s", a.Locale, b.Locale))
	}
	if a.Format != b.Format {
		diff.Changes = append(diff.Changes, fmt.Sprintf("format: %s → %s", a.Format, b.Format))
	}
	if a.TemplateVersion != b.TemplateVersion {
		diff.Changes = append(diff.Changes, fmt.Sprintf(
			"template: %s v%s → v%s (the layout changed between these two documents)",
			a.TemplateKey, a.TemplateVersion, b.TemplateVersion))
	}
	if a.Lifecycle != b.Lifecycle {
		diff.Changes = append(diff.Changes, fmt.Sprintf("state: %s → %s", a.Lifecycle, b.Lifecycle))
	}
	if !diff.SameDocument {
		diff.Changes = append(diff.Changes, fmt.Sprintf(
			"the document itself changed (%s → %s)", shortOr(a.ContentHash), shortOr(b.ContentHash)))
	} else {
		diff.Changes = append(diff.Changes,
			"the two documents are byte-identical: regenerating produced the same result")
	}

	// Parameter-level differences: the scope and period questions.
	for _, line := range diffParams(a.Params, b.Params) {
		diff.Changes = append(diff.Changes, line)
	}
	return diff, nil
}

func summarise(r *domain.Report) ReportSummary {
	return ReportSummary{
		ID: r.ID, Version: r.Version, CreatedAt: r.CreatedAt,
		Format: r.Format, Locale: r.Locale,
		Template:  fmt.Sprintf("%s v%s", r.TemplateKey, r.TemplateVersion),
		Lifecycle: r.Lifecycle, ContentHash: r.ContentHash, SizeBytes: r.SizeBytes,
	}
}

func shortOr(hash string) string {
	if hash == "" {
		return "—"
	}
	if len(hash) > 16 {
		return hash[:16]
	}
	return hash
}

func diffParams(a, b datatypes.JSON) []string {
	var pa, pb map[string]any
	_ = json.Unmarshal(a, &pa)
	_ = json.Unmarshal(b, &pb)

	keys := map[string]bool{}
	for k := range pa {
		keys[k] = true
	}
	for k := range pb {
		keys[k] = true
	}

	// Sorted so a diff reads the same way twice.
	ordered := make([]string, 0, len(keys))
	for k := range keys {
		ordered = append(ordered, k)
	}
	for i := 1; i < len(ordered); i++ {
		for j := i; j > 0 && ordered[j] < ordered[j-1]; j-- {
			ordered[j], ordered[j-1] = ordered[j-1], ordered[j]
		}
	}

	out := []string{}
	for _, k := range ordered {
		va, vb := fmt.Sprintf("%v", pa[k]), fmt.Sprintf("%v", pb[k])
		if va != vb {
			out = append(out, fmt.Sprintf("%s: %s → %s", k, orDash(va), orDash(vb)))
		}
	}
	return out
}

func orDash(s string) string {
	if s == "" || s == "<nil>" {
		return "—"
	}
	return s
}

// resolveActors fills the display emails. Best-effort: being unable to name
// someone is not a reason to refuse to show the report.
func (s *Service) resolveActors(ctx context.Context, r *domain.Report) {
	if s.users == nil {
		return
	}
	ids := []uuid.UUID{}
	if r.RequestedBy != uuid.Nil {
		ids = append(ids, r.RequestedBy)
	}
	if r.ApprovedBy != nil {
		ids = append(ids, *r.ApprovedBy)
	}
	for _, c := range r.Comments {
		if c.AuthorID != uuid.Nil {
			ids = append(ids, c.AuthorID)
		}
	}
	if len(ids) == 0 {
		return
	}
	emails, err := s.users.EmailsByIDs(ctx, ids)
	if err != nil {
		return
	}
	r.RequestedByEmail = emails[r.RequestedBy]
	if r.ApprovedBy != nil {
		r.ApprovedByEmail = emails[*r.ApprovedBy]
	}
	for i := range r.Comments {
		r.Comments[i].AuthorEmail = emails[r.Comments[i].AuthorID]
	}
}
