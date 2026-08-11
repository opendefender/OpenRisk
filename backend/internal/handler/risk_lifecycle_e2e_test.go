// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	applicationrisk "github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/pkg/crq"
)

// End-to-end walk of a risk through the WHOLE lifecycle, over the real HTTP
// handlers, the real use cases and the real repositories — including the
// governance approval that gates RESIDUAL_ACCEPTED.
//
// This runs against sqlite rather than a browser because it is the layer where
// the guards live: the point of the change set is that the SERVER refuses a bad
// transition whichever client asks, so the test drives the server. A Playwright
// journey over the same flow lives in tests/e2e/risk-lifecycle.spec.ts; it needs
// a running stack and is not exercised here.

type lifecycleHarness struct {
	app      *fiber.App
	db       *gorm.DB
	tenantID uuid.UUID
	userID   uuid.UUID
	riskID   string
}

func newLifecycleHarness(t *testing.T) *lifecycleHarness {
	t.Helper()

	dsn := "file:risk_lifecycle_" + uuid.New().String() + "?mode=memory&cache=private"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	// Hand-written DDL, as everywhere else in this package: the domain models
	// carry Postgres defaults sqlite cannot parse, and hand-writing it means a
	// model that gains a column fails here loudly instead of silently.
	require.NoError(t, db.Exec(`CREATE TABLE risks (
		id TEXT PRIMARY KEY, tenant_id TEXT, organization_id TEXT,
		name TEXT, title TEXT NOT NULL, description TEXT,
		probability REAL, impact REAL, score REAL, criticality TEXT,
		impact_legacy INTEGER, probability_legacy INTEGER,
		status TEXT, level TEXT, category_id TEXT,
		lifecycle_state TEXT, lifecycle_phase TEXT,
		created_by TEXT, assigned_to TEXT,
		owner_id TEXT, assignee_id TEXT, reviewer_id TEXT, owner TEXT,
		asset_id TEXT, treatment_plan TEXT, residual_risk REAL, last_mitigated_at DATETIME,
		slexaf REAL, aro REAL, downtime_hours REAL, hourly_downtime_cost_xaf REAL,
		data_loss_cost_xaf REAL, fines_xaf REAL, other_direct_cost_xaf REAL,
		remediation_cost_xaf REAL, mitigation_effectiveness REAL,
		smart_score REAL, smart_level TEXT, smart_factors TEXT, smart_computed_at DATETIME,
		review_interval_days INTEGER, next_review_at DATETIME, last_reviewed_at DATETIME,
		source TEXT, source_cve_id TEXT, external_id TEXT, custom_fields TEXT,
		tags TEXT, frameworks TEXT, control_ids TEXT,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	);`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE risk_assets (risk_id TEXT, asset_id TEXT);`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE risk_histories (
		id TEXT PRIMARY KEY, risk_id TEXT, score REAL, impact INTEGER, probability INTEGER,
		status TEXT, changed_by TEXT, change_type TEXT, created_at DATETIME
	);`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE mitigations (
		id TEXT PRIMARY KEY, tenant_id TEXT, risk_id TEXT,
		title TEXT, description TEXT, status TEXT, priority TEXT,
		owner_id TEXT, assignee_id TEXT, reviewer_id TEXT,
		assigned_to TEXT, progress INTEGER DEFAULT 0,
		reminder_d7_sent_at DATETIME, reminder_d1_sent_at DATETIME,
		created_by TEXT, approved_by TEXT, approved_at DATETIME,
		source TEXT, auto_detected_at DATETIME, scanner_config_id TEXT,
		organization_id TEXT, assignee TEXT, cost INTEGER, mitigation_time INTEGER, due_date DATETIME,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	);`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE mitigation_subactions (
		id TEXT PRIMARY KEY, mitigation_id TEXT, title TEXT, description TEXT,
		completed BOOLEAN DEFAULT 0, completed_at DATETIME, completed_by TEXT,
		completed_source TEXT, auto_detected_at DATETIME, depends_on TEXT,
		"order" INTEGER DEFAULT 0, due_date DATETIME,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	);`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE risk_categories (
		id TEXT PRIMARY KEY, tenant_id TEXT, name TEXT, slug TEXT, description TEXT,
		color TEXT, sort_order INTEGER, active BOOLEAN,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	);`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE approval_requests (
		id TEXT PRIMARY KEY, tenant_id TEXT, workflow_id TEXT, workflow_name TEXT,
		entity_type TEXT, entity_id TEXT, action TEXT, title TEXT, description TEXT,
		payload TEXT, status TEXT, current_step INTEGER, steps TEXT, decisions TEXT,
		requested_by TEXT, resolved_at DATETIME, created_at DATETIME, updated_at DATETIME
	);`).Error)

	database.DB = db

	h := &lifecycleHarness{db: db, tenantID: uuid.New(), userID: uuid.New()}

	riskRepo := repository.NewGormRiskRepository(db)
	planRepo := repository.NewGormMitigationRepository(db)
	subRepo := repository.NewGormMitigationSubActionRepository(db)

	// The real guard adapters, over the real repositories — the whole point.
	transitionUC := applicationrisk.NewTransitionRiskStateUseCase(riskRepo).
		WithMitigations(&e2eMitigationInspector{plans: planRepo, subs: subRepo}).
		WithApprovals(&e2eApprovalChecker{db: db})

	riskHandler := NewRiskHandler(
		applicationrisk.NewCreateRiskUseCase(riskRepo),
		applicationrisk.NewGetRiskUseCase(riskRepo),
		applicationrisk.NewListRisksUseCase(riskRepo),
		applicationrisk.NewUpdateRiskUseCase(riskRepo),
		applicationrisk.NewDeleteRiskUseCase(riskRepo),
		applicationrisk.NewMarkRiskReviewedUseCase(riskRepo),
		transitionUC,
		nil,
		crq.NewQuantifier(0, crq.Reference{}),
	)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		middleware.SetContext(c, &middleware.RequestContext{UserID: h.userID, OrganizationID: h.tenantID})
		return c.Next()
	})
	api := app.Group("/api/v1")
	api.Post("/risks", riskHandler.CreateRisk)
	api.Get("/risks/:id", riskHandler.GetRisk)
	api.Post("/risks/:id/transition", riskHandler.TransitionPhase)
	api.Get("/risks/:id/transitions", riskHandler.ListTransitions)
	api.Post("/risks/:id/mitigations", CreateMitigation)
	api.Post("/mitigations/:id/sub-actions", CreateSubAction)
	api.Post("/mitigations/:id/sub-actions/:aid/complete", CompleteSubAction)

	h.app = app
	return h
}

// --- guard adapters, mirroring cmd/server/lifecycle_wiring.go ---------------

type e2eMitigationInspector struct {
	plans repository.MitigationRepository
	subs  repository.MitigationSubActionRepository
}

func (a *e2eMitigationInspector) SnapshotForRisk(_ context.Context, tenantID, riskID uuid.UUID) ([]applicationrisk.MitigationSnapshot, error) {
	rows, err := a.plans.ListByRiskID(tenantID.String(), riskID)
	if err != nil {
		return nil, err
	}
	out := make([]applicationrisk.MitigationSnapshot, 0, len(rows))
	for i := range rows {
		m := rows[i]
		snap := applicationrisk.MitigationSnapshot{
			ID: m.ID, Ref: "MIT-" + m.ID.String()[:6], Title: m.Title,
			Active: m.Status != domain.MitigationCancelled,
		}
		subs, err := a.subs.List(tenantID.String(), m.ID)
		if err != nil {
			return nil, err
		}
		for _, s := range subs {
			snap.TotalSubActions++
			if s.Completed {
				snap.CompletedSubActions++
			}
		}
		out = append(out, snap)
	}
	return out, nil
}

type e2eApprovalChecker struct{ db *gorm.DB }

func (a *e2eApprovalChecker) HasApprovedAcceptance(_ context.Context, tenantID, riskID uuid.UUID) (bool, *uuid.UUID, error) {
	type row struct {
		ID     string
		Status string
	}
	var rows []row
	if err := a.db.Raw(
		`SELECT id, status FROM approval_requests WHERE tenant_id = ? AND entity_type = 'risk_acceptance' AND entity_id = ?`,
		tenantID, riskID.String()).Scan(&rows).Error; err != nil {
		return false, nil, err
	}
	var pending *uuid.UUID
	for _, r := range rows {
		switch domain.ApprovalStatus(r.Status) {
		case domain.ApprovalApproved:
			return true, nil, nil
		case domain.ApprovalPending:
			if pending == nil {
				if id, err := uuid.Parse(r.ID); err == nil {
					pending = &id
				}
			}
		}
	}
	return false, pending, nil
}

func (h *lifecycleHarness) do(t *testing.T, method, path string, body interface{}) (int, map[string]interface{}) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	out := map[string]interface{}{}
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

// get re-reads the whole risk through the API.
func (h *lifecycleHarness) get(t *testing.T) map[string]interface{} {
	t.Helper()
	code, body := h.do(t, http.MethodGet, "/api/v1/risks/"+h.riskID, nil)
	require.Equal(t, http.StatusOK, code)
	return body
}

// state re-reads the risk through the API, so the assertion is on what a client
// would actually see.
func (h *lifecycleHarness) state(t *testing.T) (string, string, string) {
	t.Helper()
	code, body := h.do(t, http.MethodGet, "/api/v1/risks/"+h.riskID, nil)
	require.Equal(t, http.StatusOK, code)
	str := func(k string) string {
		if v, ok := body[k].(string); ok {
			return v
		}
		return ""
	}
	return str("lifecycle_state"), str("status"), str("lifecycle_phase")
}

func (h *lifecycleHarness) transition(t *testing.T, to string) (int, map[string]interface{}) {
	t.Helper()
	return h.do(t, http.MethodPost, "/api/v1/risks/"+h.riskID+"/transition",
		map[string]interface{}{"to": to, "comment": "e2e"})
}

// approveResidualAcceptance writes the validated governance approval the
// RESIDUAL_ACCEPTED guard requires.
func (h *lifecycleHarness) approveResidualAcceptance(t *testing.T, status domain.ApprovalStatus) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, h.db.Exec(
		`INSERT INTO approval_requests (id, tenant_id, entity_type, entity_id, action, title, status, current_step, steps, decisions, requested_by, created_at, updated_at)
		 VALUES (?, ?, 'risk_acceptance', ?, 'accept', 'Acceptation du risque résiduel', ?, 0, '[]', '[]', ?, ?, ?)`,
		id, h.tenantID, h.riskID, string(status), h.userID, time.Now(), time.Now()).Error)
	return id
}

// ===========================================================================

func TestRiskLifecycle_EndToEnd_WithResidualAcceptance(t *testing.T) {
	h := newLifecycleHarness(t)

	// --- 1. Create --------------------------------------------------------
	categoryID := uuid.New()
	require.NoError(t, h.db.Exec(
		`INSERT INTO risk_categories (id, tenant_id, name, slug, color, sort_order, active) VALUES (?, ?, ?, ?, ?, 0, 1)`,
		categoryID, h.tenantID, "Cybersécurité", "cybersecurite", "critical").Error)

	code, created := h.do(t, http.MethodPost, "/api/v1/risks", map[string]interface{}{
		"title":       "Exposition du bucket S3 de production",
		"description": "Bucket accessible publiquement, contenant des exports clients.",
		"impact":      8.0,
		"probability": 0.6,
		"tags":        []string{"cloud", "production"},
		"category_id": categoryID.String(),
	})
	require.Equal(t, http.StatusCreated, code, "creation must succeed: %v", created)
	h.riskID, _ = created["id"].(string)
	require.NotEmpty(t, h.riskID)

	state, status, phase := h.state(t)
	require.Equal(t, "draft", state, "a new risk enters the lifecycle at DRAFT")
	require.Equal(t, "DRAFT", status, "status is DERIVED from the state")
	require.Equal(t, "identified", phase, "phase is DERIVED from the state")

	// The creator becomes the owner: a risk nobody answers for is unactionable.
	require.Equal(t, h.userID.String(), created["owner_id"], "the creator must own the risk by default")

	// The controlled category comes back RESOLVED, not merely as an id: it is
	// what the register's "Catégorie" column renders, and returning only the id
	// made a classified risk read as unclassified. Found by running the app.
	read := h.get(t)
	category, ok := read["category"].(map[string]interface{})
	require.True(t, ok, "the category relation must be preloaded: %v", read["category"])
	require.Equal(t, "Cybersécurité", category["name"])

	// …and the tags stay in their own field, never leaking into the compliance
	// reference — the "étiquette affichée comme framework" bug.
	require.Equal(t, []interface{}{"cloud", "production"}, read["tags"])
	require.Empty(t, read["control_mappings"], "an unmapped risk has NO compliance reference, not a tag standing in for one")

	// --- 2. Draft → Identified → Assessed → Treatment planned -------------
	for _, to := range []string{"identified", "assessed", "treatment_planned"} {
		code, body := h.transition(t, to)
		require.Equal(t, http.StatusOK, code, "%s should be reachable: %v", to, body)
	}
	state, _, _ = h.state(t)
	require.Equal(t, "treatment_planned", state)

	// A step that skips the graph is refused whatever the client asks.
	code, body := h.transition(t, "closed_forever")
	require.Equal(t, http.StatusBadRequest, code, "an unknown state must be refused: %v", body)

	// --- 3. GUARD 1: IN_TREATMENT needs an active mitigation --------------
	code, body = h.transition(t, "in_treatment")
	require.Equal(t, http.StatusBadRequest, code, "treatment must be blocked with no mitigation")
	require.Contains(t, body["error"], "mitigation", "the refusal must name the blocker: %v", body)

	state, _, _ = h.state(t)
	require.Equal(t, "treatment_planned", state, "a refused transition must not have moved the risk")

	// The stepper's contract reports the same thing, with the guard named.
	code, view := h.do(t, http.MethodGet, "/api/v1/risks/"+h.riskID+"/transitions", nil)
	require.Equal(t, http.StatusOK, code)
	blocked := findOption(view, "in_treatment")
	require.NotNil(t, blocked, "a blocked option must be RETURNED, not hidden: %v", view)
	require.Equal(t, false, blocked["allowed"])
	require.Equal(t, string(domain.GuardActiveMitigation), blocked["guard"])
	require.NotEmpty(t, blocked["reason"])

	// --- 4. Create the mitigation plan and its checklist -------------------
	code, plan := h.do(t, http.MethodPost, "/api/v1/risks/"+h.riskID+"/mitigations", map[string]interface{}{
		"title":       "Fermer l'accès public et activer le chiffrement",
		"description": "Retirer la policy publique, activer SSE-KMS, rejouer un scan.",
		"priority":    "high",
	})
	require.Equal(t, http.StatusCreated, code, "mitigation creation: %v", plan)
	planID, _ := plan["id"].(string)
	require.NotEmpty(t, planID)

	// With no sub-actions and status PLANNED, progress reads 0 — and, crucially,
	// it is COMPUTED, not the literal zero the old code returned for everything.
	require.Equal(t, float64(0), plan["progress"])

	subIDs := make([]string, 0, 2)
	for _, title := range []string{"Retirer la policy publique", "Activer SSE-KMS"} {
		code, sub := h.do(t, http.MethodPost, "/api/v1/mitigations/"+planID+"/sub-actions",
			map[string]interface{}{"title": title})
		require.Equal(t, http.StatusCreated, code, "sub-action: %v", sub)
		id, _ := sub["id"].(string)
		subIDs = append(subIDs, id)
	}

	// --- 5. The guard is satisfied: treatment starts ----------------------
	code, body = h.transition(t, "in_treatment")
	require.Equal(t, http.StatusOK, code, "treatment must be allowed once a plan exists: %v", body)

	state, status, phase = h.state(t)
	require.Equal(t, "in_treatment", state)
	require.Equal(t, "in_progress", status, "the legacy status follows the state")
	require.Equal(t, "treated", phase, "the legacy phase follows the state")

	// --- 6. GUARD 2: MITIGATED needs 100% of sub-actions -------------------
	code, body = h.transition(t, "mitigated")
	require.Equal(t, http.StatusBadRequest, code, "closing treatment with open steps must be refused")
	require.Contains(t, body["error"], "2", "the refusal must quote the remaining count: %v", body)

	// Complete the first: still blocked, and the count in the message drops.
	code, _ = h.do(t, http.MethodPost, "/api/v1/mitigations/"+planID+"/sub-actions/"+subIDs[0]+"/complete", nil)
	require.Equal(t, http.StatusOK, code)

	code, body = h.transition(t, "mitigated")
	require.Equal(t, http.StatusBadRequest, code, "one of two done is not done")
	require.Contains(t, body["error"], "1", "the count must reflect reality: %v", body)

	// Progress is recomputed server-side on that mutation — 1 of 2 = 50 %.
	var progress int
	require.NoError(t, h.db.Raw(`SELECT progress FROM mitigations WHERE id = ?`, planID).Scan(&progress).Error)
	require.Equal(t, 50, progress, "progress must be computed from the checklist, never entered")

	// --- 7. GUARD 3: RESIDUAL_ACCEPTED needs a VALIDATED approval ---------
	// The branch is reachable from IN_TREATMENT, so it can be tried right here.
	code, body = h.transition(t, "residual_accepted")
	require.Equal(t, http.StatusBadRequest, code, "residual acceptance without an approval must be refused")
	require.Contains(t, body["error"], "Gouvernance", "the refusal must point at Governance: %v", body)

	// A request that EXISTS but is pending is not an approval.
	pendingID := h.approveResidualAcceptance(t, domain.ApprovalPending)
	code, body = h.transition(t, "residual_accepted")
	require.Equal(t, http.StatusBadRequest, code, "a pending request is not an approval")
	require.Contains(t, body["error"], pendingID.String()[:8], "the pending request must be named: %v", body)

	// Approve it. Now the branch opens — even with a sub-action still open,
	// because accepting residual risk IS the decision to stop treating it.
	require.NoError(t, h.db.Exec(`UPDATE approval_requests SET status = 'approved' WHERE id = ?`, pendingID).Error)

	code, view = h.do(t, http.MethodGet, "/api/v1/risks/"+h.riskID+"/transitions", nil)
	require.Equal(t, http.StatusOK, code)
	accepted := findOption(view, "residual_accepted")
	require.NotNil(t, accepted)
	require.Equal(t, true, accepted["allowed"], "an approved acceptance must unblock: %v", accepted)

	code, body = h.transition(t, "residual_accepted")
	require.Equal(t, http.StatusOK, code, "approved acceptance must go through: %v", body)

	state, status, _ = h.state(t)
	require.Equal(t, "residual_accepted", state)
	require.Equal(t, "accepted", status)

	// --- 8. Back into treatment, finish the work, then MITIGATED ----------
	code, body = h.transition(t, "in_treatment")
	require.Equal(t, http.StatusOK, code, "an accepted risk can be retreated: %v", body)

	code, _ = h.do(t, http.MethodPost, "/api/v1/mitigations/"+planID+"/sub-actions/"+subIDs[1]+"/complete", nil)
	require.Equal(t, http.StatusOK, code)

	require.NoError(t, h.db.Raw(`SELECT progress FROM mitigations WHERE id = ?`, planID).Scan(&progress).Error)
	require.Equal(t, 100, progress, "the last completion must take the plan to 100 %")

	code, body = h.transition(t, "mitigated")
	require.Equal(t, http.StatusOK, code, "everything done → mitigated: %v", body)

	state, status, phase = h.state(t)
	require.Equal(t, "mitigated", state)
	require.Equal(t, "mitigated", status)
	require.Equal(t, "monitored", phase)

	// --- 9. Close, then reopen -------------------------------------------
	code, body = h.transition(t, "closed")
	require.Equal(t, http.StatusOK, code, "closing: %v", body)
	state, status, _ = h.state(t)
	require.Equal(t, "closed", state)
	require.Equal(t, "closed", status)

	// A closed risk is not a grave.
	code, body = h.transition(t, "reopened")
	require.Equal(t, http.StatusOK, code, "reopening: %v", body)
	code, body = h.transition(t, "assessed")
	require.Equal(t, http.StatusOK, code, "a reopened risk goes back into the flow: %v", body)

	state, status, _ = h.state(t)
	require.Equal(t, "assessed", state)
	require.Equal(t, "open", status)

	// --- 10. Everything is on the record ----------------------------------
	var audits int64
	require.NoError(t, h.db.Raw(`SELECT COUNT(*) FROM risk_histories WHERE risk_id = ?`, h.riskID).Scan(&audits).Error)
	require.Greater(t, audits, int64(0), "the walk must have left a history trail")
}

// findOption pulls one entry out of the transitions view.
func findOption(view map[string]interface{}, to string) map[string]interface{} {
	opts, ok := view["options"].([]interface{})
	if !ok {
		return nil
	}
	for _, raw := range opts {
		opt, ok := raw.(map[string]interface{})
		if ok && opt["to"] == to {
			return opt
		}
	}
	return nil
}
