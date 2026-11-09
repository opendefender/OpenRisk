// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apprealtime "github.com/opendefender/openrisk/internal/application/realtime"
	redisinfra "github.com/opendefender/openrisk/internal/infrastructure/redis"
	rt "github.com/opendefender/openrisk/pkg/realtime"
)

// ---------------------------------------------------------------------------
// The rolling SSE write deadline (#962), on a real TCP listener.
//
// fiber's in-memory test helper ignores socket deadlines, so it cannot show the
// bug: the server's WriteTimeout is one absolute deadline on the connection, and
// only a real socket enforces it. Each test shrinks production timings to keep
// the suite fast: WriteTimeout 1s instead of 10s, keepalive 200ms instead of
// 20s. The ratio is what matters: several keepalives fit inside one timeout,
// and the read window is three timeouts long.
// ---------------------------------------------------------------------------

const (
	deadlineTestWriteTimeout = 1 * time.Second
	deadlineTestKeepalive    = 200 * time.Millisecond
	deadlineTestWindow       = 3 * time.Second
)

// setSSEWriteTimeout swaps the per-write budget for one test. Zero disables the
// helper, which is how the control case reproduces the bug.
func setSSEWriteTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	prev := sseWriteTimeout.Load()
	sseWriteTimeout.Store(int64(d))
	t.Cleanup(func() { sseWriteTimeout.Store(prev) })
}

// smallSendBufferListener caps the kernel send buffer of every accepted socket,
// so a client that stops reading blocks the server's writes after kilobytes
// rather than megabytes.
type smallSendBufferListener struct {
	net.Listener
	size int
}

func (l smallSendBufferListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetWriteBuffer(l.size)
	}
	return conn, nil
}

// keepaliveRead is what a client saw while it read a stream for one window.
type keepaliveRead struct {
	keepalives int
	// closed is true when the server ended the stream inside the window.
	closed bool
}

// readKeepalives reads a stream for the window and counts keepalive comments.
func readKeepalives(t *testing.T, url string, headers map[string]string, window time.Duration) keepaliveRead {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	lines := make(chan string, 64)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	var out keepaliveRead
	timer := time.NewTimer(window)
	defer timer.Stop()
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				out.closed = true
				return out
			}
			if line == ": keepalive" {
				out.keepalives++
			}
		case <-timer.C:
			return out
		}
	}
}

// streamChannelServer serves ScannerHandler.streamChannel, the loop behind the
// agent stream, on a real listener. It needs a reachable Redis because the
// handler subscribes through the production client, which refuses to start
// without one; CI's integration job provides it.
func streamChannelServer(t *testing.T) string {
	t.Helper()
	return streamChannelServerWith(t, channelStream{})
}

func streamChannelServerWith(t *testing.T, opts channelStream) string {
	t.Helper()

	url := os.Getenv("REDIS_URL")
	if url == "" {
		url = "redis://localhost:6379"
	}
	redisOpts, err := goredis.ParseURL(url)
	require.NoError(t, err)
	probe := goredis.NewClient(redisOpts)
	pingCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := probe.Ping(pingCtx).Err(); err != nil {
		_ = probe.Close()
		t.Skipf("Redis is not reachable on %s, skipping the streamChannel case: %v", url, err)
	}
	_ = probe.Close()

	client := redisinfra.NewClient(url)
	t.Cleanup(func() { _ = client.Close() })
	h := &ScannerHandler{redis: client, keepalive: deadlineTestKeepalive}
	channel := "test:962:" + uuid.NewString()

	app := fiber.New(fiber.Config{DisableStartupMessage: true, WriteTimeout: deadlineTestWriteTimeout})
	app.Get("/stream", func(c *fiber.Ctx) error {
		h.streamChannel(c, channel, opts)
		return nil
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() { _ = app.Shutdown() })
	return "http://" + ln.Addr().String() + "/stream"
}

func realtimeDeadlineServer(t *testing.T) (*streamTestEnv, string) {
	t.Helper()
	e := newStreamTestEnvWith(t, apprealtime.HubOptions{}, streamServerOptions{
		WriteTimeout:      deadlineTestWriteTimeout,
		KeepaliveInterval: deadlineTestKeepalive,
	})
	return e, e.baseURL + "/api/v1/realtime/events"
}

func TestSSEWriteDeadline_StreamSurvivesWriteTimeout(t *testing.T) {
	setSSEWriteTimeout(t, deadlineTestWriteTimeout)

	t.Run("realtime", func(t *testing.T) {
		_, url := realtimeDeadlineServer(t)
		got := readKeepalives(t, url, tenantHeaders(uuid.New()), deadlineTestWindow)
		assert.False(t, got.closed, "the server closed the stream inside the window")
		assert.GreaterOrEqual(t, got.keepalives, 10)
	})

	t.Run("streamChannel", func(t *testing.T) {
		url := streamChannelServer(t)
		got := readKeepalives(t, url, nil, deadlineTestWindow)
		assert.False(t, got.closed, "the server closed the stream inside the window")
		assert.GreaterOrEqual(t, got.keepalives, 10)
	})
}

// The control: the same streams with the helper disabled must be cut by the
// server's WriteTimeout. If this passes with fewer keepalives and a closed
// stream, the test above reproduces the production bug rather than passing by
// accident.
func TestSSEWriteDeadline_ControlWithoutHelperIsCut(t *testing.T) {
	setSSEWriteTimeout(t, 0)

	t.Run("realtime", func(t *testing.T) {
		_, url := realtimeDeadlineServer(t)
		got := readKeepalives(t, url, tenantHeaders(uuid.New()), deadlineTestWindow)
		assert.True(t, got.closed, "without the helper the stream should be cut at WriteTimeout")
		assert.Less(t, got.keepalives, 10)
	})

	t.Run("streamChannel", func(t *testing.T) {
		url := streamChannelServer(t)
		got := readKeepalives(t, url, nil, deadlineTestWindow)
		assert.True(t, got.closed, "without the helper the stream should be cut at WriteTimeout")
		assert.Less(t, got.keepalives, 10)
	})
}

// A client that stops reading must not hold the connection forever. The server
// runs with NO WriteTimeout here, so the per-write deadline is the only thing
// that can end the stream: if the helper set no deadline, or cleared it to
// zero, the blocked write would wait forever and this test would time out.
func TestSSEWriteDeadline_SlowReaderDropped(t *testing.T) {
	const perWrite = 300 * time.Millisecond
	setSSEWriteTimeout(t, perWrite)

	e := newStreamTestEnvWith(t, apprealtime.HubOptions{}, streamServerOptions{
		WriteTimeout:      0,
		KeepaliveInterval: deadlineTestKeepalive,
		SendBuffer:        4 * 1024,
	})
	tenant := uuid.New()

	conn, err := net.Dial("tcp", strings.TrimPrefix(e.baseURL, "http://"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetReadBuffer(4 * 1024)
	}
	// Send the request, then never read a byte.
	_, err = fmt.Fprintf(conn, "GET /api/v1/realtime/events HTTP/1.1\r\nHost: test\r\nX-Test-Tenant: %s\r\n\r\n", tenant)
	require.NoError(t, err)

	require.Eventually(t, func() bool { return e.hub.Stats().Connections == 1 },
		2*time.Second, 10*time.Millisecond, "the stream never subscribed")

	// Flood the stream until the socket buffers fill and a write blocks.
	payload := map[string]any{"note": strings.Repeat("x", 15*1024)}
	started := time.Now()
	limit := started.Add(10 * time.Second)
	for e.hub.Stats().Connections > 0 && time.Now().Before(limit) {
		_, err := e.pub.Publish(context.Background(), rt.Envelope{
			Type:      rt.RiskCreated,
			TenantID:  tenant.String(),
			Aggregate: rt.Aggregate{ID: uuid.NewString()},
			Payload:   payload,
		})
		require.NoError(t, err)
		time.Sleep(2 * time.Millisecond)
	}

	assert.Equal(t, 0, e.hub.Stats().Connections,
		"a client that stopped reading still holds its stream after %s", time.Since(started).Round(time.Millisecond))
	t.Logf("slow reader dropped %s after the flood started", time.Since(started).Round(time.Millisecond))
}
