//go:build crosstenant

// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package crosstenant

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

// The entry paths the Bearer-only suite does not take (#1521, RR-SCOPE-1): the
// browser cookie session, a user who belongs to both tenants, an own object that
// names a foreign id in the body, and the two MFA branches of a fresh root.

// TestCrossTenant_CookieSession runs every probe twice, once with Bearer and once
// with the or_access cookie plus the double-submit CSRF header, each on its own
// pair of fresh tenants (a second pass over the same tenants meets state the
// first one left: a saved view already renamed, a workflow already replaced), and
// requires every answer (A's attack, A's random-id twin, B's control) to match.
// The cookie run's 2xx controls are what prove the cookie authenticates: it
// carries no Authorization header. The csrf subtest adds that a cookie write
// needs the double-submit header, and that a cookie the server cannot verify is
// refused even with a matching header.
func TestCrossTenant_CookieSession(t *testing.T) {
	var bearer, cookie map[string]answers
	t.Run("bearer", func(t *testing.T) {
		hb := newHarness(t)
		hb.seedBoth(t)
		bearer = hb.runProbes(t, nil)
	})
	h := newHarness(t)
	h.cookie = true
	h.seedBoth(t)
	t.Run("cookie", func(t *testing.T) { cookie = h.runProbes(t, nil) })
	h.cookie = false

	if len(bearer) == 0 || len(cookie) != len(bearer) {
		t.Fatalf("the runs are not comparable: bearer saw %d probes, cookie %d", len(bearer), len(cookie))
	}
	for id, b := range bearer {
		if c := cookie[id]; c != b {
			t.Errorf("%s: cookie session answered attack=%d random=%d control=%d, Bearer attack=%d random=%d control=%d",
				id, c.attack, c.random, c.control, b.attack, b.random, b.control)
		}
	}

	t.Run("csrf_is_required_on_a_write", func(t *testing.T) {
		h.A.mu.Lock()
		tok, csrf := h.A.access, h.A.csrf
		h.A.mu.Unlock()
		write := func(mod func(*http.Request)) (int, string) {
			return h.send("POST", "/api/v1/risks", "", `{"title":"csrf","description":"x","impact":1,"probability":0.1}`, mod)
		}
		cookies := func(access string) func(*http.Request) {
			return func(r *http.Request) {
				r.AddCookie(&http.Cookie{Name: "or_access", Value: access})
				r.AddCookie(&http.Cookie{Name: "or_csrf", Value: csrf})
			}
		}
		csrfRefused := func(name string, st int, body string) {
			var e struct {
				Code string `json:"code"`
			}
			_ = json.Unmarshal([]byte(body), &e)
			if st != http.StatusForbidden || e.Code != "CSRF_FAILED" {
				t.Errorf("%s answered %d code %q, want 403 CSRF_FAILED", name, st, e.Code)
			}
		}
		st, body := write(cookies(tok))
		csrfRefused("cookie write without the CSRF header", st, body)
		st, body = write(func(r *http.Request) { cookies(tok)(r); r.Header.Set("X-CSRF-Token", "not-the-cookie-value") })
		csrfRefused("cookie write with a mismatched CSRF header", st, body)
		// Positive control: the same write with a matching header succeeds, so the
		// two refusals above are the CSRF check and not a broken request.
		if st, _ = write(func(r *http.Request) { cookies(tok)(r); r.Header.Set("X-CSRF-Token", csrf) }); st < 200 || st > 299 {
			t.Errorf("cookie write with a matching CSRF header answered %d, want 2xx", st)
		}
		// A cookie that is not a valid token passes the CSRF check and is then
		// refused by token validation: the cookie is what authenticates.
		if st, _ = write(func(r *http.Request) { cookies("not-a-token")(r); r.Header.Set("X-CSRF-Token", csrf) }); st != http.StatusUnauthorized {
			t.Errorf("an unverifiable or_access cookie with a matching CSRF header answered %d, want 401", st)
		}
	})
	t.Run("no_credential", func(t *testing.T) {
		if st, _ := h.raw("GET", "/api/v1/risks", "", ""); st != http.StatusUnauthorized {
			t.Errorf("a request with no token and no cookie answered %d, want 401", st)
		}
	})
}

// tenantOf reads the tenant_id claim of a JWT without verifying it: the test
// only needs to know which tenant the server scoped the token to. The token is
// never logged.
func tenantOf(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var c struct {
		TenantID string `json:"tenant_id"`
	}
	_ = json.Unmarshal(raw, &c)
	return c.TenantID
}

// TestCrossTenant_TwoTenantPrincipal makes A's root a member of B as well, then
// runs every probe with A's token, which stays scoped to A. Each route must
// answer for B's ids as for a random id: a membership elsewhere is not access.
//
// The membership is written by SQL into organization_members: no API route
// creates it for an existing user without the emailed invitation token
// (POST /invitations/accept), which the test cannot read. The row carries the
// same columns the accept use case writes and is removed again on cleanup. Every
// statement is filtered by organization_id (the tenant column of that table).
func TestCrossTenant_TwoTenantPrincipal(t *testing.T) {
	h := newHarness(t)
	h.seedBoth(t)

	db, err := h.database()
	if err != nil {
		t.Fatalf("the two-tenant principal needs the membership row: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO organization_members
		(id, organization_id, user_id, role, is_active, status, joined_at, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, 'root', true, 'active', now(), now(), now())`,
		h.B.tenantID, h.A.userID); err != nil {
		t.Fatalf("add A's user to B's organization: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM organization_members WHERE organization_id = $1 AND user_id = $2`, h.B.tenantID, h.A.userID)
	})
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM organization_members WHERE organization_id = $1 AND user_id = $2`,
		h.B.tenantID, h.A.userID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("membership of A's user in B: count %d, err %v", n, err)
	}

	// A fresh session after the membership exists, and proof that it is scoped to A.
	h.login(h.A)
	h.A.mu.Lock()
	scoped := tenantOf(h.A.access)
	h.A.mu.Unlock()
	if scoped != h.A.tenantID {
		t.Fatalf("A's token is scoped to tenant %q, want A's %q: a second membership moved the session", scoped, h.A.tenantID)
	}

	// Switching into B is what a membership grants, by design (the endpoint
	// answers 200 for a member and 403 for anyone else), so this one probe cannot
	// answer as for a random id here. It is probed in the single-membership runs.
	skip := map[string]string{"POST /api/v1/auth/switch-org": "a member of B may switch into B"}
	h.runProbes(t, skip)

	// The session must still be A's after the whole run.
	h.A.mu.Lock()
	scoped = tenantOf(h.A.access)
	h.A.mu.Unlock()
	if scoped != h.A.tenantID {
		t.Errorf("A's token ended the run scoped to %q, want %q", scoped, h.A.tenantID)
	}
}

// TestCrossTenant_OwnObjectForeignBodyID probes the shape where the attacker owns
// the target and names B's id in the body.
func TestCrossTenant_OwnObjectForeignBodyID(t *testing.T) {
	h := newHarness(t)
	h.seedBoth(t)
	for _, p := range ownBodyProbes {
		t.Run(p.ID(), func(t *testing.T) {
			o, err := h.observeOwn(p)
			if err != nil {
				t.Fatalf("suite defect: %v", err)
			}
			if p.BlockedBy != "" && o.ControlStatus >= 500 {
				t.Skipf("blocked on %s: the control with A's own ids answers %d (attack %d, random %d); the probe runs once that is fixed",
					p.BlockedBy, o.ControlStatus, o.AttackStatus, o.RandomStatus)
			}
			for _, f := range EvaluateOwnBody(o) {
				t.Errorf("%s: %s", p.ID(), f)
			}
			t.Logf("attack=%d random=%d control=%d", o.AttackStatus, o.RandomStatus, o.ControlStatus)
		})
	}
}

// observeOwn runs one own-object probe: A's own object in the path, B's id in
// the body, then the same request with a random id, then A's own read of its
// object, then the positive control with A's own ids.
func (h *harness) observeOwn(p Probe) (OwnBodyObservation, error) {
	var o OwnBodyObservation
	attack, control := map[Kind]string{}, map[Kind]string{}
	for _, k := range p.Own {
		s := h.seedFresh(h.A, k)
		if s == nil || s[k] == "" {
			return o, errSeed(k, "A")
		}
		attack[k], control[k] = s[k], s[k]
	}
	for _, k := range p.Kinds {
		b, a := h.seedFresh(h.B, k), h.seedFresh(h.A, k)
		if b == nil || b[k] == "" {
			return o, errSeed(k, "B")
		}
		if a == nil || a[k] == "" {
			return o, errSeed(k, "A")
		}
		attack[k], control[k] = b[k], a[k]
	}

	// A's own object takes the path params; the body kinds are the only ones
	// render may replace, so the random twin keeps A's own id in the path.
	pp := p
	segs := strings.Split(p.Path, "/")
	n := 0
	for i, s := range segs {
		if strings.HasPrefix(s, ":") {
			segs[i] = attack[p.Own[n]]
			n++
		}
	}
	pp.Path = strings.Join(segs, "/")
	path, body, supplied := h.render(pp, attack, false)
	rpath, rbody, _ := h.render(pp, attack, true)
	cpath, cbody, _ := h.render(pp, control, false)

	read := h.readPath(Probe{Kinds: p.Own}, attack)
	if read == "" {
		return o, errSeed(p.Own[0], "A (no read route)")
	}
	if st, _ := h.do(h.A, "GET", uncached(read), ""); st != http.StatusOK {
		return o, seedError("A's read of its own object answered " + http.StatusText(st) + " before the attempt")
	}
	before, err := h.settled(h.A, read)
	if err != nil {
		return o, err
	}
	o.BeforeHash = hashBody(before)

	o.AttackStatus, o.AttackBody = h.do(h.A, p.Method, path, body)
	o.RandomStatus, _ = h.do(h.A, p.Method, rpath, rbody)

	// A fresh query string: the after-read must not be the cached before-read.
	st, after := h.do(h.A, "GET", uncached(read), "")
	if st != http.StatusOK {
		return o, seedError("A's read of its own object answered " + http.StatusText(st) + " after the attempt")
	}
	o.AfterHash, o.AfterBody = hashBody(after), after

	o.ControlStatus, _ = h.do(h.A, p.Method, cpath, cbody)
	if p.LinkSQL != "" && len(p.Own) == 1 && len(p.Kinds) == 1 {
		db, err := h.database()
		if err != nil {
			return o, err
		}
		var n int
		if err := db.QueryRow(p.LinkSQL, h.A.tenantID, attack[p.Own[0]], attack[p.Kinds[0]]).Scan(&n); err != nil {
			return o, seedError("link query for " + p.ID() + ": " + err.Error())
		}
		o.LinkChecked, o.ForeignLinkStored = true, n != 0
	}
	if p.DropsForeign {
		// The control must have applied A's own ids, or a 200 to B's id would
		// prove only that the field is ignored altogether.
		o.DropsForeign = true
		_, applied := h.do(h.A, "GET", uncached(read), "")
		o.ControlApplied = true
		for _, k := range p.Kinds {
			if !strings.Contains(applied, control[k]) {
				o.ControlApplied = false
			}
		}
	}
	o.Foreign = h.B.foreign(h.run)
	o.Supplied = supplied
	return o, nil
}

// database opens the suite's direct connection, for the checks no API can make
// (a membership row, a link that a tenant-filtered read hides). It is opened
// once per harness and closed with the test.
func (h *harness) database() (*sql.DB, error) {
	if h.db != nil {
		return h.db, nil
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, seedError("OPENRISK_BASE_URL is set but DATABASE_URL is not: this check needs the database (make test-cross-tenant sets both)")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	h.db = db
	h.t.Cleanup(func() { _ = db.Close() })
	return db, nil
}

type seedError string

func (e seedError) Error() string { return string(e) }

func errSeed(k Kind, who string) error { return seedError("cannot seed " + string(k) + " for " + who) }

// TestCrossTenant_MFABranches runs the suite for a root inside the 7-day MFA
// grace period (the state of every fresh registration) and for a root enrolled in
// TOTP, whose login stops at /auth/mfa/challenge. It observes the behaviour, it
// never changes it: the enrolment goes through the production endpoints and the
// guard stays on.
func TestCrossTenant_MFABranches(t *testing.T) {
	loginShape := func(h *harness, p *principal) (tokens, mfaToken bool) {
		creds, _ := json.Marshal(map[string]string{"email": p.email, "password": testPassword})
		st, body := h.raw("POST", "/api/v1/auth/login", "", string(creds))
		if st != 200 {
			t.Fatalf("login %s answered %d", p.name, st)
		}
		var out struct {
			TokenPair struct {
				Access string `json:"access_token"`
			} `json:"token_pair"`
			MFAToken string `json:"mfa_token"`
		}
		_ = json.Unmarshal([]byte(body), &out)
		return out.TokenPair.Access != "", out.MFAToken != ""
	}

	t.Run("grace_period", func(t *testing.T) {
		h := newHarness(t)
		if tok, mfa := loginShape(h, h.A); !tok || mfa {
			t.Fatalf("a fresh root must hold a session at once inside the grace period: token=%v mfa_token=%v", tok, mfa)
		}
		h.seedBoth(t)
		h.runProbes(t, nil)
	})
	t.Run("enrolled", func(t *testing.T) {
		h := newHarness(t)
		h.enrolSession(h.A)
		h.enrolSession(h.B)
		if tok, mfa := loginShape(h, h.A); tok || !mfa {
			t.Fatalf("an enrolled root must stop at the MFA challenge: token=%v mfa_token=%v", tok, mfa)
		}
		h.login(h.A)
		h.login(h.B)
		h.seedBoth(t)
		h.runProbes(t, nil)
	})
}
