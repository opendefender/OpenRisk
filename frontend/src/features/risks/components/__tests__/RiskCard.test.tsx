// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Regression for audit-2026 #244: RiskCard coloured severity from the legacy
// `level` field, which is not reliably populated and rendered EVERY risk as
// medium. Severity must come from the server-computed `criticality` band.

import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { RiskCard } from '../RiskCard';
import type { Risk } from '../../../../hooks/useRiskStore';

function make(partial: Partial<Risk>): Risk {
  return { id: 'r1', title: 'T', score: 0, ...partial } as Risk;
}

function containerClass(risk: Risk): string {
  const { container } = render(<RiskCard risk={risk} />);
  return (container.firstElementChild as HTMLElement).className;
}

describe('RiskCard severity colour', () => {
  it('colours from criticality, not the legacy level field', () => {
    // level says MEDIUM (the stuck value) but criticality is critical.
    const cls = containerClass(make({ criticality: 'critical', level: 'MEDIUM', score: 10 }));
    expect(cls).toContain('bg-danger-surface');
  });

  it('gives critical and low distinct colours (no uniform medium)', () => {
    const critical = containerClass(make({ criticality: 'critical', score: 10 }));
    const low = containerClass(make({ criticality: 'low', score: 0.1 }));
    expect(critical).toContain('bg-danger-surface');
    expect(low).toContain('bg-info-surface');
    expect(critical).not.toEqual(low);
  });
});
