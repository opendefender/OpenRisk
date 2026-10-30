// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	scanapp "github.com/opendefender/openrisk/internal/application/scanner"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
	scanpkg "github.com/opendefender/openrisk/internal/scanner"
)

// triggerKV is the Redis surface the trigger use case needs: the config lock
// and the agent dispatch.
type triggerKV struct {
	mu   sync.Mutex
	keys map[string]string
	last *scanpkg.AgentJobDispatch
}

func (k *triggerKV) Set(_ context.Context, key, v string, _ time.Duration) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.keys[key] = v
	return nil
}
func (k *triggerKV) Get(_ context.Context, key string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.keys[key], nil
}
func (k *triggerKV) Del(_ context.Context, keys ...string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, key := range keys {
		delete(k.keys, key)
	}
	return nil
}
func (k *triggerKV) SetNX(_ context.Context, key, v string, _ time.Duration) (bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, ok := k.keys[key]; ok {
		return false, nil
	}
	k.keys[key] = v
	return true, nil
}
func (k *triggerKV) Publish(_ context.Context, _ string, msg interface{}) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if d, ok := msg.(scanpkg.AgentJobDispatch); ok {
		k.last = &d
	}
	return nil
}

// triggerConfigRepo serves one config, to its own tenant only.
type triggerConfigRepo struct {
	domain.ScanConfigRepository
	cfg *domain.ScanConfig
}

func (r *triggerConfigRepo) GetByID(_ context.Context, id, tenantID uuid.UUID) (*domain.ScanConfig, error) {
	if r.cfg.ID != id || r.cfg.TenantID != tenantID {
		return nil, nil
	}
	return r.cfg, nil
}

type triggerJobRepo struct {
	domain.ScanJobRepository
	created []*domain.ScanJob
}

func (r *triggerJobRepo) ListByStatus(_ context.Context, tenantID uuid.UUID, st domain.ScanJobStatus) ([]domain.ScanJob, error) {
	var out []domain.ScanJob
	for _, j := range r.created {
		if j.TenantID == tenantID && j.Status == st {
			out = append(out, *j)
		}
	}
	return out, nil
}

func (r *triggerJobRepo) CountActiveByTenant(context.Context, uuid.UUID) (int64, error) {
	return 0, nil
}
func (r *triggerJobRepo) Create(_ context.Context, j *domain.ScanJob) error {
	r.created = append(r.created, j)
	return nil
}

func newTriggerApp(t *testing.T, provider domain.ScannerProvider) (*fiber.App, *domain.ScanConfig, *triggerKV, *triggerJobRepo) {
	t.Helper()
	tenant := uuid.New()
	cfg := &domain.ScanConfig{ID: uuid.New(), TenantID: tenant, Provider: provider, Enabled: true, Targets: []string{"10.0.0.0/24"}}
	kv := &triggerKV{keys: map[string]string{}}
	jobs := &triggerJobRepo{}
	uc := scanapp.NewTriggerScanUseCase(&triggerConfigRepo{cfg: cfg}, jobs, scanpkg.NewScanLock(kv), nil, nil, nil, kv, zerolog.Nop())
	h := &ScannerHandler{trigger: uc}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		middleware.SetContext(c, &middleware.RequestContext{UserID: uuid.New(), OrganizationID: tenant})
		return c.Next()
	})
	app.Post("/scanner/configs/:id/scan", h.TriggerScan)
	return app, cfg, kv, jobs
}

func postTrigger(t *testing.T, app *fiber.App, id uuid.UUID, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("POST", "/scanner/configs/"+id.String()+"/scan", strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func TestTriggerScanHandler_NoBodyIsVulnerability(t *testing.T) {
	app, cfg, kv, _ := newTriggerApp(t, domain.ProviderNmap)
	status, out := postTrigger(t, app, cfg.ID, "")
	require.Equal(t, 202, status)
	assert.Equal(t, "vulnerability", out["scan_type"])
	require.NotNil(t, kv.last)
	assert.Equal(t, "vulnerability", kv.last.ScanType)
}

func TestTriggerScanHandler_Discovery(t *testing.T) {
	app, cfg, kv, jobs := newTriggerApp(t, domain.ProviderAgent)
	status, out := postTrigger(t, app, cfg.ID, `{"scan_type":"discovery"}`)
	require.Equal(t, 202, status)
	assert.Equal(t, "discovery", out["scan_type"])
	require.Len(t, jobs.created, 1)
	assert.Equal(t, domain.ScanDiscovery, jobs.created[0].ScanType)
	require.NotNil(t, kv.last)
	assert.Equal(t, "discovery", kv.last.ScanType)
}

func TestTriggerScanHandler_Rejected(t *testing.T) {
	cases := []struct {
		name     string
		provider domain.ScannerProvider
		body     string
	}{
		{"unknown type", domain.ProviderNmap, `{"scan_type":"compliance"}`},
		{"malformed body", domain.ProviderNmap, `{"scan_type":`},
		{"type on a cloud provider", domain.ProviderAWS, `{"scan_type":"vulnerability"}`},
		{"type on an API provider", domain.ProviderGitHub, `{"scan_type":"discovery"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, cfg, _, jobs := newTriggerApp(t, tc.provider)
			status, _ := postTrigger(t, app, cfg.ID, tc.body)
			assert.Equal(t, 400, status)
			assert.Empty(t, jobs.created)
		})
	}
}

func TestTriggerScanHandler_OtherTenantConfigNotFound(t *testing.T) {
	app, _, _, jobs := newTriggerApp(t, domain.ProviderNmap)
	status, _ := postTrigger(t, app, uuid.New(), `{"scan_type":"discovery"}`)
	assert.Equal(t, 404, status)
	assert.Empty(t, jobs.created)
}

// A job queued while its agent was reconnecting reaches it through the stream
// preamble, not the live dispatch. The live pass for #1164 caught that path
// sending no scan_type, so a discovery scan ran as a vulnerability scan.
func TestQueuedJobFrames_CarryScanType(t *testing.T) {
	app, cfg, _, jobs := newTriggerApp(t, domain.ProviderNmap)
	status, _ := postTrigger(t, app, cfg.ID, `{"scan_type":"discovery"}`)
	require.Equal(t, 202, status)

	h := &ScannerHandler{jobRepo: jobs}
	frames := h.queuedJobFrames(context.Background(), &domain.ScannerAgent{ID: uuid.New(), TenantID: cfg.TenantID})
	require.Len(t, frames, 1)
	var d scanpkg.AgentJobDispatch
	require.NoError(t, json.Unmarshal([]byte(frames[0]), &d))
	assert.Equal(t, "scan.job", d.Type)
	assert.Equal(t, "discovery", d.ScanType)
}
