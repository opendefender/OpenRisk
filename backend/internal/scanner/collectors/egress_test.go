// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package collectors

import (
	"context"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"

	"github.com/opendefender/openrisk/internal/domain"
	scanner "github.com/opendefender/openrisk/internal/scanner"
	"github.com/opendefender/openrisk/pkg/netguard"
)

// unguardedEgress lets a test drive a collector against an httptest server on
// loopback, which the real guard refuses by design.
func unguardedEgress(t *testing.T) {
	t.Helper()
	saved := egress
	egress.dialer = func() *net.Dialer { return &net.Dialer{Timeout: 5 * time.Second} }
	egress.client = func(timeout time.Duration) *http.Client { return &http.Client{Timeout: timeout} }
	t.Cleanup(func() { egress = saved })
}

// collectErrs runs a collector and returns what it reported on errs.
func collectErrs(t *testing.T, c scanner.CloudCollector, provider domain.ScannerProvider, creds map[string]string) []error {
	t.Helper()
	assets := make(chan scanner.AssetDiscovery, 64)
	findings := make(chan scanner.FindingDiscovery, 64)
	errs := make(chan error, 64)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c.Collect(ctx, scanner.ScanConfig{Provider: provider, Credentials: creds}, assets, findings, errs)
	close(assets)
	close(findings)
	close(errs)
	assert.Empty(t, assets, "a refused endpoint must yield no assets")
	var out []error
	for err := range errs {
		out = append(out, err)
	}
	return out
}

// requireDenied asserts the collector failed on the guard and never reached the
// server behind the loopback address.
func requireDenied(t *testing.T, errs []error, hits *atomic.Int32) {
	t.Helper()
	require.NotEmpty(t, errs, "the collector must report the refused endpoint")
	assert.Contains(t, errs[0].Error(), netguard.ErrDenied.Error())
	assert.Zero(t, hits.Load(), "the server behind a loopback address must never be reached")
}

// loopbackServer counts requests, so a test can prove the guard stopped the
// connection rather than the server answering with an error.
func loopbackServer(t *testing.T, tlsServer bool) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	hits := new(atomic.Int32)
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`[]`))
	})
	var srv *httptest.Server
	if tlsServer {
		srv = httptest.NewTLSServer(h)
	} else {
		srv = httptest.NewServer(h)
	}
	t.Cleanup(srv.Close)
	return srv, hits
}

func serverCAPEM(srv *httptest.Server) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}))
}

// "localhost" passes the literal-address check; it is the dialer that must stop
// it. That is the DNS path a stored config — or a rebinding name — would take.
func loopbackName(srv *httptest.Server) string {
	return strings.Replace(srv.Listener.Addr().String(), "127.0.0.1", "localhost", 1)
}

func TestGitHubCollect_RefusesLoopbackBaseURL(t *testing.T) {
	srv, hits := loopbackServer(t, false)
	errs := collectErrs(t, GitHub{}, domain.ProviderGitHub, map[string]string{"token": "ghp_test", "base_url": srv.URL + "/"})
	requireDenied(t, errs, hits)
}

func TestGitLabCollect_RefusesLoopbackBaseURL(t *testing.T) {
	srv, hits := loopbackServer(t, false)
	errs := collectErrs(t, GitLab{}, domain.ProviderGitLab, map[string]string{"token": "glpat_test", "base_url": srv.URL})
	requireDenied(t, errs, hits)
}

func TestDockerCollect_RefusesLoopbackHost(t *testing.T) {
	srv, hits := loopbackServer(t, false)
	errs := collectErrs(t, Docker{}, domain.ProviderDocker, map[string]string{"host": "tcp://" + loopbackName(srv)})
	requireDenied(t, errs, hits)
}

func TestDockerCollect_RefusesSocketByDefault(t *testing.T) {
	t.Setenv(scanner.DockerSocketEnv, "")
	errs := collectErrs(t, Docker{}, domain.ProviderDocker, map[string]string{"host": "unix:///var/run/docker.sock"})
	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Error(), scanner.DockerSocketEnv)
}

func TestKubernetesCollect_RefusesLoopbackAPIServer(t *testing.T) {
	srv, hits := loopbackServer(t, true)
	// A valid CA, so the only thing that can stop the request is the guard.
	errs := collectErrs(t, Kubernetes{}, domain.ProviderKubernetes, map[string]string{
		"api_server": "https://" + loopbackName(srv), "token": "t", "ca_cert": serverCAPEM(srv),
	})
	requireDenied(t, errs, hits)
}

func TestActiveDirectoryCollect_RefusesLoopbackAndLocalSocket(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	hits := new(atomic.Int32)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			hits.Add(1)
			_ = c.Close()
		}
	}()
	creds := map[string]string{"bind_dn": "cn=x", "password": "p", "base_dn": "dc=x"}

	creds["url"] = "ldap://" + strings.Replace(ln.Addr().String(), "127.0.0.1", "localhost", 1)
	requireDenied(t, collectErrs(t, ActiveDirectory{}, domain.ProviderActiveDirectory, creds), hits)

	for _, u := range []string{"ldapi:///var/run/slapd/ldapi", "cldap://dc.example.com"} {
		creds["url"] = u
		errs := collectErrs(t, ActiveDirectory{}, domain.ProviderActiveDirectory, creds)
		require.NotEmpty(t, errs, u)
		assert.Contains(t, errs[0].Error(), "ldap:// or ldaps://", u)
	}
}

func TestVMwareCollect_RefusesLoopbackURL(t *testing.T) {
	simulator.Test(func(ctx context.Context, vc *vim25.Client) {
		u := vc.URL()
		target := "https://" + strings.Replace(u.Host, "127.0.0.1", "localhost", 1) + u.Path
		errs := collectErrs(t, VMware{}, domain.ProviderVMware, map[string]string{
			"url": target, "username": "user", "password": "pass", "insecure": "true",
		})
		require.NotEmpty(t, errs)
		assert.Contains(t, errs[0].Error(), netguard.ErrDenied.Error())
	})
}

// The unguarded seam must not leak into production: the default egress refuses.
func TestEgress_DefaultIsGuarded(t *testing.T) {
	_, err := egress.dialer().Dial("tcp", "127.0.0.1:1")
	require.Error(t, err)
	assert.True(t, netguard.IsDenied(err))
}
