// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

// Package ai wires the tenant-scoped GRC data (risks, assets, compliance,
// vulnerabilities, audits) to the pure pkg/ai.Assistant. Each use case assembles
// context — including a lightweight RAG retrieval for the Q&A assistant — and then
// asks the assistant to produce prose. Every LLM call is best-effort: on any error
// the use case falls back to the deterministic template assistant, so an AI
// feature is never a hard dependency (same contract as the board report).
//
// All ports are narrow and optional (nil-safe). A missing source degrades its
// slice of context, never fails the request.
package ai

import (
	"context"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/compliance"
	"github.com/opendefender/openrisk/internal/domain"
	llm "github.com/opendefender/openrisk/pkg/ai"
)

// RiskReader loads a single risk (tenant-scoped). *repository.GormRiskRepository
// satisfies it.
type RiskReader interface {
	GetByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.Risk, error)
}

// RiskLister lists risks for retrieval / dedupe (tenant-scoped).
type RiskLister interface {
	List(ctx context.Context, tenantID uuid.UUID, q domain.RiskQuery) (*domain.PaginatedResult[domain.Risk], error)
}

// AssetReader loads a single asset (tenant-scoped).
type AssetReader interface {
	GetByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.Asset, error)
}

// ComplianceReader is the subset of the compliance repository the assistant needs
// to retrieve controls and resolve framework names.
type ComplianceReader interface {
	ListFrameworks(ctx context.Context, tenantID uuid.UUID) ([]domain.ComplianceFramework, error)
	ListControlsByFramework(ctx context.Context, tenantID uuid.UUID, frameworkID uuid.UUID) ([]domain.ComplianceControl, error)
	GetFrameworkByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.ComplianceFramework, error)
	GetControlByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.ComplianceControl, error)
	GetEvidenceByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.ControlEvidence, error)
}

// VulnLister lists vulnerabilities for retrieval (tenant-scoped).
type VulnLister interface {
	List(ctx context.Context, tenantID uuid.UUID, q domain.VulnerabilityQuery) (*domain.PaginatedResult[domain.Vulnerability], error)
}

// AuditReader loads an audit and its remediation plans (tenant-scoped).
type AuditReader interface {
	GetAuditByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.ComplianceAudit, error)
	ListRemediations(ctx context.Context, tenantID uuid.UUID, filter domain.RemediationFilter) ([]domain.RemediationPlan, error)
}

// GapAnalyzer runs the compliance gap analysis. *compliance.GetGapAnalysisUseCase
// satisfies it.
type GapAnalyzer interface {
	Execute(ctx context.Context, tenantID uuid.UUID, frameworkID uuid.UUID) (*compliance.GapAnalysis, error)
}

// OrgLookup resolves the tenant's display name (for a friendlier assistant tone).
type OrgLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error)
}

// invoke runs an assistant call with a deterministic template fallback and returns
// the provider name (the model that actually produced the result). It mirrors the
// board report's best-effort narrate contract.
func invoke[T any](
	primary llm.Assistant,
	call func(llm.Assistant) (T, error),
) (T, string) {
	fallback := llm.NewTemplateAssistant()
	if primary != nil {
		if out, err := call(primary); err == nil {
			return out, primary.Name()
		}
	}
	out, _ := call(fallback)
	return out, fallback.Name()
}
