// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package email

import (
	"bytes"
	"context"
	"log"
	"os"
	"strings"
	"testing"
)

// RULE #6: the development transport never writes a full address to the log.
func TestLogService_MasksTheAddress(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	if err := NewLogService().SendEmail(context.Background(), "alice@corp.example", "Hi", "<p>secret-link</p>"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "alice@") || !strings.Contains(out, "***@corp.example") {
		t.Fatalf("address must be masked: %q", out)
	}
	if strings.Contains(out, "secret-link") {
		t.Fatal("the body must never be logged")
	}
}

func TestMode(t *testing.T) {
	cases := map[string]Service{
		ModeSMTP:     NewTransport(Config{Host: "smtp.example", Port: 587}),
		ModeLog:      NewLogService(),
		ModeDisabled: NewUnconfigured(),
	}
	for want, svc := range cases {
		if got := Mode(svc); got != want {
			t.Fatalf("Mode(%T) = %q, want %q", svc, got, want)
		}
	}
	if Delivers(NewUnconfigured()) {
		t.Fatal("an unconfigured transport delivers nothing")
	}
}
