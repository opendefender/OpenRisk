// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package activation

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// Step 4 of the tunnel (#643).
//
// The defect these guard: SaveStep's promotion switch had no case for `score`,
// so the sliders were stored by SetStepAnswers and read by nothing. A user who
// scored a risk at 6.30 found 2 in their register — the catalogue's default for
// the adopted statement.
// ---------------------------------------------------------------------------

type fakeScorer struct {
	target *domain.Risk

	// recorded call
	scoredTenant uuid.UUID
	scoredRisk   uuid.UUID
	probability  float64
	impact       float64
	calls        int

	findErr  error
	scoreErr error
}

func (f *fakeScorer) FirstStarterRisk(_ context.Context, tenantID uuid.UUID) (*domain.Risk, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("a tenant is required")
	}
	return f.target, nil
}

func (f *fakeScorer) ScoreStarterRisk(_ context.Context, tenantID, riskID uuid.UUID, probability, impact float64) (*domain.Risk, error) {
	f.calls++
	if f.scoreErr != nil {
		return nil, f.scoreErr
	}
	f.scoredTenant, f.scoredRisk = tenantID, riskID
	f.probability, f.impact = probability, impact
	return &domain.Risk{ID: riskID, Probability: probability, Impact: impact}, nil
}

func scoreFixture(t *testing.T) (*OnboardingUseCase, *fakeScorer, uuid.UUID, uuid.UUID) {
	t.Helper()
	repo := newFakeRepo()
	target := &domain.Risk{
		ID:          uuid.New(),
		Title:       "Indisponibilité de la plateforme de sinistres",
		Probability: 0.2,
		Impact:      10,
	}
	scorer := &fakeScorer{target: target}
	uc := NewOnboardingUseCase(repo, NewRecorder(repo)).WithStarterRiskScorer(scorer)
	return uc, scorer, uuid.New(), uuid.New()
}

func TestSaveScoreStep_Success_WritesTheSlidersOntoTheRisk(t *testing.T) {
	uc, scorer, tenantID, userID := scoreFixture(t)

	state, err := uc.SaveStep(context.Background(), tenantID, userID, SaveStepInput{
		Step:    domain.OnboardingStepScore,
		Answers: domain.JSONMap{"probability": 0.7, "impact": 9.0},
	})
	if err != nil {
		t.Fatalf("SaveStep: %v", err)
	}

	if scorer.calls != 1 {
		t.Fatalf("expected exactly one scoring call, got %d", scorer.calls)
	}
	if scorer.scoredTenant != tenantID {
		t.Errorf("scored under tenant %s, want %s", scorer.scoredTenant, tenantID)
	}
	if scorer.scoredRisk != scorer.target.ID {
		t.Errorf("scored risk %s, want the adopted starter risk %s", scorer.scoredRisk, scorer.target.ID)
	}
	if scorer.probability != 0.7 || scorer.impact != 9.0 {
		t.Errorf("applied %.2f/%.2f, want 0.70/9.00", scorer.probability, scorer.impact)
	}
	// 0.7 × 9 = 6.30 — the number the readout showed, which is the whole point.
	if got := scorer.probability * scorer.impact; got != 6.3 {
		t.Errorf("resulting score %.2f, want 6.30", got)
	}
	if state == nil {
		t.Fatal("expected a wizard state back")
	}
}

func TestSaveScoreStep_NotFound_NoAdoptedRiskIsNotAnError(t *testing.T) {
	uc, scorer, tenantID, userID := scoreFixture(t)
	// Adoption sits on step 2 and is skippable: a tenant can legitimately reach
	// step 4 with nothing to score. That must not fail the step and strand the
	// user on screen 4 of 5.
	scorer.target = nil

	state, err := uc.SaveStep(context.Background(), tenantID, userID, SaveStepInput{
		Step:    domain.OnboardingStepScore,
		Answers: domain.JSONMap{"probability": 0.5, "impact": 5.0},
	})
	if err != nil {
		t.Fatalf("a tenant with no adopted risk must still pass step 4: %v", err)
	}
	if state == nil {
		t.Fatal("expected a wizard state back")
	}
	if scorer.calls != 0 {
		t.Errorf("nothing to score, yet %d scoring calls were made", scorer.calls)
	}
	if state.ScoreTarget != nil {
		t.Errorf("no adopted risk, yet a score target was reported: %+v", state.ScoreTarget)
	}
}

func TestSaveScoreStep_Unauthorized_RefusesWithoutATenant(t *testing.T) {
	uc, scorer, _, userID := scoreFixture(t)

	_, err := uc.SaveStep(context.Background(), uuid.Nil, userID, SaveStepInput{
		Step:    domain.OnboardingStepScore,
		Answers: domain.JSONMap{"probability": 0.7, "impact": 9.0},
	})
	if err == nil {
		t.Fatal("a save with no tenant must be refused")
	}
	if scorer.calls != 0 {
		t.Errorf("refused save still wrote: %d scoring calls", scorer.calls)
	}
}

func TestSaveScoreStep_ScoringFailureDoesNotBlockTheTunnel(t *testing.T) {
	uc, scorer, tenantID, userID := scoreFixture(t)
	scorer.scoreErr = errors.New("risk store down")

	// The answers are persisted before this runs, so a transient write failure
	// costs a Back-and-Continue — not the user's place in a five-step tunnel.
	state, err := uc.SaveStep(context.Background(), tenantID, userID, SaveStepInput{
		Step:    domain.OnboardingStepScore,
		Answers: domain.JSONMap{"probability": 0.7, "impact": 9.0},
	})
	if err != nil {
		t.Fatalf("a scoring failure must not fail the step: %v", err)
	}
	if state == nil {
		t.Fatal("expected a wizard state back")
	}
}

func TestSaveScoreStep_IgnoresAPayloadWithoutBothSliders(t *testing.T) {
	uc, scorer, tenantID, userID := scoreFixture(t)

	// A partial payload must leave the stored value alone rather than reset the
	// missing axis to zero — which would read as "no impact at all".
	if _, err := uc.SaveStep(context.Background(), tenantID, userID, SaveStepInput{
		Step:    domain.OnboardingStepScore,
		Answers: domain.JSONMap{"probability": 0.7},
	}); err != nil {
		t.Fatalf("SaveStep: %v", err)
	}
	if scorer.calls != 0 {
		t.Errorf("a half payload was applied: %d scoring calls", scorer.calls)
	}
}

func TestSaveScoreStep_AcceptsNumbersThatRoundTrippedThroughJSON(t *testing.T) {
	uc, scorer, tenantID, userID := scoreFixture(t)

	// A stored answer read back out of the JSONMap column can come back as a
	// string; refusing it would silently stop persisting the score on a resume.
	if _, err := uc.SaveStep(context.Background(), tenantID, userID, SaveStepInput{
		Step:    domain.OnboardingStepScore,
		Answers: domain.JSONMap{"probability": "0.7", "impact": "9"},
	}); err != nil {
		t.Fatalf("SaveStep: %v", err)
	}
	if scorer.calls != 1 {
		t.Fatalf("expected the string form to be applied, got %d calls", scorer.calls)
	}
	if scorer.probability != 0.7 || scorer.impact != 9 {
		t.Errorf("applied %.2f/%.2f, want 0.70/9.00", scorer.probability, scorer.impact)
	}
}

func TestOnboardingState_NamesTheRiskStepFourScores(t *testing.T) {
	uc, scorer, tenantID, userID := scoreFixture(t)

	state, err := uc.GetState(context.Background(), tenantID, userID)
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if state.ScoreTarget == nil {
		t.Fatal("step 4 has no subject to name — the screen would say 'this risk' about nothing")
	}
	if state.ScoreTarget.Title != scorer.target.Title {
		t.Errorf("named %q, want %q", state.ScoreTarget.Title, scorer.target.Title)
	}
	// The sliders open on the risk's CURRENT values, not on a hard-coded 0.5/6.
	if state.ScoreTarget.Probability != 0.2 || state.ScoreTarget.Impact != 10 {
		t.Errorf("target carries %.2f/%.2f, want the risk's own 0.20/10.00",
			state.ScoreTarget.Probability, state.ScoreTarget.Impact)
	}
}
