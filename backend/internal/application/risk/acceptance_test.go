// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package risk

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// #811 — residual-risk acceptance is dated, approved and renewed only through
// a new governance decision.

var acceptNow = time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)

// approvalsSince is a checker that honours `since`, like the real adapter: an
// approval decided on or before the previous acceptance does not count.
type approvalsSince struct {
	approvals []domain.AcceptanceApproval
	gotSince  *time.Time
}

func (a *approvalsSince) AcceptanceApproval(_ context.Context, _, _ uuid.UUID, since *time.Time) (*domain.AcceptanceApproval, *uuid.UUID, error) {
	a.gotSince = since
	var best *domain.AcceptanceApproval
	for i := range a.approvals {
		ap := a.approvals[i]
		if since != nil && !ap.ApprovedAt.After(*since) {
			continue
		}
		if best == nil || ap.ApprovedAt.After(best.ApprovedAt) {
			best = &ap
		}
	}
	return best, nil, nil
}

type fixedPolicy struct {
	months int
	err    error
}

func (p fixedPolicy) MaxAcceptanceMonths(context.Context, uuid.UUID) (int, error) {
	return p.months, p.err
}

type memJournal struct{ events []*domain.AuditEvent }

func (j *memJournal) Append(_ context.Context, e *domain.AuditEvent) error {
	j.events = append(j.events, e)
	return nil
}

func approvalAt(t time.Time) (domain.AcceptanceApproval, uuid.UUID) {
	approver := uuid.New()
	return domain.AcceptanceApproval{RequestID: uuid.New(), ApproverID: &approver, ApprovedAt: t}, approver
}

func acceptUC(repo *stateRepo, ap ApprovalChecker, pol AcceptancePolicy, j AuditJournal) *TransitionRiskStateUseCase {
	return NewTransitionRiskStateUseCase(repo).
		WithMitigations(&fakeInspector{plans: []MitigationSnapshot{donePlan()}}).
		WithApprovals(ap).
		WithAcceptancePolicy(pol).
		WithJournal(j).
		WithClock(func() time.Time { return acceptNow })
}

func TestAcceptResidual_Success(t *testing.T) {
	repo, tenant, id := newFixture(domain.StateInTreatment)
	ap, approver := approvalAt(acceptNow.Add(-time.Hour))
	j := &memJournal{}
	uc := acceptUC(repo, &approvalsSince{approvals: []domain.AcceptanceApproval{ap}}, nil, j)

	got, err := uc.Execute(context.Background(), tenant, id, TransitionRiskStateInput{To: domain.StateResidualAccepted, Actor: uuid.New()})
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	want := acceptNow.AddDate(0, 12, 0)
	if got.AcceptanceExpiresAt == nil || !got.AcceptanceExpiresAt.Equal(want) {
		t.Fatalf("default expiry = %v, want 12 months (%v)", got.AcceptanceExpiresAt, want)
	}
	if got.AcceptanceApprovedBy == nil || *got.AcceptanceApprovedBy != approver {
		t.Fatal("the approver must be recorded on the risk")
	}
	if got.AcceptanceApprovalID == nil || *got.AcceptanceApprovalID != ap.RequestID {
		t.Fatal("the governance request must be recorded on the risk")
	}
	if got.AcceptedAt == nil || !got.AcceptedAt.Equal(acceptNow) {
		t.Fatalf("accepted_at = %v", got.AcceptedAt)
	}

	if len(j.events) != 1 {
		t.Fatalf("chained trail got %d events, want 1", len(j.events))
	}
	after := j.events[0].After
	for _, k := range []string{"acceptance_expires_at", "acceptance_approved_by", "acceptance_approval_id", "accepted_at"} {
		if _, ok := after[k]; !ok {
			t.Fatalf("trail entry lacks %q: %v", k, after)
		}
	}
	if j.events[0].TenantID != tenant || j.events[0].EntityID != id.String() {
		t.Fatal("trail entry addressed to the wrong tenant or risk")
	}
}

func TestAcceptResidual_NotFound(t *testing.T) {
	repo, tenant, _ := newFixture(domain.StateInTreatment)
	ap, _ := approvalAt(acceptNow.Add(-time.Hour))
	uc := acceptUC(repo, &approvalsSince{approvals: []domain.AcceptanceApproval{ap}}, nil, nil)
	_, err := uc.Execute(context.Background(), tenant, uuid.New(), TransitionRiskStateInput{To: domain.StateResidualAccepted})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

// Another tenant's risk reads as not found, never as forbidden, and is left
// untouched.
func TestAcceptResidual_Unauthorized(t *testing.T) {
	repo, _, id := newFixture(domain.StateInTreatment)
	ap, _ := approvalAt(acceptNow.Add(-time.Hour))
	uc := acceptUC(repo, &approvalsSince{approvals: []domain.AcceptanceApproval{ap}}, nil, nil)
	_, err := uc.Execute(context.Background(), uuid.New(), id, TransitionRiskStateInput{To: domain.StateResidualAccepted})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant accept must read as not found, got %v", err)
	}
	if repo.updated != nil {
		t.Fatal("a cross-tenant call wrote to the risk")
	}
}

func TestAcceptResidual_EndDateIsBoundedByTheTenantMaximum(t *testing.T) {
	ap, _ := approvalAt(acceptNow.Add(-time.Hour))
	cases := []struct {
		name    string
		max     int
		at      *time.Time
		wantErr bool
		want    time.Time
	}{
		{"default under a 6-month cap", 6, nil, false, acceptNow.AddDate(0, 6, 0)},
		{"explicit inside the cap", 24, ptrTime(acceptNow.AddDate(0, 18, 0)), false, acceptNow.AddDate(0, 18, 0)},
		{"last day of the cap", 12, ptrTime(time.Date(2027, 10, 15, 0, 0, 0, 0, time.UTC)), false, time.Date(2027, 10, 15, 0, 0, 0, 0, time.UTC)},
		{"past the cap", 12, ptrTime(acceptNow.AddDate(0, 13, 0)), true, time.Time{}},
		{"in the past", 12, ptrTime(acceptNow.Add(-time.Hour)), true, time.Time{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, tenant, id := newFixture(domain.StateInTreatment)
			uc := acceptUC(repo, &approvalsSince{approvals: []domain.AcceptanceApproval{ap}}, fixedPolicy{months: tc.max}, nil)
			got, err := uc.Execute(context.Background(), tenant, id, TransitionRiskStateInput{
				To: domain.StateResidualAccepted, AcceptanceExpiresAt: tc.at,
			})
			if tc.wantErr {
				if !errors.Is(err, domain.ErrValidation) {
					t.Fatalf("want ErrValidation, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !got.AcceptanceExpiresAt.Equal(tc.want) {
				t.Fatalf("expiry = %v, want %v", got.AcceptanceExpiresAt, tc.want)
			}
		})
	}
}

// After an expiry the risk is re-assessed and treated again; accepting it a
// second time must not ride on the approval the first acceptance spent.
func TestAcceptResidual_SpentApprovalIsRefused(t *testing.T) {
	repo, tenant, id := newFixture(domain.StateInTreatment)
	firstAccepted := acceptNow.AddDate(-1, 0, 0)
	repo.risk.AcceptedAt = &firstAccepted
	old, _ := approvalAt(firstAccepted.Add(-time.Hour))
	checker := &approvalsSince{approvals: []domain.AcceptanceApproval{old}}
	uc := acceptUC(repo, checker, nil, nil)

	_, err := uc.Execute(context.Background(), tenant, id, TransitionRiskStateInput{To: domain.StateResidualAccepted})
	if err == nil || !strings.Contains(err.Error(), "nouvelle approbation") {
		t.Fatalf("a spent approval must be refused with a reason, got %v", err)
	}
	if checker.gotSince == nil || !checker.gotSince.Equal(firstAccepted) {
		t.Fatalf("the guard must ask for approvals after the previous acceptance, asked since=%v", checker.gotSince)
	}

	fresh, _ := approvalAt(acceptNow.Add(-time.Minute))
	checker.approvals = append(checker.approvals, fresh)
	got, err := uc.Execute(context.Background(), tenant, id, TransitionRiskStateInput{To: domain.StateResidualAccepted})
	if err != nil {
		t.Fatalf("a fresh approval must be accepted: %v", err)
	}
	if *got.AcceptanceApprovalID != fresh.RequestID {
		t.Fatal("the fresh request must be the one recorded")
	}
}

func TestRenewAcceptance_Success(t *testing.T) {
	repo, tenant, id := newFixture(domain.StateResidualAccepted)
	began := acceptNow.AddDate(0, -11, 0)
	ends := acceptNow.AddDate(0, 0, 20)
	reminded := acceptNow.AddDate(0, 0, -10)
	repo.risk.AcceptedAt, repo.risk.AcceptanceExpiresAt, repo.risk.AcceptanceReminder30At = &began, &ends, &reminded

	fresh, approver := approvalAt(acceptNow.Add(-time.Hour))
	j := &memJournal{}
	uc := acceptUC(repo, &approvalsSince{approvals: []domain.AcceptanceApproval{fresh}}, nil, j)
	got, err := uc.RenewAcceptance(context.Background(), tenant, id, RenewAcceptanceInput{
		AcceptanceExpiresAt: ptrTime(acceptNow.AddDate(0, 9, 0)),
	})
	if err != nil {
		t.Fatalf("renew: %v", err)
	}
	if !got.AcceptanceExpiresAt.Equal(acceptNow.AddDate(0, 9, 0)) || *got.AcceptanceApprovedBy != approver {
		t.Fatalf("renewal did not record the new end date and approver: %v", got.AcceptanceExpiresAt)
	}
	if got.AcceptanceReminder30At != nil {
		t.Fatal("a renewal must re-arm the reminders")
	}
	if got.State() != domain.StateResidualAccepted {
		t.Fatalf("a renewal does not change the state, got %s", got.State())
	}
	if len(j.events) != 1 || j.events[0].Before["acceptance_expires_at"] == nil {
		t.Fatal("the renewal must be journalled with the previous end date")
	}
}

// Nobody can simply push the date: without a NEW approval the renewal fails.
func TestRenewAcceptance_RequiresANewApproval(t *testing.T) {
	repo, tenant, id := newFixture(domain.StateResidualAccepted)
	began := acceptNow.AddDate(0, -11, 0)
	ends := acceptNow.AddDate(0, 0, 20)
	repo.risk.AcceptedAt, repo.risk.AcceptanceExpiresAt = &began, &ends
	spent, _ := approvalAt(began.Add(-time.Hour))

	uc := acceptUC(repo, &approvalsSince{approvals: []domain.AcceptanceApproval{spent}}, nil, nil)
	_, err := uc.RenewAcceptance(context.Background(), tenant, id, RenewAcceptanceInput{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
	if repo.updated != nil {
		t.Fatal("a refused renewal wrote to the risk")
	}
}

func TestRenewAcceptance_NotFound(t *testing.T) {
	repo, tenant, _ := newFixture(domain.StateResidualAccepted)
	uc := acceptUC(repo, &approvalsSince{}, nil, nil)
	if _, err := uc.RenewAcceptance(context.Background(), tenant, uuid.New(), RenewAcceptanceInput{}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestRenewAcceptance_Unauthorized(t *testing.T) {
	repo, _, id := newFixture(domain.StateResidualAccepted)
	fresh, _ := approvalAt(acceptNow)
	uc := acceptUC(repo, &approvalsSince{approvals: []domain.AcceptanceApproval{fresh}}, nil, nil)
	if _, err := uc.RenewAcceptance(context.Background(), uuid.New(), id, RenewAcceptanceInput{}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant renewal must read as not found, got %v", err)
	}
}

func TestRenewAcceptance_OnlyAnAcceptedRisk(t *testing.T) {
	repo, tenant, id := newFixture(domain.StateAssessed)
	fresh, _ := approvalAt(acceptNow)
	uc := acceptUC(repo, &approvalsSince{approvals: []domain.AcceptanceApproval{fresh}}, nil, nil)
	if _, err := uc.RenewAcceptance(context.Background(), tenant, id, RenewAcceptanceInput{}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

// The other two doors into an accepted state are closed.
func TestCreateRisk_CannotBeBornAccepted(t *testing.T) {
	uc := NewCreateRiskUseCase(&stateRepo{})
	_, err := uc.Execute(context.Background(), uuid.New(), CreateRiskInput{
		Title: "x", Probability: 0.5, Impact: 5, Status: domain.RiskAccepted, CreatedBy: uuid.New(),
	})
	if !errors.Is(err, domain.ErrValidation) || !strings.Contains(err.Error(), "created as accepted") {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestBulkAction_CannotAcceptInBulk(t *testing.T) {
	status := domain.RiskAccepted
	uc := NewBulkActionUseCase(&stateRepo{}, &memJournal{})
	_, err := uc.Execute(context.Background(), uuid.New(), BulkActionRequest{
		Type: BulkActionChangeStatus, RiskIDs: []uuid.UUID{uuid.New()}, Status: &status,
	}, uuid.New())
	if !errors.Is(err, domain.ErrValidation) || !strings.Contains(err.Error(), "in bulk") {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tenant policy
// ---------------------------------------------------------------------------

type memPolicyStore struct{ byTenant map[uuid.UUID]int }

func (m *memPolicyStore) MaxAcceptanceMonths(_ context.Context, t uuid.UUID) (int, error) {
	return domain.NormalizeAcceptanceMax(m.byTenant[t]), nil
}
func (m *memPolicyStore) SetMaxAcceptanceMonths(_ context.Context, t uuid.UUID, months int) error {
	m.byTenant[t] = months
	return nil
}

func TestAcceptancePolicy_Success(t *testing.T) {
	store := &memPolicyStore{byTenant: map[uuid.UUID]int{}}
	uc := NewAcceptancePolicyUseCase(store)
	a, b := uuid.New(), uuid.New()
	if _, err := uc.Set(context.Background(), a, 6); err != nil {
		t.Fatal(err)
	}
	gotA, _ := uc.Get(context.Background(), a)
	gotB, _ := uc.Get(context.Background(), b)
	if gotA.MaxAcceptanceMonths != 6 || gotB.MaxAcceptanceMonths != domain.DefaultAcceptanceMonths {
		t.Fatalf("a=%d b=%d: one tenant's policy leaked into another", gotA.MaxAcceptanceMonths, gotB.MaxAcceptanceMonths)
	}
	for _, bad := range []int{0, -1, domain.AcceptanceMonthsCeiling + 1} {
		if _, err := uc.Set(context.Background(), a, bad); !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("%d months: want ErrValidation, got %v", bad, err)
		}
	}
}

func TestAcceptancePolicy_Unauthorized(t *testing.T) {
	uc := NewAcceptancePolicyUseCase(&memPolicyStore{byTenant: map[uuid.UUID]int{}})
	if _, err := uc.Get(context.Background(), uuid.Nil); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
	if _, err := uc.Set(context.Background(), uuid.Nil, 6); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
