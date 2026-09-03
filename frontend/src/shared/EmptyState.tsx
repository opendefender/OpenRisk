// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Compatibility shim. The implementation moved to `shared/ds/Empty.tsx` and was
// relicensed to Apache-2.0 by D-021 (2026-09-02), so that the design system has
// an empty state without the Apache layer depending on AGPL code — the
// dependency runs one way only, and this file is the allowed direction: AGPL
// importing Apache.
//
// This file exists so that the 24 importers of `shared/EmptyState` do not change
// at all: #443 Résolution 1 point 4 forbids rewriting call sites.
//
// NEW CODE SHOULD IMPORT `Empty` FROM `shared/ds`. This shim is kept for the
// existing call sites, not as a second name for a new one to choose between.

export {
  Empty as EmptyState,
  type EmptyProps as EmptyStateProps,
  type EmptyVariant as EmptyStateVariant,
} from './ds/Empty';
