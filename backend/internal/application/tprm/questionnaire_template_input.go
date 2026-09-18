// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// QuestionInput is one question as the author writes it.
type QuestionInput struct {
	Text       string                       `json:"text"`
	Help       string                       `json:"help"`
	AnswerType domain.VendorAnswerType      `json:"answer_type"`
	Options    domain.VendorQuestionOptions `json:"options"`
	Weight     float64                      `json:"weight"`
	Required   bool                         `json:"required"`
	NAAllowed  bool                         `json:"na_allowed"`
	ControlRef string                       `json:"control_ref"`
}

// TemplateInput is a whole questionnaire as the author writes it. Questions are
// taken in the order given; positions are assigned from that order.
type TemplateInput struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Language    string          `json:"language"`
	Questions   []QuestionInput `json:"questions"`
}

// applyTemplateInput writes in onto t and builds its questions, validated. It
// is shared by create and update so the two cannot validate differently.
func applyTemplateInput(t *domain.VendorQuestionnaireTemplate, in TemplateInput) error {
	t.Name = strings.TrimSpace(in.Name)
	t.Description = strings.TrimSpace(in.Description)
	t.Language = strings.TrimSpace(in.Language)
	if err := t.Validate(); err != nil {
		return err
	}

	if len(in.Questions) == 0 {
		return domain.NewValidationError("a questionnaire needs at least one question")
	}
	if len(in.Questions) > domain.MaxVendorQuestions {
		return domain.NewValidationError(fmt.Sprintf("a questionnaire holds at most %d questions", domain.MaxVendorQuestions))
	}

	questions := make([]domain.VendorQuestionnaireQuestion, 0, len(in.Questions))
	for i, qi := range in.Questions {
		var opts domain.VendorQuestionOptions
		if qi.Options != nil {
			opts = append(domain.VendorQuestionOptions(nil), qi.Options...)
		}
		q := domain.VendorQuestionnaireQuestion{
			ID:         uuid.New(),
			TenantID:   t.TenantID,
			TemplateID: t.ID,
			Position:   i + 1,
			Text:       qi.Text,
			Help:       qi.Help,
			AnswerType: qi.AnswerType,
			Options:    opts,
			Weight:     qi.Weight,
			Required:   qi.Required,
			NAAllowed:  qi.NAAllowed,
			ControlRef: qi.ControlRef,
		}
		q.Normalize()
		if err := q.Validate(); err != nil {
			return domain.NewValidationError(fmt.Sprintf("question %d: %s", i+1, domain.MessageFromError(err)))
		}
		questions = append(questions, q)
	}
	t.Questions = questions
	return nil
}
