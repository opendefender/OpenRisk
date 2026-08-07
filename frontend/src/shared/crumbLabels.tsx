// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Instance names for dynamic breadcrumbs.
//
// A crumb for /compliance/frameworks/:id should read "ISO/IEC 27001", not
// "Framework". Only the page rendering that route knows the name, and it knows
// it after its own fetch resolves — so the page registers the label and the
// breadcrumb picks it up. Until then the crumb shows the static label, which
// sharpens rather than flickering from empty.
//
// A tiny module-level store rather than context: the breadcrumb lives in the
// header, far above the page in the tree, and threading a provider through the
// whole shell to carry one string per route would be more machinery than the
// problem deserves.

import { useEffect, useSyncExternalStore } from 'react';

type Labels = Record<string, string>;

let labels: Labels = {};
const listeners = new Set<() => void>();

function emit() {
  for (const l of listeners) l();
}

function subscribe(fn: () => void) {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

function snapshot(): Labels {
  return labels;
}

/** Reads the registered instance labels, keyed by concrete href. */
export function useCrumbLabels(): Labels {
  return useSyncExternalStore(subscribe, snapshot, snapshot);
}

/**
 * Registers the human name for the route the caller is rendering.
 *
 * `href` must be the concrete path (…/frameworks/abc), not the pattern. Pass a
 * falsy name while loading; the entry is dropped on unmount so a stale name
 * never survives onto a different record.
 */
export function useRegisterCrumb(href: string | null | undefined, name: string | null | undefined) {
  useEffect(() => {
    if (!href || !name) return;
    if (labels[href] === name) return;
    labels = { ...labels, [href]: name };
    emit();
    return () => {
      if (!(href in labels)) return;
      const next = { ...labels };
      delete next[href];
      labels = next;
      emit();
    };
  }, [href, name]);
}
