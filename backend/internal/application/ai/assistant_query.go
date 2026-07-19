// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	llm "github.com/opendefender/openrisk/pkg/ai"
)

// AssistantAnswerResult wraps the answer with its provider and the snippets that
// were retrieved (so the UI can show what the answer was grounded on).
type AssistantAnswerResult struct {
	Answer      llm.AssistantAnswer    `json:"answer"`
	GeneratedBy string                 `json:"generated_by"`
	Retrieved   []llm.KnowledgeSnippet `json:"retrieved"`
}

// AssistantQueryUseCase answers a natural-language question over the tenant's GRC
// knowledge base (spec §12.3). It performs a lightweight hybrid retrieval — the
// tenant's own full-text risk search plus keyword matching over controls and
// vulnerabilities — and hands the top matches to the assistant as RAG context.
// Every source is optional and nil-safe.
type AssistantQueryUseCase struct {
	assistant  llm.Assistant
	risks      RiskLister       // optional
	compliance ComplianceReader // optional
	vulns      VulnLister       // optional
	orgs       OrgLookup        // optional
}

func NewAssistantQueryUseCase(assistant llm.Assistant) *AssistantQueryUseCase {
	return &AssistantQueryUseCase{assistant: assistant}
}

func (uc *AssistantQueryUseCase) WithRisks(r RiskLister) *AssistantQueryUseCase        { uc.risks = r; return uc }
func (uc *AssistantQueryUseCase) WithCompliance(c ComplianceReader) *AssistantQueryUseCase { uc.compliance = c; return uc }
func (uc *AssistantQueryUseCase) WithVulns(v VulnLister) *AssistantQueryUseCase         { uc.vulns = v; return uc }
func (uc *AssistantQueryUseCase) WithOrgs(o OrgLookup) *AssistantQueryUseCase           { uc.orgs = o; return uc }

// QueryInput is the user's turn plus prior conversation.
type QueryInput struct {
	Question string
	History  []llm.ChatTurn
	Locale   string
}

// Execute retrieves relevant GRC context and asks the assistant for an answer.
func (uc *AssistantQueryUseCase) Execute(ctx context.Context, tenantID uuid.UUID, in QueryInput) (*AssistantAnswerResult, error) {
	if strings.TrimSpace(in.Question) == "" {
		return nil, domain.NewValidationError("question is required")
	}

	snippets := uc.retrieve(ctx, tenantID, in.Question)

	query := llm.AssistantQuery{
		Locale:   llm.Locale(in.Locale),
		Question: in.Question,
		History:  in.History,
		Snippets: snippets,
		OrgName:  uc.orgName(ctx, tenantID),
	}

	answer, generatedBy := invoke(uc.assistant, func(a llm.Assistant) (llm.AssistantAnswer, error) {
		return a.Answer(ctx, query)
	})
	return &AssistantAnswerResult{Answer: answer, GeneratedBy: generatedBy, Retrieved: snippets}, nil
}

// retrieve performs the hybrid keyword retrieval across the three GRC sources.
func (uc *AssistantQueryUseCase) retrieve(ctx context.Context, tenantID uuid.UUID, question string) []llm.KnowledgeSnippet {
	var out []llm.KnowledgeSnippet
	out = append(out, uc.retrieveRisks(ctx, tenantID, question)...)
	out = append(out, uc.retrieveControls(ctx, tenantID, question)...)
	out = append(out, uc.retrieveVulns(ctx, tenantID, question)...)
	return out
}

func (uc *AssistantQueryUseCase) retrieveRisks(ctx context.Context, tenantID uuid.UUID, question string) []llm.KnowledgeSnippet {
	if uc.risks == nil {
		return nil
	}
	// First try the tenant's full-text risk search; fall back to the most recent
	// risks if the search matches nothing (tsvector can be strict on short queries).
	page, err := uc.risks.List(ctx, tenantID, domain.RiskQuery{Search: question, Page: 1, Limit: 6})
	if err != nil {
		return nil
	}
	if page == nil || len(page.Data) == 0 {
		page, err = uc.risks.List(ctx, tenantID, domain.RiskQuery{Page: 1, Limit: 6, SortBy: "score", SortOrder: "desc"})
		if err != nil || page == nil {
			return nil
		}
	}
	var out []llm.KnowledgeSnippet
	for _, r := range page.Data {
		out = append(out, llm.KnowledgeSnippet{
			Kind:  "risk",
			Ref:   firstNonEmpty(r.Name, r.Title),
			Title: firstNonEmpty(r.Name, r.Title),
			Detail: fmt.Sprintf("criticité=%s score=%.2f statut=%s%s",
				r.Criticality, r.Score, r.Status, describeTags(r.Tags)),
		})
	}
	return out
}

func (uc *AssistantQueryUseCase) retrieveControls(ctx context.Context, tenantID uuid.UUID, question string) []llm.KnowledgeSnippet {
	if uc.compliance == nil {
		return nil
	}
	frameworks, err := uc.compliance.ListFrameworks(ctx, tenantID)
	if err != nil {
		return nil
	}
	terms := keywords(question)
	var out []llm.KnowledgeSnippet
	const maxControls = 6
	for _, fw := range frameworks {
		if len(out) >= maxControls {
			break
		}
		controls, err := uc.compliance.ListControlsByFramework(ctx, tenantID, fw.ID)
		if err != nil {
			continue
		}
		for _, c := range controls {
			if len(out) >= maxControls {
				break
			}
			hay := strings.ToLower(c.ReferenceCode + " " + c.Name + " " + c.Description)
			if !matchesAny(hay, terms) {
				continue
			}
			out = append(out, llm.KnowledgeSnippet{
				Kind:   "control",
				Ref:    fw.Name + " " + c.ReferenceCode,
				Title:  c.Name,
				Detail: fmt.Sprintf("statut=%s référentiel=%s", c.Status, fw.Name),
			})
		}
	}
	return out
}

func (uc *AssistantQueryUseCase) retrieveVulns(ctx context.Context, tenantID uuid.UUID, question string) []llm.KnowledgeSnippet {
	if uc.vulns == nil {
		return nil
	}
	page, err := uc.vulns.List(ctx, tenantID, domain.VulnerabilityQuery{Search: question, Page: 1, Limit: 4})
	if err != nil || page == nil {
		return nil
	}
	var out []llm.KnowledgeSnippet
	for _, v := range page.Data {
		ref := v.CVEID
		if ref == "" {
			ref = v.Title
		}
		out = append(out, llm.KnowledgeSnippet{
			Kind:   "vulnerability",
			Ref:    ref,
			Title:  v.Title,
			Detail: fmt.Sprintf("CVSS=%.1f sévérité=%s tier=%s KEV=%t actif=%s", v.CVSSScore, v.Severity, v.PriorityTier, v.KEV, v.AssetName),
		})
	}
	return out
}

func (uc *AssistantQueryUseCase) orgName(ctx context.Context, tenantID uuid.UUID) string {
	if uc.orgs == nil {
		return ""
	}
	if org, err := uc.orgs.GetByID(ctx, tenantID); err == nil && org != nil {
		return org.Name
	}
	return ""
}

// keywords extracts meaningful lowercase terms (>3 chars) from a question.
func keywords(q string) []string {
	seen := map[string]bool{}
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(q), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && !(r >= 'à' && r <= 'ÿ')
	}) {
		if len(w) <= 3 || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}

func matchesAny(haystack string, terms []string) bool {
	for _, t := range terms {
		if strings.Contains(haystack, t) {
			return true
		}
	}
	return false
}

func describeTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return " tags=" + strings.Join(tags, ",")
}
