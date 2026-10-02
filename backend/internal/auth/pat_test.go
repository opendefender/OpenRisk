// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// memPATRepo is an in-memory PersonalAccessTokenRepository that applies the
// same tenant and owner predicates as the GORM one, so a service test fails if
// the service stops passing them.
type memPATRepo struct {
	rows map[uuid.UUID]*domain.PersonalAccessToken
}

func newMemPATRepo() *memPATRepo {
	return &memPATRepo{rows: map[uuid.UUID]*domain.PersonalAccessToken{}}
}

func (r *memPATRepo) Create(_ context.Context, t *domain.PersonalAccessToken) error {
	t.ID = uuid.New()
	cp := *t
	r.rows[t.ID] = &cp
	return nil
}

func (r *memPATRepo) GetByTokenHash(_ context.Context, hash string) (*domain.PersonalAccessToken, error) {
	for _, t := range r.rows {
		if t.TokenHash == hash {
			cp := *t
			return &cp, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *memPATRepo) ListByOwner(_ context.Context, tenantID, userID uuid.UUID) ([]*domain.PersonalAccessToken, error) {
	var out []*domain.PersonalAccessToken
	for _, t := range r.rows {
		if t.TenantID == tenantID && t.UserID == userID {
			cp := *t
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memPATRepo) UpdateLastUsed(_ context.Context, tenantID, id uuid.UUID) error {
	if t, ok := r.rows[id]; ok && t.TenantID == tenantID {
		now := time.Now()
		t.LastUsedAt = &now
	}
	return nil
}

func (r *memPATRepo) DeleteByOwner(_ context.Context, tenantID, userID, id uuid.UUID) (bool, error) {
	t, ok := r.rows[id]
	if !ok || t.TenantID != tenantID || t.UserID != userID {
		return false, nil
	}
	delete(r.rows, id)
	return true, nil
}

func TestCreateToken_Success(t *testing.T) {
	ctx := context.Background()
	svc := NewPersonalAccessTokenService(newMemPATRepo())
	user, tenant := uuid.New(), uuid.New()

	pat, raw, err := svc.CreateToken(ctx, user, tenant, "CI/CD", "", []string{"*"}, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.HasPrefix(raw, PATValuePrefix) {
		t.Fatalf("value %q does not start with %q", raw, PATValuePrefix)
	}
	if pat.TenantID != tenant || pat.UserID != user {
		t.Fatalf("token stored for user %s tenant %s, want %s %s", pat.UserID, pat.TenantID, user, tenant)
	}
	if strings.Contains(pat.TokenHash, strings.TrimPrefix(raw, PATValuePrefix)) {
		t.Fatal("the stored hash contains the raw secret")
	}

	got, err := svc.ValidateToken(ctx, raw)
	if err != nil {
		t.Fatalf("the value CreateToken returned does not validate: %v", err)
	}
	if got.ID != pat.ID || got.TenantID != tenant {
		t.Fatalf("validated the wrong token: %+v", got)
	}
}

func TestCreateToken_RequiresTenant(t *testing.T) {
	svc := NewPersonalAccessTokenService(newMemPATRepo())
	_, _, err := svc.CreateToken(context.Background(), uuid.New(), uuid.Nil, "x", "", []string{"*"}, nil)
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("got %v, want ErrValidation", err)
	}
}

// A token minted before #782 has no "orsk_" prefix and must keep working.
func TestValidateToken_LegacyFormat(t *testing.T) {
	ctx := context.Background()
	svc := NewPersonalAccessTokenService(newMemPATRepo())
	_, raw, err := svc.CreateToken(ctx, uuid.New(), uuid.New(), "old", "", []string{"*"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ValidateToken(ctx, strings.TrimPrefix(raw, PATValuePrefix)); err != nil {
		t.Fatalf("legacy form rejected: %v", err)
	}
}

func TestValidateToken_Unauthorized(t *testing.T) {
	ctx := context.Background()
	repo := newMemPATRepo()
	svc := NewPersonalAccessTokenService(repo)
	pat, raw, err := svc.CreateToken(ctx, uuid.New(), uuid.New(), "x", "", []string{"*"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	prefix, _, _ := SplitPATValue(raw)

	past := time.Now().Add(-time.Minute)
	repo.rows[pat.ID].ExpiresAt = &past
	expired := raw

	for name, value := range map[string]string{
		"empty":         "",
		"jwt":           "eyJhbGciOi.J9.sig",
		"unknown":       PATValuePrefix + prefix + "_" + strings.Repeat("0", 64),
		"wrong prefix":  PATValuePrefix + "deadbeef_" + strings.Split(raw, "_")[2],
		"expired":       expired,
		"missing parts": PATValuePrefix + prefix,
	} {
		if _, err := svc.ValidateToken(ctx, value); !errors.Is(err, domain.ErrUnauthorized) {
			t.Errorf("%s: got %v, want ErrUnauthorized", name, err)
		}
	}
}

func TestRevokeToken_Success(t *testing.T) {
	ctx := context.Background()
	svc := NewPersonalAccessTokenService(newMemPATRepo())
	user, tenant := uuid.New(), uuid.New()
	pat, raw, _ := svc.CreateToken(ctx, user, tenant, "x", "", []string{"*"}, nil)

	if err := svc.RevokeToken(ctx, tenant, pat.ID, user); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := svc.ValidateToken(ctx, raw); err == nil {
		t.Fatal("a revoked token still authenticates")
	}
}

func TestRevokeToken_NotFound(t *testing.T) {
	svc := NewPersonalAccessTokenService(newMemPATRepo())
	err := svc.RevokeToken(context.Background(), uuid.New(), uuid.New(), uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

// Another user, or the owner acting from another organization, cannot revoke
// the token, cannot list it, and learns nothing: the answer is ErrNotFound.
func TestRevokeToken_Unauthorized(t *testing.T) {
	ctx := context.Background()
	svc := NewPersonalAccessTokenService(newMemPATRepo())
	owner, tenantA, tenantB := uuid.New(), uuid.New(), uuid.New()
	pat, raw, _ := svc.CreateToken(ctx, owner, tenantA, "x", "", []string{"*"}, nil)

	if err := svc.RevokeToken(ctx, tenantA, pat.ID, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("other user: got %v, want ErrNotFound", err)
	}
	if err := svc.RevokeToken(ctx, tenantB, pat.ID, owner); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("owner from tenant B: got %v, want ErrNotFound", err)
	}
	if list, _ := svc.ListUserTokens(ctx, tenantB, owner); len(list) != 0 {
		t.Errorf("tenant B lists %d of tenant A's tokens", len(list))
	}
	if _, err := svc.ValidateToken(ctx, raw); err != nil {
		t.Fatalf("the token should still work after refused revokes: %v", err)
	}
}
