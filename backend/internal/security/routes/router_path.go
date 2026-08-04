// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package routes

import (
	"os"
	"path/filepath"
	"testing"
)

// RealRouterFile is the router the isolation suite derives its surface from.
const RealRouterFile = "cmd/server/main.go"

// realRouterPath locates the application router from a test's working
// directory, which Go sets to the package under test.
//
// It walks up to the module root rather than hardcoding "../../../" so the
// helper keeps working if these packages are ever moved.
func realRouterPath(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			path := filepath.Join(dir, RealRouterFile)
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("router not found at %s: %v", path, err)
			}
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("module root (go.mod) not found above the test directory")
		}
		dir = parent
	}
}
