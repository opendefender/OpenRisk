// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// unsupportedContentEncodingMessage is the client-facing reason for the 415. It
// names no encoding and echoes nothing the client sent.
const unsupportedContentEncodingMessage = "compressed request bodies are not accepted; send the body uncompressed"

// RejectCompressedRequestBody refuses any request whose Content-Encoding is
// anything other than absent or "identity" (#1472).
//
// Fiber v2 Ctx.Body() decodes gzip, deflate and br bodies in full, in memory,
// before a handler can look at the size. The global BodyLimit bounds only the
// compressed bytes on the wire, so a request of a few MiB could inflate to
// several GiB: an unauthenticated, remote way to exhaust the API's memory. No
// client of ours sends compressed bodies, so the guard refuses them outright
// rather than decoding them under a limit.
//
// It reads the request headers only and never touches the body, so nothing is
// inflated. It inspects every Content-Encoding header the request carries,
// because Ctx.Body() reads them all (the last one wins) and a request could
// otherwise hide "gzip" behind a first, harmless "identity".
//
// The 415 carries Accept-Encoding: identity (RFC 7694 §3) and goes through the
// app ErrorHandler as a fiber.Error, so the client gets the usual JSON error
// body and nothing is logged beyond the access log line.
//
// Mount it globally, right after fiber.New and before any handler or
// middleware that reads the body, so every route present and future is
// covered without opting in.
func RejectCompressedRequestBody() fiber.Handler {
	return func(c *fiber.Ctx) error {
		rejected := false
		c.Request().Header.VisitAll(func(key, value []byte) {
			if rejected || !strings.EqualFold(string(key), fiber.HeaderContentEncoding) {
				return
			}
			if !isIdentityEncoding(string(value)) {
				rejected = true
			}
		})
		if rejected {
			c.Set(fiber.HeaderAcceptEncoding, "identity")
			return fiber.NewError(fiber.StatusUnsupportedMediaType, unsupportedContentEncodingMessage)
		}
		return c.Next()
	}
}

// isIdentityEncoding reports whether a Content-Encoding value applies no
// coding: every comma-separated token is empty or "identity", case-insensitive.
func isIdentityEncoding(value string) bool {
	for _, token := range strings.Split(value, ",") {
		token = strings.TrimSpace(token)
		if token != "" && !strings.EqualFold(token, "identity") {
			return false
		}
	}
	return true
}
