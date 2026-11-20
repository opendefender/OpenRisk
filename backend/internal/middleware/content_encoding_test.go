// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #1472: Fiber's Ctx.Body() inflates a compressed request body in full before
// any handler can bound it. These tests use an app with the same posture as
// cmd/server — the real ErrorHandler, the guard mounted on the root app — and
// two routes: an unauthenticated one shaped like POST /api/v1/auth/login and an
// authenticated JSON route behind a stand-in auth gate.

const testBearer = "Bearer valid-token"

// encodingApp builds the test app. reached counts how many times a handler
// read the body, which is the only place an inflate can happen.
func encodingApp(withGuard bool) (*fiber.App, *atomic.Int32) {
	reached := &atomic.Int32{}
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler:          ErrorHandler(true),
	})
	if withGuard {
		app.Use(RejectCompressedRequestBody())
	}
	echo := func(c *fiber.Ctx) error {
		reached.Add(1)
		var payload map[string]string
		if err := c.BodyParser(&payload); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid body")
		}
		return c.JSON(payload)
	}
	api := app.Group("/api/v1")
	api.Post("/auth/login", echo)
	protected := api.Use(func(c *fiber.Ctx) error {
		if c.Get(fiber.HeaderAuthorization) != testBearer {
			return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
		}
		return c.Next()
	})
	protected.Post("/risks", echo)
	return app, reached
}

func gzipped(t *testing.T, raw []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(raw)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func sendEncoded(t *testing.T, app *fiber.App, path string, body []byte, headers http.Header) (*http.Response, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, b
}

// Control: without the guard, Fiber does inflate a gzip body for the handler.
// This is the vector the guard closes; if Fiber ever stops decoding, this test
// says so instead of the guard tests passing for the wrong reason.
func TestRejectCompressedRequestBody_FiberInflatesWithoutGuard(t *testing.T) {
	app, reached := encodingApp(false)
	body := gzipped(t, []byte(`{"email":"a@b.c"}`))

	resp, out := sendEncoded(t, app, "/api/v1/auth/login", body, http.Header{"Content-Encoding": {"gzip"}})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.JSONEq(t, `{"email":"a@b.c"}`, string(out))
	assert.EqualValues(t, 1, reached.Load())
}

// Criterion 1: every coding is refused with a JSON 415 on the unauthenticated
// login route and on an authenticated JSON route, and no handler runs.
func TestRejectCompressedRequestBody_RejectsEncodedBodies(t *testing.T) {
	cases := map[string]http.Header{
		"gzip":                  {"Content-Encoding": {"gzip"}},
		"deflate":               {"Content-Encoding": {"deflate"}},
		"br":                    {"Content-Encoding": {"br"}},
		"zstd":                  {"Content-Encoding": {"zstd"}},
		"upper case":            {"Content-Encoding": {"GZIP"}},
		"unknown coding":        {"Content-Encoding": {"x-made-up"}},
		"identity then gzip":    {"Content-Encoding": {"identity, gzip"}},
		"second header is gzip": {"Content-Encoding": {"identity", "gzip"}},
	}
	routes := map[string]http.Header{
		"unauthenticated login": {},
		"authenticated risks":   {"Authorization": {testBearer}},
	}
	paths := map[string]string{
		"unauthenticated login": "/api/v1/auth/login",
		"authenticated risks":   "/api/v1/risks",
	}
	body := gzipped(t, []byte(`{"name":"x"}`))

	for route, auth := range routes {
		for name, enc := range cases {
			t.Run(route+"/"+name, func(t *testing.T) {
				app, reached := encodingApp(true)
				h := http.Header{}
				for k, v := range auth {
					h[k] = v
				}
				for k, v := range enc {
					h[k] = v
				}

				resp, out := sendEncoded(t, app, paths[route], body, h)

				assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)
				assert.Equal(t, "identity", resp.Header.Get(fiber.HeaderAcceptEncoding))
				var got map[string]any
				require.NoError(t, json.Unmarshal(out, &got), "415 body must be JSON: %s", out)
				assert.Equal(t, true, got["error"])
				assert.Equal(t, unsupportedContentEncodingMessage, got["msg"])
				assert.EqualValues(t, 0, reached.Load(), "handler reached: the body could have been inflated")
			})
		}
	}
}

// Criterion 1, the attack itself: a gzip bomb (64 MiB of zeros, well under
// 1 MiB on the wire) never reaches a handler, so it is never inflated.
func TestRejectCompressedRequestBody_GzipBombNeverInflated(t *testing.T) {
	app, reached := encodingApp(true)
	bomb := gzipped(t, make([]byte, 64<<20))
	require.Less(t, len(bomb), 1<<20)

	resp, _ := sendEncoded(t, app, "/api/v1/auth/login", bomb, http.Header{"Content-Encoding": {"gzip"}})

	assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)
	assert.EqualValues(t, 0, reached.Load())
}

// Criterion 2: no Content-Encoding, or identity in any spelling, is unchanged.
func TestRejectCompressedRequestBody_Success(t *testing.T) {
	cases := map[string]http.Header{
		"absent":             {},
		"identity":           {"Content-Encoding": {"identity"}},
		"identity uppercase": {"Content-Encoding": {"IDENTITY"}},
		"empty":              {"Content-Encoding": {""}},
	}
	for name, enc := range cases {
		for _, path := range []string{"/api/v1/auth/login", "/api/v1/risks"} {
			t.Run(name+path, func(t *testing.T) {
				app, reached := encodingApp(true)
				h := http.Header{"Authorization": {testBearer}}
				for k, v := range enc {
					h[k] = v
				}

				resp, out := sendEncoded(t, app, path, []byte(`{"name":"x"}`), h)

				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.JSONEq(t, `{"name":"x"}`, string(out))
				assert.Empty(t, resp.Header.Get(fiber.HeaderAcceptEncoding))
				assert.EqualValues(t, 1, reached.Load())
			})
		}
	}
}

// Criterion 2: the auth gate behind the guard behaves as before.
func TestRejectCompressedRequestBody_Unauthorized(t *testing.T) {
	app, reached := encodingApp(true)

	resp, _ := sendEncoded(t, app, "/api/v1/risks", []byte(`{"name":"x"}`), http.Header{})

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.EqualValues(t, 0, reached.Load())
}

// Criterion 2: an unknown route still answers 404 when uncompressed. It sits
// outside /api/v1, whose stand-in auth gate answers 401 for any unknown path.
func TestRejectCompressedRequestBody_NotFound(t *testing.T) {
	app, _ := encodingApp(true)

	resp, _ := sendEncoded(t, app, "/no-such-route", []byte(`{}`), http.Header{})

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// Criterion 3: a rejection logs nothing — no body, no header — through the
// standard logger the ErrorHandler uses.
func TestRejectCompressedRequestBody_LogsNothing(t *testing.T) {
	var logged bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(prev) })

	app, _ := encodingApp(true)
	resp, out := sendEncoded(t, app, "/api/v1/auth/login", gzipped(t, []byte(`{"password":"s3cret"}`)),
		http.Header{"Content-Encoding": {"gzip"}, "Authorization": {"Bearer secret-token"}})

	assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)
	assert.Empty(t, logged.String())
	assert.False(t, strings.Contains(string(out), "secret"), "response must echo nothing the client sent")
}
