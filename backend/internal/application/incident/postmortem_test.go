// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package incident

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

type fakeRepo struct {
	items map[uint]*domain.IncidentPostMortem
}

func newFakeRepo() *fakeRepo { return &fakeRepo{items: map[uint]*domain.IncidentPostMortem{}} }

func (r *fakeRepo) Get(_ context.Context, tenantID uuid.UUID, incidentID uint) (*domain.IncidentPostMortem, error) {
	pm, ok := r.items[incidentID]
	if !ok || pm.TenantID != tenantID {
		return nil, nil
	}
	clone := *pm
	return &clone, nil
}
func (r *fakeRepo) Upsert(_ context.Context, p *domain.IncidentPostMortem) error {
	clone := *p
	r.items[p.IncidentID] = &clone
	return nil
}
func (r *fakeRepo) ListByTenant(_ context.Context, tenantID uuid.UUID, status string, limit int) ([]domain.IncidentPostMortem, error) {
	var out []domain.IncidentPostMortem
	for _, p := range r.items {
		if p.TenantID == tenantID && (status == "" || p.Status == status) {
			out = append(out, *p)
		}
	}
	return out, nil
}

type fakeIncidents struct{ inc *domain.Incident }

func (f fakeIncidents) GetIncident(tenantID string, id uint) (*domain.Incident, error) {
	if f.inc == nil || f.inc.TenantID != tenantID || f.inc.ID != id {
		return nil, errors.New("not found")
	}
	return f.inc, nil
}

type fakeMitigations struct {
	created []CorrectiveActionPlan
	fail    bool
}

func (f *fakeMitigations) CreateFromCorrectiveAction(_ context.Context, in CorrectiveActionPlan) (string, error) {
	if f.fail {
		return "", errors.New("the mitigation store refused the plan")
	}
	f.created = append(f.created, in)
	return uuid.NewString(), nil
}

func fixture(t *testing.T, severity string, riskIDs ...string) (*PostMortemService, uuid.UUID, *domain.Incident, *fakeMitigations) {
	t.Helper()
	tenant := uuid.New()
	inc := &domain.Incident{
		ID: 42, TenantID: tenant.String(), Title: "Payment gateway outage",
		Severity: severity, Status: "open", Origin: domain.OriginManual,
		RiskIDs:   domain.StringList(riskIDs),
		CreatedAt: time.Now().Add(-3 * time.Hour),
	}
	mits := &fakeMitigations{}
	svc := NewPostMortemService(newFakeRepo(), fakeIncidents{inc: inc}).WithMitigations(mits)
	return svc, tenant, inc, mits
}

func completeInput() PostMortemInput {
	return PostMortemInput{
		Summary:   "The payment gateway was unreachable for 42 minutes.",
		RootCause: "A certificate renewal job had been disabled during a migration and nobody owned it.",
		Impact:    "Roughly 1 200 transactions failed; no data was lost.",
		Timeline: []domain.PostMortemTimelineEntry{
			{At: time.Now().Add(-3 * time.Hour), Title: "First failed transaction", Kind: "detection"},
			{At: time.Now().Add(-2 * time.Hour), Title: "Certificate replaced", Kind: "mitigation"},
		},
		CorrectiveActions: []domain.CorrectiveAction{
			{Title: "Give certificate renewal a named owner", Priority: "high"},
			{Title: "Alert 30 days before any certificate expiry", Priority: "medium"},
		},
	}
}

func TestPostMortem_GetSeedsTheTimelineFromWhatIsKnown(t *testing.T) {
	svc, tenant, _, _ := fixture(t, "critical")

	view, err := svc.Get(context.Background(), tenant, 42)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(view.PostMortem.Timeline) == 0 {
		t.Fatal("a new review should start from what the platform already recorded, not a blank page")
	}
	if !view.Required {
		t.Fatal("a critical incident requires a review")
	}
	if view.BlocksClosure == "" {
		t.Fatal("the view must say why the incident cannot be closed yet")
	}
}

func TestPostMortem_PublicationRequiresACompleteReview(t *testing.T) {
	svc, tenant, _, _ := fixture(t, "critical", uuid.NewString())

	// Empty review → the checklist names every missing field.
	view, err := svc.Save(context.Background(), tenant, 42, uuid.New(), PostMortemInput{})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	for _, want := range []string{"summary", "root_cause", "impact", "timeline", "corrective_actions"} {
		if !contains(view.Missing, want) {
			t.Errorf("the checklist should name %q as missing, got %v", want, view.Missing)
		}
	}
	if _, err := svc.Publish(context.Background(), tenant, 42, uuid.New()); err == nil {
		t.Fatal("an incomplete review must not be publishable")
	}

	// A review with a story but no decisions is not a review.
	partial := completeInput()
	partial.CorrectiveActions = nil
	view, _ = svc.Save(context.Background(), tenant, 42, uuid.New(), partial)
	if !contains(view.Missing, "corrective_actions") {
		t.Fatal("a review with no corrective action must not be publishable")
	}

	view, _ = svc.Save(context.Background(), tenant, 42, uuid.New(), completeInput())
	if len(view.Missing) != 0 {
		t.Fatalf("a complete review should have nothing missing, got %v", view.Missing)
	}
}

func TestPostMortem_PublishTurnsCorrectiveActionsIntoMitigations(t *testing.T) {
	riskID := uuid.NewString()
	svc, tenant, _, mits := fixture(t, "critical", riskID)
	actor := uuid.New()

	if _, err := svc.Save(context.Background(), tenant, 42, actor, completeInput()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	res, err := svc.Publish(context.Background(), tenant, 42, actor)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if res.MitigationsCreated != 2 {
		t.Fatalf("both corrective actions should become tracked plans, got %d", res.MitigationsCreated)
	}
	if len(mits.created) != 2 {
		t.Fatalf("the mitigation store should have received 2 plans, got %d", len(mits.created))
	}
	for _, plan := range mits.created {
		if plan.RiskID.String() != riskID {
			t.Errorf("a plan must hang off the incident's linked risk, got %s", plan.RiskID)
		}
		if !strings.Contains(plan.Description, "INC-42") {
			t.Errorf("a plan should say which incident it came from, got %q", plan.Description)
		}
	}
	for _, a := range res.View.PostMortem.CorrectiveActions {
		if a.Status != domain.CorrectiveActionConverted || a.MitigationID == "" {
			t.Errorf("a converted action must record its plan: %+v", a)
		}
	}
	if res.View.PostMortem.Status != domain.PostMortemPublished || res.View.PostMortem.PublishedAt == nil {
		t.Fatal("publishing must freeze the review")
	}
	if res.View.BlocksClosure != "" {
		t.Fatal("a published review must stop blocking closure")
	}
}

func TestPostMortem_PublishReportsWhatItCouldNotConvert(t *testing.T) {
	// No linked risk: the actions have nothing to hang off.
	svc, tenant, _, mits := fixture(t, "critical")
	actor := uuid.New()
	_, _ = svc.Save(context.Background(), tenant, 42, actor, completeInput())

	res, err := svc.Publish(context.Background(), tenant, 42, actor)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if res.MitigationsCreated != 0 || len(mits.created) != 0 {
		t.Fatal("with no linked risk there is nothing to attach a plan to")
	}
	if len(res.NotConverted) != 2 {
		t.Fatalf("every un-converted action must be reported, got %v", res.NotConverted)
	}
	if !strings.Contains(res.NotConverted[0], "not linked to a risk") {
		t.Fatalf("the reason must be actionable, got %q", res.NotConverted[0])
	}
	// The decisions are still recorded — losing them would be worse than not
	// tracking them yet.
	if len(res.View.PostMortem.CorrectiveActions) != 2 {
		t.Fatal("un-converted actions must survive publication")
	}
	if res.View.PostMortem.Status != domain.PostMortemPublished {
		t.Fatal("publication proceeds even when conversion could not")
	}
}

func TestPostMortem_PublishedReviewIsImmutable(t *testing.T) {
	svc, tenant, _, _ := fixture(t, "critical", uuid.NewString())
	actor := uuid.New()
	_, _ = svc.Save(context.Background(), tenant, 42, actor, completeInput())
	if _, err := svc.Publish(context.Background(), tenant, 42, actor); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if _, err := svc.Save(context.Background(), tenant, 42, actor, completeInput()); err == nil {
		t.Fatal("a published review is the record of what was concluded; it must not be editable")
	}
	if _, err := svc.Publish(context.Background(), tenant, 42, actor); err == nil {
		t.Fatal("publishing twice must be refused")
	}
}

func TestPostMortem_OnlyCriticalIncidentsRequireOne(t *testing.T) {
	for _, tc := range []struct {
		severity string
		required bool
	}{
		{"critical", true},
		{"high", false},
		{"medium", false},
		{"low", false},
	} {
		if got := domain.RequiresPostMortem(tc.severity); got != tc.required {
			t.Errorf("%s: expected required=%v, got %v", tc.severity, tc.required, got)
		}
		svc, tenant, _, _ := fixture(t, tc.severity)
		view, err := svc.Get(context.Background(), tenant, 42)
		if err != nil {
			t.Fatalf("%s: Get: %v", tc.severity, err)
		}
		if view.Required != tc.required {
			t.Errorf("%s: the view should mirror the rule", tc.severity)
		}
		if !tc.required && view.BlocksClosure != "" {
			t.Errorf("%s: a non-critical incident must not be blocked from closing", tc.severity)
		}
	}
}

func TestPostMortem_TenantIsolation(t *testing.T) {
	svc, tenant, _, _ := fixture(t, "critical", uuid.NewString())
	_, _ = svc.Save(context.Background(), tenant, 42, uuid.New(), completeInput())

	// Another tenant asking for the same (sequential, guessable) incident id must
	// find nothing — the incident lookup is tenant-scoped before the review is.
	if _, err := svc.Get(context.Background(), uuid.New(), 42); err == nil {
		t.Fatal("another tenant must not be able to read this review by guessing the incident id")
	}
}

func TestIncidentOrigins_CatalogueMatchesTheCode(t *testing.T) {
	origins := domain.IncidentOrigins()
	if len(origins) == 0 {
		t.Fatal("the help page needs a catalogue")
	}
	seen := map[string]bool{}
	automatic := 0
	for _, o := range origins {
		if seen[o.Key] {
			t.Errorf("duplicate origin %s", o.Key)
		}
		seen[o.Key] = true
		if o.Label == "" || o.Description == "" {
			t.Errorf("%s: an origin must explain itself in one sentence", o.Key)
		}
		if o.Automatic != domain.IsAutomaticOrigin(o.Key) {
			t.Errorf("%s: the catalogue disagrees with IsAutomaticOrigin", o.Key)
		}
		if o.Automatic {
			automatic++
			if o.WhereToConfigure == "" {
				t.Errorf("%s: an automatic source must say where it is configured, so 'why did I get this?' ends at a setting", o.Key)
			}
		}
	}
	if !seen[domain.OriginManual] || automatic == 0 {
		t.Fatal("the catalogue must cover both the manual and the automatic paths")
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
