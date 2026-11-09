// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// parseDir parses every non-test Go file of dir. Like
// pkg/vendorscore/imports_test.go, it fails on a missing or empty directory
// rather than letting a boundary test pass vacuously.
func parseDir(t *testing.T, dir string) map[string]*ast.File {
	t.Helper()
	info, err := os.Stat(dir)
	require.NoError(t, err, "%s must exist: this boundary test would otherwise pass vacuously", dir)
	require.True(t, info.IsDir())
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	out := map[string]*ast.File{}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		require.NoError(t, err)
		out[name] = f
	}
	require.NotEmpty(t, out, "no Go file found in %s", dir)
	return out
}

func importPath(spec *ast.ImportSpec) string { return strings.Trim(spec.Path.Value, `"`) }

// allowedImports is the allowlist of TestCanonical_ImportsOnlyStdlibAndUUID:
// the standard-library packages the canonical types need, and google/uuid.
// It names packages, not "the standard library", so that net/http and
// database/sql are refused without being named, as ADR 0008 D9 requires. A new
// entry is a reviewed decision: it must be pure (no I/O, no network, no
// database) and must not be anything under the module.
var allowedImports = map[string]bool{
	"bytes":                  true,
	"encoding/json":          true,
	"errors":                 true,
	"fmt":                    true,
	"io":                     true, // io.EOF only, to refuse trailing data after a JSON value
	"math":                   true,
	"net/url":                true, // parsing only, for D3's credential-shaped URL refusal
	"regexp":                 true,
	"sort":                   true,
	"strconv":                true,
	"strings":                true,
	"time":                   true,
	"unicode/utf8":           true,
	"github.com/google/uuid": true,
}

// TestCanonical_ImportsOnlyStdlibAndUUID is ADR 0008 D9 / #1240 criterion 9.
func TestCanonical_ImportsOnlyStdlibAndUUID(t *testing.T) {
	for _, refused := range []string{
		"gorm.io/gorm", "github.com/gofiber/fiber/v2", "net/http", "database/sql", "os",
		"github.com/opendefender/openrisk/internal/domain",
		"github.com/opendefender/openrisk/pkg/scoring",
	} {
		require.False(t, allowedImports[refused], "the allowlist must not admit %s", refused)
	}
	for file, f := range parseDir(t, ".") {
		for _, spec := range f.Imports {
			imp := importPath(spec)
			assert.True(t, allowedImports[imp],
				"%s imports %s: internal/domain/canonical is pure, stdlib allowlist + google/uuid only (ADR 0008 D9)", file, imp)
		}
	}
}

// entityTypes is every canonical entity of the package, by name, for the
// reflection half of TestCanonical_EveryEntityCarriesTheEnvelope. It must name
// exactly the exported struct types the AST half classifies as entities.
var entityTypes = map[string]reflect.Type{
	"Requirement":           reflect.TypeOf(canonical.Requirement{}),
	"Control":               reflect.TypeOf(canonical.Control{}),
	"ControlImplementation": reflect.TypeOf(canonical.ControlImplementation{}),
	"ControlTest":           reflect.TypeOf(canonical.ControlTest{}),
	"Evidence":              reflect.TypeOf(canonical.Evidence{}),
	"Risk":                  reflect.TypeOf(canonical.Risk{}),
	"Finding":               reflect.TypeOf(canonical.Finding{}),
}

// nonEntities are the exported struct types that are NOT canonical entities
// and so do not embed Envelope. Each is a value carried inside an entity or an
// edge, or a helper of the API. A new exported struct type must be added here
// with its reason, or embed Envelope; anything else fails the test.
var nonEntities = map[string]string{
	"Envelope":          "the envelope itself",
	"Edge":              "a relationship record with its own envelope (D5): no source_version, observed_at or relationships",
	"EdgeTarget":        "the `to` end of an edge",
	"EdgeAttributes":    "an edge's per-verb attributes",
	"EdgeKey":           "an edge's identity tuple",
	"Triple":            "a row of the D5 vocabulary",
	"RecordRef":         "a `{type, id}` reference",
	"Provenance":        "an envelope field (D3)",
	"Actor":             "provenance.actor and ControlTestResult.executed_by",
	"ExternalRef":       "provenance.external_refs item",
	"ReviewContext":     "the input of the user_entered uplift (D3)",
	"Ownership":         "the Ownership block embedded in entities",
	"Framework":         "Requirement.framework; a grouping, not an entity (D6)",
	"ControlTestResult": "a sub-record of ControlTest, not an entity (D1); reached through its test's tenant",
	"EvidenceReview":    "Evidence.review",
	"RiskScore":         "Risk.score",
	"Residual":          "Risk.residual",
	"SmartScore":        "Risk.smart_score",
	"SmartFactor":       "Risk.smart_score.factors item",
	"Acceptance":        "Risk.acceptance",
	"ReviewCadence":     "Risk.review_cadence",
	"Category":          "Risk.category; a tenant vocabulary, not an entity (D6)",
	"Quantification":    "Risk.quantification",
	"FindingIdentifier": "Finding.identifiers item",
	"CVSS":              "Finding.cvss",
	"Exploitability":    "Finding.exploitability",
	"FindingPriority":   "Finding.priority",
}

// TestCanonical_EveryEntityCarriesTheEnvelope is ADR 0008 D9 / #1240
// criterion 2. The AST half enumerates every exported struct type of the
// package, so a twentieth entity without an Envelope fails here even if nobody
// remembers to list it; the reflection half checks the embedding on the types
// themselves.
func TestCanonical_EveryEntityCarriesTheEnvelope(t *testing.T) {
	declared := map[string]*ast.StructType{}
	for _, f := range parseDir(t, ".") {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts := spec.(*ast.TypeSpec)
				if st, ok := ts.Type.(*ast.StructType); ok && ts.Name.IsExported() {
					declared[ts.Name.Name] = st
				}
			}
		}
	}
	for name := range nonEntities {
		require.Contains(t, declared, name, "nonEntities lists %s, which the package no longer declares", name)
	}

	var entities []string
	for name, st := range declared {
		if _, skip := nonEntities[name]; skip {
			continue
		}
		entities = append(entities, name)
		embeds := false
		for _, field := range st.Fields.List {
			if id, ok := field.Type.(*ast.Ident); ok && len(field.Names) == 0 && id.Name == "Envelope" {
				embeds = true
			}
		}
		assert.True(t, embeds, "%s is an exported struct type that does not embed Envelope: embed it, or list it in nonEntities with the reason", name)
	}
	sort.Strings(entities)
	assert.Equal(t, []string{"Control", "ControlImplementation", "ControlTest", "Evidence", "Finding", "Requirement", "Risk"},
		entities, "the v1 entities are exactly ADR 0008 D6's seven")

	envelope := reflect.TypeOf(canonical.Envelope{})
	for _, name := range entities {
		typ, ok := entityTypes[name]
		require.True(t, ok, "%s is an entity but is missing from entityTypes", name)
		field, ok := typ.FieldByName("Envelope")
		require.True(t, ok, "%s has no Envelope", name)
		assert.True(t, field.Anonymous, "%s must embed Envelope, not name it", name)
		assert.Equal(t, envelope, field.Type, name)
		assert.Equal(t, []int{0}, field.Index, "%s embeds Envelope at depth 1", name)
	}
	assert.Len(t, entityTypes, len(entities))
}

// TestCanonical_RiskScoreIsReadOnly is ADR 0008 D6, D9 / #1240 criterion 10:
// the canonical Risk carries the frozen score and never produces or accepts
// one. The same holds for Finding.priority (D6: "read-only like Risk.score").
func TestCanonical_RiskScoreIsReadOnly(t *testing.T) {
	files := parseDir(t, ".")

	t.Run("no import of a score engine", func(t *testing.T) {
		for file, f := range files {
			for _, spec := range f.Imports {
				imp := importPath(spec)
				assert.False(t, strings.Contains(imp, "pkg/scoring") || strings.Contains(imp, "domain/scoring") || strings.Contains(imp, "pkg/vulnprio"),
					"%s imports %s", file, imp)
			}
		}
	})

	t.Run("no exported score field", func(t *testing.T) {
		for typ, fields := range map[reflect.Type][]string{
			reflect.TypeOf(canonical.Risk{}):    {"Score", "SmartScore"},
			reflect.TypeOf(canonical.Finding{}): {"Priority"},
		} {
			for _, name := range fields {
				if f, ok := typ.FieldByName(name); ok {
					assert.False(t, f.IsExported(), "%s.%s is an exported, writable field", typ.Name(), name)
				}
			}
		}
	})

	setter := regexp.MustCompile(`(?i)^set.*(score|priority)`)
	t.Run("no setter method", func(t *testing.T) {
		for _, typ := range []reflect.Type{
			reflect.TypeOf(canonical.Risk{}), reflect.TypeOf(&canonical.Risk{}),
			reflect.TypeOf(canonical.Finding{}), reflect.TypeOf(&canonical.Finding{}),
		} {
			for i := 0; i < typ.NumMethod(); i++ {
				assert.False(t, setter.MatchString(typ.Method(i).Name), "%s has setter %s", typ, typ.Method(i).Name)
			}
		}
		for file, f := range files {
			for _, decl := range f.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok {
					assert.False(t, setter.MatchString(fn.Name.Name), "%s declares %s", file, fn.Name.Name)
				}
			}
		}
	})

	t.Run("written in exactly one function each", func(t *testing.T) {
		// The read-only fields may be assigned, or set in a composite literal,
		// only by the projection entry point that copies what the engine
		// computed. Any other writer, a computation included, fails here.
		owner := map[string]string{"score": "WithEngineScores", "smartScore": "WithEngineScores", "priority": "WithVulnPriority"}
		for file, f := range files {
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					var written []string
					switch x := n.(type) {
					case *ast.AssignStmt:
						for _, lhs := range x.Lhs {
							if sel, ok := lhs.(*ast.SelectorExpr); ok {
								written = append(written, sel.Sel.Name)
							}
						}
					case *ast.KeyValueExpr:
						if id, ok := x.Key.(*ast.Ident); ok {
							written = append(written, id.Name)
						}
					}
					for _, name := range written {
						if want, guarded := owner[name]; guarded {
							assert.Equal(t, want, fn.Name.Name, "%s: %s writes %s", file, fn.Name.Name, name)
						}
					}
					return true
				})
			}
		}
	})

	t.Run("a decoded Risk never carries a score from its input", func(t *testing.T) {
		raw := `{"name":"x","score":{"value":30,"criticality":"critical"},"smart_score":{"value":99,"level":"critical"}}`
		var r canonical.Risk
		require.NoError(t, json.Unmarshal([]byte(raw), &r))
		_, has := r.Score()
		assert.False(t, has)
		_, has = r.SmartScore()
		assert.False(t, has)
		var f canonical.Finding
		require.NoError(t, json.Unmarshal([]byte(`{"priority":{"score":99,"tier":"P1"}}`), &f))
		_, has = f.Priority()
		assert.False(t, has)
	})

	t.Run("a projected score is published", func(t *testing.T) {
		r := canonical.WithEngineScores(validRisk(), canonical.RiskScore{Value: 3.2, Criticality: canonical.CriticalityMedium}, nil)
		raw, err := json.Marshal(r)
		require.NoError(t, err)
		assert.Contains(t, string(raw), `"score":{"value":3.2,"criticality":"medium"}`)
		assert.Contains(t, string(raw), `"smart_score":null`)
		got, ok := r.Score()
		require.True(t, ok)
		assert.Equal(t, 3.2, got.Value)
	})
}

// TestScoring_DoesNotReferenceConfidence is ADR 0008 D3, D9 / #1240
// criterion 10: confidence is never an input to Risk.Score or SmartScore.
// Comments are not identifiers; internal/domain/scoring/model.go's prose use of
// "confidence" in a comment is not a reference.
func TestScoring_DoesNotReferenceConfidence(t *testing.T) {
	for _, dir := range []string{
		filepath.Join("..", "..", "..", "pkg", "scoring"),
		filepath.Join("..", "scoring"),
	} {
		for file, f := range parseDir(t, dir) {
			for _, spec := range f.Imports {
				imp := importPath(spec)
				assert.NotContains(t, imp, "internal/domain/canonical",
					"%s/%s imports the canonical model: confidence must never reach the score (ADR 0008 D3)", dir, file)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					assert.NotContains(t, strings.ToLower(id.Name), "confidence",
						"%s/%s names %s: confidence must never reach the score (ADR 0008 D3)", dir, file, id.Name)
				}
				return true
			})
		}
	}
}
