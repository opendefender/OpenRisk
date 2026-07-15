// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

var errRevoked = errors.New("agent revoked")

// State is the only thing the agent persists — its identity + credentials.
type State struct {
	AgentID    string `json:"agent_id"`
	Token      string `json:"token"`       // scoped "scanner" JWT (7d)
	PushSecret string `json:"push_secret"` // HMAC-SHA256 key for push signing
}

// Agent holds runtime config and the loaded State.
type Agent struct {
	Server    string
	StatePath string
	Name      string
	Hostname  string
	OS        string

	State State
	busy  atomic.Bool
	http  *http.Client
}

func (a *Agent) client() *http.Client {
	if a.http == nil {
		a.http = &http.Client{Timeout: 30 * time.Second}
	}
	return a.http
}

// --- state persistence -----------------------------------------------------

func (a *Agent) loadState() error {
	b, err := os.ReadFile(a.StatePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &a.State)
}

func (a *Agent) saveState() error {
	if err := os.MkdirAll(filepath.Dir(a.StatePath), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(a.State, "", "  ")
	return os.WriteFile(a.StatePath, b, 0o600) // creds — owner-only
}

// --- registration ----------------------------------------------------------

type registerResponse struct {
	Agent struct {
		ID string `json:"id"`
	} `json:"agent"`
	Token      string `json:"token"`
	PushSecret string `json:"push_secret"`
}

func (a *Agent) register(regToken string) error {
	body, _ := json.Marshal(map[string]string{
		"name": a.Name, "version": AgentVersion, "hostname": a.Hostname, "os": a.OS,
	})
	req, _ := http.NewRequest(http.MethodPost, a.Server+"/api/v1/scanner/agents/register", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+regToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("register HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var rr registerResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return err
	}
	a.State = State{AgentID: rr.Agent.ID, Token: rr.Token, PushSecret: rr.PushSecret}
	return a.saveState()
}

// --- heartbeat -------------------------------------------------------------

func (a *Agent) heartbeat(status string) error {
	req, _ := http.NewRequest(http.MethodPost, a.Server+"/api/v1/scanner/agent/heartbeat?status="+status, nil)
	req.Header.Set("Authorization", "Bearer "+a.State.Token)
	resp, err := a.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		return errRevoked
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat HTTP %d", resp.StatusCode)
	}
	return nil
}

// --- SSE job stream --------------------------------------------------------

type jobDispatch struct {
	Type     string   `json:"type"`
	JobID    string   `json:"job_id"`
	ConfigID string   `json:"config_id"`
	Provider string   `json:"provider"`
	Targets  []string `json:"targets"`
	AgentIDs []string `json:"agent_ids"`
}

// stream opens the SSE job stream and blocks reading it until the connection
// drops or the context is cancelled. Each queued job is handled on a worker so
// the read loop keeps flowing.
func (a *Agent) stream(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, a.Server+"/api/v1/scanner/agent/stream", nil)
	req.Header.Set("Authorization", "Bearer "+a.State.Token)
	req.Header.Set("Accept", "text/event-stream")

	// A long-lived request: no client timeout for the stream body.
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return errRevoked
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("stream HTTP %d", resp.StatusCode)
	}
	log.Println("SSE job stream connected")

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimRight(line, "\r\n")
		if !strings.HasPrefix(line, "data:") {
			continue // comments (": connected"/": keepalive") + blank lines
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var d jobDispatch
		if json.Unmarshal([]byte(payload), &d) != nil {
			continue
		}
		switch d.Type {
		case "agent.revoked":
			return errRevoked
		case "scan.job":
			a.dispatchJob(ctx, d)
		}
	}
}

func (a *Agent) dispatchJob(ctx context.Context, d jobDispatch) {
	if !a.forMe(d.AgentIDs) {
		return
	}
	if !a.busy.CompareAndSwap(false, true) {
		log.Printf("job %s arrived while busy — skipping (will stay queued for another agent)", d.JobID)
		return
	}
	go func() {
		defer a.busy.Store(false)
		a.runJob(ctx, d)
	}()
}

// forMe reports whether a job restricted to specific agents includes this one.
func (a *Agent) forMe(agentIDs []string) bool {
	if len(agentIDs) == 0 {
		return true
	}
	for _, id := range agentIDs {
		if id == a.State.AgentID {
			return true
		}
	}
	return false
}
