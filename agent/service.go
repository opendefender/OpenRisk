// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// defaultStatePath keeps the agent's credentials in the per-user config dir
// (owner-only). A service running as its own user gets a stable, private path.
func defaultStatePath() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "openrisk-agent", "state.json")
}

// printSystemdUnit prints a ready-to-install systemd unit for a persistent,
// boot-started agent (Linux). Windows/macOS install notes live in README.md.
func printSystemdUnit(server, statePath string) {
	exe, _ := os.Executable()
	fmt.Printf(`# Save as /etc/systemd/system/openrisk-agent.service, then:
#   sudo systemctl daemon-reload && sudo systemctl enable --now openrisk-agent
#
[Unit]
Description=OpenRisk Scanner Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
# First run needs -token <REGISTRATION_TOKEN>; drop it after the state file exists.
ExecStart=%s -server %s -state %s
Restart=always
RestartSec=10
# Least privilege; add AmbientCapabilities=CAP_NET_RAW to enable nmap -O (OS detect).
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=%s

[Install]
WantedBy=multi-user.target
`, exe, server, statePath, filepath.Dir(statePath))
}
