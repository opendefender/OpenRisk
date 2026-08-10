// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package hibp

import (
	"context"
	"crypto/sha1" // #nosec G505 — mirrors the production digest choice.
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// digestOf returns the uppercase SHA-1 prefix/suffix split the range API uses.
func digestOf(password string) (prefix, suffix string) {
	sum := sha1.Sum([]byte(password)) // #nosec G401
	d := strings.ToUpper(hex.EncodeToString(sum[:]))
	return d[:5], d[5:]
}

func TestCheck_ReportsBreachCount(t *testing.T) {
	const password = "Password123!"
	wantPrefix, wantSuffix := digestOf(password)

	var gotPath, gotPadding string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotPadding = r.Header.Get("Add-Padding")
		// A realistic bucket: decoys around the real answer.
		_, _ = w.Write([]byte(
			"0018A45C4D1DEF81644B54AB7F969B88D65:1\r\n" +
				wantSuffix + ":37359\r\n" +
				"00D4F6E8FA6EECAD2A3AA415EEC418D38EC:2\r\n"))
	}))
	defer srv.Close()

	count, err := New().WithEndpoint(srv.URL+"/range/").Check(context.Background(), password)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 37359 {
		t.Errorf("expected count 37359, got %d", count)
	}

	// k-anonymity: only the 5-char prefix may leave the process.
	if !strings.HasSuffix(gotPath, "/range/"+wantPrefix) {
		t.Errorf("expected only the %s prefix sent, got path %q", wantPrefix, gotPath)
	}
	if strings.Contains(gotPath, wantSuffix) {
		t.Errorf("the digest suffix must never be transmitted, found it in %q", gotPath)
	}
	if strings.Contains(gotPath, password) {
		t.Errorf("the password must never be transmitted, found it in %q", gotPath)
	}
	if gotPadding != "true" {
		t.Errorf("expected Add-Padding:true so bucket size cannot fingerprint the query, got %q", gotPadding)
	}
}

func TestCheck_CleanPasswordReturnsZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// A bucket that simply does not contain our suffix.
		_, _ = w.Write([]byte("0018A45C4D1DEF81644B54AB7F969B88D65:1\r\nAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA:9\r\n"))
	}))
	defer srv.Close()

	count, err := New().WithEndpoint(srv.URL+"/range/").Check(context.Background(), "Ancre-Vitrail7-Cobalt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 for an absent password, got %d", count)
	}
}

func TestCheck_PaddedDecoyIsNotAHit(t *testing.T) {
	// With Add-Padding, HIBP injects decoy suffixes carrying a count of 0.
	// Treating one as a hit would refuse a perfectly good password.
	const password = "Ancre-Vitrail7-Cobalt"
	_, suffix := digestOf(password)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(suffix + ":0\r\n"))
	}))
	defer srv.Close()

	count, err := New().WithEndpoint(srv.URL+"/range/").Check(context.Background(), password)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected a zero-count padding entry to read as clean, got %d", count)
	}
}

func TestCheck_UnreachableCorpusIsDistinctFromClean(t *testing.T) {
	// The caller must be able to tell "checked, clean" from "could not check".
	c := New().WithHTTPDoer(doerFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial tcp: no route to host")
	}))

	count, err := c.Check(context.Background(), "whatever-long-enough")

	if err == nil {
		t.Fatal("expected an error when the corpus is unreachable")
	}
	var unavailable *ErrUnavailable
	if !errors.As(err, &unavailable) {
		t.Errorf("expected *ErrUnavailable so callers can branch on it, got %T", err)
	}
	if count != 0 {
		t.Errorf("expected count 0 alongside the error, got %d", count)
	}
}

func TestCheck_NonOKStatusIsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, err := New().WithEndpoint(srv.URL+"/range/").Check(context.Background(), "whatever-long-enough")

	var unavailable *ErrUnavailable
	if !errors.As(err, &unavailable) {
		t.Errorf("expected rate-limiting to surface as *ErrUnavailable, got %v", err)
	}
}

func TestCheck_EmptyPasswordSkipsTheNetwork(t *testing.T) {
	called := false
	c := New().WithHTTPDoer(doerFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("should not be reached")
	}))

	count, err := c.Check(context.Background(), "")

	if err != nil || count != 0 {
		t.Errorf("expected a silent zero for the empty password, got %d/%v", count, err)
	}
	if called {
		t.Error("expected no request for an empty password")
	}
}

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }
