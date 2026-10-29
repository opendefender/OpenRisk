// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #612 — billing does not show on a self-hosted instance.

import { describe, it, expect } from 'vitest';
import { settingsTabShown, settingsTabVisible } from '../settingsTabs';

describe('settings tabs on a self-hosted instance', () => {
  it('hides billing on self-hosted', () => {
    expect(settingsTabVisible('billing', true)).toBe(false);
    expect(settingsTabShown('billing', true, 'general')).toBe('general');
  });

  it('keeps billing on the hosted service', () => {
    expect(settingsTabVisible('billing', false)).toBe(true);
    expect(settingsTabShown('billing', false, 'general')).toBe('billing');
  });

  it('leaves every other tab alone', () => {
    for (const k of ['profile', 'general', 'members', 'security', 'danger']) {
      expect(settingsTabVisible(k, true)).toBe(true);
      expect(settingsTabShown(k, true, 'general')).toBe(k);
    }
  });
});
