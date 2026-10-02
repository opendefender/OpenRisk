// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// CSV import of the risk register (#755). The page itself is the shared
// CsvImportPage; this file says what is particular to risks.

import { useRiskStore } from '../hooks/useRiskStore';
import { useI18n } from '../hooks/useI18n';
import { CsvImportPage, type CsvImportConfig } from '../shared/csvImport/CsvImportPage';
import { IMPORT_TEMPLATE } from '../features/risks/importRisksSchema';

export const ImportRisksPage = () => {
  const { t } = useI18n();
  const { fetchRisks } = useRiskStore();

  const config: CsvImportConfig = {
    endpoint: '/risks/import',
    back: { to: '/risks', label: t('csvImport.risks.back') },
    title: t('csvImport.risks.title'),
    intro: t('csvImport.risks.intro'),
    columns: [
      { name: 'title', help: t('csvImport.risks.colTitle') },
      { name: 'probability', help: t('csvImport.risks.colProbability') },
      { name: 'impact', help: t('csvImport.risks.colImpact') },
      { name: 'description, tags, frameworks', help: t('csvImport.risks.colOptional') },
      { name: 'assets', help: t('csvImport.risks.colAssets') },
    ],
    note: t('csvImport.risks.note'),
    template: IMPORT_TEMPLATE,
    templateFilename: 'openrisk-risks-template.csv',
    open: { to: '/risks', label: t('csvImport.risks.open') },
    created: (count) => t('csvImport.risks.created', { count }),
    emptyFile: t('csvImport.risks.emptyFile'),
    forbidden: t('csvImport.risks.forbidden'),
    limitTitle: t('csvImport.risks.limitTitle'),
    limitDetail: (requested, remaining) =>
      t('csvImport.risks.limitDetail', { requested, remaining }),
    onCreated: () => void fetchRisks(),
  };

  return <CsvImportPage config={config} />;
};

export default ImportRisksPage;
