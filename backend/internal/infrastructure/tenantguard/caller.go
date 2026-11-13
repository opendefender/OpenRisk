// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tenantguard

import (
	"runtime"
	"strconv"
	"strings"
)

// ownPrefix is this package's function-name prefix. The trailing dot keeps the
// external test package (tenantguard_test) out of it.
const ownPrefix = "github.com/opendefender/openrisk/internal/infrastructure/tenantguard."

func skippedFrame(fn string) bool {
	return strings.HasPrefix(fn, "gorm.io/") ||
		strings.HasPrefix(fn, "database/sql") ||
		strings.HasPrefix(fn, "runtime.") ||
		strings.HasPrefix(fn, ownPrefix)
}

// callerFrame returns "file:line" of the first frame outside GORM, database/sql,
// the runtime and this package, and whether that file is a *_test.go. It is
// computed only on a violation. Matching is by function package path, not file
// path, so a GORM served from a replace directive is still skipped.
func callerFrame() (string, bool) {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		f, more := frames.Next()
		if f.Function != "" && !skippedFrame(f.Function) {
			return f.File + ":" + strconv.Itoa(f.Line), strings.HasSuffix(f.File, "_test.go")
		}
		if !more {
			return "unknown", false
		}
	}
}
