// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package vendorscore

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// importsOf returns every import path of the non-test Go files in dir.
func importsOf(t *testing.T, dir string) map[string][]string {
	t.Helper()
	info, err := os.Stat(dir)
	require.NoError(t, err, "%s must exist: this boundary test would otherwise pass vacuously", dir)
	require.True(t, info.IsDir())

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	out := map[string][]string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imp := range f.Imports {
			out[name] = append(out[name], strings.Trim(imp.Path.Value, `"`))
		}
	}
	require.NotEmpty(t, out, "no Go file found in %s", dir)
	return out
}

// TestBoundary_VendorScoreImportsNothingFromTheProduct pins purity: no database,
// no HTTP, nothing under internal/, and not the risk score engine.
func TestBoundary_VendorScoreImportsNothingFromTheProduct(t *testing.T) {
	for file, imports := range importsOf(t, ".") {
		for _, imp := range imports {
			for _, forbidden := range []string{"openrisk/internal", "openrisk/pkg/scoring", "gorm.io", "gofiber", "net/http", "database/sql", "time"} {
				require.False(t, imp == forbidden || strings.Contains(imp, forbidden),
					"%s imports %s: pkg/vendorscore must stay pure (ADR 0004 D5)", file, imp)
			}
		}
	}
}

// TestBoundary_NothingInTheRiskScoringPathImportsVendorScore pins D-044: the
// vendor score is never an input to Risk.Score, SmartScore or residual risk.
func TestBoundary_NothingInTheRiskScoringPathImportsVendorScore(t *testing.T) {
	riskScoringPath := []string{
		filepath.Join("..", "scoring"),
		filepath.Join("..", "..", "internal", "domain", "scoring"),
		filepath.Join("..", "..", "internal", "application", "risk"),
	}
	for _, dir := range riskScoringPath {
		for file, imports := range importsOf(t, dir) {
			for _, imp := range imports {
				require.NotContains(t, imp, "vendorscore",
					"%s/%s imports the vendor score: it must never feed the risk score (D-044, ADR 0004 D5)", dir, file)
			}
		}
	}
}
