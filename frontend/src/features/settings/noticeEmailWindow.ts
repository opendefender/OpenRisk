// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #997 — how the notice e-mail grouping window reads on screen.

type Tr = (fr: string, en: string) => string;

export function windowLabel(minutes: number, tr: Tr): string {
  if (minutes % 60 === 0) {
    const h = minutes / 60;
    return tr(`${h} h`, `${h} h`);
  }
  return tr(`${minutes} min`, `${minutes} min`);
}

/** The sentence under the choice: what the window means for one person. */
export function windowSummary(minutes: number, tr: Tr): string {
  const span =
    minutes === 60
      ? tr('toutes les heures', 'every hour')
      : minutes === 1440
        ? tr('par jour', 'a day')
        : minutes % 60 === 0
          ? tr(`toutes les ${minutes / 60} heures`, `every ${minutes / 60} hours`)
          : tr(`toutes les ${minutes} minutes`, `every ${minutes} minutes`);
  return tr(
    `Au plus un e-mail par élément et par personne ${span}. Le premier part tout de suite ; les changements suivants arrivent ensemble, dans un récapitulatif, à la fin de la période.`,
    `At most one e-mail per item and per person ${span}. The first goes out at once; the changes that follow arrive together, in one digest, when the period ends.`,
  );
}
