// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package netguard is the one gate every server-side request to a
// tenant-configured URL goes through (#573). A tenant can point an integration
// at any host it likes; netguard makes sure that host is on the public
// internet, so the API pod never becomes a proxy into its own network — cloud
// metadata, loopback services, or the cluster's private ranges.
//
// Two layers, because one is not enough:
//
//   - ValidateURL rejects a bad URL when it is saved or submitted: https only, no
//     credentials in the URL, and no host that is literally a denied address.
//   - Client returns an *http.Client whose dialer checks the IP it is actually
//     about to connect to. A hostname that resolves to a private address, or
//     that re-resolves to one after it was validated (DNS rebinding), is refused
//     at connect time, on every request and every redirect.
//
// An operator running OpenRisk next to on-premise tools (Nessus, a self-hosted
// Jira) can open specific private ranges with OUTBOUND_ALLOWED_CIDRS. Loopback,
// link-local (which holds the cloud metadata endpoints) and unspecified
// addresses stay denied whatever that list says.
package netguard

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"
)

// ErrDenied is returned (wrapped) when a URL or the address it resolves to is
// not allowed. Callers match it with errors.Is to answer 400 instead of 502.
var ErrDenied = errors.New("outbound target not allowed")

// AllowedCIDRsEnv names the operator allow-list of private ranges.
const AllowedCIDRsEnv = "OUTBOUND_ALLOWED_CIDRS"

// deniedPrefixes are the ranges no tenant-configured request may reach, on top
// of what netip.Addr's own predicates cover (loopback, private, link-local,
// multicast, unspecified).
var deniedPrefixes = mustPrefixes(
	"0.0.0.0/8",       // "this network"
	"100.64.0.0/10",   // carrier-grade NAT; also Alibaba's metadata endpoint
	"192.0.0.0/24",    // IETF protocol assignments
	"192.0.2.0/24",    // TEST-NET-1
	"198.18.0.0/15",   // benchmarking
	"198.51.100.0/24", // TEST-NET-2
	"203.0.113.0/24",  // TEST-NET-3
	"240.0.0.0/4",     // reserved, includes 255.255.255.255
	"64:ff9b::/96",    // NAT64: embeds an IPv4 address
	"64:ff9b:1::/48",  // local-use NAT64
	"2001:db8::/32",   // documentation
	"2002::/16",       // 6to4: embeds an IPv4 address
	"fec0::/10",       // deprecated site-local
	"::/96",           // IPv4-compatible (deprecated): embeds an IPv4 address
	"::ffff:0:0:0/96", // SIIT IPv4-translated: embeds an IPv4 address
	"2001::/32",       // Teredo: embeds an IPv4 address
	"100::/64",        // discard-only
	"192.88.99.0/24",  // retired 6to4 relay anycast
)

func mustPrefixes(ss ...string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(ss))
	for _, s := range ss {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}

// Policy decides which addresses are reachable. The zero value denies every
// non-public address.
type Policy struct {
	// Allowed opens these ranges even though they are private. It never opens
	// loopback, link-local or unspecified addresses.
	Allowed []netip.Prefix
}

// PolicyFromEnv reads OUTBOUND_ALLOWED_CIDRS (comma-separated CIDRs). An entry
// that does not parse is an error, so a typo fails loudly at startup instead
// of silently leaving a range closed or open.
func PolicyFromEnv() (Policy, error) {
	raw := strings.TrimSpace(os.Getenv(AllowedCIDRsEnv))
	if raw == "" {
		return Policy{}, nil
	}
	var p Policy
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		pfx, err := netip.ParsePrefix(s)
		if err != nil {
			return Policy{}, fmt.Errorf("%s: %q is not a CIDR: %w", AllowedCIDRsEnv, s, err)
		}
		p.Allowed = append(p.Allowed, pfx.Masked())
	}
	return p, nil
}

// defaultPolicy is what the package-level helpers use. It is read once from
// the environment; an unparsable value leaves it at the deny-all zero value.
var defaultPolicy, defaultPolicyErr = PolicyFromEnv()

// DefaultPolicyError reports a malformed OUTBOUND_ALLOWED_CIDRS so main can
// refuse to start rather than run with a policy the operator did not intend.
func DefaultPolicyError() error { return defaultPolicyErr }

// AddrAllowed reports whether ip may be connected to.
func (p Policy) AddrAllowed(ip netip.Addr) bool {
	// Drop the zone: netip.Prefix.Contains is always false for a zoned address,
	// so "64:ff9b::a00:1%1" would otherwise slip past every IPv6 prefix.
	ip = ip.Unmap().WithZone("")
	if !ip.IsValid() {
		return false
	}
	// Never openable: these reach the pod itself or the cloud metadata service.
	if ip.IsUnspecified() || ip.IsLoopback() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return false
	}
	denied := ip.IsPrivate()
	for _, pfx := range deniedPrefixes {
		if pfx.Contains(ip) {
			denied = true
			break
		}
	}
	if !denied {
		return true
	}
	for _, pfx := range p.Allowed {
		if pfx.Contains(ip) {
			return true
		}
	}
	return false
}

// ValidateURL checks a URL before it is stored or requested. It does not
// resolve DNS — the dialer does that check at connect time, where it cannot be
// raced. The returned error wraps ErrDenied and names the rejected target.
func (p Policy) ValidateURL(raw string) error {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("%w: %q is not an absolute URL", ErrDenied, raw)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("%w: %q must use https", ErrDenied, u.Redacted())
	}
	if u.User != nil {
		return fmt.Errorf("%w: %q must not carry credentials", ErrDenied, u.Redacted())
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return fmt.Errorf("%w: %q has no host", ErrDenied, u.Redacted())
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("%w: host %q", ErrDenied, host)
	}
	if strings.Contains(host, "%") {
		return fmt.Errorf("%w: host %q carries an IPv6 zone", ErrDenied, host)
	}
	ip, err := netip.ParseAddr(host)
	if err == nil && !p.AddrAllowed(ip) {
		return fmt.Errorf("%w: address %s", ErrDenied, ip)
	}
	if err != nil && numericHost(host) {
		// 2130706433, 0x7f.1, 0177.0.0.1, 127.1: not a DNS name, but some
		// resolvers turn them into an address. Refuse them outright.
		return fmt.Errorf("%w: host %q is an encoded address", ErrDenied, host)
	}
	return nil
}

// numericHost reports whether the last label of host is a number (decimal,
// octal or 0x-hex). No real top-level domain is numeric, so such a host can only
// be an encoded IP address.
func numericHost(host string) bool {
	last := host[strings.LastIndex(host, ".")+1:]
	if strings.HasPrefix(last, "0x") {
		last = last[2:]
		return last == "" || strings.Trim(last, "0123456789abcdef") == ""
	}
	return last != "" && strings.Trim(last, "0123456789") == ""
}

// ValidateURL checks raw against the default (environment) policy.
func ValidateURL(raw string) error { return defaultPolicy.ValidateURL(raw) }

// control runs after the socket is created and before it connects, with the
// resolved address. It is the check DNS rebinding cannot get past.
func (p Policy) control(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrDenied, address)
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !p.AddrAllowed(ip) {
		return fmt.Errorf("%w: address %s", ErrDenied, host)
	}
	return nil
}

// Options tunes a guarded client.
type Options struct {
	// Timeout bounds the whole request. Zero means 30s.
	Timeout time.Duration
	// NoRedirects returns the 3xx response itself instead of following it.
	NoRedirects bool
}

// Client returns an *http.Client that can only reach addresses p allows.
//
// It deliberately ignores HTTP(S)_PROXY: a proxy would make the connection the
// dialer checks the proxy's, not the target's, and the proxy would then fetch
// the denied target on our behalf.
func (p Policy) Client(o Options) *http.Client {
	if o.Timeout <= 0 {
		o.Timeout = 30 * time.Second
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second, Control: p.control}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	c := &http.Client{Timeout: o.Timeout, Transport: transport}
	if o.NoRedirects {
		c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	} else {
		c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("stopped after 5 redirects")
			}
			// Stay on the host the request was aimed at. Go drops Authorization
			// on a cross-host redirect but keeps custom credential headers
			// (X-ApiKeys) and re-sends a 307/308 body (an OAuth client secret).
			if len(via) > 0 && via[0].URL != nil && !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
				return fmt.Errorf("%w: redirect to another host %q", ErrDenied, req.URL.Host)
			}
			// The dialer re-checks the address; this keeps a redirect from
			// downgrading to http or smuggling credentials in the URL.
			return p.ValidateURL(req.URL.String())
		}
	}
	return c
}

// Client returns a guarded client under the default (environment) policy.
func Client(o Options) *http.Client { return defaultPolicy.Client(o) }

// IsDenied reports whether err came from the guard.
func IsDenied(err error) bool { return errors.Is(err, ErrDenied) }
