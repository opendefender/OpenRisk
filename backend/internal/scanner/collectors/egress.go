// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package collectors

import (
	"net"
	"net/http"
	"time"

	"github.com/opendefender/openrisk/pkg/netguard"
)

// scanHTTPTimeout bounds one API request of a collector, not the whole scan.
const scanHTTPTimeout = 30 * time.Second

// egress is how an in-process collector reaches a tenant-configured endpoint
// (#750, GHSA-98hp-4h7m-v45m). Every SDK that dials an address from the scan
// config's credentials goes through it, so the API pod can never be pointed at
// loopback, the cloud metadata service or a private range the operator has not
// opened with OUTBOUND_ALLOWED_CIDRS. Both also ignore HTTP(S)_PROXY.
//
// Tests swap it for an unguarded one to drive the SDKs against httptest servers,
// which listen on loopback and are therefore refused by the real guard.
var egress = struct {
	dialer func() *net.Dialer
	client func(timeout time.Duration) *http.Client
}{
	dialer: netguard.Dialer,
	client: func(timeout time.Duration) *http.Client {
		return netguard.Client(netguard.Options{Timeout: timeout})
	},
}
