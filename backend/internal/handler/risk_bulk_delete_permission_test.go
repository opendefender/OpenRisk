// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
)

// #1423 (P0) — POST /risks/bulk is behind risks:update and takes its action from
// the body. A bulk delete must also require risks:delete, or every role holding
// update without delete (RSSI, DSI, risk owner, security analyst) can delete
// risks.

// bulkDeleteRepo records the two bulk writes; any other repository call panics,
// which fails the test.
type bulkDeleteRepo struct {
	domain.RiskRepository
	deletes int
	applies int
}

func (r *bulkDeleteRepo) BulkDelete(_ context.Context, ids []uuid.UUID, _ uuid.UUID) (int64, error) {
	r.deletes++
	return int64(len(ids)), nil
}

func (r *bulkDeleteRepo) BulkApply(_ context.Context, _ uuid.UUID, ids []uuid.UUID, _ func(*domain.Risk) error) ([]domain.RiskMutation, error) {
	r.applies++
	out := make([]domain.RiskMutation, 0, len(ids))
	for _, id := range ids {
		out = append(out, domain.RiskMutation{RiskID: id})
	}
	return out, nil
}

// postLegacyBulk mounts the route as main.go does — behind risks:update — for a
// caller holding perms, and posts body.
func postLegacyBulk(t *testing.T, repo *bulkDeleteRepo, body string, perms ...string) int {
	t.Helper()
	tenant, user := uuid.New(), uuid.New()
	h := (&RiskHandler{}).WithBulkAction(risk.NewBulkActionUseCase(repo, nil))

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Post("/risks/bulk", func(c *fiber.Ctx) error {
		c.Locals("permissions", perms)
		middleware.SetContext(c, &middleware.RequestContext{UserID: user, OrganizationID: tenant})
		return c.Next()
	}, middleware.RequirePermission("risks:update"), h.BulkAction)

	req := httptest.NewRequest("POST", "/risks/bulk", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	return resp.StatusCode
}

func deleteBody() string {
	return `{"type":"delete","risk_ids":["` + uuid.New().String() + `"]}`
}

// Criterion 1 — the defect: update without delete is refused, nothing deleted.
func TestRiskBulkDelete_Unauthorized_UpdateWithoutDeleteIsForbidden(t *testing.T) {
	repo := &bulkDeleteRepo{}
	status := postLegacyBulk(t, repo, deleteBody(), "risks:read", "risks:create", "risks:update")
	assert.Equal(t, fiber.StatusForbidden, status)
	assert.Zero(t, repo.deletes, "no risk may be deleted")
}

// Criterion 2 — a caller who holds the right, in any of its three spellings,
// still deletes.
func TestRiskBulkDelete_Success_DeleteHolderDeletes(t *testing.T) {
	for _, perms := range [][]string{
		{"risks:update", "risks:delete"},
		{"risks:*"},
		{"*"},
	} {
		repo := &bulkDeleteRepo{}
		status := postLegacyBulk(t, repo, deleteBody(), perms...)
		assert.Equal(t, fiber.StatusOK, status, "%v", perms)
		assert.Equal(t, 1, repo.deletes, "%v", perms)
	}
}

// Criterion 3 — the other actions are unchanged for an update holder.
func TestRiskBulkDelete_OtherActionsStillNeedOnlyUpdate(t *testing.T) {
	repo := &bulkDeleteRepo{}
	status := postLegacyBulk(t, repo,
		`{"type":"add_tags","risk_ids":["`+uuid.New().String()+`"],"tags":["pci"]}`, "risks:update")
	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, 1, repo.applies)
	assert.Zero(t, repo.deletes)
}

// The route guard still applies first: no update right, no bulk at all.
func TestRiskBulkDelete_NotFound_NoUpdateRightNeverReachesTheHandler(t *testing.T) {
	repo := &bulkDeleteRepo{}
	status := postLegacyBulk(t, repo, deleteBody(), "risks:read", "risks:delete")
	assert.Equal(t, fiber.StatusForbidden, status)
	assert.Zero(t, repo.deletes)
}
