// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1

package automation

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/rs/zerolog"
)

func TestSLA_SweepEscalations(t *testing.T) {
	repo := newMockSLARepo()
	notifier := &mockNotifier{}
	svc := NewSLAService(repo, zerolog.Nop()).WithNotifier(notifier)
	tenant := uuid.New()

	now := time.Now()
	past := now.Add(-30 * time.Minute)
	due := now.Add(-90 * time.Minute)
	// Overdue tracker whose escalation window has elapsed.
	_ = repo.Create(context.Background(), &domain.SLATracker{
		ID: uuid.New(), TenantID: tenant, Severity: "critical",
		Status: domain.SLAOpen, DueAt: due, EscalateAt: &past, EscalateToRole: "admin",
		Title: "Log4Shell", CreatedAt: due.Add(-2 * time.Hour),
	})
	// Not-yet-due tracker must NOT escalate.
	future := now.Add(2 * time.Hour)
	_ = repo.Create(context.Background(), &domain.SLATracker{
		ID: uuid.New(), TenantID: tenant, Severity: "high",
		Status: domain.SLAOpen, DueAt: future, EscalateAt: &future,
	})

	n, err := svc.SweepEscalations(context.Background(), now)
	if err != nil {
		t.Fatalf("SweepEscalations: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 escalation, got %d", n)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("expected 1 escalation notice, got %d", len(notifier.calls))
	}
	// The escalated tracker's status must be updated.
	escalated := 0
	for _, tr := range repo.trackers {
		if tr.Status == domain.SLAEscalated {
			escalated++
			if tr.EscalationLevel != 1 || tr.EscalatedAt == nil {
				t.Fatalf("escalation bookkeeping wrong: %+v", tr)
			}
		}
	}
	if escalated != 1 {
		t.Fatalf("expected 1 escalated tracker, got %d", escalated)
	}
}

func TestSLA_SweepAutoClose(t *testing.T) {
	repo := newMockSLARepo()
	resolver := &mockRiskResolver{resolved: true}
	svc := NewSLAService(repo, zerolog.Nop()).WithRiskLookup(resolver)
	tenant := uuid.New()
	riskID := uuid.New()

	now := time.Now()
	// Within budget → should become "met".
	_ = repo.Create(context.Background(), &domain.SLATracker{
		ID: uuid.New(), TenantID: tenant, RiskID: &riskID, Severity: "high",
		Status: domain.SLAOpen, DueAt: now.Add(time.Hour),
	})

	n, err := svc.SweepAutoClose(context.Background())
	if err != nil {
		t.Fatalf("SweepAutoClose: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 auto-close, got %d", n)
	}
	for _, tr := range repo.trackers {
		if tr.Status != domain.SLAMet || tr.ClosedAt == nil {
			t.Fatalf("expected met+closed, got %+v", tr)
		}
	}
}

func TestSLA_SweepAutoClose_NoopWhenUnresolved(t *testing.T) {
	repo := newMockSLARepo()
	resolver := &mockRiskResolver{resolved: false}
	svc := NewSLAService(repo, zerolog.Nop()).WithRiskLookup(resolver)
	tenant := uuid.New()
	riskID := uuid.New()
	_ = repo.Create(context.Background(), &domain.SLATracker{
		ID: uuid.New(), TenantID: tenant, RiskID: &riskID, Status: domain.SLAOpen, DueAt: time.Now().Add(time.Hour),
	})
	n, err := svc.SweepAutoClose(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("expected no auto-close (n=%d, err=%v)", n, err)
	}
}

func TestSLA_StatsAtRisk(t *testing.T) {
	repo := newMockSLARepo()
	svc := NewSLAService(repo, zerolog.Nop())
	tenant := uuid.New()
	now := time.Now()

	// Healthy: plenty of budget left.
	_ = repo.Create(context.Background(), &domain.SLATracker{
		ID: uuid.New(), TenantID: tenant, Status: domain.SLAOpen,
		CreatedAt: now.Add(-10 * time.Minute), DueAt: now.Add(4 * time.Hour),
	})
	// At risk: within the last 25% of its budget.
	_ = repo.Create(context.Background(), &domain.SLATracker{
		ID: uuid.New(), TenantID: tenant, Status: domain.SLAOpen,
		CreatedAt: now.Add(-100 * time.Minute), DueAt: now.Add(5 * time.Minute),
	})

	stats, err := svc.Stats(context.Background(), tenant)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.Open != 2 {
		t.Fatalf("expected 2 open, got %d", stats.Open)
	}
	if stats.AtRisk != 1 {
		t.Fatalf("expected 1 at-risk, got %d", stats.AtRisk)
	}
}

func TestSLAConfig_MinutesForFallback(t *testing.T) {
	cfg := domain.AutomationSLAConfig{CriticalMinutes: 240}
	if got := cfg.MinutesFor("high"); got != 240 {
		t.Fatalf("high should fall back to critical 240, got %d", got)
	}
	if got := cfg.MinutesFor("low"); got != 240 {
		t.Fatalf("low should fall back to critical 240, got %d", got)
	}
	cfg2 := domain.AutomationSLAConfig{CriticalMinutes: 60, HighMinutes: 480}
	if got := cfg2.MinutesFor("high"); got != 480 {
		t.Fatalf("high should use its own 480, got %d", got)
	}
}
