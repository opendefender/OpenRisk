// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package collectors

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/opendefender/openrisk/internal/domain"
	scanner "github.com/opendefender/openrisk/internal/scanner"
)

// GitLab is a real gitlab-org/api CloudCollector. It enumerates the projects the
// token is a member of (github.com or a self-managed instance via base_url) and
// flags public projects as an exposure finding.
type GitLab struct{}

// NewGitLab returns the GitLab collector.
func NewGitLab() scanner.CloudCollector { return GitLab{} }

func (GitLab) Collect(ctx context.Context, cfg scanner.ScanConfig, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery, errs chan<- error) {
	opts := []gitlab.ClientOptionFunc{}
	if base := strings.TrimSpace(cfg.Credentials["base_url"]); base != "" {
		opts = append(opts, gitlab.WithBaseURL(base))
	}
	client, err := gitlab.NewClient(cfg.Credentials["token"], opts...)
	if err != nil {
		errs <- fmt.Errorf("gitlab: client: %w", err)
		return
	}

	opt := &gitlab.ListProjectsOptions{
		Membership:  gitlab.Ptr(true),
		ListOptions: gitlab.ListOptions{PerPage: 100, Page: 1},
	}
	for {
		if ctx.Err() != nil {
			errs <- ctx.Err()
			return
		}
		projects, resp, err := client.Projects.ListProjects(opt, gitlab.WithContext(ctx))
		if err != nil {
			errs <- fmt.Errorf("gitlab: list projects: %w", err)
			return
		}
		for _, p := range projects {
			if p == nil {
				continue
			}
			emitProject(p, assets, findings)
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
}

func emitProject(p *gitlab.Project, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery) {
	name := p.PathWithNamespace
	if name == "" {
		name = p.Name
	}
	vis := string(p.Visibility)
	if vis == "" {
		vis = "private"
	}
	externalID := "gitlab:project:" + strconv.FormatInt(p.ID, 10)
	tags := []string{"gitlab", vis}
	if p.Namespace != nil && p.Namespace.FullPath != "" {
		tags = append(tags, "group:"+p.Namespace.FullPath)
	}
	if p.Archived {
		tags = append(tags, "archived")
	}
	assets <- scanner.AssetDiscovery{
		ExternalID: externalID,
		Name:       name,
		Type:       domain.AssetTypeRepository,
		Tags:       tags,
		Location:   ptr(p.WebURL),
		RawMetadata: map[string]any{
			"visibility":     vis,
			"default_branch": p.DefaultBranch,
			"archived":       p.Archived,
			"star_count":     p.StarCount,
		},
	}
	if vis == string(gitlab.PublicVisibility) && !p.Archived {
		findings <- scanner.FindingDiscovery{
			Title:           "Publicly visible repository",
			Description:     fmt.Sprintf("Project %q is publicly visible. Confirm it is intended to be open and contains no secrets.", name),
			Severity:        scanner.SeverityLow,
			Evidence:        "visibility=public",
			RemediationHint: "If the project should be private, change its visibility and rotate any exposed credentials.",
			Source:          "gitlab",
			AssetExternalID: externalID,
		}
	}
}
