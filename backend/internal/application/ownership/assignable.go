// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package ownership backs the generalised owner / assignee / reviewer model:
// who may be assigned, whether a given user may be, and who gets told about it.
//
// It is the server side of the <UserPicker> component. The picker is only
// trustworthy if the list it shows is the list the server will actually accept —
// so the same use case answers "who can I pick" and the same resolver decides
// "may this id be written", rather than the UI filtering a list the API would
// have accepted anyway.
package ownership

import (
	"context"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// MemberRepository is the narrow port this package needs. It is satisfied
// structurally by repository.GormMemberRBACRepository (User preloaded), the same
// one application/rbac uses — no new persistence code.
type MemberRepository interface {
	ListMembers(ctx context.Context, tenantID uuid.UUID) ([]domain.OrganizationMember, error)
	GetMember(ctx context.Context, tenantID, userID uuid.UUID) (*domain.OrganizationMember, error)
}

// Assignee is one pickable row.
type Assignee struct {
	UserID       uuid.UUID              `json:"user_id"`
	Email        string                 `json:"email"`
	FullName     string                 `json:"full_name"`
	Initials     string                 `json:"initials"`
	OrgRole      domain.MemberRole      `json:"org_role"`
	BusinessRole domain.BusinessRoleKey `json:"business_role,omitempty"`
	// BusinessRoleLabel is the human label of the preset, so the picker can group
	// by job role without shipping the catalogue to the client.
	BusinessRoleLabel string `json:"business_role_label,omitempty"`
	IsActive          bool   `json:"is_active"`
	// CanAct answers the permission filter: does this member actually hold the
	// permission the caller asked for? A picker that offers someone who cannot
	// do the work is a dead control with extra steps.
	CanAct bool `json:"can_act"`
}

// AssignableGroup is a role-shaped bucket of members. The spec asks for
// "groupes/rôles assignables": a tenant assigns work to "the RSSI" as often as
// to a named person, and this lets the picker offer both without a second API.
type AssignableGroup struct {
	Key     string      `json:"key"`   // business role key, or "admins"
	Label   string      `json:"label"` // localized label
	Members []uuid.UUID `json:"members"`
	Count   int         `json:"count"`
}

// AssignableResult is what GET /ownership/assignable returns.
type AssignableResult struct {
	Users  []Assignee        `json:"users"`
	Groups []AssignableGroup `json:"groups"`
	// Permission echoes the filter that was applied (empty = none), so the UI can
	// explain why a name is missing instead of just not showing it.
	Permission string `json:"permission,omitempty"`
}

// ListAssignableInput narrows the picker.
type ListAssignableInput struct {
	// Search matches email or full name, case-insensitively. Empty returns all.
	Search string
	// Permission, when set, marks members holding it as CanAct. Members who do
	// not hold it are still returned (with CanAct false) unless OnlyCapable.
	Permission string
	// OnlyCapable drops members who cannot act — used when the caller wants a
	// hard filter rather than a visual hint.
	OnlyCapable bool
	// IncludeInactive returns deactivated members too (default: excluded — you
	// should not be able to assign work to a disabled account).
	IncludeInactive bool
	// Locale drives group labels ("fr" | "en"). Defaults to fr.
	Locale string
}

// ListAssignableUseCase lists the users of a tenant who can be given work.
type ListAssignableUseCase struct {
	members MemberRepository
}

func NewListAssignableUseCase(members MemberRepository) *ListAssignableUseCase {
	return &ListAssignableUseCase{members: members}
}

// Execute returns the tenant's assignable users and role groups.
func (uc *ListAssignableUseCase) Execute(ctx context.Context, tenantID uuid.UUID, in ListAssignableInput) (*AssignableResult, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewValidationError("tenant is required")
	}
	rows, err := uc.members.ListMembers(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	needle := strings.ToLower(strings.TrimSpace(in.Search))
	result := &AssignableResult{
		Users:      make([]Assignee, 0, len(rows)),
		Groups:     make([]AssignableGroup, 0, 4),
		Permission: in.Permission,
	}
	groups := map[string]*AssignableGroup{}

	for i := range rows {
		m := rows[i]
		if !m.IsActive && !in.IncludeInactive {
			continue
		}
		a := Assignee{
			UserID:       m.UserID,
			OrgRole:      m.Role,
			BusinessRole: m.BusinessRole,
			IsActive:     m.IsActive,
		}
		if m.User != nil {
			a.Email = m.User.Email
			a.FullName = m.User.FullName
		}
		a.Initials = initialsOf(a.FullName, a.Email)

		if needle != "" &&
			!strings.Contains(strings.ToLower(a.Email), needle) &&
			!strings.Contains(strings.ToLower(a.FullName), needle) {
			continue
		}

		a.CanAct = in.Permission == "" || holdsPermission(m.EffectivePermissions(), in.Permission)
		if in.OnlyCapable && !a.CanAct {
			continue
		}

		if preset, ok := domain.GetBusinessRole(m.BusinessRole); ok {
			a.BusinessRoleLabel = presetLabel(preset, in.Locale)
		}

		result.Users = append(result.Users, a)

		key, label := groupOf(m, in.Locale)
		if key != "" {
			g, ok := groups[key]
			if !ok {
				g = &AssignableGroup{Key: key, Label: label}
				groups[key] = g
			}
			g.Members = append(g.Members, m.UserID)
			g.Count++
		}
	}

	// Stable, human order: name first, then email for the nameless.
	sort.SliceStable(result.Users, func(i, j int) bool {
		a, b := result.Users[i], result.Users[j]
		ka := strings.ToLower(a.FullName)
		if ka == "" {
			ka = strings.ToLower(a.Email)
		}
		kb := strings.ToLower(b.FullName)
		if kb == "" {
			kb = strings.ToLower(b.Email)
		}
		return ka < kb
	})

	for _, g := range groups {
		result.Groups = append(result.Groups, *g)
	}
	sort.Slice(result.Groups, func(i, j int) bool { return result.Groups[i].Key < result.Groups[j].Key })

	return result, nil
}

// groupOf buckets a member by business role, falling back to "admins" for
// members who hold the wildcard instead of a preset.
func groupOf(m domain.OrganizationMember, locale string) (string, string) {
	if m.BusinessRole != "" {
		if preset, ok := domain.GetBusinessRole(m.BusinessRole); ok {
			return string(m.BusinessRole), presetLabel(preset, locale)
		}
		return string(m.BusinessRole), string(m.BusinessRole)
	}
	if m.IsAdmin() {
		if strings.HasPrefix(strings.ToLower(locale), "en") {
			return "admins", "Administrators"
		}
		return "admins", "Administrateurs"
	}
	return "", ""
}

// holdsPermission applies the same wildcard semantics as
// middleware.RequirePermission: "*" covers everything and "risks:*" covers
// "risks:read". The picker must agree with the guard, or it lies.
func holdsPermission(held []string, required string) bool {
	for _, p := range held {
		if p == required || p == "*" {
			return true
		}
		if strings.HasSuffix(p, ":*") && strings.HasPrefix(required, strings.TrimSuffix(p, "*")) {
			return true
		}
	}
	return false
}

// presetLabel picks the locale-appropriate label of a business-role preset.
func presetLabel(p domain.BusinessRole, locale string) string {
	if strings.HasPrefix(strings.ToLower(locale), "en") && p.LabelEN != "" {
		return p.LabelEN
	}
	if p.LabelFR != "" {
		return p.LabelFR
	}
	return string(p.Key)
}

func initialsOf(fullName, email string) string {
	src := strings.TrimSpace(fullName)
	if src == "" {
		src = strings.TrimSpace(email)
	}
	if src == "" {
		return "?"
	}
	parts := strings.Fields(src)
	if len(parts) >= 2 {
		return strings.ToUpper(string([]rune(parts[0])[:1]) + string([]rune(parts[1])[:1]))
	}
	r := []rune(src)
	if len(r) >= 2 {
		return strings.ToUpper(string(r[:2]))
	}
	return strings.ToUpper(string(r))
}
