// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package incident holds the use cases that make the incident module real:
// declaring one with everything it touches, telling the people who must know,
// and the structured review that turns an incident into work that gets done.
package incident

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// IncidentReader is the slice of the incident store this package needs. Narrow
// on purpose: the legacy IncidentService keeps owning incident CRUD, and this
// package does not inherit its surface.
type IncidentReader interface {
	GetIncident(tenantID string, incidentID uint) (*domain.Incident, error)
}

// MitigationCreator turns a corrective action into a real, tracked plan. This is
// the whole point of the corrective-actions field: decisions taken in a review
// have to leave the document, or the review was theatre.
type MitigationCreator interface {
	CreateFromCorrectiveAction(ctx context.Context, in CorrectiveActionPlan) (mitigationID string, err error)
}

// CorrectiveActionPlan is what a corrective action becomes.
type CorrectiveActionPlan struct {
	TenantID    uuid.UUID
	RiskID      uuid.UUID // the risk the plan hangs off
	IncidentID  uint
	Title       string
	Description string
	OwnerID     string
	DueDate     *time.Time
	Priority    string
	CreatedBy   uuid.UUID
}

// PostMortemInput is the editable body of a review.
type PostMortemInput struct {
	Summary             string
	RootCause           string
	ContributingFactors string
	Impact              string
	Detection           string
	WhatWentWell        string
	LessonsLearned      string
	Timeline            []domain.PostMortemTimelineEntry
	CorrectiveActions   []domain.CorrectiveAction
}

// PostMortemService owns the review lifecycle.
type PostMortemService struct {
	repo        domain.IncidentPostMortemRepository
	incidents   IncidentReader
	mitigations MitigationCreator
	lookup      UserLookup
}

// UserLookup resolves emails for display.
type UserLookup interface {
	EmailsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// NewPostMortemService builds the service. mitigations is optional: without it,
// publishing still works and each action is reported as un-converted rather than
// silently dropped.
func NewPostMortemService(repo domain.IncidentPostMortemRepository, incidents IncidentReader) *PostMortemService {
	return &PostMortemService{repo: repo, incidents: incidents}
}

func (s *PostMortemService) WithMitigations(m MitigationCreator) *PostMortemService {
	s.mitigations = m
	return s
}
func (s *PostMortemService) WithUserLookup(l UserLookup) *PostMortemService {
	s.lookup = l
	return s
}

// PostMortemView is a review plus what still stands between it and publication.
type PostMortemView struct {
	PostMortem *domain.IncidentPostMortem `json:"post_mortem"`
	// Missing is the checklist a reviewer works through. Empty means publishable.
	Missing []string `json:"missing"`
	// Required says whether this incident's severity makes the review mandatory
	// before closing.
	Required bool `json:"required"`
	// BlocksClosure is the sentence the incident screen shows when the review is
	// the reason the close button is disabled.
	BlocksClosure string `json:"blocks_closure,omitempty"`
}

// Get returns the review for an incident, creating an empty draft view when none
// exists yet — so the UI always has a shape to render rather than a 404 to
// special-case.
func (s *PostMortemService) Get(ctx context.Context, tenantID uuid.UUID, incidentID uint) (*PostMortemView, error) {
	inc, err := s.incidents.GetIncident(tenantID.String(), incidentID)
	if err != nil || inc == nil {
		return nil, domain.NewNotFoundError("incident", incidentID)
	}
	pm, err := s.repo.Get(ctx, tenantID, incidentID)
	if err != nil {
		return nil, err
	}
	if pm == nil {
		pm = &domain.IncidentPostMortem{
			TenantID:   tenantID,
			IncidentID: incidentID,
			Status:     domain.PostMortemDraft,
			Timeline:   domain.PostMortemTimeline{},
			// Seed the review with what the platform already knows, so a reviewer
			// starts from the record rather than from a blank page.
			CorrectiveActions: domain.CorrectiveActionList{},
		}
		pm.Timeline = seedTimeline(inc)
	}
	s.resolveAuthor(ctx, pm)
	return s.view(pm, inc), nil
}

// seedTimeline pre-fills the moments the platform can prove: when the incident
// was opened and when it was resolved. Everything else is authored.
func seedTimeline(inc *domain.Incident) domain.PostMortemTimeline {
	tl := domain.PostMortemTimeline{
		{At: inc.CreatedAt, Title: "Incident declared", Kind: "detection",
			Detail: "Opened as " + strings.ToUpper(inc.Severity) + " (" + originLabel(inc) + ")"},
	}
	if inc.ResolvedAt != nil {
		tl = append(tl, domain.PostMortemTimelineEntry{
			At: *inc.ResolvedAt, Title: "Incident resolved", Kind: "resolution", Detail: inc.Resolution,
		})
	}
	return tl
}

func originLabel(inc *domain.Incident) string {
	if o, ok := domain.FindIncidentOrigin(inc.Origin); ok {
		return o.Label
	}
	return "source inconnue"
}

// Save creates or updates the draft. A published review is immutable: it is the
// organisation's record of what it concluded, and editing it after the fact
// would make every earlier reading of it worthless.
func (s *PostMortemService) Save(ctx context.Context, tenantID uuid.UUID, incidentID uint, authorID uuid.UUID, in PostMortemInput) (*PostMortemView, error) {
	inc, err := s.incidents.GetIncident(tenantID.String(), incidentID)
	if err != nil || inc == nil {
		return nil, domain.NewNotFoundError("incident", incidentID)
	}
	pm, err := s.repo.Get(ctx, tenantID, incidentID)
	if err != nil {
		return nil, err
	}
	if pm == nil {
		pm = &domain.IncidentPostMortem{
			ID: uuid.New(), TenantID: tenantID, IncidentID: incidentID, Status: domain.PostMortemDraft,
		}
		if authorID != uuid.Nil {
			a := authorID
			pm.AuthorID = &a
		}
	}
	if pm.Status == domain.PostMortemPublished {
		return nil, domain.NewValidationError("this post-mortem is published and can no longer be edited — it is the record of what was concluded")
	}

	pm.Summary = strings.TrimSpace(in.Summary)
	pm.RootCause = strings.TrimSpace(in.RootCause)
	pm.ContributingFactors = strings.TrimSpace(in.ContributingFactors)
	pm.Impact = strings.TrimSpace(in.Impact)
	pm.Detection = strings.TrimSpace(in.Detection)
	pm.WhatWentWell = strings.TrimSpace(in.WhatWentWell)
	pm.LessonsLearned = strings.TrimSpace(in.LessonsLearned)
	pm.Timeline = domain.PostMortemTimeline(in.Timeline)
	pm.CorrectiveActions = normaliseActions(in.CorrectiveActions)

	if err := s.repo.Upsert(ctx, pm); err != nil {
		return nil, err
	}
	s.resolveAuthor(ctx, pm)
	return s.view(pm, inc), nil
}

// PublishResult reports what publication produced.
type PublishResult struct {
	View *PostMortemView `json:"view"`
	// MitigationsCreated is how many corrective actions became tracked plans.
	MitigationsCreated int `json:"mitigations_created"`
	// NotConverted explains, per action, why it did not become a plan — rather
	// than reporting a clean success over silently dropped work.
	NotConverted []string `json:"not_converted,omitempty"`
}

// Publish freezes the review and turns its corrective actions into mitigation
// plans. A corrective action needs a risk to hang off; when the incident is not
// linked to one, the action is kept and the reason is reported, because losing
// the decision would be worse than not tracking it yet.
func (s *PostMortemService) Publish(ctx context.Context, tenantID uuid.UUID, incidentID uint, actorID uuid.UUID) (*PublishResult, error) {
	inc, err := s.incidents.GetIncident(tenantID.String(), incidentID)
	if err != nil || inc == nil {
		return nil, domain.NewNotFoundError("incident", incidentID)
	}
	pm, err := s.repo.Get(ctx, tenantID, incidentID)
	if err != nil {
		return nil, err
	}
	if pm == nil {
		return nil, domain.NewValidationError("there is no post-mortem to publish — write one first")
	}
	if pm.Status == domain.PostMortemPublished {
		return nil, domain.NewValidationError("this post-mortem is already published")
	}
	if missing := pm.MissingForPublication(); len(missing) > 0 {
		return nil, domain.NewValidationError("the post-mortem is incomplete: " + strings.Join(missing, ", ") + " still to fill in")
	}

	result := &PublishResult{}
	riskID := firstRiskID(inc)

	for i := range pm.CorrectiveActions {
		a := &pm.CorrectiveActions[i]
		if a.Status == domain.CorrectiveActionConverted || a.MitigationID != "" {
			continue
		}
		if s.mitigations == nil {
			result.NotConverted = append(result.NotConverted,
				a.Title+": mitigation tracking is not available on this deployment")
			continue
		}
		if riskID == uuid.Nil {
			result.NotConverted = append(result.NotConverted,
				a.Title+": this incident is not linked to a risk, so there is nothing to attach the plan to — link a risk and publish again, or create the plan by hand")
			continue
		}
		id, err := s.mitigations.CreateFromCorrectiveAction(ctx, CorrectiveActionPlan{
			TenantID:    tenantID,
			RiskID:      riskID,
			IncidentID:  incidentID,
			Title:       a.Title,
			Description: buildActionDescription(a, inc),
			OwnerID:     a.OwnerID,
			DueDate:     a.DueDate,
			Priority:    a.Priority,
			CreatedBy:   actorID,
		})
		if err != nil {
			result.NotConverted = append(result.NotConverted, a.Title+": "+err.Error())
			continue
		}
		a.MitigationID = id
		a.RiskID = riskID.String()
		a.Status = domain.CorrectiveActionConverted
		result.MitigationsCreated++
	}

	now := time.Now().UTC()
	pm.Status = domain.PostMortemPublished
	pm.PublishedAt = &now
	if actorID != uuid.Nil {
		a := actorID
		pm.PublishedBy = &a
	}
	if err := s.repo.Upsert(ctx, pm); err != nil {
		return nil, err
	}
	s.resolveAuthor(ctx, pm)
	result.View = s.view(pm, inc)
	return result, nil
}

func buildActionDescription(a *domain.CorrectiveAction, inc *domain.Incident) string {
	var b strings.Builder
	if a.Description != "" {
		b.WriteString(a.Description)
		b.WriteString("\n\n")
	}
	b.WriteString("Corrective action from the post-mortem of incident INC-")
	b.WriteString(itoa(int(inc.ID)))
	b.WriteString(" (")
	b.WriteString(inc.Title)
	b.WriteString(").")
	return b.String()
}

func firstRiskID(inc *domain.Incident) uuid.UUID {
	for _, raw := range inc.RiskIDs {
		if id, err := uuid.Parse(strings.TrimSpace(raw)); err == nil && id != uuid.Nil {
			return id
		}
	}
	return uuid.Nil
}

func normaliseActions(in []domain.CorrectiveAction) domain.CorrectiveActionList {
	out := make(domain.CorrectiveActionList, 0, len(in))
	for _, a := range in {
		a.Title = strings.TrimSpace(a.Title)
		if a.Title == "" {
			continue // an action with no title is not an action
		}
		if a.ID == "" {
			a.ID = uuid.NewString()
		}
		if a.Status == "" {
			a.Status = domain.CorrectiveActionOpen
		}
		if a.Priority == "" {
			a.Priority = "medium"
		}
		out = append(out, a)
	}
	return out
}

func (s *PostMortemService) view(pm *domain.IncidentPostMortem, inc *domain.Incident) *PostMortemView {
	v := &PostMortemView{
		PostMortem: pm,
		Missing:    pm.MissingForPublication(),
		Required:   domain.RequiresPostMortem(inc.Severity),
	}
	if v.Required && pm.Status != domain.PostMortemPublished {
		v.BlocksClosure = "This incident is CRITICAL, so it cannot be closed until its post-mortem is published. " +
			"Remaining: " + strings.Join(v.Missing, ", ")
		if len(v.Missing) == 0 {
			v.BlocksClosure = "This incident is CRITICAL. The post-mortem is complete — publish it to allow closure."
		}
	}
	return v
}

func (s *PostMortemService) resolveAuthor(ctx context.Context, pm *domain.IncidentPostMortem) {
	if s.lookup == nil || pm.AuthorID == nil {
		return
	}
	if emails, err := s.lookup.EmailsByIDs(ctx, []uuid.UUID{*pm.AuthorID}); err == nil {
		pm.AuthorEmail = emails[*pm.AuthorID]
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		b[p] = '-'
	}
	return string(b[p:])
}
