// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package osv queries OSV (https://osv.dev) for the advisories affecting an
// exact package version (#910). Only the package name, version and ecosystem
// are sent: never a hostname, an IP or a tenant identifier. Plain net/http.
package osv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DefaultBaseURL is OSV's public API.
const DefaultBaseURL = "https://api.osv.dev"

// maxBatch is OSV's limit of queries per querybatch call.
const maxBatch = 1000

// cacheTTL: an advisory list for a given version changes rarely; a day keeps
// the daily re-match from re-asking OSV for what did not change.
const cacheTTL = 24 * time.Hour

// Query is one exact package version.
type Query struct {
	Ecosystem string
	Name      string
	Version   string
}

func (q Query) key() string { return q.Ecosystem + "\x00" + q.Name + "\x00" + q.Version }

// Advisory is what the matcher needs from an OSV record.
type Advisory struct {
	ID         string   `json:"id"`
	Summary    string   `json:"summary"`
	Details    string   `json:"details"`
	Aliases    []string `json:"aliases"`
	CVSSVector string   `json:"cvss_vector,omitempty"`
	// Severity is the source's own word when it gives one (Ubuntu priority,
	// GHSA severity), lower-cased; empty otherwise.
	Severity string `json:"severity,omitempty"`
	// Fixed is the first fixed version for the queried package, empty when no
	// fix is published.
	Fixed string `json:"fixed,omitempty"`
}

// Cache stores raw answers. Implementations: Redis in production, a map in tests.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration)
}

// Client talks to OSV.
type Client struct {
	base  string
	http  *http.Client
	cache Cache
	limit *Interval
}

// NewClient builds a client. cache may be nil (no caching). A base URL that
// is not https is refused, except on loopback (tests, a local mirror): package
// lists must not travel in the clear. Redirects to another host are refused.
func NewClient(baseURL string, cache Cache) (*Client, error) {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("osv: invalid base URL %q", baseURL)
	}
	if u.Scheme != "https" && !isLoopback(u.Hostname()) {
		return nil, fmt.Errorf("osv: base URL must be https: %q", baseURL)
	}
	host := u.Host
	return &Client{
		base: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, _ []*http.Request) error {
				if req.URL.Host != host {
					return fmt.Errorf("osv: redirect to another host refused")
				}
				return nil
			},
		},
		cache: cache,
		limit: NewInterval(100 * time.Millisecond), // ≤ 10 requests/s, whole process
	}, nil
}

func isLoopback(h string) bool {
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// Lookup returns, per query, the advisories affecting that exact version. A
// transport or HTTP failure returns an error and no partial result: the caller
// must not read "no advisory" into a failed lookup.
func (c *Client) Lookup(ctx context.Context, queries []Query) (map[Query][]Advisory, error) {
	out := make(map[Query][]Advisory, len(queries))
	ids := make(map[Query][]string, len(queries))

	var todo []Query
	for _, q := range queries {
		if b, ok := c.cacheGet(ctx, "osv:q:"+q.key()); ok {
			var cached []string
			if json.Unmarshal(b, &cached) == nil {
				ids[q] = cached
				continue
			}
		}
		todo = append(todo, q)
	}
	for start := 0; start < len(todo); start += maxBatch {
		end := min(start+maxBatch, len(todo))
		got, err := c.batch(ctx, todo[start:end])
		if err != nil {
			return nil, err
		}
		for i, q := range todo[start:end] {
			ids[q] = got[i]
			if b, err := json.Marshal(got[i]); err == nil {
				c.cacheSet(ctx, "osv:q:"+q.key(), b)
			}
		}
	}

	for q, list := range ids {
		for _, id := range list {
			adv, err := c.advisory(ctx, id, q)
			if err != nil {
				return nil, err
			}
			out[q] = append(out[q], adv)
		}
	}
	return out, nil
}

type batchReq struct {
	Queries []batchQuery `json:"queries"`
}
type batchQuery struct {
	Package struct {
		Name      string `json:"name"`
		Ecosystem string `json:"ecosystem"`
	} `json:"package"`
	Version string `json:"version"`
}
type batchResp struct {
	Results []struct {
		Vulns []struct {
			ID string `json:"id"`
		} `json:"vulns"`
	} `json:"results"`
}

func (c *Client) batch(ctx context.Context, qs []Query) ([][]string, error) {
	req := batchReq{Queries: make([]batchQuery, len(qs))}
	for i, q := range qs {
		req.Queries[i].Package.Name, req.Queries[i].Package.Ecosystem, req.Queries[i].Version = q.Name, q.Ecosystem, q.Version
	}
	body, _ := json.Marshal(req)
	raw, err := c.do(ctx, http.MethodPost, "/v1/querybatch", body)
	if err != nil {
		return nil, err
	}
	var resp batchResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("osv querybatch: %w", err)
	}
	if len(resp.Results) != len(qs) {
		return nil, fmt.Errorf("osv querybatch: %d results for %d queries", len(resp.Results), len(qs))
	}
	out := make([][]string, len(qs))
	for i, r := range resp.Results {
		for _, v := range r.Vulns {
			out[i] = append(out[i], v.ID)
		}
	}
	return out, nil
}

type osvRecord struct {
	ID       string   `json:"id"`
	Summary  string   `json:"summary"`
	Details  string   `json:"details"`
	Aliases  []string `json:"aliases"`
	Severity []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	} `json:"severity"`
	Affected []struct {
		Package struct {
			Ecosystem string `json:"ecosystem"`
			Name      string `json:"name"`
		} `json:"package"`
		Ranges []struct {
			Events []map[string]string `json:"events"`
		} `json:"ranges"`
		Severity []struct {
			Type  string `json:"type"`
			Score string `json:"score"`
		} `json:"severity"`
		EcosystemSpecific map[string]any `json:"ecosystem_specific"`
		DatabaseSpecific  map[string]any `json:"database_specific"`
	} `json:"affected"`
	DatabaseSpecific map[string]any `json:"database_specific"`
}

// advisoryID is what an OSV id looks like (GHSA-…, DEBIAN-CVE-…, USN-…-1).
var advisoryID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

func (c *Client) advisory(ctx context.Context, id string, q Query) (Advisory, error) {
	// The id comes from OSV's (or a mirror's) answer: it becomes a path, so it
	// is checked, escaped, and the record must say it is that id.
	if !advisoryID.MatchString(id) {
		return Advisory{}, fmt.Errorf("osv: unexpected advisory id %q", id)
	}
	raw, ok := c.cacheGet(ctx, "osv:v:"+id)
	if !ok {
		var err error
		raw, err = c.do(ctx, http.MethodGet, "/v1/vulns/"+url.PathEscape(id), nil)
		if err != nil {
			return Advisory{}, err
		}
		c.cacheSet(ctx, "osv:v:"+id, raw)
	}
	var rec osvRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return Advisory{}, fmt.Errorf("osv vuln %s: %w", id, err)
	}
	if rec.ID != id {
		return Advisory{}, fmt.Errorf("osv: asked %s, got %s", id, rec.ID)
	}
	adv := Advisory{ID: rec.ID, Summary: rec.Summary, Details: rec.Details, Aliases: rec.Aliases}
	for _, s := range rec.Severity {
		if strings.HasPrefix(s.Type, "CVSS_V3") {
			adv.CVSSVector = s.Score
		}
	}
	adv.Severity = severityWord(rec.DatabaseSpecific)
	for _, a := range rec.Affected {
		if a.Package.Name != q.Name || !sameEcosystem(a.Package.Ecosystem, q.Ecosystem) {
			continue
		}
		for _, s := range a.Severity {
			if adv.CVSSVector == "" && strings.HasPrefix(s.Type, "CVSS_V3") {
				adv.CVSSVector = s.Score
			}
			// Ubuntu puts its priority here as type "Ubuntu".
			if adv.Severity == "" && s.Type == "Ubuntu" {
				adv.Severity = normaliseWord(s.Score)
			}
		}
		if adv.Severity == "" {
			adv.Severity = severityWord(a.EcosystemSpecific)
		}
		if adv.Severity == "" {
			adv.Severity = severityWord(a.DatabaseSpecific)
		}
		for _, r := range a.Ranges {
			for _, ev := range r.Events {
				if f := ev["fixed"]; f != "" && adv.Fixed == "" {
					adv.Fixed = f
				}
			}
		}
	}
	return adv, nil
}

// sameEcosystem compares "Debian:12" with "Debian:12", and accepts a record
// that names the bare ecosystem ("Debian") for a suffixed query.
func sameEcosystem(record, query string) bool {
	if record == query {
		return true
	}
	base, _, _ := strings.Cut(query, ":")
	return record == base
}

func severityWord(m map[string]any) string {
	for _, k := range []string{"severity", "urgency", "priority"} {
		if s, ok := m[k].(string); ok && s != "" {
			return normaliseWord(s)
		}
	}
	return ""
}

// normaliseWord maps the sources' words onto the register's scale; anything
// else is dropped rather than guessed (the ingest then derives it from CVSS).
func normaliseWord(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return "critical"
	case "high", "important":
		return "high"
	case "medium", "moderate":
		return "medium"
	case "low", "negligible", "unimportant":
		return "low"
	}
	return ""
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	if err := c.limit.Wait(ctx); err != nil {
		return nil, err
	}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rd)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("osv unreachable: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("osv %s %s: HTTP %d", method, path, resp.StatusCode)
	}
	return raw, nil
}

func (c *Client) cacheGet(ctx context.Context, k string) ([]byte, bool) {
	if c.cache == nil {
		return nil, false
	}
	return c.cache.Get(ctx, k)
}

func (c *Client) cacheSet(ctx context.Context, k string, v []byte) {
	if c.cache != nil {
		c.cache.Set(ctx, k, v, cacheTTL)
	}
}

// Interval spaces calls at least gap apart, process-wide. Enough for the
// published limits of OSV and NVD without a new dependency.
type Interval struct {
	mu   sync.Mutex
	gap  time.Duration
	next time.Time
}

// NewInterval builds a limiter.
func NewInterval(gap time.Duration) *Interval { return &Interval{gap: gap} }

// Wait blocks until the next slot or until ctx ends.
func (l *Interval) Wait(ctx context.Context) error {
	l.mu.Lock()
	now := time.Now()
	at := l.next
	if at.Before(now) {
		at = now
	}
	l.next = at.Add(l.gap)
	l.mu.Unlock()
	if d := time.Until(at); d > 0 {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
	return nil
}
