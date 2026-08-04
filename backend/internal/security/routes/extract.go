// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package routes extracts the API surface from the router source by parsing it,
// rather than from a hand-maintained list.
//
// This exists to serve audit finding F-07. Tenant-isolation coverage used to be
// a mosaic of per-module tests written by hand as leaks were found; its weakness
// was not what it covered but how it grew — a route added tomorrow was covered
// by nothing until somebody remembered to write its test. Deriving the surface
// from the source means the isolation suite cannot silently fall behind: a new
// route shows up here the moment it is registered.
package routes

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

// Route is one registered HTTP endpoint.
type Route struct {
	Method string // GET, POST, PUT, PATCH, DELETE
	Path   string // full path, e.g. /api/v1/risks/:id
	Router string // router variable it was registered on (api, protected, app...)
	Line   int    // line in the source file, for actionable failure messages
}

// String renders the route as it appears in a failure message.
func (r Route) String() string { return r.Method + " " + r.Path }

// IsParameterised reports whether the path takes an ID-like segment. These are
// the direct-object-reference surface an isolation test must probe: they let a
// caller name someone else's resource.
func (r Route) IsParameterised() bool {
	for _, segment := range strings.Split(r.Path, "/") {
		if strings.HasPrefix(segment, ":") {
			return true
		}
	}
	return false
}

// httpMethods are the Fiber registration calls we treat as routes. Use/Group are
// deliberately excluded: they mount middleware and prefixes, not endpoints.
var httpMethods = map[string]bool{
	"Get": true, "Post": true, "Put": true, "Patch": true, "Delete": true,
}

// Extract parses a router source file and returns every route it registers,
// sorted by path then method for stable output.
//
// It resolves two things that a regex over the same file cannot:
//   - group prefixes, so `protected.Group("/incidents")` followed by
//     `incidentsGroup.Get("/:id")` yields /api/v1/incidents/:id rather than /:id
//   - the base prefix from `app.Group("/api/v1")`
//
// Routes registered on `app` directly keep their literal path, since they are
// mounted outside the API group on purpose (scanner agents, webhooks).
func Extract(filename string) ([]Route, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}

	prefixes := collectGroupPrefixes(file)

	var found []Route
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !httpMethods[selector.Sel.Name] {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		// Only receivers we resolved as routers count. This is what separates a
		// route registration from `c.Get("X-Request-ID")` reading a header off the
		// Fiber context — both are Ident.Get calls with a string argument.
		prefix, isRouter := prefixes[receiver.Name]
		if !isRouter {
			return true
		}
		if len(call.Args) == 0 {
			return true
		}
		path, ok := stringLiteral(call.Args[0])
		if !ok {
			return true
		}

		found = append(found, Route{
			Method: strings.ToUpper(selector.Sel.Name),
			Path:   joinPath(prefix, path),
			Router: receiver.Name,
			Line:   fset.Position(call.Pos()).Line,
		})
		return true
	})

	sort.Slice(found, func(i, j int) bool {
		if found[i].Path != found[j].Path {
			return found[i].Path < found[j].Path
		}
		return found[i].Method < found[j].Method
	})
	return found, nil
}

// collectGroupPrefixes resolves each router variable to its path prefix.
//
// It walks assignments of the form `x := y.Group("/p")` repeatedly, because a
// group may be declared after the group it derives from has itself been
// resolved. Iterating to a fixed point avoids depending on declaration order.
func collectGroupPrefixes(file *ast.File) map[string]string {
	type groupDecl struct{ name, parent, path string }
	var decls []groupDecl

	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		lhs, ok := assign.Lhs[0].(*ast.Ident)
		if !ok {
			return true
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		parent, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}

		switch selector.Sel.Name {
		case "Group":
			if len(call.Args) == 0 {
				return true
			}
			path, ok := stringLiteral(call.Args[0])
			if !ok {
				return true
			}
			decls = append(decls, groupDecl{name: lhs.Name, parent: parent.Name, path: path})
		case "Use":
			// `protected := api.Use(...)` does not add a prefix — it attaches
			// middleware and returns the same router. Verified empirically: routes
			// registered on either variable after this call share both the prefix
			// and the middleware.
			decls = append(decls, groupDecl{name: lhs.Name, parent: parent.Name, path: ""})
		}
		return true
	})

	prefixes := map[string]string{"app": ""}
	for range decls { // fixed point; bounded by the number of declarations
		progressed := false
		for _, d := range decls {
			if _, done := prefixes[d.name]; done {
				continue
			}
			parentPrefix, known := prefixes[d.parent]
			if !known {
				continue
			}
			prefixes[d.name] = joinPath(parentPrefix, d.path)
			progressed = true
		}
		if !progressed {
			break
		}
	}
	return prefixes
}

func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
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
