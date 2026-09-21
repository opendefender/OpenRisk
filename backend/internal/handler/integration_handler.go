// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	vulnapp "github.com/opendefender/openrisk/internal/application/vulnerability"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/pkg/netguard"
)

// TestIntegrationInput is the optional body of POST /integrations/:id/test.
//
// APIUrl is only used when the stored integration has no base_url of its own
// (the connector then defaults to the vendor's cloud endpoint). It is validated
// like any other outbound target and can never override a stored base_url.
type TestIntegrationInput struct {
	APIUrl string `json:"api_url"`
	APIKey string `json:"api_key"`
}

type IntegrationTestResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Status    int    `json:"status"`
	Target    string `json:"target,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Timestamp string `json:"timestamp"`
}

// IntegrationTestHandler probes a configured integration's endpoint (#573).
//
// The target comes from the integration named by :id, looked up in the
// caller's tenant. Every target goes through pkg/netguard, redirects are not
// followed, and the remote response body is never returned — only the status
// and a reason — so the endpoint cannot be used to read internal services.
type IntegrationTestHandler struct {
	get   *vulnapp.GetIntegrationUseCase
	probe *http.Client
	audit func(*domain.AuditLog)
}

func NewIntegrationTestHandler(get *vulnapp.GetIntegrationUseCase) *IntegrationTestHandler {
	return &IntegrationTestHandler{
		get:   get,
		probe: netguard.Client(netguard.Options{Timeout: 10 * time.Second, NoRedirects: true}),
		audit: func(l *domain.AuditLog) { _ = auditService.LogAction(l) },
	}
}

// TestIntegration POST /integrations/:id/test — admin/root only (#529).
func (h *IntegrationTestHandler) TestIntegration(c *fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	rc := middleware.GetContext(c)
	if claims == nil || rc == nil || rc.OrganizationID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid integration ID"})
	}

	var input TestIntegrationInput
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
		}
	}

	// A denied URL in the body is refused outright, before any lookup or request.
	if input.APIUrl != "" {
		if err := netguard.ValidateURL(input.APIUrl); err != nil {
			return h.reject(c, claims.Sub, &id, input.APIUrl, err)
		}
	}

	integ, err := h.get.Execute(c.UserContext(), rc.OrganizationID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Integration not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to load integration"})
	}

	target := integ.BaseURL
	if target == "" {
		target = input.APIUrl
	}
	if target == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Integration has no base_url to test"})
	}
	// Stored rows predating the guard are re-checked here.
	if err := netguard.ValidateURL(target); err != nil {
		return h.reject(c, claims.Sub, &integ.ID, target, err)
	}

	resp := h.run(c.UserContext(), target, input.APIKey)
	result := domain.ResultSuccess
	code := fiber.StatusOK
	if !resp.Success {
		result = domain.ResultFailure
		code = fiber.StatusBadRequest
	}
	h.record(c, claims.Sub, &integ.ID, result, resp.Target, resp.Reason)
	return c.Status(code).JSON(resp)
}

// run issues the probe and reduces the outcome to a status and a reason. The
// response body is drained and discarded, never read into the reply.
func (h *IntegrationTestHandler) run(ctx context.Context, target, apiKey string) IntegrationTestResponse {
	out := IntegrationTestResponse{Target: redact(target), Timestamp: time.Now().UTC().Format(time.RFC3339)}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		out.Message, out.Reason = "Invalid API URL", "invalid url"
		return out
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "OpenRisk/1.0")

	// Never follow redirects, whatever client was injected.
	client := *h.probe
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	resp, err := client.Do(req)
	if err != nil {
		out.Message, out.Reason = "Failed to connect to API", transportReason(err)
		return out
	}
	resp.Body.Close()

	out.Status = resp.StatusCode
	out.Reason = http.StatusText(resp.StatusCode)
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		out.Success, out.Message = true, "Integration test successful"
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		out.Message, out.Reason = "Integration test failed", "redirect not followed"
	default:
		out.Message = "Integration test failed"
	}
	return out
}

// reject answers 400 for a target the guard refuses, and audits the attempt.
func (h *IntegrationTestHandler) reject(c *fiber.Ctx, user uuid.UUID, id *uuid.UUID, target string, err error) error {
	h.record(c, user, id, domain.ResultFailure, redact(target), "target not allowed")
	return c.Status(fiber.StatusBadRequest).JSON(IntegrationTestResponse{
		Message:   err.Error(),
		Target:    redact(target),
		Reason:    "target not allowed",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// record writes the audit event with the probed target, so an abuse attempt
// is visible after the fact.
func (h *IntegrationTestHandler) record(c *fiber.Ctx, user uuid.UUID, id *uuid.UUID, result domain.AuditLogResult, target, reason string) {
	h.audit(&domain.AuditLog{
		TenantID:     auditTenant(c),
		UserID:       &user,
		Action:       domain.ActionIntegrationTest,
		Resource:     domain.ResourceIntegration,
		ResourceID:   id,
		Result:       result,
		ErrorMessage: "target=" + target + " reason=" + reason,
		IPAddress:    parseIPAddressHelper(c.IP()),
		UserAgent:    c.Get("User-Agent"),
	})
}

// transportReason classifies a transport error without echoing its text.
func transportReason(err error) string {
	var nerr interface{ Timeout() bool }
	switch {
	case netguard.IsDenied(err):
		return "target not allowed"
	case errors.As(err, &nerr) && nerr.Timeout():
		return "timeout"
	default:
		return "connection failed"
	}
}

// redact strips userinfo and the query string, which may carry secrets.
func redact(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "invalid url"
	}
	u.User, u.RawQuery, u.Fragment = nil, "", ""
	return u.String()
}
