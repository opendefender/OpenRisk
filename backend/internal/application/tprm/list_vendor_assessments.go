// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// ListVendorAssessmentsUseCase lists a vendor's assessments, newest first,
// without their items. The status reported is the EFFECTIVE one, so an open
// assessment past its grace period reads as expired.
type ListVendorAssessmentsUseCase struct{ deps AssessmentDeps }

func NewListVendorAssessmentsUseCase(deps AssessmentDeps) *ListVendorAssessmentsUseCase {
	return &ListVendorAssessmentsUseCase{deps: deps}
}

func (uc *ListVendorAssessmentsUseCase) Execute(ctx context.Context, tenantID, vendorID uuid.UUID) ([]domain.VendorAssessment, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("missing tenant context")
	}
	vendor, err := uc.deps.Vendors.GetVendor(ctx, vendorID, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if vendor == nil {
		return nil, domain.NewNotFoundError("vendor", vendorID)
	}

	list, err := uc.deps.Assessments.ListAssessmentsByVendor(ctx, tenantID, vendorID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if list == nil {
		list = []domain.VendorAssessment{}
	}
	now := uc.deps.now()
	for i := range list {
		list[i].Status = list[i].State(now)
	}
	return list, nil
}
