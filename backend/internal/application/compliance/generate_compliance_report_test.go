// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeOrgLookup / fakeUserLookup satisfy the report use case's lookup ports.
type fakeOrgLookup struct {
	org *domain.Organization
	err error
}

func (f fakeOrgLookup) GetByID(_ context.Context, _ uuid.UUID) (*domain.Organization, error) {
	return f.org, f.err
}

type fakeUserLookup struct {
	user *domain.User
	err  error
}

func (f fakeUserLookup) GetByID(_ context.Context, _ uuid.UUID) (*domain.User, error) {
	return f.user, f.err
}

func TestGenerateComplianceReport_Success(t *testing.T) {
	tenantID := uuid.New()
	frameworkID := uuid.New()
	userID := uuid.New()
	c1, c2, c3, c4 := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(_ context.Context, id uuid.UUID) (*domain.ComplianceFramework, error) {
			assert.Equal(t, frameworkID, id)
			return &domain.ComplianceFramework{ID: frameworkID, Name: "ISO/IEC 27001", Version: "2022"}, nil
		},
		listControlsByFrameworkFunc: func(_ context.Context, tid, fid uuid.UUID) ([]domain.ComplianceControl, error) {
			// The use case MUST scope by the caller's tenant, not some other tenant.
			assert.Equal(t, tenantID, tid)
			assert.Equal(t, frameworkID, fid)
			return []domain.ComplianceControl{
				{ID: c1, ReferenceCode: "A.5.1", Name: "Policies", Status: domain.ControlStatusImplemented, SourceReference: "src1"},
				{ID: c2, ReferenceCode: "A.5.2", Name: "Roles", Status: domain.ControlStatusInProgress},
				{ID: c3, ReferenceCode: "A.5.3", Name: "Duties", Status: domain.ControlStatusNotImplemented},
				{ID: c4, ReferenceCode: "A.5.4", Name: "N/A", Status: domain.ControlStatusNotApplicable},
			}, nil
		},
		countEvidencesByFwFunc: func(_ context.Context, tid, fid uuid.UUID) (map[uuid.UUID]int, error) {
			assert.Equal(t, tenantID, tid)
			return map[uuid.UUID]int{c1: 2}, nil
		},
	}
	orgs := fakeOrgLookup{org: &domain.Organization{ID: tenantID, Name: "Banque Atlantique"}}
	usrs := fakeUserLookup{user: &domain.User{Email: "admin@opendefender.io"}}

	uc := NewGenerateComplianceReportUseCase(repo, orgs, usrs)
	data, err := uc.Execute(context.Background(), tenantID, frameworkID, userID, report.LocaleFR)

	require.NoError(t, err)
	require.NotNil(t, data)
	assert.Equal(t, "Banque Atlantique", data.OrganizationName)
	assert.Equal(t, "admin@opendefender.io", data.GeneratedBy)
	assert.Equal(t, "ISO/IEC 27001", data.FrameworkName)
	assert.Equal(t, "2022", data.FrameworkVersion)

	assert.Equal(t, 4, data.Total)
	assert.Equal(t, 1, data.Implemented)
	assert.Equal(t, 1, data.InProgress)
	assert.Equal(t, 1, data.NotImplemented)
	assert.Equal(t, 1, data.NotApplicable)
	assert.Equal(t, 3, data.Applicable) // 4 total - 1 not_applicable
	assert.InDelta(t, 100.0/3.0, data.PercentComplete, 0.001)

	require.Len(t, data.Controls, 4)
	assert.Equal(t, "A.5.1", data.Controls[0].ReferenceCode)
	assert.Equal(t, 2, data.Controls[0].EvidenceCount, "control with evidence must carry its count")
	assert.Equal(t, 0, data.Controls[1].EvidenceCount, "control without evidence defaults to zero")
	assert.Equal(t, report.StatusImplemented, data.Controls[0].Status)

	// The assembled data must actually render to a valid PDF.
	pdf, err := report.RenderCompliancePDF(*data)
	require.NoError(t, err)
	require.NotEmpty(t, pdf)
}

func TestGenerateComplianceReport_FrameworkNotFound(t *testing.T) {
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(_ context.Context, _ uuid.UUID) (*domain.ComplianceFramework, error) {
			return nil, nil // not found
		},
	}
	uc := NewGenerateComplianceReportUseCase(repo, fakeOrgLookup{}, fakeUserLookup{})

	data, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), uuid.New(), report.LocaleFR)

	require.Error(t, err)
	assert.Nil(t, data)
	assert.True(t, errors.Is(err, domain.ErrNotFound), "expected a typed not_found error")
}

// A tenant that has imported no controls still gets a valid (empty) report,
// never a divide-by-zero or a nil-deref.
func TestGenerateComplianceReport_NoControls(t *testing.T) {
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(_ context.Context, id uuid.UUID) (*domain.ComplianceFramework, error) {
			return &domain.ComplianceFramework{ID: id, Name: "COBAC R-2016/04"}, nil
		},
		listControlsByFrameworkFunc: func(_ context.Context, _, _ uuid.UUID) ([]domain.ComplianceControl, error) {
			return []domain.ComplianceControl{}, nil
		},
	}
	uc := NewGenerateComplianceReportUseCase(repo, fakeOrgLookup{}, fakeUserLookup{})

	data, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), uuid.New(), report.LocaleFR)

	require.NoError(t, err)
	assert.Equal(t, 0, data.Total)
	assert.Equal(t, 0.0, data.PercentComplete)
	assert.Empty(t, data.Controls)
}

// A missing/erroring org or user lookup must degrade gracefully, not fail the
// whole report — the identity lines simply come back empty.
func TestGenerateComplianceReport_LookupErrorsDegradeGracefully(t *testing.T) {
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(_ context.Context, id uuid.UUID) (*domain.ComplianceFramework, error) {
			return &domain.ComplianceFramework{ID: id, Name: "BCEAO"}, nil
		},
	}
	orgs := fakeOrgLookup{err: errors.New("db down")}
	usrs := fakeUserLookup{err: errors.New("db down")}
	uc := NewGenerateComplianceReportUseCase(repo, orgs, usrs)

	data, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), uuid.New(), report.LocaleFR)

	require.NoError(t, err)
	assert.Empty(t, data.OrganizationName)
	assert.Empty(t, data.GeneratedBy)
}
