// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package activation

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/onboarding"
)

// ---------------------------------------------------------------------------
// The five-step signup wizard (spec §4).
//
// Every step is savable, resumable and reversible: the client PUTs one step at a
// time, the server stores the raw answers per step, and back-navigation is a
// plain sequence move — there is no forbidden transition, because a wizard that
// will not let you go back to fix a typo is a wizard people abandon.
// ---------------------------------------------------------------------------

// OrgUpdater applies the organization step to the real Organization row. Optional
// (nil-safe): if it is absent the answers are still stored, so the wizard works
// even where the caller has no organization write path.
type OrgUpdater interface {
	UpdateOrganizationProfile(ctx context.Context, orgID uuid.UUID, name, industry, size string) error
}

// OrgCurrencyUpdater persists the tenant's display currency (onboarding step, and
// later the settings screen). Optional (nil-safe), kept off OrgUpdater so callers
// without a currency path are unaffected.
type OrgCurrencyUpdater interface {
	SetOrganizationCurrency(ctx context.Context, orgID uuid.UUID, currency string) error
}

// ProfileUpdater applies the profile step to the User row. Optional.
type ProfileUpdater interface {
	UpdateUserProfile(ctx context.Context, userID uuid.UUID, fullName, jobTitle, avatarURL string) error
}

// WizardState is the payload of GET /onboarding/state.
type WizardState struct {
	CurrentStep string         `json:"current_step"`
	Steps       []string       `json:"steps"`
	StepIndex   int            `json:"step_index"`
	Completed   bool           `json:"completed"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Percent     int            `json:"percent"`
	Industry    string         `json:"industry,omitempty"`
	Country     string         `json:"country,omitempty"`
	Goal        string         `json:"goal,omitempty"`
	Answers     domain.JSONMap `json:"answers"`
	// Landing is where the wizard drops the user on completion (derived from the
	// chosen goal).
	Landing string `json:"landing"`
}

// OnboardingUseCase serves the wizard: read state, save one step, complete.
type OnboardingUseCase struct {
	repo     domain.OnboardingRepository
	recorder *Recorder
	orgs     OrgUpdater
	orgCur   OrgCurrencyUpdater
	profiles ProfileUpdater
	now      func() time.Time
}

// WithOrgCurrencyUpdater attaches the optional tenant-currency writer.
func (uc *OnboardingUseCase) WithOrgCurrencyUpdater(u OrgCurrencyUpdater) *OnboardingUseCase {
	uc.orgCur = u
	return uc
}

// NewOnboardingUseCase builds the use case with the required repository.
func NewOnboardingUseCase(repo domain.OnboardingRepository, recorder *Recorder) *OnboardingUseCase {
	return &OnboardingUseCase{
		repo:     repo,
		recorder: recorder,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// WithOrgUpdater attaches the optional organization writer.
func (uc *OnboardingUseCase) WithOrgUpdater(o OrgUpdater) *OnboardingUseCase { uc.orgs = o; return uc }

// WithProfileUpdater attaches the optional user-profile writer.
func (uc *OnboardingUseCase) WithProfileUpdater(p ProfileUpdater) *OnboardingUseCase {
	uc.profiles = p
	return uc
}

// GetState returns the user's wizard state, creating an implicit empty one the
// first time (never an error, never a 404 — a newcomer's very first request must
// not be a failure).
func (uc *OnboardingUseCase) GetState(ctx context.Context, tenantID, userID uuid.UUID) (*WizardState, error) {
	if tenantID == uuid.Nil || userID == uuid.Nil {
		return nil, domain.NewValidationError("tenant and user are required")
	}

	progress, err := uc.repo.Get(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	if progress == nil {
		progress = &domain.OnboardingProgress{
			TenantID:    tenantID,
			UserID:      userID,
			CurrentStep: domain.OnboardingStepOrganization,
		}
	}
	return toWizardState(progress), nil
}

// SaveStepInput is one step's submission.
type SaveStepInput struct {
	Step domain.OnboardingStepKey
	// Answers is the raw step payload, stored verbatim so the form can be
	// repopulated exactly as the user left it.
	Answers domain.JSONMap
	// Next is the step to move to. Empty means "the following step"; it may point
	// BACKWARDS — going back to fix an answer is a supported move, not an error.
	Next string
	// CanEditOrganization gates the ONE answer in this wizard that is not personal.
	//
	// Everyone who joins a tenant walks their own wizard, including a member
	// invited into an already-configured organization. Without this gate, that
	// member's organization step would rename the whole company — a privilege
	// escalation smuggled in through an onboarding form. The handler sets it from
	// the caller's claims; when it is false the answers are still stored (so the
	// sector still drives their suggestions) but the Organization row is untouched.
	CanEditOrganization bool
}

// SaveStep persists one step and advances (or rewinds) the cursor.
func (uc *OnboardingUseCase) SaveStep(ctx context.Context, tenantID, userID uuid.UUID, input SaveStepInput) (*WizardState, error) {
	if tenantID == uuid.Nil || userID == uuid.Nil {
		return nil, domain.NewValidationError("tenant and user are required")
	}
	if _, err := domain.ParseOnboardingStep(string(input.Step)); err != nil {
		return nil, err
	}

	progress, err := uc.repo.Get(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	if progress == nil {
		progress = &domain.OnboardingProgress{TenantID: tenantID, UserID: userID}
	}
	progress.TenantID = tenantID
	progress.UserID = userID
	progress.SetStepAnswers(input.Step, input.Answers)

	// Promote the answers that drive the templates.
	switch input.Step {
	case domain.OnboardingStepOrganization:
		progress.Industry = stringAnswer(input.Answers, "industry")
		progress.Country = stringAnswer(input.Answers, "country")
		if uc.orgs != nil && input.CanEditOrganization {
			// Best-effort: a failure to rename the organization must not lose the
			// user's answers or block the wizard.
			_ = uc.orgs.UpdateOrganizationProfile(ctx, tenantID,
				stringAnswer(input.Answers, "name"),
				progress.Industry,
				stringAnswer(input.Answers, "size"),
			)
		}
		if uc.orgCur != nil && input.CanEditOrganization {
			// Persist the chosen display currency so the financial engine converts
			// every figure into it. Best-effort — never blocks the wizard.
			_ = uc.orgCur.SetOrganizationCurrency(ctx, tenantID, stringAnswer(input.Answers, "currency"))
		}
	case domain.OnboardingStepProfile:
		if uc.profiles != nil {
			_ = uc.profiles.UpdateUserProfile(ctx, userID,
				stringAnswer(input.Answers, "full_name"),
				stringAnswer(input.Answers, "job_title"),
				stringAnswer(input.Answers, "avatar_url"),
			)
		}
		// The profile step is itself a checklist step — one event key, recorded
		// here, so the panel ticks from a server fact like every other row.
		uc.recorder.RecordFor(ctx, tenantID, userID, string(domain.ActivationProfileCompleted), map[string]interface{}{
			"source": "onboarding_wizard",
		})
	case domain.OnboardingStepGoal:
		progress.Goal = stringAnswer(input.Answers, "goal")
	}

	// Move the cursor. An explicit Next wins (including backwards); otherwise
	// advance one step, clamped at the last.
	progress.CurrentStep = uc.nextStep(input.Step, input.Next)

	if err := uc.repo.Save(ctx, progress); err != nil {
		return nil, err
	}
	return toWizardState(progress), nil
}

// nextStep resolves the cursor move.
func (uc *OnboardingUseCase) nextStep(current domain.OnboardingStepKey, next string) domain.OnboardingStepKey {
	if next != "" {
		if parsed, err := domain.ParseOnboardingStep(next); err == nil {
			return parsed
		}
	}
	idx := current.Index() + 1
	if idx >= len(domain.OnboardingStepOrder) {
		idx = len(domain.OnboardingStepOrder) - 1
	}
	return domain.OnboardingStepOrder[idx]
}

// Complete closes the wizard and lifts the route guard. Idempotent: completing an
// already-complete wizard returns the same state instead of erroring, so a double
// submit (or a refresh on the last step) is harmless.
func (uc *OnboardingUseCase) Complete(ctx context.Context, tenantID, userID uuid.UUID) (*WizardState, error) {
	if tenantID == uuid.Nil || userID == uuid.Nil {
		return nil, domain.NewValidationError("tenant and user are required")
	}

	progress, err := uc.repo.Get(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	if progress == nil {
		progress = &domain.OnboardingProgress{TenantID: tenantID, UserID: userID}
	}
	if !progress.Completed {
		now := uc.now()
		progress.Completed = true
		progress.CompletedAt = &now
	}
	progress.CurrentStep = domain.OnboardingStepTeam
	progress.TenantID = tenantID
	progress.UserID = userID

	if err := uc.repo.Save(ctx, progress); err != nil {
		return nil, err
	}
	return toWizardState(progress), nil
}

// Suggestions is the payload of GET /onboarding/suggestions: the sector/goal
// driven content the wizard and the guided first risk render.
type Suggestions struct {
	Sectors    []onboarding.Sector         `json:"sectors"`
	Goals      []onboarding.Goal           `json:"goals"`
	Risks      []onboarding.RiskSuggestion `json:"risks"`
	Frameworks []string                    `json:"frameworks"`
	Industry   string                      `json:"industry,omitempty"`
	Country    string                      `json:"country,omitempty"`
	Goal       string                      `json:"goal,omitempty"`
}

// GetSuggestions resolves the templates for this user's stored answers, with
// per-request overrides so the wizard can preview a sector before saving it.
//
// Nothing here creates anything: these are drafts the user opens, edits and
// validates (spec §5 — we never auto-create a risk).
func (uc *OnboardingUseCase) GetSuggestions(ctx context.Context, tenantID, userID uuid.UUID, industry, country, goal string) *Suggestions {
	if industry == "" || country == "" || goal == "" {
		if progress, err := uc.repo.Get(ctx, tenantID, userID); err == nil && progress != nil {
			if industry == "" {
				industry = progress.Industry
			}
			if country == "" {
				country = progress.Country
			}
			if goal == "" {
				goal = progress.Goal
			}
		}
	}

	return &Suggestions{
		Sectors:    onboarding.Sectors(),
		Goals:      onboarding.Goals(),
		Risks:      onboarding.RiskSuggestionsFor(industry),
		Frameworks: onboarding.SuggestedFrameworks(industry, country, goal),
		Industry:   industry,
		Country:    country,
		Goal:       goal,
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func toWizardState(p *domain.OnboardingProgress) *WizardState {
	steps := make([]string, 0, len(domain.OnboardingStepOrder))
	for _, s := range domain.OnboardingStepOrder {
		steps = append(steps, string(s))
	}

	answers := p.Answers
	if answers == nil {
		answers = domain.JSONMap{}
	}

	idx := p.CurrentStep.Index()
	state := &WizardState{
		CurrentStep: string(p.CurrentStep),
		Steps:       steps,
		StepIndex:   idx,
		Completed:   p.Completed,
		CompletedAt: p.CompletedAt,
		Industry:    p.Industry,
		Country:     p.Country,
		Goal:        p.Goal,
		Answers:     answers,
		Landing:     onboarding.LandingForGoal(p.Goal),
	}
	if p.Completed {
		state.Percent = 100
	} else {
		state.Percent = idx * 100 / len(steps)
	}
	if state.CurrentStep == "" {
		state.CurrentStep = string(domain.OnboardingStepOrganization)
	}
	return state
}

func stringAnswer(answers domain.JSONMap, key string) string {
	if answers == nil {
		return ""
	}
	if v, ok := answers[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
