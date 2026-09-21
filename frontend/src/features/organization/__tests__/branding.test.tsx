// #718 — the organization's logo and accent.

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { BrandingSync } from '../BrandingSync';
import { OrganizationLogoField } from '../../settings/OrganizationLogoField';
import { ACCENT_PRESETS } from '../../../shared/accentPresets';
import { useUIStore } from '../../../store/uiStore';
import type { OrganizationView } from '../organizationService';

const getBranding = vi.fn();
const uploadLogo = vi.fn();
vi.mock('../organizationService', async () => {
  const actual =
    await vi.importActual<typeof import('../organizationService')>('../organizationService');
  return {
    ...actual,
    organizationService: {
      ...actual.organizationService,
      getBranding: () => getBranding(),
      uploadLogo: (...a: unknown[]) => uploadLogo(...a),
      deleteLogo: vi.fn(),
      getLogoBlob: vi.fn(),
    },
  };
});

const toastError = vi.fn();
vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: (...a: unknown[]) => toastError(...a) },
}));

function wrap(ui: React.ReactNode) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

const tr = (_fr: string, en: string) => en;

beforeEach(() => {
  getBranding.mockReset();
  uploadLogo.mockReset();
  toastError.mockReset();
  useUIStore.getState().setVariant('azure');
});

describe('accent presets', () => {
  it('match the variants the design tokens declare for both themes', () => {
    const css = readFileSync(resolve(__dirname, '../../../styles/tokens.css'), 'utf8');
    const dark = [...css.matchAll(/^:root\[data-variant='([a-z]+)'\] \{/gm)].map((m) => m[1]);
    const light = [
      ...css.matchAll(/^:root\[data-theme='light'\]\[data-variant='([a-z]+)'\] \{/gm),
    ].map((m) => m[1]);
    const both = dark.filter((v) => light.includes(v)).sort();
    expect([...ACCENT_PRESETS].sort()).toEqual(both);
  });
});

describe('BrandingSync', () => {
  it('applies the organization accent to the interface', async () => {
    getBranding.mockResolvedValue({ name: 'Org', has_logo: false, accent: 'iris' });
    wrap(<BrandingSync />);
    await waitFor(() => expect(useUIStore.getState().variant).toBe('iris'));
  });

  it('leaves the device accent alone when the organization set none', async () => {
    getBranding.mockResolvedValue({ name: 'Org', has_logo: false });
    wrap(<BrandingSync />);
    await waitFor(() => expect(getBranding).toHaveBeenCalled());
    expect(useUIStore.getState().variant).toBe('azure');
  });
});

describe('OrganizationLogoField', () => {
  const org = { has_logo: false } as OrganizationView;

  it('uploads an accepted image', async () => {
    uploadLogo.mockResolvedValue({ ...org, has_logo: true });
    wrap(<OrganizationLogoField org={org} tr={tr} />);
    const png = new File([new Uint8Array([137, 80, 78, 71])], 'logo.png', { type: 'image/png' });
    await userEvent.upload(screen.getByTestId('org-logo-input'), png);
    await waitFor(() => expect(uploadLogo).toHaveBeenCalledTimes(1));
  });

  it('refuses an SVG or an oversized file in the browser', async () => {
    wrap(<OrganizationLogoField org={org} tr={tr} />);
    const input = screen.getByTestId('org-logo-input');
    await userEvent.upload(input, new File(['<svg/>'], 'a.svg', { type: 'image/svg+xml' }), {
      applyAccept: false,
    });
    await userEvent.upload(
      input,
      new File([new Uint8Array(1024 * 1024 + 1)], 'big.png', { type: 'image/png' }),
    );
    expect(toastError).toHaveBeenCalledTimes(2);
    expect(uploadLogo).not.toHaveBeenCalled();
  });
});
