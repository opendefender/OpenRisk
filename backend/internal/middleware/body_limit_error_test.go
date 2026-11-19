// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// serveOverLimit boots a real listener with the same body-limit posture as
// cmd/server (Fiber default BodyLimit, real ErrorHandler): fasthttp rejects an
// over-limit body before any route runs, so app.Test cannot reproduce it.
func serveOverLimit(t *testing.T) string {
	t.Helper()
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler:          ErrorHandler(true),
	})
	h := func(c *fiber.Ctx) error { return c.SendStatus(http.StatusCreated) }
	app.Post("/api/v1/compliance/frameworks/import", h)
	app.Post("/api/v1/risks", h)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() { _ = app.Shutdown() })
	return ln.Addr().String()
}

// postBig declares an over-limit Content-Length and sends only the headers:
// fasthttp rejects on the declared length before reading the body, and a client
// still streaming megabytes at a closed socket sees a reset instead of the
// response, which would make the assertion racy.
func postBig(t *testing.T, base, path string, size int) (int, string) {
	t.Helper()
	conn, err := net.Dial("tcp", base)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	fmt.Fprintf(conn, "POST %s HTTP/1.1\r\nHost: x\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n", path, size)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}
	return resp.StatusCode, string(b)
}

func TestErrorHandler_OverBodyLimitImportGetsValidation400(t *testing.T) {
	base := serveOverLimit(t)
	code, body := postBig(t, base, "/api/v1/compliance/frameworks/import", 5<<20)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", code, body)
	}
	if want := `{"error":"file exceeds the 2 MiB limit"}`; body != want {
		t.Fatalf("body = %s, want %s", body, want)
	}
}

func TestErrorHandler_OverBodyLimitOtherRouteStays413JSON(t *testing.T) {
	base := serveOverLimit(t)
	code, body := postBig(t, base, "/api/v1/risks", 5<<20)
	if code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413 (body %s)", code, body)
	}
	if want := `{"error":true,"msg":"Request Entity Too Large"}`; body != want {
		t.Fatalf("body = %s, want %s", body, want)
	}
}
