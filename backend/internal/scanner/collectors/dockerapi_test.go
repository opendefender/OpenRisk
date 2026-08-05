// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package collectors

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestDockerAPI points the client at a stub Engine.
func newTestDockerAPI(t *testing.T, handler http.HandlerFunc) *dockerAPI {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &dockerAPI{http: srv.Client(), base: srv.URL}
}

func TestDockerAPI_ListContainers(t *testing.T) {
	var gotPath, gotQuery string
	api := newTestDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"Id":"abc123","Names":["/web"],"Image":"nginx:1.25","State":"running","Status":"Up 2 hours",
			 "HostConfig":{"NetworkMode":"host"}},
			{"Id":"def456","Names":["/db"],"Image":"postgres:16","State":"exited","Status":"Exited (0)",
			 "HostConfig":{"NetworkMode":"bridge"}}
		]`))
	})

	containers, err := api.listContainers(context.Background())
	require.NoError(t, err)
	require.Len(t, containers, 2)

	assert.Equal(t, "/containers/json", gotPath)
	// all=true is what makes stopped containers appear; without it the inventory
	// silently omits everything not currently running.
	assert.Equal(t, "all=true", gotQuery)

	assert.Equal(t, "abc123", containers[0].ID)
	assert.Equal(t, []string{"/web"}, containers[0].Names)
	assert.Equal(t, "host", containers[0].HostConfig.NetworkMode)
	assert.Equal(t, "exited", containers[1].State)
}

func TestDockerAPI_ListImages(t *testing.T) {
	api := newTestDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/images/json", r.URL.Path)
		_, _ = w.Write([]byte(`[{"Id":"sha256:deadbeef","RepoTags":["postgres:16"],"Size":123456}]`))
	})

	images, err := api.listImages(context.Background())
	require.NoError(t, err)
	require.Len(t, images, 1)
	assert.Equal(t, "sha256:deadbeef", images[0].ID)
	assert.Equal(t, []string{"postgres:16"}, images[0].RepoTags)
	assert.Equal(t, int64(123456), images[0].Size)
}

// TestDockerAPI_UnknownFieldsIgnored: the Engine adds fields between versions,
// and a scan must not break because of one.
func TestDockerAPI_UnknownFieldsIgnored(t *testing.T) {
	api := newTestDockerAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"Id":"abc","Names":["/x"],"SomeFutureField":{"nested":true},"Mounts":[]}]`))
	})

	containers, err := api.listContainers(context.Background())
	require.NoError(t, err)
	require.Len(t, containers, 1)
	assert.Equal(t, "abc", containers[0].ID)
}

// TestDockerAPI_SurfacesEngineError: a misconfigured host must be diagnosable
// from the error alone.
func TestDockerAPI_SurfacesEngineError(t *testing.T) {
	api := newTestDockerAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"permission denied on /var/run/docker.sock"}`))
	})

	_, err := api.listContainers(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
	assert.Contains(t, err.Error(), "403")
}

func TestDockerAPI_ErrorWithoutMessageStillReportsStatus(t *testing.T) {
	api := newTestDockerAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := api.listContainers(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestSplitDockerHost(t *testing.T) {
	cases := []struct {
		host       string
		wantScheme string
		wantAddr   string
	}{
		// No scheme means a socket path, matching the Docker CLI.
		{"/var/run/docker.sock", "unix", "/var/run/docker.sock"},
		{"unix:///var/run/docker.sock", "unix", "/var/run/docker.sock"},
		{"tcp://10.0.0.5:2376", "tcp", "10.0.0.5:2376"},
		{"https://docker.internal:2376", "https", "docker.internal:2376"},
	}
	for _, tc := range cases {
		scheme, addr := splitDockerHost(tc.host)
		assert.Equal(t, tc.wantScheme, scheme, tc.host)
		assert.Equal(t, tc.wantAddr, addr, tc.host)
	}
}

func TestNewDockerAPI(t *testing.T) {
	t.Run("host is required", func(t *testing.T) {
		_, err := newDockerAPI(map[string]string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "host")
	})

	t.Run("unix socket", func(t *testing.T) {
		api, err := newDockerAPI(map[string]string{"host": "unix:///var/run/docker.sock"})
		require.NoError(t, err)
		// The URL host is a placeholder: the socket path lives in the dialer.
		assert.Equal(t, "http://docker", api.base)
	})

	t.Run("plain tcp stays cleartext", func(t *testing.T) {
		// Matches the SDK's behaviour — tcp:// without certificates is not TLS.
		api, err := newDockerAPI(map[string]string{"host": "tcp://10.0.0.5:2375"})
		require.NoError(t, err)
		assert.Equal(t, "http://10.0.0.5:2375", api.base)
	})

	t.Run("tcp upgrades to TLS when a CA is supplied", func(t *testing.T) {
		api, err := newDockerAPI(map[string]string{"host": "tcp://10.0.0.5:2376", "ca_cert": testCAPEM})
		require.NoError(t, err)
		assert.Equal(t, "https://10.0.0.5:2376", api.base)
	})

	t.Run("invalid CA is rejected", func(t *testing.T) {
		_, err := newDockerAPI(map[string]string{"host": "tcp://10.0.0.5:2376", "ca_cert": "not a pem"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ca_cert")
	})

	t.Run("unsupported scheme", func(t *testing.T) {
		_, err := newDockerAPI(map[string]string{"host": "npipe:////./pipe/docker_engine"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported")
	})
}

// testCAPEM is a throwaway self-signed certificate used only to exercise the
// PEM-parsing path. It is not a credential.
const testCAPEM = `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`
