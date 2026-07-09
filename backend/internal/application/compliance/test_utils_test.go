// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

import (
	"bytes"
	"context"
	"io"
	"sync"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// MockComplianceRepository is a hand-rolled mock implementing
// domain.ComplianceRepository, mirroring internal/application/risk's
// MockRiskRepository pattern (function fields, nil-safe defaults).
type MockComplianceRepository struct {
	createFrameworkFunc         func(ctx context.Context, fw *domain.ComplianceFramework) error
	getFrameworkByIDFunc        func(ctx context.Context, id uuid.UUID) (*domain.ComplianceFramework, error)
	listFrameworksFunc          func(ctx context.Context) ([]domain.ComplianceFramework, error)
	createControlFunc           func(ctx context.Context, c *domain.ComplianceControl) error
	getControlByIDFunc          func(ctx context.Context, id, tenantID uuid.UUID) (*domain.ComplianceControl, error)
	listControlsByFrameworkFunc func(ctx context.Context, tenantID, frameworkID uuid.UUID) ([]domain.ComplianceControl, error)
	updateControlFunc           func(ctx context.Context, c *domain.ComplianceControl) error
	deleteControlFunc           func(ctx context.Context, id, tenantID uuid.UUID) error
	createEvidenceFunc          func(ctx context.Context, e *domain.ControlEvidence) error
	getEvidenceByIDFunc         func(ctx context.Context, id, tenantID uuid.UUID) (*domain.ControlEvidence, error)
	listEvidencesByControlFunc  func(ctx context.Context, tenantID, controlID uuid.UUID) ([]domain.ControlEvidence, error)
	countEvidencesByFwFunc      func(ctx context.Context, tenantID, frameworkID uuid.UUID) (map[uuid.UUID]int, error)
	deleteEvidenceFunc          func(ctx context.Context, id, tenantID uuid.UUID) error
}

func (m *MockComplianceRepository) CreateFramework(ctx context.Context, fw *domain.ComplianceFramework) error {
	if m.createFrameworkFunc != nil {
		return m.createFrameworkFunc(ctx, fw)
	}
	return nil
}

func (m *MockComplianceRepository) GetFrameworkByID(ctx context.Context, id uuid.UUID) (*domain.ComplianceFramework, error) {
	if m.getFrameworkByIDFunc != nil {
		return m.getFrameworkByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockComplianceRepository) ListFrameworks(ctx context.Context) ([]domain.ComplianceFramework, error) {
	if m.listFrameworksFunc != nil {
		return m.listFrameworksFunc(ctx)
	}
	return []domain.ComplianceFramework{}, nil
}

func (m *MockComplianceRepository) CreateControl(ctx context.Context, c *domain.ComplianceControl) error {
	if m.createControlFunc != nil {
		return m.createControlFunc(ctx, c)
	}
	return nil
}

func (m *MockComplianceRepository) GetControlByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.ComplianceControl, error) {
	if m.getControlByIDFunc != nil {
		return m.getControlByIDFunc(ctx, id, tenantID)
	}
	return nil, nil
}

func (m *MockComplianceRepository) ListControlsByFramework(ctx context.Context, tenantID, frameworkID uuid.UUID) ([]domain.ComplianceControl, error) {
	if m.listControlsByFrameworkFunc != nil {
		return m.listControlsByFrameworkFunc(ctx, tenantID, frameworkID)
	}
	return []domain.ComplianceControl{}, nil
}

func (m *MockComplianceRepository) UpdateControl(ctx context.Context, c *domain.ComplianceControl) error {
	if m.updateControlFunc != nil {
		return m.updateControlFunc(ctx, c)
	}
	return nil
}

func (m *MockComplianceRepository) DeleteControl(ctx context.Context, id, tenantID uuid.UUID) error {
	if m.deleteControlFunc != nil {
		return m.deleteControlFunc(ctx, id, tenantID)
	}
	return nil
}

func (m *MockComplianceRepository) CreateEvidence(ctx context.Context, e *domain.ControlEvidence) error {
	if m.createEvidenceFunc != nil {
		return m.createEvidenceFunc(ctx, e)
	}
	return nil
}

func (m *MockComplianceRepository) GetEvidenceByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.ControlEvidence, error) {
	if m.getEvidenceByIDFunc != nil {
		return m.getEvidenceByIDFunc(ctx, id, tenantID)
	}
	return nil, nil
}

func (m *MockComplianceRepository) ListEvidencesByControl(ctx context.Context, tenantID, controlID uuid.UUID) ([]domain.ControlEvidence, error) {
	if m.listEvidencesByControlFunc != nil {
		return m.listEvidencesByControlFunc(ctx, tenantID, controlID)
	}
	return []domain.ControlEvidence{}, nil
}

func (m *MockComplianceRepository) CountEvidencesByFramework(ctx context.Context, tenantID, frameworkID uuid.UUID) (map[uuid.UUID]int, error) {
	if m.countEvidencesByFwFunc != nil {
		return m.countEvidencesByFwFunc(ctx, tenantID, frameworkID)
	}
	return map[uuid.UUID]int{}, nil
}

func (m *MockComplianceRepository) DeleteEvidence(ctx context.Context, id, tenantID uuid.UUID) error {
	if m.deleteEvidenceFunc != nil {
		return m.deleteEvidenceFunc(ctx, id, tenantID)
	}
	return nil
}

// MockStorage is an in-memory stand-in for storage.Storage, used by
// evidence use-case tests so they don't touch the filesystem.
type MockStorage struct {
	mu       sync.Mutex
	objects  map[string][]byte
	saveErr  error
	openErr  error
	deleteFn func(key string) error
}

func NewMockStorage() *MockStorage {
	return &MockStorage{objects: map[string][]byte{}}
}

func (m *MockStorage) Save(_ context.Context, tenantID uuid.UUID, filename string, content io.Reader) (string, error) {
	if m.saveErr != nil {
		return "", m.saveErr
	}
	data, _ := io.ReadAll(content)
	key := tenantID.String() + "/" + uuid.New().String() + "-" + filename
	m.mu.Lock()
	m.objects[key] = data
	m.mu.Unlock()
	return key, nil
}

func (m *MockStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	if m.openErr != nil {
		return nil, m.openErr
	}
	m.mu.Lock()
	data, ok := m.objects[key]
	m.mu.Unlock()
	if !ok {
		return nil, io.EOF
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *MockStorage) Delete(_ context.Context, key string) error {
	if m.deleteFn != nil {
		return m.deleteFn(key)
	}
	m.mu.Lock()
	delete(m.objects, key)
	m.mu.Unlock()
	return nil
}
