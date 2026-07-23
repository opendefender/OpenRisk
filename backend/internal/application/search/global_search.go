// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Package search provides ONE tenant-scoped "universal search" use case that the
// ⌘K command palette calls to find any entity by free text. It composes the
// existing per-entity repositories through narrow, OPTIONAL (nil-safe) ports: a
// missing or erroring source degrades its own slice and never fails the whole
// search — the same best-effort contract as the executive dashboard / smart-risk
// engines. Results are gated by the caller's permissions so search never surfaces
// something a route would 403, keeping it consistent with the sidebar filter.
package search

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	cti "github.com/opendefender/openrisk/pkg/cti"
)

// Result is one hit, shaped for direct rendering by the palette (the icon is
// chosen from Type on the client, and clicking navigates to URL).
type Result struct {
	Type     string  `json:"type"` // risk|asset|vulnerability|control|audit|report|cve|user
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Subtitle string  `json:"subtitle,omitempty"`
	Badge    string  `json:"badge,omitempty"` // severity token, for severity-bearing types
	URL      string  `json:"url"`             // frontend deep-link / section for the entity
	Score    float64 `json:"score,omitempty"` // intra-type ordering hint
}

// Response is the endpoint payload.
type Response struct {
	Query   string   `json:"query"`
	Results []Result `json:"results"`
}

// Narrow ports — satisfied structurally by the existing repositories, so no domain
// interface changes (mocks stay intact).
type (
	RiskSearcher interface {
		SearchByText(ctx context.Context, tenantID uuid.UUID, q string, limit int) ([]domain.Risk, error)
	}
	AssetLister interface {
		List(ctx context.Context, tenantID uuid.UUID) ([]domain.Asset, error)
	}
	VulnSearcher interface {
		List(ctx context.Context, tenantID uuid.UUID, q domain.VulnerabilityQuery) (*domain.PaginatedResult[domain.Vulnerability], error)
	}
	ControlSearcher interface {
		SearchControls(ctx context.Context, tenantID uuid.UUID, q string, limit int) ([]domain.ComplianceControl, error)
	}
	AuditLister interface {
		ListAudits(ctx context.Context, tenantID uuid.UUID) ([]domain.ComplianceAudit, error)
	}
	ReportLister interface {
		List(ctx context.Context, tenantID uuid.UUID) ([]domain.BoardReport, error)
	}
	CVESearcher interface {
		Search(ctx context.Context, query string, filter cti.CTIFilter) ([]cti.CTIVulnerability, error)
	}
	MemberLister interface {
		ListMembers(ctx context.Context, tenantID uuid.UUID) ([]domain.OrganizationMember, error)
	}
)

// UseCase aggregates the optional sources.
type UseCase struct {
	risks    RiskSearcher
	assets   AssetLister
	vulns    VulnSearcher
	controls ControlSearcher
	audits   AuditLister
	reports  ReportLister
	cve      CVESearcher
	members  MemberLister
}

// New builds an empty use case; attach sources with the With* methods.
func New() *UseCase { return &UseCase{} }

func (uc *UseCase) WithRisks(r RiskSearcher) *UseCase       { uc.risks = r; return uc }
func (uc *UseCase) WithAssets(a AssetLister) *UseCase       { uc.assets = a; return uc }
func (uc *UseCase) WithVulns(v VulnSearcher) *UseCase       { uc.vulns = v; return uc }
func (uc *UseCase) WithControls(c ControlSearcher) *UseCase { uc.controls = c; return uc }
func (uc *UseCase) WithAudits(a AuditLister) *UseCase       { uc.audits = a; return uc }
func (uc *UseCase) WithReports(r ReportLister) *UseCase     { uc.reports = r; return uc }
func (uc *UseCase) WithCVE(c CVESearcher) *UseCase          { uc.cve = c; return uc }
func (uc *UseCase) WithMembers(m MemberLister) *UseCase     { uc.members = m; return uc }

// perTypeLimit caps how many hits each source contributes, so the palette stays
// scannable and no single type drowns the others.
const perTypeLimit = 6

// Execute runs the search across every source the caller may read. `can` mirrors
// the route permission check (wildcard-aware); a source whose permission is denied
// is skipped entirely, so search respects RBAC exactly like the nav. Best-effort:
// a source that errors contributes nothing and never fails the call.
func (uc *UseCase) Execute(ctx context.Context, tenantID uuid.UUID, query string, can func(perm string) bool) Response {
	q := strings.TrimSpace(query)
	out := Response{Query: q, Results: []Result{}}
	if q == "" || tenantID == uuid.Nil {
		return out
	}
	if uc.risks != nil && can("risks:read") {
		out.Results = append(out.Results, uc.searchRisks(ctx, tenantID, q)...)
	}
	if uc.assets != nil && can("assets:read") {
		out.Results = append(out.Results, uc.searchAssets(ctx, tenantID, q)...)
	}
	if uc.vulns != nil && can("vulnerabilities:read") {
		out.Results = append(out.Results, uc.searchVulns(ctx, tenantID, q)...)
	}
	if uc.controls != nil && can("compliance:read") {
		out.Results = append(out.Results, uc.searchControls(ctx, tenantID, q)...)
	}
	if uc.audits != nil && can("compliance:read") {
		out.Results = append(out.Results, uc.searchAudits(ctx, tenantID, q)...)
	}
	if uc.reports != nil && can("reports:board:read") {
		out.Results = append(out.Results, uc.searchReports(ctx, tenantID, q)...)
	}
	// CVEs are global threat-intel (not tenant data); gate by the same perm as the
	// Threat Intel nav item.
	if uc.cve != nil && can("risks:read") {
		out.Results = append(out.Results, uc.searchCVE(ctx, q)...)
	}
	// Members are org administration — admin-only, mirroring the "Roles & access"
	// screen. HasPermission("*") is true only for literal admins/root.
	if uc.members != nil && can("*") {
		out.Results = append(out.Results, uc.searchUsers(ctx, tenantID, q)...)
	}
	return out
}

func (uc *UseCase) searchRisks(ctx context.Context, tenantID uuid.UUID, q string) []Result {
	risks, err := uc.risks.SearchByText(ctx, tenantID, q, perTypeLimit)
	if err != nil {
		return nil
	}
	res := make([]Result, 0, len(risks))
	for _, r := range risks {
		title := r.Title
		if title == "" {
			title = r.Name
		}
		res = append(res, Result{
			Type:     "risk",
			ID:       r.ID.String(),
			Title:    title,
			Subtitle: string(r.Status),
			Badge:    strings.ToLower(string(r.Criticality)),
			URL:      "/risks?focus=" + r.ID.String(),
			Score:    r.Score,
		})
	}
	return res
}

func (uc *UseCase) searchAssets(ctx context.Context, tenantID uuid.UUID, q string) []Result {
	assets, err := uc.assets.List(ctx, tenantID)
	if err != nil {
		return nil
	}
	ql := strings.ToLower(q)
	res := make([]Result, 0, perTypeLimit)
	for _, a := range assets {
		if !strings.Contains(strings.ToLower(a.Name), ql) && !strings.Contains(strings.ToLower(a.Type), ql) {
			continue
		}
		res = append(res, Result{
			Type:     "asset",
			ID:       a.ID.String(),
			Title:    a.Name,
			Subtitle: a.Type,
			Badge:    strings.ToLower(string(a.Criticality)),
			URL:      "/assets?focus=" + a.ID.String(),
		})
		if len(res) >= perTypeLimit {
			break
		}
	}
	return res
}

func (uc *UseCase) searchVulns(ctx context.Context, tenantID uuid.UUID, q string) []Result {
	vq := domain.NewVulnerabilityQuery()
	vq.Search = q
	vq.Limit = perTypeLimit
	vq.Page = 1
	page, err := uc.vulns.List(ctx, tenantID, vq)
	if err != nil || page == nil {
		return nil
	}
	res := make([]Result, 0, len(page.Data))
	for _, v := range page.Data {
		label := v.CVEID
		if label == "" {
			label = v.AssetName
		}
		res = append(res, Result{
			Type:     "vulnerability",
			ID:       v.ID.String(),
			Title:    v.Title,
			Subtitle: strings.TrimSpace(label),
			Badge:    string(v.Severity),
			URL:      "/vulnerabilities?focus=" + v.ID.String(),
			Score:    v.PriorityScore,
		})
	}
	return res
}

func (uc *UseCase) searchControls(ctx context.Context, tenantID uuid.UUID, q string) []Result {
	controls, err := uc.controls.SearchControls(ctx, tenantID, q, perTypeLimit)
	if err != nil {
		return nil
	}
	res := make([]Result, 0, len(controls))
	for _, c := range controls {
		sub := c.ReferenceCode
		if sub == "" {
			sub = string(c.Status)
		}
		res = append(res, Result{
			Type:     "control",
			ID:       c.ID.String(),
			Title:    c.Name,
			Subtitle: sub,
			URL:      "/compliance",
		})
	}
	return res
}

func (uc *UseCase) searchAudits(ctx context.Context, tenantID uuid.UUID, q string) []Result {
	audits, err := uc.audits.ListAudits(ctx, tenantID)
	if err != nil {
		return nil
	}
	ql := strings.ToLower(q)
	res := make([]Result, 0, perTypeLimit)
	for _, a := range audits {
		if !strings.Contains(strings.ToLower(a.Title), ql) && !strings.Contains(strings.ToLower(a.Auditor), ql) {
			continue
		}
		res = append(res, Result{
			Type:     "audit",
			ID:       a.ID.String(),
			Title:    a.Title,
			Subtitle: string(a.Type) + " · " + string(a.Status),
			URL:      "/compliance/audits",
		})
		if len(res) >= perTypeLimit {
			break
		}
	}
	return res
}

func (uc *UseCase) searchReports(ctx context.Context, tenantID uuid.UUID, q string) []Result {
	reports, err := uc.reports.List(ctx, tenantID)
	if err != nil {
		return nil
	}
	ql := strings.ToLower(q)
	res := make([]Result, 0, perTypeLimit)
	for _, r := range reports {
		if !strings.Contains(strings.ToLower(r.Title), ql) && !strings.Contains(strings.ToLower(r.PeriodLabel), ql) {
			continue
		}
		title := r.Title
		if title == "" {
			title = r.PeriodLabel
		}
		res = append(res, Result{
			Type:     "report",
			ID:       r.ID.String(),
			Title:    title,
			Subtitle: r.PeriodLabel,
			URL:      "/reports",
		})
		if len(res) >= perTypeLimit {
			break
		}
	}
	return res
}

func (uc *UseCase) searchCVE(ctx context.Context, q string) []Result {
	cves, err := uc.cve.Search(ctx, q, cti.CTIFilter{Limit: perTypeLimit})
	if err != nil {
		return nil
	}
	res := make([]Result, 0, len(cves))
	for _, v := range cves {
		res = append(res, Result{
			Type:     "cve",
			ID:       v.CVEID,
			Title:    v.CVEID,
			Subtitle: snippet(v.Description, 80),
			Badge:    strings.ToLower(v.Severity),
			URL:      "/threat-map",
		})
		if len(res) >= perTypeLimit {
			break
		}
	}
	return res
}

func (uc *UseCase) searchUsers(ctx context.Context, tenantID uuid.UUID, q string) []Result {
	members, err := uc.members.ListMembers(ctx, tenantID)
	if err != nil {
		return nil
	}
	ql := strings.ToLower(q)
	res := make([]Result, 0, perTypeLimit)
	for _, m := range members {
		u := m.User
		if u == nil {
			continue
		}
		if !strings.Contains(strings.ToLower(u.FullName), ql) &&
			!strings.Contains(strings.ToLower(u.Email), ql) &&
			!strings.Contains(strings.ToLower(u.Username), ql) {
			continue
		}
		title := u.FullName
		if title == "" {
			title = u.Username
		}
		res = append(res, Result{
			Type:     "user",
			ID:       u.ID.String(),
			Title:    title,
			Subtitle: u.Email,
			URL:      "/settings/roles",
		})
		if len(res) >= perTypeLimit {
			break
		}
	}
	return res
}

// snippet trims s to at most n runes, appending an ellipsis if it was cut.
func snippet(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}
