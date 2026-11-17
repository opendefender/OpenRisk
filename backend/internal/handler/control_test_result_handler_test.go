// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler_test

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/controlchain"
	"github.com/opendefender/openrisk/internal/security/routes"
)

// #1416 — the five result routes through the real HTTP stack (the stack is
// internal_control_handler_test.go's): the permission guards, the real use cases
// and repository, the real chained audit store and mutation middleware.

const resultBody = `{"outcome":"pass","observed_at":"2026-11-16T10:00:00Z","note":"quarterly run"}`

func (s *intCtlStack) record(t *testing.T, who intCtlReq, testID, body string) map[string]any {
	t.Helper()
	status, out := s.do(t, who, http.MethodPost, testBase+"/"+testID+"/results", body)
	require.Equal(t, http.StatusCreated, status, "%v", out)
	return out
}

// AC1 and AC2: a result and an evidence on it persist and read back with the
// server-set envelope; the same artifact backs two results of one test.
func TestControlTestResultRoutes_Success(t *testing.T) {
	s := newIntCtlStack(t)
	a := s.asA(intCtlResultPerms)
	ids := s.seed(t)

	r1 := s.record(t, a, ids.test, resultBody)
	assert.Equal(t, ids.test, r1["test_id"])
	assert.Equal(t, "pass", r1["outcome"])
	assert.Equal(t, "2026-11-16T10:00:00Z", r1["observed_at"])
	assert.Equal(t, "quarterly run", r1["note"])
	assert.Equal(t, "openrisk_user", r1["source"])
	assert.EqualValues(t, 0.7, r1["confidence"])
	assert.Equal(t, map[string]any{"kind": "user", "id": s.userA.String()}, r1["executed_by"])
	r2 := s.record(t, a, ids.test, `{"outcome":"fail","observed_at":"2026-11-15T10:00:00+01:00"}`)
	assert.Equal(t, "2026-11-15T09:00:00Z", r2["observed_at"], "normalised to UTC")

	status, body := s.do(t, a, http.MethodGet, testBase+"/"+ids.test+"/results", "")
	require.Equal(t, http.StatusOK, status)
	data := body["data"].([]any)
	require.Len(t, data, 2)
	assert.Equal(t, r1["id"], data[0].(map[string]any)["id"], "newest observed first")
	status, body = s.do(t, a, http.MethodGet, testBase+"/"+ids.test+"/results/latest", "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, r1["id"], body["id"])

	ev := s.evidence(t, s.tenantA)
	path1 := testBase + "/" + ids.test + "/results/" + r1["id"].(string) + "/evidence"
	path2 := testBase + "/" + ids.test + "/results/" + r2["id"].(string) + "/evidence"
	status, link := s.do(t, a, http.MethodPost, path1, `{"evidence_id":"`+ev+`"}`)
	require.Equal(t, http.StatusCreated, status, "%v", link)
	assert.Equal(t, r1["id"], link["result_id"])
	assert.Equal(t, ev, link["evidence_id"])
	var edge controlchain.CanonicalEdgeRow
	require.NoError(t, s.db.Where("tenant_id = ? AND verb = ?", s.tenantA, "evidenced_by").Take(&edge).Error)
	assert.Equal(t, r1["id"], edge.Qualifier.String(), "AC2: the edge carries the result_id qualifier")

	status, again := s.do(t, a, http.MethodPost, path1, `{"evidence_id":"`+ev+`"}`)
	assert.Equal(t, http.StatusOK, status, "the pair exists: 200 with the stored edge")
	assert.Equal(t, link["id"], again["id"])
	assert.EqualValues(t, 2, s.count(t, &controlchain.CanonicalEdgeRow{}), "the applies_to edge and one evidenced_by edge: the repeat wrote no row")
	var audits int64
	require.NoError(t, s.db.Model(&domain.AuditEvent{}).Where("entity_type = ? AND entity_id = ?", "control_test_result", r1["id"]).Count(&audits).Error)
	assert.EqualValues(t, 2, audits, "the record and the first attach, journaled once each")

	status, link2 := s.do(t, a, http.MethodPost, path2, `{"evidence_id":"`+ev+`"}`)
	require.Equal(t, http.StatusCreated, status, "%v", link2)
	assert.NotEqual(t, link["id"], link2["id"], "the same artifact backs a second result of the same test")

	status, body = s.do(t, a, http.MethodGet, path1, "")
	require.Equal(t, http.StatusOK, status)
	require.Len(t, body["data"], 1)
	assert.Equal(t, ev, body["data"].([]any)[0].(map[string]any)["evidence_id"])
}

func TestControlTestResultRoutes_Validation(t *testing.T) {
	s := newIntCtlStack(t)
	a := s.asA(intCtlResultPerms)
	ids := s.seed(t)
	path := testBase + "/" + ids.test + "/results"
	for name, body := range map[string]string{
		"no observed_at":     `{"outcome":"pass"}`,
		"no outcome":         `{"observed_at":"2026-11-16T10:00:00Z"}`,
		"unknown outcome":    `{"outcome":"maybe","observed_at":"2026-11-16T10:00:00Z"}`,
		"future observed_at": `{"outcome":"pass","observed_at":"2099-01-01T00:00:00Z"}`,
		"bad observed_at":    `{"outcome":"pass","observed_at":"yesterday"}`,
		"stranger executor":  `{"outcome":"pass","observed_at":"2026-11-16T10:00:00Z","executed_by":"` + uuid.NewString() + `"}`,
		"bad executor":       `{"outcome":"pass","observed_at":"2026-11-16T10:00:00Z","executed_by":"nope"}`,
		"note too long":      `{"outcome":"pass","observed_at":"2026-11-16T10:00:00Z","note":"` + strings.Repeat("x", 10001) + `"}`,
		"not json":           `{`,
	} {
		status, _ := s.do(t, a, http.MethodPost, path, body)
		assert.Equal(t, http.StatusBadRequest, status, name)
	}
	assert.Zero(t, s.count(t, &controlchain.ControlTestResultRow{}))

	// A member of the tenant may be named; the typist stays in the audit trail.
	status, body := s.do(t, a, http.MethodPost, path, `{"outcome":"pass","observed_at":"2026-11-16T10:00:00Z","executed_by":"`+s.ownerA.String()+`"}`)
	require.Equal(t, http.StatusCreated, status, "%v", body)
	assert.Equal(t, map[string]any{"kind": "user", "id": s.ownerA.String()}, body["executed_by"])
	// system and vendor_contact are not settable: the body has no kind.
	status, body = s.do(t, a, http.MethodPost, path, `{"outcome":"pass","observed_at":"2026-11-16T10:00:00Z","executed_by":{"kind":"system","id":"worker"}}`)
	assert.Equal(t, http.StatusBadRequest, status, "%v", body)

	// Invalid ids and a missing evidence id.
	for _, p := range []string{testBase + "/not-a-uuid/results", testBase + "/not-a-uuid/results/latest"} {
		status, _ = s.do(t, a, http.MethodGet, p, "")
		assert.Equal(t, http.StatusBadRequest, status, p)
	}
	status, _ = s.do(t, a, http.MethodPost, path+"/not-a-uuid/evidence", `{"evidence_id":"`+uuid.NewString()+`"}`)
	assert.Equal(t, http.StatusBadRequest, status)
	status, _ = s.do(t, a, http.MethodPost, path+"/"+uuid.NewString()+"/evidence", `{}`)
	assert.Equal(t, http.StatusBadRequest, status)
	status, _ = s.do(t, a, http.MethodGet, path+"?limit=abc", "")
	assert.Equal(t, http.StatusBadRequest, status)
}

func TestControlTestResultRoutes_LatestWithNoResult(t *testing.T) {
	s := newIntCtlStack(t)
	a := s.asA(intCtlResultPerms)
	ids := s.seed(t)
	status, body := s.do(t, a, http.MethodGet, testBase+"/"+ids.test+"/results/latest", "")
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, domain.IntCtlNoResult, body["error"], "a test that never ran")
	status, body = s.do(t, a, http.MethodGet, testBase+"/"+uuid.NewString()+"/results/latest", "")
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, domain.IntCtlTestNotFound, body["error"], "no such test")
	status, body = s.do(t, s.asB(), http.MethodGet, testBase+"/"+ids.test+"/results/latest", "")
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, domain.IntCtlTestNotFound, body["error"], "another tenant's test reads like an absent one")
}

func TestControlTestResultRoutes_Forbidden(t *testing.T) {
	s := newIntCtlStack(t)
	ids := s.seed(t)
	full := s.asA(intCtlResultPerms)
	r := s.record(t, full, ids.test, resultBody)
	ev := s.evidence(t, s.tenantA)
	evPath := testBase + "/" + ids.test + "/results/" + r["id"].(string) + "/evidence"

	// Reading needs controls:read; recording controls:update; attaching evidences:create.
	status, _ := s.do(t, s.asA("compliance:controls:read"), http.MethodPost, testBase+"/"+ids.test+"/results", resultBody)
	assert.Equal(t, http.StatusForbidden, status, "record without controls:update")
	status, _ = s.do(t, s.asA(intCtlAllPerms), http.MethodPost, evPath, `{"evidence_id":"`+ev+`"}`)
	assert.Equal(t, http.StatusForbidden, status, "attach with every controls:* but no evidences:create")
	status, _ = s.do(t, s.asA("compliance:evidences:create"), http.MethodPost, testBase+"/"+ids.test+"/results", resultBody)
	assert.Equal(t, http.StatusForbidden, status, "record with evidences:create only")
	for _, p := range []string{testBase + "/" + ids.test + "/results", testBase + "/" + ids.test + "/results/latest", evPath} {
		status, _ = s.do(t, s.asA("compliance:controls:update,compliance:evidences:create"), http.MethodGet, p, "")
		assert.Equal(t, http.StatusForbidden, status, "read without controls:read: "+p)
	}
	assert.EqualValues(t, 1, s.count(t, &controlchain.ControlTestResultRow{}))
	assert.EqualValues(t, 1, s.count(t, &controlchain.CanonicalEdgeRow{}), "only the implementation's applies_to edge exists")
}

func TestControlTestResultRoutes_Unauthorized(t *testing.T) {
	s := newIntCtlStack(t)
	ids := s.seed(t)
	r := s.record(t, s.asA(intCtlResultPerms), ids.test, resultBody)
	ev := s.evidence(t, s.tenantA)
	noTenant := intCtlReq{user: s.userA, perms: intCtlResultPerms}
	noUser := intCtlReq{tenant: s.tenantA, perms: intCtlResultPerms}
	evPath := testBase + "/" + ids.test + "/results/" + r["id"].(string) + "/evidence"
	for name, who := range map[string]intCtlReq{"no tenant": noTenant, "no user": noUser} {
		for _, c := range []struct{ method, path, body string }{
			{http.MethodGet, testBase + "/" + ids.test + "/results", ""},
			{http.MethodPost, testBase + "/" + ids.test + "/results", resultBody},
			{http.MethodGet, testBase + "/" + ids.test + "/results/latest", ""},
			{http.MethodGet, evPath, ""},
			{http.MethodPost, evPath, `{"evidence_id":"` + ev + `"}`},
		} {
			status, _ := s.do(t, who, c.method, c.path, c.body)
			assert.Equal(t, http.StatusUnauthorized, status, name+" "+c.method+" "+c.path)
		}
	}
	assert.EqualValues(t, 1, s.count(t, &controlchain.ControlTestResultRow{}))
}

// Tenant B reaches nothing of tenant A's by id: a 404 with the same shape as a
// fabricated id on all five routes, and A's rows are untouched.
func TestControlTestResultRoutes_CrossTenantIsNotFound(t *testing.T) {
	s := newIntCtlStack(t)
	ids := s.seed(t)
	a := s.asA(intCtlResultPerms)
	r := s.record(t, a, ids.test, resultBody)
	resultID := r["id"].(string)
	b := intCtlReq{tenant: s.tenantB, user: s.userB, perms: intCtlResultPerms}
	evB := s.evidence(t, s.tenantB)
	evA := s.evidence(t, s.tenantA)
	fake := uuid.NewString()
	base := testBase + "/" + ids.test + "/results"
	for name, c := range map[string]struct{ method, path, body string }{
		"list":                   {http.MethodGet, base, ""},
		"record":                 {http.MethodPost, base, resultBody},
		"latest":                 {http.MethodGet, base + "/latest", ""},
		"attach (own evidence)":  {http.MethodPost, base + "/" + resultID + "/evidence", `{"evidence_id":"` + evB + `"}`},
		"attach (A's evidence)":  {http.MethodPost, base + "/" + resultID + "/evidence", `{"evidence_id":"` + evA + `"}`},
		"list evidence of a res": {http.MethodGet, base + "/" + resultID + "/evidence", ""},
	} {
		status, foreign := s.do(t, b, c.method, c.path, c.body)
		assert.Equal(t, http.StatusNotFound, status, name)
		fakePath := replaceOnce(c.path, ids.test, fake)
		fakePath = replaceOnce(fakePath, resultID, fake)
		status, fabricated := s.do(t, b, c.method, fakePath, c.body)
		assert.Equal(t, http.StatusNotFound, status, name+" (fabricated)")
		assert.Equal(t, foreign["error"], fabricated["error"], name+": the same body as a fabricated id")
	}

	// Tenant A: evidence of tenant B, a result of another test, and a result that does not exist are 404s.
	status, _ := s.do(t, a, http.MethodPost, base+"/"+resultID+"/evidence", `{"evidence_id":"`+evB+`"}`)
	assert.Equal(t, http.StatusNotFound, status, "another tenant's evidence")
	status, _ = s.do(t, a, http.MethodPost, base+"/"+fake+"/evidence", `{"evidence_id":"`+evA+`"}`)
	assert.Equal(t, http.StatusNotFound, status, "no such result")
	otherTest := s.otherTest(t, ids)
	status, _ = s.do(t, a, http.MethodPost, testBase+"/"+otherTest+"/results/"+resultID+"/evidence", `{"evidence_id":"`+evA+`"}`)
	assert.Equal(t, http.StatusNotFound, status, "a result of another test")
	status, _ = s.do(t, a, http.MethodPost, base+"/"+resultID+"/evidence", `{"evidence_id":"`+fake+`"}`)
	assert.Equal(t, http.StatusNotFound, status, "no such evidence")
	status, _ = s.do(t, a, http.MethodGet, testBase+"/"+otherTest+"/results/"+resultID+"/evidence", "")
	assert.Equal(t, http.StatusNotFound, status)

	assert.EqualValues(t, 1, s.count(t, &controlchain.ControlTestResultRow{}))
	assert.EqualValues(t, 1, s.count(t, &controlchain.CanonicalEdgeRow{}), "no evidenced_by edge: only A's applies_to")
}

// otherTest creates a second test under the same implementation.
func (s *intCtlStack) otherTest(t *testing.T, ids chainIDs) string {
	t.Helper()
	status, body := s.do(t, s.asA(intCtlAllPerms), http.MethodPost, implBase+"/"+ids.impl+"/tests", manualTestBody)
	require.Equal(t, http.StatusCreated, status, "%v", body)
	return body["id"].(string)
}

// A deleted test hides its results.
func TestControlTestResultRoutes_DeletedTestHidesItsResults(t *testing.T) {
	s := newIntCtlStack(t)
	a := s.asA(intCtlResultPerms)
	ids := s.seed(t)
	s.record(t, a, ids.test, resultBody)
	status, _ := s.do(t, a, http.MethodDelete, testBase+"/"+ids.test, "")
	require.Equal(t, http.StatusNoContent, status)
	for _, p := range []string{testBase + "/" + ids.test + "/results", testBase + "/" + ids.test + "/results/latest"} {
		status, _ = s.do(t, a, http.MethodGet, p, "")
		assert.Equal(t, http.StatusNotFound, status, p)
	}
	status, _ = s.do(t, a, http.MethodPost, testBase+"/"+ids.test+"/results", resultBody)
	assert.Equal(t, http.StatusNotFound, status)
}

// AC3: a result is append-only. No route updates or deletes it: the router has
// no PATCH, PUT or DELETE under a results path (read from cmd/server/main.go), and
// the stack answers none of them either.
func TestControlTestResultRoutes_NoUpdateOrDelete(t *testing.T) {
	all, err := routes.Extract(filepath.Join("..", "..", "cmd", "server", "main.go"))
	require.NoError(t, err)
	var underResults int
	for _, r := range all {
		if !strings.Contains(r.Path, "/results") {
			continue
		}
		if strings.HasPrefix(r.Path, "/compliance/tests/") || strings.Contains(r.Path, "/compliance/tests/") {
			underResults++
		}
		assert.Containsf(t, []string{http.MethodGet, http.MethodPost}, r.Method, "%s: a result is append-only", r)
	}
	assert.GreaterOrEqual(t, underResults, 5, "the result routes were found in main.go")

	s := newIntCtlStack(t)
	a := s.asA(intCtlResultPerms)
	ids := s.seed(t)
	r := s.record(t, a, ids.test, resultBody)
	rid := r["id"].(string)
	for _, method := range []string{http.MethodPatch, http.MethodPut, http.MethodDelete} {
		for _, p := range []string{
			testBase + "/" + ids.test + "/results", testBase + "/" + ids.test + "/results/" + rid,
			testBase + "/" + ids.test + "/results/latest", testBase + "/" + ids.test + "/results/" + rid + "/evidence",
		} {
			status, _ := s.do(t, a, method, p, `{"outcome":"fail"}`)
			assert.Containsf(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, status, "%s %s", method, p)
		}
	}
	status, body := s.do(t, a, http.MethodGet, testBase+"/"+ids.test+"/results/latest", "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "pass", body["outcome"], "the result is unchanged")
	assert.EqualValues(t, 1, s.count(t, &controlchain.ControlTestResultRow{}))
}
