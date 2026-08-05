// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package collectors

import (
	"context"
	"fmt"
	"strings"

	"github.com/opendefender/openrisk/internal/domain"
	scanner "github.com/opendefender/openrisk/internal/scanner"
)

// Docker is a real Docker-Engine CloudCollector. It connects to a Docker host
// (tcp:// with optional mTLS, or a unix socket) and enumerates containers
// (Container assets) and images, flagging containers attached to the host
// network namespace.
//
// It speaks the Engine REST API directly rather than importing the Moby SDK —
// see dockerapi.go for why.
type Docker struct{}

// NewDocker returns the Docker collector.
func NewDocker() scanner.CloudCollector { return Docker{} }

func (Docker) Collect(ctx context.Context, cfg scanner.ScanConfig, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery, errs chan<- error) {
	cli, err := newDockerAPI(cfg.Credentials)
	if err != nil {
		errs <- fmt.Errorf("docker: client: %w", err)
		return
	}

	containers, err := cli.listContainers(ctx)
	if err != nil {
		errs <- fmt.Errorf("docker: list containers: %w", err)
		return
	}
	for _, c := range containers {
		emitContainer(c, assets, findings)
	}

	images, err := cli.listImages(ctx)
	if err != nil {
		// Images are secondary — surface the error but keep the container inventory.
		errs <- fmt.Errorf("docker: list images: %w", err)
		return
	}
	for _, im := range images {
		emitImage(im, assets)
	}
}

func emitContainer(c dockerContainer, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery) {
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

func emitImage(im dockerImage, assets chan<- scanner.AssetDiscovery) {
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
