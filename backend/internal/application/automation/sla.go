// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package automation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/rs/zerolog"
)

// SLATrackerView is a tracker plus the fields the dashboard computes on read
// (remaining budget and whether it is currently past due).
type SLATrackerView struct {
	domain.SLATracker
	RemainingMinutes int  `json:"remaining_minutes"`
	BreachedNow      bool `json:"breached_now"`
}

// RiskStateLookup reports whether a risk has reached a resolved state
// (mitigated/closed/accepted). Used by the auto-close sweep.
type RiskStateLookup interface {
	IsRiskResolved(ctx context.Context, tenantID, riskID uuid.UUID) (bool, error)
}

// SLAService backs the SLA dashboard and the escalation + auto-close sweeps the
// SLAMonitor runs on a cadence. The Notifier and RiskStateLookup are optional
// (nil-safe): without the Notifier escalations are recorded but not delivered;
// without the lookup the auto-close sweep is a no-op.
type SLAService struct {
	repo     domain.SLATrackerRepository
	notifier Notifier
	risks    RiskStateLookup
	logger   zerolog.Logger
}

// NewSLAService builds the SLA service.
func NewSLAService(repo domain.SLATrackerRepository, logger zerolog.Logger) *SLAService {
	return &SLAService{repo: repo, logger: logger}
}

// WithNotifier attaches the alert dispatcher used for escalation notices.
func (s *SLAService) WithNotifier(n Notifier) *SLAService { s.notifier = n; return s }

// WithRiskLookup attaches the risk-state lookup used by the auto-close sweep.
func (s *SLAService) WithRiskLookup(l RiskStateLookup) *SLAService { s.risks = l; return s }

// ListOpen returns the tenant's live SLA countdowns with computed fields.
func (s *SLAService) ListOpen(ctx context.Context, tenantID uuid.UUID) ([]SLATrackerView, error) {
	trackers, err := s.repo.ListOpen(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]SLATrackerView, 0, len(trackers))
	for i := range trackers {
		t := trackers[i]
		out = append(out, SLATrackerView{
			SLATracker:       t,
			RemainingMinutes: t.RemainingMinutes(now),
			BreachedNow:      t.IsBreached(now),
		})
	}
	return out, nil
}

// Stats returns the SLA summary for the dashboard, adding the at-risk count
// (open trackers within 25% of their remaining budget or already breached).
func (s *SLAService) Stats(ctx context.Context, tenantID uuid.UUID) (domain.SLAStats, error) {
	stats, err := s.repo.Stats(ctx, tenantID)
	if err != nil {
		return domain.SLAStats{}, err
	}
	trackers, err := s.repo.ListOpen(ctx, tenantID)
	if err != nil {
		return stats, err
	}
	now := time.Now()
	var atRisk int64
	for i := range trackers {
		t := trackers[i]
		if t.Status != domain.SLAOpen {
			continue
		}
		remaining := t.DueAt.Sub(now)
		total := t.DueAt.Sub(t.CreatedAt)
		if remaining <= 0 || (total > 0 && remaining <= total/4) {
			atRisk++
		}
	}
	stats.AtRisk = atRisk
	return stats, nil
}

// SweepEscalations is the SLAMonitor's core: escalate every still-open tracker
// whose escalation window has elapsed. Runs cross-tenant; each row carries its
// tenant. Idempotent within a tick — a tracker is escalated once per level.
func (s *SLAService) SweepEscalations(ctx context.Context, now time.Time) (int, error) {
	breaching, err := s.repo.ListBreaching(ctx, now)
	if err != nil {
		return 0, err
	}
	escalated := 0
	for i := range breaching {
		t := breaching[i]
		if s.escalate(ctx, &t, now) {
			escalated++
		}
	}
	return escalated, nil
}

func (s *SLAService) escalate(ctx context.Context, t *domain.SLATracker, now time.Time) bool {
	// Raise the escalation notice (best-effort) before flipping state so a
	// delivery failure doesn't permanently swallow the alert on retry.
	if s.notifier != nil {
		channels := []string(t.EscalateChannels)
		if len(channels) == 0 {
			channels = []string{"in_app", "email"}
		}
		overdueMin := int(now.Sub(t.DueAt).Minutes())
		subject := fmt.Sprintf("SLA breached — %s", firstNonEmpty(t.Title, t.SubjectID))
		message := fmt.Sprintf(
			"The remediation SLA for %q (%s) is overdue by %d minutes and requires immediate attention.",
			firstNonEmpty(t.Title, t.SubjectID), t.Severity, overdueMin)
		if _, err := s.notifier.Notify(ctx, NotifyRequest{
			TenantID:     t.TenantID,
			Channels:     channels,
			Severity:     firstNonEmpty(t.Severity, "high"),
			Subject:      subject,
			Message:      message,
			TargetRole:   firstNonEmpty(t.EscalateToRole, "admin"),
			OwnerID:      t.OwnerID,
			ResourceID:   t.RiskID,
			ResourceType: "risk",
		}); err != nil {
			s.logger.Warn().Err(err).Str("tracker", t.ID.String()).Msg("sla: escalation notify failed")
		}
	}
	t.Status = domain.SLAEscalated
	t.EscalationLevel++
	t.EscalatedAt = &now
	// Push the next escalation window out one hour so a still-open breach is
	// re-escalated on a cadence rather than every tick.
	next := now.Add(time.Hour)
	t.EscalateAt = &next
	if err := s.repo.Update(ctx, t); err != nil {
		s.logger.Warn().Err(err).Str("tracker", t.ID.String()).Msg("sla: could not persist escalation")
		return false
	}
	s.logger.Info().
		Str("tracker", t.ID.String()).
		Str("tenant", t.TenantID.String()).
		Int("level", t.EscalationLevel).
		Msg("sla: escalated overdue remediation")
	return true
}

// SweepAutoClose closes SLA trackers whose linked risk is now resolved — the
// automatic-closure half of the workflow (spec §10 step 8). Runs cross-tenant.
func (s *SLAService) SweepAutoClose(ctx context.Context) (int, error) {
	if s.risks == nil {
		return 0, nil
	}
	trackers, err := s.repo.ListOpenLinkedToRisk(ctx)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	closed := 0
	for i := range trackers {
		t := trackers[i]
		if t.RiskID == nil {
			continue
		}
		resolved, err := s.risks.IsRiskResolved(ctx, t.TenantID, *t.RiskID)
		if err != nil || !resolved {
			continue
		}
		if now.After(t.DueAt) {
			t.Status = domain.SLAClosed
		} else {
			t.Status = domain.SLAMet
		}
		t.ClosedAt = &now
		if err := s.repo.Update(ctx, &t); err != nil {
			s.logger.Warn().Err(err).Str("tracker", t.ID.String()).Msg("sla: auto-close update failed")
			continue
		}
		closed++
		s.logger.Info().Str("tracker", t.ID.String()).Str("risk", t.RiskID.String()).Msg("sla: auto-closed on risk resolution")
	}
	return closed, nil
}

// CloseForResolvedRisk marks a risk's open trackers as met (if within budget)
// or closed (if already breached). Called when a risk is resolved — the
// automatic-closure half of the workflow.
func (s *SLAService) CloseForResolvedRisk(ctx context.Context, tenantID, riskID uuid.UUID) (int, error) {
	trackers, err := s.repo.ListOpenByRisk(ctx, tenantID, riskID)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	closed := 0
	for i := range trackers {
		t := trackers[i]
		if now.After(t.DueAt) {
			t.Status = domain.SLAClosed
		} else {
			t.Status = domain.SLAMet
		}
		t.ClosedAt = &now
		if err := s.repo.Update(ctx, &t); err != nil {
			s.logger.Warn().Err(err).Str("tracker", t.ID.String()).Msg("sla: could not close on risk resolution")
			continue
		}
		closed++
	}
	return closed, nil
}
