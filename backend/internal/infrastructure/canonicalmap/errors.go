// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package canonicalmap is the infrastructure side of the canonical model
// (ADR 0008 D9): the layer that maps persisted GORM models to and from
// internal/domain/canonical. It may import the canonical package; the canonical
// package never imports it. The projections land with #553; today it holds the
// typed-error boundary they will return through.
package canonicalmap

import (
	"errors"
	"fmt"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// WrapValidation maps a canonical validation error onto domain.ErrValidation,
// so the existing HTTP error mapping answers it as any other validation error.
// The result satisfies errors.Is for both sentinels. Any other error, and nil,
// is returned unchanged: only a validation refusal becomes a 400.
func WrapValidation(err error) error {
	if err == nil || !errors.Is(err, canonical.ErrValidation) {
		return err
	}
	if errors.Is(err, domain.ErrValidation) {
		return err
	}
	return fmt.Errorf("%w: %w", domain.ErrValidation, err)
}
