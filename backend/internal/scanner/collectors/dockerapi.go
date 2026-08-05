// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package collectors

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// This file speaks the Docker Engine REST API directly instead of importing
// github.com/docker/docker.
//
// Why: that module is the Moby monorepo, so depending on it for two list calls
// pulled the entire daemon codebase into the build. govulncheck reported three
// vulnerabilities (GO-2026-5668, GO-2026-4887, GO-2026-4883) against it, all
// with "Fixed in: N/A" — no upstream release resolves them. They are daemon-side
// issues and this process only ever acted as a client, but they were reachable
// on paper through package init, and an unfixable advisory in a dependency
// scanner output is a permanent finding that every future audit must re-litigate.
//
// The collector needs exactly two endpoints. Calling them over net/http removes
// the dependency, and with it the finding, rather than waiting for a fix that
// does not exist.
//
// API reference: GET /containers/json, GET /images/json.

// dockerContainer mirrors the fields of the Engine container summary this
// collector uses. Deliberately partial: unknown fields are ignored by
// encoding/json, so the Engine can add fields without breaking the scan.
type dockerContainer struct {
	ID         string   `json:"Id"`
	Names      []string `json:"Names"`
	Image      string   `json:"Image"`
	State      string   `json:"State"`
	Status     string   `json:"Status"`
	HostConfig struct {
		NetworkMode string `json:"NetworkMode"`
	} `json:"HostConfig"`
}

// dockerImage mirrors the Engine image summary fields this collector uses.
type dockerImage struct {
	ID       string   `json:"Id"`
	RepoTags []string `json:"RepoTags"`
	Size     int64    `json:"Size"`
}

// dockerAPI is a minimal Docker Engine client.
type dockerAPI struct {
	http *http.Client
	// base is the URL prefix for requests. Unix sockets use a fixed dummy host
	// because the socket path is carried by the dialer, not the URL.
	base string
}

const dockerRequestTimeout = 30 * time.Second

// newDockerAPI builds a client from the credential map: `host` (required) plus
// optional PEM `ca_cert` / `client_cert` / `client_key` for mTLS over tcp://.
//
// Accepts the same host forms as the SDK it replaces: unix:// (default when the
// scheme is omitted), tcp:// and http(s)://.
func newDockerAPI(creds map[string]string) (*dockerAPI, error) {
	host := strings.TrimSpace(creds["host"])
	if host == "" {
		return nil, fmt.Errorf("missing required credential: host")
	}

	scheme, addr := splitDockerHost(host)

	switch scheme {
	case "unix":
		// The URL host is ignored by the dialer but must be present and stable,
		// otherwise net/http cannot build a request.
		return &dockerAPI{
			base: "http://docker",
			http: &http.Client{
				Timeout: dockerRequestTimeout,
				Transport: &http.Transport{
					DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
						return (&net.Dialer{}).DialContext(ctx, "unix", addr)
					},
				},
			},
		}, nil

	case "tcp", "http", "https":
		tlsConf, err := dockerTLSConfig(creds)
		if err != nil {
			return nil, err
		}
		// tcp:// means TLS only when certificates were supplied — matching the
		// SDK's behaviour, where plain tcp:// is unencrypted.
		urlScheme := "http"
		if tlsConf != nil || scheme == "https" {
			urlScheme = "https"
		}
		return &dockerAPI{
			base: urlScheme + "://" + addr,
			http: &http.Client{
				Timeout:   dockerRequestTimeout,
				Transport: &http.Transport{TLSClientConfig: tlsConf},
			},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported docker host scheme %q", scheme)
	}
}

// splitDockerHost separates the scheme from the address, defaulting to a unix
// socket path when no scheme is present (as the Docker CLI does).
func splitDockerHost(host string) (scheme, addr string) {
	if idx := strings.Index(host, "://"); idx >= 0 {
		return host[:idx], host[idx+3:]
	}
	return "unix", host
}

// dockerTLSConfig builds the mTLS config, or nil when no certificates are set.
func dockerTLSConfig(creds map[string]string) (*tls.Config, error) {
	if creds["ca_cert"] == "" && creds["client_cert"] == "" {
		return nil, nil
	}

	conf := &tls.Config{MinVersion: tls.VersionTLS12}

	if ca := creds["ca_cert"]; ca != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(ca)) {
			return nil, fmt.Errorf("invalid ca_cert PEM")
		}
		conf.RootCAs = pool
	}

	if creds["client_cert"] != "" && creds["client_key"] != "" {
		pair, err := tls.X509KeyPair([]byte(creds["client_cert"]), []byte(creds["client_key"]))
		if err != nil {
			return nil, fmt.Errorf("invalid client cert/key: %w", err)
		}
		conf.Certificates = []tls.Certificate{pair}
	}

	return conf, nil
}

// get issues a GET against the Engine and decodes the JSON body into out.
func (d *dockerAPI) get(ctx context.Context, path string, query url.Values, out any) error {
	endpoint := d.base + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := d.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// The Engine returns {"message": "..."} on error; surfacing it makes a
		// misconfigured host diagnosable without enabling debug logging.
		var apiErr struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Message != "" {
			return fmt.Errorf("docker api %s: %s (status %d)", path, apiErr.Message, resp.StatusCode)
		}
		return fmt.Errorf("docker api %s: status %d", path, resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

// listContainers returns every container, running or not.
func (d *dockerAPI) listContainers(ctx context.Context) ([]dockerContainer, error) {
	var containers []dockerContainer
	err := d.get(ctx, "/containers/json", url.Values{"all": []string{"true"}}, &containers)
	return containers, err
}

// listImages returns the images present on the host.
func (d *dockerAPI) listImages(ctx context.Context) ([]dockerImage, error) {
	var images []dockerImage
	err := d.get(ctx, "/images/json", nil, &images)
	return images, err
}
