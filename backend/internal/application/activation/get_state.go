// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package activation

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// Step is one checklist row as the client renders it. Everything the panel needs
// is here — including the copy — so the panel itself holds NO logic: it maps this
// slice to rows and nothing else. That is the direct fix for the checklist that
// never ticked and the two rows that struck through together.
type Step struct {
	Key       string            `json:"key"`
	EventKey  string            `json:"event_key"` // the ONE event that ticks it
	LabelI18n map[string]string `json:"label_i18n"`
	HintI18n  map[string]string `json:"hint_i18n"`
	Completed bool              `json:"completed"`
	// CompletedAt is nil until the step's event first occurs, and never moves
	// afterwards (the repository takes the FIRST occurrence per key).
	CompletedAt *time.Time `json:"completed_at"`
	DeepLink    string     `json:"deep_link"`
	Order       int        `json:"order"`
	Primary     bool       `json:"primary"`
	// Celebrate is the server's instruction to fire the celebration exactly once:
	// true only when the step is completed AND this user has not yet acknowledged
	// it via POST /activation/celebrated. The client never decides on its own.
	Celebrate bool `json:"celebrate"`
}

// State is the payload of GET /activation/state.
type State struct {
	Steps   []Step `json:"steps"`
	Percent int    `json:"percent"`
	// AhaReachedAt is when the product first proved its value to this tenant —
	// see MaybeRecordAha for the definition. Nil until then.
	AhaReachedAt *time.Time `json:"aha_reached_at"`
	SignupAt     *time.Time `json:"signup_at,omitempty"`
	// TimeToAhaSeconds mirrors the Prometheus histogram for the UI/debugging.
	TimeToAhaSeconds *float64 `json:"time_to_aha_seconds,omitempty"`
}

// GetStateUseCase builds the activation state for one (tenant, user).
type GetStateUseCase struct {
	repo domain.ActivationRepository
}

// NewGetStateUseCase builds the use case.
func NewGetStateUseCase(repo domain.ActivationRepository) *GetStateUseCase {
	return &GetStateUseCase{repo: repo}
}

// Execute returns the checklist. It degrades rather than fails: a broken event
// store yields an all-incomplete checklist (an honest "nothing done yet"), never
// an error page in place of the dashboard's first card.
func (uc *GetStateUseCase) Execute(ctx context.Context, tenantID, userID uuid.UUID) (*State, error) {
	defs := domain.ActivationSteps()

	firsts := map[domain.ActivationEventKey]time.Time{}
	celebrated := map[string]time.Time{}
	if uc.repo != nil && tenantID != uuid.Nil {
		if got, err := uc.repo.FirstOccurrences(ctx, tenantID); err == nil {
			firsts = got
		}
		if got, err := uc.repo.CelebratedSteps(ctx, tenantID, userID); err == nil {
			celebrated = got
		}
	}

	steps := make([]Step, 0, len(defs))
	done := 0
	for _, def := range defs {
		step := Step{
			Key:       def.Key,
			EventKey:  string(def.EventKey),
			LabelI18n: def.LabelI18n,
			HintI18n:  def.HintI18n,
			DeepLink:  def.DeepLink,
			Order:     def.Order,
			Primary:   def.Primary,
		}
		// Exactly one event key per step (domain.ValidateActivationSteps), so one
		// import can never tick two rows.
		if at, ok := firsts[def.EventKey]; ok {
			completedAt := at
			step.Completed = true
			step.CompletedAt = &completedAt
			done++

			_, alreadyCelebrated := celebrated[def.Key]
			step.Celebrate = !alreadyCelebrated && userID != uuid.Nil
		}
		steps = append(steps, step)
	}

	state := &State{Steps: steps, Percent: percent(done, len(steps))}

	if at, ok := firsts[domain.ActivationAhaReached]; ok {
		aha := at
		state.AhaReachedAt = &aha
		if signup, ok := firsts[domain.ActivationSignup]; ok {
			signupAt := signup
			state.SignupAt = &signupAt
			if secs := aha.Sub(signup).Seconds(); secs > 0 {
				state.TimeToAhaSeconds = &secs
			}
		}
	} else if signup, ok := firsts[domain.ActivationSignup]; ok {
		signupAt := signup
		state.SignupAt = &signupAt
	}

	return state, nil
}

// MarkCelebratedUseCase acknowledges that a user has seen a step's celebration.
type MarkCelebratedUseCase struct {
	repo domain.ActivationRepository
}

// NewMarkCelebratedUseCase builds the use case.
func NewMarkCelebratedUseCase(repo domain.ActivationRepository) *MarkCelebratedUseCase {
	return &MarkCelebratedUseCase{repo: repo}
}

// Execute records the acknowledgement. Idempotent at the repository level, and
// validated against the step catalog so a stray key cannot pollute the ledger.
func (uc *MarkCelebratedUseCase) Execute(ctx context.Context, tenantID, userID uuid.UUID, stepKey string) error {
	if tenantID == uuid.Nil || userID == uuid.Nil {
		return domain.NewValidationError("tenant and user are required")
	}
	if !isKnownStep(stepKey) {
		return domain.NewValidationError("unknown activation step: " + stepKey)
	}
	if uc.repo == nil {
		return nil
	}
	return uc.repo.MarkCelebrated(ctx, tenantID, userID, stepKey)
}

func isKnownStep(key string) bool {
	for _, s := range domain.ActivationSteps() {
		if s.Key == key {
			return true
		}
	}
	return false
}

func percent(done, total int) int {
	if total <= 0 {
		return 0
	}
	return done * 100 / total
}
