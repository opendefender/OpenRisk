//go:build crosstenant

// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package crosstenant

import (
	"encoding/json"
	"fmt"
	"time"
)

// seeder creates one object of a kind for p through the API, so the row carries
// exactly the tenant columns production writes. It returns the ids it created,
// keyed by kind (a sub-action seed also returns its mitigation and risk), or nil
// when the create route refused. Every text field carries the canary.
type seeder func(h *harness, p *principal) map[Kind]string

func jb(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

var seeders map[Kind]seeder

func init() { seeders = seederTable() }

func seederTable() map[Kind]seeder {
	return map[Kind]seeder{
		KindRisk: func(h *harness, p *principal) map[Kind]string {
			c := h.canary(p, KindRisk)
			id := h.create(p, "/api/v1/risks", jb(map[string]any{
				"title": c, "description": c, "impact": 6, "probability": 0.4,
			}))
			return one(KindRisk, id)
		},
		KindMitigation: func(h *harness, p *principal) map[Kind]string {
			r := h.seedFresh(p, KindRisk)
			if r == nil {
				return nil
			}
			c := h.canary(p, KindMitigation)
			id := h.create(p, "/api/v1/risks/"+r[KindRisk]+"/mitigations", jb(map[string]any{
				"title": c, "description": c, "status": "PLANNED",
			}))
			return with(one(KindMitigation, id), KindRisk, r[KindRisk])
		},
		KindSubAction: func(h *harness, p *principal) map[Kind]string {
			m := h.seedFresh(p, KindMitigation)
			if m == nil {
				return nil
			}
			c := h.canary(p, KindSubAction)
			status, body := h.do(p, "POST", "/api/v1/mitigations/"+m[KindMitigation]+"/sub-actions", jb(map[string]any{"title": c}))
			if status < 200 || status > 299 {
				return nil
			}
			id := h.idOf(p, body)
			return with(with(one(KindSubAction, id), KindMitigation, m[KindMitigation]), KindRisk, m[KindRisk])
		},
		KindAsset: func(h *harness, p *principal) map[Kind]string {
			c := h.canary(p, KindAsset)
			return one(KindAsset, h.create(p, "/api/v1/assets", jb(map[string]any{
				"name": c, "type": "Server", "criticality": "HIGH", "owner": c,
			})))
		},
		KindIncident: func(h *harness, p *principal) map[Kind]string {
			c := h.canary(p, KindIncident)
			return one(KindIncident, h.create(p, "/api/v1/incidents", jb(map[string]any{
				"title": c, "description": c, "severity": "high",
			})))
		},
		KindCustomField: func(h *harness, p *principal) map[Kind]string {
			c := h.canary(p, KindCustomField)
			return one(KindCustomField, h.create(p, "/api/v1/custom-fields", jb(map[string]any{
				"name": fmt.Sprintf("cf_%s_%s", p.name, randHex(3)), "display_name": c, "field_type": "text", "scope": "risk",
			})))
		},
		KindSavedView: func(h *harness, p *principal) map[Kind]string {
			c := h.canary(p, KindSavedView)
			return one(KindSavedView, h.create(p, "/api/v1/saved-views", jb(map[string]any{
				"table_id": "risks", "name": c + "-" + randHex(3), "filters": map[string]any{},
			})))
		},
		KindCategory: func(h *harness, p *principal) map[Kind]string {
			c := h.canary(p, KindCategory)
			return one(KindCategory, h.create(p, "/api/v1/risk-categories", jb(map[string]any{"name": c + "-" + randHex(3), "description": c})))
		},
		KindTeam: func(h *harness, p *principal) map[Kind]string {
			c := h.canary(p, KindTeam)
			return one(KindTeam, h.create(p, "/api/v1/teams", jb(map[string]any{"name": c, "description": c})))
		},
		KindRule: func(h *harness, p *principal) map[Kind]string {
			c := h.canary(p, KindRule)
			return one(KindRule, h.create(p, "/api/v1/automation/rules", jb(map[string]any{
				"name": c, "description": c, "trigger": "risk_created", "actions": []map[string]any{{"type": "notify"}},
			})))
		},
		KindReport: func(h *harness, p *principal) map[Kind]string {
			c := h.canary(p, KindReport)
			return one(KindReport, h.create(p, "/api/v1/reports", jb(map[string]any{
				"type": "executive_summary", "title": c, "format": "pdf",
			})))
		},
		KindBoardReport: func(h *harness, p *principal) map[Kind]string {
			return one(KindBoardReport, h.create(p, "/api/v1/reports/board", `{}`))
		},
		KindTemplate: func(h *harness, p *principal) map[Kind]string {
			c := h.canary(p, KindTemplate)
			return one(KindTemplate, h.create(p, "/api/v1/vendor-questionnaire-templates", jb(map[string]any{
				"name": c, "description": c, "language": "en",
				"questions": []map[string]any{{"text": c, "answer_type": "text"}},
			})))
		},
		KindEvidence: func(h *harness, p *principal) map[Kind]string {
			c := h.canary(p, KindEvidence)
			return one(KindEvidence, h.create(p, "/api/v1/evidence", jb(map[string]any{"title": c, "description": c})))
		},
		KindFramework: func(h *harness, p *principal) map[Kind]string {
			c := h.canary(p, KindFramework)
			return one(KindFramework, h.create(p, "/api/v1/compliance/frameworks", jb(map[string]any{"name": c, "description": c})))
		},
		KindAudit: func(h *harness, p *principal) map[Kind]string {
			f := h.seedFresh(p, KindFramework)
			if f == nil {
				return nil
			}
			c := h.canary(p, KindAudit)
			id := h.create(p, "/api/v1/compliance/audits", jb(map[string]any{"title": c, "summary": c, "framework_id": f[KindFramework]}))
			return with(one(KindAudit, id), KindFramework, f[KindFramework])
		},
		KindAssetPeer: func(h *harness, p *principal) map[Kind]string {
			c := h.canary(p, KindAssetPeer)
			return one(KindAssetPeer, h.create(p, "/api/v1/assets", jb(map[string]any{"name": c, "type": "Server", "criticality": "LOW"})))
		},
		KindAssetDep: func(h *harness, p *principal) map[Kind]string {
			a, b := h.seedFresh(p, KindAsset), h.seedFresh(p, KindAssetPeer)
			if a == nil || b == nil {
				return nil
			}
			id := h.create(p, "/api/v1/asset-dependencies", jb(map[string]any{
				"source_asset_id": a[KindAsset], "target_asset_id": b[KindAssetPeer], "description": h.canary(p, KindAssetDep),
			}))
			return with(with(one(KindAssetDep, id), KindAsset, a[KindAsset]), KindAssetPeer, b[KindAssetPeer])
		},
		KindControl: func(h *harness, p *principal) map[Kind]string {
			f := h.seedFresh(p, KindFramework)
			if f == nil {
				return nil
			}
			c := h.canary(p, KindControl)
			id := h.create(p, "/api/v1/compliance/frameworks/"+f[KindFramework]+"/controls", jb(map[string]any{
				"name": c, "description": c, "reference_code": "XT-1",
			}))
			return with(one(KindControl, id), KindFramework, f[KindFramework])
		},
		KindAssessment: func(h *harness, p *principal) map[Kind]string {
			v, tpl := h.seedFresh(p, KindVendor), h.seedShared(p, KindTemplate)
			if v == nil || tpl == nil {
				return nil
			}
			id := h.create(p, "/api/v1/vendors/"+v[KindVendor]+"/assessments", jb(map[string]any{
				"template_id": tpl[KindTemplate], "due_at": time.Now().UTC().AddDate(0, 0, 30).Format(time.RFC3339),
				"contact_email": "vendor-" + randHex(3) + "@xt.example.com", "contact_language": "en",
			}))
			return with(one(KindAssessment, id), KindVendor, v[KindVendor])
		},
		KindScoreConfig: func(h *harness, p *principal) map[Kind]string {
			// The id is the caller's choice (?id=), so it is chosen here.
			c := h.canary(p, KindScoreConfig)
			id := c + "-" + randHex(3)
			status, resp := h.do(p, "POST", "/api/v1/score-engine/configs?id="+id, jb(map[string]any{
				"name": c, "description": c, "base_formula": "impact*probability",
				"risk_matrix_thresholds": map[string]int{"low": 3, "medium": 8, "high": 15, "critical": 22},
			}))
			if status != 201 {
				h.t.Logf("seed: POST score-engine/configs as %s answered %d %.200s", p.name, status, resp)
				return nil
			}
			p.learn(id)
			return map[Kind]string{KindScoreConfig: id}
		},
		KindVendor: func(h *harness, p *principal) map[Kind]string {
			c := h.canary(p, KindVendor)
			return one(KindVendor, h.create(p, "/api/v1/assets", jb(map[string]any{
				"name": c, "type": "Vendor", "category": "vendor", "criticality": "HIGH", "owner": c,
				"attributes": map[string]any{"legal_name": c, "country": "FR", "service_provided": c, "data_shared": "aucune"},
			})))
		},
		KindInternalControl: func(h *harness, p *principal) map[Kind]string {
			c := h.canary(p, KindInternalControl)
			return one(KindInternalControl, h.create(p, "/api/v1/compliance/internal-controls", jb(map[string]any{
				"name": c, "description": c, "applies_to": "all",
			})))
		},
		KindTenant: func(h *harness, p *principal) map[Kind]string {
			return map[Kind]string{KindTenant: p.tenantID}
		},
		KindWorkflow: func(h *harness, p *principal) map[Kind]string {
			// One workflow per request type and tenant: a fresh victim first
			// removes the previous one of the same type.
			_, body := h.do(p, "GET", "/api/v1/governance/workflows", "")
			var existing []struct {
				ID          string `json:"id"`
				RequestType string `json:"request_type"`
			}
			_ = json.Unmarshal([]byte(body), &existing)
			for _, w := range existing {
				if w.RequestType == "security_exception" {
					h.do(p, "DELETE", "/api/v1/governance/workflows/"+w.ID, "")
				}
			}
			c := h.canary(p, KindWorkflow)
			return one(KindWorkflow, h.create(p, "/api/v1/governance/workflows", jb(map[string]any{
				"name": c, "request_type": "security_exception", "steps": []map[string]any{{"approver_role": "admin"}},
			})))
		},
		KindApproval: func(h *harness, p *principal) map[Kind]string {
			// An approval needs a workflow for its request type, one per tenant
			// and type: the first call creates it, later ones answer 409.
			h.do(p, "POST", "/api/v1/governance/workflows", jb(map[string]any{
				"name": h.canary(p, KindApproval), "request_type": "risk_acceptance",
				"steps": []map[string]any{{"approver_role": "admin"}},
			}))
			r := h.seedFresh(p, KindRisk)
			if r == nil {
				return nil
			}
			c := h.canary(p, KindApproval)
			id := h.create(p, "/api/v1/governance/approvals", jb(map[string]any{
				"title": c, "entity_type": "risk_acceptance", "entity_id": r[KindRisk], "request_type": "risk_acceptance",
			}))
			return with(one(KindApproval, id), KindRisk, r[KindRisk])
		},
		KindDelegation: func(h *harness, p *principal) map[Kind]string {
			u := h.seedFresh(p, KindUser)
			if u == nil {
				return nil
			}
			id := h.create(p, "/api/v1/governance/delegations", jb(map[string]any{
				"delegate_id": u[KindUser], "permissions": []string{"risks:read"},
				"starts_at": "2026-01-01T00:00:00Z", "ends_at": "2035-01-01T00:00:00Z",
			}))
			return with(one(KindDelegation, id), KindUser, u[KindUser])
		},
		KindMember: func(h *harness, p *principal) map[Kind]string {
			u := h.seedFresh(p, KindUser)
			if u == nil {
				return nil
			}
			_, body := h.do(p, "GET", "/api/v1/organization/members", "")
			var out struct {
				Items []struct {
					MemberID string `json:"member_id"`
					UserID   string `json:"user_id"`
				} `json:"items"`
			}
			_ = json.Unmarshal([]byte(body), &out)
			for _, m := range out.Items {
				if m.UserID == u[KindUser] {
					p.learn(m.MemberID)
					return with(one(KindMember, m.MemberID), KindUser, u[KindUser])
				}
			}
			return nil
		},
		KindInvitation: func(h *harness, p *principal) map[Kind]string {
			return one(KindInvitation, h.create(p, "/api/v1/organization/invitations", jb(map[string]any{
				"email": "xt-" + h.run + "-" + p.name + "-i" + randHex(3) + "@xt.example.com", "role": "user",
			})))
		},
		KindUser: func(h *harness, p *principal) map[Kind]string {
			n := randHex(4)
			return one(KindUser, h.create(p, "/api/v1/users", jb(map[string]any{
				"email": "xt-" + h.run + "-" + p.name + "-u" + n + "@xt.example.com", "username": "xt" + n + p.name,
				"full_name": h.canary(p, KindUser), "password": testPassword,
			})))
		},
		KindMitigationInReview: func(h *harness, p *principal) map[Kind]string {
			m := h.seedFresh(p, KindMitigation)
			if m == nil {
				return nil
			}
			if st, _ := h.do(p, "PATCH", "/api/v1/mitigations/"+m[KindMitigation], `{"status":"REVIEW"}`); st != 200 {
				return nil
			}
			return with(one(KindMitigationInReview, m[KindMitigation]), KindRisk, m[KindRisk])
		},
		KindSubActionDone: func(h *harness, p *principal) map[Kind]string {
			s := h.seedFresh(p, KindSubAction)
			if s == nil {
				return nil
			}
			if st, _ := h.do(p, "POST", "/api/v1/mitigations/"+s[KindMitigation]+"/sub-actions/"+s[KindSubAction]+"/complete", `{}`); st != 200 {
				return nil
			}
			s[KindSubActionDone] = s[KindSubAction]
			return s
		},
		KindIncidentPM: func(h *harness, p *principal) map[Kind]string {
			i := h.seedFresh(p, KindIncident)
			if i == nil {
				return nil
			}
			c := h.canary(p, KindIncidentPM)
			if st, _ := h.do(p, "PUT", "/api/v1/incidents/"+i[KindIncident]+"/post-mortem", jb(map[string]any{
				"summary": c, "root_cause": c, "impact": c,
				"timeline":           []map[string]any{{"at": "2026-01-01T00:00:00Z", "title": c, "kind": "detection"}},
				"corrective_actions": []map[string]any{{"id": "ca1", "title": c, "status": "open"}},
			})); st != 200 {
				return nil
			}
			i[KindIncidentPM] = i[KindIncident]
			return i
		},
		KindRemediation: func(h *harness, p *principal) map[Kind]string {
			c := h.canary(p, KindRemediation)
			return one(KindRemediation, h.create(p, "/api/v1/compliance/remediations", jb(map[string]any{"title": c, "description": c})))
		},
	}
}

func one(k Kind, id string) map[Kind]string {
	if id == "" {
		return nil
	}
	return map[Kind]string{k: id}
}

func with(m map[Kind]string, k Kind, id string) map[Kind]string {
	if m == nil {
		return nil
	}
	m[k] = id
	return m
}

// seedFresh creates a new object of kind for p, never shared with another
// probe: destructive and state-changing probes each get a victim of their own.
func (h *harness) seedFresh(p *principal, kind Kind) map[Kind]string {
	s, ok := seeders[kind]
	if !ok {
		return nil
	}
	return s(h, p)
}

// seedShared returns the memoised seed of kind for p, creating it once.
func (h *harness) seedShared(p *principal, kind Kind) map[Kind]string {
	p.mu.Lock()
	m, ok := p.memo[kind]
	p.mu.Unlock()
	if ok {
		return m
	}
	m = h.seedFresh(p, kind)
	p.mu.Lock()
	p.memo[kind] = m
	p.mu.Unlock()
	return m
}
