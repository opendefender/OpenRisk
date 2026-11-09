// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package canonical holds the OpenRisk Canonical Model v1 (ADR 0008): the
// envelope every record carries, the source registry and its confidence rule,
// provenance, the closed relationship vocabulary and the seven entities of the
// GRC slice, with their validation.
//
// The package is pure. Its non-test files import only an explicit allowlist of
// standard-library packages and github.com/google/uuid
// (TestCanonical_ImportsOnlyStdlibAndUUID). It has no repository and no
// persistence: mapping from GORM models lives in internal/infrastructure,
// which may import this package, never the reverse (ADR 0008 D9).
package canonical

import (
	"errors"
	"fmt"
)

// ErrValidation is the only error this package raises. A pure package cannot
// import internal/domain (it would bring GORM in transitively), so it declares
// its own sentinel; the infrastructure mapping layer wraps it into
// domain.ErrValidation so HTTP error handling is unchanged (ADR 0008 D9).
var ErrValidation = errors.New("canonical: validation")

// invalid builds a validation error that satisfies errors.Is(err, ErrValidation).
// Messages name the field and the rule, never the offending value: a refused
// value may be a credential (ABSOLUTE RULE 6).
func invalid(field, format string, args ...any) error {
	return fmt.Errorf("%w: %s: %s", ErrValidation, field, fmt.Sprintf(format, args...))
}

// within prefixes a nested validation error with the path of its parent.
func within(field string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", field, err)
}
