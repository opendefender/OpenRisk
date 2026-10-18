// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// rosterRepo answers GetMember from a fixed roster keyed by (tenant, user),
// the way GormMembershipRepository does: nil, nil when there is no row.
// Every other method panics through the nil embedded interface.
type rosterRepo struct {
	domain.MembershipRepository
	rows map[[2]uuid.UUID]*domain.OrganizationMember
}

func (r rosterRepo) GetMember(_ context.Context, tenantID, userID uuid.UUID) (*domain.OrganizationMember, error) {
	return r.rows[[2]uuid.UUID{tenantID, userID}], nil
}

func member(userActive, memberActive bool, email string) *domain.OrganizationMember {
	return &domain.OrganizationMember{
		IsActive: memberActive,
		User:     &domain.User{Email: email, FullName: "N", Locale: "en", IsActive: userActive},
	}
}

func TestNoticeDirectory_Success(t *testing.T) {
	tenant, user := uuid.New(), uuid.New()
	dir := noticeDirectory{members: memberDirectory{repo: rosterRepo{rows: map[[2]uuid.UUID]*domain.OrganizationMember{
		{tenant, user}: member(true, true, "a@x.example"),
	}}}}

	r, err := dir.NoticeRecipient(context.Background(), tenant, user)
	require.NoError(t, err)
	require.NotNil(t, r)
	require.Equal(t, "a@x.example", r.Email)
	require.Equal(t, "en", r.Locale)
}

// A member of another tenant is nobody here.
func TestNoticeDirectory_NotFound(t *testing.T) {
	tenantA, tenantB, user := uuid.New(), uuid.New(), uuid.New()
	dir := noticeDirectory{members: memberDirectory{repo: rosterRepo{rows: map[[2]uuid.UUID]*domain.OrganizationMember{
		{tenantB, user}: member(true, true, "b@x.example"),
	}}}}

	r, err := dir.NoticeRecipient(context.Background(), tenantA, user)
	require.NoError(t, err)
	require.Nil(t, r, "a tenant-B member must not resolve inside tenant A")
}

// Deactivated members and deactivated accounts are not told anything.
func TestNoticeDirectory_Unauthorized(t *testing.T) {
	tenant := uuid.New()
	inactiveMember, inactiveUser, noUser := uuid.New(), uuid.New(), uuid.New()
	dir := noticeDirectory{members: memberDirectory{repo: rosterRepo{rows: map[[2]uuid.UUID]*domain.OrganizationMember{
		{tenant, inactiveMember}: member(true, false, "m@x.example"),
		{tenant, inactiveUser}:   member(false, true, "u@x.example"),
		{tenant, noUser}:         {IsActive: true},
	}}}}

	for _, id := range []uuid.UUID{inactiveMember, inactiveUser, noUser} {
		r, err := dir.NoticeRecipient(context.Background(), tenant, id)
		require.NoError(t, err)
		require.Nil(t, r)
	}
}

func TestNoticeLink(t *testing.T) {
	id := uuid.New()
	require.Equal(t, "/risks/mitigations/"+id.String(), noticeLink("mitigation", &id))
	require.Contains(t, noticeLink("risk", &id), id.String())
	require.Equal(t, "", noticeLink("risk", nil))
	require.Equal(t, "", noticeLink("vendor_assessment", &id), "no page without a lookup")
}
