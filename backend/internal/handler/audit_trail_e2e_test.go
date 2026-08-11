// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/application/governance"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/audittrail"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
)

// ---------------------------------------------------------------------------
// The exhaustive-audit-trail proof.
//
// Twenty distinct actions of different kinds go through the real HTTP stack —
// the real mutation middleware, the real GORM audit plugin, the real chained
// repository. The trail must then contain EXACTLY twenty entries, each carrying
// the who / what / where metadata, and the hash-chain verification must pass.
//
// The value of "exactly" is the point: a trail that double-counts is a trail
// nobody can reconcile, and a trail that under-counts is one you cannot trust.
// ---------------------------------------------------------------------------

// widget stands in for any tenant-scoped, Auditable domain model. Using a local
// model keeps the test about the audit machinery rather than about any one
// module's schema.
type widget struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;index" json:"tenant_id"`
	Name     string    `json:"name"`
	Status   string    `json:"status"`
	Secret   string    `json:"-"` // never captured — RULE #6
}

func (widget) AuditEntityType() string { return "widget" }

type auditFixture struct {
	app    *fiber.App
	db     *gorm.DB
	chain  *repository.GormAuditChainRepository
	tenant uuid.UUID
	actor  uuid.UUID
}

func newAuditFixture(t *testing.T) *auditFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// Fresh schema per test — the shared in-memory cache would otherwise leak
	// rows between runs and make the "exactly 20" assertion meaningless.
	_ = db.Migrator().DropTable(&widget{}, &domain.AuditEvent{}, &domain.AuditChainSeal{}, &domain.AuditRetentionPolicy{})
	if err := db.AutoMigrate(&widget{}, &domain.AuditEvent{}, &domain.AuditChainSeal{}, &domain.AuditRetentionPolicy{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	chain := repository.NewGormAuditChainRepository(db)
	if err := db.Use(audittrail.New(db).WithAppender(chain)); err != nil {
		t.Fatalf("install audit plugin: %v", err)
	}

	f := &auditFixture{db: db, chain: chain, tenant: uuid.New(), actor: uuid.New()}

	app := fiber.New()
	api := app.Group("/api/v1")
	// Same order as the composition root: identity, then actor stamp, then the
	// mutation journal, then the business routes.
	api.Use(func(c *fiber.Ctx) error {
		middleware.SetContext(c, &middleware.RequestContext{UserID: f.actor, OrganizationID: f.tenant})
		c.SetUserContext(audittrail.WithActor(c.UserContext(), audittrail.Actor{
			ID: &f.actor, TenantID: f.tenant, IPAddress: c.IP(),
			UserAgent: c.Get("User-Agent"), RequestID: c.Get("X-Request-ID"),
		}))
		return c.Next()
	})
	api.Use(middleware.AuditMutations(chain))

	tx := func(c *fiber.Ctx) *gorm.DB { return db.WithContext(c.UserContext()) }

	api.Post("/widgets", func(c *fiber.Ctx) error {
		var body widget
		_ = c.BodyParser(&body)
		body.ID = uuid.New()
		body.TenantID = f.tenant
		if err := tx(c).Create(&body).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(201).JSON(body)
	})
	api.Put("/widgets/:id", func(c *fiber.Ctx) error {
		var w widget
		if err := tx(c).Where("id = ? AND tenant_id = ?", c.Params("id"), f.tenant).Take(&w).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		var body widget
		_ = c.BodyParser(&body)
		w.Name = body.Name
		w.Status = body.Status
		if err := tx(c).Save(&w).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(w)
	})
	api.Delete("/widgets/:id", func(c *fiber.Ctx) error {
		if err := tx(c).Where("id = ? AND tenant_id = ?", c.Params("id"), f.tenant).Delete(&widget{}).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.SendStatus(204)
	})
	// A route whose meaning lives in the application layer, not in a row diff —
	// the explicit Recorder path.
	api.Post("/widgets/:id/approve", func(c *fiber.Ctx) error {
		rec := governance.NewAuditRecorder(chain)
		actor := f.actor
		rec.Record(c.UserContext(), domain.AuditEvent{
			TenantID: f.tenant, ActorID: &actor, Action: domain.AuditActionApprove,
			EntityType: "widget", EntityID: c.Params("id"),
			Summary: "approved widget " + c.Params("id"),
		})
		return c.JSON(fiber.Map{"approved": true})
	})
	// A mutation that touches several rows in one action.
	api.Post("/widgets/bulk-archive", func(c *fiber.Ctx) error {
		var ws []widget
		if err := tx(c).Where("tenant_id = ?", f.tenant).Limit(3).Find(&ws).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		for i := range ws {
			ws[i].Status = "archived"
			if err := tx(c).Save(&ws[i]).Error; err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
		}
		return c.JSON(fiber.Map{"archived": len(ws)})
	})
	// A refused action: it must NOT reach the trail — nothing changed.
	api.Post("/widgets/forbidden", func(c *fiber.Ctx) error {
		return c.Status(403).JSON(fiber.Map{"error": "nope"})
	})
	// A read: reads are not mutations.
	api.Get("/widgets", func(c *fiber.Ctx) error {
		var ws []widget
		_ = tx(c).Where("tenant_id = ?", f.tenant).Find(&ws).Error
		return c.JSON(ws)
	})

	f.app = app
	return f
}

func (f *auditFixture) do(t *testing.T, method, path, body string) *http.Response {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "openrisk-audit-test/1.0")
	req.Header.Set("X-Request-ID", "req-"+uuid.NewString()[:8])
	resp, err := f.app.Test(req, 5000)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func (f *auditFixture) createWidget(t *testing.T, name string) string {
	t.Helper()
	resp := f.do(t, http.MethodPost, "/api/v1/widgets", fmt.Sprintf(`{"name":%q,"status":"new","secret":"hunter2"}`, name))
	if resp.StatusCode != 201 {
		t.Fatalf("create widget %s: status %d", name, resp.StatusCode)
	}
	var w widget
	b, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(b, &w)
	return w.ID.String()
}

// TestAuditTrail_TwentyDistinctActions_ProduceExactlyTwentyVerifiedEntries is
// the acceptance test for the exhaustive audit trail.
func TestAuditTrail_TwentyDistinctActions_ProduceExactlyTwentyVerifiedEntries(t *testing.T) {
	f := newAuditFixture(t)

	// --- 20 distinct actions of different types -----------------------------
	// 1-8: eight creations.
	ids := make([]string, 0, 8)
	for i := 1; i <= 8; i++ {
		ids = append(ids, f.createWidget(t, fmt.Sprintf("widget-%02d", i)))
	}
	// 9-14: six updates.
	for i := 0; i < 6; i++ {
		resp := f.do(t, http.MethodPut, "/api/v1/widgets/"+ids[i],
			fmt.Sprintf(`{"name":"widget-%02d-renamed","status":"active"}`, i+1))
		if resp.StatusCode != 200 {
			t.Fatalf("update %d: status %d", i, resp.StatusCode)
		}
	}
	// 15-17: three deletions.
	for i := 5; i < 8; i++ {
		resp := f.do(t, http.MethodDelete, "/api/v1/widgets/"+ids[i], "")
		if resp.StatusCode != 204 {
			t.Fatalf("delete %d: status %d", i, resp.StatusCode)
		}
	}
	// 18-19: two approvals (explicit, application-level meaning).
	for i := 0; i < 2; i++ {
		resp := f.do(t, http.MethodPost, "/api/v1/widgets/"+ids[i]+"/approve", "")
		if resp.StatusCode != 200 {
			t.Fatalf("approve %d: status %d", i, resp.StatusCode)
		}
	}
	// 20: one bulk action touching several rows — still ONE user action.
	if resp := f.do(t, http.MethodPost, "/api/v1/widgets/bulk-archive", ""); resp.StatusCode != 200 {
		t.Fatalf("bulk archive: status %d", resp.StatusCode)
	}

	// Noise that must NOT land in the trail.
	f.do(t, http.MethodGet, "/api/v1/widgets", "")           // a read
	f.do(t, http.MethodPost, "/api/v1/widgets/forbidden", "") // a refused action

	// --- exactly twenty entries ---------------------------------------------
	events, err := f.chain.ListAll(t.Context(), f.tenant, domain.AuditEventFilter{})
	if err != nil {
		t.Fatalf("read trail: %v", err)
	}
	if len(events) != 20 {
		var got []string
		for _, e := range events {
			got = append(got, fmt.Sprintf("#%d %s %s %s", e.Sequence, e.Method, e.Action, e.EntityType))
		}
		t.Fatalf("expected exactly 20 audit entries for 20 actions, got %d:\n%s",
			len(events), strings.Join(got, "\n"))
	}

	// --- every entry is correct ---------------------------------------------
	byAction := map[domain.AuditAction]int{}
	for i, e := range events {
		if e.TenantID != f.tenant {
			t.Fatalf("entry %d: wrong tenant", i)
		}
		if e.ActorID == nil || *e.ActorID != f.actor {
			t.Fatalf("entry %d (%s): actor not recorded", i, e.Summary)
		}
		if e.EntityType == "" || e.EntityID == "" {
			t.Fatalf("entry %d: resource not identified (type=%q id=%q)", i, e.EntityType, e.EntityID)
		}
		if e.IPAddress == "" {
			t.Fatalf("entry %d: ip address missing", i)
		}
		if e.UserAgent != "openrisk-audit-test/1.0" {
			t.Fatalf("entry %d: user agent missing, got %q", i, e.UserAgent)
		}
		if e.RequestID == "" {
			t.Fatalf("entry %d: request id missing", i)
		}
		if e.Method == "" || e.Path == "" || e.StatusCode < 200 || e.StatusCode >= 300 {
			t.Fatalf("entry %d: http envelope wrong (%s %s → %d)", i, e.Method, e.Path, e.StatusCode)
		}
		if e.CreatedAt.IsZero() {
			t.Fatalf("entry %d: timestamp missing", i)
		}
		if e.Sequence != int64(i+1) {
			t.Fatalf("entry %d: expected sequence %d, got %d", i, i+1, e.Sequence)
		}
		if e.Hash == "" {
			t.Fatalf("entry %d: not hashed", i)
		}
		byAction[e.Action]++
	}

	if byAction[domain.AuditActionCreate] != 8 {
		t.Errorf("expected 8 create entries, got %d", byAction[domain.AuditActionCreate])
	}
	if byAction[domain.AuditActionUpdate] != 6+1 { // 6 updates + the bulk archive
		t.Errorf("expected 7 update entries, got %d", byAction[domain.AuditActionUpdate])
	}
	if byAction[domain.AuditActionDelete] != 3 {
		t.Errorf("expected 3 delete entries, got %d", byAction[domain.AuditActionDelete])
	}
	if byAction[domain.AuditActionApprove] != 2 {
		t.Errorf("expected 2 approve entries, got %d", byAction[domain.AuditActionApprove])
	}

	// The before → after evidence is really captured on an update...
	var sawDiff bool
	for _, e := range events {
		if e.Action == domain.AuditActionUpdate && e.Before != nil && e.After != nil {
			if e.Before["name"] != e.After["name"] {
				sawDiff = true
			}
			// ...and secrets are not, by construction (json:"-").
			if _, leaked := e.After["Secret"]; leaked {
				t.Fatal("a json:\"-\" field leaked into the audit snapshot")
			}
		}
	}
	if !sawDiff {
		t.Error("no update entry captured a before → after difference")
	}

	// --- the chain verifies --------------------------------------------------
	seals, err := f.chain.ListSeals(t.Context(), f.tenant)
	if err != nil {
		t.Fatalf("read seals: %v", err)
	}
	report := domain.VerifyAuditChain(f.tenant, events, seals)
	if !report.Valid {
		t.Fatalf("hash chain verification failed: %+v", report.Breaks)
	}
	if report.Verified != 20 {
		t.Fatalf("expected 20 verified entries, got %d", report.Verified)
	}
}

// TestAuditTrail_TamperingIsDetectedThroughTheRepository proves the chain does
// real work: an attacker with database access edits a stored entry, and the
// verification run against the live store catches it.
func TestAuditTrail_TamperingIsDetectedThroughTheRepository(t *testing.T) {
	f := newAuditFixture(t)
	for i := 0; i < 5; i++ {
		f.createWidget(t, fmt.Sprintf("w-%d", i))
	}

	verify := governance.NewVerifyAuditChainUseCase(f.chain)
	before, err := verify.Execute(t.Context(), f.tenant)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !before.Valid {
		t.Fatalf("baseline chain should be valid: %+v", before.Breaks)
	}

	// Straight to the table, bypassing every application guard.
	if err := f.db.Exec(
		`UPDATE audit_events SET summary = ? WHERE tenant_id = ? AND sequence = 3`,
		"create widget (nothing happened here)", f.tenant,
	).Error; err != nil {
		t.Fatalf("tamper: %v", err)
	}

	after, err := verify.Execute(t.Context(), f.tenant)
	if err != nil {
		t.Fatalf("verify after tamper: %v", err)
	}
	if after.Valid {
		t.Fatal("editing a stored entry must be detected by chain verification")
	}
	if len(after.Breaks) == 0 || after.Breaks[0].Sequence != 3 {
		t.Fatalf("verification should point at sequence 3, got %+v", after.Breaks)
	}

	// Deleting an entry outright is detected too.
	if err := f.db.Exec(`DELETE FROM audit_events WHERE tenant_id = ? AND sequence = 4`, f.tenant).Error; err != nil {
		t.Fatalf("delete: %v", err)
	}
	afterDelete, _ := verify.Execute(t.Context(), f.tenant)
	kinds := map[string]bool{}
	for _, b := range afterDelete.Breaks {
		kinds[b.Kind] = true
	}
	if !kinds[domain.BreakSequenceGap] {
		t.Fatalf("deleting an entry must raise a sequence_gap, got %+v", afterDelete.Breaks)
	}
}

// TestAuditTrail_RetentionPruneKeepsTheChainVerifiable proves retention is not a
// hole: pruned ranges are sealed and the surviving chain still verifies.
func TestAuditTrail_RetentionPruneKeepsTheChainVerifiable(t *testing.T) {
	f := newAuditFixture(t)
	for i := 0; i < 10; i++ {
		f.createWidget(t, fmt.Sprintf("w-%d", i))
	}
	// Age the first six entries past the retention window.
	old := time.Now().UTC().AddDate(0, 0, -120)
	if err := f.db.Exec(`UPDATE audit_events SET created_at = ? WHERE tenant_id = ? AND sequence <= 6`, old, f.tenant).Error; err != nil {
		t.Fatalf("age entries: %v", err)
	}

	retentionRepo := repository.NewGormAuditRetentionRepository(f.db)
	if _, err := governance.NewSetRetentionPolicyUseCase(retentionRepo).
		Execute(t.Context(), f.tenant, f.actor, 90); err != nil {
		t.Fatalf("set retention: %v", err)
	}

	res, err := governance.NewPruneAuditTrailUseCase(f.chain, retentionRepo).
		ExecuteForTenant(t.Context(), f.tenant, time.Now().UTC())
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	// Six aged entries, minus the retention-policy change the Set use case just
	// recorded — assert on the seal rather than a magic number.
	if res.Seal == nil || res.Pruned == 0 {
		t.Fatalf("expected a prune to happen and be sealed, got %+v", res)
	}

	report, err := governance.NewVerifyAuditChainUseCase(f.chain).Execute(t.Context(), f.tenant)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !report.Valid {
		t.Fatalf("a sealed retention prune must leave the chain verifiable: %+v", report.Breaks)
	}
	if report.Seals != 1 {
		t.Fatalf("expected exactly one seal, got %d", report.Seals)
	}

	// New entries keep chaining onto the surviving head.
	f.createWidget(t, "after-prune")
	report2, _ := governance.NewVerifyAuditChainUseCase(f.chain).Execute(t.Context(), f.tenant)
	if !report2.Valid {
		t.Fatalf("appending after a prune must stay verifiable: %+v", report2.Breaks)
	}
}

// TestAuditTrail_ExportCarriesTheVerdict proves an export cannot look clean when
// the trail is not: the verdict travels inside the file.
func TestAuditTrail_ExportCarriesTheVerdict(t *testing.T) {
	f := newAuditFixture(t)
	for i := 0; i < 4; i++ {
		f.createWidget(t, fmt.Sprintf("w-%d", i))
	}
	export := governance.NewExportAuditTrailUseCase(f.chain)

	clean, err := export.Execute(t.Context(), f.tenant, "tester", domain.AuditEventFilter{})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if clean.Count != 4 || !clean.Verification.Valid {
		t.Fatalf("expected a clean export of 4 entries, got %d valid=%v", clean.Count, clean.Verification.Valid)
	}
	if clean.Signature == nil {
		t.Fatal("an export must always carry a signature block")
	}
	// No key configured in tests: the block must SAY it is unsigned rather than
	// ship an empty value that reads as signed.
	if clean.Signature.Value == "" && clean.Signature.Reason == "" {
		t.Fatal("an unsigned export must explain why it is unsigned")
	}

	_ = f.db.Exec(`UPDATE audit_events SET entity_id = 'forged' WHERE tenant_id = ? AND sequence = 2`, f.tenant).Error
	tampered, err := export.Execute(t.Context(), f.tenant, "tester", domain.AuditEventFilter{})
	if err != nil {
		t.Fatalf("export after tamper: %v", err)
	}
	if tampered.Verification.Valid {
		t.Fatal("an export of a tampered trail must say so on its face")
	}
}

// TestAuditTrail_SignedExportRoundTrip proves the signature is real when a key
// is configured, and that editing the exported file invalidates it.
func TestAuditTrail_SignedExportRoundTrip(t *testing.T) {
	t.Setenv("AUDIT_EXPORT_KEY", "test-export-key-at-least-16-chars")
	f := newAuditFixture(t)
	f.createWidget(t, "signed")

	exp, err := governance.NewExportAuditTrailUseCase(f.chain).
		Execute(t.Context(), f.tenant, "tester", domain.AuditEventFilter{})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if exp.Signature.Value == "" {
		t.Fatal("a configured key must produce a signature")
	}
	if !governance.VerifyExportSignature(exp) {
		t.Fatal("a freshly signed export must verify")
	}

	exp.Events[0].Summary = "edited after export"
	if governance.VerifyExportSignature(exp) {
		t.Fatal("editing an exported entry must invalidate the export signature")
	}
}
