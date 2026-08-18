// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package billing

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	pkgbilling "github.com/opendefender/openrisk/pkg/billing"
)

type memStore struct {
	sub  *domain.Subscription
	plan string
}

func (m *memStore) GetByTenant(context.Context, uuid.UUID) (*domain.Subscription, error) {
	return m.sub, nil
}
func (m *memStore) Upsert(_ context.Context, s *domain.Subscription) error { m.sub = s; return nil }
func (m *memStore) SetOrganizationPlanAndRegion(_ context.Context, _ uuid.UUID, plan, _ string) error {
	m.plan = plan
	return nil
}
func (m *memStore) ListInvoices(context.Context, uuid.UUID) ([]domain.Invoice, error) { return nil, nil }

func TestStartTrial(t *testing.T) {
	m := &memStore{}
	svc := NewService(m, pkgbilling.NewRegistry())
	sub, err := svc.StartTrial(context.Background(), uuid.New(), "pro", "africa")
	if err != nil {
		t.Fatal(err)
	}
	if sub.Status != domain.SubTrialing || sub.TrialEndsAt == nil {
		t.Fatal("expected trialing with an end date")
	}
	if m.plan != "pro" {
		t.Fatalf("org plan should be pro, got %s", m.plan)
	}
	if sub.TrialEndsAt.Before(time.Now().Add(13 * 24 * time.Hour)) {
		t.Fatal("trial should be ~14 days")
	}
}

func TestStartTrial_RejectsFreeAndDouble(t *testing.T) {
	m := &memStore{}
	svc := NewService(m, pkgbilling.NewRegistry())
	if _, err := svc.StartTrial(context.Background(), uuid.New(), "free", "eu"); err != ErrInvalidPlan {
		t.Fatalf("free trial should be rejected, got %v", err)
	}
	end := time.Now().Add(time.Hour)
	m.sub = &domain.Subscription{Status: domain.SubTrialing, TrialEndsAt: &end, Plan: "pro"}
	if _, err := svc.StartTrial(context.Background(), uuid.New(), "business", "eu"); err != ErrAlreadySubscribed {
		t.Fatalf("double subscribe should be rejected, got %v", err)
	}
}

func TestApplyPlanAndCancel(t *testing.T) {
	m := &memStore{}
	svc := NewService(m, pkgbilling.NewRegistry())
	tenant := uuid.New()

	if _, err := svc.ApplyPlan(context.Background(), tenant, "business", "eu", domain.ProviderStripe, "sub_1"); err != nil {
		t.Fatal(err)
	}
	if m.sub.Status != domain.SubActive || m.plan != "business" {
		t.Fatalf("apply should activate business, got %s/%s", m.sub.Status, m.plan)
	}

	// Cancel an active sub → cancel at period end (keeps access), org plan unchanged.
	if _, err := svc.Cancel(context.Background(), tenant); err != nil {
		t.Fatal(err)
	}
	if !m.sub.CancelAtPeriodEnd {
		t.Fatal("active cancel should set cancel_at_period_end")
	}
	if m.plan != "business" {
		t.Fatal("active cancel keeps plan until period end")
	}
}

func TestCheckout_NoGateway(t *testing.T) {
	svc := NewService(&memStore{}, pkgbilling.NewRegistry()).WithBaseURL("https://app")
	if _, err := svc.Checkout(context.Background(), uuid.New(), "pro", "eu", "a@b.co", ""); err != ErrNoGateway {
		t.Fatalf("expected ErrNoGateway, got %v", err)
	}
}
