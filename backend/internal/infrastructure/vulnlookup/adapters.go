// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package vulnlookup adapts the OSV and NVD clients to the inventory matcher's
// ports (#910), with a Redis cache shared across tenants: what it stores is
// public advisory data keyed only on package or program, version and
// ecosystem, never on a tenant or a host.
package vulnlookup

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/opendefender/openrisk/internal/application/inventory"
	"github.com/opendefender/openrisk/internal/infrastructure/osv"
	"github.com/opendefender/openrisk/internal/vulnscan"
	"github.com/opendefender/openrisk/pkg/cti"
)

// KV is the part of the Redis client the cache needs.
type KV interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

// RedisCache implements osv.Cache. A nil cache or client caches nothing.
type RedisCache struct{ c KV }

// NewRedisCache builds the cache.
func NewRedisCache(c KV) *RedisCache { return &RedisCache{c: c} }

// Get reads a cached answer.
func (r *RedisCache) Get(ctx context.Context, k string) ([]byte, bool) {
	if r == nil || r.c == nil {
		return nil, false
	}
	// The project's client answers a missing key with "" and no error, so an
	// empty value is a miss; nothing is ever stored empty (see unresolvedCPE).
	s, err := r.c.Get(ctx, "vulnlookup:"+k)
	if err != nil || s == "" {
		return nil, false
	}
	return []byte(s), true
}

// Set stores an answer; a failure only costs a later lookup.
func (r *RedisCache) Set(ctx context.Context, k string, v []byte, ttl time.Duration) {
	if r == nil || r.c == nil {
		return
	}
	_ = r.c.Set(ctx, "vulnlookup:"+k, string(v), ttl)
}

// OSV adapts osv.Client to inventory.PackageLookup.
type OSV struct{ client *osv.Client }

// NewOSV builds the adapter.
func NewOSV(c *osv.Client) *OSV { return &OSV{client: c} }

// Lookup asks OSV and maps its advisories: the CVE among the id and aliases,
// and a CVSS base score computed from the vector when there is one.
func (o *OSV) Lookup(ctx context.Context, qs []inventory.PackageQuery) (map[inventory.PackageQuery][]inventory.Advisory, error) {
	in := make([]osv.Query, len(qs))
	for i, q := range qs {
		in[i] = osv.Query{Ecosystem: q.Ecosystem, Name: q.Name, Version: q.Version}
	}
	got, err := o.client.Lookup(ctx, in)
	if err != nil {
		return nil, err
	}
	out := make(map[inventory.PackageQuery][]inventory.Advisory, len(got))
	for q, advs := range got {
		key := inventory.PackageQuery{Ecosystem: q.Ecosystem, Name: q.Name, Version: q.Version}
		for _, a := range advs {
			score, _ := vulnscan.CVSS3BaseScore(a.CVSSVector)
			out[key] = append(out[key], inventory.Advisory{
				ID: a.ID, CVE: inventory.FirstCVE(a.ID, a.Aliases), Summary: a.Summary,
				Details: a.Details, CVSS: score, Severity: a.Severity, Fixed: a.Fixed,
			})
		}
	}
	return out, nil
}

// unresolvedCPE is cached for a program NVD has no strict match for, so the
// answer "not checkable" is remembered without storing an empty value.
const unresolvedCPE = "-"

// nvdCacheTTL: a product's NVD identity and its CVE list move slowly; a week
// keeps a Windows fleet from re-spending NVD's small request budget daily.
const nvdCacheTTL = 7 * 24 * time.Hour

// NVDPrograms adapts the NVD client to inventory.ProgramLookup.
type NVDPrograms struct {
	client *cti.ExternalClient
	cache  osv.Cache
	limit  *osv.Interval
}

// NewNVDPrograms builds the adapter. withKey: an NVD_API_KEY is configured,
// which raises NVD's limit from 5 to 50 requests per 30 s.
func NewNVDPrograms(c *cti.ExternalClient, cache osv.Cache, withKey bool) *NVDPrograms {
	gap := 6 * time.Second
	if withKey {
		gap = 600 * time.Millisecond
	}
	return &NVDPrograms{client: c, cache: cache, limit: osv.NewInterval(gap)}
}

// Lookup resolves the program to an NVD product, then asks for the CVEs of
// that exact version. resolved is false when no product matches strictly.
func (n *NVDPrograms) Lookup(ctx context.Context, publisher, name, version string) ([]inventory.Advisory, bool, error) {
	cpeKey := "nvd:cpe:" + strings.ToLower(publisher) + "\x00" + strings.ToLower(cti.ProgramKeyword(name))
	vp, cached := n.cacheString(ctx, cpeKey)
	if !cached {
		if err := n.limit.Wait(ctx); err != nil {
			return nil, false, err
		}
		got, err := n.client.ResolveCPE(ctx, publisher, name)
		if err != nil {
			return nil, false, err
		}
		vp = got
		if vp == "" {
			n.setCache(ctx, cpeKey, []byte(unresolvedCPE))
		} else {
			n.setCache(ctx, cpeKey, []byte(vp))
		}
	}
	if vp == "" || vp == unresolvedCPE {
		return nil, false, nil
	}

	cveKey := "nvd:cve:" + vp + "\x00" + version
	if b, ok := n.get(ctx, cveKey); ok {
		var advs []inventory.Advisory
		if json.Unmarshal(b, &advs) == nil {
			return advs, true, nil
		}
	}
	if err := n.limit.Wait(ctx); err != nil {
		return nil, true, err
	}
	vulns, err := n.client.FetchNVDByCPE(ctx, vp, version)
	if err != nil {
		return nil, true, err
	}
	advs := make([]inventory.Advisory, 0, len(vulns))
	for _, v := range vulns {
		advs = append(advs, inventory.Advisory{
			ID: v.CVEID, CVE: inventory.FirstCVE(v.CVEID, nil), Details: v.Description,
			CVSS: v.CVSSV3, Severity: strings.ToLower(v.Severity),
		})
	}
	if b, err := json.Marshal(advs); err == nil {
		n.setCache(ctx, cveKey, b)
	}
	return advs, true, nil
}

func (n *NVDPrograms) get(ctx context.Context, k string) ([]byte, bool) {
	if n.cache == nil {
		return nil, false
	}
	return n.cache.Get(ctx, k)
}

func (n *NVDPrograms) cacheString(ctx context.Context, k string) (string, bool) {
	b, ok := n.get(ctx, k)
	return string(b), ok
}

func (n *NVDPrograms) setCache(ctx context.Context, k string, v []byte) {
	if n.cache != nil {
		n.cache.Set(ctx, k, v, nvdCacheTTL)
	}
}
