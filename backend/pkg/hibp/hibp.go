// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package hibp checks a password against the HaveIBeenPwned breach corpus using
// the k-anonymity range API.
//
// The password never leaves this process. We SHA-1 it, send only the first five
// hex characters of the digest, and HIBP returns every suffix sharing that
// prefix (a bucket of ~800 hashes). The match happens locally. That is the whole
// point of the range API: the server cannot tell which password was asked about,
// and a passive observer sees a five-character prefix that fits millions of
// candidates.
//
// SHA-1 is not a security choice here — it is the digest HIBP's corpus is keyed
// on, so it is the only one that can address the range endpoint.
package hibp

import (
	"bufio"
	"context"
	"crypto/sha1" // #nosec G505 — required by the HIBP range API, not used as a security primitive.
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DefaultEndpoint is the public range API.
const DefaultEndpoint = "https://api.pwnedpasswords.com/range/"

// HTTPDoer is the seam that keeps this package testable without network access
// (same pattern as pkg/notify and pkg/ticketing).
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client queries the HIBP range API.
type Client struct {
	endpoint string
	http     HTTPDoer
}

// New builds a client with a bounded-timeout HTTP client.
//
// The timeout is deliberately short: this check sits in the signup and password
// reset path, and a slow third party must not hold a user's registration open.
// Callers treat a timeout as "unknown", never as "breached" (see Check).
func New() *Client {
	return &Client{
		endpoint: DefaultEndpoint,
		http:     &http.Client{Timeout: 3 * time.Second},
	}
}

// WithEndpoint overrides the range API base URL (tests, air-gapped mirrors).
func (c *Client) WithEndpoint(endpoint string) *Client {
	c.endpoint = endpoint
	return c
}

// WithHTTPDoer overrides the transport.
func (c *Client) WithHTTPDoer(d HTTPDoer) *Client {
	c.http = d
	return c
}

// ErrUnavailable reports that the corpus could not be consulted. It is distinct
// from "not breached" on purpose: callers must be able to tell "we checked and
// it is clean" from "we could not check", and decide their own policy.
type ErrUnavailable struct{ Err error }

func (e *ErrUnavailable) Error() string { return "hibp unavailable: " + e.Err.Error() }
func (e *ErrUnavailable) Unwrap() error { return e.Err }

// Check reports how many times the password appears in the breach corpus.
//
// A count of 0 means the password was checked and not found. Any transport or
// protocol failure returns an *ErrUnavailable with count 0 — never a silent
// "clean" verdict, so the caller decides whether to fail open or closed.
func (c *Client) Check(ctx context.Context, password string) (int, error) {
	if password == "" {
		return 0, nil
	}

	sum := sha1.Sum([]byte(password)) // #nosec G401 — HIBP corpus is SHA-1 keyed.
	digest := strings.ToUpper(hex.EncodeToString(sum[:]))
	prefix, suffix := digest[:5], digest[5:]

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+prefix, nil)
	if err != nil {
		return 0, &ErrUnavailable{Err: err}
	}
	// Padding asks HIBP to pad every bucket to a uniform size, so response length
	// cannot be used to narrow down which prefix was requested.
	req.Header.Set("Add-Padding", "true")
	req.Header.Set("User-Agent", "OpenRisk-GRC")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, &ErrUnavailable{Err: err}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, &ErrUnavailable{Err: fmt.Errorf("unexpected status %d", resp.StatusCode)}
	}

	// Each line is "SUFFIX:COUNT". Scan rather than ReadAll: buckets are ~20 KB
	// and we stop at the first match.
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		sep := strings.IndexByte(line, ':')
		if sep <= 0 {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(line[:sep]), suffix) {
			continue
		}
		count, convErr := strconv.Atoi(strings.TrimSpace(line[sep+1:]))
		if convErr != nil {
			return 0, &ErrUnavailable{Err: convErr}
		}
		// Padded entries are returned with a count of 0; they are decoys, not hits.
		if count == 0 {
			return 0, nil
		}
		return count, nil
	}
	if err := scanner.Err(); err != nil {
		return 0, &ErrUnavailable{Err: err}
	}

	return 0, nil
}
