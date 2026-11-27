//go:build crosstenant

// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package crosstenant

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
)

const testPassword = "Xt-Passw0rd!long-enough-1"

// principal is one tenant's root user, the strongest in-tenant caller.
type principal struct {
	name     string // "a" or "b"
	email    string
	tenantID string
	userID   string

	mu      sync.Mutex
	access  string
	refresh string
	// csrf is the double-submit token of the session, used only in cookie mode.
	csrf string
	// totp is the authenticator secret once p is enrolled (TestCrossTenant_MFABranches);
	// login then goes through /auth/mfa/challenge.
	totp string
	// lastStep is the last TOTP time step spent, so a code is never replayed.
	lastStep int64
	// memo holds the shared seed of each kind; every id a seed learns is also
	// recorded in ids so the echo scan knows all of this tenant's data.
	memo map[Kind]map[Kind]string
	ids  map[string]struct{}
}

func (p *principal) learn(id string) {
	if id == "" {
		return
	}
	p.mu.Lock()
	p.ids[id] = struct{}{}
	p.mu.Unlock()
}

// foreign returns everything that identifies this tenant's data: its ids, its
// email and its canary prefix.
func (p *principal) foreign(run string) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := []string{p.email, p.tenantID, p.userID, "xt-" + run + "-" + p.name + "-"}
	for id := range p.ids {
		out = append(out, id)
	}
	return out
}

type harness struct {
	t      *testing.T
	base   string
	client *http.Client
	run    string
	A, B   *principal
	// supplied is every id the attacker put in a request so far.
	supplied []string

	// cookie makes do() carry the session as the HttpOnly or_access cookie plus
	// the double-submit CSRF header, as the browser SPA does, instead of Bearer.
	// Provisioning and login always use Bearer or no credential.
	cookie bool

	roleMu  sync.Mutex
	roleIDs map[string]string // predefined role name (lower case) -> id
}

// predefinedRole is the id of a predefined RBAC role ("viewer", "analyst"). The
// built-in roles are shared by every tenant, so any principal can list them.
func (h *harness) predefinedRole(name string) string {
	h.roleMu.Lock()
	defer h.roleMu.Unlock()
	if id, ok := h.roleIDs[name]; ok {
		return id
	}
	status, body := h.do(h.B, "GET", "/api/v1/rbac/roles", "")
	if status != 200 {
		h.t.Fatalf("list roles as B answered %d %.200s: no predefined role id for %s", status, body, name)
	}
	var roles struct {
		Roles []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			IsPredefined bool   `json:"is_predefined"`
		} `json:"roles"`
	}
	_ = json.Unmarshal([]byte(body), &roles)
	h.roleIDs = map[string]string{}
	for _, r := range roles.Roles {
		if r.IsPredefined {
			h.roleIDs[strings.ToLower(r.Name)] = r.ID
		}
	}
	id, ok := h.roleIDs[name]
	if !ok {
		h.t.Fatalf("no predefined role %q among %d roles", name, len(roles.Roles))
	}
	return id
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	base := strings.TrimRight(os.Getenv("OPENRISK_BASE_URL"), "/")
	if base == "" {
		t.Skip("OPENRISK_BASE_URL is not set: the cross-tenant suite needs a booted server (make test-cross-tenant)")
	}
	h := &harness{
		t:      t,
		base:   base,
		client: &http.Client{Timeout: 30 * time.Second},
		run:    randHex(4),
	}
	// Set but silent is a failure, never a skip: a green run that probed nothing
	// would look like coverage.
	status, body := h.raw("GET", "/api/v1/health", "", "")
	if status != 200 {
		t.Fatalf("OPENRISK_BASE_URL=%s is set but /api/v1/health answered %d %s", base, status, body)
	}
	h.A = h.provision("a")
	h.B = h.provision("b")
	if h.A.tenantID == h.B.tenantID {
		t.Fatalf("tenants A and B are the same organisation %s", h.A.tenantID)
	}
	return h
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *harness) raw(method, path, token, body string) (int, string) {
	return h.send(method, path, token, body, nil)
}

// send is raw with an optional hook on the request, for the cookie session.
func (h *harness) send(method, path, token, body string, mod func(*http.Request)) (int, string) {
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, h.base+path, rdr)
	if err != nil {
		h.t.Fatalf("build request %s %s: %v", method, path, err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if mod != nil {
		mod(req)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return resp.StatusCode, string(b)
}

// do calls the API as p. On a 401 it logs p in again once; a second 401 is
// returned as is, and Evaluate reports it as a suite defect, not a pass.
func (h *harness) do(p *principal, method, path, body string) (int, string) {
	status, resp := h.once(p, method, path, body)
	if status == http.StatusUnauthorized {
		h.login(p)
		status, resp = h.once(p, method, path, body)
	}
	return status, resp
}

// once sends one request with p's current session, as Bearer or, in cookie mode,
// as the or_access cookie with the or_csrf double-submit cookie and header. The
// cookie mode sends no Authorization header at all.
func (h *harness) once(p *principal, method, path, body string) (int, string) {
	p.mu.Lock()
	tok, csrf := p.access, p.csrf
	p.mu.Unlock()
	if !h.cookie {
		return h.raw(method, path, tok, body)
	}
	return h.send(method, path, "", body, func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: "or_access", Value: tok})
		r.AddCookie(&http.Cookie{Name: "or_csrf", Value: csrf})
		r.Header.Set("X-CSRF-Token", csrf)
	})
}

func (h *harness) provision(name string) *principal {
	h.t.Helper()
	p := &principal{
		name:  name,
		email: fmt.Sprintf("xt-%s-%s@xt.example.com", h.run, name),
		memo:  map[Kind]map[Kind]string{},
		ids:   map[string]struct{}{},
	}
	reg, _ := json.Marshal(map[string]string{
		"email": p.email, "password": testPassword,
		"full_name": "XT " + strings.ToUpper(name), "company_name": "xt-" + h.run + "-" + name + "-org",
	})
	status, body := h.raw("POST", "/api/v1/auth/register", "", string(reg))
	if status != 201 {
		h.t.Fatalf("register tenant %s: %d %s", name, status, body)
	}
	var out struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
		Organization struct {
			ID string `json:"id"`
		} `json:"organization"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		h.t.Fatalf("register %s: %v", name, err)
	}
	p.userID, p.tenantID = out.User.ID, out.Organization.ID
	h.login(p)
	return p
}

// login obtains a session for p. A fresh root account is inside the MFA grace
// period and gets its tokens straight away. When the deployment enforces
// enrolment at once the response carries mfa_token instead, and the harness
// enrols a TOTP secret through the production endpoints, so the guard is never
// switched off.
func (h *harness) login(p *principal) {
	h.t.Helper()
	creds, _ := json.Marshal(map[string]string{"email": p.email, "password": testPassword})
	status, body := h.raw("POST", "/api/v1/auth/login", "", string(creds))
	if status != 200 {
		h.t.Fatalf("login %s: %d %s", p.name, status, body)
	}
	var out struct {
		TokenPair struct {
			Access  string `json:"access_token"`
			Refresh string `json:"refresh_token"`
		} `json:"token_pair"`
		MFAToken string `json:"mfa_token"`
		CSRF     string `json:"csrf_token"`
	}
	_ = json.Unmarshal([]byte(body), &out)
	switch {
	case out.TokenPair.Access == "" && out.MFAToken != "" && p.totp != "":
		out.TokenPair.Access, out.TokenPair.Refresh, out.CSRF = h.challengeMFA(p, out.MFAToken)
	case out.TokenPair.Access == "" && out.MFAToken != "":
		out.TokenPair.Access, out.TokenPair.Refresh = h.enrolMFA(p, out.MFAToken)
	}
	if out.TokenPair.Access == "" {
		// The status line only: the body may carry an mfa_token.
		h.t.Fatalf("login %s returned no access token (body of %d bytes withheld)", p.name, len(body))
	}
	p.mu.Lock()
	p.access, p.refresh = out.TokenPair.Access, out.TokenPair.Refresh
	// The double-submit check compares the cookie with the header only, so a
	// login that returned no csrf_token still gets a usable random value.
	p.csrf = out.CSRF
	if p.csrf == "" {
		p.csrf = randHex(16)
	}
	p.mu.Unlock()
}

func (h *harness) enrolMFA(p *principal, mfaToken string) (string, string) {
	h.t.Helper()
	status, body := h.raw("POST", "/api/v1/auth/mfa/setup", mfaToken, "")
	if status != 200 {
		h.t.Fatalf("mfa setup %s: %d %s", p.name, status, body)
	}
	var setup struct {
		Secret string `json:"secret"`
	}
	_ = json.Unmarshal([]byte(body), &setup)
	code, err := totp.GenerateCode(setup.Secret, time.Now())
	if err != nil {
		h.t.Fatalf("totp: %v", err)
	}
	vb, _ := json.Marshal(map[string]string{"code": code})
	status, body = h.raw("POST", "/api/v1/auth/mfa/verify", mfaToken, string(vb))
	if status != 200 {
		h.t.Fatalf("mfa verify %s: %d %s", p.name, status, body)
	}
	var out struct {
		TokenPair struct {
			Access  string `json:"access_token"`
			Refresh string `json:"refresh_token"`
		} `json:"token_pair"`
	}
	_ = json.Unmarshal([]byte(body), &out)
	return out.TokenPair.Access, out.TokenPair.Refresh
}

func hashBody(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func (h *harness) canary(p *principal, kind Kind) string {
	return fmt.Sprintf("xt-%s-%s-%s", h.run, p.name, kind)
}

// create posts a seed request as p and returns the "id" of the response.
func (h *harness) create(p *principal, path, body string) string {
	h.t.Helper()
	status, resp := h.do(p, "POST", path, body)
	if status < 200 || status > 299 {
		h.t.Logf("seed: POST %s as %s answered %d %.200s", path, p.name, status, resp)
		return ""
	}
	return h.idOf(p, resp)
}

func (h *harness) idOf(p *principal, resp string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(resp), &m); err != nil {
		return ""
	}
	// Some create routes wrap the object ({"invitation": {...}}).
	if _, ok := m["id"]; !ok {
		for _, v := range m {
			if inner, ok := v.(map[string]any); ok {
				if _, ok := inner["id"]; ok {
					m = inner
					break
				}
			}
		}
	}
	var id string
	switch v := m["id"].(type) {
	case string:
		id = v
	case float64:
		id = fmt.Sprintf("%d", int64(v))
	}
	p.learn(id)
	return id
}

func newUUID() string { return uuid.NewString() }

var _ = bytes.NewBuffer

// totpCode returns a code for the first time step after the last one this
// principal spent, waiting when that step is more than one ahead of the clock
// (a server accepts one step of skew, and refuses a code it has already seen).
// The step is reserved under p.mu and the wait happens outside it, so a
// concurrent request of p is never held up by the clock.
func (p *principal) totpCode() (string, error) {
	p.mu.Lock()
	step := time.Now().Unix() / 30
	if step <= p.lastStep {
		step = p.lastStep + 1
	}
	p.lastStep = step
	secret := p.totp
	p.mu.Unlock()
	for step > time.Now().Unix()/30+1 {
		time.Sleep(time.Second)
	}
	return totp.GenerateCode(secret, time.Unix(step*30, 0))
}

// enrolSession turns MFA on for p through the production endpoints, from the
// session p already holds (a user in the grace period may do so from Settings).
func (h *harness) enrolSession(p *principal) {
	h.t.Helper()
	p.mu.Lock()
	tok := p.access
	p.mu.Unlock()
	status, body := h.raw("POST", "/api/v1/auth/mfa/setup", tok, "")
	if status != 200 {
		h.t.Fatalf("mfa setup %s: %d (body of %d bytes withheld)", p.name, status, len(body))
	}
	var setup struct {
		Secret string `json:"secret"`
	}
	_ = json.Unmarshal([]byte(body), &setup)
	if setup.Secret == "" {
		h.t.Fatalf("mfa setup %s returned no secret", p.name)
	}
	p.mu.Lock()
	p.totp = setup.Secret
	p.mu.Unlock()
	code, err := p.totpCode()
	if err != nil {
		h.t.Fatalf("totp: %v", err)
	}
	vb, _ := json.Marshal(map[string]string{"code": code})
	if status, _ = h.raw("POST", "/api/v1/auth/mfa/verify", tok, string(vb)); status != 200 {
		h.t.Fatalf("mfa verify %s: %d", p.name, status)
	}
}

// challengeMFA is the second leg of an MFA login for an enrolled principal.
func (h *harness) challengeMFA(p *principal, mfaToken string) (string, string, string) {
	h.t.Helper()
	code, err := p.totpCode()
	if err != nil {
		h.t.Fatalf("totp: %v", err)
	}
	vb, _ := json.Marshal(map[string]string{"code": code})
	status, body := h.raw("POST", "/api/v1/auth/mfa/challenge", mfaToken, string(vb))
	if status != 200 {
		h.t.Fatalf("mfa challenge %s: %d (body of %d bytes withheld)", p.name, status, len(body))
	}
	var out struct {
		TokenPair struct {
			Access  string `json:"access_token"`
			Refresh string `json:"refresh_token"`
		} `json:"token_pair"`
		CSRF string `json:"csrf_token"`
	}
	_ = json.Unmarshal([]byte(body), &out)
	return out.TokenPair.Access, out.TokenPair.Refresh, out.CSRF
}
