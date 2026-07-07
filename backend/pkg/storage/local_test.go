// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package storage

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newTestStorage(t *testing.T) *LocalStorage {
	t.Helper()
	s, err := NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	return s
}

func TestSaveOpenDelete_RoundTrip(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	tenantID := uuid.New()

	key, err := s.Save(ctx, tenantID, "evidence.pdf", strings.NewReader("hello world"))
	require.NoError(t, err)
	require.NotEmpty(t, key)

	rc, err := s.Open(ctx, key)
	require.NoError(t, err)
	defer rc.Close()

	content, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, "hello world", string(content))

	require.NoError(t, s.Delete(ctx, key))

	_, err = s.Open(ctx, key)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestSave_NamespacesByTenant(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	tenantA := uuid.New()
	tenantB := uuid.New()

	keyA, err := s.Save(ctx, tenantA, "same-name.txt", strings.NewReader("from tenant A"))
	require.NoError(t, err)
	keyB, err := s.Save(ctx, tenantB, "same-name.txt", strings.NewReader("from tenant B"))
	require.NoError(t, err)

	require.NotEqual(t, keyA, keyB)
	require.True(t, strings.HasPrefix(keyA, tenantA.String()+"/"))
	require.True(t, strings.HasPrefix(keyB, tenantB.String()+"/"))

	rcA, err := s.Open(ctx, keyA)
	require.NoError(t, err)
	defer rcA.Close()
	contentA, _ := io.ReadAll(rcA)
	require.Equal(t, "from tenant A", string(contentA))
}

func TestOpen_NotFound(t *testing.T) {
	s := newTestStorage(t)
	_, err := s.Open(context.Background(), uuid.New().String()+"/nonexistent-file.txt")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestDelete_NonExistentKeyIsIdempotent(t *testing.T) {
	s := newTestStorage(t)
	err := s.Delete(context.Background(), uuid.New().String()+"/nonexistent-file.txt")
	require.NoError(t, err)
}

func TestSave_SanitizesFilename(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	tenantID := uuid.New()

	// Attempt path traversal via the filename — must never escape BasePath.
	key, err := s.Save(ctx, tenantID, "../../../etc/passwd", strings.NewReader("malicious"))
	require.NoError(t, err)
	require.False(t, strings.Contains(key, ".."))
	require.True(t, strings.HasPrefix(key, tenantID.String()+"/"))

	// The file must be readable back from inside BasePath (i.e. it was
	// actually written under the tenant's namespace, not outside it).
	rc, err := s.Open(ctx, key)
	require.NoError(t, err)
	defer rc.Close()
	content, _ := io.ReadAll(rc)
	require.Equal(t, "malicious", string(content))
}

func TestOpen_RejectsKeyEscapingBasePath(t *testing.T) {
	s := newTestStorage(t)
	_, err := s.Open(context.Background(), "../../../../etc/passwd")
	require.True(t, errors.Is(err, ErrNotFound))
}
