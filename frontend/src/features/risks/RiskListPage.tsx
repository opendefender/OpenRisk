// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useEffect, useMemo, useState, useCallback } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { ChevronLeft, ChevronRight, ChevronUp, ChevronDown, Search, Settings2, Plus, X } from 'lucide-react';
import { useRisks } from './useRisks';
import { useRiskUIStore } from './store';
import { useDebounce } from '../../hooks/useDebounce';
import { useKeyboardShortcuts } from '../../hooks/useKeyboard';
import { useToast } from '../../hooks/useToast';
import { useSSE } from '../../hooks/useSSE';
import { useI18n } from '../../hooks/useI18n';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { EmptyState, FloatingBulkBar, RiskBadge, ScoreMeter, StatusDot, SkeletonTable, UserAvatar } from '../../components/shared';
import { CreateRiskModal } from './CreateRiskModal';
import { RiskDrawer } from './RiskDrawer';
import { type Risk } from '../../services/riskService';

const sortFields = ['title', 'score', 'impact', 'probability', 'status', 'created_at'] as const;

export const RiskListPage = () => {
  const { t } = useI18n();
  const { success, error } = useToast();

  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [sortBy, setSortBy] = useState<typeof sortFields[number]>('score');
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc');
  const [query, setQuery] = useState('');
  const [focusedIndex, setFocusedIndex] = useState<number>(0);

  const debouncedQuery = useDebounce(query, 300);

  const {
    filters,
    selectedIds,
    isCreateModalOpen,
    isDrawerOpen,
    drawerRiskId,
    activeDrawerTab,
    showFilterPanel,
    setFilters,
    clearFilters,
    toggleSelection,
    setSelectedIds,
    clearSelection,
    openCreateModal,
    closeCreateModal,
    openDrawer,
    closeDrawer,
    setActiveDrawerTab,
    setShowFilterPanel,
  } = useRiskUIStore();

  const queryParams = useMemo(
    () => ({
      q: debouncedQuery || undefined,
      status: filters.status,
      framework: filters.framework,
      assigned_to: filters.assignedTo,
      created_by: filters.createdBy,
      source: filters.source,
      tag: filters.tag,
      min_score: filters.minScore,
      max_score: filters.maxScore,
      date_from: filters.dateFrom,
      date_to: filters.dateTo,
      page,
      limit: pageSize,
      sort_by: sortBy,
      sort_dir: sortDir,
    }),
    [debouncedQuery, filters, page, pageSize, sortBy, sortDir]
  );

  const {
    risks,
    total,
    isLoading,
    refetch,
    deleteRisk,
    duplicateRisk,
    acceptRisk,
    updateRisk,
  } = useRisks(queryParams);

  const totalPages = useMemo(() => Math.max(1, Math.ceil(total / pageSize)), [pageSize, total]);
  const selectedCount = selectedIds.length;

  const selectedRisk = useMemo(
    () => risks.find((risk) => risk.id === drawerRiskId) ?? null,
    [drawerRiskId, risks]
  );

  useEffect(() => {
    if (page > totalPages) setPage(totalPages);
  }, [page, totalPages]);

  // Live updates are a progressive enhancement: if the SSE stream is unavailable the
  // list still works (it refetches on every mutation). A dropped/absent stream must NOT
  // surface as a user-facing error — doing so previously produced bursts of "server error"
  // toasts because EventSource reconnects and each attempt fired a toast. useSSE now caps
  // reconnects; we simply log here instead of alarming the user.
  useSSE({
    url: '/api/v1/risks/events',
    enabled: true,
    onMessage: (event) => {
      if (['risk.created', 'risk.updated', 'risk.score_updated'].includes(event.type)) {
        refetch();
      }
    },
    onError: () => {
      if (import.meta.env.DEV) console.debug('[risks] realtime stream unavailable — falling back to on-demand refetch');
    },
  });

  useKeyboardShortcuts([
    {
      options: { key: 'n' },
      callback: () => openCreateModal(),
    },
    {
      options: { key: 'Escape' },
      callback: () => {
        closeCreateModal();
        if (isDrawerOpen) closeDrawer();
      },
    },
    {
      options: { key: 'ArrowDown' },
      callback: () => setFocusedIndex((prev) => Math.min(prev + 1, risks.length - 1)),
    },
    {
      options: { key: 'ArrowUp' },
      callback: () => setFocusedIndex((prev) => Math.max(prev - 1, 0)),
    },
    {
      options: { key: 'Enter' },
      callback: () => {
        const risk = risks[focusedIndex];
        if (risk) {
          openDrawer(risk.id);
        }
      },
    },
  ]);

  const handleHeaderSort = (field: typeof sortFields[number]) => {
    setSortBy(field);
    setSortDir((current) => (current === 'asc' ? 'desc' : 'asc'));
  };

  const handleBulkDelete = async () => {
    if (!selectedCount) return;
    if (!confirm(`${selectedCount} ${t('common.delete')} ?`)) return;
    try {
      await Promise.all(selectedIds.map((id) => deleteRisk.mutateAsync(id)));
      success(t('messages.riskDeletedSuccess'));
      clearSelection();
    } catch {
      error(t('errors.failedToDeleteRisk'));
    }
  };

  const handleBulkExport = async () => {
    try {
      const response = await fetch(`/api/v1/risks/export?format=csv&${new URLSearchParams({ q: debouncedQuery || '' })}`);
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `risks-${new Date().toISOString().slice(0, 10)}.csv`;
      a.click();
      URL.revokeObjectURL(url);
      success(t('messages.exportCompleted'));
    } catch {
      error(t('errors.failedToExportRisks'));
    }
  };

  const activeFilters = useMemo(() => {
    return Object.entries(filters).filter(([, value]) => value !== undefined && value !== '');
  }, [filters]);

  return (
    // h-full + overflow-y-auto instead of min-h-screen: the page sits inside an
    // overflow-hidden layout main, so min-h-screen just overflowed and got clipped
    // (no scrollbar, content jumped). This makes the page its own scroll area.
    <div className="h-full overflow-y-auto bg-background text-white">
      <div className="max-w-7xl mx-auto px-4 py-6 sm:px-6 lg:px-8">
        <div className="flex flex-col gap-6 lg:flex-row lg:items-center lg:justify-between">
          <div className="space-y-2">
            <h1 className="text-3xl font-semibold">{t('risks.title')}</h1>
            <p className="text-sm text-zinc-400 max-w-2xl">{t('risks.description')}</p>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <Button onClick={openCreateModal} variant="secondary" className="gap-2">
              <Plus size={16} /> {t('risks.createNew')}
            </Button>
            <Button onClick={() => setShowFilterPanel(!showFilterPanel)} variant={showFilterPanel ? 'secondary' : 'ghost'} className="gap-2">
              <Settings2 size={16} /> {t('common.filter')}
            </Button>
          </div>
        </div>

        <div className="mt-6 grid gap-4 xl:grid-cols-[320px_1fr]">
          <AnimatePresence>
            {showFilterPanel && (
              <motion.aside
                initial={{ opacity: 0, x: -24 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -24 }}
                className="rounded-3xl border border-zinc-800 bg-zinc-950/80 p-5 shadow-xl shadow-black/30"
              >
                <div className="flex items-center justify-between gap-3 mb-5">
                  <h2 className="text-sm font-semibold uppercase tracking-[0.15em] text-zinc-400">Filtres</h2>
                  <button type="button" onClick={clearFilters} className="text-xs text-primary hover:text-primary/80">{t('filters.clearAll')}</button>
                </div>
                <div className="space-y-4">
                  <div className="space-y-2">
                    <label className="text-xs font-semibold uppercase tracking-[0.16em] text-zinc-500">{t('filters.status')}</label>
                    <select
                      value={filters.status || ''}
                      onChange={(event) => setFilters({ status: event.target.value as any || undefined })}
                      className="w-full rounded-2xl border border-zinc-800 bg-zinc-950 px-4 py-3 text-sm text-white"
                    >
                      <option value="">Toutes</option>
                      <option value="open">{t('statuses.open')}</option>
                      <option value="in_progress">{t('statuses.in_progress')}</option>
                      <option value="mitigated">{t('statuses.mitigated')}</option>
                      <option value="accepted">{t('statuses.accepted')}</option>
                      <option value="closed">{t('statuses.closed')}</option>
                    </select>
                  </div>
                  <div className="space-y-2">
                    <label className="text-xs font-semibold uppercase tracking-[0.16em] text-zinc-500">{t('filters.framework')}</label>
                    <select
                      value={filters.framework || ''}
                      onChange={(event) => setFilters({ framework: event.target.value || undefined })}
                      className="w-full rounded-2xl border border-zinc-800 bg-zinc-950 px-4 py-3 text-sm text-white"
                    >
                      <option value="">Toutes</option>
                      <option value="iso27001">{t('frameworks.iso27001')}</option>
                      <option value="cis">{t('frameworks.cis')}</option>
                      <option value="nist">{t('frameworks.nist')}</option>
                      <option value="owasp">{t('frameworks.owasp')}</option>
                    </select>
                  </div>
                  <div className="space-y-2">
                    <label className="text-xs font-semibold uppercase tracking-[0.16em] text-zinc-500">{t('filters.scoreRange')}</label>
                    <div className="grid grid-cols-2 gap-3">
                      <Input
                        placeholder="Min"
                        type="number"
                        value={filters.minScore ?? ''}
                        onChange={(event) => setFilters({ minScore: event.target.value ? Number(event.target.value) : undefined })}
                        className="rounded-2xl"
                      />
                      <Input
                        placeholder="Max"
                        type="number"
                        value={filters.maxScore ?? ''}
                        onChange={(event) => setFilters({ maxScore: event.target.value ? Number(event.target.value) : undefined })}
                        className="rounded-2xl"
                      />
                    </div>
                  </div>
                </div>
              </motion.aside>
            )}
          </AnimatePresence>

          <div className="space-y-4">
            <div className="grid gap-4 md:grid-cols-[1fr_220px] items-end">
              <div className="relative">
                <Search className="absolute left-4 top-1/2 -translate-y-1/2 text-zinc-500" size={16} />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder={t('common.search')}
                  className="pl-11"
                />
              </div>
              <div className="flex flex-wrap gap-2 items-center justify-end">
                <Button onClick={handleBulkExport} variant="ghost" className="gap-2">
                  <Plus size={16} /> {t('common.export')}
                </Button>
                <select
                  value={pageSize}
                  onChange={(event) => { setPageSize(Number(event.target.value)); setPage(1); }}
                  className="rounded-2xl border border-zinc-800 bg-zinc-950 px-4 py-3 text-sm text-white"
                >
                  <option value={10}>10</option>
                  <option value={20}>20</option>
                  <option value={50}>50</option>
                </select>
              </div>
            </div>

            {isLoading ? (
              <SkeletonTable rows={6} columns={8} />
            ) : risks.length === 0 ? (
              <EmptyState
                icon="📌"
                title={t('risks.noRisks')}
                description={t('risks.noRisksDescription')}
                action={{ label: t('risks.createFirstRisk'), onClick: openCreateModal }}
              />
            ) : (
              <div className="rounded-3xl border border-zinc-800 bg-zinc-950/70 overflow-hidden">
                <div className="overflow-x-auto scrollbar-thin">
                <div className="min-w-[960px]">
                <div className="grid grid-cols-[48px_2fr_120px_120px_180px_140px_180px_120px] gap-0 bg-zinc-900 border-b border-zinc-800 text-xs uppercase tracking-[0.18em] text-zinc-500">
                  <div className="px-4 py-3">
                    <input
                      type="checkbox"
                      aria-label="Select all risks"
                      checked={selectedCount === risks.length}
                      onChange={(event) => {
                        if (event.target.checked) {
                          setSelectedIds(risks.map((risk) => risk.id));
                        } else {
                          clearSelection();
                        }
                      }}
                      className="rounded"
                    />
                  </div>
                  <div className="px-4 py-3">{t('risks.riskName')}</div>
                  <div className="px-4 py-3">
                    <button type="button" onClick={() => handleHeaderSort('score')} className="flex items-center gap-2">
                      {t('risks.riskScore')}
                      {sortBy === 'score' && (sortDir === 'asc' ? <ChevronUp size={14} /> : <ChevronDown size={14} />)}
                    </button>
                  </div>
                  <div className="px-4 py-3">{t('risks.riskStatus')}</div>
                  <div className="px-4 py-3">{t('risks.riskFramework')}</div>
                  <div className="px-4 py-3">{t('risks.riskAssignedTo')}</div>
                  <div className="px-4 py-3">{t('risks.riskUpdatedAt')}</div>
                  <div className="px-4 py-3">{t('common.actions')}</div>
                </div>

                <div className="divide-y divide-zinc-800">
                  {risks.map((risk, index) => (
                    <motion.div
                      key={risk.id}
                      layout
                      initial={{ opacity: 0, y: 10 }}
                      animate={{ opacity: 1, y: 0 }}
                      whileHover={{ backgroundColor: 'rgba(255,255,255,0.03)' }}
                      className={`grid grid-cols-[48px_2fr_120px_120px_180px_140px_180px_120px] items-center gap-0 px-4 py-4 text-sm text-zinc-200 transition-colors ${focusedIndex === index ? 'bg-white/5' : ''}`}
                      tabIndex={0}
                      onFocus={() => setFocusedIndex(index)}
                      onClick={() => openDrawer(risk.id)}
                    >
                      <div>
                        <input
                          type="checkbox"
                          checked={selectedIds.includes(risk.id)}
                          onChange={(event) => { event.stopPropagation(); toggleSelection(risk.id); }}
                          className="rounded"
                          aria-label={`Select risk ${risk.title}`}
                        />
                      </div>
                      <div className="space-y-1">
                        <div className="font-semibold text-white truncate">{risk.title}</div>
                        <div className="text-xs text-zinc-500 truncate">{risk.description}</div>
                      </div>
                      <div>
                        <div className="flex items-center gap-2">
                          <ScoreMeter score={risk.score} maxScore={100} size="sm" showLabel={false} />
                          <span className="text-xs text-zinc-400">{risk.score.toFixed(1)}</span>
                        </div>
                      </div>
                      <div><StatusDot status={risk.status} size="sm" withLabel /></div>
                      <div className="text-xs text-zinc-300 truncate">{risk.frameworks?.[0] ?? '-'}</div>
                      <div>
                        {risk.assigned_to ? <UserAvatar name={risk.assigned_to} size="sm" tooltip={false} /> : <span className="text-xs text-zinc-500">-</span>}
                      </div>
                      <div className="text-xs text-zinc-500">{risk.updated_at ? new Date(risk.updated_at).toLocaleDateString() : '-'}</div>
                      <div className="flex items-center gap-2">
                        <Button onClick={(event) => { event.stopPropagation(); openDrawer(risk.id); }} variant="ghost" className="text-xs py-1 px-2">{t('common.edit')}</Button>
                      </div>
                    </motion.div>
                  ))}
                </div>
                </div>
                </div>
              </div>
            )}

            {risks.length > 0 && (
              <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                <div className="text-sm text-zinc-400">{total} risques trouvés</div>
                <div className="flex items-center gap-2">
                  <Button onClick={() => setPage(Math.max(1, page - 1))} variant="ghost" disabled={page === 1}>
                    <ChevronLeft size={16} />
                  </Button>
                  <span className="text-sm text-zinc-300">{page} / {totalPages}</span>
                  <Button onClick={() => setPage(Math.min(totalPages, page + 1))} variant="ghost" disabled={page === totalPages}>
                    <ChevronRight size={16} />
                  </Button>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      <FloatingBulkBar
        selectedCount={selectedCount}
        isVisible={selectedCount > 0}
        onCancel={clearSelection}
        onConfirm={handleBulkDelete}
        actions={[{ id: 'delete', label: t('risks.bulkDelete'), icon: <X size={16} />, variant: 'danger' }]}
      />

      <CreateRiskModal isOpen={isCreateModalOpen} onClose={closeCreateModal} />
      <RiskDrawer
        risk={selectedRisk}
        isOpen={isDrawerOpen}
        onClose={closeDrawer}
        onDelete={async (id) => { await deleteRisk.mutateAsync(id); closeDrawer(); }}
        onDuplicate={async (id) => { await duplicateRisk.mutateAsync(id); }}
        onAccept={async (id, reason) => { await acceptRisk.mutateAsync({ id, justification: reason }); }}
        onUpdate={async (id, payload) => { await updateRisk.mutateAsync({ id, payload }); }}
      />
    </div>
  );
};

export default RiskListPage;
