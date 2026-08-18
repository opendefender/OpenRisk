// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"

	billingapp "github.com/opendefender/openrisk/internal/application/billing"
	"github.com/opendefender/openrisk/internal/domain"
	pkgbilling "github.com/opendefender/openrisk/pkg/billing"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
)

// BillingHandler exposes the self-service billing surface: current subscription +
// usage, start a no-card trial, open a checkout, change plan, cancel.
type BillingHandler struct {
	svc      *billingapp.Service
	gateways *pkgbilling.Registry
}

// NewBillingHandler builds the handler.
func NewBillingHandler(svc *billingapp.Service, gateways *pkgbilling.Registry) *BillingHandler {
	return &BillingHandler{svc: svc, gateways: gateways}
}

// GetBilling handles GET /billing → subscription + invoices + configured providers.
func (h *BillingHandler) GetBilling(c *fiber.Ctx) error {
	sub, invoices, err := h.svc.Get(c.UserContext(), tenantID(c))
	if err != nil {
		return writeAppError(c, err)
	}
	providers := []string{}
	if h.gateways != nil {
		for _, p := range h.gateways.Configured() {
			providers = append(providers, string(p))
		}
	}
	return c.JSON(fiber.Map{
		"subscription":         sub,
		"invoices":             invoices,
		"configured_providers": providers,
		"trial_days":           ent.TrialDays,
	})
}

type planInput struct {
	Plan     string `json:"plan"`
	Region   string `json:"region"`
	Provider string `json:"provider"`
}

// StartTrial handles POST /billing/trial — 14-day, no-credit-card trial.
func (h *BillingHandler) StartTrial(c *fiber.Ctx) error {
	var in planInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	sub, err := h.svc.StartTrial(c.UserContext(), tenantID(c), in.Plan, in.Region)
	if err != nil {
		return billingError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(sub)
}

// Checkout handles POST /billing/checkout — opens a hosted payment session.
func (h *BillingHandler) Checkout(c *fiber.Ctx) error {
	var in planInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	email := ""
	sess, err := h.svc.Checkout(c.UserContext(), tenantID(c), in.Plan, in.Region, email, pkgbilling.Provider(in.Provider))
	if err != nil {
		return billingError(c, err)
	}
	return c.JSON(sess)
}

// ChangePlan handles POST /billing/change-plan — admin/manual plan application
// (e.g. Enterprise agreed by sales, or a downgrade to Free). Guarded to admin at
// the route.
func (h *BillingHandler) ChangePlan(c *fiber.Ctx) error {
	var in planInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	sub, err := h.svc.ApplyPlan(c.UserContext(), tenantID(c), in.Plan, in.Region, domain.ProviderManual, "")
	if err != nil {
		return billingError(c, err)
	}
	return c.JSON(sub)
}

// Cancel handles POST /billing/cancel.
func (h *BillingHandler) Cancel(c *fiber.Ctx) error {
	sub, err := h.svc.Cancel(c.UserContext(), tenantID(c))
	if err != nil {
		return billingError(c, err)
	}
	return c.JSON(sub)
}

// billingError maps billing service errors to sensible HTTP statuses.
func billingError(c *fiber.Ctx, err error) error {
	switch err {
	case billingapp.ErrInvalidPlan:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	case billingapp.ErrAlreadySubscribed:
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	case billingapp.ErrNoGateway:
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Aucun moyen de paiement n'est configuré sur cette instance. Contactez OpenRisk pour activer votre plan.",
			"code":  "no_gateway",
		})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
}
