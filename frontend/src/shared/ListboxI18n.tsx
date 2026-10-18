// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Hands the design-system Listbox its words in the user's language (#935).
 *
 * shared/ds is Apache-2.0 and cannot import the AGPL i18n hook, so the strings
 * cross the licence boundary the allowed way: this AGPL file reads the
 * catalogue and provides them through the Apache context, once, at the root.
 */

import { useMemo, type ReactNode } from 'react';
import { useI18n } from '../hooks/useI18n';
import { ListboxLabelsContext, type ListboxLabels } from './ds/listboxLabels';

export function ListboxI18nProvider({ children }: { children: ReactNode }) {
  const { t } = useI18n();
  const labels = useMemo<ListboxLabels>(
    () => ({
      placeholder: t('common.listbox.placeholder', 'Choisir…'),
      search: t('common.listbox.search', 'Rechercher dans la liste'),
      searchPlaceholder: t('common.listbox.searchPlaceholder', 'Rechercher…'),
      loading: t('common.listbox.loading', 'Chargement…'),
      empty: t('common.listbox.empty', 'Aucune option'),
      noMatch: t('common.listbox.noMatch', 'Aucun résultat'),
    }),
    [t],
  );
  return <ListboxLabelsContext.Provider value={labels}>{children}</ListboxLabelsContext.Provider>;
}
