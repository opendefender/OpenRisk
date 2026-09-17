// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package profile holds the self-service use cases of a signed-in person: read
// and edit their own profile and preferences, and manage their avatar (#719).
//
// SCOPE. Every write takes the caller's user id from the session and nothing
// else, so no input can select another account. users has no tenant_id of its
// own (people span organizations); reading somebody ELSE's avatar is gated
// through the parent entity instead — an access-granting membership of the
// target in the caller's tenant — and a foreign user answers exactly like a
// missing one.
package profile

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// UserStore is the slice of the user store this package needs.
type UserStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	UpdateUserColumns(ctx context.Context, userID uuid.UUID, columns map[string]interface{}) error
}

// MembershipReader resolves a person's membership in an organization.
type MembershipReader interface {
	GetOrganizationMember(ctx context.Context, userID, orgID uuid.UUID) (*domain.OrganizationMember, error)
}

// OrganizationReader resolves the caller's organization, for its defaults.
type OrganizationReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error)
}

// BlobStore persists avatar bytes. Satisfied by pkg/storage.Storage.
type BlobStore interface {
	Save(ctx context.Context, tenantID uuid.UUID, filename string, content io.Reader) (string, error)
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// AuditSink records a profile event. Best-effort and nil-safe.
type AuditSink interface {
	Record(ctx context.Context, ev domain.AuditEvent)
}

// Effective is what the interface should actually use: the person's choice,
// else the organization's default, else empty (the client's own fallback).
type Effective struct {
	Timezone   string `json:"timezone,omitempty"`
	Locale     string `json:"locale,omitempty"`
	DateFormat string `json:"date_format,omitempty"`
}

// View is the caller's own profile. An allowlist: a column added to users
// tomorrow does not reach this response by default.
type View struct {
	ID         uuid.UUID `json:"id"`
	Email      string    `json:"email"`
	Username   string    `json:"username"`
	FullName   string    `json:"full_name"`
	JobTitle   string    `json:"job_title"`
	Phone      string    `json:"phone"`
	Bio        string    `json:"bio"`
	Timezone   string    `json:"timezone"`
	Locale     string    `json:"locale"`
	DateFormat string    `json:"date_format"`
	ThemeMode  string    `json:"theme_mode"`
	HasAvatar  bool      `json:"has_avatar"`
	// AvatarURL is set only for an uploaded avatar, and always points at this
	// API — never at an address somebody typed.
	AvatarURL string    `json:"avatar_url,omitempty"`
	Effective Effective `json:"effective"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AvatarPath is the API path serving a user's avatar.
func AvatarPath(userID uuid.UUID) string { return "/api/v1/users/" + userID.String() + "/avatar" }

// Service is the profile use cases.
type Service struct {
	users   UserStore
	members MembershipReader
	orgs    OrganizationReader
	blobs   BlobStore
	audit   AuditSink
}

// NewService builds the service. orgs, blobs and audit are optional: without
// orgs there are no organization defaults, without blobs avatar calls fail
// closed, without audit nothing is journalled.
func NewService(users UserStore, members MembershipReader) *Service {
	return &Service{users: users, members: members}
}

func (s *Service) WithOrganizations(o OrganizationReader) *Service { s.orgs = o; return s }
func (s *Service) WithBlobStore(b BlobStore) *Service              { s.blobs = b; return s }
func (s *Service) WithAudit(a AuditSink) *Service                  { s.audit = a; return s }

// loadSelf reads the caller's own row.
func (s *Service) loadSelf(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	if userID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no user in session")
	}
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, domain.NewNotFoundError("user", userID)
	}
	return u, nil
}

// view builds the response, resolving effective preferences against the
// caller's organization.
func (s *Service) view(ctx context.Context, tenantID uuid.UUID, u *domain.User) *View {
	v := &View{
		ID: u.ID, Email: u.Email, Username: u.Username, FullName: u.FullName,
		JobTitle: u.Department, Phone: u.Phone, Bio: u.Bio,
		Timezone: u.Timezone, Locale: u.Locale, DateFormat: u.DateFormat, ThemeMode: u.ThemeMode,
		HasAvatar: u.AvatarKey != "", UpdatedAt: u.UpdatedAt,
	}
	if v.HasAvatar {
		v.AvatarURL = AvatarPath(u.ID)
	}
	var orgSettings map[string]interface{}
	if s.orgs != nil && tenantID != uuid.Nil {
		if org, err := s.orgs.GetByID(ctx, tenantID); err == nil && org != nil {
			orgSettings = org.GetSettings()
		}
	}
	pick := func(own, key string) string {
		if own != "" {
			return own
		}
		d, _ := orgSettings[key].(string)
		return strings.TrimSpace(d)
	}
	v.Effective = Effective{
		Timezone:   pick(u.Timezone, domain.OrgSettingTimezone),
		Locale:     pick(u.Locale, domain.OrgSettingDefaultLocale),
		DateFormat: pick(u.DateFormat, domain.OrgSettingDateFormat),
	}
	return v
}

func (s *Service) record(ctx context.Context, tenantID, userID uuid.UUID, summary string, after domain.JSONMap) {
	if s.audit == nil || tenantID == uuid.Nil {
		return
	}
	actor := userID
	s.audit.Record(ctx, domain.AuditEvent{
		TenantID: tenantID, ActorID: &actor, Action: domain.AuditActionUpdate,
		EntityType: "user", EntityID: userID.String(), Summary: summary, After: after,
	})
}
