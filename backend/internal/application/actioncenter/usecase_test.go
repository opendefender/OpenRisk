// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package actioncenter

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// A pinned clock, so "overdue by three days" means the same thing on every run.
var testNow = time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)

func ptime(t time.Time) *time.Time { return &t }

// stubRepo returns whatever the test seeded, and RECORDS which categories were
// asked for. The recording is the point: role scoping must not merely filter the
// output, it must avoid running the query at all, and a stub that only checked
// the returned items could not tell the difference.
type stubRepo struct {
	role domain.BusinessRoleKey

	mitigations []domain.Mitigation
	risks       []domain.Risk
	approvals   []domain.ApprovalRequest
	incidents   []domain.Incident
	evidence    []domain.Evidence
	remediation []domain.RemediationPlan

	called map[string]bool
	err    error
}

func newStub() *stubRepo { return &stubRepo{called: map[string]bool{}} }

func (s *stubRepo) mark(k string) { s.called[k] = true }

func (s *stubRepo) BusinessRoleFor(_, _ uuid.UUID) (domain.BusinessRoleKey, error) {
	return s.role, s.err
}
func (s *stubRepo) OverdueMitigations(_ uuid.UUID, _ time.Time, _ int) ([]domain.Mitigation, error) {
	s.mark("mitigations")
	return s.mitigations, s.err
}
func (s *stubRepo) CriticalRisksWithoutActiveMitigation(_ uuid.UUID, _ float64, _ int) ([]domain.Risk, error) {
	s.mark("risks")
	return s.risks, s.err
}
func (s *stubRepo) PendingApprovals(_ uuid.UUID, _ int) ([]domain.ApprovalRequest, error) {
	s.mark("approvals")
	return s.approvals, s.err
}
func (s *stubRepo) OpenIncidents(_ uuid.UUID, _ int) ([]domain.Incident, error) {
	s.mark("incidents")
	return s.incidents, s.err
}
func (s *stubRepo) ExpiringEvidence(_ uuid.UUID, _ time.Time, _ int) ([]domain.Evidence, error) {
	s.mark("evidence")
	return s.evidence, s.err
}
func (s *stubRepo) OverdueRemediationPlans(_ uuid.UUID, _ time.Time, _ int) ([]domain.RemediationPlan, error) {
	s.mark("remediation")
	return s.remediation, s.err
}

var (
	tenantA = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userA   = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	otherU  = uuid.MustParse("33333333-3333-3333-3333-333333333333")
)

func caller() Caller {
	return Caller{UserID: userA, TenantID: tenantA, Roles: []string{"user"}}
}

func newUC(repo Repository) *UseCase {
	return NewUseCase(repo).WithClock(func() time.Time { return testNow })
}

// seedAll gives the tenant at least one qualifying item in every category, so a
// role test can prove both what is shown and what is withheld.
func seedAll(s *stubRepo) {
	s.mitigations = []domain.Mitigation{{
		ID: uuid.New(), TenantID: tenantA, Title: "Patch the edge proxy",
		Status: domain.MitigationInProgress, DueDate: ptime(testNow.AddDate(0, 0, -3)),
	}}
	s.risks = []domain.Risk{{
		ID: uuid.New(), TenantID: tenantA, Title: "Unpatched internet-facing host", Score: 8.4,
	}}
	s.incidents = []domain.Incident{{
		ID: 7, TenantID: tenantA.String(), Title: "Suspected exfiltration",
		Severity: "critical", Status: "open", CreatedAt: testNow.AddDate(0, 0, -1),
	}}
	s.evidence = []domain.Evidence{{
		ID: uuid.New(), TenantID: tenantA, Title: "Q2 access review",
		Review: domain.EvidenceReviewAccepted, ValidUntil: ptime(testNow.AddDate(0, 0, -5)),
	}}
	s.remediation = []domain.RemediationPlan{{
		ID: uuid.New(), TenantID: tenantA, Title: "Close the logging gap",
		Status: domain.RemediationStatusOpen, DueDate: ptime(testNow.AddDate(0, 0, -2)),
	}}
}

func types(items []ActionItem) []ItemType {
	out := make([]ItemType, 0, len(items))
	for _, i := range items {
		out = append(out, i.Type)
	}
	return out
}

// ---------------------------------------------------------------------------
// Criterion 8 — unauthenticated callers
// ---------------------------------------------------------------------------

func TestActionCenter_Unauthorized(t *testing.T) {
	uc := newUC(newStub())

	for _, tc := range []struct {
		name   string
		caller Caller
	}{
		{"no user", Caller{TenantID: tenantA}},
		{"no tenant", Caller{UserID: userA}},
		{"neither", Caller{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uc.GetActionCenter(tc.caller, 20, 0)
			require.ErrorIs(t, err, ErrUnauthorized)
		})
	}
}

// ---------------------------------------------------------------------------
// Criterion 7 — an empty Action Center is a good day, not an error
// ---------------------------------------------------------------------------

func TestActionCenter_Empty(t *testing.T) {
	s := newStub()
	s.role = domain.BusinessRoleRiskManager

	res, err := newUC(s).GetActionCenter(caller(), 20, 0)
	require.NoError(t, err)
	require.NotNil(t, res.Data, "data must marshal to [] and never to null")
	require.Len(t, res.Data, 0)
	require.Equal(t, 0, res.Total)
	require.Equal(t, testNow, res.GeneratedAt)
}

// ---------------------------------------------------------------------------
// Criterion 3 — role scoping, one sub-test per mapped role plus the default
// ---------------------------------------------------------------------------

func TestActionCenter_RoleScoping(t *testing.T) {
	for _, tc := range []struct {
		name      string
		role      domain.BusinessRoleKey
		wantTypes []ItemType
		wantCalls []string
		denyCalls []string
	}{
		{
			name: "RiskManager", role: domain.BusinessRoleRiskManager,
			wantTypes: []ItemType{ItemTypeOverdueMitigation, ItemTypeCriticalRisk},
			wantCalls: []string{"mitigations", "risks"},
			denyCalls: []string{"incidents", "evidence", "remediation"},
		},
		{
			name: "RSSI", role: domain.BusinessRoleRSSI,
			wantTypes: []ItemType{ItemTypeCriticalRisk, ItemTypeOpenIncident},
			wantCalls: []string{"risks", "incidents"},
			denyCalls: []string{"mitigations", "evidence", "remediation"},
		},
		{
			name: "Auditor", role: domain.BusinessRoleAuditor,
			wantTypes: []ItemType{ItemTypeExpiringEvidence},
			wantCalls: []string{"evidence"},
			denyCalls: []string{"mitigations", "risks", "incidents", "remediation"},
		},
		{
			name: "ComplianceOfficer", role: domain.BusinessRoleComplianceOfficer,
			wantTypes: []ItemType{ItemTypeExpiringEvidence, ItemTypeOverdueRemediation},
			wantCalls: []string{"evidence", "remediation"},
			denyCalls: []string{"mitigations", "risks", "incidents"},
		},
		{
			// The least-privilege default: an unmapped role sees nothing but its
			// own approvals, and there are none seeded here.
			name: "Executive falls back to approvals only", role: domain.BusinessRoleExecutive,
			wantTypes: []ItemType{},
			wantCalls: []string{"approvals"},
			denyCalls: []string{"mitigations", "risks", "incidents", "evidence", "remediation"},
		},
		{
			name: "no business role at all", role: "",
			wantTypes: []ItemType{},
			wantCalls: []string{"approvals"},
			denyCalls: []string{"mitigations", "risks", "incidents", "evidence", "remediation"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newStub()
			s.role = tc.role
			seedAll(s) // every category has a qualifying item for this tenant

			res, err := newUC(s).GetActionCenter(caller(), 50, 0)
			require.NoError(t, err)
			require.Equal(t, tc.wantTypes, types(res.Data))

			for _, c := range tc.wantCalls {
				require.True(t, s.called[c], "expected the %s query to run", c)
			}
			for _, c := range tc.denyCalls {
				require.False(t, s.called[c],
					"%s must not even be queried for role %q — role scoping has to gate the query, not just the output",
					c, tc.role)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Criterion 5 — approvals are per-request, never role-gated
// ---------------------------------------------------------------------------

func pendingApprovalNaming(approver uuid.UUID, requestedBy uuid.UUID, title string) domain.ApprovalRequest {
	return domain.ApprovalRequest{
		ID: uuid.New(), TenantID: tenantA, Title: title,
		Status:      domain.ApprovalPending,
		CurrentStep: 0,
		Steps: domain.WorkflowStepList{{
			Order: 0, Name: "Sign-off", MinApprovals: 1,
			ApproverUserIDs: []string{approver.String()},
		}},
		RequestedBy: requestedBy,
		CreatedAt:   testNow.AddDate(0, 0, -1),
	}
}

func TestActionCenter_ApprovalAlwaysIncluded(t *testing.T) {
	// Executive is deliberately NOT in roleCategories: if approvals were role
	// gated, this caller would see nothing.
	for _, role := range []domain.BusinessRoleKey{
		domain.BusinessRoleExecutive, domain.BusinessRoleViewer,
		domain.BusinessRoleAuditor, "",
	} {
		t.Run(string("role="+role), func(t *testing.T) {
			s := newStub()
			s.role = role
			s.approvals = []domain.ApprovalRequest{
				pendingApprovalNaming(userA, otherU, "Approve the new vendor"),
			}

			res, err := newUC(s).GetActionCenter(caller(), 20, 0)
			require.NoError(t, err)
			require.Equal(t, []ItemType{ItemTypePendingApproval}, types(res.Data))
			require.Equal(t, "Approve the new vendor", res.Data[0].Title)
		})
	}
}

func TestActionCenter_ApprovalExcludedWhenCallerIsNotTheApprover(t *testing.T) {
	s := newStub()
	s.role = domain.BusinessRoleRiskManager
	s.approvals = []domain.ApprovalRequest{
		// Named for somebody else.
		pendingApprovalNaming(otherU, otherU, "Not yours to sign"),
		// Raised by the caller: the four-eyes rule in domain.CanSign means you
		// cannot approve your own request, so this must not appear either.
		pendingApprovalNaming(userA, userA, "Your own request"),
	}

	res, err := newUC(s).GetActionCenter(caller(), 20, 0)
	require.NoError(t, err)
	require.Empty(t, res.Data,
		"an approval the caller would be refused at the button must never be listed as their action")
}

// ---------------------------------------------------------------------------
// Criterion 4 — deterministic ordering
// ---------------------------------------------------------------------------

func TestActionCenter_PriorityOrdering(t *testing.T) {
	s := newStub()
	// An admin sees every category, which is what lets one fixture assert the
	// whole cross-category order in a single sequence.
	s.role = domain.BusinessRoleRiskManager

	older := uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000001")
	newer := uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000002")
	s.mitigations = []domain.Mitigation{
		{ID: newer, TenantID: tenantA, Title: "overdue 1d", Status: domain.MitigationPlanned,
			DueDate: ptime(testNow.AddDate(0, 0, -1))},
		{ID: older, TenantID: tenantA, Title: "overdue 30d", Status: domain.MitigationPlanned,
			DueDate: ptime(testNow.AddDate(0, 0, -30))},
	}
	s.risks = []domain.Risk{
		{ID: uuid.New(), TenantID: tenantA, Title: "score 7.1", Score: 7.1},
		{ID: uuid.New(), TenantID: tenantA, Title: "score 9.9", Score: 9.9},
	}

	res, err := newUC(s).GetActionCenter(caller(), 50, 0)
	require.NoError(t, err)

	titles := []string{}
	for _, i := range res.Data {
		titles = append(titles, i.Title)
	}
	require.Equal(t, []string{
		// Category 1 first, most overdue first inside it…
		"overdue 30d", "overdue 1d",
		// …then category 2, highest score first.
		"score 9.9", "score 7.1",
	}, titles)

	// And the ranks are actually carried on the wire, not merely implied by the
	// order, so the frontend can group without re-deriving the rule.
	require.Equal(t, []int{RankOverdueMitigation, RankOverdueMitigation, RankCriticalRisk, RankCriticalRisk},
		[]int{res.Data[0].CategoryRank, res.Data[1].CategoryRank, res.Data[2].CategoryRank, res.Data[3].CategoryRank})
}

func TestActionCenter_OrderingWithinRemainingCategories(t *testing.T) {
	t.Run("approvals: expiry ascending, nulls last", func(t *testing.T) {
		s := newStub()
		s.role = ""
		soon := pendingApprovalNaming(userA, otherU, "expires soon")
		soon.ExpiresAt = ptime(testNow.AddDate(0, 0, 1))
		later := pendingApprovalNaming(userA, otherU, "expires later")
		later.ExpiresAt = ptime(testNow.AddDate(0, 0, 9))
		never := pendingApprovalNaming(userA, otherU, "no expiry")
		s.approvals = []domain.ApprovalRequest{never, later, soon}

		res, err := newUC(s).GetActionCenter(caller(), 20, 0)
		require.NoError(t, err)
		require.Equal(t, []string{"expires soon", "expires later", "no expiry"},
			[]string{res.Data[0].Title, res.Data[1].Title, res.Data[2].Title})
	})

	t.Run("incidents: severity then oldest first", func(t *testing.T) {
		s := newStub()
		s.role = domain.BusinessRoleRSSI
		s.incidents = []domain.Incident{
			{ID: 1, TenantID: tenantA.String(), Title: "high, new", Severity: "high", Status: "open", CreatedAt: testNow.AddDate(0, 0, -1)},
			{ID: 2, TenantID: tenantA.String(), Title: "critical, new", Severity: "critical", Status: "open", CreatedAt: testNow.AddDate(0, 0, -1)},
			{ID: 3, TenantID: tenantA.String(), Title: "critical, old", Severity: "critical", Status: "investigating", CreatedAt: testNow.AddDate(0, 0, -20)},
		}
		res, err := newUC(s).GetActionCenter(caller(), 20, 0)
		require.NoError(t, err)
		require.Equal(t, []string{"critical, old", "critical, new", "high, new"},
			[]string{res.Data[0].Title, res.Data[1].Title, res.Data[2].Title})
	})

	t.Run("evidence: expired before expiring_soon", func(t *testing.T) {
		s := newStub()
		s.role = domain.BusinessRoleAuditor
		s.evidence = []domain.Evidence{
			{ID: uuid.New(), TenantID: tenantA, Title: "expiring in 3d",
				Review: domain.EvidenceReviewAccepted, ValidUntil: ptime(testNow.AddDate(0, 0, 3))},
			{ID: uuid.New(), TenantID: tenantA, Title: "expired 10d ago",
				Review: domain.EvidenceReviewAccepted, ValidUntil: ptime(testNow.AddDate(0, 0, -10))},
		}
		res, err := newUC(s).GetActionCenter(caller(), 20, 0)
		require.NoError(t, err)
		require.Equal(t, []string{"expired 10d ago", "expiring in 3d"},
			[]string{res.Data[0].Title, res.Data[1].Title})
	})
}

// Rejected evidence is not proof, whatever the calendar says — the use case must
// defer to EffectiveStatus rather than trusting the SQL date window.
func TestActionCenter_RejectedEvidenceIsNotAnAction(t *testing.T) {
	s := newStub()
	s.role = domain.BusinessRoleAuditor
	s.evidence = []domain.Evidence{
		{ID: uuid.New(), TenantID: tenantA, Title: "rejected and expired",
			Review: domain.EvidenceReviewRejected, ValidUntil: ptime(testNow.AddDate(0, 0, -10))},
	}
	res, err := newUC(s).GetActionCenter(caller(), 20, 0)
	require.NoError(t, err)
	require.Empty(t, res.Data)
}

// ---------------------------------------------------------------------------
// Criterion 10 — a caller with items across several categories at once
// ---------------------------------------------------------------------------

func TestActionCenter_Success(t *testing.T) {
	s := newStub()
	s.role = domain.BusinessRoleRiskManager
	seedAll(s)
	s.approvals = []domain.ApprovalRequest{
		pendingApprovalNaming(userA, otherU, "Approve the treatment plan"),
	}

	res, err := newUC(s).GetActionCenter(caller(), 20, 0)
	require.NoError(t, err)

	// Three categories: mitigation (1), risk (2), approval (3) — the seeded
	// incident/evidence/remediation belong to categories a risk manager is not
	// shown.
	require.Equal(t, []ItemType{
		ItemTypeOverdueMitigation, ItemTypeCriticalRisk, ItemTypePendingApproval,
	}, types(res.Data))
	require.Equal(t, 3, res.Total)

	for _, item := range res.Data {
		require.Equal(t, tenantA, item.TenantID, "every item must carry the caller's tenant")
		require.NotEmpty(t, item.ID)
		require.NotEmpty(t, item.Title)
		require.NotEmpty(t, item.DeepLink)
		require.NotEmpty(t, item.SubjectResourceID)
	}
}

// ---------------------------------------------------------------------------
// Criterion 6 — paging
// ---------------------------------------------------------------------------

func TestActionCenter_ClampPaging(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		inLimit, inOffset     int
		wantLimit, wantOffset int
	}{
		{"default when unset", 0, 0, DefaultLimit, 0},
		{"default when negative", -5, 0, DefaultLimit, 0},
		{"clamped to max", 5000, 0, MaxLimit, 0},
		{"exact max kept", 100, 0, 100, 0},
		{"honoured in range", 7, 3, 7, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, o := ClampPaging(tc.inLimit, tc.inOffset)
			require.Equal(t, tc.wantLimit, l)
			require.Equal(t, tc.wantOffset, o)
		})
	}
}

func TestActionCenter_Paginates(t *testing.T) {
	s := newStub()
	s.role = domain.BusinessRoleRiskManager
	for i := 0; i < 5; i++ {
		s.risks = append(s.risks, domain.Risk{
			ID: uuid.New(), TenantID: tenantA, Title: "risk", Score: float64(10 - i),
		})
	}

	first, err := newUC(s).GetActionCenter(caller(), 2, 0)
	require.NoError(t, err)
	require.Len(t, first.Data, 2)
	require.Equal(t, 5, first.Total, "total counts everything outstanding, not the page")

	last, err := newUC(s).GetActionCenter(caller(), 2, 4)
	require.NoError(t, err)
	require.Len(t, last.Data, 1)

	// Past the end is an empty page, not an error and not null.
	beyond, err := newUC(s).GetActionCenter(caller(), 2, 99)
	require.NoError(t, err)
	require.NotNil(t, beyond.Data)
	require.Len(t, beyond.Data, 0)
}

// An absurd offset must not overflow int when added to the limit. The early
// "offset past the end" return is what makes that safe, so it is asserted here
// rather than left as a property of the arithmetic.
func TestActionCenter_PagingExtremesDoNotPanic(t *testing.T) {
	s := newStub()
	s.role = domain.BusinessRoleRiskManager
	s.risks = []domain.Risk{{ID: uuid.New(), TenantID: tenantA, Title: "r", Score: 9}}

	for _, tc := range []struct {
		name          string
		limit, offset int
	}{
		{"max int offset", 100, math.MaxInt},
		{"max int on both", math.MaxInt, math.MaxInt},
		{"max int limit", math.MaxInt, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := newUC(s).GetActionCenter(caller(), tc.limit, tc.offset)
			require.NoError(t, err)
			require.NotNil(t, res.Data)
			require.LessOrEqual(t, res.Limit, MaxLimit)
		})
	}
}

// ---------------------------------------------------------------------------
// Criterion 9 — deep links match real frontend routes
// ---------------------------------------------------------------------------

func TestActionCenter_DeepLinkShapes(t *testing.T) {
	id := "0f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0"
	for _, tc := range []struct {
		name string
		got  string
		want string
	}{
		{"mitigation", MitigationLink(id), "/risks/mitigations/" + id},
		// No /risks/:riskId page exists; the register opens the entity drawer.
		{"risk", RiskLink(id), "/risks?drawer=risk&entity=" + id},
		{"approval", ApprovalLink(), "/governance"},
		{"incident", IncidentLink("42"), "/incidents/42/war-room"},
		{"evidence", EvidenceLink(id), "/compliance/evidence?drawer=evidence&entity=" + id},
		{"remediation", RemediationLink(id), "/compliance/remediation/" + id},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.got)
		})
	}
}

func TestActionCenter_EveryItemTypeProducesItsDocumentedLink(t *testing.T) {
	s := newStub()
	s.role = domain.BusinessRoleComplianceOfficer
	seedAll(s)
	s.approvals = []domain.ApprovalRequest{pendingApprovalNaming(userA, otherU, "sign")}

	res, err := newUC(s).GetActionCenter(caller(), 50, 0)
	require.NoError(t, err)
	require.NotEmpty(t, res.Data)

	prefixes := map[ItemType]string{
		ItemTypePendingApproval:    "/governance",
		ItemTypeExpiringEvidence:   "/compliance/evidence?drawer=evidence&entity=",
		ItemTypeOverdueRemediation: "/compliance/remediation/",
	}
	for _, item := range res.Data {
		want, ok := prefixes[item.Type]
		require.True(t, ok, "unexpected item type %q", item.Type)
		require.True(t, len(item.DeepLink) >= len(want) && item.DeepLink[:len(want)] == want,
			"deep link %q for %q must start with %q", item.DeepLink, item.Type, want)
	}
}

// ---------------------------------------------------------------------------
// Repository failures surface rather than silently truncating the list
// ---------------------------------------------------------------------------

func TestActionCenter_RepositoryErrorPropagates(t *testing.T) {
	s := newStub()
	s.role = domain.BusinessRoleRiskManager
	s.err = errors.New("db down")

	_, err := newUC(s).GetActionCenter(caller(), 20, 0)
	require.Error(t, err,
		"a half-built Action Center is worse than an error: it reads as 'nothing needs you'")
}
