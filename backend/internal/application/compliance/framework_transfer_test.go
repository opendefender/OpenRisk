// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validDoc = `{"schema_version":1,"framework":{"name":"Circulaire 01","version":"2026","description":"d"},
"controls":[{"reference_code":"A.1","name":"One","description":"x","source_reference":"art. 1"},
{"reference_code":"A.2","name":"Two"}]}`

func TestExportFramework_Success(t *testing.T) {
	tenant, fwID := uuid.New(), uuid.New()
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(_ context.Context, id, tid uuid.UUID) (*domain.ComplianceFramework, error) {
			require.Equal(t, tenant, tid)
			return &domain.ComplianceFramework{ID: id, TenantID: tid, Name: "F", Version: "1", Description: "d"}, nil
		},
		listControlsByFrameworkFunc: func(_ context.Context, tid, _ uuid.UUID) ([]domain.ComplianceControl, error) {
			require.Equal(t, tenant, tid)
			return []domain.ComplianceControl{
				{ID: uuid.New(), TenantID: tid, ReferenceCode: "B", Name: "b", Status: domain.ControlStatusImplemented, EvidenceCount: 3},
				{ID: uuid.New(), TenantID: tid, ReferenceCode: "A", Name: "a", SourceReference: "s"},
			}, nil
		},
	}
	doc, err := NewExportFrameworkUseCase(repo).Execute(context.Background(), tenant, fwID)
	require.NoError(t, err)
	assert.Equal(t, 1, doc.SchemaVersion)
	require.Len(t, doc.Controls, 2)
	assert.Equal(t, "A", doc.Controls[0].ReferenceCode, "ordered by reference_code")

	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	for _, forbidden := range []string{"tenant_id", `"id"`, "status", "evidence", "catalog_key", "framework_id", "created_at"} {
		assert.NotContains(t, string(raw), forbidden)
	}
}

func TestExportFramework_NotFound(t *testing.T) {
	_, err := NewExportFrameworkUseCase(&MockComplianceRepository{}).Execute(context.Background(), uuid.New(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestExportFramework_Unauthorized(t *testing.T) {
	_, err := NewExportFrameworkUseCase(&MockComplianceRepository{}).Execute(context.Background(), uuid.Nil, uuid.New())
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestImportFramework_Success(t *testing.T) {
	tenant := uuid.New()
	var gotFw *domain.ComplianceFramework
	var gotCtl []*domain.ComplianceControl
	repo := &MockComplianceRepository{createFwWithControlsFunc: func(_ context.Context, fw *domain.ComplianceFramework, c []*domain.ComplianceControl) error {
		gotFw, gotCtl = fw, c
		return nil
	}}
	// The file tries to smuggle a tenant, a status and a catalog key.
	doc := `{"schema_version":1,"tenant_id":"` + uuid.NewString() + `","framework":{"name":"F","tenant_id":"` + uuid.NewString() + `","catalog_key":"iso27001"},
"controls":[{"reference_code":" A.1 ","name":"One","status":"implemented","tenant_id":"` + uuid.NewString() + `"}]}`
	res, err := NewImportFrameworkUseCase(repo).Execute(context.Background(), tenant, []byte(doc))
	require.NoError(t, err)
	assert.Equal(t, 1, res.ControlsCreated)
	assert.Equal(t, tenant, gotFw.TenantID)
	assert.Equal(t, "", gotFw.CatalogKey)
	require.Len(t, gotCtl, 1)
	assert.Equal(t, tenant, gotCtl[0].TenantID)
	assert.Equal(t, gotFw.ID, gotCtl[0].FrameworkID)
	assert.Equal(t, domain.ControlStatusNotImplemented, gotCtl[0].Status)
	assert.Equal(t, "A.1", gotCtl[0].ReferenceCode)
}

func TestImportFramework_Unauthorized(t *testing.T) {
	called := false
	repo := &MockComplianceRepository{createFwWithControlsFunc: func(context.Context, *domain.ComplianceFramework, []*domain.ComplianceControl) error {
		called = true
		return nil
	}}
	_, err := NewImportFrameworkUseCase(repo).Execute(context.Background(), uuid.Nil, []byte(validDoc))
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
	assert.False(t, called)
}

func TestImportFramework_Conflict(t *testing.T) {
	repo := &MockComplianceRepository{createFwWithControlsFunc: func(context.Context, *domain.ComplianceFramework, []*domain.ComplianceControl) error {
		return domain.NewConflictError("framework", "name+version")
	}}
	_, err := NewImportFrameworkUseCase(repo).Execute(context.Background(), uuid.New(), []byte(validDoc))
	assert.ErrorIs(t, err, domain.ErrConflict)
}

func TestImportFramework_RepoFailurePropagates(t *testing.T) {
	boom := errors.New("boom")
	repo := &MockComplianceRepository{createFwWithControlsFunc: func(context.Context, *domain.ComplianceFramework, []*domain.ComplianceControl) error { return boom }}
	_, err := NewImportFrameworkUseCase(repo).Execute(context.Background(), uuid.New(), []byte(validDoc))
	assert.ErrorIs(t, err, boom)
}

func ctlJSON(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = `{"reference_code":"C` + uuid.NewString()[:8] + `","name":"n"}`
	}
	return strings.Join(parts, ",")
}

func TestImportFramework_Validation(t *testing.T) {
	long := func(n int) string { return strings.Repeat("é", n) }
	tests := []struct {
		name      string
		doc       string
		wantIndex *int
		wantField string
	}{
		{"empty code", `{"schema_version":1,"framework":{"name":"F"},"controls":[{"reference_code":"A","name":"a"},{"reference_code":"  ","name":"b"}]}`, ptr(1), "reference_code"},
		{"duplicate code", `{"schema_version":1,"framework":{"name":"F"},"controls":[{"reference_code":"A","name":"a"},{"reference_code":"A","name":"b"}]}`, ptr(1), "reference_code"},
		{"empty control name", `{"schema_version":1,"framework":{"name":"F"},"controls":[{"reference_code":"A","name":""}]}`, ptr(0), "name"},
		{"empty framework name", `{"schema_version":1,"framework":{"name":""},"controls":[{"reference_code":"A","name":"a"}]}`, nil, "framework.name"},
		{"framework name too long", `{"schema_version":1,"framework":{"name":"` + long(256) + `"},"controls":[{"reference_code":"A","name":"a"}]}`, nil, "framework.name"},
		{"code too long", `{"schema_version":1,"framework":{"name":"F"},"controls":[{"reference_code":"` + long(51) + `","name":"a"}]}`, ptr(0), "reference_code"},
		{"source ref too long", `{"schema_version":1,"framework":{"name":"F"},"controls":[{"reference_code":"A","name":"a","source_reference":"` + long(256) + `"}]}`, ptr(0), "source_reference"},
		{"description too long", `{"schema_version":1,"framework":{"name":"F"},"controls":[{"reference_code":"A","name":"a","description":"` + long(10001) + `"}]}`, ptr(0), "description"},
		{"bad schema version", `{"schema_version":2,"framework":{"name":"F"},"controls":[{"reference_code":"A","name":"a"}]}`, nil, "schema_version"},
		{"missing schema version", `{"framework":{"name":"F"},"controls":[{"reference_code":"A","name":"a"}]}`, nil, "schema_version"},
		{"NUL in framework name", `{"schema_version":1,"framework":{"name":"F\u0000"},"controls":[{"reference_code":"A","name":"a"}]}`, nil, "framework.name"},
		{"NUL in framework version", `{"schema_version":1,"framework":{"name":"F","version":"1\u0000"},"controls":[{"reference_code":"A","name":"a"}]}`, nil, "framework.version"},
		{"NUL in framework description", `{"schema_version":1,"framework":{"name":"F","description":"d\u0000"},"controls":[{"reference_code":"A","name":"a"}]}`, nil, "framework.description"},
		{"NUL in reference code", `{"schema_version":1,"framework":{"name":"F"},"controls":[{"reference_code":"A","name":"a"},{"reference_code":"B\u0000","name":"b"}]}`, ptr(1), "reference_code"},
		{"NUL in control name", `{"schema_version":1,"framework":{"name":"F"},"controls":[{"reference_code":"A","name":"a\u0000"}]}`, ptr(0), "name"},
		{"NUL in control description", `{"schema_version":1,"framework":{"name":"F"},"controls":[{"reference_code":"A","name":"a","description":"\u0000"}]}`, ptr(0), "description"},
		{"NUL in source reference", `{"schema_version":1,"framework":{"name":"F"},"controls":[{"reference_code":"A","name":"a","source_reference":"\u0000"}]}`, ptr(0), "source_reference"},
		{"no controls", `{"schema_version":1,"framework":{"name":"F"},"controls":[]}`, nil, "controls"},
		{"too many controls", `{"schema_version":1,"framework":{"name":"F"},"controls":[` + ctlJSON(2001) + `]}`, nil, "controls"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			repo := &MockComplianceRepository{createFwWithControlsFunc: func(context.Context, *domain.ComplianceFramework, []*domain.ComplianceControl) error {
				called = true
				return nil
			}}
			_, err := NewImportFrameworkUseCase(repo).Execute(context.Background(), uuid.New(), []byte(tc.doc))
			require.ErrorIs(t, err, domain.ErrValidation)
			assert.Equal(t, 400, domain.HTTPStatusFromError(err))
			assert.False(t, called, "nothing may be written")
			fields := domain.FieldErrorsFromError(err)
			require.NotEmpty(t, fields)
			found := false
			for _, f := range fields {
				if f.Field == tc.wantField && ((tc.wantIndex == nil && f.Index == nil) || (tc.wantIndex != nil && f.Index != nil && *f.Index == *tc.wantIndex)) {
					found = true
				}
			}
			assert.True(t, found, "want %s at %v, got %+v", tc.wantField, tc.wantIndex, fields)
		})
	}
}

func TestImportFramework_Validation_MalformedAndOversize(t *testing.T) {
	repo := &MockComplianceRepository{}
	uc := NewImportFrameworkUseCase(repo)
	for name, raw := range map[string][]byte{
		"malformed":   []byte(`{"schema_version":`),
		"wrong types": []byte(`{"schema_version":"1"}`),
		"oversize":    []byte(`{"schema_version":1,"pad":"` + strings.Repeat("a", MaxFrameworkImportBytes) + `"}`),
	} {
		_, err := uc.Execute(context.Background(), uuid.New(), raw)
		require.ErrorIs(t, err, domain.ErrValidation, name)
		assert.Equal(t, 400, domain.HTTPStatusFromError(err), name)
	}
}

func TestImportFramework_Validation_CollectsAllRowErrorsCapped(t *testing.T) {
	rows := make([]string, 150)
	for i := range rows {
		rows[i] = `{"reference_code":"","name":""}`
	}
	doc := `{"schema_version":1,"framework":{"name":"F"},"controls":[` + strings.Join(rows, ",") + `]}`
	_, err := NewImportFrameworkUseCase(&MockComplianceRepository{}).Execute(context.Background(), uuid.New(), []byte(doc))
	require.ErrorIs(t, err, domain.ErrValidation)
	assert.Len(t, domain.FieldErrorsFromError(err), 100)
}

func ptr(i int) *int { return &i }
