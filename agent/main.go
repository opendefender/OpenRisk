// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

// Command openrisk-agent is the on-prem OpenRisk Scanner Agent.
//
// It enrols with the SaaS using a 24h registration token, then runs continuously
// in the background: it holds an SSE stream for jobs, heartbeats to stay online,
// and on a job it runs nmap (and osquery when present) LOCALLY, then pushes the
// results back over an RS256 (scoped "scanner") + HMAC-SHA256-signed channel.
//
// It is stateless with respect to scan data: nothing but its own credentials is
// persisted, and scan output is discarded after each push.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

// AgentVersion is stamped into registrations and used by the updater.
const AgentVersion = "1.0.0"

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	var (
		server    = flag.String("server", envOr("OPENRISK_SERVER", "http://localhost:8080"), "OpenRisk SaaS base URL")
		regToken  = flag.String("token", os.Getenv("OPENRISK_TOKEN"), "24h registration token (first run only)")
		name      = flag.String("name", "", "agent display name (default: hostname)")
		statePath = flag.String("state", defaultStatePath(), "path to the agent state file")
		install   = flag.Bool("install", false, "print a systemd unit for this agent and exit")
		showVer   = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVer {
		fmt.Println("openrisk-agent", AgentVersion, runtime.GOOS+"/"+runtime.GOARCH)
		return
	}
	if *install {
		printSystemdUnit(*server, *statePath)
		return
	}

	hostname, _ := os.Hostname()
	displayName := *name
	if displayName == "" {
		displayName = hostname
	}

	ag := &Agent{
		Server:    trimSlash(*server),
		StatePath: *statePath,
		Name:      displayName,
		Hostname:  hostname,
		OS:        runtime.GOOS,
	}

	// Load or create state (enrol).
	if err := ag.loadState(); err != nil {
		if *regToken == "" {
			log.Fatalf("no saved state at %s and no -token to enrol with", *statePath)
		}
		log.Printf("enrolling with %s …", ag.Server)
		if err := ag.register(*regToken); err != nil {
			log.Fatalf("registration failed: %v", err)
		}
		log.Printf("enrolled as agent %s", ag.State.AgentID)
	} else {
		log.Printf("resumed as agent %s (state: %s)", ag.State.AgentID, *statePath)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Background loops: heartbeat, auto-update check. The SSE stream runs on the
	// main goroutine and reconnects until the context is cancelled.
	go ag.heartbeatLoop(ctx)
	go ag.updateLoop(ctx)

	log.Printf("OpenRisk Agent %s online — hostname=%s os=%s", AgentVersion, hostname, runtime.GOOS)
	ag.streamLoop(ctx)
	log.Println("agent stopped")
}

// streamLoop holds the SSE job stream, reconnecting with backoff until the
// context is cancelled or the agent is revoked.
func (a *Agent) streamLoop(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		err := a.stream(ctx)
		if err == errRevoked {
			log.Println("this agent was revoked by the SaaS — exiting")
			return
		}
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("stream disconnected: %v (reconnecting in %s)", err, backoff)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (a *Agent) heartbeatLoop(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := a.heartbeat("online"); err != nil {
				log.Printf("heartbeat failed: %v", err)
			}
		}
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
