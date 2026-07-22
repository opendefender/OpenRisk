// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package scanner

import (
	"github.com/opendefender/openrisk/internal/domain"
)

// Registry resolves a Scanner by provider. It is populated once at wiring time
// and read-only thereafter, so it needs no locking.
type Registry struct {
	scanners map[domain.ScannerProvider]Scanner
}

func NewRegistry() *Registry {
	return &Registry{scanners: make(map[domain.ScannerProvider]Scanner)}
}

// Register adds a scanner under its provider key. A second registration for the
// same provider overwrites the first (last wins) — wiring registers each once.
func (r *Registry) Register(s Scanner) {
	r.scanners[domain.ScannerProvider(s.Provider())] = s
}

// Get returns the scanner for a provider, or (nil, false) if none is registered.
func (r *Registry) Get(p domain.ScannerProvider) (Scanner, bool) {
	s, ok := r.scanners[p]
	return s, ok
}
