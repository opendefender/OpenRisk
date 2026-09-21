// #719 — dates written in the person's own zone and numeric pattern.

import { describe, it, expect } from 'vitest';

import { formatPreferredDate } from '../format';

// 23:30 UTC on 30 September: already 1 October in Douala (UTC+1).
const instant = '2026-09-30T23:30:00Z';

describe('formatPreferredDate', () => {
  it('applies each numeric pattern in the chosen time zone', () => {
    const zone = { timeZone: 'Africa/Douala' };
    expect(formatPreferredDate('fr', instant, { ...zone, pattern: 'DD/MM/YYYY' })).toBe(
      '01/10/2026',
    );
    expect(formatPreferredDate('en', instant, { ...zone, pattern: 'MM/DD/YYYY' })).toBe(
      '10/01/2026',
    );
    expect(formatPreferredDate('en', instant, { ...zone, pattern: 'YYYY-MM-DD' })).toBe(
      '2026-10-01',
    );
  });

  it('stays on the UTC day in UTC', () => {
    expect(formatPreferredDate('fr', instant, { timeZone: 'UTC', pattern: 'YYYY-MM-DD' })).toBe(
      '2026-09-30',
    );
  });

  it('falls back to the language style without a pattern, and appends time on request', () => {
    const day = formatPreferredDate('en', instant, { timeZone: 'UTC' });
    expect(day).toContain('2026');
    expect(day).toContain('Sep');
    const withTime = formatPreferredDate('en', instant, { timeZone: 'UTC' }, true);
    expect(withTime.startsWith(day)).toBe(true);
    expect(withTime).toMatch(/11:30/);
  });

  it('ignores an unknown time zone instead of throwing, and dashes an invalid date', () => {
    expect(() => formatPreferredDate('fr', instant, { timeZone: 'Mars/Olympus' })).not.toThrow();
    expect(formatPreferredDate('fr', 'not a date')).toBe('—');
  });
});
