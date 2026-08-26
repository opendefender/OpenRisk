// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entity

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// In-memory fakes that mirror the real repositories' tenant contract.
//
// Each one filters by tenant exactly as its Gorm counterpart does, so a test
// that proves a cross-tenant read is refused is proving the SERVICE's behaviour
// against a store that behaves like the real one — not against a store that was
// rigged to refuse.

// --- permissions -----------------------------------------------------------

type fakePerms struct{ granted map[string]bool }

func perms(list ...string) fakePerms {
	m := map[string]bool{}
	for _, p := range list {
		m[p] = true
	}
	return fakePerms{granted: m}
}

// HasPermission mirrors the middleware's wildcard semantics.
func (f fakePerms) HasPermission(required string) bool {
	if f.granted["*"] {
		return true
	}
	if f.granted[required] {
		return true
	}
	for p := range f.granted {
		if len(p) > 2 && p[len(p)-2:] == ":*" {
			prefix := p[:len(p)-1]
			if len(required) > len(prefix) && required[:len(prefix)] == prefix {
				return true
			}
		}
	}
	return false
}

func callerIn(tenant uuid.UUID, list ...string) Caller {
	return Caller{UserID: uuid.New(), TenantID: tenant, Perms: perms(list...)}
}

// --- entity stores ---------------------------------------------------------

type fakeAssets struct{ items map[uuid.UUID]*domain.Asset }

func newFakeAssets() *fakeAssets { return &fakeAssets{items: map[uuid.UUID]*domain.Asset{}} }

func (f *fakeAssets) add(a *domain.Asset) *domain.Asset {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	cp := *a
	f.items[a.ID] = &cp
	return &cp
}

func (f *fakeAssets) GetByID(_ context.Context, id, tenantID uuid.UUID) (*domain.Asset, error) {
	a, ok := f.items[id]
	if !ok || a.TenantID != tenantID {
		return nil, nil
	}
	cp := *a
	return &cp, nil
}

type fakeRisks struct{ items map[uuid.UUID]*domain.Risk }

func newFakeRisks() *fakeRisks { return &fakeRisks{items: map[uuid.UUID]*domain.Risk{}} }

func (f *fakeRisks) add(r *domain.Risk) *domain.Risk {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	cp := *r
	f.items[r.ID] = &cp
	return &cp
}

func (f *fakeRisks) GetByID(_ context.Context, id, tenantID uuid.UUID) (*domain.Risk, error) {
	r, ok := f.items[id]
	if !ok || r.TenantID != tenantID {
		return nil, nil
	}
	cp := *r
	return &cp, nil
}

type fakeVulns struct {
	items map[uuid.UUID]*domain.Vulnerability
}

func newFakeVulns() *fakeVulns { return &fakeVulns{items: map[uuid.UUID]*domain.Vulnerability{}} }

func (f *fakeVulns) add(v *domain.Vulnerability) *domain.Vulnerability {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	cp := *v
	f.items[v.ID] = &cp
	return &cp
}

func (f *fakeVulns) GetByID(_ context.Context, id, tenantID uuid.UUID) (*domain.Vulnerability, error) {
	v, ok := f.items[id]
	if !ok || v.TenantID != tenantID {
		return nil, nil
	}
	cp := *v
	return &cp, nil
}

type fakeControls struct {
	controls   map[uuid.UUID]*domain.ComplianceControl
	frameworks map[uuid.UUID]*domain.ComplianceFramework
}

func newFakeControls() *fakeControls {
	return &fakeControls{
		controls:   map[uuid.UUID]*domain.ComplianceControl{},
		frameworks: map[uuid.UUID]*domain.ComplianceFramework{},
	}
}

func (f *fakeControls) add(c *domain.ComplianceControl) *domain.ComplianceControl {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	cp := *c
	f.controls[c.ID] = &cp
	return &cp
}

func (f *fakeControls) GetControlByID(_ context.Context, id, tenantID uuid.UUID) (*domain.ComplianceControl, error) {
	c, ok := f.controls[id]
	if !ok || c.TenantID != tenantID {
		return nil, nil
	}
	cp := *c
	return &cp, nil
}

func (f *fakeControls) GetFrameworkByID(_ context.Context, id, tenantID uuid.UUID) (*domain.ComplianceFramework, error) {
	fw, ok := f.frameworks[id]
	if !ok || fw.TenantID != tenantID {
		return nil, nil
	}
	cp := *fw
	return &cp, nil
}

type fakeIncidents struct{ items map[uint]*domain.Incident }

func newFakeIncidents() *fakeIncidents { return &fakeIncidents{items: map[uint]*domain.Incident{}} }

func (f *fakeIncidents) add(i *domain.Incident) *domain.Incident {
	if i.ID == 0 {
		i.ID = uint(len(f.items) + 1)
	}
	cp := *i
	f.items[i.ID] = &cp
	return &cp
}

// GetIncident mirrors the service: another tenant's id is a not-found error,
// never a row.
func (f *fakeIncidents) GetIncident(tenantID string, id uint) (*domain.Incident, error) {
	i, ok := f.items[id]
	if !ok || i.TenantID != tenantID {
		return nil, domain.ErrNotFound
	}
	cp := *i
	return &cp, nil
}

type fakeEvidence struct {
	items map[uuid.UUID]*domain.Evidence
}

func newFakeEvidence() *fakeEvidence { return &fakeEvidence{items: map[uuid.UUID]*domain.Evidence{}} }

func (f *fakeEvidence) add(e *domain.Evidence) *domain.Evidence {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	cp := *e
	f.items[e.ID] = &cp
	return &cp
}

// GetByID takes (tenant, id) — the argument order the real evidence repository
// uses, deliberately kept different here so a swapped pair would fail.
func (f *fakeEvidence) GetByID(_ context.Context, tenantID, id uuid.UUID) (*domain.Evidence, error) {
	e, ok := f.items[id]
	if !ok || e.TenantID != tenantID {
		return nil, nil
	}
	cp := *e
	return &cp, nil
}

// --- relations -------------------------------------------------------------

// fakeRelations records every call so a test can assert the tenant it was asked
// for, and returns rows keyed by (tenant, entity) so a cross-tenant query comes
// back empty exactly as the real repository's predicate makes it.
type fakeRelations struct {
	rows map[string][]RelationRow
	// failFor makes one relation kind return an error, so the degradation path
	// (§27: one broken source must not blank the drawer) can be exercised.
	failFor string
	calls   []string
}

func newFakeRelations() *fakeRelations {
	return &fakeRelations{rows: map[string][]RelationRow{}}
}

func relKey(kind string, tenant uuid.UUID, id string) string {
	return kind + "|" + tenant.String() + "|" + id
}

func (f *fakeRelations) set(kind string, tenant uuid.UUID, id string, rows ...RelationRow) {
	f.rows[relKey(kind, tenant, id)] = rows
}

func (f *fakeRelations) get(kind string, tenant uuid.UUID, id string) ([]RelationRow, int, error) {
	f.calls = append(f.calls, relKey(kind, tenant, id))
	if f.failFor == kind {
		return nil, 0, errors.New("relation source unavailable")
	}
	rows := f.rows[relKey(kind, tenant, id)]
	return rows, len(rows), nil
}

func (f *fakeRelations) RisksForAsset(_ context.Context, t, id uuid.UUID, _ int) ([]RelationRow, int, error) {
	return f.get("risks_for_asset", t, id.String())
}
func (f *fakeRelations) VulnerabilitiesForAsset(_ context.Context, t, id uuid.UUID, _ int) ([]RelationRow, int, error) {
	return f.get("vulns_for_asset", t, id.String())
}
func (f *fakeRelations) FindingsForAsset(_ context.Context, t, id uuid.UUID, _ int) ([]RelationRow, int, error) {
	return f.get("findings_for_asset", t, id.String())
}
func (f *fakeRelations) IncidentsForAsset(_ context.Context, t, id uuid.UUID, _ int) ([]RelationRow, int, error) {
	return f.get("incidents_for_asset", t, id.String())
}
func (f *fakeRelations) DependenciesForAsset(_ context.Context, t, id uuid.UUID, _ int) ([]RelationRow, int, error) {
	return f.get("deps_for_asset", t, id.String())
}
func (f *fakeRelations) AssetsForRisk(_ context.Context, t, id uuid.UUID, _ int) ([]RelationRow, int, error) {
	return f.get("assets_for_risk", t, id.String())
}
func (f *fakeRelations) MitigationsForRisk(_ context.Context, t, id uuid.UUID, _ int) ([]RelationRow, int, error) {
	return f.get("mitigations_for_risk", t, id.String())
}
func (f *fakeRelations) IncidentsForRisk(_ context.Context, t, id uuid.UUID, _ int) ([]RelationRow, int, error) {
	return f.get("incidents_for_risk", t, id.String())
}
func (f *fakeRelations) ControlsForRisk(_ context.Context, t, id uuid.UUID, _ int) ([]RelationRow, int, error) {
	return f.get("controls_for_risk", t, id.String())
}
func (f *fakeRelations) VulnerabilitiesForRisk(_ context.Context, t, id uuid.UUID, _ int) ([]RelationRow, int, error) {
	return f.get("vulns_for_risk", t, id.String())
}
func (f *fakeRelations) RisksForControl(_ context.Context, t, id uuid.UUID, _ int) ([]RelationRow, int, error) {
	return f.get("risks_for_control", t, id.String())
}
func (f *fakeRelations) EvidenceForControl(_ context.Context, t, id uuid.UUID, _ int) ([]RelationRow, int, error) {
	return f.get("evidence_for_control", t, id.String())
}
func (f *fakeRelations) ControlsForEvidence(_ context.Context, t, id uuid.UUID, _ int) ([]RelationRow, int, error) {
	return f.get("controls_for_evidence", t, id.String())
}
func (f *fakeRelations) AssetsForVulnerability(_ context.Context, t, id uuid.UUID, _ int) ([]RelationRow, int, error) {
	return f.get("assets_for_vuln", t, id.String())
}
func (f *fakeRelations) RisksForVulnerability(_ context.Context, t, id uuid.UUID, _ int) ([]RelationRow, int, error) {
	return f.get("risks_for_vuln", t, id.String())
}
func (f *fakeRelations) RisksForIncident(_ context.Context, t uuid.UUID, id uint, _ int) ([]RelationRow, int, error) {
	return f.get("risks_for_incident", t, uintStr(id))
}
func (f *fakeRelations) AssetsForIncident(_ context.Context, t uuid.UUID, id uint, _ int) ([]RelationRow, int, error) {
	return f.get("assets_for_incident", t, uintStr(id))
}

func uintStr(n uint) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// --- audit trail -----------------------------------------------------------

type fakeAudit struct {
	events []domain.AuditEvent
	err    error
	// lastFilter records what the service asked for.
	lastFilter domain.AuditEventFilter
	lastTenant uuid.UUID
}

func (f *fakeAudit) Append(_ context.Context, e *domain.AuditEvent) error {
	f.events = append(f.events, *e)
	return nil
}

// List mirrors the Gorm repository: tenant predicate, entity-type IN, the
// inclusive To bound, newest first, limit applied last.
func (f *fakeAudit) List(_ context.Context, tenantID uuid.UUID, flt domain.AuditEventFilter) ([]domain.AuditEvent, int64, error) {
	f.lastFilter = flt
	f.lastTenant = tenantID
	if f.err != nil {
		return nil, 0, f.err
	}
	var out []domain.AuditEvent
	for _, e := range f.events {
		if e.TenantID != tenantID {
			continue
		}
		if flt.EntityID != "" && e.EntityID != flt.EntityID {
			continue
		}
		if len(flt.EntityTypes) > 0 && !containsStr(flt.EntityTypes, e.EntityType) {
			continue
		}
		if flt.Action != "" && string(e.Action) != flt.Action {
			continue
		}
		if flt.ActorID != nil && (e.ActorID == nil || *e.ActorID != *flt.ActorID) {
			continue
		}
		if flt.To != nil && e.CreatedAt.After(*flt.To) {
			continue
		}
		if flt.From != nil && e.CreatedAt.Before(*flt.From) {
			continue
		}
		out = append(out, e)
	}
	// Newest first, ties broken by id descending — the same total order the
	// real repository applies, because a fake with a weaker order would let a
	// pagination bug pass here and fail in production.
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID.String() > out[j].ID.String()
	})
	total := int64(len(out))
	if flt.Offset > 0 {
		if flt.Offset >= len(out) {
			return nil, total, nil
		}
		out = out[flt.Offset:]
	}
	if flt.Limit > 0 && len(out) > flt.Limit {
		out = out[:flt.Limit]
	}
	return out, total, nil
}

func containsStr(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// auditEvent is a small builder for trail rows.
func auditEvent(tenant uuid.UUID, entityType, entityID string, action domain.AuditAction, at time.Time) domain.AuditEvent {
	return domain.AuditEvent{
		ID:         uuid.New(),
		TenantID:   tenant,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Summary:    string(action) + " " + entityType,
		CreatedAt:  at,
	}
}

// --- supplementary journal -------------------------------------------------

// fakeSource is a supplementary journal. It records the tenant it was asked for
// so a test can prove the service never queries a journal with a tenant the
// caller did not present.
type fakeSource struct {
	name       TimelineSource
	events     []TimelineEvent
	err        error
	lastTenant uuid.UUID
}

func (f *fakeSource) Source() TimelineSource { return f.name }

func (f *fakeSource) Events(_ context.Context, tenantID uuid.UUID, _ string, before *time.Time, limit int) ([]TimelineEvent, error) {
	f.lastTenant = tenantID
	if f.err != nil {
		return nil, f.err
	}
	var out []TimelineEvent
	for _, e := range f.events {
		if before != nil && e.OccurredAt.After(*before) {
			continue
		}
		out = append(out, e)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// --- user lookup -----------------------------------------------------------

type fakeLookup struct {
	emails map[uuid.UUID]string
	err    error
	calls  int
}

func (f *fakeLookup) EmailsByIDs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		if e, ok := f.emails[id]; ok {
			out[id] = e
		}
	}
	return out, nil
}
