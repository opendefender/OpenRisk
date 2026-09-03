// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * The D-021 compatibility shim.
 *
 * D-021 relicensed the empty state to Apache-2.0 and moved the implementation to
 * `shared/ds/Empty.tsx`, leaving `shared/EmptyState.tsx` re-exporting it so the
 * existing importers did not have to be rewritten (#443 Résolution 1 point 4).
 * If the shim stops pointing at the real component, every one of those call
 * sites breaks at once — so it is asserted, not assumed.
 *
 * This suite lives on the AGPL side deliberately. It was previously in
 * `shared/ds/__tests__/feedback.test.tsx`, which is Apache-2.0, and importing the
 * AGPL shim from there pointed the dependency the one way it may not go: the
 * licence-boundary gate failed on it, and took four later CI steps down with it
 * (see #503). AGPL importing Apache is the allowed direction, so the assertion
 * belongs here.
 */

import { describe, expect, it } from 'vitest';

import { EmptyState } from '../EmptyState';
import { Empty } from '../ds/Empty';

describe('shared/EmptyState (D-021 shim)', () => {
  it('is still reachable under its old name and path', () => {
    expect(EmptyState).toBe(Empty);
  });

  it('re-exports the component itself, not a wrapper around it', () => {
    // A wrapper would satisfy "is defined" while quietly changing displayName,
    // defaultProps or ref forwarding for all 24 call sites.
    expect(EmptyState).toBeTypeOf('function');
    expect(EmptyState.name).toBe(Empty.name);
  });
});
