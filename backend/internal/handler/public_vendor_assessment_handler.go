// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/opendefender/openrisk/internal/application/tprm"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
)

// VendorAssessmentTokenHeader carries the questionnaire token (ADR 0004 D4).
// Never a path segment or a query parameter: those are recorded by every access
// log and proxy, and leak through Referer.
const VendorAssessmentTokenHeader = "X-Vendor-Assessment-Token"

const (
	// publicTokenRequestsPerHour caps all requests presenting the same token.
	publicTokenRequestsPerHour = 300
	// publicSubmitsPerHour caps submissions presenting the same token.
	publicSubmitsPerHour = 10
)

// PrefixedRateLimitBackend namespaces a shared counter store.
//
// middleware.RateLimit keys its counter by the raw client IP, and the Redis
// store is shared by every limiter in the process. Without a prefix, the
// questionnaire's 60/min counter and the login throttle's 15/5min counter would
// be the SAME key, each consuming the other's budget.
type PrefixedRateLimitBackend struct {
	Prefix string
	Inner  middleware.RateLimitBackend
}

// IsAllowed consults the inner store under the prefixed key.
func (p PrefixedRateLimitBackend) IsAllowed(key string, maxRequests int, window time.Duration) bool {
	return p.Inner.IsAllowed(p.Prefix+key, maxRequests, window)
}

// PublicVendorAssessmentHandler serves the questionnaire to a vendor contact who
// holds no account (#670, ADR 0004 D4). No auth middleware runs in front of it;
// the token IS the credential, and the tenant comes from the row it resolves to.
type PublicVendorAssessmentHandler struct {
	resolve *tprm.ResolvePublicAssessmentUseCase
	save    *tprm.SavePublicAnswersUseCase
	submit  *tprm.SubmitPublicAssessmentUseCase
	limiter middleware.RateLimitBackend
}

func NewPublicVendorAssessmentHandler(deps tprm.AssessmentDeps, limiter middleware.RateLimitBackend) *PublicVendorAssessmentHandler {
	return &PublicVendorAssessmentHandler{
		resolve: tprm.NewResolvePublicAssessmentUseCase(deps),
		save:    tprm.NewSavePublicAnswersUseCase(deps),
		submit:  tprm.NewSubmitPublicAssessmentUseCase(deps),
		limiter: limiter,
	}
}

// presentedToken sets the response headers every public answer needs and
// applies the per-token limits. The limits are keyed by the token's HASH, so the
// counter store never holds a credential, and they run before any lookup, so
// guessing tokens is throttled too.
func (h *PublicVendorAssessmentHandler) presentedToken(c *fiber.Ctx, submitting bool) (string, error) {
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set("Referrer-Policy", "no-referrer")

	token := strings.TrimSpace(c.Get(VendorAssessmentTokenHeader))
	if h.limiter != nil {
		key := domain.HashVendorAssessmentToken(token)
		if !h.limiter.IsAllowed("requests:"+key, publicTokenRequestsPerHour, time.Hour) {
			return "", domain.NewRateLimitError("too many requests for this questionnaire link — try again later")
		}
		if submitting && !h.limiter.IsAllowed("submits:"+key, publicSubmitsPerHour, time.Hour) {
			return "", domain.NewRateLimitError("too many submissions for this questionnaire link — try again later")
		}
	}
	return token, nil
}

// Get — GET /api/v1/public/vendor-assessment
func (h *PublicVendorAssessmentHandler) Get(c *fiber.Ctx) error {
	token, err := h.presentedToken(c, false)
	if err != nil {
		return writeAppError(c, err)
	}
	view, err := h.resolve.Execute(c.UserContext(), token)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(view)
}

type saveAnswersBody struct {
	Answers []domain.VendorAnswerInput `json:"answers"`
}

// SaveAnswers — PUT /api/v1/public/vendor-assessment/answers
func (h *PublicVendorAssessmentHandler) SaveAnswers(c *fiber.Ctx) error {
	token, err := h.presentedToken(c, false)
	if err != nil {
		return writeAppError(c, err)
	}
	var body saveAnswersBody
	if err := c.BodyParser(&body); err != nil {
		return writeAppError(c, domain.NewValidationError("invalid request body"))
	}
	if len(body.Answers) > domain.MaxVendorQuestions {
		return writeAppError(c, domain.NewValidationError("too many answers"))
	}
	view, err := h.save.Execute(c.UserContext(), token, body.Answers)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(view)
}

// Submit — POST /api/v1/public/vendor-assessment/submit
func (h *PublicVendorAssessmentHandler) Submit(c *fiber.Ctx) error {
	token, err := h.presentedToken(c, true)
	if err != nil {
		return writeAppError(c, err)
	}
	view, err := h.submit.Execute(c.UserContext(), token)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(view)
}
