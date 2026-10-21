// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package keycheck notices replicas that were given different secret keys
// (#1063).
//
// Two replicas with different AUDIT_HASH_KEY values split the per-address
// limits and the audit trail; with different MFA_ENCRYPTION_KEY values, a
// second factor enrolled through one cannot be read by the other. Nothing
// failed loudly. Each replica now publishes a fingerprint of each key to Redis
// and reads the others'. A fingerprint reveals nothing of the key
// (domain.KeyFingerprint).
//
// A mismatch is logged and exported, never fatal: rotating a key across a
// rolling upgrade is a brief, legitimate mismatch.
package keycheck

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// Interval is how often a replica publishes and compares.
	Interval = time.Minute
	// staleAfter drops replicas that stopped publishing (scaled down, crashed).
	staleAfter = 3 * Interval
	keyPrefix  = "keyfp:"
)

// Store is the slice of Redis the check needs. Satisfied by *redis.Client.
type Store interface {
	HSet(ctx context.Context, key, field, value string) error
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
}

// Result is what one check found for one key.
type Result struct {
	Key          string
	Mismatch     bool
	Fingerprints []string // distinct fingerprints seen, sorted
}

// Checker publishes this replica's fingerprints and compares them with the
// other live replicas'.
type Checker struct {
	store    Store
	instance string
	keys     map[string]string // key name -> fingerprint
	now      func() time.Time
}

// New builds a checker. keys maps a key's NAME (never its value) to its
// fingerprint.
func New(store Store, instance string, keys map[string]string) *Checker {
	return &Checker{store: store, instance: instance, keys: keys, now: time.Now}
}

// Check publishes, then reads, every key.
func (c *Checker) Check(ctx context.Context) ([]Result, error) {
	names := make([]string, 0, len(c.keys))
	for name := range c.keys {
		names = append(names, name)
	}
	sort.Strings(names)

	now := c.now()
	results := make([]Result, 0, len(names))
	for _, name := range names {
		hash := keyPrefix + name
		entry := c.keys[name] + "|" + strconv.FormatInt(now.Unix(), 10)
		if err := c.store.HSet(ctx, hash, c.instance, entry); err != nil {
			return results, fmt.Errorf("publish %s fingerprint: %w", name, err)
		}
		_ = c.store.Expire(ctx, hash, staleAfter)

		all, err := c.store.HGetAll(ctx, hash)
		if err != nil {
			return results, fmt.Errorf("read %s fingerprints: %w", name, err)
		}
		seen := map[string]bool{}
		for _, v := range all {
			fp, at, ok := strings.Cut(v, "|")
			if !ok {
				continue
			}
			sec, err := strconv.ParseInt(at, 10, 64)
			if err != nil || now.Sub(time.Unix(sec, 0)) > staleAfter {
				continue
			}
			seen[fp] = true
		}
		distinct := make([]string, 0, len(seen))
		for fp := range seen {
			distinct = append(distinct, fp)
		}
		sort.Strings(distinct)
		results = append(results, Result{Key: name, Mismatch: len(distinct) > 1, Fingerprints: distinct})
	}
	return results, nil
}
