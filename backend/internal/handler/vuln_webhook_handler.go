// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package handler

import (
	"encoding/json"
	"strings"

	"github.com/gofiber/fiber/v2"
	vulnapp "github.com/opendefender/openrisk/internal/application/vulnerability"
	"github.com/opendefender/openrisk/internal/domain"
)

// VulnWebhookHandler ingests scanner findings pushed to a per-integration webhook
// URL. It authenticates with the integration's opaque webhook token (NOT a user
// JWT) — the token itself carries the tenant identity — so it is mounted on `app`
// BEFORE the /api/v1 JWT gate, just like the scanner agent endpoints.
type VulnWebhookHandler struct {
	integrations domain.VulnIntegrationRepository
	ingest       *vulnapp.IngestUseCase
}

func NewVulnWebhookHandler(integrations domain.VulnIntegrationRepository, ingest *vulnapp.IngestUseCase) *VulnWebhookHandler {
	return &VulnWebhookHandler{integrations: integrations, ingest: ingest}
}

// webhookToken extracts the token from the query string, the X-Webhook-Token
// header, or a Bearer Authorization header (in that order).
func webhookToken(c *fiber.Ctx) string {
	if t := c.Query("token"); t != "" {
		return t
	}
	if t := c.Get("X-Webhook-Token"); t != "" {
		return t
	}
	if auth := c.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	return ""
}

// Ingest POST /api/v1/vulnerabilities/webhook/:source — token-authenticated push.
// Body is either {"findings":[...]} or a bare JSON array of findings.
func (h *VulnWebhookHandler) Ingest(c *fiber.Ctx) error {
	token := webhookToken(c)
	if token == "" {
		return c.Status(401).JSON(fiber.Map{"error": "missing webhook token"})
	}
	integ, err := h.integrations.GetIntegrationByWebhookToken(c.UserContext(), token)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "could not resolve webhook token"})
	}
	if integ == nil {
		// Uniform 401 whether the token is unknown, disabled, or webhook-off:
		// never leak which integrations exist.
		return c.Status(401).JSON(fiber.Map{"error": "invalid or disabled webhook token"})
	}

	// The :source path segment must match the token's integration (defence in depth).
	if src := c.Params("source"); src != "" && domain.VulnSource(src) != integ.Source {
		return c.Status(400).JSON(fiber.Map{"error": "source mismatch for this webhook"})
	}

	findings, err := parseWebhookFindings(c.Body())
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid payload", "details": err.Error()})
	}
	if len(findings) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "no findings in payload"})
	}

	res, err := h.ingest.Execute(c.UserContext(), integ.TenantID, vulnapp.IngestInput{
		Source:           integ.Source,
		Findings:         findings,
		AutoCreateRisk:   integ.AutoCreateRisk,
		AutoCreateTicket: integ.AutoCreateTicket,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(202).JSON(fiber.Map{
		"accepted": true,
		"source":   res.Source,
		"received": res.Received,
		"created":  res.Created,
		"updated":  res.Updated,
		"skipped":  res.Skipped,
	})
}

// parseWebhookFindings accepts {"findings":[...]}, {"results":[...]}, {"data":[...]},
// a bare array, or a single finding object.
func parseWebhookFindings(body []byte) ([]map[string]any, error) {
	body = []byte(strings.TrimSpace(string(body)))
	if len(body) == 0 {
		return nil, nil
	}
	switch body[0] {
	case '[':
		var arr []map[string]any
		if err := json.Unmarshal(body, &arr); err != nil {
			return nil, err
		}
		return arr, nil
	case '{':
		// Try a wrapper object with a findings array first.
		var wrap map[string]json.RawMessage
		if err := json.Unmarshal(body, &wrap); err != nil {
			return nil, err
		}
		for _, key := range []string{"findings", "results", "data", "vulnerabilities", "items"} {
			if raw, ok := wrap[key]; ok {
				var arr []map[string]any
				if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
					return arr, nil
				}
			}
		}
		// Otherwise treat the whole object as a single finding.
		var single map[string]any
		if err := json.Unmarshal(body, &single); err != nil {
			return nil, err
		}
		return []map[string]any{single}, nil
	default:
		return nil, nil
	}
}
