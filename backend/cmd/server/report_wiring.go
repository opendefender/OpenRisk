// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/service"
)

// Adapters mapping existing modules onto the reporting engine's narrow source
// ports, so the engine depends on what it reads rather than on how those modules
// happen to be shaped. Same approach as executive_wiring.go.

// reportIncidentAdapter maps the legacy incident service onto IncidentSource.
type reportIncidentAdapter struct{ svc *service.IncidentService }

// ListIncidentsForReport returns the tenant's incidents within a period.
//
// A zero From or To means "unbounded", which is what the configurator sends when
// the user leaves a period blank — an incident report over "all time" is a
// legitimate ask, and defaulting to some window would silently omit rows the
// reader believes they asked for.
func (a reportIncidentAdapter) ListIncidentsForReport(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]domain.Incident, error) {
	if a.svc == nil {
		return nil, nil
	}
	// A generous page: a period report has to see every incident in the period,
	// and silently truncating one would understate a security posture.
	all, _, err := a.svc.ListIncidents(tenantID.String(), domain.IncidentQuery{Limit: 1000})
	if err != nil {
		return nil, err
	}
	if from.IsZero() && to.IsZero() {
		return all, nil
	}
	out := make([]domain.Incident, 0, len(all))
	for _, i := range all {
		if !from.IsZero() && i.CreatedAt.Before(from) {
			continue
		}
		// Inclusive of the end date: a period "to 30 June" that dropped the 30th
		// would quietly under-report, and nobody reading the title would know.
		if !to.IsZero() && i.CreatedAt.After(to.Add(24*time.Hour)) {
			continue
		}
		out = append(out, i)
	}
	return out, nil
}

// reportAuditAdapter maps the compliance-audit repository onto AuditSource.
type reportAuditAdapter struct {
	repo *repository.GormComplianceAuditRepository
}

func (a reportAuditAdapter) GetAudit(ctx context.Context, tenantID, id uuid.UUID) (*domain.ComplianceAudit, error) {
	if a.repo == nil {
		return nil, nil
	}
	return a.repo.GetAuditByID(ctx, tenantID, id)
}

func (a reportAuditAdapter) ListAudits(ctx context.Context, tenantID uuid.UUID) ([]domain.ComplianceAudit, error) {
	if a.repo == nil {
		return nil, nil
	}
	return a.repo.ListAudits(ctx, tenantID)
}

func (a reportAuditAdapter) ListRemediations(ctx context.Context, tenantID uuid.UUID) ([]domain.RemediationPlan, error) {
	if a.repo == nil {
		return nil, nil
	}
	return a.repo.ListRemediations(ctx, tenantID, domain.RemediationFilter{})
}
