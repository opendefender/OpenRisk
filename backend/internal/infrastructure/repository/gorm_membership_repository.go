// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormMembershipRepository is the tenant-scoped store behind organization
// member management (W0-04).
//
// Every query carries `organization_id = ?` in its WHERE clause, including the
// writes: a member id or invitation id lifted from a URL is never trusted on
// its own, so a forged or cross-tenant id resolves to nothing rather than to
// another organization's row.
type GormMembershipRepository struct {
	db *gorm.DB
}

// NewGormMembershipRepository builds the repository.
func NewGormMembershipRepository(db *gorm.DB) *GormMembershipRepository {
	return &GormMembershipRepository{db: db}
}

// maxPageSize bounds a listing. An organization with ten thousand members must
// not be able to answer one request with ten thousand rows, whatever the caller
// asked for.
const maxPageSize = 200

func page(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 50
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// ---------------------------------------------------------------------------
// Members
// ---------------------------------------------------------------------------

// memberOrder maps a caller-supplied sort key to a column. Anything unknown
// falls back to joined_at, so a crafted sort parameter cannot reach a column
// (or an injection) of the caller's choosing.
func memberOrder(sortBy string, desc bool) string {
	col := "organization_members.joined_at"
	switch sortBy {
	case "email":
		col = "users.email"
	case "role":
		col = "organization_members.role"
	case "status":
		col = "organization_members.status"
	case "joined_at", "":
	}
	if desc {
		return col + " DESC"
	}
	return col + " ASC"
}

// ListMembers returns one page of the tenant's memberships plus the total count
// matching the filter (not the page), which is what the UI needs to paginate.
func (r *GormMembershipRepository) ListMembers(ctx context.Context, tenantID uuid.UUID, q domain.MemberQuery) ([]domain.OrganizationMember, int64, error) {
	limit, offset := page(q.Limit, q.Offset)

	base := r.db.WithContext(ctx).
		Model(&domain.OrganizationMember{}).
		// Joined rather than preloaded: searching and sorting on the user's email
		// has to happen in SQL, or "page 3 of members sorted by email" is sorted
		// within page 3 only.
		Joins("LEFT JOIN users ON users.id = organization_members.user_id").
		Where("organization_members.organization_id = ?", tenantID)

	if s := strings.TrimSpace(q.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		base = base.Where("LOWER(users.email) LIKE ? OR LOWER(users.full_name) LIKE ?", like, like)
	}
	if q.Status != "" {
		if q.Status == domain.MembershipActive {
			// Rows written before the status column (NULL) and rows written before
			// the BeforeCreate hook ('' — GORM sends the Go zero value explicitly,
			// so the column DEFAULT never applied) both mean "unset", and are
			// active exactly when the legacy boolean says so. Same fallback as
			// EffectiveStatus.
			base = base.Where(`(organization_members.status = ?
			                    OR (COALESCE(organization_members.status, '') = '' AND organization_members.is_active))`,
				domain.MembershipActive)
		} else {
			base = base.Where("organization_members.status = ?", q.Status)
		}
	}
	if q.Role != "" {
		base = base.Where("organization_members.role = ?", q.Role)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var members []domain.OrganizationMember
	err := base.
		Preload("User").
		Preload("Profile.Permissions").
		Order(memberOrder(q.SortBy, q.SortDesc)).
		Limit(limit).Offset(offset).
		Find(&members).Error
	if err != nil {
		return nil, 0, err
	}
	return members, total, nil
}

// GetMember returns the membership of userID in tenantID, or nil if there is
// none. A missing membership is nil, not an error: "this user is not a member
// here" is an answer, and the caller decides what status it deserves.
func (r *GormMembershipRepository) GetMember(ctx context.Context, tenantID, userID uuid.UUID) (*domain.OrganizationMember, error) {
	return r.firstMember(ctx, r.db.WithContext(ctx).
		Where("organization_id = ? AND user_id = ?", tenantID, userID))
}

// GetMemberByID resolves a membership by its own id, scoped to the tenant.
func (r *GormMembershipRepository) GetMemberByID(ctx context.Context, tenantID, memberID uuid.UUID) (*domain.OrganizationMember, error) {
	return r.firstMember(ctx, r.db.WithContext(ctx).
		Where("organization_id = ? AND id = ?", tenantID, memberID))
}

func (r *GormMembershipRepository) firstMember(ctx context.Context, tx *gorm.DB) (*domain.OrganizationMember, error) {
	var m domain.OrganizationMember
	err := tx.Preload("User").Preload("Profile.Permissions").First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// SaveMember persists a role / lifecycle change.
//
// Scoped by id AND organization_id, and written through an explicit column list
// so a deliberate clear (an emptied business role, a nulled deactivated_at) is
// written rather than skipped as a GORM zero value.
func (r *GormMembershipRepository) SaveMember(ctx context.Context, m *domain.OrganizationMember) error {
	res := r.db.WithContext(ctx).
		Model(&domain.OrganizationMember{}).
		Where("id = ? AND organization_id = ?", m.ID, m.OrganizationID).
		Updates(map[string]interface{}{
			"role":           m.Role,
			"business_role":  m.BusinessRole,
			"is_active":      m.IsActive,
			"status":         m.Status,
			"deactivated_at": m.DeactivatedAt,
			"revoked_at":     m.RevokedAt,
			"updated_at":     time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		// Either the row is gone or it belongs to another tenant. Both answer the
		// same way, so the response cannot be used to probe for existence.
		return domain.NewNotFoundError("member", m.ID)
	}
	return nil
}

// CountActiveAdmins counts memberships that grant access and hold root/admin.
func (r *GormMembershipRepository) CountActiveAdmins(ctx context.Context, tenantID uuid.UUID) (int, error) {
	var n int64
	err := r.db.WithContext(ctx).
		Model(&domain.OrganizationMember{}).
		Where("organization_id = ?", tenantID).
		Where("role IN ?", []domain.MemberRole{domain.RoleRoot, domain.RoleAdmin}).
		Where("(status = ? OR (COALESCE(status, '') = '' AND is_active))", domain.MembershipActive).
		Count(&n).Error
	return int(n), err
}

// Counts computes the membership headline in two aggregate queries — one over
// memberships, one over invitations — rather than one query per number.
func (r *GormMembershipRepository) Counts(ctx context.Context, tenantID uuid.UUID) (domain.OrganizationCounts, error) {
	var out domain.OrganizationCounts

	// COUNT(*) FILTER is standard SQL and supported by both Postgres and the
	// sqlite build used by the tests, so one row answers every membership number.
	row := struct {
		Total       int64
		Active      int64
		Deactivated int64
		Revoked     int64
		Admins      int64
	}{}
	err := r.db.WithContext(ctx).
		Model(&domain.OrganizationMember{}).
		Select(`COUNT(*) AS total,
		        COUNT(*) FILTER (WHERE status = 'active' OR (COALESCE(status,'') = '' AND is_active)) AS active,
		        COUNT(*) FILTER (WHERE status = 'deactivated' OR (COALESCE(status,'') = '' AND NOT is_active)) AS deactivated,
		        COUNT(*) FILTER (WHERE status = 'revoked') AS revoked,
		        COUNT(*) FILTER (WHERE role IN ('root','admin') AND (status = 'active' OR (COALESCE(status,'') = '' AND is_active))) AS admins`).
		Where("organization_id = ?", tenantID).
		Scan(&row).Error
	if err != nil {
		return out, err
	}
	out.TotalMembers = row.Total
	out.ActiveMembers = row.Active
	out.DeactivatedMembers = row.Deactivated
	out.RevokedMembers = row.Revoked
	out.Admins = row.Admins

	// Pending counts what is actually still redeemable, so a stale invitation
	// nobody revoked does not inflate the badge forever.
	if err := r.db.WithContext(ctx).
		Model(&domain.Invitation{}).
		Where("organization_id = ? AND status = ? AND expires_at > ?", tenantID, domain.InvitationPending, time.Now()).
		Count(&out.PendingInvitations).Error; err != nil {
		return out, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Invitations
// ---------------------------------------------------------------------------

// CreateInvitation persists a new invitation. A unique-violation on the partial
// index over (organization_id, email) WHERE status='pending' surfaces as a
// conflict: that index is the half of the duplicate check that survives two
// admins inviting the same person at the same moment.
func (r *GormMembershipRepository) CreateInvitation(ctx context.Context, inv *domain.Invitation) error {
	if err := r.db.WithContext(ctx).Create(inv).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.NewConflictError("invitation", "email")
		}
		return err
	}
	return nil
}

// SaveInvitation persists a rotation / revocation / acceptance, scoped by
// organization so a forged id cannot reach another tenant's invitation.
func (r *GormMembershipRepository) SaveInvitation(ctx context.Context, inv *domain.Invitation) error {
	res := r.db.WithContext(ctx).
		Model(&domain.Invitation{}).
		Where("id = ? AND organization_id = ?", inv.ID, inv.OrganizationID).
		Updates(map[string]interface{}{
			"token_hash":     inv.TokenHash,
			"status":         inv.Status,
			"role":           inv.Role,
			"business_role":  inv.BusinessRole,
			"expires_at":     inv.ExpiresAt,
			"accepted_at":    inv.AcceptedAt,
			"accepted_by_id": inv.AcceptedByID,
			"revoked_at":     inv.RevokedAt,
			"revoked_by_id":  inv.RevokedByID,
			"last_sent_at":   inv.LastSentAt,
			"send_count":     inv.SendCount,
			"updated_at":     time.Now(),
		})
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return domain.NewConflictError("invitation", "email")
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("invitation", inv.ID)
	}
	return nil
}

// ListInvitations returns one page of the tenant's invitations, newest first.
func (r *GormMembershipRepository) ListInvitations(ctx context.Context, tenantID uuid.UUID, q domain.InvitationQuery) ([]domain.Invitation, int64, error) {
	limit, offset := page(q.Limit, q.Offset)

	base := r.db.WithContext(ctx).
		Model(&domain.Invitation{}).
		Where("organization_id = ?", tenantID)
	if q.Status != "" {
		base = base.Where("status = ?", q.Status)
	}
	if e := domain.NormalizeEmail(q.Email); e != "" {
		base = base.Where("email = ?", e)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []domain.Invitation
	if err := base.Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetInvitation resolves an invitation by id within the tenant.
func (r *GormMembershipRepository) GetInvitation(ctx context.Context, tenantID, id uuid.UUID) (*domain.Invitation, error) {
	var inv domain.Invitation
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND id = ?", tenantID, id).
		First(&inv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

// FindPendingInvitation returns the tenant's outstanding invitation for email.
func (r *GormMembershipRepository) FindPendingInvitation(ctx context.Context, tenantID uuid.UUID, email string) (*domain.Invitation, error) {
	var inv domain.Invitation
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND email = ? AND status = ?", tenantID, domain.NormalizeEmail(email), domain.InvitationPending).
		First(&inv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

// FindInvitationByToken resolves a bearer token by its hash.
//
// Deliberately not tenant-scoped: the caller has no membership yet, so there is
// no tenant to scope by. The lookup is by hash — the plaintext never touches a
// query — and the organization comes from the row that was found.
func (r *GormMembershipRepository) FindInvitationByToken(ctx context.Context, token string) (*domain.Invitation, error) {
	hash := domain.HashInvitationToken(token)
	if strings.TrimSpace(token) == "" {
		return nil, nil
	}
	var inv domain.Invitation
	err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&inv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

// AcceptInvitation consumes the invitation and writes the membership atomically.
//
// The consuming UPDATE is conditional on the invitation still being pending, so
// two acceptances racing on the same token produce exactly one membership: the
// loser's UPDATE matches no row and its transaction rolls back. Doing it as a
// read-then-write would let both pass the read.
func (r *GormMembershipRepository) AcceptInvitation(ctx context.Context, inv *domain.Invitation, member *domain.OrganizationMember) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&domain.Invitation{}).
			Where("id = ? AND status = ?", inv.ID, domain.InvitationPending).
			Updates(map[string]interface{}{
				"status":         domain.InvitationAccepted,
				"accepted_at":    inv.AcceptedAt,
				"accepted_by_id": inv.AcceptedByID,
				"updated_at":     time.Now(),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// Someone else consumed it between the check and here.
			return domain.NewConflictError("invitation", "status")
		}
		if err := tx.Create(member).Error; err != nil {
			if isUniqueViolation(err) {
				return domain.NewConflictError("member", "user_id")
			}
			return err
		}
		return nil
	})
}

// isUniqueViolation recognises a duplicate-key error across the drivers this
// project runs on: Postgres in production, sqlite in the handler tests. Matching
// on the message keeps the repository free of a driver-specific import.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "unique violation")
}
