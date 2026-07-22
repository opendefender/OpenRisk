// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// unsafeFilenameChars matches anything outside a conservative safe set,
// so a sanitized filename can never be used to escape BasePath (no "..",
// no path separators, no NUL, no control characters).
var unsafeFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// LocalStorage stores files on the local filesystem under BasePath,
// namespaced by tenant. It satisfies Storage and is meant as the default,
// zero-dependency driver — a future S3Storage (or similar) implementing
// the same interface can be swapped in via a STORAGE_DRIVER config switch
// without any caller (use cases, handlers) changing.
type LocalStorage struct {
	BasePath string
}

// NewLocalStorage creates a LocalStorage rooted at basePath, creating the
// directory if it does not already exist.
func NewLocalStorage(basePath string) (*LocalStorage, error) {
	if err := os.MkdirAll(basePath, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create storage base path: %w", err)
	}
	return &LocalStorage{BasePath: basePath}, nil
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name) // strip any directory components
	name = unsafeFilenameChars.ReplaceAllString(name, "_")
	name = strings.TrimLeft(name, "._") // avoid ".", "..", or hidden-file-looking names
	if name == "" {
		name = "file"
	}
	// Cap length so an absurdly long filename can't create pathological paths.
	if len(name) > 100 {
		name = name[:100]
	}
	return name
}

func (s *LocalStorage) Save(_ context.Context, tenantID uuid.UUID, filename string, content io.Reader) (string, error) {
	safeName := sanitizeFilename(filename)
	key := filepath.Join(tenantID.String(), fmt.Sprintf("%s-%s", uuid.New().String(), safeName))

	fullPath := filepath.Join(s.BasePath, key)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return "", fmt.Errorf("failed to create tenant storage dir: %w", err)
	}

	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o640)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, content); err != nil {
		_ = os.Remove(fullPath)
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return key, nil
}

// resolveKey joins key under BasePath and rejects any attempt to escape it
// (defense in depth: Open/Delete should only ever be called with a key
// previously returned by Save, but never trust that blindly).
func (s *LocalStorage) resolveKey(key string) (string, error) {
	full := filepath.Join(s.BasePath, key)
	base, err := filepath.Abs(s.BasePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base path: %w", err)
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", fmt.Errorf("failed to resolve key path: %w", err)
	}
	if absFull != base && !strings.HasPrefix(absFull, base+string(filepath.Separator)) {
		return "", fmt.Errorf("key escapes storage base path")
	}
	return absFull, nil
}

func (s *LocalStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	fullPath, err := s.resolveKey(key)
	if err != nil {
		return nil, ErrNotFound
	}
	f, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	return f, nil
}

func (s *LocalStorage) Delete(_ context.Context, key string) error {
	fullPath, err := s.resolveKey(key)
	if err != nil {
		return nil // an invalid/escaping key can't refer to a real stored file
	}
	if rmErr := os.Remove(fullPath); rmErr != nil && !os.IsNotExist(rmErr) {
		return fmt.Errorf("failed to delete file: %w", rmErr)
	}
	return nil
}
