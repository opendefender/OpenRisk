// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/application/governance"
	"github.com/opendefender/openrisk/internal/application/ownership"
	"github.com/opendefender/openrisk/internal/domain"
	handlers "github.com/opendefender/openrisk/internal/handler"
	"github.com/opendefender/openrisk/internal/infrastructure/audittrail"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

// The integration test for #302: the three transfer-owner routes, driven over
// HTTP through the real handler, use case, GORM stores, audit middleware, GORM
// audit plugin and chained audit repository, on sqlite.
//
// There is deliberately no risk_histories table. Risk.AfterSave writes a
// snapshot there, so if the store ever let the model's save hooks run on its
// empty struct, the transfer would fail here instead of silently recording a
// snapshot of risk uuid.Nil in production.

type transferMembers struct{ rows []domain.OrganizationMember }

func (m *transferMembers) ListMembers(_ context.Context, tenantID uuid.UUID) ([]domain.OrganizationMember, error) {
	var out []domain.OrganizationMember
	for _, r := range m.rows {
		if r.OrganizationID == tenantID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *transferMembers) GetMember(_ context.Context, tenantID, userID uuid.UUID) (*domain.OrganizationMember, error) {
	for i := range m.rows {
		if m.rows[i].OrganizationID == tenantID && m.rows[i].UserID == userID {
			return &m.rows[i], nil
		}
	}
	return nil, nil
}

type transferNotice struct {
	userID       uuid.UUID
	tenantID     uuid.UUID
	resourceType string
}

type transferNotifier struct{ sent []transferNotice }

func (n *transferNotifier) NotifyInApp(userID, tenantID uuid.UUID, _ domain.NotificationType, _, _ string, _ *uuid.UUID, resourceType string) error {
	n.sent = append(n.sent, transferNotice{userID, tenantID, resourceType})
	return nil
}

type transferFixture struct {
	app      *fiber.App
	db       *gorm.DB
	chain    *repository.GormAuditChainRepository
	notifier *transferNotifier

	tenantA, tenantB   uuid.UUID
	actor, alice, bob  uuid.UUID
	stranger           uuid.UUID
	riskA, mitigationA string
	incidentA          string
	riskB              string
}

func newTransferFixture(t *testing.T) *transferFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1) // one connection: each ":memory:" connection is its own database

	for _, tbl := range []struct {
		ddl, name string
		model     interface{}
	}{
		{`CREATE TABLE risks (id TEXT PRIMARY KEY)`, "risks", &domain.Risk{}},
		{`CREATE TABLE mitigations (id TEXT PRIMARY KEY)`, "mitigations", &domain.Mitigation{}},
		{`CREATE TABLE incidents (id INTEGER PRIMARY KEY AUTOINCREMENT)`, "incidents", &domain.Incident{}},
	} {
		if err := db.Exec(tbl.ddl).Error; err != nil {
			t.Fatalf("create %s: %v", tbl.name, err)
		}
		if err := sqliteschema.Reconcile(db, tbl.name, tbl.model); err != nil {
			t.Fatalf("reconcile %s: %v", tbl.name, err)
		}
	}
	if err := db.AutoMigrate(&domain.AuditEvent{}, &domain.AuditChainSeal{}, &domain.AuditRetentionPolicy{}); err != nil {
		t.Fatalf("migrate audit: %v", err)
	}
	chain := repository.NewGormAuditChainRepository(db)
	if err := db.Use(audittrail.New(db).WithAppender(chain)); err != nil {
		t.Fatalf("install audit plugin: %v", err)
	}

	f := &transferFixture{
		db: db, chain: chain, notifier: &transferNotifier{},
		tenantA: uuid.New(), tenantB: uuid.New(),
		actor: uuid.New(), alice: uuid.New(), bob: uuid.New(), stranger: uuid.New(),
		riskA: uuid.New().String(), mitigationA: uuid.New().String(), riskB: uuid.New().String(),
	}

	// Rows are inserted with plain SQL so no model hook runs during setup.
	mustExec := func(q string, args ...interface{}) {
		t.Helper()
		if err := db.Exec(q, args...).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	mustExec(`INSERT INTO risks (id, tenant_id, organization_id, name, title, owner_id) VALUES (?, ?, ?, ?, ?, ?)`,
		f.riskA, f.tenantA.String(), f.tenantA.String(), "ERP ransomware", "ERP ransomware", f.alice.String())
	mustExec(`INSERT INTO risks (id, tenant_id, organization_id, name, title, owner_id) VALUES (?, ?, ?, ?, ?, ?)`,
		f.riskB, f.tenantB.String(), f.tenantB.String(), "Tenant B risk", "Tenant B risk", f.stranger.String())
	mustExec(`INSERT INTO mitigations (id, tenant_id, risk_id, title, owner_id) VALUES (?, ?, ?, ?, ?)`,
		f.mitigationA, f.tenantA.String(), f.riskA, "Offline backups", f.alice.String())
	mustExec(`INSERT INTO incidents (tenant_id, title, owner_id) VALUES (?, ?, ?)`,
		f.tenantA.String(), "Phishing wave", f.alice.String())
	var incidentID int
	if err := db.Raw(`SELECT id FROM incidents WHERE title = ?`, "Phishing wave").Scan(&incidentID).Error; err != nil {
		t.Fatalf("incident id: %v", err)
	}
	f.incidentA = itoaTransfer(incidentID)

	members := &transferMembers{rows: []domain.OrganizationMember{
		{OrganizationID: f.tenantA, UserID: f.actor, IsActive: true},
		{OrganizationID: f.tenantA, UserID: f.alice, IsActive: true},
		{OrganizationID: f.tenantA, UserID: f.bob, IsActive: true},
		{OrganizationID: f.tenantB, UserID: f.stranger, IsActive: true},
	}}
	svc := ownership.NewService().WithMembers(members).WithNotifier(f.notifier)
	uc := ownership.NewTransferOwnershipUseCase(svc, map[ownership.TransferEntity]ownership.OwnerStore{
		ownership.TransferRisk:       repository.NewGormRiskOwnerStore(db),
		ownership.TransferMitigation: repository.NewGormMitigationOwnerStore(db),
		ownership.TransferIncident:   repository.NewGormIncidentOwnerStore(db),
	}).WithAudit(governance.NewAuditRecorder(chain))
	h := handlers.NewOwnershipTransferHandler(uc)

	app := fiber.New()
	api := app.Group("/api/v1")
	api.Use(func(c *fiber.Ctx) error {
		middleware.SetContext(c, &middleware.RequestContext{UserID: f.actor, OrganizationID: f.tenantA})
		c.SetUserContext(audittrail.WithActor(c.UserContext(), audittrail.Actor{ID: &f.actor, TenantID: f.tenantA}))
		return c.Next()
	})
	api.Use(middleware.AuditMutations(chain))
	api.Post("/risks/:id/transfer-owner", h.TransferRiskOwner)
	api.Post("/mitigations/:id/transfer-owner", h.TransferMitigationOwner)
	api.Post("/incidents/:id/transfer-owner", h.TransferIncidentOwner)
	f.app = app
	return f
}

func itoaTransfer(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func (f *transferFixture) post(t *testing.T, path string, newOwner string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1"+path, strings.NewReader(`{"new_owner_id":"`+newOwner+`"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.app.Test(req, 5000)
	if err != nil {
		t.Fatalf("request %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	body := map[string]interface{}{}
	_ = json.Unmarshal(raw, &body)
	return resp.StatusCode, body
}

func (f *transferFixture) ownerOf(t *testing.T, table, id string) string {
	t.Helper()
	var owner string
	if err := f.db.Raw(`SELECT owner_id FROM `+table+` WHERE id = ?`, id).Scan(&owner).Error; err != nil {
		t.Fatalf("read owner: %v", err)
	}
	return owner
}

func (f *transferFixture) transferEvents(t *testing.T) []domain.AuditEvent {
	t.Helper()
	all, err := f.chain.ListAll(t.Context(), f.tenantA, domain.AuditEventFilter{})
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	var out []domain.AuditEvent
	for _, ev := range all {
		if ev.Action == domain.AuditActionTransfer {
			out = append(out, ev)
		}
	}
	return out
}

func TestOwnershipTransferRoutes_Success(t *testing.T) {
	f := newTransferFixture(t)

	for _, tc := range []struct {
		entity, table, path, id string
	}{
		{"risk", "risks", "/risks/", f.riskA},
		{"mitigation", "mitigations", "/mitigations/", f.mitigationA},
		{"incident", "incidents", "/incidents/", f.incidentA},
	} {
		t.Run(tc.entity, func(t *testing.T) {
			status, body := f.post(t, tc.path+tc.id+"/transfer-owner", f.bob.String())
			if status != http.StatusOK {
				t.Fatalf("status = %d, body = %v", status, body)
			}
			if body["previous_owner_id"] != f.alice.String() || body["owner_id"] != f.bob.String() || body["entity_type"] != tc.entity {
				t.Fatalf("response = %v", body)
			}
			if got := f.ownerOf(t, tc.table, tc.id); got != f.bob.String() {
				t.Fatalf("stored owner = %q, want %s", got, f.bob)
			}
		})
	}

	// One transfer entry per request, each saying who moved what from whom to whom.
	events := f.transferEvents(t)
	if len(events) != 3 {
		t.Fatalf("transfer entries = %d, want 3", len(events))
	}
	byType := map[string]domain.AuditEvent{}
	for _, ev := range events {
		byType[ev.EntityType] = ev
	}
	for entity, id := range map[string]string{"risk": f.riskA, "mitigation": f.mitigationA, "incident": f.incidentA} {
		ev, ok := byType[entity]
		if !ok {
			t.Fatalf("no transfer entry for %s", entity)
		}
		if ev.EntityID != id || ev.ActorID == nil || *ev.ActorID != f.actor {
			t.Fatalf("%s entry: id=%s actor=%v", entity, ev.EntityID, ev.ActorID)
		}
		if ev.Before["owner_id"] != f.alice.String() || ev.After["owner_id"] != f.bob.String() {
			t.Fatalf("%s entry before/after: %v → %v", entity, ev.Before, ev.After)
		}
	}
	all, err := f.chain.ListAll(t.Context(), f.tenantA, domain.AuditEventFilter{})
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("audit entries = %d, want exactly 3 (one per request, no duplicate from the row layer)", len(all))
	}
	for _, ev := range all {
		if strings.Contains(ev.Summary, "records affected") {
			t.Fatalf("one transfer touches one row, but the entry says otherwise: %q", ev.Summary)
		}
	}

	// The new owner is told, once per entity.
	if len(f.notifier.sent) != 3 {
		t.Fatalf("notifications = %d, want 3", len(f.notifier.sent))
	}
	for _, n := range f.notifier.sent {
		if n.userID != f.bob || n.tenantID != f.tenantA {
			t.Fatalf("notification went to %v in %v", n.userID, n.tenantID)
		}
	}
}

func TestOwnershipTransferRoutes_CrossTenant(t *testing.T) {
	f := newTransferFixture(t)

	// Tenant A's session names tenant B's risk.
	status, _ := f.post(t, "/risks/"+f.riskB+"/transfer-owner", f.bob.String())
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for another tenant's risk", status)
	}
	if got := f.ownerOf(t, "risks", f.riskB); got != f.stranger.String() {
		t.Fatalf("tenant B's risk was modified: owner = %s", got)
	}

	// And a user of tenant B cannot be made owner of tenant A's risk.
	status, _ = f.post(t, "/risks/"+f.riskA+"/transfer-owner", f.stranger.String())
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an owner outside the tenant", status)
	}
	if got := f.ownerOf(t, "risks", f.riskA); got != f.alice.String() {
		t.Fatalf("risk A was modified: owner = %s", got)
	}
	if n := len(f.transferEvents(t)); n != 0 {
		t.Fatalf("rejected transfers were journalled as transfers: %d", n)
	}
}

func TestOwnershipTransferRoutes_NotFound(t *testing.T) {
	f := newTransferFixture(t)

	for _, path := range []string{
		"/risks/" + uuid.New().String() + "/transfer-owner",
		"/mitigations/" + uuid.New().String() + "/transfer-owner",
		"/incidents/999999/transfer-owner",
	} {
		if status, _ := f.post(t, path, f.bob.String()); status != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404", path, status)
		}
	}
}

func TestOwnershipTransferRoutes_SoftDeletedIsNotFound(t *testing.T) {
	f := newTransferFixture(t)
	if err := f.db.Exec(`UPDATE mitigations SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?`, f.mitigationA).Error; err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if status, _ := f.post(t, "/mitigations/"+f.mitigationA+"/transfer-owner", f.bob.String()); status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for a deleted mitigation", status)
	}
}

func TestOwnershipTransferRoutes_Validation(t *testing.T) {
	f := newTransferFixture(t)

	for name, tc := range map[string]struct{ path, owner string }{
		"malformed risk id":     {"/risks/not-a-uuid/transfer-owner", f.bob.String()},
		"malformed incident id": {"/incidents/abc/transfer-owner", f.bob.String()},
		"malformed new owner":   {"/risks/" + f.riskA + "/transfer-owner", "bob"},
		"missing new owner":     {"/risks/" + f.riskA + "/transfer-owner", ""},
	} {
		t.Run(name, func(t *testing.T) {
			if status, _ := f.post(t, tc.path, tc.owner); status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", status)
			}
		})
	}
	if status, _ := f.post(t, "/risks/"+f.riskA+"/transfer-owner", f.alice.String()); status != http.StatusConflict {
		t.Fatalf("transfer to the current owner: status = %d, want 409", status)
	}
	if got := f.ownerOf(t, "risks", f.riskA); got != f.alice.String() {
		t.Fatalf("risk was modified by an invalid request: owner = %s", got)
	}
}

// Outside an HTTP request there is no collector: anything the GORM audit plugin
// observes is appended to the trail directly. The transfer write must not be
// observed at all, since the use case journals it explicitly: a row-level entry
// here would be a second record of the same change, and with an empty model it
// would name entity 00000000-0000-0000-0000-000000000000.
func TestOwnerStore_WriteIsNotJournalledByTheRowLayer(t *testing.T) {
	f := newTransferFixture(t)
	ctx := audittrail.WithActor(context.Background(), audittrail.Actor{ID: &f.actor, TenantID: f.tenantA})

	for name, store := range map[string]*repository.GormOwnerStore{
		f.mitigationA: repository.NewGormMitigationOwnerStore(f.db),
		f.riskA:       repository.NewGormRiskOwnerStore(f.db),
		f.incidentA:   repository.NewGormIncidentOwnerStore(f.db),
	} {
		if err := store.SetOwner(ctx, f.tenantA, name, f.bob); err != nil {
			t.Fatalf("SetOwner %s: %v", name, err)
		}
	}
	all, err := f.chain.ListAll(t.Context(), f.tenantA, domain.AuditEventFilter{})
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("the row layer journalled %d entries, first: %s %s %q", len(all), all[0].EntityType, all[0].EntityID, all[0].Summary)
	}
	if got := f.ownerOf(t, "mitigations", f.mitigationA); got != f.bob.String() {
		t.Fatalf("mitigation owner = %s, want %s", got, f.bob)
	}
}

// SetOwner writes without a model, so GORM's soft-delete scope does not apply;
// the store must filter deleted_at itself.
func TestOwnerStore_SetOwnerSkipsSoftDeletedRows(t *testing.T) {
	f := newTransferFixture(t)
	if err := f.db.Exec(`UPDATE mitigations SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?`, f.mitigationA).Error; err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	err := repository.NewGormMitigationOwnerStore(f.db).SetOwner(context.Background(), f.tenantA, f.mitigationA, f.bob)
	if domain.HTTPStatusFromError(err) != http.StatusNotFound {
		t.Fatalf("want not found for a deleted mitigation, got %v", err)
	}
	if got := f.ownerOf(t, "mitigations", f.mitigationA); got != f.alice.String() {
		t.Fatalf("deleted mitigation was modified: owner = %s", got)
	}
}
