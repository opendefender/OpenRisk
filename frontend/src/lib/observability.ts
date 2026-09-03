// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Lightweight, dependency-free error reporting. Integrates with Sentry when a
// deployment loads it (the official loader snippet exposes `window.Sentry`), and
// otherwise falls back to the console. This keeps the app bundle free of the
// Sentry SDK unless a deployer opts in, while giving the ErrorBoundary and the
// global handlers a single place to report to.
//
// To enable Sentry, add the loader snippet to index.html (or inject it at deploy
// time) with your DSN — no code change needed here.

interface SentryLike {
  captureException?: (err: unknown, context?: unknown) => void;
  captureMessage?: (msg: string, level?: string) => void;
}

function sentry(): SentryLike | undefined {
  return (window as unknown as { Sentry?: SentryLike }).Sentry;
}

/** Report a caught error. Returns a short correlation id shown to the user. */
export function reportError(error: unknown, context?: Record<string, unknown>): string {
  const id = correlationId();
  const s = sentry();
  if (s?.captureException) {
    s.captureException(error, { extra: { ...context, correlation_id: id } });
  }
  // Always log locally too — Sentry may be absent or the DSN unset.
  // eslint-disable-next-line no-console
  console.error(`[openrisk] error ${id}`, error, context ?? '');
  return id;
}

/** A short, human-quotable id (not a UUID) tying the UI message to the log. */
function correlationId(): string {
  const rnd = Math.floor(Math.random() * 0xfffff)
    .toString(16)
    .padStart(5, '0');
  const t = Date.now().toString(36).slice(-4);
  return `${t}-${rnd}`.toUpperCase();
}

/** Install global handlers for uncaught errors and unhandled promise rejections. */
export function installGlobalErrorReporting(): void {
  window.addEventListener('error', (e) => {
    reportError(e.error ?? e.message, { kind: 'window.onerror' });
  });
  window.addEventListener('unhandledrejection', (e) => {
    reportError(e.reason, { kind: 'unhandledrejection' });
  });
}
