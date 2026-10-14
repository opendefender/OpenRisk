// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package threatintel

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

var cvePattern = regexp.MustCompile(`^CVE-\d{4}-\d{4,}$`)

// DismissThreatUseCase records, or withdraws, a tenant's "not relevant" on a
// threat. The threat stays listed, dimmed, so the call can be undone.
type DismissThreatUseCase struct {
	tenant    TenantSide
	catalogue Catalogue
	now       func() time.Time
}

// NewDismissThreatUseCase builds the use case.
func NewDismissThreatUseCase(t TenantSide, c Catalogue) *DismissThreatUseCase {
	return &DismissThreatUseCase{tenant: t, catalogue: c, now: time.Now}
}

func normaliseCVE(cveID string) (string, error) {
	id := strings.ToUpper(strings.TrimSpace(cveID))
	if !cvePattern.MatchString(id) {
		return "", domain.NewValidationError("not a CVE identifier: " + cveID)
	}
	return id, nil
}

// Dismiss marks the threat not relevant for the tenant. Idempotent. A CVE the
// catalogue does not hold is not found: there is nothing to dismiss.
func (uc *DismissThreatUseCase) Dismiss(ctx context.Context, tenantID, userID uuid.UUID, cveID string) error {
	id, err := normaliseCVE(cveID)
	if err != nil {
		return err
	}
	v, err := uc.catalogue.GetByCVE(ctx, id)
	if err != nil || v == nil {
		return domain.NewNotFoundError("threat", id)
	}
	d := &domain.CTIThreatDismissal{
		ID: uuid.New(), TenantID: tenantID, CVEID: id, DismissedBy: userID, CreatedAt: uc.now(),
	}
	if err := uc.tenant.Dismiss(ctx, d); err != nil {
		return domain.NewInternalError("could not record the dismissal: " + err.Error())
	}
	return nil
}

// Restore withdraws the tenant's dismissal. Not found when there was none.
func (uc *DismissThreatUseCase) Restore(ctx context.Context, tenantID uuid.UUID, cveID string) error {
	id, err := normaliseCVE(cveID)
	if err != nil {
		return err
	}
	ok, err := uc.tenant.Restore(ctx, tenantID, id)
	if err != nil {
		return domain.NewInternalError("could not restore the threat: " + err.Error())
	}
	if !ok {
		return domain.NewNotFoundError("threat dismissal", id)
	}
	return nil
}
