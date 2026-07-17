// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package collectors

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v66/github"

	"github.com/opendefender/openrisk/internal/domain"
	scanner "github.com/opendefender/openrisk/internal/scanner"
)

// GitHub is a real go-github CloudCollector. It enumerates repositories the
// token can see (an organisation's repos when `org` is set, otherwise the
// authenticated user's) and flags publicly-visible repositories as an exposure
// finding. Works against github.com and GitHub Enterprise Server (via base_url).
type GitHub struct{}

// NewGitHub returns the GitHub collector.
func NewGitHub() scanner.CloudCollector { return GitHub{} }

func (GitHub) Collect(ctx context.Context, cfg scanner.ScanConfig, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery, errs chan<- error) {
	token := cfg.Credentials["token"]
	client := github.NewClient(nil).WithAuthToken(token)
	if base := strings.TrimSpace(cfg.Credentials["base_url"]); base != "" {
		ec, err := client.WithEnterpriseURLs(base, base)
		if err != nil {
			errs <- fmt.Errorf("github: invalid base_url: %w", err)
			return
		}
		client = ec
	}

	org := strings.TrimSpace(cfg.Credentials["org"])
	perPage := 100

	if org != "" {
		opt := &github.RepositoryListByOrgOptions{ListOptions: github.ListOptions{PerPage: perPage}}
		for {
			if ctx.Err() != nil {
				errs <- ctx.Err()
				return
			}
			repos, resp, err := client.Repositories.ListByOrg(ctx, org, opt)
			if err != nil {
				errs <- fmt.Errorf("github: list org %q repos: %w", org, err)
				return
			}
			emitRepos(repos, assets, findings)
			if resp == nil || resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}
		return
	}

	opt := &github.RepositoryListByAuthenticatedUserOptions{ListOptions: github.ListOptions{PerPage: perPage}}
	for {
		if ctx.Err() != nil {
			errs <- ctx.Err()
			return
		}
		repos, resp, err := client.Repositories.ListByAuthenticatedUser(ctx, opt)
		if err != nil {
			errs <- fmt.Errorf("github: list repos: %w", err)
			return
		}
		emitRepos(repos, assets, findings)
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
}

func emitRepos(repos []*github.Repository, assets chan<- scanner.AssetDiscovery, findings chan<- scanner.FindingDiscovery) {
	for _, r := range repos {
		if r == nil {
			continue
		}
		full := r.GetFullName()
		if full == "" {
			full = r.GetName()
		}
		vis := r.GetVisibility()
		if vis == "" {
			if r.GetPrivate() {
				vis = "private"
			} else {
				vis = "public"
			}
		}
		tags := []string{"github", vis}
		if lang := r.GetLanguage(); lang != "" {
			tags = append(tags, "lang:"+strings.ToLower(lang))
		}
		if r.GetArchived() {
			tags = append(tags, "archived")
		}
		if owner := r.GetOwner().GetLogin(); owner != "" {
			tags = append(tags, "owner:"+owner)
		}
		externalID := r.GetNodeID()
		if externalID == "" {
			externalID = "github:" + full
		}
		assets <- scanner.AssetDiscovery{
			ExternalID: externalID,
			Name:       full,
			Type:       domain.AssetTypeRepository,
			Tags:       tags,
			Location:   ptr(r.GetHTMLURL()),
			RawMetadata: map[string]any{
				"visibility":     vis,
				"default_branch": r.GetDefaultBranch(),
				"language":       r.GetLanguage(),
				"archived":       r.GetArchived(),
				"fork":           r.GetFork(),
				"pushed_at":      r.GetPushedAt().String(),
			},
		}
		if vis == "public" && !r.GetArchived() {
			findings <- scanner.FindingDiscovery{
				Title:           "Publicly visible repository",
				Description:     fmt.Sprintf("Repository %q is publicly visible. Confirm it is intended to be open-source and contains no secrets.", full),
				Severity:        scanner.SeverityLow,
				Evidence:        "visibility=public",
				RemediationHint: "If the repository should be private, change its visibility and rotate any exposed credentials.",
				Source:          "github",
				AssetExternalID: externalID,
			}
		}
	}
}
