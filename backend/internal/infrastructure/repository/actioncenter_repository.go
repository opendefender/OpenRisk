// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/application/actioncenter"
	"github.com/opendefender/openrisk/internal/domain"
)

// ActionCenterRepository reads the six existing tables the Action Center
// aggregates. It owns no table of its own and writes nothing.
//
// EVERY method here filters on the tenant. That is not a convention in this
// file, it is the entire security model of the endpoint: the use case above has
// no way to re-filter what it is handed, so a missing WHERE here is a
// cross-tenant leak with no second line of defence (CLAUDE.md rule 2, P0). The
// tenant is always the first parameter for exactly that reason — it is
// impossible to call one of these methods without deciding on a tenant.
type ActionCenterRepository struct {
	db *gorm.DB
}

func NewActionCenterRepository(db *gorm.DB) *ActionCenterRepository {
	return &ActionCenterRepository{db: db}
}

// guard fails closed on a zero tenant. A uuid.Nil reaching a WHERE clause would
// match nothing today, but it would also mean the caller's identity was never
// established, and that is a bug we want loud rather than empty.
func guard(tenantID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return domain.ErrForbidden
	}
	return nil
}

// BusinessRoleFor resolves the caller's GRC job-role preset. Scoped by BOTH the
// user and the organisation: a person who belongs to two tenants holds a
// different business role in each, and reading the wrong row would hand them
// the wrong tenant's view of their own work.
func (r *ActionCenterRepository) BusinessRoleFor(userID, tenantID uuid.UUID) (domain.BusinessRoleKey, error) {
	if err := guard(tenantID); err != nil {
		return "", err
	}
	var member domain.OrganizationMember
	err := r.db.
		Where("user_id = ? AND organization_id = ?", userID, tenantID).
		First(&member).Error
	if err != nil {
		// A caller with no membership row has no business role, which the use
		// case treats as the least-privilege default (approvals only). Not an
		// error: a root/admin operating outside a member row is legitimate.
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	return member.BusinessRole, nil
}

// OverdueMitigations — category 1. Non-terminal status with a due date in the
// past.
func (r *ActionCenterRepository) OverdueMitigations(tenantID uuid.UUID, now time.Time, limit int) ([]domain.Mitigation, error) {
	if err := guard(tenantID); err != nil {
		return nil, err
	}
	var rows []domain.Mitigation
	err := r.db.
		Where("tenant_id = ?", tenantID).
		Where("status NOT IN ?", []string{string(domain.MitigationDone), string(domain.MitigationCancelled)}).
		Where("due_date IS NOT NULL AND due_date < ?", now).
		Order("due_date ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// CriticalRisksWithoutActiveMitigation — category 2.
//
// "No active mitigation" is a NOT EXISTS rather than a LEFT JOIN so a risk with
// three finished mitigations and none open still counts as unattended, and a
// risk with one open mitigation is excluded exactly once rather than
// suppressing duplicate rows afterwards.
//
// The subquery is tenant-scoped too. It would be tempting to skip it — the
// mitigation is joined on risk_id and the outer query already picked the
// tenant's risks — but relying on that means a mitigation row written with a
// mismatched tenant_id could mask one tenant's risk using another tenant's
// data. Filtering both sides costs an index lookup and removes the question.
func (r *ActionCenterRepository) CriticalRisksWithoutActiveMitigation(tenantID uuid.UUID, threshold float64, limit int) ([]domain.Risk, error) {
	if err := guard(tenantID); err != nil {
		return nil, err
	}
	active := r.db.
		Table("mitigations").
		Select("1").
		Where("mitigations.risk_id = risks.id").
		Where("mitigations.tenant_id = ?", tenantID).
		Where("mitigations.status NOT IN ?", []string{string(domain.MitigationDone), string(domain.MitigationCancelled)}).
		Where("mitigations.deleted_at IS NULL")

	var rows []domain.Risk
	err := r.db.
		Where("risks.tenant_id = ?", tenantID).
		Where("risks.score >= ?", threshold).
		// A risk that has been mitigated, formally accepted or closed is a
		// decision already taken, not outstanding work. Compared lower-cased
		// because this column carries two historical vocabularies ("closed" and
		// "CLOSED") and an exact match would silently list resolved risks.
		Where("LOWER(risks.status) NOT IN ?", []string{"mitigated", "accepted", "closed"}).
		Where("NOT EXISTS (?)", active).
		Order("risks.score DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// PendingApprovals — category 3. Returns the tenant's pending requests; WHICH of
// them the caller may sign is decided by domain.CanSign in the use case, not
// here, so there is one implementation of that rule in the product.
func (r *ActionCenterRepository) PendingApprovals(tenantID uuid.UUID, limit int) ([]domain.ApprovalRequest, error) {
	if err := guard(tenantID); err != nil {
		return nil, err
	}
	var rows []domain.ApprovalRequest
	err := r.db.
		Where("tenant_id = ?", tenantID).
		Where("status = ?", string(domain.ApprovalPending)).
		Order("created_at ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// OpenIncidents — category 4.
//
// NOTE the tenant comparison: incidents.tenant_id is a VARCHAR on this table,
// not a uuid, unlike every other model the Action Center reads. The string form
// is passed explicitly so the driver compares text to text. Handing it a
// uuid.UUID works on Postgres (which casts) and silently matches nothing on
// sqlite, which is precisely the kind of difference that makes a tenant-leak
// test pass locally and fail in production, or vice versa.
func (r *ActionCenterRepository) OpenIncidents(tenantID uuid.UUID, limit int) ([]domain.Incident, error) {
	if err := guard(tenantID); err != nil {
		return nil, err
	}
	var rows []domain.Incident
	err := r.db.
		Where("tenant_id = ?", tenantID.String()).
		Where("status IN ?", []string{"open", "investigating"}).
		Order("created_at ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// ExpiringEvidence — category 5. Narrowed by the calendar here; the final
// verdict is domain.Evidence.EffectiveStatus in the use case, which also knows
// that a rejected or unreviewed artifact is not proof whatever its date says.
func (r *ActionCenterRepository) ExpiringEvidence(tenantID uuid.UUID, now time.Time, limit int) ([]domain.Evidence, error) {
	if err := guard(tenantID); err != nil {
		return nil, err
	}
	var rows []domain.Evidence
	err := r.db.
		Where("tenant_id = ?", tenantID).
		Where("valid_until IS NOT NULL AND valid_until < ?", now.Add(domain.EvidenceExpiryWindow)).
		Order("valid_until ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// OverdueRemediationPlans — category 6.
func (r *ActionCenterRepository) OverdueRemediationPlans(tenantID uuid.UUID, now time.Time, limit int) ([]domain.RemediationPlan, error) {
	if err := guard(tenantID); err != nil {
		return nil, err
	}
	var rows []domain.RemediationPlan
	err := r.db.
		Where("tenant_id = ?", tenantID).
		Where("status = ?", string(domain.RemediationStatusOpen)).
		Where("due_date IS NOT NULL AND due_date < ?", now).
		Order("due_date ASC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// Compile-time proof that the GORM implementation still satisfies the port the
// use case declares. Without this the two drift apart silently until main.go
// fails to build, which is a worse place to find out.
var _ actioncenter.Repository = (*ActionCenterRepository)(nil)
