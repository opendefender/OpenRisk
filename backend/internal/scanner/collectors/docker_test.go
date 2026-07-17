// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1

package collectors

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	scanner "github.com/opendefender/openrisk/internal/scanner"
)

func TestDockerEmitContainer(t *testing.T) {
	assets := make(chan scanner.AssetDiscovery, 4)
	findings := make(chan scanner.FindingDiscovery, 4)

	c := container.Summary{
		ID:    "abc123def456",
		Names: []string{"/web-proxy"},
		Image: "nginx:1.25",
		State: "running",
	}
	c.HostConfig.NetworkMode = "host"
	emitContainer(c, assets, findings)
	close(assets)
	close(findings)

	a := <-assets
	assert.Equal(t, domain.AssetTypeContainer, a.Type)
	assert.Equal(t, "web-proxy", a.Name)
	assert.Equal(t, "docker:container:abc123def456", a.ExternalID)
	assert.Contains(t, a.CPE, "cpe:2.3:a:nginx:nginx")

	f := <-findings
	assert.Equal(t, scanner.SeverityMedium, f.Severity)
	assert.Equal(t, "docker:container:abc123def456", f.AssetExternalID)
}

func TestDockerEmitContainer_NoHostNetwork_NoFinding(t *testing.T) {
	assets := make(chan scanner.AssetDiscovery, 4)
	findings := make(chan scanner.FindingDiscovery, 4)
	c := container.Summary{ID: "x", Names: []string{"/app"}, Image: "app:latest", State: "running"}
	c.HostConfig.NetworkMode = "bridge"
	emitContainer(c, assets, findings)
	close(assets)
	close(findings)

	require.Len(t, assets, 1)
	assert.Empty(t, findings, "bridge networking must not raise a finding")
}

func TestDockerEmitImage(t *testing.T) {
	assets := make(chan scanner.AssetDiscovery, 4)
	emitImage(image.Summary{ID: "sha256:deadbeefcafebabe0000", RepoTags: []string{"postgres:16"}}, assets)
	close(assets)

	a := <-assets
	assert.Equal(t, domain.AssetTypeContainer, a.Type)
	assert.Equal(t, "postgres:16", a.Name)
	assert.Contains(t, a.Tags, "image")
	assert.Contains(t, a.CPE, "cpe:2.3:a:postgresql:postgresql")
}
