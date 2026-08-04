// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"os"
	"strings"
)

// TrustedProxies parses the TRUSTED_PROXIES env var into the list Fiber expects
// for fiber.Config.TrustedProxies.
//
// Why this exists (audit finding F-04): the rate limiter used to derive its key
// from the raw X-Forwarded-For header, which any client can set. An attacker
// rotating that header got a fresh counter on every request, so the login
// throttle — the only brute-force protection on password auth — was trivially
// bypassable.
//
// The fix is to let Fiber own the decision. With EnableTrustedProxyCheck=true,
// c.IP() only honours the proxy header when the peer socket address is in this
// list; otherwise it returns the real socket IP. An empty list therefore means
// "trust nobody", which is the correct default for a directly exposed service:
// the forwarded header is ignored entirely rather than believed.
//
// Format: comma-separated IPs and/or CIDR ranges, e.g.
//
//	TRUSTED_PROXIES=10.0.0.0/8,192.168.1.10
//
// Deployments behind a load balancer MUST set this to the balancer's addresses,
// otherwise every request is attributed to the balancer's own IP and per-IP
// limits collapse into one shared bucket.
func TrustedProxies() []string {
	raw := os.Getenv("TRUSTED_PROXIES")
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	proxies := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			proxies = append(proxies, trimmed)
		}
	}
	return proxies
}
