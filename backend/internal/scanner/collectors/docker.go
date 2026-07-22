// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package collectors

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"

	"github.com/opendefender/openrisk/internal/domain"
	scanner "github.com/opendefender/openrisk/internal/scanner"
)

// Docker is a real Docker-Engine-SDK CloudCollector. It connects to a Docker
// host (tcp:// with optional mTLS, or a unix socket) and enumerates containers
// (Container assets) and images, flagging containers attached to the host
// network namespace.
type Docker struct{}

// NewDocker returns the Docker collector.
func NewDocker() scanner.CloudCollector { return Docker{} }

func (Docker) Collect(ctx context.Context, cfg scanner.ScanConfig, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery, errs chan<- error) {
	cli, err := dockerClient(cfg.Credentials)
	if err != nil {
		errs <- fmt.Errorf("docker: client: %w", err)
		return
	}
	defer func() { _ = cli.Close() }()

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		errs <- fmt.Errorf("docker: list containers: %w", err)
		return
	}
	for _, c := range containers {
		emitContainer(c, assets, findings)
	}

	images, err := cli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		// Images are secondary — surface the error but keep the container inventory.
		errs <- fmt.Errorf("docker: list images: %w", err)
		return
	}
	for _, im := range images {
		emitImage(im, assets)
	}
}

// dockerClient builds a Docker API client from the credential map: `host`
// (required) plus optional PEM `ca_cert`/`client_cert`/`client_key` for mTLS.
func dockerClient(creds map[string]string) (*client.Client, error) {
	opts := []client.Opt{client.WithHost(creds["host"]), client.WithAPIVersionNegotiation()}
	if creds["ca_cert"] != "" || creds["client_cert"] != "" {
		tlsConf := &tls.Config{MinVersion: tls.VersionTLS12}
		if ca := creds["ca_cert"]; ca != "" {
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM([]byte(ca)) {
				return nil, fmt.Errorf("invalid ca_cert PEM")
			}
			tlsConf.RootCAs = pool
		}
		if creds["client_cert"] != "" && creds["client_key"] != "" {
			pair, err := tls.X509KeyPair([]byte(creds["client_cert"]), []byte(creds["client_key"]))
			if err != nil {
				return nil, fmt.Errorf("invalid client cert/key: %w", err)
			}
			tlsConf.Certificates = []tls.Certificate{pair}
		}
		opts = append(opts, client.WithHTTPClient(&http.Client{Transport: &http.Transport{TLSClientConfig: tlsConf}}))
	}
	return client.NewClientWithOpts(opts...)
}

func emitContainer(c container.Summary, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery) {
	name := ""
	if len(c.Names) > 0 {
		name = strings.TrimPrefix(c.Names[0], "/")
	}
	if name == "" {
		name = shortID(c.ID)
	}
	externalID := "docker:container:" + c.ID
	tags := []string{"docker", "container"}
	if c.State != "" {
		tags = append(tags, c.State)
	}
	netMode := ""
	if c.HostConfig.NetworkMode != "" {
		netMode = c.HostConfig.NetworkMode
	}
	assets <- scanner.AssetDiscovery{
		ExternalID:  externalID,
		Name:        name,
		Type:        domain.AssetTypeContainer,
		CPE:         imageCPE(c.Image),
		Tags:        tags,
		RawMetadata: map[string]any{"image": c.Image, "state": c.State, "status": c.Status, "network_mode": netMode},
	}
	if netMode == "host" {
		findings <- scanner.FindingDiscovery{
			Title:           "Container attached to the host network",
			Description:     fmt.Sprintf("Container %q runs with network mode 'host', bypassing network isolation.", name),
			Severity:        scanner.SeverityMedium,
			Evidence:        "HostConfig.NetworkMode=host",
			RemediationHint: "Use a bridge/overlay network with explicit port publishing instead of host networking.",
			Source:          "docker",
			AssetExternalID: externalID,
		}
	}
}

func emitImage(im image.Summary, assets chan<- scanner.AssetDiscovery) {
	name := shortID(im.ID)
	if len(im.RepoTags) > 0 && im.RepoTags[0] != "<none>:<none>" {
		name = im.RepoTags[0]
	}
	assets <- scanner.AssetDiscovery{
		ExternalID:  "docker:image:" + im.ID,
		Name:        name,
		Type:        domain.AssetTypeContainer,
		CPE:         imageCPE(name),
		Tags:        []string{"docker", "image"},
		RawMetadata: map[string]any{"repo_tags": im.RepoTags, "size": im.Size},
	}
}

// imageCPE derives a coarse CPE from a Docker image reference (registry/name:tag).
func imageCPE(ref string) []string {
	ref = strings.ToLower(ref)
	switch {
	case strings.Contains(ref, "nginx"):
		return []string{"cpe:2.3:a:nginx:nginx"}
	case strings.Contains(ref, "postgres"):
		return []string{"cpe:2.3:a:postgresql:postgresql"}
	case strings.Contains(ref, "redis"):
		return []string{"cpe:2.3:a:redis:redis"}
	case strings.Contains(ref, "mysql"), strings.Contains(ref, "mariadb"):
		return []string{"cpe:2.3:a:mysql:mysql"}
	case strings.Contains(ref, "node"):
		return []string{"cpe:2.3:a:nodejs:node.js"}
	default:
		return nil
	}
}

func shortID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
