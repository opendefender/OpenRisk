// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonicalmap

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// TestWrapValidation_Success: a real canonical refusal reaches the HTTP layer
// as domain.ErrValidation, and keeps its canonical identity and message.
func TestWrapValidation_Success(t *testing.T) {
	refusal := canonical.Requirement{}.Validate()
	require.True(t, errors.Is(refusal, canonical.ErrValidation))

	wrapped := WrapValidation(refusal)
	assert.True(t, errors.Is(wrapped, domain.ErrValidation))
	assert.True(t, errors.Is(wrapped, canonical.ErrValidation))
	assert.Contains(t, wrapped.Error(), "tenant_id")
	assert.Equal(t, wrapped, WrapValidation(wrapped), "wrapping twice adds nothing")
}

// TestWrapValidation_LeavesOtherErrorsAlone: nil stays nil, and an error that
// is not a canonical refusal is never turned into a 400.
func TestWrapValidation_LeavesOtherErrorsAlone(t *testing.T) {
	assert.NoError(t, WrapValidation(nil))
	other := errors.New("connection reset")
	assert.Same(t, other, WrapValidation(other))
	assert.False(t, errors.Is(WrapValidation(other), domain.ErrValidation))
	assert.False(t, errors.Is(WrapValidation(domain.ErrNotFound), domain.ErrValidation))
}
