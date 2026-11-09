// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package netguard

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
)

// hostAddr returns a non-loopback IPv4 address of this machine, so a test server
// can listen somewhere a Policy can open (loopback never can).
func hostAddr(t *testing.T) netip.Addr {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Skipf("no interface addresses: %v", err)
	}
	for _, a := range addrs {
		n, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip, ok := netip.AddrFromSlice(n.IP)
		if ok && ip.Unmap().Is4() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
			return ip.Unmap()
		}
	}
	t.Skip("no non-loopback IPv4 interface to listen on")
	return netip.Addr{}
}

// A server the policy allows answers normally; every redirect it issues towards
// loopback, the metadata endpoint or plain http is refused, end to end, and the
// loopback server behind the redirect is never reached (#1315).
func TestClient_RedirectFromAllowedServer(t *testing.T) {
	var internalHits atomic.Int32
	internal := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		internalHits.Add(1)
	}))
	defer internal.Close()

	ip := hostAddr(t)
	ln, err := net.Listen("tcp", net.JoinHostPort(ip.String(), "0"))
	if err != nil {
		t.Skipf("cannot listen on %s: %v", ip, err)
	}
	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)
	srv.Listener.Close()
	srv.Listener = ln
	srv.StartTLS()
	defer srv.Close()
	base := srv.URL // https://<ip>:<port>
	mux.HandleFunc("/ok", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("/next", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/ok", http.StatusFound) })
	redirect := map[string]string{
		"/to-loopback": internal.URL + "/",
		"/to-metadata": "https://169.254.169.254/latest/meta-data/",
		"/to-private":  "https://10.0.0.5/",
		"/to-http":     "http://" + ln.Addr().String() + "/ok", // downgrade, same host
		"/to-file":     "file:///etc/passwd",
	}
	for path, to := range redirect {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, to, http.StatusFound) })
	}

	c := Policy{Allowed: []netip.Prefix{netip.PrefixFrom(ip, 32)}}.Client(Options{})
	// The httptest certificate names example.com, not the interface address.
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	c.Transport.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: pool, ServerName: "example.com"}

	for _, path := range []string{"/ok", "/next"} {
		resp, err := c.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s on an allowed host: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: status %d", path, resp.StatusCode)
		}
	}
	for path, to := range redirect {
		resp, err := c.Get(base + path)
		if resp != nil {
			resp.Body.Close()
		}
		if err == nil || !IsDenied(err) {
			t.Errorf("redirect %s -> %s: err = %v, want ErrDenied", path, to, err)
		}
	}
	if n := internalHits.Load(); n != 0 {
		t.Fatalf("the loopback server behind a redirect was reached %d time(s)", n)
	}
}
