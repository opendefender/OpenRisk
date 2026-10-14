// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package vulnlookup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/application/inventory"
	"github.com/opendefender/openrisk/internal/infrastructure/osv"
)

func TestOSVAdapter_MapsCVEAndScore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/querybatch":
			_, _ = w.Write([]byte(`{"results":[{"vulns":[{"id":"DEBIAN-CVE-2024-0727"}]}]}`))
		case "/v1/vulns/DEBIAN-CVE-2024-0727":
			_, _ = w.Write([]byte(`{"id":"DEBIAN-CVE-2024-0727","aliases":["CVE-2024-0727"],
			  "severity":[{"type":"CVSS_V3","score":"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"}],
			  "affected":[{"package":{"ecosystem":"Debian:12","name":"openssl"},
			    "ranges":[{"type":"ECOSYSTEM","events":[{"introduced":"0"},{"fixed":"3.0.13-1~deb12u1"}]}]}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	q := inventory.PackageQuery{Ecosystem: "Debian:12", Name: "openssl", Version: "3.0.11-1~deb12u2"}
	got, err := NewOSV(osv.NewClient(srv.URL, nil)).Lookup(context.Background(), []inventory.PackageQuery{q})
	require.NoError(t, err)
	require.Len(t, got[q], 1)
	a := got[q][0]
	assert.Equal(t, "CVE-2024-0727", a.CVE)
	assert.Equal(t, 9.8, a.CVSS)
	assert.Equal(t, "3.0.13-1~deb12u1", a.Fixed)
}

// fakeKV answers a missing key the way the project's Redis client does: "" and no error.
type fakeKV map[string]string

func (f fakeKV) Get(_ context.Context, k string) (string, error) { return f[k], nil }
func (f fakeKV) Set(_ context.Context, k, v string, _ time.Duration) error {
	f[k] = v
	return nil
}

func TestRedisCache_MissIsAMiss(t *testing.T) {
	c := NewRedisCache(fakeKV{})
	_, ok := c.Get(context.Background(), "osv:v:X")
	assert.False(t, ok, "a missing key must not read as a cached empty answer")
	c.Set(context.Background(), "osv:v:X", []byte(`{"id":"X"}`), time.Hour)
	b, ok := c.Get(context.Background(), "osv:v:X")
	assert.True(t, ok)
	assert.Equal(t, `{"id":"X"}`, string(b))
}

// No Redis: nothing is cached and nothing fails.
func TestRedisCache_Nil(t *testing.T) {
	var c *RedisCache
	_, ok := c.Get(context.Background(), "k")
	assert.False(t, ok)
	c.Set(context.Background(), "k", []byte("v"), 0)
}
