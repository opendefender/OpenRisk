// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package board

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// =============================================================================
// Get / List
// =============================================================================

type GetBoardReportUseCase struct{ repo domain.BoardReportRepository }

func NewGetBoardReportUseCase(repo domain.BoardReportRepository) *GetBoardReportUseCase {
	return &GetBoardReportUseCase{repo: repo}
}

func (uc *GetBoardReportUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID) (*domain.BoardReport, error) {
	report, err := uc.repo.GetByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, domain.NewNotFoundError("board_report", id)
	}
	return report, nil
}

type ListBoardReportsUseCase struct{ repo domain.BoardReportRepository }

func NewListBoardReportsUseCase(repo domain.BoardReportRepository) *ListBoardReportsUseCase {
	return &ListBoardReportsUseCase{repo: repo}
}

func (uc *ListBoardReportsUseCase) Execute(ctx context.Context, tenantID uuid.UUID) ([]domain.BoardReport, error) {
	return uc.repo.List(ctx, tenantID)
}

// =============================================================================
// Update (narrative edits — draft only)
// =============================================================================

type UpdateBoardReportInput struct {
	Title                *string
	ExecutiveSummary     *string
	RiskCommentary       *string
	ComplianceCommentary *string
	FinancialCommentary  *string
	Recommendations      *[]string
}

type UpdateBoardReportUseCase struct{ repo domain.BoardReportRepository }

func NewUpdateBoardReportUseCase(repo domain.BoardReportRepository) *UpdateBoardReportUseCase {
	return &UpdateBoardReportUseCase{repo: repo}
}

// Execute applies partial narrative edits. Only DRAFT reports are editable — an
// approved report is frozen so the diffused version stays authoritative.
func (uc *UpdateBoardReportUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID, in UpdateBoardReportInput) (*domain.BoardReport, error) {
	report, err := uc.repo.GetByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, domain.NewNotFoundError("board_report", id)
	}
	if report.Status == domain.BoardReportApproved {
		return nil, domain.NewValidationError("an approved report cannot be edited")
	}

	if in.Title != nil {
		report.Title = *in.Title
	}
	if in.ExecutiveSummary != nil {
		report.ExecutiveSummary = *in.ExecutiveSummary
	}
	if in.RiskCommentary != nil {
		report.RiskCommentary = *in.RiskCommentary
	}
	if in.ComplianceCommentary != nil {
		report.ComplianceCommentary = *in.ComplianceCommentary
	}
	if in.FinancialCommentary != nil {
		report.FinancialCommentary = *in.FinancialCommentary
	}
	if in.Recommendations != nil {
		report.Recommendations = *in.Recommendations
	}

	if err := uc.repo.Update(ctx, report); err != nil {
		return nil, err
	}
	return report, nil
}

// =============================================================================
// Approve (human-in-the-loop endorsement)
// =============================================================================

type ApproveBoardReportUseCase struct{ repo domain.BoardReportRepository }

func NewApproveBoardReportUseCase(repo domain.BoardReportRepository) *ApproveBoardReportUseCase {
	return &ApproveBoardReportUseCase{repo: repo}
}

// Execute marks a draft report approved, recording who approved it and when.
// Approving an already-approved report is idempotent (returns it unchanged).
func (uc *ApproveBoardReportUseCase) Execute(ctx context.Context, tenantID, id, approvedBy uuid.UUID) (*domain.BoardReport, error) {
	report, err := uc.repo.GetByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, domain.NewNotFoundError("board_report", id)
	}
	if report.Status == domain.BoardReportApproved {
		return report, nil
	}

	now := time.Now()
	report.Status = domain.BoardReportApproved
	report.ApprovedBy = &approvedBy
	report.ApprovedAt = &now

	if err := uc.repo.Update(ctx, report); err != nil {
		return nil, err
	}
	return report, nil
}

// =============================================================================
// Delete
// =============================================================================

type DeleteBoardReportUseCase struct{ repo domain.BoardReportRepository }

func NewDeleteBoardReportUseCase(repo domain.BoardReportRepository) *DeleteBoardReportUseCase {
	return &DeleteBoardReportUseCase{repo: repo}
}

func (uc *DeleteBoardReportUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID) error {
	return uc.repo.Delete(ctx, id, tenantID)
}
