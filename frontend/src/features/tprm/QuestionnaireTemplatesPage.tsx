// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The tenant's vendor questionnaire templates (#681, ADR 0004 D3).
//
// A tenant starts with none: v1 ships no pre-filled regulatory questionnaire
// (ADR 0004 D8), so the empty state is the first thing most tenants see, and it
// must lead somewhere — to the editor for someone who can manage vendors, and to
// an honest "ask someone who can" for everyone else.

import { useState } from 'react';
import { useNavigate } from 'react-router';
import { Plus } from 'lucide-react';

import { PageFrame, PageHeader, Btn, Card } from '../../shared/ui';
import { FeatureGate } from '../../shared/FeatureGate';
import { Checkbox } from '../../shared/ds';
import { DataTable, useTableState, type Column } from '../../shared/datatable';
import { useI18n } from '../../hooks/useI18n';
import { useAuthStore } from '../../hooks/useAuthStore';
import { localeTag } from '../../i18n';
import { useQuestionnaireTemplates } from './useQuestionnaireTemplates';
import type { QuestionnaireTemplate } from './questionnaireTemplateService';

export function QuestionnaireTemplatesPage() {
  const { t, locale } = useI18n();
  const navigate = useNavigate();
  const canManage = useAuthStore((s) => s.hasPermission('vendors:manage'));
  const [showArchived, setShowArchived] = useState(false);
  const templates = useQuestionnaireTemplates(showArchived);
  const table = useTableState({ urlPrefix: 'qt_', defaultSort: { key: 'name', dir: 'asc' } });

  const openNew = () => navigate('/vendors/questionnaires/new');
  const dateTag = localeTag(locale);

  const columns: Column<QuestionnaireTemplate>[] = [
    {
      key: 'name',
      header: t('vendorTemplates.columns.name'),
      render: (row) => <span className="font-semibold text-ink">{row.name}</span>,
      sortValue: (row) => row.name.toLowerCase(),
      hideable: false,
      frozen: true,
    },
    {
      key: 'language',
      header: t('vendorTemplates.columns.language'),
      render: (row) => t(`vendorTemplates.languages.${row.language === 'en' ? 'en' : 'fr'}`),
      sortValue: (row) => row.language,
    },
    {
      key: 'version',
      header: t('vendorTemplates.columns.version'),
      render: (row) => <span className="tabular-nums">v{row.version}</span>,
      sortValue: (row) => row.version,
      align: 'right',
    },
    {
      key: 'status',
      header: t('vendorTemplates.columns.status'),
      // Status in words, not only in colour.
      render: (row) =>
        row.archived_at ? (
          <span className="text-ink-muted">{t('vendorTemplates.archived')}</span>
        ) : (
          <span className="text-ink">{t('vendorTemplates.active')}</span>
        ),
      sortValue: (row) => (row.archived_at ? 1 : 0),
    },
    {
      key: 'updated_at',
      header: t('vendorTemplates.columns.updated'),
      render: (row) => new Date(row.updated_at).toLocaleDateString(dateTag),
      sortValue: (row) => row.updated_at,
    },
  ];

  const empty = (
    <Card className="p-6 text-center">
      <h2 className="text-[15px] font-bold text-ink mb-1.5">{t('vendorTemplates.emptyTitle')}</h2>
      <p className="text-[13.5px] text-ink-soft mb-4">
        {canManage ? t('vendorTemplates.emptyBody') : t('vendorTemplates.emptyReadOnly')}
      </p>
      {canManage && (
        <Btn primary icon={Plus} label={t('vendorTemplates.create')} onClick={openNew} />
      )}
    </Card>
  );

  return (
    <PageFrame>
      <FeatureGate feature="vendor_risk">
        <PageHeader
          title={t('vendorTemplates.title')}
          count={templates.data ? String(templates.data.length) : null}
          actions={
            canManage ? (
              <Btn primary icon={Plus} label={t('vendorTemplates.create')} onClick={openNew} />
            ) : undefined
          }
        />
        <p className="text-[13.5px] text-ink-soft -mt-2 mb-4">{t('vendorTemplates.subtitle')}</p>

        <div className="mb-3">
          <Checkbox
            label={t('vendorTemplates.showArchived')}
            checked={showArchived}
            onChange={(e) => setShowArchived(e.target.checked)}
          />
        </div>

        <DataTable
          id="vendor-questionnaire-templates"
          mode="client"
          rows={templates.data ?? []}
          columns={columns}
          rowKey={(row) => row.id}
          api={table}
          loading={templates.isLoading}
          error={templates.isError}
          onRetry={() => void templates.refetch()}
          empty={empty}
          clientSearch={(row, q) => row.name.toLowerCase().includes(q.toLowerCase())}
          onRowClick={(row) => navigate(`/vendors/questionnaires/${row.id}`)}
          exportFilename=""
          ariaLabel={t('vendorTemplates.title')}
        />
      </FeatureGate>
    </PageFrame>
  );
}

export default QuestionnaireTemplatesPage;
