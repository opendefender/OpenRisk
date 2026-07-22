// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package automation

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// ---- in-memory rule repo ----

type mockRuleRepo struct {
	rules       map[uuid.UUID]*domain.AutomationRule
	triggeredID uuid.UUID
}

func newMockRuleRepo() *mockRuleRepo { return &mockRuleRepo{rules: map[uuid.UUID]*domain.AutomationRule{}} }

func (m *mockRuleRepo) add(r *domain.AutomationRule) { m.rules[r.ID] = r }

func (m *mockRuleRepo) Create(_ context.Context, r *domain.AutomationRule) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	m.rules[r.ID] = r
	return nil
}
func (m *mockRuleRepo) Update(_ context.Context, r *domain.AutomationRule) error {
	if _, ok := m.rules[r.ID]; !ok {
		return domain.NewNotFoundError("automation rule", r.ID)
	}
	m.rules[r.ID] = r
	return nil
}
func (m *mockRuleRepo) GetByID(_ context.Context, id, tenantID uuid.UUID) (*domain.AutomationRule, error) {
	r, ok := m.rules[id]
	if !ok || r.TenantID != tenantID {
		return nil, domain.NewNotFoundError("automation rule", id)
	}
	return r, nil
}
func (m *mockRuleRepo) List(_ context.Context, tenantID uuid.UUID) ([]domain.AutomationRule, error) {
	var out []domain.AutomationRule
	for _, r := range m.rules {
		if r.TenantID == tenantID {
			out = append(out, *r)
		}
	}
	return out, nil
}
func (m *mockRuleRepo) ListEnabledByTrigger(_ context.Context, tenantID uuid.UUID, trigger domain.AutomationTrigger) ([]domain.AutomationRule, error) {
	var out []domain.AutomationRule
	for _, r := range m.rules {
		if r.TenantID == tenantID && r.Enabled && r.Trigger == trigger {
			out = append(out, *r)
		}
	}
	return out, nil
}
func (m *mockRuleRepo) Delete(_ context.Context, id, tenantID uuid.UUID) error {
	if r, ok := m.rules[id]; !ok || r.TenantID != tenantID {
		return domain.NewNotFoundError("automation rule", id)
	}
	delete(m.rules, id)
	return nil
}
func (m *mockRuleRepo) RecordTriggered(_ context.Context, id, _ uuid.UUID, _ time.Time) error {
	m.triggeredID = id
	return nil
}

// ---- in-memory execution repo ----

type mockExecRepo struct{ execs map[uuid.UUID]*domain.AutomationExecution }

func newMockExecRepo() *mockExecRepo {
	return &mockExecRepo{execs: map[uuid.UUID]*domain.AutomationExecution{}}
}

func (m *mockExecRepo) Create(_ context.Context, e *domain.AutomationExecution) error {
	m.execs[e.ID] = e
	return nil
}
func (m *mockExecRepo) Update(_ context.Context, e *domain.AutomationExecution) error {
	m.execs[e.ID] = e
	return nil
}
func (m *mockExecRepo) GetByID(_ context.Context, id, tenantID uuid.UUID) (*domain.AutomationExecution, error) {
	e, ok := m.execs[id]
	if !ok || e.TenantID != tenantID {
		return nil, domain.NewNotFoundError("automation execution", id)
	}
	return e, nil
}
func (m *mockExecRepo) List(_ context.Context, tenantID uuid.UUID, _, _ int) ([]domain.AutomationExecution, error) {
	var out []domain.AutomationExecution
	for _, e := range m.execs {
		if e.TenantID == tenantID {
			out = append(out, *e)
		}
	}
	return out, nil
}
func (m *mockExecRepo) ListByRule(_ context.Context, ruleID, tenantID uuid.UUID, _ int) ([]domain.AutomationExecution, error) {
	var out []domain.AutomationExecution
	for _, e := range m.execs {
		if e.TenantID == tenantID && e.RuleID == ruleID {
			out = append(out, *e)
		}
	}
	return out, nil
}

// ---- in-memory sla repo ----

type mockSLARepo struct{ trackers map[uuid.UUID]*domain.SLATracker }

func newMockSLARepo() *mockSLARepo { return &mockSLARepo{trackers: map[uuid.UUID]*domain.SLATracker{}} }

func (m *mockSLARepo) Create(_ context.Context, t *domain.SLATracker) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	m.trackers[t.ID] = t
	return nil
}
func (m *mockSLARepo) Update(_ context.Context, t *domain.SLATracker) error {
	if _, ok := m.trackers[t.ID]; !ok {
		return domain.NewNotFoundError("sla tracker", t.ID)
	}
	m.trackers[t.ID] = t
	return nil
}
func (m *mockSLARepo) GetByID(_ context.Context, id, tenantID uuid.UUID) (*domain.SLATracker, error) {
	t, ok := m.trackers[id]
	if !ok || t.TenantID != tenantID {
		return nil, domain.NewNotFoundError("sla tracker", id)
	}
	return t, nil
}
func (m *mockSLARepo) ListOpen(_ context.Context, tenantID uuid.UUID) ([]domain.SLATracker, error) {
	var out []domain.SLATracker
	for _, t := range m.trackers {
		if t.TenantID == tenantID && (t.Status == domain.SLAOpen || t.Status == domain.SLABreached || t.Status == domain.SLAEscalated) {
			out = append(out, *t)
		}
	}
	return out, nil
}
func (m *mockSLARepo) ListBreaching(_ context.Context, now time.Time) ([]domain.SLATracker, error) {
	var out []domain.SLATracker
	for _, t := range m.trackers {
		if (t.Status == domain.SLAOpen || t.Status == domain.SLABreached) && t.EscalateAt != nil && !t.EscalateAt.After(now) {
			out = append(out, *t)
		}
	}
	return out, nil
}
func (m *mockSLARepo) ListOpenByRisk(_ context.Context, tenantID, riskID uuid.UUID) ([]domain.SLATracker, error) {
	var out []domain.SLATracker
	for _, t := range m.trackers {
		if t.TenantID == tenantID && t.RiskID != nil && *t.RiskID == riskID &&
			(t.Status == domain.SLAOpen || t.Status == domain.SLABreached || t.Status == domain.SLAEscalated) {
			out = append(out, *t)
		}
	}
	return out, nil
}
func (m *mockSLARepo) ListOpenLinkedToRisk(_ context.Context) ([]domain.SLATracker, error) {
	var out []domain.SLATracker
	for _, t := range m.trackers {
		if t.RiskID != nil && (t.Status == domain.SLAOpen || t.Status == domain.SLABreached || t.Status == domain.SLAEscalated) {
			out = append(out, *t)
		}
	}
	return out, nil
}
func (m *mockSLARepo) Stats(_ context.Context, tenantID uuid.UUID) (domain.SLAStats, error) {
	var s domain.SLAStats
	for _, t := range m.trackers {
		if t.TenantID != tenantID {
			continue
		}
		switch t.Status {
		case domain.SLAOpen:
			s.Open++
		case domain.SLABreached:
			s.Breached++
		case domain.SLAEscalated:
			s.Escalated++
		case domain.SLAMet:
			s.Met++
		case domain.SLAClosed:
			s.Closed++
		}
	}
	return s, nil
}

// ---- action port mocks ----

type mockNotifier struct {
	calls []NotifyRequest
}

func (m *mockNotifier) Notify(_ context.Context, req NotifyRequest) ([]string, error) {
	m.calls = append(m.calls, req)
	chans := req.Channels
	if len(chans) == 0 {
		chans = []string{"in_app"}
	}
	return chans, nil
}

type mockTicketer struct{ opened int }

func (m *mockTicketer) OpenTicket(_ context.Context, req TicketRequest) (TicketResult, error) {
	m.opened++
	return TicketResult{Provider: "jira", Key: "SEC-1", URL: "https://jira/SEC-1"}, nil
}

type mockRiskCreator struct {
	created int
	id      uuid.UUID
}

func (m *mockRiskCreator) EnsureRisk(_ context.Context, _ RiskRequest) (RiskResult, error) {
	m.created++
	if m.id == uuid.Nil {
		m.id = uuid.New()
	}
	return RiskResult{RiskID: m.id, Created: true}, nil
}

type mockRiskResolver struct{ resolved bool }

func (m *mockRiskResolver) IsRiskResolved(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return m.resolved, nil
}
