// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The copy for the framework cap (D-076), shared by the notice and the toast a
// refused create shows.

/** The sentence for a reached cap, shared with the toast a refused create shows. */
export function frameworkCapMessage(fr: boolean, limit: number, used: number): string {
  if (fr) {
    return limit === 1
      ? `Votre plan garde 1 référentiel actif à la fois (${used}/${limit}). Supprimez-en un ou passez à un plan supérieur pour en ajouter.`
      : `Votre plan garde ${limit} référentiels actifs à la fois (${used}/${limit}). Supprimez-en un ou passez à un plan supérieur pour en ajouter.`;
  }
  return limit === 1
    ? `Your plan keeps 1 framework active at a time (${used}/${limit}). Delete one or upgrade to add another.`
    : `Your plan keeps ${limit} frameworks active at a time (${used}/${limit}). Delete one or upgrade to add another.`;
}
