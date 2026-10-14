// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package router mounts the HTTP routes and writes down, as it mounts them,
// which permission or role each one requires (ADR 0005, #992 spike).
//
// It does not enforce anything itself. A Check is turned into the existing
// middleware.RequirePermission / middleware.RequireRole handler, with the same
// arguments, in the same position of the chain as main.go mounts it today.
// The Matrix is a record of those arguments, kept next to the call that used
// them so the two cannot drift.
package router

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/opendefender/openrisk/internal/middleware"
)

// Check is one guard middleware. Inside a check the values are alternatives,
// exactly as the middleware reads them: RequirePermission("a", "b") admits a
// caller holding a OR b. Several checks on one route must all pass.
type Check struct {
	Permissions []string
	Roles       []string
}

// Permission is middleware.RequirePermission(perms...).
func Permission(perms ...string) Check { return Check{Permissions: perms} }

// Role is middleware.RequireRole(roles...). The middleware also admits root.
func Role(roles ...string) Check { return Check{Roles: roles} }

func (c Check) handler() fiber.Handler {
	if len(c.Roles) > 0 {
		return middleware.RequireRole(c.Roles...)
	}
	return middleware.RequirePermission(c.Permissions...)
}

func (c Check) String() string {
	if len(c.Roles) > 0 {
		return "role(" + strings.Join(c.Roles, "|") + ")"
	}
	return "perm(" + strings.Join(c.Permissions, "|") + ")"
}

// Guard is the list of checks a route mounts ahead of its handlers.
type Guard []Check

// SessionOnly says, at the mount site, that being signed in is the whole
// middleware-level authorization for this route.
var SessionOnly Guard

// Require builds a route guard from one or more checks.
func Require(checks ...Check) Guard { return Guard(checks) }

// Access is whether the auth gate runs before the route.
type Access string

const (
	Public        Access = "public"
	Authenticated Access = "authenticated"
)

// Row is one line of the route matrix.
type Row struct {
	Method string
	Path   string
	Access Access
	Checks []Check // group checks first, then the route's own
}

func (r Row) String() string {
	parts := make([]string, len(r.Checks))
	for i, c := range r.Checks {
		parts[i] = c.String()
	}
	guard := "session only"
	if len(parts) > 0 {
		guard = strings.Join(parts, " AND ")
	}
	return fmt.Sprintf("%-6s %-40s %-13s → %s", r.Method, r.Path, r.Access, guard)
}

// Matrix collects the rows in mount order.
type Matrix struct{ rows []Row }

// Rows returns the rows in the order they were mounted.
func (m *Matrix) Rows() []Row { return append([]Row(nil), m.rows...) }

// Mounter mounts routes on a Fiber router and records each one.
type Mounter struct {
	r      fiber.Router
	prefix string
	access Access
	checks []Check
	matrix *Matrix
}

// New wraps a Fiber router whose full path prefix is prefix.
func New(r fiber.Router, prefix string, access Access, matrix *Matrix) Mounter {
	return Mounter{r: r, prefix: prefix, access: access, matrix: matrix}
}

// Group is fiber.Router.Group with its checks mounted as group middleware, as
// protected.Group("/rbac/roles", adminRole) does today.
func (m Mounter) Group(prefix string, checks ...Check) Mounter {
	hs := make([]fiber.Handler, len(checks))
	for i, c := range checks {
		hs[i] = c.handler()
	}
	inherited := append(append([]Check(nil), m.checks...), checks...)
	return Mounter{
		r:      m.r.Group(prefix, hs...),
		prefix: joinPath(m.prefix, prefix),
		access: m.access,
		checks: inherited,
		matrix: m.matrix,
	}
}

func (m Mounter) add(mount func(string, ...fiber.Handler) fiber.Router, method, path string, g Guard, hs []fiber.Handler) {
	chain := make([]fiber.Handler, 0, len(g)+len(hs))
	for _, c := range g {
		chain = append(chain, c.handler())
	}
	chain = append(chain, hs...)
	// The same fiber.Router method main.go calls today (Get also adds HEAD).
	mount(path, chain...)
	m.matrix.rows = append(m.matrix.rows, Row{
		Method: method,
		Path:   joinPath(m.prefix, path),
		Access: m.access,
		Checks: append(append([]Check(nil), m.checks...), g...),
	})
}

func (m Mounter) Get(path string, g Guard, hs ...fiber.Handler) {
	m.add(m.r.Get, fiber.MethodGet, path, g, hs)
}
func (m Mounter) Post(path string, g Guard, hs ...fiber.Handler) {
	m.add(m.r.Post, fiber.MethodPost, path, g, hs)
}
func (m Mounter) Put(path string, g Guard, hs ...fiber.Handler) {
	m.add(m.r.Put, fiber.MethodPut, path, g, hs)
}
func (m Mounter) Patch(path string, g Guard, hs ...fiber.Handler) {
	m.add(m.r.Patch, fiber.MethodPatch, path, g, hs)
}
func (m Mounter) Delete(path string, g Guard, hs ...fiber.Handler) {
	m.add(m.r.Delete, fiber.MethodDelete, path, g, hs)
}

// Verify compares the matrix with the routes Fiber actually holds. A route
// mounted around the Mounter, or a row with no route behind it, is an error.
func Verify(app *fiber.App, m *Matrix) error {
	want := map[string]int{}
	for _, r := range m.rows {
		want[r.Method+" "+r.Path]++
	}
	got := map[string]int{}
	for _, r := range app.GetRoutes(true) {
		if r.Method == fiber.MethodHead {
			continue
		}
		got[r.Method+" "+r.Path]++
	}
	var problems []string
	for k, n := range got {
		if want[k] != n {
			problems = append(problems, fmt.Sprintf("%s: mounted %d time(s), recorded %d", k, n, want[k]))
		}
	}
	for k, n := range want {
		if _, ok := got[k]; !ok {
			problems = append(problems, fmt.Sprintf("%s: recorded %d time(s), not mounted", k, n))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("route matrix does not match the router:\n  %s", strings.Join(problems, "\n  "))
	}
	return nil
}

func joinPath(prefix, path string) string {
	switch {
	case prefix == "":
		return path
	case path == "" || path == "/":
		return prefix
	}
	return strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(path, "/")
}
