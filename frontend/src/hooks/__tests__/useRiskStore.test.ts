// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';
import { useRiskStore, type Risk } from '../useRiskStore';
import { api } from '../../lib/api';

describe('useRiskStore', () => {
  beforeEach(() => {
    // reset store state
    const s = useRiskStore.getState();
    s.risks = [];
    s.total = 0;
    s.page = 1;
    s.pageSize = 10;
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('fetchRisks sets risks and total for paginated response', async () => {
    const fakeData = { items: [{ id: 1, title: 'Risk A' }], total: 1 };
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: fakeData });

    await useRiskStore.getState().fetchRisks({ page: 1, limit: 10 });

    const state = useRiskStore.getState();
    expect(state.risks).toEqual(fakeData.items);
    expect(state.total).toBe(1);
    expect(api.get).toHaveBeenCalledWith('/risks', { params: { page: 1, limit: 10 } });
  });

  it('fetchRisks handles legacy array response', async () => {
    const legacy = [{ id: 2, title: 'Risk B' }];
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: legacy });

    await useRiskStore.getState().fetchRisks();

    const state = useRiskStore.getState();
    expect(state.risks).toEqual(legacy);
    expect(state.total).toBe(1);
  });

  it('createRisk posts payload and refreshes list', async () => {
    const newRisk = { title: 'New' };
    vi.spyOn(api, 'post').mockResolvedValueOnce({ data: { id: 3, ...newRisk } });
    // after create, fetchRisks will be called; mock its response
    vi.spyOn(api, 'get').mockResolvedValueOnce({
      data: { items: [{ id: 3, title: 'New' }], total: 1 },
    });

    await useRiskStore.getState().createRisk(newRisk);

    const state = useRiskStore.getState();
    expect(api.post).toHaveBeenCalledWith('/risks', newRisk);
    expect(state.risks).toEqual([{ id: 3, title: 'New' }]);
    expect(state.total).toBe(1);
  });

  it('updateRisk patches the risk optimistically in place', async () => {
    // updateRisk is an OPTIMISTIC mutation (RULE #10): it patches the row in
    // place and reconciles with the PATCH response — it does not refetch.
    const id = '4';
    const patch = { title: 'Updated' };
    useRiskStore.setState({ risks: [{ id, title: 'Old' } as unknown as Risk] });
    vi.spyOn(api, 'patch').mockResolvedValueOnce({ data: { id, title: 'Updated' } });

    await useRiskStore.getState().updateRisk(id, patch);

    const state = useRiskStore.getState();
    expect(api.patch).toHaveBeenCalledWith(`/risks/${id}`, patch);
    expect(state.risks).toEqual([{ id, title: 'Updated' }]);
  });

  it('deleteRisk calls delete and refreshes list', async () => {
    const id = '5';
    vi.spyOn(api, 'delete').mockResolvedValueOnce({ data: {} });
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: { items: [], total: 0 } });

    await useRiskStore.getState().deleteRisk(id);

    const state = useRiskStore.getState();
    expect(api.delete).toHaveBeenCalledWith(`/risks/${id}`);
    expect(state.risks).toEqual([]);
    expect(state.total).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// Governed bulk delete (#598).
//
// The register used to fan out one DELETE per row (#598), then applied one
// governed request without a preview. The request now belongs to
// useGovernedBulk, which previews first and sends the fingerprint (#1441); the
// store keeps only the optimistic half. These pin that it makes no request of
// its own, and that its undo is exact — the only correct restore under
// all-or-nothing (D-036), because a refusal means nothing was written.
// ---------------------------------------------------------------------------

describe('useRiskStore.removeOptimistically — optimistic half of the governed delete', () => {
  const seed = (): Risk[] =>
    [
      { id: 'r1', title: 'A' },
      { id: 'r2', title: 'B' },
      { id: 'r3', title: 'C' },
    ] as unknown as Risk[];

  beforeEach(() => {
    useRiskStore.setState({ risks: seed(), total: 3, selectedIds: ['r1', 'r2'] });
    vi.restoreAllMocks();
  });

  it('removes the rows and clears the selection without sending any request', () => {
    const post = vi.spyOn(api, 'post');
    const del = vi.spyOn(api, 'delete');

    useRiskStore.getState().removeOptimistically(new Set(['r1', 'r2']));

    const s = useRiskStore.getState();
    expect(s.risks.map((r) => r.id)).toEqual(['r3']);
    expect(s.total).toBe(1);
    expect(s.selectedIds).toEqual([]);
    // Applying is useGovernedBulk's job, after a preview — never the store's.
    expect(post).not.toHaveBeenCalled();
    expect(del).not.toHaveBeenCalled();
  });

  it('only counts rows actually on the page against the total', () => {
    useRiskStore.getState().removeOptimistically(new Set(['r1', 'gone']));
    expect(useRiskStore.getState().total).toBe(2);
  });

  it('undo restores rows, total AND selection byte-for-byte', () => {
    const before = useRiskStore.getState();
    const rowsBefore = before.risks;
    const totalBefore = before.total;
    const selectionBefore = before.selectedIds;

    const undo = useRiskStore.getState().removeOptimistically(new Set(['r1', 'r2']));
    undo();

    const after = useRiskStore.getState();
    expect(after.risks).toBe(rowsBefore);
    expect(after.total).toBe(totalBefore);
    // The selection survives a refusal: nothing was deleted, so the user's
    // selection is still meaningful and they can retry without re-picking.
    expect(after.selectedIds).toBe(selectionBefore);
  });
});
