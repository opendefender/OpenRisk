// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	apprealtime "github.com/opendefender/openrisk/internal/application/realtime"
	applicationrisk "github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/pkg/crq"
	rt "github.com/opendefender/openrisk/pkg/realtime"
)

// ---------------------------------------------------------------------------
// #348 — a real POST /risks must reach an SSE subscriber as risk.created.
//
// The live run found the event never fired: the create route has no id in its
// path and Risk is deliberately not audited row by row, so nothing named the new
// risk. The unit tests guard the pieces (stub route, capturing publisher). This
// drives the whole chain as cmd/server/main.go wires it:
//
//	POST /risks -> permission gate -> AuditMutations -> RiskHandler.CreateRisk
//	  -> MutationBridge -> Publisher -> durable log + Hub -> GET /realtime/events
//
// Stand-ins: the session is resolved from test headers (as in every test of the
// stream) and the audit trail is an in-memory appender, because the chained
// Postgres repository is not what is under test. Everything else is the real
// production object.
// ---------------------------------------------------------------------------

// memAuditAppender records appended entries; satisfies middleware.AuditAppender.
type memAuditAppender struct {
	mu     sync.Mutex
	events []*domain.AuditEvent
}

func (m *memAuditAppender) Append(_ context.Context, e *domain.AuditEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return nil
}

func (m *memAuditAppender) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

type riskCreatedEnv struct {
	*streamTestEnv
	audit *memAuditAppender
}

func newRiskCreatedEnv(t *testing.T) *riskCreatedEnv {
	t.Helper()

	riskDB, err := gorm.Open(
		sqlite.Open("file:risk_created_"+uuid.NewString()+"?mode=memory&cache=private"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	createRisksTable(t, riskDB)
	require.NoError(t, riskDB.Exec(`CREATE TABLE risk_assets (risk_id TEXT, asset_id TEXT);`).Error)
	require.NoError(t, riskDB.Exec(`CREATE TABLE risk_histories (
		id TEXT PRIMARY KEY, risk_id TEXT, score REAL, impact INTEGER, probability INTEGER,
		status TEXT, changed_by TEXT, change_type TEXT, created_at DATETIME
	);`).Error)

	// CreateRisk re-reads the saved risk through the package-level handle, as
	// the lifecycle harness accounts for. Global state: no t.Parallel here.
	prev := database.DB
	database.DB = riskDB
	t.Cleanup(func() { database.DB = prev })

	riskRepo := repository.NewGormRiskRepository(riskDB)
	riskHandler := NewRiskHandler(
		applicationrisk.NewCreateRiskUseCase(riskRepo),
		applicationrisk.NewGetRiskUseCase(riskRepo),
		applicationrisk.NewListRisksUseCase(riskRepo),
		applicationrisk.NewUpdateRiskUseCase(riskRepo),
		applicationrisk.NewDeleteRiskUseCase(riskRepo),
		applicationrisk.NewMarkRiskReviewedUseCase(riskRepo),
		applicationrisk.NewTransitionRiskStateUseCase(riskRepo),
		nil,
		crq.NewQuantifier(0, crq.Reference{}),
	)

	audit := &memAuditAppender{}
	env := newStreamTestEnvWith(t, apprealtime.HubOptions{}, streamServerOptions{
		Mount: func(app *fiber.App, pub *apprealtime.Publisher) {
			// Stand-in for the Protected group: no resolved session, no route.
			protected := app.Group("/api/v1", func(c *fiber.Ctx) error {
				if mw := middleware.GetContext(c); mw == nil || mw.OrganizationID == uuid.Nil {
					return c.SendStatus(fiber.StatusUnauthorized)
				}
				perms := []string{"*"}
				if raw := c.Get("X-Test-Perms"); raw != "" {
					perms = strings.Split(raw, ",")
				}
				c.Locals("permissions", perms)
				return c.Next()
			})
			// The order main.go uses: audit (with the bridge) on the group, then
			// the route's own permission gate and handler.
			protected.Use(middleware.AuditMutations(audit, apprealtime.NewMutationBridge(pub)))
			protected.Post("/risks", middleware.RequirePermission("risks:create"), riskHandler.CreateRisk)
		},
	})
	return &riskCreatedEnv{streamTestEnv: env, audit: audit}
}

// postRisk issues a real POST /risks and returns the status and decoded body.
func (e *riskCreatedEnv) postRisk(t *testing.T, headers map[string]string) (int, map[string]any) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"title": "Ransomware on file server", "impact": 8.0, "probability": 0.5})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, e.baseURL+"/api/v1/risks", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

// collect gathers the frames of one event type that arrive within the window,
// ignoring every other type (a create may also yield risk.score_changed).
func (c *sseClient) collect(typ rt.EventType, within time.Duration) []sseFrame {
	var got []sseFrame
	deadline := time.After(within)
	for {
		select {
		case f, ok := <-c.frames:
			if !ok {
				return got
			}
			if f.Event == string(typ) {
				got = append(got, f)
			}
		case <-deadline:
			return got
		}
	}
}

// riskCreatedRows reads the durable log the way the stream's replay does, for
// one tenant, and keeps the risk.created rows.
func riskCreatedRows(t *testing.T, e *streamTestEnv, tenant uuid.UUID) []domain.RealtimeEvent {
	t.Helper()
	all, err := e.log.Replay(context.Background(), tenant, 0, 100)
	require.NoError(t, err)
	var out []domain.RealtimeEvent
	for _, env := range all {
		if env.Type == string(rt.RiskCreated) {
			out = append(out, env)
		}
	}
	return out
}

func TestRiskCreatedRealtime_Success(t *testing.T) {
	e := newRiskCreatedEnv(t)
	tenantA := uuid.New()

	stream, status := e.openStream(t, tenantHeaders(tenantA), "")
	require.Equal(t, http.StatusOK, status)
	stream.awaitHello(t)

	// An earlier event the subscriber has already seen: its sequence is the
	// cursor the subscriber holds when the create happens. (Cursor 0 means "no
	// cursor", a first connection, which replays nothing by design.)
	seed := e.publish(t, tenantA, rt.RiskUpdated, "earlier-risk")
	seedFrame := stream.next(t, 3*time.Second)
	require.Equal(t, seed.ID, seedFrame.Data["id"])

	status, body := e.postRisk(t, tenantHeaders(tenantA))
	require.Equal(t, http.StatusCreated, status, "the create must really succeed: %v", body)
	riskID, _ := body["id"].(string)
	require.NotEmpty(t, riskID, "the POST response must carry the new risk id")
	require.Equal(t, 1, e.audit.count(), "exactly one audit entry for the create")

	// Criterion 1: exactly one risk.created frame.
	frames := stream.collect(rt.RiskCreated, 700*time.Millisecond)
	require.Len(t, frames, 1, "exactly one risk.created frame, other types ignored")

	// Criterion 2: the frame names the new risk, the tenant and a sequence.
	frame := frames[0]
	agg, ok := frame.Data["aggregate"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, rt.AggregateRisk, agg["type"])
	assert.Equal(t, riskID, agg["id"], "aggregate.id must be the id the POST returned")
	assert.Equal(t, tenantA.String(), frame.Data["tenantId"])
	seq, ok := frame.Data["sequence"].(float64)
	require.True(t, ok)
	assert.Greater(t, seq, float64(0))

	// Criterion 3: exactly one durable row, same aggregate id and sequence.
	rows := riskCreatedRows(t, e.streamTestEnv, tenantA)
	require.Len(t, rows, 1)
	assert.Equal(t, riskID, rows[0].AggregateID)
	assert.Equal(t, tenantA, rows[0].TenantID)
	assert.EqualValues(t, seq, rows[0].Sequence)

	// Criterion 5: a subscriber reconnecting with the cursor it last saw before
	// the create replays the same event once, not twice.
	cursor := seedFrame.ID
	reconnected, status := e.openStream(t, map[string]string{
		"X-Test-Tenant": tenantA.String(),
		"Last-Event-ID": cursor,
	}, "")
	require.Equal(t, http.StatusOK, status)
	replayed := reconnected.collect(rt.RiskCreated, 700*time.Millisecond)
	require.Len(t, replayed, 1, "replay must deliver risk.created once")
	assert.Equal(t, frame.ID, replayed[0].ID)
	assert.Equal(t, frame.Data["id"], replayed[0].Data["id"])

	// And a reconnect from the cursor of the frame itself replays nothing again.
	after, status := e.openStream(t, map[string]string{
		"X-Test-Tenant": tenantA.String(),
		"Last-Event-ID": frame.ID,
	}, "")
	require.Equal(t, http.StatusOK, status)
	assert.Empty(t, after.collect(rt.RiskCreated, 500*time.Millisecond))
}

func TestRiskCreatedRealtime_CrossTenant(t *testing.T) {
	e := newRiskCreatedEnv(t)
	tenantA, tenantB := uuid.New(), uuid.New()

	streamA, status := e.openStream(t, tenantHeaders(tenantA), "")
	require.Equal(t, http.StatusOK, status)
	streamA.awaitHello(t)
	streamB, status := e.openStream(t, tenantHeaders(tenantB), "")
	require.Equal(t, http.StatusOK, status)
	streamB.awaitHello(t)

	status, _ = e.postRisk(t, tenantHeaders(tenantA))
	require.Equal(t, http.StatusCreated, status)

	require.Len(t, streamA.collect(rt.RiskCreated, 700*time.Millisecond), 1)

	// Criterion 4: B sees no risk.created, and B's log read holds none.
	assert.Empty(t, streamB.collect(rt.RiskCreated, 500*time.Millisecond))
	assert.Empty(t, riskCreatedRows(t, e.streamTestEnv, tenantB))
	assert.Len(t, riskCreatedRows(t, e.streamTestEnv, tenantA), 1)

	// Replay for B is checked with a cursor that really replays: B has two
	// events of its own (the cursor of 0 would mean "no cursor" and replay
	// nothing, proving nothing). Reconnecting after the first must hand B its
	// own second event, which shows B's replay path is live and tenant-scoped,
	// and must not hand it A's risk.created.
	b1 := e.publish(t, tenantB, rt.RiskUpdated, "b-risk")
	b1Frame := streamB.next(t, 3*time.Second)
	require.Equal(t, b1.ID, b1Frame.Data["id"])
	b2 := e.publish(t, tenantB, rt.RiskUpdated, "b-risk")
	require.Equal(t, b2.ID, streamB.next(t, 3*time.Second).Data["id"])

	replayB, status := e.openStream(t, map[string]string{
		"X-Test-Tenant": tenantB.String(),
		"Last-Event-ID": b1Frame.ID,
	}, "")
	require.Equal(t, http.StatusOK, status)
	replayB.awaitHello(t)
	got := replayB.next(t, 3*time.Second)
	assert.Equal(t, b2.ID, got.Data["id"], "B's own missed event is replayed")
	assert.Equal(t, string(rt.RiskUpdated), got.Event)
	assert.Empty(t, replayB.collect(rt.RiskCreated, 500*time.Millisecond))
}

func TestRiskCreatedRealtime_Forbidden(t *testing.T) {
	e := newRiskCreatedEnv(t)
	tenantA := uuid.New()

	stream, status := e.openStream(t, tenantHeaders(tenantA), "")
	require.Equal(t, http.StatusOK, status)
	stream.awaitHello(t)

	// Authenticated, in tenant A, but holding only risks:read.
	status, _ = e.postRisk(t, tenantHeaders(tenantA, "risks:read"))
	assert.Equal(t, http.StatusForbidden, status)
	assert.Zero(t, e.audit.count(), "a forbidden create must leave no audit entry")
	assert.Empty(t, stream.collect(rt.RiskCreated, 500*time.Millisecond))
	assert.Empty(t, riskCreatedRows(t, e.streamTestEnv, tenantA))
}

func TestRiskCreatedRealtime_Unauthorized(t *testing.T) {
	e := newRiskCreatedEnv(t)
	tenantA := uuid.New()

	// A legitimate subscriber, to prove nothing leaks to it either.
	stream, status := e.openStream(t, tenantHeaders(tenantA), "")
	require.Equal(t, http.StatusOK, status)
	stream.awaitHello(t)

	// An unauthenticated create is refused and produces no event.
	status, _ = e.postRisk(t, nil)
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Zero(t, e.audit.count(), "a refused create must leave no audit entry")
	assert.Empty(t, stream.collect(rt.RiskCreated, 500*time.Millisecond))
	assert.Empty(t, riskCreatedRows(t, e.streamTestEnv, tenantA))

	// An unauthenticated stream is refused outright: no frame at all.
	anon, status := e.openStream(t, nil, "")
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Nil(t, anon)
}
