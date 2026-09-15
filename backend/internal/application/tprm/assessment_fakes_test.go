// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
)

// assessmentStore is an in-memory TemplateStore and AssessmentStore that
// enforces the tenant and the open-status conditions exactly where the GORM
// repository does, so a use case that forgets either gets the answer the
// database would give it.
type assessmentStore struct {
	templates   map[uuid.UUID]domain.VendorQuestionnaireTemplate
	assessments map[uuid.UUID]domain.VendorAssessment
	tokens      []domain.VendorAssessmentToken
}

func newAssessmentStore() *assessmentStore {
	return &assessmentStore{
		templates:   map[uuid.UUID]domain.VendorQuestionnaireTemplate{},
		assessments: map[uuid.UUID]domain.VendorAssessment{},
	}
}

func copyAssessment(a domain.VendorAssessment) domain.VendorAssessment {
	a.Items = append([]domain.VendorAssessmentItem(nil), a.Items...)
	prov := domain.JSONMap{}
	for k, v := range a.Provenance {
		prov[k] = v
	}
	a.Provenance = prov
	return a
}

func isOpen(a domain.VendorAssessment) bool {
	return a.Status == domain.VendorAssessmentSent || a.Status == domain.VendorAssessmentInProgress
}

func (s *assessmentStore) CreateTemplate(_ context.Context, t *domain.VendorQuestionnaireTemplate) error {
	c := *t
	c.Questions = append([]domain.VendorQuestionnaireQuestion(nil), t.Questions...)
	s.templates[t.ID] = c
	return nil
}

func (s *assessmentStore) GetTemplate(_ context.Context, id, tenantID uuid.UUID) (*domain.VendorQuestionnaireTemplate, error) {
	t, ok := s.templates[id]
	if !ok || t.TenantID != tenantID {
		return nil, nil
	}
	t.Questions = append([]domain.VendorQuestionnaireQuestion(nil), t.Questions...)
	return &t, nil
}

func (s *assessmentStore) ListTemplates(_ context.Context, tenantID uuid.UUID, includeArchived bool) ([]domain.VendorQuestionnaireTemplate, error) {
	out := []domain.VendorQuestionnaireTemplate{}
	for _, t := range s.templates {
		if t.TenantID != tenantID || (!includeArchived && t.ArchivedAt != nil) {
			continue
		}
		t.Questions = nil
		out = append(out, t)
	}
	return out, nil
}

func (s *assessmentStore) ReplaceTemplate(_ context.Context, t *domain.VendorQuestionnaireTemplate) (bool, error) {
	existing, ok := s.templates[t.ID]
	if !ok || existing.TenantID != t.TenantID || existing.ArchivedAt != nil {
		return false, nil
	}
	c := *t
	c.Questions = append([]domain.VendorQuestionnaireQuestion(nil), t.Questions...)
	s.templates[t.ID] = c
	return true, nil
}

func (s *assessmentStore) ArchiveTemplate(_ context.Context, id, tenantID uuid.UUID, at time.Time) (bool, error) {
	t, ok := s.templates[id]
	if !ok || t.TenantID != tenantID || t.ArchivedAt != nil {
		return false, nil
	}
	t.ArchivedAt = &at
	s.templates[id] = t
	return true, nil
}

func (s *assessmentStore) CreateAssessment(_ context.Context, a *domain.VendorAssessment, tok *domain.VendorAssessmentToken) error {
	if tok.TenantID != a.TenantID {
		return errors.New("token tenant mismatch")
	}
	s.assessments[a.ID] = copyAssessment(*a)
	s.tokens = append(s.tokens, *tok)
	return nil
}

func (s *assessmentStore) GetAssessment(_ context.Context, id, tenantID uuid.UUID) (*domain.VendorAssessment, error) {
	a, ok := s.assessments[id]
	if !ok || a.TenantID != tenantID {
		return nil, nil
	}
	c := copyAssessment(a)
	return &c, nil
}

func (s *assessmentStore) ListAssessmentsByVendor(_ context.Context, tenantID, vendorID uuid.UUID) ([]domain.VendorAssessment, error) {
	out := []domain.VendorAssessment{}
	for _, a := range s.assessments {
		if a.TenantID == tenantID && a.VendorAssetID == vendorID {
			a.Items = nil
			out = append(out, a)
		}
	}
	return out, nil
}

func (s *assessmentStore) RevokeAssessment(_ context.Context, id, tenantID, by uuid.UUID, at time.Time) (bool, error) {
	a, ok := s.assessments[id]
	if !ok || a.TenantID != tenantID || !isOpen(a) {
		return false, nil
	}
	a.Status = domain.VendorAssessmentRevoked
	a.RevokedAt = &at
	a.RevokedBy = &by
	s.assessments[id] = a
	return true, nil
}

func (s *assessmentStore) IssueToken(_ context.Context, tok *domain.VendorAssessmentToken) (bool, error) {
	a, ok := s.assessments[tok.AssessmentID]
	if !ok || a.TenantID != tok.TenantID || !isOpen(a) {
		return false, nil
	}
	for i := range s.tokens {
		if s.tokens[i].AssessmentID == tok.AssessmentID && s.tokens[i].SupersededAt == nil {
			at := tok.IssuedAt
			s.tokens[i].SupersededAt = &at
		}
	}
	s.tokens = append(s.tokens, *tok)
	return true, nil
}

func (s *assessmentStore) FindTokenByHash(_ context.Context, hash string) (*domain.VendorAssessmentToken, error) {
	for _, t := range s.tokens {
		if t.TokenHash == hash {
			t := t
			return &t, nil
		}
	}
	return nil, nil
}

func (s *assessmentStore) applyItems(a *domain.VendorAssessment, items []domain.VendorAssessmentItem) {
	for _, in := range items {
		for i := range a.Items {
			if a.Items[i].ID == in.ID {
				a.Items[i].AnswerValue = in.AnswerValue
				a.Items[i].AnswerNA = in.AnswerNA
				a.Items[i].AnswerComment = in.AnswerComment
				a.Items[i].AnsweredAt = in.AnsweredAt
			}
		}
	}
}

func (s *assessmentStore) SaveAnswers(_ context.Context, tenantID, assessmentID uuid.UUID, items []domain.VendorAssessmentItem, at time.Time) (bool, error) {
	a, ok := s.assessments[assessmentID]
	if !ok || a.TenantID != tenantID || !isOpen(a) {
		return false, nil
	}
	a = copyAssessment(a)
	s.applyItems(&a, items)
	a.Status = domain.VendorAssessmentInProgress
	a.UpdatedAt = at
	s.assessments[assessmentID] = a
	return true, nil
}

func (s *assessmentStore) SubmitAssessment(_ context.Context, tenantID, assessmentID uuid.UUID, items []domain.VendorAssessmentItem, provenance domain.JSONMap, at time.Time) (bool, error) {
	a, ok := s.assessments[assessmentID]
	if !ok || a.TenantID != tenantID || !isOpen(a) {
		return false, nil
	}
	a = copyAssessment(a)
	s.applyItems(&a, items)
	a.Status = domain.VendorAssessmentSubmitted
	a.SubmittedAt = &at
	a.ObservedAt = &at
	a.Provenance = provenance
	s.assessments[assessmentID] = a
	return true, nil
}

// --- the other ports ----------------------------------------------------------

type featureGate struct{ denied map[uuid.UUID]bool }

func (f *featureGate) Allowed(_ context.Context, tenant uuid.UUID, feature ent.Feature) (bool, ent.Plan, ent.Plan, error) {
	if feature != ent.FeatVendorRisk {
		return false, ent.PlanFree, ent.PlanBusiness, errors.New("unexpected feature")
	}
	return !f.denied[tenant], ent.PlanBusiness, ent.PlanBusiness, nil
}

type auditLog struct{ events []domain.AuditEvent }

func (l *auditLog) Record(_ context.Context, ev domain.AuditEvent) { l.events = append(l.events, ev) }

func (l *auditLog) ofType(entity string) []domain.AuditEvent {
	var out []domain.AuditEvent
	for _, e := range l.events {
		if e.EntityType == entity {
			out = append(out, e)
		}
	}
	return out
}

type mailbox struct {
	sent []QuestionnaireMail
	fail bool
}

func (m *mailbox) SendQuestionnaire(_ context.Context, mail QuestionnaireMail) error {
	if m.fail {
		return errors.New("smtp: connection refused")
	}
	m.sent = append(m.sent, mail)
	return nil
}

type countingThrottle struct{ counts map[string]int }

func (c *countingThrottle) IsAllowed(key string, maxRequests int, _ time.Duration) bool {
	c.counts[key]++
	return c.counts[key] <= maxRequests
}

type orgDirectory map[uuid.UUID]string

func (o orgDirectory) GetByID(_ context.Context, id uuid.UUID) (*domain.Organization, error) {
	name, ok := o[id]
	if !ok {
		return nil, nil
	}
	return &domain.Organization{ID: id, Name: name}, nil
}

// --- the harness ----------------------------------------------------------------

type harness struct {
	vendors  *store
	as       *assessmentStore
	audit    *auditLog
	mail     *mailbox
	features *featureGate
	throttle *countingThrottle
	orgs     orgDirectory
	now      time.Time
}

func newHarness() *harness {
	return &harness{
		vendors:  newStore(),
		as:       newAssessmentStore(),
		audit:    &auditLog{},
		mail:     &mailbox{},
		features: &featureGate{denied: map[uuid.UUID]bool{}},
		throttle: &countingThrottle{counts: map[string]int{}},
		orgs:     orgDirectory{},
		now:      time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC),
	}
}

func (h *harness) deps() AssessmentDeps {
	return AssessmentDeps{
		Vendors:     h.vendors,
		Templates:   h.as,
		Assessments: h.as,
		Features:    h.features,
		Orgs:        h.orgs,
		Mailer:      h.mail,
		Audit:       h.audit,
		Throttle:    h.throttle,
		BaseURL:     "https://app.openrisk.example/",
		Now:         func() time.Time { return h.now },
	}
}

// depsWithoutMail is the same wiring with no transport, which hands the link
// back to the sender — the easiest way for a test to hold a real token.
func (h *harness) depsWithoutMail() AssessmentDeps {
	d := h.deps()
	d.Mailer = nil
	return d
}

func baselineInput() TemplateInput {
	return TemplateInput{
		Name:     "Baseline sécurité",
		Language: "fr",
		Questions: []QuestionInput{
			{Text: "Imposez-vous l'authentification multifacteur ?", AnswerType: domain.VendorAnswerChoice, Weight: 5, Required: true,
				Options: domain.VendorQuestionOptions{{Value: "yes", Label: "Oui", Points: 1}, {Value: "no", Label: "Non", Points: 0}}},
			{Text: "Décrivez votre plan de continuité", AnswerType: domain.VendorAnswerText},
		},
	}
}

// fixture is one tenant with a vendor, a template and a sent questionnaire.
type fixture struct {
	tenant   uuid.UUID
	actor    uuid.UUID
	vendor   domain.Asset
	template *domain.VendorQuestionnaireTemplate
	delivery *AssessmentDelivery
	token    string
}

func tokenFromURL(t *testing.T, url string) string {
	t.Helper()
	i := strings.LastIndex(url, "#")
	require.Greater(t, i, 0, "the token travels in the URL fragment")
	return url[i+1:]
}

func (h *harness) sendOne(t *testing.T) fixture {
	t.Helper()
	f := fixture{tenant: uuid.New(), actor: uuid.New()}
	h.orgs[f.tenant] = "Banque Exemple"
	f.vendor = h.vendors.addAsset(f.tenant, "Acme Cloud", domain.CategoryVendor,
		domain.AssetAttributes{"contact_email": "security@acme.example"})

	var err error
	f.template, err = NewCreateQuestionnaireTemplateUseCase(h.deps()).Execute(context.Background(), f.tenant, f.actor, baselineInput())
	require.NoError(t, err)

	f.delivery, err = NewSendVendorAssessmentUseCase(h.depsWithoutMail()).Execute(context.Background(), f.tenant, f.actor, f.vendor.ID,
		SendAssessmentInput{TemplateID: f.template.ID, DueAt: h.now.Add(14 * 24 * time.Hour)})
	require.NoError(t, err)
	f.token = tokenFromURL(t, f.delivery.QuestionnaireURL)
	return f
}
