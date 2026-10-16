// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * The words a Listbox says on its own: placeholder, search field, notices.
 *
 * This directory is Apache-2.0 and may not import the AGPL i18n hook, so the
 * strings arrive through context. The app provides them once, translated, at
 * its root (shared/ListboxI18n.tsx); a screen never passes them. Outside that
 * provider (tests, the gallery) the English defaults below apply, as Modal's
 * and Drawer's `closeLabel` do.
 */

import { createContext } from 'react';

export interface ListboxLabels {
  placeholder: string;
  /** Accessible name of the search field. */
  search: string;
  searchPlaceholder: string;
  loading: string;
  empty: string;
  noMatch: string;
}

export const DEFAULT_LISTBOX_LABELS: ListboxLabels = {
  placeholder: 'Choose…',
  search: 'Search the list',
  searchPlaceholder: 'Search…',
  loading: 'Loading…',
  empty: 'No option',
  noMatch: 'No match',
};

export const ListboxLabelsContext = createContext<ListboxLabels>(DEFAULT_LISTBOX_LABELS);
