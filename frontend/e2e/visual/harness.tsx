// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1

/**
 * Overlay visual harness.
 *
 * Mounts one overlay at a time, chosen by ?overlay=<id>, against the real
 * stylesheet and the real components. Playwright drives it in both themes and
 * compares screenshots.
 *
 * Why a harness rather than the running app: opening these overlays through the
 * UI needs a backend, a session, seeded data and a navigation path per modal.
 * All of that can fail for reasons that have nothing to do with colour, and a
 * visual suite that goes red for unrelated reasons stops being read. Here a
 * diff means exactly one thing — a surface changed.
 *
 * Registering an overlay is what creates its snapshot. e2e/visual/overlays.spec.ts
 * cross-checks this registry against docs/ui/overlays.md, so an overlay added to
 * the app without being registered here fails CI.
 */

import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { AlertTriangle, ShieldOff, Trash2 } from 'lucide-react';

import '../../src/index.css';

import { DangerConfirm } from '../../src/shared/DangerConfirm';
import { ImpactDialog } from '../../src/shared/ImpactDialog';

/**
 * Re-apply the requested theme after the imports above.
 *
 * The overlay components pull in the UI store, which applies its own persisted
 * theme to <html> at module load. That runs after the inline script in
 * harness.html, so without this the store silently wins and both themes would
 * screenshot identically — a suite that passes while testing nothing.
 */
function applyRequestedTheme() {
  const params = new URLSearchParams(window.location.search);
  document.documentElement.setAttribute('data-theme', params.get('theme') ?? 'dark');
  document.documentElement.setAttribute('data-variant', 'azure');
}
applyRequestedTheme();

const noop = () => {};

/**
 * The registry. Each entry renders one overlay in a representative state —
 * populated, not empty, since an empty modal exercises almost no tokens.
 */
export const OVERLAYS: Record<string, () => JSX.Element> = {
  'danger-confirm': () => (
    <DangerConfirm
      open
      onClose={noop}
      onConfirm={noop}
      title="Revoke API token"
      subject="ci-pipeline-token"
      intro="Any integration using this token stops working immediately."
      impact={[
        { label: 'Created', value: '12 March 2026' },
        { label: 'Last used', value: '2 hours ago' },
        { label: 'Scopes', value: 'risks:read, compliance:read' },
      ]}
      alternatives={[
        {
          label: 'Rotate instead — keeps the integration alive',
          description: 'Issues a new secret and gives you 24 hours to migrate.',
          icon: ShieldOff,
          onClick: noop,
        },
      ]}
      confirmLabel="Revoke token"
    />
  ),

  'impact-dialog': () => (
    <ImpactDialog
      open
      onClose={noop}
      onConfirm={noop}
      title="Delete framework"
      subject="ISO/IEC 27001:2022"
      description="Removing a framework also removes the controls imported with it."
      confirmLabel="Delete framework"
      impacts={[
        { label: '93 controls will be removed', icon: <Trash2 size={14} /> },
        {
          label: '41 pieces of evidence lose their link',
          detail: 'The files stay, but no longer prove any control.',
          icon: <AlertTriangle size={14} />,
        },
      ]}
    />
  ),
};

function Harness() {
  const id = new URLSearchParams(window.location.search).get('overlay') ?? '';
  const render = OVERLAYS[id];

  if (!render) {
    return (
      <div style={{ padding: 24, color: 'var(--text-primary)', background: 'var(--surface-0)' }}>
        <p>Unknown overlay: {id || '(none given)'}</p>
        <ul>
          {Object.keys(OVERLAYS).map((key) => (
            <li key={key}>{key}</li>
          ))}
        </ul>
      </div>
    );
  }

  return (
    // A page behind the overlay, so the scrim and elevation are visible. A
    // modal photographed on a blank background hides exactly the token that
    // most often goes wrong.
    <div
      style={{
        minHeight: '100vh',
        background: 'var(--surface-0)',
        color: 'var(--text-primary)',
        padding: 32,
      }}
    >
      <h1 style={{ fontSize: 24, fontWeight: 600 }}>Page behind the overlay</h1>
      <p style={{ color: 'var(--text-secondary)', marginTop: 8 }}>
        Body copy at secondary weight, to show the scrim over real content.
      </p>
      {render()}
    </div>
  );
}

// Once more after mount: the store also applies the theme on rehydrate.
queueMicrotask(applyRequestedTheme);

createRoot(document.getElementById('harness-root')!).render(
  <StrictMode>
    <Harness />
  </StrictMode>,
);
