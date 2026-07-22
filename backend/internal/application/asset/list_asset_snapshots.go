// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package asset

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// UserLookup resolves user IDs to a human-readable label (email) for the
// history "who" column. It is an optional dependency: history still works
// without it, snapshots simply carry the raw ChangedBy UUID and an empty
// ChangedByEmail. Any type with EmailsByIDs satisfies it (structural interface),
// so it can be backed by the existing user repository without a new port on
// domain.AssetRepository.
type UserLookup interface {
	EmailsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// ListAssetSnapshotsUseCase retrieves the history of an asset (ROADMAP.md M3
// "historical snapshots"): every prior state recorded before an update or
// deletion, newest first, each attributed to the user who made the change
// ("qui a modifié quoi, et quand").
type ListAssetSnapshotsUseCase struct {
	repo  domain.AssetRepository
	users UserLookup
}

func NewListAssetSnapshotsUseCase(repo domain.AssetRepository) *ListAssetSnapshotsUseCase {
	return &ListAssetSnapshotsUseCase{repo: repo}
}

// WithUserLookup enables resolving each snapshot's ChangedBy UUID to a
// ChangedByEmail display label. Kept optional (like DeleteAssetUseCase's
// WithDependencyRepository) so the plain constructor and its tests are unchanged.
func (uc *ListAssetSnapshotsUseCase) WithUserLookup(users UserLookup) *ListAssetSnapshotsUseCase {
	uc.users = users
	return uc
}

func (uc *ListAssetSnapshotsUseCase) Execute(ctx context.Context, tenantID uuid.UUID, assetID uuid.UUID) ([]domain.AssetSnapshot, error) {
	// Confirm the asset exists (and belongs to this tenant) before returning
	// history — otherwise an empty history for a nonexistent/foreign asset
	// ID would look identical to "no history yet", leaking existence info.
	existing, err := uc.repo.GetByID(ctx, assetID, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if existing == nil {
		return nil, domain.NewNotFoundError("asset", assetID)
	}

	history, err := uc.repo.ListSnapshots(ctx, assetID, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}

	uc.resolveActors(ctx, history)
	return history, nil
}

// resolveActors best-effort populates ChangedByEmail from ChangedBy. It never
// fails the request: if the lookup is not wired or errors, the history is
// returned with raw UUIDs and empty emails.
func (uc *ListAssetSnapshotsUseCase) resolveActors(ctx context.Context, history []domain.AssetSnapshot) {
	if uc.users == nil || len(history) == 0 {
		return
	}
	// Collect the distinct, non-nil actor IDs.
	seen := make(map[uuid.UUID]struct{})
	ids := make([]uuid.UUID, 0, len(history))
	for i := range history {
		id := history[i].ChangedBy
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return
	}
	emails, err := uc.users.EmailsByIDs(ctx, ids)
	if err != nil {
		return // degrade gracefully — history is still usable
	}
	for i := range history {
		if email, ok := emails[history[i].ChangedBy]; ok {
			history[i].ChangedByEmail = email
		}
	}
}
