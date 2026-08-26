// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/application/entity"
)

// The supplementary timeline journals.
//
// audit_events is the canonical trail and covers every mutating HTTP request.
// These three exist because each records something that never travels through a
// request, or that predates the trail:
//
//   - risk_histories: the Score Engine recomputes a risk from a Redis event, in
//     a worker. No request, no audit entry — and a risk's score moving is the
//     single most interesting thing that happens to it, so a risk timeline
//     without it would be quietly wrong.
//   - incident_timelines: the incident service appends its own lifecycle entries.
//   - asset_snapshots: point-in-time asset state with the actor who caused it,
//     predating the trail and still written on every asset change.
//
// Each is tenant-gated through its PARENT entity, because none of the three has
// a tenant column of its own — risk_histories is a child of a risk,
// incident_timelines of an incident. Reading them without that gate is exactly
// the leak this project fixed in RiskTimelineHandler on 2026-07-23, and the gate
// is re-applied here rather than assumed.

// =============================================================================
// Risk history
// =============================================================================

type RiskHistorySource struct{ db *gorm.DB }

func NewRiskHistorySource(db *gorm.DB) *RiskHistorySource { return &RiskHistorySource{db: db} }

var _ entity.EventSource = (*RiskHistorySource)(nil)

func (s *RiskHistorySource) Source() entity.TimelineSource { return entity.SourceRiskHistory }

func (s *RiskHistorySource) Events(ctx context.Context, tenantID uuid.UUID, entityID string, before *time.Time, limit int) ([]entity.TimelineEvent, error) {
	riskID, err := uuid.Parse(entityID)
	if err != nil {
		return nil, nil
	}
	// The tenant gate: join the parent risk rather than trusting the id.
	q := s.db.WithContext(ctx).
		Table("risk_histories AS h").
		Select("CAST(h.id AS TEXT) AS id, h.score AS score, h.status AS status, h.change_type AS change_type, h.changed_by AS changed_by, h.created_at AS created_at").
		Joins("JOIN risks AS r ON r.id = h.risk_id AND r.tenant_id = ?", tenantID).
		Where("h.risk_id = ?", riskID)
	if before != nil {
		q = q.Where("h.created_at <= ?", *before)
	}

	var rows []struct {
		ID         string
		Score      float64
		Status     string
		ChangeType string
		ChangedBy  string
		CreatedAt  time.Time
	}
	if err := q.Order("h.created_at DESC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]entity.TimelineEvent, 0, len(rows))
	for _, row := range rows {
		ev := entity.TimelineEvent{
			// Prefixed so a history id can never collide with an audit event id
			// in the merged stream's cursor.
			ID:         "risk_history:" + row.ID,
			Kind:       normaliseChangeType(row.ChangeType),
			OccurredAt: row.CreatedAt,
			Target:     entity.Ref{Type: entity.TypeRisk, ID: entityID},
			TargetURL:  entity.DeepLink(entity.TypeRisk, entityID),
			Source:     entity.SourceRiskHistory,
			Summary:    fmt.Sprintf("Score recorded at %s (status %s)", strconv.FormatFloat(row.Score, 'f', 2, 64), row.Status),
		}
		// ChangedBy is a user id OR the literal "System" for worker-made
		// changes, so it is parsed rather than assumed to be an id.
		if id, err := uuid.Parse(row.ChangedBy); err == nil && id != uuid.Nil {
			ev.Actor = &entity.Actor{ID: id.String()}
		} else if row.ChangedBy != "" {
			ev.Actor = &entity.Actor{Label: row.ChangedBy}
		}
		out = append(out, ev)
	}
	return out, nil
}

// normaliseChangeType maps the history's verbs onto the timeline's.
func normaliseChangeType(s string) string {
	switch s {
	case "CREATE", "create":
		return "create"
	case "MITIGATE", "mitigate":
		return "mitigate"
	case "":
		return "update"
	default:
		return "update"
	}
}

// =============================================================================
// Incident journal
// =============================================================================

type IncidentTimelineSource struct{ db *gorm.DB }

func NewIncidentTimelineSource(db *gorm.DB) *IncidentTimelineSource {
	return &IncidentTimelineSource{db: db}
}

var _ entity.EventSource = (*IncidentTimelineSource)(nil)

func (s *IncidentTimelineSource) Source() entity.TimelineSource { return entity.SourceIncident }

func (s *IncidentTimelineSource) Events(ctx context.Context, tenantID uuid.UUID, entityID string, before *time.Time, limit int) ([]entity.TimelineEvent, error) {
	n, err := strconv.ParseUint(entityID, 10, 64)
	if err != nil {
		return nil, nil
	}
	// Gate through the parent incident. Incident ids are sequential integers,
	// which makes this the easiest journal in the product to enumerate.
	q := s.db.WithContext(ctx).
		Table("incident_timelines AS t").
		Select("CAST(t.id AS TEXT) AS id, t.event_type AS event_type, t.message AS message, t.created_by AS created_by, t.created_at AS created_at").
		Joins("JOIN incidents AS i ON i.id = t.incident_id AND i.tenant_id = ?", tenantID.String()).
		Where("t.incident_id = ?", n)
	if before != nil {
		q = q.Where("t.created_at <= ?", *before)
	}

	var rows []struct {
		ID        string
		EventType string
		Message   string
		CreatedBy string
		CreatedAt time.Time
	}
	if err := q.Order("t.created_at DESC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]entity.TimelineEvent, 0, len(rows))
	for _, row := range rows {
		ev := entity.TimelineEvent{
			ID:         "incident_timeline:" + row.ID,
			Kind:       incidentEventKind(row.EventType),
			OccurredAt: row.CreatedAt,
			Target:     entity.Ref{Type: entity.TypeIncident, ID: entityID},
			TargetURL:  entity.DeepLink(entity.TypeIncident, entityID),
			Source:     entity.SourceIncident,
			Summary:    row.Message,
		}
		if id, err := uuid.Parse(row.CreatedBy); err == nil && id != uuid.Nil {
			ev.Actor = &entity.Actor{ID: id.String()}
		} else if row.CreatedBy != "" {
			ev.Actor = &entity.Actor{Label: row.CreatedBy}
		}
		out = append(out, ev)
	}
	return out, nil
}

func incidentEventKind(eventType string) string {
	switch eventType {
	case "status_change":
		return "update"
	case "assignment":
		return "assign"
	case "comment":
		return "comment"
	case "action":
		return "action"
	default:
		return "update"
	}
}

// =============================================================================
// Asset snapshots
// =============================================================================

type AssetSnapshotSource struct{ db *gorm.DB }

func NewAssetSnapshotSource(db *gorm.DB) *AssetSnapshotSource { return &AssetSnapshotSource{db: db} }

var _ entity.EventSource = (*AssetSnapshotSource)(nil)

func (s *AssetSnapshotSource) Source() entity.TimelineSource { return entity.SourceAssetSnapshot }

func (s *AssetSnapshotSource) Events(ctx context.Context, tenantID uuid.UUID, entityID string, before *time.Time, limit int) ([]entity.TimelineEvent, error) {
	assetID, err := uuid.Parse(entityID)
	if err != nil {
		return nil, nil
	}
	// asset_snapshots carries its own tenant column, so this one is filtered
	// directly rather than through a parent join.
	q := s.db.WithContext(ctx).
		Table("asset_snapshots").
		Select("CAST(id AS TEXT) AS id, name, criticality, reason, CAST(changed_by AS TEXT) AS changed_by, created_at").
		Where("tenant_id = ? AND asset_id = ?", tenantID, assetID)
	if before != nil {
		q = q.Where("created_at <= ?", *before)
	}

	var rows []struct {
		ID          string
		Name        string
		Criticality string
		Reason      string
		ChangedBy   string
		CreatedAt   time.Time
	}
	if err := q.Order("created_at DESC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]entity.TimelineEvent, 0, len(rows))
	for _, row := range rows {
		kind := "update"
		if row.Reason == "delete" {
			kind = "delete"
		}
		ev := entity.TimelineEvent{
			ID:         "asset_snapshot:" + row.ID,
			Kind:       kind,
			OccurredAt: row.CreatedAt,
			Target:     entity.Ref{Type: entity.TypeAsset, ID: entityID},
			TargetURL:  entity.DeepLink(entity.TypeAsset, entityID),
			Source:     entity.SourceAssetSnapshot,
			Summary:    fmt.Sprintf("State recorded: %s (criticality %s)", row.Name, row.Criticality),
		}
		if id, err := uuid.Parse(row.ChangedBy); err == nil && id != uuid.Nil {
			ev.Actor = &entity.Actor{ID: id.String()}
		}
		out = append(out, ev)
	}
	return out, nil
}
