// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

// Package storage provides a driver-agnostic blob store for user-uploaded
// files (e.g. compliance evidence). Callers are responsible for tenant
// authorization before calling Open/Delete — a Storage implementation only
// guarantees it will return exactly what was Saved under a given key, it is
// not itself an authorization boundary.
package storage

import (
	"context"
	"errors"
	"io"

	"github.com/google/uuid"
)

// ErrNotFound is returned by Open/Delete when the key does not exist.
var ErrNotFound = errors.New("storage: key not found")

// Storage is the port for persisting uploaded file content.
// Save/Open/Delete operate on an opaque key — implementations decide their
// own internal layout (local filesystem path, S3 object key, etc.).
type Storage interface {
	// Save persists content under a new key namespaced by tenantID, and
	// returns that key. filename is used only to keep a human-readable
	// suffix; it is sanitized and must not be trusted as a path.
	Save(ctx context.Context, tenantID uuid.UUID, filename string, content io.Reader) (key string, err error)

	// Open returns a reader for the content stored under key.
	// Returns ErrNotFound if the key does not exist.
	Open(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes the content stored under key. Deleting a key that
	// does not exist is not an error (idempotent).
	Delete(ctx context.Context, key string) error
}
