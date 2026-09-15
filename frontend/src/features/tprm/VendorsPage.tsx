// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The vendor register (#673, backend #669, ADR 0004 D1).
//
// A vendor is an asset of category vendor, so "add a vendor" opens the asset
// form with the Supplier type chosen, which selects that category and its
// schema. The register is paged, searched and filtered by the server.

import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router';
import { useQueryClient } from '@tanstack/react-query';
import { ClipboardList, Plus } from 'lucide-react';

import { PageFrame, PageHeader, Btn, Card } from '../../shared/ui';
import { FeatureGate } from '../../shared/FeatureGate';
import { Badge } from '../../shared/ds';
import { DataTable, useTableState, type Column, type Facet } from '../../shared/datatable';
import { useI18n } from '../../hooks/useI18n';
import { useAuthStore } from '../../hooks/useAuthStore';
import { localeTag } from '../../i18n';
import { CriticalityBadge } from '../assets/CriticalityBadge';
import { CreateAssetModal } from '../assets/CreateAssetModal';
import { VENDORS_KEY, useVendorRegister } from './useVendors';
import { useVendorLabels } from './useVendorLabels';
import { SERVICE_CRITICALITIES, formatNumber, statusIntent, tierIntent } from './vendorDisplay';
import type { VendorListParams, VendorRegisterEntry } from './vendorService';

export function VendorsPage() {
  const { t, locale } = useI18n();
  const labels = useVendorLabels();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const canCreateAsset = useAuthStore((s) => s.hasPermission('assets:create'));
  const [creating, setCreating] = useState(false);

  const table = useTableState({ urlPrefix: 'v_' });
  const { state } = table;
  const params = useMemo<VendorListParams>(() => {
    const p: VendorListParams = {
      limit: state.pageSize,
      offset: (state.page - 1) * state.pageSize,
    };
    const q = state.q.trim();
    if (q) p.search = q;
    const criticality = state.filters.service_criticality?.[0];
    if (criticality) p.service_criticality = criticality;
    return p;
  }, [state]);

  const register = useVendorRegister(params);
  const rows = register.data?.items ?? [];
  const tag = localeTag(locale);

  const date = (value: string) => {
    if (!value) return '—';
    const d = new Date(value);
    return Number.isNaN(d.getTime()) ? value : d.toLocaleDateString(tag);
  };

  // The backend takes one value, so the facet is single-choice.
  const facets: Facet<VendorRegisterEntry>[] = [
    {
      key: 'service_criticality',
      label: t('vendors.columns.serviceCriticality'),
      single: true,
      options: SERVICE_CRITICALITIES.map((v) => ({ value: v, label: labels.serviceCriticality(v) })),
    },
  ];

  const columns: Column<VendorRegisterEntry>[] = [
    {
      key: 'name',
      header: t('vendors.columns.name'),
      frozen: true,
      hideable: false,
      exportValue: (row) => row.name,
      render: (row) => (
        <div className="min-w-0">
          <div className="font-semibold text-ink truncate">{row.name}</div>
          {row.legal_name && row.legal_name !== row.name && (
            <div className="text-[12px] text-ink-muted truncate">{row.legal_name}</div>
          )}
        </div>
      ),
    },
    {
      key: 'service',
      header: t('vendors.columns.service'),
      exportValue: (row) => row.service_provided,
      render: (row) => (
        <span className="line-clamp-2 text-ink-soft">{row.service_provided || '—'}</span>
      ),
    },
    {
      key: 'service_criticality',
      header: t('vendors.columns.serviceCriticality'),
      exportValue: (row) => labels.serviceCriticality(row.service_criticality),
      render: (row) => labels.serviceCriticality(row.service_criticality) || '—',
    },
    {
      key: 'criticality',
      header: t('vendors.columns.criticality'),
      exportValue: (row) => row.criticality,
      render: (row) => <CriticalityBadge level={row.criticality} />,
    },
    {
      key: 'linked_assets',
      header: t('vendors.columns.linkedAssets'),
      align: 'right',
      exportValue: (row) => row.linked_assets,
      render: (row) => <span className="tabular-nums">{row.linked_assets}</span>,
    },
    {
      key: 'assessment',
      header: t('vendors.columns.assessment'),
      exportValue: (row) => {
        const la = row.latest_assessment;
        if (!la) return '';
        return la.score !== null && la.score !== undefined
          ? `${labels.status(la.status)} · ${la.score} · ${labels.tier(la.tier)}`
          : labels.status(la.status);
      },
      render: (row) => {
        const la = row.latest_assessment;
        if (!la) return <span className="text-ink-muted">{t('vendors.noAssessment')}</span>;
        return (
          <span className="inline-flex flex-wrap items-center gap-1.5">
            <Badge intent={statusIntent(la.status)}>{labels.status(la.status)}</Badge>
            {la.score !== null && la.score !== undefined && (
              <Badge intent={tierIntent(la.tier)}>
                {formatNumber(la.score, tag)} · {labels.tier(la.tier)}
              </Badge>
            )}
          </span>
        );
      },
    },
    {
      key: 'contract_end',
      header: t('vendors.columns.contractEnd'),
      exportValue: (row) => row.contract_end,
      render: (row) => date(row.contract_end),
    },
    {
      key: 'owner',
      header: t('vendors.columns.owner'),
      defaultHidden: true,
      exportValue: (row) => row.owner,
      render: (row) => row.owner || '—',
    },
    {
      key: 'country',
      header: t('vendors.columns.country'),
      defaultHidden: true,
      exportValue: (row) => row.country,
      render: (row) => row.country || '—',
    },
  ];

  const empty = (
    <Card className="p-6 text-center">
      <h2 className="text-[15px] font-bold text-ink mb-1.5">{t('vendors.emptyTitle')}</h2>
      <p className="text-[13.5px] text-ink-soft mb-4 max-w-xl mx-auto">
        {canCreateAsset ? t('vendors.emptyBody') : t('vendors.emptyReadOnly')}
      </p>
      {canCreateAsset && (
        <Btn primary icon={Plus} label={t('vendors.addVendor')} onClick={() => setCreating(true)} />
      )}
    </Card>
  );

  return (
    <PageFrame>
      <FeatureGate feature="vendor_risk">
        <PageHeader
          title={t('vendors.title')}
          count={register.data ? String(register.data.total) : null}
          actions={
            <>
              <Btn
                icon={ClipboardList}
                label={t('vendors.questionnaires')}
                onClick={() => navigate('/vendors/questionnaires')}
              />
              {canCreateAsset && (
                <Btn
                  primary
                  icon={Plus}
                  label={t('vendors.addVendor')}
                  onClick={() => setCreating(true)}
                />
              )}
            </>
          }
        />
        <p className="text-[13.5px] text-ink-soft -mt-2 mb-4">{t('vendors.subtitle')}</p>

        <DataTable
          id="vendors"
          mode="server"
          rows={rows}
          total={register.data?.total ?? 0}
          columns={columns}
          facets={facets}
          rowKey={(row) => row.id}
          api={table}
          loading={register.isLoading}
          error={register.isError}
          onRetry={() => void register.refetch()}
          empty={empty}
          searchPlaceholder={t('vendors.searchPlaceholder')}
          onRowClick={(row) => navigate(`/vendors/${row.id}`)}
          exportFilename="vendors"
          pageSizeOptions={[25, 50, 100, 200]}
          minWidth={900}
          ariaLabel={t('vendors.title')}
        />

        <CreateAssetModal
          isOpen={creating}
          initialType="Supplier"
          onClose={() => {
            setCreating(false);
            void qc.invalidateQueries({ queryKey: VENDORS_KEY });
          }}
        />
      </FeatureGate>
    </PageFrame>
  );
}

export default VendorsPage;
