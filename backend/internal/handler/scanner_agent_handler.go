// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package handler

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	scanapp "github.com/opendefender/openrisk/internal/application/scanner"
	"github.com/opendefender/openrisk/internal/domain"
	scanpkg "github.com/opendefender/openrisk/internal/scanner"
)

// bearerToken extracts a "Bearer <token>" value, or "" if absent/malformed.
func bearerToken(c *fiber.Ctx) string {
	h := c.Get("Authorization")
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// authenticateAgent authenticates an on-prem Agent for the stream/push endpoints.
// It is called INLINE by those handlers (not as Fiber middleware) so the agent
// routes can be mounted ahead of the /api/v1 user-token middleware without
// inheriting it. Returns (agent, true) on success; on failure it has already
// written the 401/500 response and returns (nil, false).
//
// Steps: (1) validate the RS256 scanner token + required scope; (2) resolve the
// Agent by its token hash — a revoked/rotated token has no matching hash, so
// this doubles as the instant-revocation check; (3) bind token identity
// (tenant + subject) to the resolved Agent.
func (h *ScannerHandler) authenticateAgent(c *fiber.Ctx, requiredScope string) (*domain.ScannerAgent, bool) {
	token := bearerToken(c)
	if token == "" {
		_ = c.Status(401).JSON(fiber.Map{"error": "missing agent token"})
		return nil, false
	}
	claims, err := scanpkg.ValidateScannerToken(h.rsaKeys, token, requiredScope, h.blacklist)
	if err != nil {
		_ = c.Status(401).JSON(fiber.Map{"error": "invalid or unscoped agent token"})
		return nil, false
	}
	agent, err := h.agentRepo.GetByTokenHash(c.UserContext(), scanpkg.HashToken(token))
	if err != nil {
		_ = c.Status(500).JSON(fiber.Map{"error": "agent lookup failed"})
		return nil, false
	}
	if agent == nil {
		_ = c.Status(401).JSON(fiber.Map{"error": "agent unknown or revoked"})
		return nil, false
	}
	if agent.TenantID != claims.TenantID || agent.ID != claims.Sub {
		_ = c.Status(401).JSON(fiber.Map{"error": "token/agent mismatch"})
		return nil, false
	}
	return agent, true
}

// --- Register --------------------------------------------------------------

type registerAgentInput struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
}

// RegisterAgent POST /scanner/agents/register — the Agent's first contact.
// Authenticated by the 24h registration token (scope scanner:register); the
// tenant and config come from the token, never the body.
func (h *ScannerHandler) RegisterAgent(c *fiber.Ctx) error {
	token := bearerToken(c)
	if token == "" {
		return c.Status(401).JSON(fiber.Map{"error": "missing registration token"})
	}
	claims, err := scanpkg.ValidateScannerToken(h.rsaKeys, token, scanpkg.ScopeRegister, h.blacklist)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "invalid or expired registration token"})
	}
	in := new(registerAgentInput)
	if err := c.BodyParser(in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}

	var configID *uuid.UUID
	if claims.Sub != uuid.Nil {
		cid := claims.Sub // registration token carries the config ID in the subject
		configID = &cid
	}
	result, err := h.register.Execute(c.UserContext(), scanapp.RegisterAgentInput{
		TenantID: claims.TenantID,
		ConfigID: configID,
		Name:     in.Name,
		Version:  in.Version,
		IP:       c.IP(),
		Hostname: in.Hostname,
		OS:       in.OS,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	// Token + push_secret are shown exactly once, here.
	return c.Status(201).JSON(result)
}

// --- Stream ----------------------------------------------------------------

// AgentStream GET /scanner/agent/stream — SSE of queued jobs + keepalive.
// Connecting marks the Agent online; disconnecting marks it offline.
func (h *ScannerHandler) AgentStream(c *fiber.Ctx) error {
	agent, ok := h.authenticateAgent(c, scanpkg.ScopeStream)
	if !ok {
		return nil // response already written
	}
	// Mark online on connect (best-effort).
	_ = h.heartbeat.Execute(c.UserContext(), agent, domain.AgentOnline)

	tenantID := agent.TenantID
	agentCopy := *agent
	h.streamChannel(c, scanpkg.AgentJobChannel(tenantID), func() {
		// On disconnect, mark offline. Detached context (the request is gone).
		_ = h.heartbeat.Execute(context.Background(), &agentCopy, domain.AgentOffline)
	})
	return nil
}

// --- Push ------------------------------------------------------------------

type pushResultsInput struct {
	JobID    string                     `json:"job_id"`
	Assets   []scanpkg.AssetDiscovery   `json:"assets"`
	Findings []scanpkg.FindingDiscovery `json:"findings"`
	Errors   []string                   `json:"errors"`
}

// AgentPush POST /scanner/agent/push — receive an Agent's scan results.
// Requires the scanner:push scope AND a valid HMAC-SHA256 body signature
// (X-OpenRisk-Signature) using the Agent's per-agent push secret.
func (h *ScannerHandler) AgentPush(c *fiber.Ctx) error {
	agent, ok := h.authenticateAgent(c, scanpkg.ScopePush)
	if !ok {
		return nil // response already written
	}

	body := c.Body()
	sig := c.Get("X-OpenRisk-Signature")
	secret, err := h.cipher.DecryptString(agent.PushSecretEnc)
	if err != nil || secret == "" {
		return c.Status(401).JSON(fiber.Map{"error": "agent push secret unavailable"})
	}
	if sig == "" || !scanpkg.VerifyPushSignature(secret, body, sig) {
		return c.Status(401).JSON(fiber.Map{"error": "invalid push signature"})
	}

	in := new(pushResultsInput)
	if err := json.Unmarshal(body, in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}
	jobID, err := uuid.Parse(in.JobID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid job id"})
	}

	job, err := h.push.Execute(c.UserContext(), scanapp.PushResultsInput{
		Agent:    agent,
		JobID:    jobID,
		Assets:   in.Assets,
		Findings: in.Findings,
		Errors:   in.Errors,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(job)
}
