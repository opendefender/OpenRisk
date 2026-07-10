// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormBoardReportRepository implements domain.BoardReportRepository using GORM.
// ABSOLUTE RULE: every query filters by tenant_id — a tenant only ever sees or
// mutates its own board reports.
type GormBoardReportRepository struct {
	db *gorm.DB
}

// NewGormBoardReportRepository creates a new GORM-backed board-report repository.
func NewGormBoardReportRepository(db *gorm.DB) *GormBoardReportRepository {
	return &GormBoardReportRepository{db: db}
}

func (r *GormBoardReportRepository) Create(ctx context.Context, report *domain.BoardReport) error {
	if report.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	return r.db.WithContext(ctx).Create(report).Error
}

func (r *GormBoardReportRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.BoardReport, error) {
	var report domain.BoardReport
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&report).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &report, nil
}

func (r *GormBoardReportRepository) List(ctx context.Context, tenantID uuid.UUID) ([]domain.BoardReport, error) {
	var reports []domain.BoardReport
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Find(&reports).Error
	if err != nil {
		return nil, err
	}
	return reports, nil
}

// Update saves the report. The WHERE clause pins both id and tenant_id so a
// crafted payload can never write across tenants.
func (r *GormBoardReportRepository) Update(ctx context.Context, report *domain.BoardReport) error {
	res := r.db.WithContext(ctx).
		Model(&domain.BoardReport{}).
		Where("id = ? AND tenant_id = ?", report.ID, report.TenantID).
		Updates(map[string]interface{}{
			"title":                 report.Title,
			"status":                report.Status,
			"executive_summary":     report.ExecutiveSummary,
			"risk_commentary":       report.RiskCommentary,
			"compliance_commentary": report.ComplianceCommentary,
			"financial_commentary":  report.FinancialCommentary,
			"recommendations":       report.Recommendations,
			"approved_by":           report.ApprovedBy,
			"approved_at":           report.ApprovedAt,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("board_report", report.ID)
	}
	return nil
}

func (r *GormBoardReportRepository) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&domain.BoardReport{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("board_report", id)
	}
	return nil
}
