// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// Register views, the summary aggregates and the score-trend batch (#913).
//
// Every query here opens with the tenant predicate. risk_histories has no
// tenant column, so the trend read joins it to risks and filters on
// risks.tenant_id; a history row of another tenant's risk cannot be reached by
// passing its risk id.

var (
	_ domain.RiskSummaryRepository = (*GormRiskRepository)(nil)
	_ domain.RiskTrendReader       = (*GormRiskRepository)(nil)
)

// involvedCondition is the "mine" predicate: owner, assignee (current or
// legacy column) or reviewer is the user. The one definition behind ?mine=true,
// the "mine" view and its count.
func involvedCondition(u uuid.UUID) (string, []any) {
	return "(owner_id = ? OR assignee_id = ? OR reviewer_id = ? OR assigned_to = ?)", []any{u, u, u, u}
}

// applyRiskSearch narrows a risk query by the register's search box. Full-text
// search (tsvector on name+description) OR'd with a prefix-ish ILIKE. The
// tsvector alone is lexeme-exact: typing "log" matched nothing while
// "Log4Shell" sat in the register, which made the search box look broken. The
// ILIKE arm only ever ADDS rows, so full-text ranking behaviour for real words
// is unchanged. Both arms are bound parameters.
func applyRiskSearch(db *gorm.DB, search, language string) *gorm.DB {
	if search == "" {
		return db
	}
	if language == "" {
		language = "french"
	}
	like := "%" + search + "%"
	return db.Where(
		"(to_tsvector(?, name || ' ' || COALESCE(description, '')) @@ plainto_tsquery(?, ?) OR name ILIKE ? OR title ILIKE ? OR COALESCE(description, '') ILIKE ?)",
		language, language, search, like, like, like,
	)
}

// statusSelectorCondition renders a resolved status filter. The column is
// compared lower-cased and NULL-safe (NULL reads as open). "Open" is the
// complement of every other status, so a stored value nobody claims still
// belongs to a register status.
func statusSelectorCondition(sel domain.StatusSelector) (string, []any) {
	const col = "LOWER(COALESCE(status, ''))"
	var parts []string
	var args []any
	if len(sel.In) > 0 {
		parts, args = append(parts, col+" IN ?"), append(args, sel.In)
	}
	if sel.IncludeOpen {
		parts, args = append(parts, col+" NOT IN ?"), append(args, sel.NonOpen)
	}
	if len(parts) == 0 {
		return "1 = 0", nil
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

// viewCondition renders a view predicate as one SQL condition. ok is false for
// the unconstrained view. "mine" with no authenticated user renders a condition
// that matches nothing: failing closed beats showing the whole register under a
// tab that says "mine".
//
// Status goes through domain.SelectStatuses, the one mapping of the register's
// values onto both stored vocabularies.
func viewCondition(p domain.RiskViewPredicate, user *uuid.UUID) (cond string, args []any, ok bool) {
	if p.IsZero() {
		return "", nil, false
	}
	var parts []string
	if p.InvolvedOnly {
		if user == nil || *user == uuid.Nil {
			return "1 = 0", nil, true
		}
		c, a := involvedCondition(*user)
		parts, args = append(parts, c), append(args, a...)
	}
	if len(p.CriticalityIn) > 0 {
		levels := make([]string, len(p.CriticalityIn))
		for i, l := range p.CriticalityIn {
			levels[i] = strings.ToLower(string(l))
		}
		parts, args = append(parts, "LOWER(criticality) IN ?"), append(args, levels)
	}
	if p.StatusIs != "" {
		c, a := statusSelectorCondition(domain.SelectStatuses([]string{string(p.StatusIs)}))
		parts, args = append(parts, c), append(args, a...)
	}
	if p.StatusNot != "" {
		c, a := statusSelectorCondition(domain.SelectStatuses([]string{string(p.StatusNot)}))
		parts, args = append(parts, "NOT "+c), append(args, a...)
	}
	if p.NextReviewBy != nil {
		parts, args = append(parts, "(next_review_at IS NOT NULL AND next_review_at <= ?)"), append(args, *p.NextReviewBy)
	}
	return "(" + strings.Join(parts, " AND ") + ")", args, true
}

// applyRiskView narrows a risk query by a view definition.
func applyRiskView(db *gorm.DB, p domain.RiskViewPredicate, user *uuid.UUID) *gorm.DB {
	if cond, args, ok := viewCondition(p, user); ok {
		return db.Where(cond, args...)
	}
	return db
}

// ViewCounts counts every register view for the tenant in one query. Each
// count is a FILTER over the same condition the list applies.
func (r *GormRiskRepository) ViewCounts(ctx context.Context, tenantID, userID uuid.UUID) (map[domain.RiskView]int64, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("tenant required")
	}
	now := r.now()
	var user *uuid.UUID
	if userID != uuid.Nil {
		user = &userID
	}

	cols := make([]string, 0, len(domain.AllRiskViews))
	var args []any
	for i, v := range domain.AllRiskViews {
		cond, a, ok := viewCondition(v.Predicate(now), user)
		if !ok {
			cond, a = "TRUE", nil
		}
		cols = append(cols, fmt.Sprintf("COUNT(*) FILTER (WHERE %s) AS v%d", cond, i))
		args = append(args, a...)
	}

	row := map[string]any{}
	err := r.db.WithContext(ctx).
		Model(&domain.Risk{}).
		Select(strings.Join(cols, ", "), args...).
		Where("tenant_id = ?", tenantID).
		Take(&row).Error
	if err != nil {
		return nil, fmt.Errorf("count risk views: %w", err)
	}

	out := make(map[domain.RiskView]int64, len(domain.AllRiskViews))
	for i, v := range domain.AllRiskViews {
		out[v] = toInt64(row[fmt.Sprintf("v%d", i)])
	}
	return out, nil
}

// CriticalityCounts counts the tenant's risks per criticality level inside a
// view and a search. Levels with no risk are present with 0; a level outside
// the four documented ones is not counted.
func (r *GormRiskRepository) CriticalityCounts(ctx context.Context, tenantID, userID uuid.UUID, view domain.RiskView, search string) (map[domain.CriticalityLevel]int64, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("tenant required")
	}
	var user *uuid.UUID
	if userID != uuid.Nil {
		user = &userID
	}

	db := r.db.WithContext(ctx).
		Model(&domain.Risk{}).
		Where("tenant_id = ?", tenantID)
	db = applyRiskSearch(db, search, "french")
	db = applyRiskView(db, view.Predicate(r.now()), user)

	type row struct {
		Criticality string
		Count       int64
	}
	var rows []row
	if err := db.
		Select("LOWER(criticality) AS criticality, COUNT(*) AS count").
		Group("LOWER(criticality)").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("count risks by criticality: %w", err)
	}

	out := map[domain.CriticalityLevel]int64{
		domain.CriticalityLowNew:      0,
		domain.CriticalityMediumNew:   0,
		domain.CriticalityHighNew:     0,
		domain.CriticalityCriticalNew: 0,
	}
	for _, rr := range rows {
		level := domain.CriticalityLevel(rr.Criticality)
		if _, known := out[level]; known {
			out[level] += rr.Count
		}
	}
	return out, nil
}

// activeRisks is the tenant's not-closed risks. Soft-deleted rows are excluded
// by the model's DeletedAt.
func (r *GormRiskRepository) activeRisks(ctx context.Context, tenantID uuid.UUID) *gorm.DB {
	cond, args, _ := viewCondition(domain.RiskViewPredicate{StatusNot: domain.RiskClosed}, nil)
	return r.db.WithContext(ctx).
		Model(&domain.Risk{}).
		Where("tenant_id = ?", tenantID).
		Where(cond, args...)
}

// CountActive counts the tenant's risks that are not closed.
func (r *GormRiskRepository) CountActive(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	if tenantID == uuid.Nil {
		return 0, domain.NewUnauthorizedError("tenant required")
	}
	var n int64
	if err := r.activeRisks(ctx, tenantID).Count(&n).Error; err != nil {
		return 0, fmt.Errorf("count active risks: %w", err)
	}
	return n, nil
}

// ActiveExposureInputs returns the ALE inputs (SLE, ARO, criticality) of every
// not-closed risk, three columns each. The sum is taken by the caller with the
// same quantifier the list uses per row.
func (r *GormRiskRepository) ActiveExposureInputs(ctx context.Context, tenantID uuid.UUID) ([]domain.RiskExposureInput, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("tenant required")
	}
	var risks []domain.Risk
	if err := r.activeRisks(ctx, tenantID).
		Select("SLEXAF", "ARO", "Criticality").
		Find(&risks).Error; err != nil {
		return nil, fmt.Errorf("read active risk exposure: %w", err)
	}
	out := make([]domain.RiskExposureInput, len(risks))
	for i := range risks {
		out[i] = domain.RiskExposureInput{SLEXAF: risks[i].SLEXAF, ARO: risks[i].ARO, Criticality: risks[i].Criticality}
	}
	return out, nil
}

// TrendSnapshots reads, for a page of risks and in ONE query, the score of the
// latest snapshot taken at or before cutoff and the score of the oldest one.
// The choice between them is domain.TrendSnapshots.Baseline. Risks with no
// snapshot have no entry.
//
// Postgres-specific (array_agg ... FILTER); covered by a DATABASE_URL-gated
// test. The join to risks is what scopes it to the tenant.
func (r *GormRiskRepository) TrendSnapshots(ctx context.Context, tenantID uuid.UUID, riskIDs []uuid.UUID, cutoff time.Time) (map[uuid.UUID]domain.TrendSnapshots, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("tenant required")
	}
	out := make(map[uuid.UUID]domain.TrendSnapshots, len(riskIDs))
	if len(riskIDs) == 0 {
		return out, nil
	}

	type row struct {
		RiskID   uuid.UUID
		AtCutoff *float64
		Oldest   float64
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("risk_histories AS h").
		Joins("JOIN risks r ON r.id = h.risk_id").
		Where("r.tenant_id = ? AND r.deleted_at IS NULL AND h.risk_id IN ?", tenantID, riskIDs).
		Select(`h.risk_id AS risk_id,
			(array_agg(h.score ORDER BY h.created_at DESC) FILTER (WHERE h.created_at <= ?))[1] AS at_cutoff,
			(array_agg(h.score ORDER BY h.created_at ASC))[1] AS oldest`, cutoff).
		Group("h.risk_id").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("read risk trend snapshots: %w", err)
	}
	for _, rr := range rows {
		out[rr.RiskID] = domain.TrendSnapshots{AtCutoff: rr.AtCutoff, Oldest: rr.Oldest}
	}
	return out, nil
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int32:
		return int64(n)
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case []byte:
		var x int64
		_, _ = fmt.Sscan(string(n), &x)
		return x
	default:
		return 0
	}
}
