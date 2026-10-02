// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// CSV import of the asset inventory (#861). The page is the shared
// CsvImportPage, with the same all-or-nothing contract as the risk import.

import { useQueryClient } from '@tanstack/react-query';

import { useI18n } from '../../hooks/useI18n';
import { CsvImportPage, type CsvImportConfig } from '../../shared/csvImport/CsvImportPage';
import { ASSETS_QUERY_KEY } from './useAssets';
import { ASSET_IMPORT_TEMPLATE } from './importAssetsTemplate';

export const ImportAssetsPage = () => {
  const { t } = useI18n();
  const queryClient = useQueryClient();

  const config: CsvImportConfig = {
    endpoint: '/assets/import',
    back: { to: '/assets', label: t('csvImport.assets.back') },
    title: t('csvImport.assets.title'),
    intro: t('csvImport.assets.intro'),
    columns: [
      { name: 'name', help: t('csvImport.assets.colName') },
      { name: 'criticality', help: t('csvImport.assets.colCriticality') },
      { name: 'type, owner', help: t('csvImport.assets.colOptional') },
    ],
    note: t('csvImport.assets.note'),
    template: ASSET_IMPORT_TEMPLATE,
    templateFilename: 'openrisk-assets-template.csv',
    open: { to: '/assets', label: t('csvImport.assets.open') },
    created: (count) => t('csvImport.assets.created', { count }),
    emptyFile: t('csvImport.assets.emptyFile'),
    forbidden: t('csvImport.assets.forbidden'),
    limitTitle: t('csvImport.assets.limitTitle'),
    limitDetail: (requested, remaining) =>
      t('csvImport.assets.limitDetail', { requested, remaining }),
    onCreated: () => void queryClient.invalidateQueries({ queryKey: ASSETS_QUERY_KEY }),
  };

  return <CsvImportPage config={config} />;
};

export default ImportAssetsPage;
