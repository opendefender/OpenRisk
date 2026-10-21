// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package email

import (
	"context"
	"net"
	"testing"
	"time"
)

// #997 — a relay that accepts the connection and never answers holds a send
// until its deadline. Cancelling the context (the shutdown drain does) ends it
// at once.
func TestTransportSendEmail_CancelEndsABlackholedRelay(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = conn.Close() }) // silent: no greeting, ever
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	tr := NewTransport(Config{Host: "127.0.0.1", Port: addr.Port, From: "noreply@openrisk.test", Timeout: 10 * time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	start := time.Now()
	if err := tr.SendEmail(ctx, "someone@example.test", "s", "<p>b</p>"); err == nil {
		t.Fatal("a send to a silent relay cannot succeed")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("cancel did not end the send: took %s, the timeout is 10s", elapsed)
	}
}
