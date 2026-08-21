// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/membership"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
)

// OrganizationMemberHandler serves organization member administration.
//
// THE TENANT IS NEVER IN THE REQUEST. Every handler here reads the
// organization from middleware.GetContext — the session the caller
// authenticated with. There is no path segment, query parameter or body field
// that names an organization, so there is nothing for a caller to change. The
// acceptance endpoints are the exception and take no organization at all: it
// comes from the invitation the token resolves to.
type OrganizationMemberHandler struct {
	svc *membership.Service
}

// NewOrganizationMemberHandler builds the handler.
func NewOrganizationMemberHandler(svc *membership.Service) *OrganizationMemberHandler {
	return &OrganizationMemberHandler{svc: svc}
}

// locale reads the preferred language for outbound mail: an explicit body or
// query value, else the Accept-Language header, defaulting to French.
func locale(c *fiber.Ctx, explicit string) string {
	v := strings.ToLower(strings.TrimSpace(explicit))
	if v == "" {
		v = strings.ToLower(strings.TrimSpace(c.Query("locale")))
	}
	if v == "" && strings.HasPrefix(strings.ToLower(c.Get("Accept-Language")), "en") {
		v = "en"
	}
	if v != "en" {
		return "fr"
	}
	return "en"
}

func intQuery(c *fiber.Ctx, key string, def int) int {
	v, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return def
	}
	return v
}

// ---------------------------------------------------------------------------
// Members
// ---------------------------------------------------------------------------

// ListMembers — GET /organization/members
func (h *OrganizationMemberHandler) ListMembers(c *fiber.Ctx) error {
	page, err := h.svc.ListMembers(c.UserContext(), tenantID(c), domain.MemberQuery{
		Search:   c.Query("q"),
		Status:   domain.MembershipStatus(c.Query("status")),
		Role:     domain.MemberRole(c.Query("role")),
		Limit:    intQuery(c, "limit", 50),
		Offset:   intQuery(c, "offset", 0),
		SortBy:   c.Query("sort_by"),
		SortDesc: c.Query("sort_dir") == "desc",
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(page)
}

// GetMember — GET /organization/members/:memberId
func (h *OrganizationMemberHandler) GetMember(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("memberId"))
	if err != nil {
		// A malformed id answers exactly like an id that does not exist, so the
		// shape of the response cannot be used to tell real ids from invented
		// ones.
		return writeAppError(c, domain.NewNotFoundError("member", c.Params("memberId")))
	}
	view, err := h.svc.GetMember(c.UserContext(), tenantID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(view)
}

// UpdateMemberRole — PUT /organization/members/:memberId/role
func (h *OrganizationMemberHandler) UpdateMemberRole(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("memberId"))
	if err != nil {
		return writeAppError(c, domain.NewNotFoundError("member", c.Params("memberId")))
	}
	var body struct {
		Role domain.MemberRole `json:"role"`
		// A pointer distinguishes "clear the business role" from "the form did
		// not carry that field", so saving a partial form cannot silently strip
		// the preset a member depends on.
		BusinessRole *domain.BusinessRoleKey `json:"business_role"`
	}
	if err := c.BodyParser(&body); err != nil {
		return writeAppError(c, domain.NewValidationError("invalid request body"))
	}
	in := membership.ChangeRoleInput{ActorID: userID(c), MemberID: id, Role: body.Role}
	if body.BusinessRole != nil {
		in.BusinessRole, in.BusinessRoleSet = *body.BusinessRole, true
	}
	view, err := h.svc.ChangeRole(c.UserContext(), tenantID(c), in)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(view)
}

// UpdateMemberStatus — PUT /organization/members/:memberId/status
func (h *OrganizationMemberHandler) UpdateMemberStatus(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("memberId"))
	if err != nil {
		return writeAppError(c, domain.NewNotFoundError("member", c.Params("memberId")))
	}
	var body struct {
		Status domain.MembershipStatus `json:"status"`
		Reason string                  `json:"reason"`
	}
	if err := c.BodyParser(&body); err != nil {
		return writeAppError(c, domain.NewValidationError("invalid request body"))
	}
	view, err := h.svc.SetStatus(c.UserContext(), tenantID(c), membership.SetStatusInput{
		ActorID: userID(c), MemberID: id, Status: body.Status, Reason: strings.TrimSpace(body.Reason),
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(view)
}

// ---------------------------------------------------------------------------
// Invitations
// ---------------------------------------------------------------------------

// ListInvitations — GET /organization/invitations
func (h *OrganizationMemberHandler) ListInvitations(c *fiber.Ctx) error {
	page, err := h.svc.ListInvitations(c.UserContext(), tenantID(c), domain.InvitationQuery{
		Status: domain.InvitationStatus(c.Query("status")),
		Email:  c.Query("email"),
		Limit:  intQuery(c, "limit", 50),
		Offset: intQuery(c, "offset", 0),
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(page)
}

// CreateInvitation — POST /organization/invitations
func (h *OrganizationMemberHandler) CreateInvitation(c *fiber.Ctx) error {
	var body struct {
		Email        string                 `json:"email"`
		Role         domain.MemberRole      `json:"role"`
		BusinessRole domain.BusinessRoleKey `json:"business_role"`
		Locale       string                 `json:"locale"`
	}
	if err := c.BodyParser(&body); err != nil {
		return writeAppError(c, domain.NewValidationError("invalid request body"))
	}
	res, err := h.svc.Invite(c.UserContext(), tenantID(c), membership.InviteInput{
		ActorID: userID(c), Email: body.Email, Role: body.Role,
		BusinessRole: body.BusinessRole, Locale: locale(c, body.Locale),
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// ResendInvitation — POST /organization/invitations/:invitationId/resend
func (h *OrganizationMemberHandler) ResendInvitation(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("invitationId"))
	if err != nil {
		return writeAppError(c, domain.NewNotFoundError("invitation", c.Params("invitationId")))
	}
	var body struct {
		Locale string `json:"locale"`
	}
	_ = c.BodyParser(&body) // an empty body is fine; the locale falls back
	res, err := h.svc.Resend(c.UserContext(), tenantID(c), membership.ResendInput{
		ActorID: userID(c), InvitationID: id, Locale: locale(c, body.Locale),
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(res)
}

// RevokeInvitation — DELETE /organization/invitations/:invitationId
func (h *OrganizationMemberHandler) RevokeInvitation(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("invitationId"))
	if err != nil {
		return writeAppError(c, domain.NewNotFoundError("invitation", c.Params("invitationId")))
	}
	view, err := h.svc.RevokeInvitation(c.UserContext(), tenantID(c), userID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(view)
}

// ---------------------------------------------------------------------------
// Acceptance — unauthenticated or self-authenticated, never tenant-scoped
// ---------------------------------------------------------------------------

// PreviewInvitation — GET /invitations/preview?token=…
//
// Public: the holder of a link needs to know what they are being asked to join
// before signing up. It reveals only the organization's name, the invited
// address, the role and the expiry — never anything about the organization's
// contents, its size or its other members.
func (h *OrganizationMemberHandler) PreviewInvitation(c *fiber.Ctx) error {
	view, err := h.svc.PreviewInvitation(c.UserContext(), c.Query("token"))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(view)
}

// AcceptInvitation — POST /invitations/accept
//
// Mounted outside the authenticated group so an invitee with no account can
// redeem their link. When a session IS present the middleware has already
// stamped it, and the use case requires the session's address to match the
// invited one.
func (h *OrganizationMemberHandler) AcceptInvitation(c *fiber.Ctx) error {
	var body struct {
		Token    string `json:"token"`
		FullName string `json:"full_name"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&body); err != nil {
		return writeAppError(c, domain.NewValidationError("invalid request body"))
	}
	res, err := h.svc.AcceptInvitation(c.UserContext(), membership.AcceptInput{
		Token:    body.Token,
		UserID:   userID(c), // uuid.Nil when unauthenticated
		FullName: body.FullName,
		Password: body.Password,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// ---------------------------------------------------------------------------
// Organization profile, counts and membership audit
// ---------------------------------------------------------------------------

// GetOrganization — GET /organization
func (h *OrganizationMemberHandler) GetOrganization(c *fiber.Ctx) error {
	view, err := h.svc.GetOrganization(c.UserContext(), tenantID(c), isOrgAdmin(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(view)
}

// GetCounts — GET /organization/counts
//
// What the sidebar reads. Available to any authenticated member: knowing how
// many colleagues you have is not privileged, and gating it would leave the
// badge permanently empty for everyone who is not an administrator.
func (h *OrganizationMemberHandler) GetCounts(c *fiber.Ctx) error {
	counts, err := h.svc.Counts(c.UserContext(), tenantID(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(counts)
}

// GetMembershipAudit — GET /organization/members/audit
func (h *OrganizationMemberHandler) GetMembershipAudit(c *fiber.Ctx) error {
	f := domain.AuditEventFilter{
		EntityType: c.Query("entity_type"),
		EntityID:   c.Query("entity_id"),
		Action:     c.Query("action"),
		Search:     c.Query("q"),
		Limit:      intQuery(c, "limit", 50),
		Offset:     intQuery(c, "offset", 0),
	}
	page, err := h.svc.MembershipAudit(c.UserContext(), tenantID(c), f)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(page)
}

// isOrgAdmin reports whether the caller may edit the organization profile. It
// reads the same claims the route guards do, so the UI's read-only rendering
// and the API's refusal come from one answer.
func isOrgAdmin(c *fiber.Ctx) bool {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return false
	}
	if claims.HasPermission("*") || claims.HasPermission("organization:update") {
		return true
	}
	for _, role := range claims.OrgRoles {
		if role == string(domain.RoleAdmin) || role == string(domain.RoleRoot) {
			return true
		}
	}
	return false
}
