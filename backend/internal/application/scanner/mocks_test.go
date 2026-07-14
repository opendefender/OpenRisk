// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package scanner

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// --- fake Redis (KV + Locker) ----------------------------------------------

type fakeKV struct {
	mu       sync.Mutex
	store    map[string]string
	messages int
}

func newFakeKV() *fakeKV { return &fakeKV{store: map[string]string{}} }

func (f *fakeKV) Set(_ context.Context, k, v string, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.store[k] = v
	return nil
}
func (f *fakeKV) Get(_ context.Context, k string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.store[k], nil
}
func (f *fakeKV) Del(_ context.Context, keys ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, k := range keys {
		delete(f.store, k)
	}
	return nil
}
func (f *fakeKV) Publish(_ context.Context, _ string, _ interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages++
	return nil
}
func (f *fakeKV) SetNX(_ context.Context, k, v string, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.store[k]; ok {
		return false, nil
	}
	f.store[k] = v
	return true, nil
}

// --- mock repositories -----------------------------------------------------

type mockConfigRepo struct {
	createFunc  func(context.Context, *domain.ScanConfig) error
	getByIDFunc func(context.Context, uuid.UUID, uuid.UUID) (*domain.ScanConfig, error)
	listFunc    func(context.Context, uuid.UUID) ([]domain.ScanConfig, error)
	updateFunc  func(context.Context, *domain.ScanConfig) error
	deleteFunc  func(context.Context, uuid.UUID, uuid.UUID) error
}

func (m *mockConfigRepo) Create(c context.Context, cfg *domain.ScanConfig) error {
	if m.createFunc != nil {
		return m.createFunc(c, cfg)
	}
	return nil
}
func (m *mockConfigRepo) GetByID(c context.Context, id, t uuid.UUID) (*domain.ScanConfig, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(c, id, t)
	}
	return nil, nil
}
func (m *mockConfigRepo) List(c context.Context, t uuid.UUID) ([]domain.ScanConfig, error) {
	if m.listFunc != nil {
		return m.listFunc(c, t)
	}
	return nil, nil
}
func (m *mockConfigRepo) Update(c context.Context, cfg *domain.ScanConfig) error {
	if m.updateFunc != nil {
		return m.updateFunc(c, cfg)
	}
	return nil
}
func (m *mockConfigRepo) Delete(c context.Context, id, t uuid.UUID) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(c, id, t)
	}
	return nil
}

type mockJobRepo struct {
	createFunc      func(context.Context, *domain.ScanJob) error
	getByIDFunc     func(context.Context, uuid.UUID, uuid.UUID) (*domain.ScanJob, error)
	listFunc        func(context.Context, uuid.UUID) ([]domain.ScanJob, error)
	byStatusFunc    func(context.Context, uuid.UUID, domain.ScanJobStatus) ([]domain.ScanJob, error)
	countActiveFunc func(context.Context, uuid.UUID) (int64, error)
	updateFunc      func(context.Context, *domain.ScanJob) error
}

func (m *mockJobRepo) Create(c context.Context, j *domain.ScanJob) error {
	if m.createFunc != nil {
		return m.createFunc(c, j)
	}
	return nil
}
func (m *mockJobRepo) GetByID(c context.Context, id, t uuid.UUID) (*domain.ScanJob, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(c, id, t)
	}
	return nil, nil
}
func (m *mockJobRepo) List(c context.Context, t uuid.UUID) ([]domain.ScanJob, error) {
	if m.listFunc != nil {
		return m.listFunc(c, t)
	}
	return nil, nil
}
func (m *mockJobRepo) ListByStatus(c context.Context, t uuid.UUID, s domain.ScanJobStatus) ([]domain.ScanJob, error) {
	if m.byStatusFunc != nil {
		return m.byStatusFunc(c, t, s)
	}
	return nil, nil
}
func (m *mockJobRepo) CountActiveByTenant(c context.Context, t uuid.UUID) (int64, error) {
	if m.countActiveFunc != nil {
		return m.countActiveFunc(c, t)
	}
	return 0, nil
}
func (m *mockJobRepo) Update(c context.Context, j *domain.ScanJob) error {
	if m.updateFunc != nil {
		return m.updateFunc(c, j)
	}
	return nil
}

type mockAgentRepo struct {
	createFunc      func(context.Context, *domain.ScannerAgent) error
	getByIDFunc     func(context.Context, uuid.UUID, uuid.UUID) (*domain.ScannerAgent, error)
	byTokenHashFunc func(context.Context, string) (*domain.ScannerAgent, error)
	listFunc        func(context.Context, uuid.UUID) ([]domain.ScannerAgent, error)
	updateFunc      func(context.Context, *domain.ScannerAgent) error
	deleteFunc      func(context.Context, uuid.UUID, uuid.UUID) error
}

func (m *mockAgentRepo) Create(c context.Context, a *domain.ScannerAgent) error {
	if m.createFunc != nil {
		return m.createFunc(c, a)
	}
	return nil
}
func (m *mockAgentRepo) GetByID(c context.Context, id, t uuid.UUID) (*domain.ScannerAgent, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(c, id, t)
	}
	return nil, nil
}
func (m *mockAgentRepo) GetByTokenHash(c context.Context, h string) (*domain.ScannerAgent, error) {
	if m.byTokenHashFunc != nil {
		return m.byTokenHashFunc(c, h)
	}
	return nil, nil
}
func (m *mockAgentRepo) List(c context.Context, t uuid.UUID) ([]domain.ScannerAgent, error) {
	if m.listFunc != nil {
		return m.listFunc(c, t)
	}
	return nil, nil
}
func (m *mockAgentRepo) Update(c context.Context, a *domain.ScannerAgent) error {
	if m.updateFunc != nil {
		return m.updateFunc(c, a)
	}
	return nil
}
func (m *mockAgentRepo) Delete(c context.Context, id, t uuid.UUID) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(c, id, t)
	}
	return nil
}

// mockAssetRepo implements domain.AssetRepository for import tests.
type mockAssetRepo struct {
	created    []*domain.Asset
	createFunc func(context.Context, *domain.Asset) error
}

func (m *mockAssetRepo) Create(c context.Context, a *domain.Asset) error {
	if m.createFunc != nil {
		return m.createFunc(c, a)
	}
	m.created = append(m.created, a)
	return nil
}
func (m *mockAssetRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (*domain.Asset, error) {
	return nil, nil
}
func (m *mockAssetRepo) List(context.Context, uuid.UUID) ([]domain.Asset, error) { return nil, nil }
func (m *mockAssetRepo) Update(context.Context, *domain.Asset) error             { return nil }
func (m *mockAssetRepo) Delete(context.Context, uuid.UUID, uuid.UUID) error      { return nil }
func (m *mockAssetRepo) CreateSnapshot(context.Context, *domain.AssetSnapshot) error {
	return nil
}
func (m *mockAssetRepo) ListSnapshots(context.Context, uuid.UUID, uuid.UUID) ([]domain.AssetSnapshot, error) {
	return nil, nil
}

// --- helpers ---------------------------------------------------------------

func testKeys() *authpkg.RSAKeys {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	return &authpkg.RSAKeys{PrivateKey: priv, PublicKey: &priv.PublicKey}
}

func testCipher() *CredentialCipher {
	c, _ := NewCredentialCipher([]byte("unit-test-scanner-credential-key-0001"))
	return c
}
