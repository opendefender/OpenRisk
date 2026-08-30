// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package actioncenter answers one question the rest of the product does not:
// "of everything currently outstanding, what should I personally do first?"
//
// It is deliberately NOT the notification bell. The bell
// (internal/application/notification) is a chronological feed of things that
// already happened, and it is delivered — each row was written by a producer at
// the moment of the event. The Action Center is a LIVE READ over the current
// state of six existing tables: nothing is persisted here, nothing is delivered,
// and an item disappears from the list the moment the underlying work is done.
// That is why there is no action_items table and no "dismiss" — dismissing an
// overdue mitigation would be lying to the person who has to answer for it.
package actioncenter

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

var (
	// ErrUnauthorized is returned when the caller carries no usable identity.
	// Mirrors notification.ErrUnauthorized so the handler layer maps both the
	// same way.
	ErrUnauthorized = errors.New("unauthorized")
)

// ItemType names the source an action item was derived from. The frontend keys
// its icon and label off this, so the values are contract.
type ItemType string

const (
	ItemTypeOverdueMitigation  ItemType = "overdue_mitigation"
	ItemTypeCriticalRisk       ItemType = "critical_risk"
	ItemTypePendingApproval    ItemType = "pending_approval"
	ItemTypeOpenIncident       ItemType = "open_incident"
	ItemTypeExpiringEvidence   ItemType = "expiring_evidence"
	ItemTypeOverdueRemediation ItemType = "overdue_remediation"
)

// Category ranks. Lower sorts first. These are the priority order itself, not a
// display hint: an overdue mitigation outranks a critical risk because the
// mitigation is a commitment someone already made and missed, while the risk is
// a condition that may be correctly accepted.
const (
	RankOverdueMitigation  = 1
	RankCriticalRisk       = 2
	RankPendingApproval    = 3
	RankOpenIncident       = 4
	RankExpiringEvidence   = 5
	RankOverdueRemediation = 6
)

// CriticalScoreThreshold is the frozen `critical` cut from the Score Engine
// (CLAUDE.md: critical >= 7.0). Read here, never recomputed — pkg/scoring owns
// the formula and this package must not drift from it.
const CriticalScoreThreshold = 7.0

// perSourceFetchLimit caps how many rows each of the six queries may return
// before ordering. A tenant with 40 000 overdue mitigations must not be able to
// turn one page view into a 40 000-row scan; the list is a prioritised top-N,
// and anything past this cap could not have been reached by paging anyway.
const perSourceFetchLimit = 200

// DefaultLimit and MaxLimit mirror GetNotifications' clamping.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// ActionItem is a response DTO, not a persisted model. It is assembled on read
// and has no table.
type ActionItem struct {
	ID                  string     `json:"id"`
	Type                ItemType   `json:"type"`
	Title               string     `json:"title"`
	SubjectResourceType string     `json:"subject_resource_type"`
	SubjectResourceID   string     `json:"subject_resource_id"`
	DeepLink            string     `json:"deep_link"`
	DueAt               *time.Time `json:"due_at"`
	CategoryRank        int        `json:"category_rank"`
	TenantID            uuid.UUID  `json:"tenant_id"`

	// sortA/sortB are the per-category secondary keys, both ascending. They are
	// unexported so the ordering rule cannot be contradicted by a client that
	// decides to re-sort on a field it can see.
	sortA float64
	sortB float64
}

// Result is the full response envelope.
type Result struct {
	Data        []ActionItem `json:"data"`
	GeneratedAt time.Time    `json:"generated_at"`
	Limit       int          `json:"limit"`
	Offset      int          `json:"offset"`
	Total       int          `json:"total"`
}

// Repository is the persistence port. Every method takes the tenant explicitly
// and MUST filter on it — there is no ambient tenant here, precisely so that a
// missing filter is visible at the call site during review.
//
// Each method returns rows already narrowed to its category's predicate; the
// use case does the role gating and the ordering, which keeps both pure and
// unit-testable without a database.
type Repository interface {
	// BusinessRoleFor resolves the caller's GRC job-role preset from their
	// OrganizationMember row. Returns "" when the member has none.
	BusinessRoleFor(userID, tenantID uuid.UUID) (domain.BusinessRoleKey, error)

	OverdueMitigations(tenantID uuid.UUID, now time.Time, limit int) ([]domain.Mitigation, error)
	CriticalRisksWithoutActiveMitigation(tenantID uuid.UUID, threshold float64, limit int) ([]domain.Risk, error)
	PendingApprovals(tenantID uuid.UUID, limit int) ([]domain.ApprovalRequest, error)
	OpenIncidents(tenantID uuid.UUID, limit int) ([]domain.Incident, error)
	ExpiringEvidence(tenantID uuid.UUID, now time.Time, limit int) ([]domain.Evidence, error)
	OverdueRemediationPlans(tenantID uuid.UUID, now time.Time, limit int) ([]domain.RemediationPlan, error)
}

// Caller is everything the use case needs to know about who is asking. Built by
// the handler from the request context; never trusted from a request body.
type Caller struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
	// Roles are the caller's organisation-level roles (root/admin/user) for this
	// tenant, plus their business role. Fed to domain.CanSign, which matches a
	// workflow step's approver_role against them.
	Roles   []string
	IsAdmin bool
}

// roleCategories maps a GRC job role to the categories it is shown.
//
// A role appears here only for work it can actually act on. Category 3
// (approvals) is deliberately absent from every entry: it is NOT role-gated but
// per-request, and is evaluated separately for every caller by domain.CanSign —
// see gather(). Adding 3 to a role here would be wrong in both directions, both
// granting approvals to people who are not the approver and implying that
// someone whose role is missing from this map cannot be one.
//
// A role that is not listed — asset_owner, executive, viewer, dsi,
// internal_control, risk_owner, security_analyst, or a member with no business
// role at all — sees ONLY their own pending approvals. That default is
// intentional and least-privilege: showing someone an "action" they have no
// permission to complete is worse than showing them nothing, because the item
// looks like a task and behaves like a locked door.
var roleCategories = map[domain.BusinessRoleKey][]int{
	domain.BusinessRoleRSSI:              {RankCriticalRisk, RankOpenIncident},
	domain.BusinessRoleRiskManager:       {RankOverdueMitigation, RankCriticalRisk},
	domain.BusinessRoleAuditor:           {RankExpiringEvidence},
	domain.BusinessRoleComplianceOfficer: {RankExpiringEvidence, RankOverdueRemediation},
}

// UseCase is the read-only aggregation. It writes nothing, so it needs no
// transaction.
type UseCase struct {
	repo Repository
	// now is injectable so the ordering tests can pin the clock. Production
	// wiring leaves it nil and gets time.Now.
	now func() time.Time
}

func NewUseCase(repo Repository) *UseCase {
	return &UseCase{repo: repo}
}

// WithClock pins the clock, for tests that assert an exact ordering.
func (uc *UseCase) WithClock(now func() time.Time) *UseCase {
	uc.now = now
	return uc
}

func (uc *UseCase) clock() time.Time {
	if uc.now != nil {
		return uc.now()
	}
	return time.Now().UTC()
}

// GetActionCenter returns the caller's prioritised outstanding work.
//
// Ordering is total and deterministic: category rank, then the category's own
// secondary key, then the item id as a final tiebreak so two items that are
// equal on every business key still come back in a stable order across calls.
// Without that last tiebreak the list would shuffle between refreshes for a
// tenant that bulk-imported rows with identical due dates, which reads as a bug.
func (uc *UseCase) GetActionCenter(caller Caller, limit, offset int) (*Result, error) {
	if caller.UserID == uuid.Nil || caller.TenantID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	limit, offset = ClampPaging(limit, offset)
	now := uc.clock()

	role, err := uc.repo.BusinessRoleFor(caller.UserID, caller.TenantID)
	if err != nil {
		return nil, err
	}

	items, err := uc.gather(caller, role, now)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.CategoryRank != b.CategoryRank {
			return a.CategoryRank < b.CategoryRank
		}
		if a.sortA != b.sortA {
			return a.sortA < b.sortA
		}
		if a.sortB != b.sortB {
			return a.sortB < b.sortB
		}
		return a.ID < b.ID
	})

	total := len(items)
	page := paginate(items, limit, offset)

	return &Result{
		Data:        page,
		GeneratedAt: now,
		Limit:       limit,
		Offset:      offset,
		Total:       total,
	}, nil
}

// ClampPaging applies the documented limit/offset rules. Exported so the handler
// and the tests agree on one implementation.
func ClampPaging(limit, offset int) (int, int) {
	if limit > MaxLimit {
		limit = MaxLimit
	}
	if limit <= 0 {
		limit = DefaultLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// paginate always returns a non-nil slice so the JSON contract is `[]`, never
// `null` — an empty Action Center is a good day, not a missing field.
func paginate(items []ActionItem, limit, offset int) []ActionItem {
	if offset >= len(items) {
		return []ActionItem{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	page := make([]ActionItem, 0, end-offset)
	page = append(page, items[offset:end]...)
	return page
}

func allows(role domain.BusinessRoleKey, rank int) bool {
	for _, r := range roleCategories[role] {
		if r == rank {
			return true
		}
	}
	return false
}

// gather runs only the queries the caller's role can actually use. A compliance
// officer never triggers the incident scan, which keeps the endpoint's cost
// proportional to what the caller is allowed to see rather than to the size of
// the tenant.
func (uc *UseCase) gather(caller Caller, role domain.BusinessRoleKey, now time.Time) ([]ActionItem, error) {
	items := make([]ActionItem, 0, 32)

	if allows(role, RankOverdueMitigation) {
		rows, err := uc.repo.OverdueMitigations(caller.TenantID, now, perSourceFetchLimit)
		if err != nil {
			return nil, err
		}
		for i := range rows {
			items = append(items, mitigationItem(&rows[i], now))
		}
	}

	if allows(role, RankCriticalRisk) {
		rows, err := uc.repo.CriticalRisksWithoutActiveMitigation(caller.TenantID, CriticalScoreThreshold, perSourceFetchLimit)
		if err != nil {
			return nil, err
		}
		for i := range rows {
			items = append(items, riskItem(&rows[i]))
		}
	}

	// Category 3 is ALWAYS evaluated, for every caller, regardless of role: being
	// the named approver of a pending step is a property of the request, not of
	// a job title. domain.CanSign is the same predicate the governance module
	// uses to decide whether the Approve button works, so an item can never
	// appear here that the caller would be refused on.
	approvals, err := uc.repo.PendingApprovals(caller.TenantID, perSourceFetchLimit)
	if err != nil {
		return nil, err
	}
	// The business role joins the caller's org roles for the eligibility check:
	// a workflow step whose approver_role is "compliance_officer" names a GRC job
	// role, not an org role, and matching only root/admin/user would refuse the
	// very people such a step exists to nominate.
	roles := caller.Roles
	if role != "" {
		roles = append(append([]string{}, roles...), string(role))
	}
	who := domain.Approver{UserID: caller.UserID, Roles: roles, IsAdmin: caller.IsAdmin}
	for i := range approvals {
		req := &approvals[i]
		step := req.CurrentStepDef()
		if step == nil {
			continue
		}
		if !domain.CanSign(req, step, who).Eligible {
			continue
		}
		items = append(items, approvalItem(req))
	}

	if allows(role, RankOpenIncident) {
		rows, err := uc.repo.OpenIncidents(caller.TenantID, perSourceFetchLimit)
		if err != nil {
			return nil, err
		}
		for i := range rows {
			items = append(items, incidentItem(&rows[i]))
		}
	}

	if allows(role, RankExpiringEvidence) {
		rows, err := uc.repo.ExpiringEvidence(caller.TenantID, now, perSourceFetchLimit)
		if err != nil {
			return nil, err
		}
		for i := range rows {
			// EffectiveStatus is re-checked here rather than trusted from the
			// query: the SQL narrows by valid_until, but "rejected" and "pending"
			// review outrank the calendar and only the domain method knows that.
			st := rows[i].EffectiveStatus(now)
			if st != domain.EvidenceStatusExpired && st != domain.EvidenceStatusExpiring {
				continue
			}
			items = append(items, evidenceItem(&rows[i], st, now))
		}
	}

	if allows(role, RankOverdueRemediation) {
		rows, err := uc.repo.OverdueRemediationPlans(caller.TenantID, now, perSourceFetchLimit)
		if err != nil {
			return nil, err
		}
		for i := range rows {
			items = append(items, remediationItem(&rows[i], now))
		}
	}

	return items, nil
}

// ---------------------------------------------------------------------------
// Item builders. Each owns its deep link, so there is exactly one place per
// category where a link shape is decided (see deeplinks.go for the routes and
// why they are what they are).
// ---------------------------------------------------------------------------

func daysBetween(from, to time.Time) float64 {
	return to.Sub(from).Hours() / 24
}

func mitigationItem(m *domain.Mitigation, now time.Time) ActionItem {
	overdue := 0.0
	if m.DueDate != nil {
		overdue = daysBetween(*m.DueDate, now)
	}
	return ActionItem{
		ID:                  "mitigation:" + m.ID.String(),
		Type:                ItemTypeOverdueMitigation,
		Title:               m.Title,
		SubjectResourceType: "mitigation",
		SubjectResourceID:   m.ID.String(),
		DeepLink:            MitigationLink(m.ID.String()),
		DueAt:               m.DueDate,
		CategoryRank:        RankOverdueMitigation,
		TenantID:            m.TenantID,
		sortA:               -overdue, // most overdue first
	}
}

func riskItem(r *domain.Risk) ActionItem {
	return ActionItem{
		ID:                  "risk:" + r.ID.String(),
		Type:                ItemTypeCriticalRisk,
		Title:               r.Title,
		SubjectResourceType: "risk",
		SubjectResourceID:   r.ID.String(),
		DeepLink:            RiskLink(r.ID.String()),
		CategoryRank:        RankCriticalRisk,
		TenantID:            r.TenantID,
		sortA:               -r.Score, // highest score first
	}
}

func approvalItem(a *domain.ApprovalRequest) ActionItem {
	// Nulls last: a request with no expiry is not more urgent than one that
	// lapses on Friday, so it sorts after every dated one.
	expires := math.MaxFloat64
	if a.ExpiresAt != nil {
		expires = float64(a.ExpiresAt.Unix())
	}
	return ActionItem{
		ID:                  "approval:" + a.ID.String(),
		Type:                ItemTypePendingApproval,
		Title:               a.Title,
		SubjectResourceType: "approval_request",
		SubjectResourceID:   a.ID.String(),
		DeepLink:            ApprovalLink(),
		DueAt:               a.ExpiresAt,
		CategoryRank:        RankPendingApproval,
		TenantID:            a.TenantID,
		sortA:               expires,
		sortB:               float64(a.CreatedAt.Unix()),
	}
}

// incidentSeverityRank orders the severity vocabulary. Anything unrecognised
// sorts last rather than being dropped: an incident with a typo'd severity is
// still an open incident, and silently hiding it would be the worse failure.
func incidentSeverityRank(sev string) float64 {
	switch sev {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}

func incidentItem(i *domain.Incident) ActionItem {
	id := strconv.FormatUint(uint64(i.ID), 10)
	return ActionItem{
		ID:                  "incident:" + id,
		Type:                ItemTypeOpenIncident,
		Title:               i.Title,
		SubjectResourceType: "incident",
		SubjectResourceID:   id,
		DeepLink:            IncidentLink(id),
		CategoryRank:        RankOpenIncident,
		// Incident.TenantID is a string on this table, unlike every other model
		// here. Parsed rather than reinterpreted so a malformed value surfaces as
		// uuid.Nil instead of silently becoming another tenant's id.
		TenantID: parseTenant(i.TenantID),
		sortA:    incidentSeverityRank(i.Severity),
		sortB:    float64(i.CreatedAt.Unix()), // oldest first within a severity
	}
}

func parseTenant(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}

func evidenceItem(e *domain.Evidence, st domain.EvidenceStatus, now time.Time) ActionItem {
	// Expired before expiring_soon, then by how far past (or from) the date.
	statusRank := 1.0
	if st == domain.EvidenceStatusExpired {
		statusRank = 0
	}
	days := 0.0
	if e.ValidUntil != nil {
		days = daysBetween(now, *e.ValidUntil) // negative once expired
	}
	return ActionItem{
		ID:                  "evidence:" + e.ID.String(),
		Type:                ItemTypeExpiringEvidence,
		Title:               e.Title,
		SubjectResourceType: "evidence",
		SubjectResourceID:   e.ID.String(),
		DeepLink:            EvidenceLink(e.ID.String()),
		DueAt:               e.ValidUntil,
		CategoryRank:        RankExpiringEvidence,
		TenantID:            e.TenantID,
		sortA:               statusRank,
		sortB:               days,
	}
}

func remediationItem(p *domain.RemediationPlan, now time.Time) ActionItem {
	overdue := 0.0
	if p.DueDate != nil {
		overdue = daysBetween(*p.DueDate, now)
	}
	return ActionItem{
		ID:                  "remediation:" + p.ID.String(),
		Type:                ItemTypeOverdueRemediation,
		Title:               p.Title,
		SubjectResourceType: "remediation_plan",
		SubjectResourceID:   p.ID.String(),
		DeepLink:            RemediationLink(p.ID.String()),
		DueAt:               p.DueDate,
		CategoryRank:        RankOverdueRemediation,
		TenantID:            p.TenantID,
		sortA:               -overdue,
	}
}
