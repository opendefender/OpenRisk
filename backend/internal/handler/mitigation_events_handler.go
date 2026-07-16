// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	redisinfra "github.com/opendefender/openrisk/internal/infrastructure/redis"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// MitigationEventsHandler streams mitigation.auto_completed events to the browser
// over SSE so a scanner-driven completion updates the mitigation map in real time
// (Auto-detected badge). Native EventSource can't send an Authorization header, so
// the access token is passed as ?token= and validated here; events are filtered to
// the caller's tenant.
type MitigationEventsHandler struct {
	redis     *redisinfra.Client
	rsaKeys   *authpkg.RSAKeys
	blacklist func(jti string) (bool, error)
}

// NewMitigationEventsHandler wires the SSE handler.
func NewMitigationEventsHandler(redis *redisinfra.Client, rsaKeys *authpkg.RSAKeys, blacklist func(jti string) (bool, error)) *MitigationEventsHandler {
	return &MitigationEventsHandler{redis: redis, rsaKeys: rsaKeys, blacklist: blacklist}
}

// Stream is GET /mitigations/events?token=<jwt>.
func (h *MitigationEventsHandler) Stream(c *fiber.Ctx) error {
	claims, err := authpkg.ValidateAccessToken(h.rsaKeys, c.Query("token"), h.blacklist)
	if err != nil || claims.TenantID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or missing token"})
	}
	tenantID := claims.TenantID

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pubsub := h.redis.Subscribe(ctx, "mitigation.auto_completed")
		defer pubsub.Close()
		msgs := pubsub.Channel()

		fmt.Fprint(w, ": connected\n\n")
		if err := w.Flush(); err != nil {
			return
		}

		keepalive := time.NewTicker(20 * time.Second)
		defer keepalive.Stop()

		for {
			select {
			case msg, ok := <-msgs:
				if !ok {
					return
				}
				// Only forward events for this tenant.
				var evt domain.MitigationAutoCompleted
				if err := json.Unmarshal([]byte(msg.Payload), &evt); err != nil || evt.TenantID != tenantID {
					continue
				}
				fmt.Fprintf(w, "event: mitigation.auto_completed\ndata: %s\n\n", msg.Payload)
				if err := w.Flush(); err != nil {
					return
				}
			case <-keepalive.C:
				fmt.Fprint(w, ": keepalive\n\n")
				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	})
	return nil
}
