// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #783 criterion 15 — a 402 is never retried, by a query or by a mutation. The
// effect is asserted: how many times the service was called.

import { describe, it, expect } from 'vitest';
import { QueryClient } from '@tanstack/react-query';
import { retryUnlessPlanRefusal } from '../retryPolicy';

const refusal = {
  response: { status: 402, data: { code: 'limit_reached', limit_key: 'frameworks' } },
};
const outage = { response: { status: 503 } };

function client() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: retryUnlessPlanRefusal(2), retryDelay: 0 },
      mutations: { retry: retryUnlessPlanRefusal(3), retryDelay: 0 },
    },
  });
}

describe('retryUnlessPlanRefusal', () => {
  it('calls a capped create exactly once', async () => {
    let calls = 0;
    const qc = client();
    const m = qc.getMutationCache().build(qc, {
      mutationFn: async () => {
        calls++;
        throw refusal;
      },
    });
    await expect(m.execute(undefined)).rejects.toBe(refusal);
    expect(calls).toBe(1);
  });

  it('still retries a mutation that failed for another reason', async () => {
    let calls = 0;
    const qc = client();
    const m = qc.getMutationCache().build(qc, {
      mutationFn: async () => {
        calls++;
        throw outage;
      },
    });
    await expect(m.execute(undefined)).rejects.toBe(outage);
    expect(calls).toBe(4);
  });

  it('calls a gated query exactly once', async () => {
    let calls = 0;
    const qc = client();
    await expect(
      qc.fetchQuery({
        queryKey: ['cti', 'threats'],
        queryFn: async () => {
          calls++;
          throw refusal;
        },
      }),
    ).rejects.toBe(refusal);
    expect(calls).toBe(1);
  });
});
