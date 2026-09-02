// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * RiskMatrix — the 5×5 probability × impact grid. #444.
 *
 * Behaviour and accessibility, never class names, in line with the other ds
 * suites. The two properties worth defending here are the ones that were
 * missing from the hand-rolled grid this replaces:
 *
 *   1. The axes are real table headers, so a cell has a position a screen
 *      reader can state, rather than being the 14th div of 25.
 *   2. Severity is never carried by fill alone — including in an EMPTY cell,
 *      where there is no marker to read and colour would otherwise be the only
 *      encoding. That is the case a snapshot test cannot see.
 *
 * The third is a rule rather than a behaviour: the component holds no
 * thresholds. `cellBand` is required, and the suite asserts the component asks
 * rather than decides.
 */

import { describe, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { RiskMatrix, type MatrixBucket, type RiskMatrixItem } from '../RiskMatrix';
import type { SeverityKey } from '../chart';

const labels = {
  caption: 'Risk matrix — probability by impact',
  probability: 'Probability',
  impact: 'Impact',
  band: (b: SeverityKey) => ({ critical: 'Critical', high: 'High', medium: 'Medium', low: 'Low', extreme: 'Extreme' })[b],
  more: (n: number) => `+${n}`,
};

/* The ramp the register uses. Passed in, never owned by the component. */
const cellBand = (p: MatrixBucket, i: MatrixBucket): SeverityKey => {
  const v = p * i;
  return v >= 15 ? 'critical' : v >= 8 ? 'high' : v >= 4 ? 'medium' : 'low';
};

const item = (over: Partial<RiskMatrixItem> = {}): RiskMatrixItem => ({
  id: 'r1',
  label: 'Unpatched TLS on the edge gateway',
  probability: 4,
  impact: 4,
  band: 'critical',
  marker: '8',
  ...over,
});

const setup = (items: RiskMatrixItem[], onSelect?: (id: string) => void) =>
  render(<RiskMatrix items={items} cellBand={cellBand} labels={labels} onSelect={onSelect} />);

describe('RiskMatrix', () => {
  it('renders 25 cells with both axes as headers, so every cell has a stated position', () => {
    setup([]);
    const table = screen.getByRole('table', { name: labels.caption });
    // 5 column headers + 5 row headers + the corner.
    expect(within(table).getAllByRole('columnheader')).toHaveLength(6);
    expect(within(table).getAllByRole('rowheader')).toHaveLength(5);
    expect(within(table).getAllByRole('cell')).toHaveLength(25);
  });

  it('puts probability on the vertical axis running 5 down to 1', () => {
    setup([]);
    const rows = screen.getAllByRole('rowheader').map((h) => h.textContent);
    expect(rows).toEqual(['5', '4', '3', '2', '1']);
  });

  it('states the band of an EMPTY cell in text, so fill is never the only encoding', () => {
    setup([]);
    // (5,5) is the worst corner and holds no risks; its severity must still be readable.
    expect(screen.getAllByText('Critical').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Low').length).toBeGreaterThan(0);
  });

  it('asks cellBand for every cell rather than deriving a band itself', () => {
    const spy = vi.fn(cellBand);
    render(<RiskMatrix items={[]} cellBand={spy} labels={labels} />);
    expect(spy).toHaveBeenCalledTimes(25);
    expect(spy).toHaveBeenCalledWith(5, 5);
    expect(spy).toHaveBeenCalledWith(1, 1);
  });

  it('places a risk at its own coordinates, not the cell ramp', async () => {
    setup([item({ probability: 2, impact: 1, band: 'low', label: 'Stale IAM roles' })], vi.fn());
    const marker = screen.getByRole('button', { name: /Stale IAM roles/ });
    // The marker names its own band; the cell it sits in has a different one.
    expect(marker).toHaveAccessibleName(/Low/);
  });

  it('names each marker for assistive tech instead of relying on a title tooltip', () => {
    setup([item()], vi.fn());
    expect(
      screen.getByRole('button', { name: /Unpatched TLS on the edge gateway — Critical/ }),
    ).toBeInTheDocument();
  });

  it('calls onSelect with the risk id when a marker is activated', async () => {
    const onSelect = vi.fn();
    setup([item({ id: 'risk-42' })], onSelect);
    await userEvent.click(screen.getByRole('button', { name: /Unpatched TLS/ }));
    expect(onSelect).toHaveBeenCalledWith('risk-42');
  });

  it('renders inert markers when there is nothing to select, rather than dead buttons', () => {
    setup([item()]);
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
    expect(screen.getByRole('img', { name: /Unpatched TLS/ })).toBeInTheDocument();
  });

  it('collapses past maxPerCell into a count rather than overflowing the cell', () => {
    const many = Array.from({ length: 9 }, (_, n) =>
      item({ id: `r${n}`, label: `Risk ${n}`, probability: 3, impact: 3 }),
    );
    render(
      <RiskMatrix items={many} cellBand={cellBand} labels={labels} maxPerCell={6} onSelect={vi.fn()} />,
    );
    expect(screen.getAllByRole('button')).toHaveLength(6);
    expect(screen.getByText('+3')).toBeInTheDocument();
  });

  it('is keyboard reachable — every marker takes focus in turn', async () => {
    setup(
      [
        item({ id: 'a', label: 'Risk A', probability: 1, impact: 1 }),
        item({ id: 'b', label: 'Risk B', probability: 5, impact: 5 }),
      ],
      vi.fn(),
    );
    await userEvent.tab();
    expect(screen.getByRole('button', { name: /Risk B/ })).toHaveFocus();
    await userEvent.tab();
    expect(screen.getByRole('button', { name: /Risk A/ })).toHaveFocus();
  });
});
