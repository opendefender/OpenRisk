// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/require"
)

// #1519: an empty or unknown severity is a typed validation error naming the
// field, not a plain error that the central mapper answers with 500.

func TestCreateIncident_Severity_Success(t *testing.T) {
	db := newIncidentRiskDB(t)
	svc := NewIncidentService(db)
	for _, sev := range []string{"critical", "high", "medium", "low"} {
		_, err := svc.CreateIncident(uuid.NewString(), domain.IncidentCreateRequest{Title: "i", Severity: sev})
		require.NoError(t, err, sev)
	}
}

func TestCreateIncident_Severity_Invalid_IsAValidationError(t *testing.T) {
	db := newIncidentRiskDB(t)
	svc := NewIncidentService(db)
	for _, sev := range []string{"", "urgent", "HIGH", " high"} {
		_, err := svc.CreateIncident(uuid.NewString(), domain.IncidentCreateRequest{Title: "i", Severity: sev})
		require.ErrorIs(t, err, domain.ErrValidation, "severity %q", sev)
		require.Equal(t, 400, domain.HTTPStatusFromError(err))
		fields := domain.FieldErrorsFromError(err)
		require.Len(t, fields, 1)
		require.Equal(t, "severity", fields[0].Field)
	}
	require.EqualValues(t, 0, incidentCount(t, db))
}
