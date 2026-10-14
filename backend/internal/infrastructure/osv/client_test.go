// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package osv

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mapCache struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (c *mapCache) Get(_ context.Context, k string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[k]
	return v, ok
}
func (c *mapCache) Set(_ context.Context, k string, v []byte, _ time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[k] = v
}

// A recorded-shape OSV: openssl 3.0.2-0ubuntu1.10 has one Ubuntu advisory with
// a CVE alias, a CVSS vector and a fixed version; bash has none.
func fakeOSV(t *testing.T, batches *atomic.Int32, sent *[]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/querybatch":
			batches.Add(1)
			b, _ := io.ReadAll(r.Body)
			*sent = append(*sent, string(b))
			var req batchReq
			require.NoError(t, json.Unmarshal(b, &req))
			res := make([]map[string]any, len(req.Queries))
			for i, q := range req.Queries {
				res[i] = map[string]any{}
				if q.Package.Name == "openssl" {
					res[i]["vulns"] = []map[string]string{{"id": "UBUNTU-CVE-2024-0727"}}
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"results": res})
		case r.URL.Path == "/v1/vulns/UBUNTU-CVE-2024-0727":
			_, _ = w.Write([]byte(`{
			  "id": "UBUNTU-CVE-2024-0727",
			  "summary": "PKCS12 NULL dereference",
			  "aliases": ["CVE-2024-0727"],
			  "severity": [{"type": "CVSS_V3", "score": "CVSS:3.1/AV:N/AC:L/PR:N/UI:R/S:U/C:N/I:N/A:H"}],
			  "affected": [{
			    "package": {"ecosystem": "Ubuntu:22.04:LTS", "name": "openssl"},
			    "ranges": [{"type": "ECOSYSTEM", "events": [{"introduced": "0"}, {"fixed": "3.0.2-0ubuntu1.14"}]}],
			    "severity": [{"type": "Ubuntu", "score": "low"}]
			  }]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestLookup_Success(t *testing.T) {
	var batches atomic.Int32
	var sent []string
	srv := fakeOSV(t, &batches, &sent)
	defer srv.Close()
	c := NewClient(srv.URL, &mapCache{m: map[string][]byte{}})

	ssl := Query{Ecosystem: "Ubuntu:22.04:LTS", Name: "openssl", Version: "3.0.2-0ubuntu1.10"}
	bash := Query{Ecosystem: "Ubuntu:22.04:LTS", Name: "bash", Version: "5.1-6ubuntu1"}
	got, err := c.Lookup(context.Background(), []Query{ssl, bash})
	require.NoError(t, err)
	require.Len(t, got[ssl], 1)
	adv := got[ssl][0]
	assert.Equal(t, []string{"CVE-2024-0727"}, adv.Aliases)
	assert.Equal(t, "3.0.2-0ubuntu1.14", adv.Fixed)
	assert.Equal(t, "low", adv.Severity)
	assert.True(t, strings.HasPrefix(adv.CVSSVector, "CVSS:3.1/"))
	assert.Empty(t, got[bash])

	// Only package, version and ecosystem leave the server.
	assert.NotContains(t, sent[0], "host")

	// Cached: a second lookup sends no batch.
	_, err = c.Lookup(context.Background(), []Query{ssl, bash})
	require.NoError(t, err)
	assert.Equal(t, int32(1), batches.Load())
}

// A failure is an error, never an empty "nothing found".
func TestLookup_Unreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	got, err := NewClient(srv.URL, nil).Lookup(context.Background(),
		[]Query{{Ecosystem: "Debian:12", Name: "openssl", Version: "1"}})
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestInterval_SpacesCalls(t *testing.T) {
	l := NewInterval(20 * time.Millisecond)
	start := time.Now()
	for i := 0; i < 3; i++ {
		require.NoError(t, l.Wait(context.Background()))
	}
	assert.GreaterOrEqual(t, time.Since(start), 40*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	l2 := NewInterval(time.Hour)
	_ = l2.Wait(context.Background())
	assert.Error(t, l2.Wait(ctx), "a cancelled wait returns")
}
