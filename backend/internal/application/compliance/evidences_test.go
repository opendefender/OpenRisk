// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateEvidenceUseCase_Success(t *testing.T) {
	tenantID := uuid.New()
	controlID := uuid.New()
	uploader := uuid.New()
	store := NewMockStorage()
	repo := &MockComplianceRepository{
		getControlByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ComplianceControl, error) {
			return &domain.ComplianceControl{ID: controlID, TenantID: tenantID}, nil
		},
	}
	uc := NewCreateEvidenceUseCase(repo, store)

	evidence, err := uc.Execute(context.Background(), tenantID, CreateEvidenceInput{
		ControlID: controlID, Filename: "audit-report.pdf", Content: strings.NewReader("pdf bytes"), UploadedBy: uploader,
	})

	require.NoError(t, err)
	require.NotNil(t, evidence)
	assert.Equal(t, tenantID, evidence.TenantID)
	assert.Equal(t, "audit-report.pdf", evidence.Filename)
	assert.NotEmpty(t, evidence.URL, "URL must hold the storage key returned by Save")
	require.NotNil(t, evidence.UploadedBy)
	assert.Equal(t, uploader, *evidence.UploadedBy)
}

func TestCreateEvidenceUseCase_ControlNotFound(t *testing.T) {
	repo := &MockComplianceRepository{
		getControlByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ComplianceControl, error) {
			return nil, nil
		},
	}
	uc := NewCreateEvidenceUseCase(repo, NewMockStorage())

	_, err := uc.Execute(context.Background(), uuid.New(), CreateEvidenceInput{
		ControlID: uuid.New(), Filename: "x.pdf", Content: strings.NewReader("x"),
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestCreateEvidenceUseCase_CrossTenantControl_NotFound(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	controlID := uuid.New()
	repo := &MockComplianceRepository{
		getControlByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ComplianceControl, error) {
			if tid == tenantA {
				return &domain.ComplianceControl{ID: controlID, TenantID: tenantA}, nil
			}
			return nil, nil
		},
	}
	uc := NewCreateEvidenceUseCase(repo, NewMockStorage())

	_, err := uc.Execute(context.Background(), tenantB, CreateEvidenceInput{
		ControlID: controlID, Filename: "x.pdf", Content: strings.NewReader("x"),
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound, "tenantB must not be able to attach evidence to tenantA's control")
}

func TestCreateEvidenceUseCase_RepoFailure_RollsBackStoredFile(t *testing.T) {
	tenantID := uuid.New()
	controlID := uuid.New()
	store := NewMockStorage()
	deletedKeys := []string{}
	store.deleteFn = func(key string) error {
		deletedKeys = append(deletedKeys, key)
		return nil
	}
	repo := &MockComplianceRepository{
		getControlByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ComplianceControl, error) {
			return &domain.ComplianceControl{ID: controlID, TenantID: tenantID}, nil
		},
		createEvidenceFunc: func(ctx context.Context, e *domain.ControlEvidence) error {
			return errors.New("db exploded")
		},
	}
	uc := NewCreateEvidenceUseCase(repo, store)

	_, err := uc.Execute(context.Background(), tenantID, CreateEvidenceInput{
		ControlID: controlID, Filename: "x.pdf", Content: strings.NewReader("x"),
	})

	require.Error(t, err)
	assert.Len(t, deletedKeys, 1, "the file saved before the DB write failed must be cleaned up")
}

func TestDownloadEvidenceUseCase_Success(t *testing.T) {
	tenantID := uuid.New()
	store := NewMockStorage()
	key, err := store.Save(context.Background(), tenantID, "report.pdf", strings.NewReader("content"))
	require.NoError(t, err)

	evID := uuid.New()
	repo := &MockComplianceRepository{
		getEvidenceByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ControlEvidence, error) {
			return &domain.ControlEvidence{ID: evID, TenantID: tenantID, URL: key, Filename: "report.pdf"}, nil
		},
	}
	uc := NewDownloadEvidenceUseCase(repo, store)

	evidence, reader, err := uc.Execute(context.Background(), tenantID, evID)

	require.NoError(t, err)
	assert.Equal(t, "report.pdf", evidence.Filename)
	content, _ := io.ReadAll(reader)
	assert.Equal(t, "content", string(content))
}

func TestDownloadEvidenceUseCase_CrossTenant_NotFound(t *testing.T) {
	repo := &MockComplianceRepository{
		getEvidenceByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ControlEvidence, error) {
			return nil, nil
		},
	}
	uc := NewDownloadEvidenceUseCase(repo, NewMockStorage())

	_, _, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestDeleteEvidenceUseCase_Success(t *testing.T) {
	tenantID := uuid.New()
	evID := uuid.New()
	store := NewMockStorage()
	key, _ := store.Save(context.Background(), tenantID, "x.pdf", strings.NewReader("x"))
	repo := &MockComplianceRepository{
		getEvidenceByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ControlEvidence, error) {
			return &domain.ControlEvidence{ID: evID, TenantID: tenantID, URL: key}, nil
		},
	}
	uc := NewDeleteEvidenceUseCase(repo, store)

	err := uc.Execute(context.Background(), tenantID, evID)

	require.NoError(t, err)
	_, openErr := store.Open(context.Background(), key)
	assert.Error(t, openErr, "the underlying file must be removed too")
}

func TestDeleteEvidenceUseCase_CrossTenant_NotFound(t *testing.T) {
	repo := &MockComplianceRepository{
		getEvidenceByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ControlEvidence, error) {
			return nil, nil
		},
	}
	uc := NewDeleteEvidenceUseCase(repo, NewMockStorage())

	err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestListEvidencesUseCase_Success(t *testing.T) {
	repo := &MockComplianceRepository{
		listEvidencesByControlFunc: func(ctx context.Context, tid, cid uuid.UUID) ([]domain.ControlEvidence, error) {
			return []domain.ControlEvidence{{Filename: "a.pdf"}, {Filename: "b.pdf"}}, nil
		},
	}
	uc := NewListEvidencesUseCase(repo)

	evidences, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.NoError(t, err)
	assert.Len(t, evidences, 2)
}
