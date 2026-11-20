// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// #1472 criterion 4: the compressed-body guard is global, so a new route is
// covered without opting in. main() needs a database to run, so this pins the
// wiring in the source: the guard is mounted on the root app, after fiber.New
// and before the first route, the first group and the CORS middleware.
func TestRejectCompressedRequestBody_MountedGloballyBeforeRoutes(t *testing.T) {
	src, err := os.ReadFile("main.go")
	require.NoError(t, err)
	code := string(src)

	guard := strings.Index(code, "app.Use(middleware.RejectCompressedRequestBody())")
	require.NotEqual(t, -1, guard, "guard not mounted on the root app")
	require.Equal(t, 1, strings.Count(code, "middleware.RejectCompressedRequestBody()"), "guard mounted more than once")

	newApp := strings.Index(code, "app := fiber.New(")
	require.NotEqual(t, -1, newApp)
	require.Less(t, newApp, guard, "guard must come after fiber.New")

	for _, later := range []string{"app.Use(cors.New(", "app.Get(", "app.Post(", "app.Group("} {
		at := strings.Index(code, later)
		require.NotEqual(t, -1, at, "%s not found in main.go", later)
		require.Less(t, guard, at, "guard must be mounted before %s", later)
	}
}
