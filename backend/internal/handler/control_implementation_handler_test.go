// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/controlchain"
)

// #1415 — the ten implementation and test-definition routes through the real
// HTTP stack (the stack is internal_control_handler_test.go's): the permission
// guards, the real use cases and repositories, the real chained audit store.

const (
	ciBase   = "/api/v1/compliance/internal-controls"
	implBase = "/api/v1/compliance/implementations"
	testBase = "/api/v1/compliance/tests"
)

func implBody(assets ...string) string {
	out := `{"description":"Quarterly access review on core banking","status":"in_place","in_place_since":"2026-09-30","applies_to":[`
	for i, a := range assets {
		if i > 0 {
			out += ","
		}
		out += `{"type":"Asset","id":"` + a + `"}`
	}
	return out + `]}`
}

const manualTestBody = `{"name":"Quarterly review","checks":"access list matches HR","method":"manual","procedure":"export the list, compare","cadence":"P3M","pass_criteria":"no orphan account"}`

type chainIDs struct{ control, impl, test, asset string }

// seed creates a control of tenant A, an implementation on one asset, and a test.
func (s *intCtlStack) seed(t *testing.T) chainIDs {
	t.Helper()
	a := s.asA(intCtlAllPerms)
	ids := chainIDs{control: s.createControl(t, a, "Access reviews"), asset: s.asset(t, s.tenantA)}
	status, body := s.do(t, a, http.MethodPost, ciBase+"/"+ids.control+"/implementations", implBody(ids.asset))
	require.Equal(t, http.StatusCreated, status, "%v", body)
	ids.impl, _ = body["id"].(string)
	status, body = s.do(t, a, http.MethodPost, implBase+"/"+ids.impl+"/tests", manualTestBody)
	require.Equal(t, http.StatusCreated, status, "%v", body)
	ids.test, _ = body["id"].(string)
	return ids
}

// AC1: an implementation on an asset and a test on it persist and read back with
// the server-set envelope.
func TestControlImplementationRoutes_Success(t *testing.T) {
	s := newIntCtlStack(t)
	a := s.asA(intCtlAllPerms)
	ids := s.seed(t)

	status, body := s.do(t, a, http.MethodGet, implBase+"/"+ids.impl, "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, ids.control, body["control_id"])
	assert.Equal(t, "in_place", body["status"])
	assert.Equal(t, "2026-09-30", body["in_place_since"])
	assert.Equal(t, []any{map[string]any{"type": "Asset", "id": ids.asset}}, body["applies_to"])
	assert.Equal(t, "openrisk_user", body["source"])
	assert.EqualValues(t, 0.7, body["confidence"])
	assert.NotContains(t, body, "tenant_id", "the tenant is not part of the response")

	status, body = s.do(t, a, http.MethodGet, ciBase+"/"+ids.control+"/implementations", "")
	require.Equal(t, http.StatusOK, status)
	require.Len(t, body["data"], 1)

	status, body = s.do(t, a, http.MethodGet, testBase+"/"+ids.test, "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, ids.impl, body["implementation_id"])
	assert.Equal(t, "manual", body["method"])
	assert.Equal(t, "P3M", body["cadence"])
	assert.Equal(t, "openrisk_user", body["source"])
	assert.EqualValues(t, 0.7, body["confidence"])
	assert.NotContains(t, body, "tenant_id")
	assert.NotContains(t, body, "results", "results are #1416")

	status, body = s.do(t, a, http.MethodGet, implBase+"/"+ids.impl+"/tests", "")
	require.Equal(t, http.StatusOK, status)
	require.Len(t, body["data"], 1)

	// Edit both, replace the targets of the implementation, then delete.
	asset2 := s.asset(t, s.tenantA)
	status, body = s.do(t, a, http.MethodPatch, implBase+"/"+ids.impl, `{"status":"retired","in_place_since":"","applies_to":[{"id":"`+asset2+`"}]}`)
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, "retired", body["status"])
	assert.Nil(t, body["in_place_since"])
	assert.Equal(t, []any{map[string]any{"type": "Asset", "id": asset2}}, body["applies_to"])
	status, body = s.do(t, a, http.MethodPatch, testBase+"/"+ids.test, `{"method":"automated","collector":"nessus","cadence":""}`)
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, "automated", body["method"])
	assert.Nil(t, body["cadence"])

	status, _ = s.do(t, a, http.MethodDelete, implBase+"/"+ids.impl, "")
	assert.Equal(t, http.StatusNoContent, status)
	for _, p := range []string{implBase + "/" + ids.impl, testBase + "/" + ids.test} {
		status, _ = s.do(t, a, http.MethodGet, p, "")
		assert.Equal(t, http.StatusNotFound, status, p)
	}

	// Every write is one audit entry of the right entity type, and the
	// middleware adds none (markJournaled).
	var entries []domain.AuditEvent
	require.NoError(t, s.db.Where("tenant_id = ? AND entity_type IN ?", s.tenantA, []string{"control_implementation", "control_test"}).Find(&entries).Error)
	got := map[string]int{}
	for _, e := range entries {
		assert.Equal(t, domain.AuditSourceExplicit, e.Source)
		got[e.EntityType+"/"+string(e.Action)]++
	}
	assert.Equal(t, map[string]int{
		"control_implementation/create": 1, "control_implementation/update": 1, "control_implementation/delete": 1,
		"control_test/create": 1, "control_test/update": 1, "control_test/delete": 1,
	}, got)
}

// AC2 and AC3 through the routes: no asset is a 400, another tenant's asset a 404.
func TestControlImplementationRoutes_AppliesToRules(t *testing.T) {
	s := newIntCtlStack(t)
	a := s.asA(intCtlAllPerms)
	ctl := s.createControl(t, a, "c")
	mine, foreign := s.asset(t, s.tenantA), s.asset(t, s.tenantB)
	path := ciBase + "/" + ctl + "/implementations"

	for name, body := range map[string]string{
		"no applies_to":  `{"description":"d"}`,
		"empty list":     `{"description":"d","applies_to":[]}`,
		"nil asset id":   `{"description":"d","applies_to":[{"id":"` + uuid.Nil.String() + `"}]}`,
		"system target":  `{"description":"d","applies_to":[{"type":"System","id":"` + mine + `"}]}`,
		"blank text":     `{"description":" ","applies_to":[{"id":"` + mine + `"}]}`,
		"unknown status": `{"description":"d","status":"done","applies_to":[{"id":"` + mine + `"}]}`,
	} {
		status, resp := s.do(t, a, http.MethodPost, path, body)
		assert.Equal(t, http.StatusBadRequest, status, name+" %v", resp)
	}
	status, resp := s.do(t, a, http.MethodPost, path, `{"description":"d"}`)
	require.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, domain.IntCtlAppliesToRequired, resp["error"])

	status, _ = s.do(t, a, http.MethodPost, path, implBody(foreign))
	assert.Equal(t, http.StatusNotFound, status, "another tenant's asset")
	status, _ = s.do(t, a, http.MethodPost, path, implBody(mine, foreign))
	assert.Equal(t, http.StatusNotFound, status, "one good asset and one of another tenant")
	status, _ = s.do(t, a, http.MethodPost, path, implBody(uuid.NewString()))
	assert.Equal(t, http.StatusNotFound, status, "a fabricated asset reads the same")
	assert.Zero(t, s.count(t, &controlchain.ControlImplementationRow{}))
	assert.Zero(t, s.count(t, &controlchain.CanonicalEdgeRow{}), "nothing was written, no edge in either tenant")

	// An update may not empty the set, nor reach another tenant's asset.
	status, body := s.do(t, a, http.MethodPost, path, implBody(mine))
	require.Equal(t, http.StatusCreated, status, "%v", body)
	id := body["id"].(string)
	status, _ = s.do(t, a, http.MethodPatch, implBase+"/"+id, `{"applies_to":[]}`)
	assert.Equal(t, http.StatusBadRequest, status)
	status, _ = s.do(t, a, http.MethodPatch, implBase+"/"+id, `{"applies_to":[{"id":"`+foreign+`"}]}`)
	assert.Equal(t, http.StatusNotFound, status)
	status, body = s.do(t, a, http.MethodGet, implBase+"/"+id, "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, []any{map[string]any{"type": "Asset", "id": mine}}, body["applies_to"])
}

// AC4 through the routes.
func TestControlTestRoutes_Validation(t *testing.T) {
	s := newIntCtlStack(t)
	a := s.asA(intCtlAllPerms)
	ids := s.seed(t)
	path := implBase + "/" + ids.impl + "/tests"
	for name, body := range map[string]string{
		"no checks":               `{"name":"t","method":"manual","procedure":"p","pass_criteria":"c"}`,
		"no pass criteria":        `{"name":"t","checks":"x","method":"manual","procedure":"p"}`,
		"manual, no procedure":    `{"name":"t","checks":"x","method":"manual","pass_criteria":"c"}`,
		"automated, no collector": `{"name":"t","checks":"x","method":"automated","procedure":"p","pass_criteria":"c"}`,
		"cadence not ISO 8601":    `{"name":"t","checks":"x","method":"manual","procedure":"p","cadence":"monthly","pass_criteria":"c"}`,
		"unknown method":          `{"name":"t","checks":"x","method":"magic","procedure":"p","pass_criteria":"c"}`,
		"malformed body":          `{"name":`,
	} {
		status, resp := s.do(t, a, http.MethodPost, path, body)
		assert.Equal(t, http.StatusBadRequest, status, name+" %v", resp)
	}
	// Cadence is optional; an automated test needs only its collector.
	status, body := s.do(t, a, http.MethodPost, path, `{"name":"scan","checks":"x","method":"automated","collector":"nessus","pass_criteria":"c"}`)
	require.Equal(t, http.StatusCreated, status, "%v", body)
	assert.Nil(t, body["cadence"])
	assert.EqualValues(t, 2, s.count(t, &controlchain.ControlTestRow{}))
}

func TestControlImplementationRoutes_InvalidIds(t *testing.T) {
	s := newIntCtlStack(t)
	a := s.asA(intCtlAllPerms)
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPost, ciBase + "/nope/implementations", implBody(uuid.NewString())},
		{http.MethodGet, ciBase + "/" + uuid.Nil.String() + "/implementations", ""},
		{http.MethodGet, implBase + "/nope", ""},
		{http.MethodPatch, implBase + "/nope", `{}`},
		{http.MethodDelete, implBase + "/nope", ""},
		{http.MethodGet, implBase + "/nope/tests", ""},
		{http.MethodPost, implBase + "/nope/tests", manualTestBody},
		{http.MethodGet, testBase + "/nope", ""},
		{http.MethodPatch, testBase + "/nope", `{}`},
		{http.MethodDelete, testBase + "/" + uuid.Nil.String(), ""},
		{http.MethodGet, ciBase + "/" + uuid.NewString() + "/implementations?limit=x", ""},
	} {
		status, _ := s.do(t, a, c.method, c.path, c.body)
		assert.Equal(t, http.StatusBadRequest, status, c.method+" "+c.path)
	}
}

func TestControlImplementationRoutes_Forbidden(t *testing.T) {
	s := newIntCtlStack(t)
	ids := s.seed(t)
	readOnly := s.asA("compliance:controls:read")
	for name, c := range map[string]struct{ method, path, body string }{
		"create implementation": {http.MethodPost, ciBase + "/" + ids.control + "/implementations", implBody(ids.asset)},
		"patch implementation":  {http.MethodPatch, implBase + "/" + ids.impl, `{"status":"retired"}`},
		"delete implementation": {http.MethodDelete, implBase + "/" + ids.impl, ""},
		"create test":           {http.MethodPost, implBase + "/" + ids.impl + "/tests", manualTestBody},
		"patch test":            {http.MethodPatch, testBase + "/" + ids.test, `{"name":"x"}`},
		"delete test":           {http.MethodDelete, testBase + "/" + ids.test, ""},
	} {
		status, _ := s.do(t, readOnly, c.method, c.path, c.body)
		assert.Equal(t, http.StatusForbidden, status, name)
	}
	for name, p := range map[string]string{
		"list implementations": ciBase + "/" + ids.control + "/implementations",
		"get implementation":   implBase + "/" + ids.impl,
		"list tests":           implBase + "/" + ids.impl + "/tests",
		"get test":             testBase + "/" + ids.test,
	} {
		status, _ := s.do(t, s.asA("compliance:controls:update"), http.MethodGet, p, "")
		assert.Equal(t, http.StatusForbidden, status, name+" without read")
	}
	assert.EqualValues(t, 1, s.count(t, &controlchain.ControlImplementationRow{}))
	assert.EqualValues(t, 1, s.count(t, &controlchain.ControlTestRow{}))
}

func TestControlImplementationRoutes_Unauthorized(t *testing.T) {
	s := newIntCtlStack(t)
	ids := s.seed(t)
	noTenant := intCtlReq{user: s.userA, perms: intCtlAllPerms}
	noUser := intCtlReq{tenant: s.tenantA, perms: intCtlAllPerms}
	for name, who := range map[string]intCtlReq{"no tenant": noTenant, "no user": noUser} {
		for _, c := range []struct{ method, path, body string }{
			{http.MethodGet, ciBase + "/" + ids.control + "/implementations", ""},
			{http.MethodPost, ciBase + "/" + ids.control + "/implementations", implBody(ids.asset)},
			{http.MethodGet, implBase + "/" + ids.impl, ""},
			{http.MethodPatch, implBase + "/" + ids.impl, `{"status":"retired"}`},
			{http.MethodDelete, implBase + "/" + ids.impl, ""},
			{http.MethodGet, implBase + "/" + ids.impl + "/tests", ""},
			{http.MethodPost, implBase + "/" + ids.impl + "/tests", manualTestBody},
			{http.MethodGet, testBase + "/" + ids.test, ""},
			{http.MethodPatch, testBase + "/" + ids.test, `{"name":"x"}`},
			{http.MethodDelete, testBase + "/" + ids.test, ""},
		} {
			status, _ := s.do(t, who, c.method, c.path, c.body)
			assert.Equal(t, http.StatusUnauthorized, status, name+" "+c.method+" "+c.path)
		}
	}
	assert.EqualValues(t, 1, s.count(t, &controlchain.ControlImplementationRow{}))
	assert.EqualValues(t, 1, s.count(t, &controlchain.ControlTestRow{}))
}

// Tenant B reaches nothing of tenant A's by id: a 404 with the same shape as a
// fabricated id, on all ten routes, and A's rows are untouched.
func TestControlImplementationRoutes_CrossTenantIsNotFound(t *testing.T) {
	s := newIntCtlStack(t)
	b := s.asB()
	ids := s.seed(t)
	fake := uuid.NewString()
	foreignAssetOfB := s.asset(t, s.tenantB)
	for name, c := range map[string]struct{ method, path, body string }{
		"list implementations":  {http.MethodGet, ciBase + "/" + ids.control + "/implementations", ""},
		"create implementation": {http.MethodPost, ciBase + "/" + ids.control + "/implementations", implBody(foreignAssetOfB)},
		"get implementation":    {http.MethodGet, implBase + "/" + ids.impl, ""},
		"patch implementation":  {http.MethodPatch, implBase + "/" + ids.impl, `{"description":"hijacked"}`},
		"delete implementation": {http.MethodDelete, implBase + "/" + ids.impl, ""},
		"list tests":            {http.MethodGet, implBase + "/" + ids.impl + "/tests", ""},
		"create test":           {http.MethodPost, implBase + "/" + ids.impl + "/tests", manualTestBody},
		"get test":              {http.MethodGet, testBase + "/" + ids.test, ""},
		"patch test":            {http.MethodPatch, testBase + "/" + ids.test, `{"name":"hijacked"}`},
		"delete test":           {http.MethodDelete, testBase + "/" + ids.test, ""},
	} {
		status, foreignBody := s.do(t, b, c.method, c.path, c.body)
		assert.Equal(t, http.StatusNotFound, status, name)
		// A fabricated id answers the same, so a foreign object is not confirmed to exist.
		fakePath := c.path
		for _, id := range []string{ids.control, ids.impl, ids.test} {
			fakePath = replaceOnce(fakePath, id, fake)
		}
		status, fakeBody := s.do(t, b, c.method, fakePath, c.body)
		assert.Equal(t, http.StatusNotFound, status, name+" (fabricated)")
		assert.Equal(t, foreignBody["error"] != nil, fakeBody["error"] != nil, name)
	}

	a := s.asA(intCtlAllPerms)
	status, body := s.do(t, a, http.MethodGet, implBase+"/"+ids.impl, "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "Quarterly access review on core banking", body["description"])
	status, body = s.do(t, a, http.MethodGet, testBase+"/"+ids.test, "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "Quarterly review", body["name"])
	assert.EqualValues(t, 1, s.count(t, &controlchain.ControlImplementationRow{}))
	assert.EqualValues(t, 1, s.count(t, &controlchain.ControlTestRow{}))
	assert.EqualValues(t, 1, s.count(t, &controlchain.CanonicalEdgeRow{}), "A's applies_to edge is untouched")
}

func replaceOnce(s, old, repl string) string {
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + repl + s[i+len(old):]
		}
	}
	return s
}

// A deleted control hides its implementations and tests (S3's delete does not cascade).
func TestControlImplementationRoutes_DeletedControlHidesTheChain(t *testing.T) {
	s := newIntCtlStack(t)
	a := s.asA(intCtlAllPerms)
	ids := s.seed(t)
	status, _ := s.do(t, a, http.MethodDelete, ciBase+"/"+ids.control, "")
	require.Equal(t, http.StatusNoContent, status)
	for _, p := range []string{
		ciBase + "/" + ids.control + "/implementations", implBase + "/" + ids.impl, implBase + "/" + ids.impl + "/tests", testBase + "/" + ids.test,
	} {
		status, _ = s.do(t, a, http.MethodGet, p, "")
		assert.Equal(t, http.StatusNotFound, status, p)
	}
	status, _ = s.do(t, a, http.MethodPost, implBase+"/"+ids.impl+"/tests", manualTestBody)
	assert.Equal(t, http.StatusNotFound, status)
}
