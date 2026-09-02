// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * CartesianChart — the visx replacement for Recharts' Area/Bar/Line/Composed
 * families and their furniture. #444, D-024 option A.
 *
 * What is worth defending here is not "does it draw a line" — the visual suite
 * does that better, in a real browser. It is the set of rules the component
 * OWNS so a caller cannot break them, because the dashboards it replaces broke
 * every one of them: hardcoded `fill="#ef4444"` for a series that was not a
 * severity, and no text alternative anywhere.
 *
 * jsdom gives every element a zero bounding box, so ParentSize measures 0 and
 * the SVG does not render. That is fine: the assertions below are about the
 * legend and the text alternative, which live outside the SVG.
 */

import { describe, expect, it } from 'vitest';
import { render, screen, within } from '@testing-library/react';

import { CartesianChart } from '../CartesianChart';

interface Row {
  month: string;
  opened: number;
  closed: number;
}

const DATA: Row[] = [
  { month: 'Jan', opened: 12, closed: 4 },
  { month: 'Feb', opened: 8, closed: 9 },
  { month: 'Mar', opened: 15, closed: 11 },
];

describe('CartesianChart', () => {
  it('states its data as a table, so the numbers survive without the SVG', () => {
    render(
      <CartesianChart
        data={DATA}
        x="month"
        series={[{ type: 'bar', key: 'opened', label: 'Opened' }]}
        ariaLabel="Risks opened per month"
      />,
    );
    const table = screen.getByRole('table', { name: 'Risks opened per month' });
    expect(within(table).getAllByRole('row')).toHaveLength(4); // header + 3
    expect(within(table).getByRole('rowheader', { name: 'Feb' })).toBeInTheDocument();
    expect(within(table).getByRole('columnheader', { name: 'Opened' })).toBeInTheDocument();
  });

  it('names every value in the text alternative, not just the last series', () => {
    render(
      <CartesianChart
        data={DATA}
        x="month"
        series={[
          { type: 'bar', key: 'opened', label: 'Opened' },
          { type: 'line', key: 'closed', label: 'Closed' },
        ]}
        ariaLabel="Risks opened and closed per month"
      />,
    );
    const table = screen.getByRole('table');
    expect(within(table).getByText('15')).toBeInTheDocument();
    expect(within(table).getByText('11')).toBeInTheDocument();
  });

  it('draws a legend only when there is more than one series', () => {
    const { rerender } = render(
      <CartesianChart
        data={DATA}
        x="month"
        series={[{ type: 'bar', key: 'opened', label: 'Opened' }]}
        ariaLabel="One series"
      />,
    );
    // A single series is named by the panel heading; a legend would repeat it.
    expect(screen.queryByRole('list')).not.toBeInTheDocument();

    rerender(
      <CartesianChart
        data={DATA}
        x="month"
        series={[
          { type: 'bar', key: 'opened', label: 'Opened' },
          { type: 'line', key: 'closed', label: 'Closed' },
        ]}
        ariaLabel="Two series"
      />,
    );
    const legend = screen.getByRole('list');
    expect(within(legend).getAllByRole('listitem')).toHaveLength(2);
  });

  it('renders the empty slot instead of an axis-only chart when there is no data', () => {
    render(
      <CartesianChart
        data={[] as Row[]}
        x="month"
        series={[{ type: 'bar', key: 'opened', label: 'Opened' }]}
        ariaLabel="Nothing yet"
        empty={<p>No data yet</p>}
      />,
    );
    expect(screen.getByText('No data yet')).toBeInTheDocument();
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });

  it('formats values through formatValue in the text alternative', () => {
    render(
      <CartesianChart
        data={DATA}
        x="month"
        series={[{ type: 'bar', key: 'opened', label: 'Opened' }]}
        formatValue={(v) => `${v} risks`}
        ariaLabel="Formatted"
      />,
    );
    expect(screen.getByText('12 risks')).toBeInTheDocument();
  });
});
