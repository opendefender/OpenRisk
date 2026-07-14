// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// releasesURL is the GitHub Releases API for the OpenRisk repo. The agent checks
// it every 24h and logs when a newer version is available. Actual self-replace
// is deliberately left to the packaged installer / container image tag to keep
// the running binary from rewriting itself in place.
const releasesURL = "https://api.github.com/repos/opendefender/OpenRisk/releases/latest"

// updateLoop checks for a newer agent release on start and every 24h.
func (a *Agent) updateLoop(ctx context.Context) {
	a.checkUpdate()
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.checkUpdate()
		}
	}
}

func (a *Agent) checkUpdate() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, releasesURL, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := a.client().Do(req)
	if err != nil {
		return // offline / rate-limited — silent, retry next cycle
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	var rel struct {
		Tag  string `json:"tag_name"`
		HTML string `json:"html_url"`
	}
	if json.NewDecoder(resp.Body).Decode(&rel) != nil {
		return
	}
	latest := strings.TrimPrefix(rel.Tag, "v")
	if latest != "" && latest != AgentVersion {
		log.Printf("update available: openrisk-agent %s (running %s, %s/%s) — %s",
			latest, AgentVersion, runtime.GOOS, runtime.GOARCH, rel.HTML)
	}
}
