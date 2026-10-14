// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #966 — when the English catalogue fails to load at boot, the screen falls back
// to French. When a retry succeeds the language has not changed, so only the
// catalogue version can tell React to re-render. This is that path.

import { describe, expect, it, vi } from 'vitest';
import { act, render, screen } from '@testing-library/react';

describe('useI18n after a failed catalogue load (#966)', () => {
  it('turns English once a retry on reconnect succeeds', async () => {
    vi.resetModules();
    localStorage.clear();
    let failNext = true;
    vi.doMock('../../i18n/catalog', async () => {
      const actual =
        await vi.importActual<typeof import('../../i18n/catalog')>('../../i18n/catalog');
      return {
        ...actual,
        loadCatalog: (code: Parameters<typeof actual.loadCatalog>[0]) => {
          if (code === 'en' && failNext) {
            failNext = false;
            return Promise.reject(new Error('offline'));
          }
          return actual.loadCatalog(code);
        },
      };
    });
    const { useUIStore } = await import('../../store/uiStore');
    const { useI18n } = await import('../useI18n');
    // A stored English preference, as main.tsx finds it on a cold boot.
    useUIStore.setState({ lang: 'en' });

    function Label() {
      const { t } = useI18n();
      return <p>{t('common.cancel')}</p>;
    }
    render(<Label />);

    // The first fetch failed: French fallback, never a blank or a key.
    expect(await screen.findByText('Annuler')).toBeInTheDocument();

    await act(async () => {
      window.dispatchEvent(new Event('online'));
    });
    expect(await screen.findByText('Cancel')).toBeInTheDocument();
    vi.doUnmock('../../i18n/catalog');
  });
});
