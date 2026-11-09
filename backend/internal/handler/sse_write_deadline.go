// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Rolling write deadline for server-sent-event streams (#962).
//
// The server runs with WriteTimeout 10s (cmd/server/main.go). fasthttp applies
// it as ONE absolute deadline, set after the handler returns and before the
// streamed body is written. Nothing a stream writes moves it, so every SSE
// connection died on its first keepalive after 10s: the agent reconnected in a
// loop and its status flickered offline on every drop.
//
// Raising or zeroing WriteTimeout would remove slow-reader protection from every
// route. Instead each SSE writer pushes the deadline forward immediately before
// each write. A client that keeps reading keeps its stream; a client that stops
// reading is dropped after one sseWriteTimeout, because the blocked write then
// hits the deadline. The protection stays, scoped to each write rather than to
// the whole response.

// sseDefaultWriteTimeout matches the server's WriteTimeout in cmd/server/main.go,
// so one stream write gets exactly the budget an ordinary response gets.
const sseDefaultWriteTimeout = 10 * time.Second

// sseKeepaliveInterval is the production keepalive of the scanner, mitigation
// and report streams. The realtime stream keeps its own setting.
const sseKeepaliveInterval = 20 * time.Second

// sseKeepaliveOr returns d, or the production interval when d is zero. The
// override exists so a test can watch many keepalives in seconds (#962).
func sseKeepaliveOr(d time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return sseKeepaliveInterval
}

// sseWriteTimeout is the per-write budget. Atomic because stream goroutines read
// it while a test swaps it in t.Cleanup. Production never changes it; a value
// of zero or less is reachable only from a test, where it disables the helper
// to prove the tests reproduce the bug.
var sseWriteTimeout atomic.Int64

func init() {
	sseWriteTimeout.Store(int64(sseDefaultWriteTimeout))
}

// sseWriteDeadline returns the function an SSE writer calls before each write.
//
// It must be called in the handler, BEFORE SetBodyStreamWriter: the writer runs
// after the handler returns and the Fiber context is recycled by then. The
// returned function only holds the connection.
func sseWriteDeadline(c *fiber.Ctx) func() {
	conn := c.Context().Conn()
	return func() {
		d := time.Duration(sseWriteTimeout.Load())
		if conn == nil || d <= 0 {
			return
		}
		// An error means the socket is already closed; the write that follows
		// fails and ends the stream, which is the right outcome.
		_ = conn.SetWriteDeadline(time.Now().Add(d))
	}
}
