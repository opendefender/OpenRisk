// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useEffect, useMemo, useState, useCallback } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  ChevronDown,
  Search,
  Plus,
  Settings2,
  Download,
  Upload,
} from 'lucide-react';
import { useRiskStore, type Risk } from '../hooks/useRiskStore';
import { useDebounce } from '../hooks/useDebounce';
import { useKeyboardShortcuts } from '../hooks/useKeyboard';
import { useToast } from '../hooks/useToast';
import { useSSE } from '../hooks/useSSE';
import { useI18n } from '../hooks/useI18n';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { ViewToggle } from '../components/ViewToggle';
import {
  RiskBadge,
  ScoreMeter,
  StatusDot,
  FloatingBulkBar,
  EmptyState,
  SkeletonTable,
  UserAvatar,
} from '../components/shared';
import { EditRiskModal } from '../features/risks/components/EditRiskModal';
import { CreateRiskModal } from '../features/risks/components/CreateRiskModal';
import { RiskDetails } from '../features/risks/components/RiskDetails';
import { Drawer } from '../components/ui/Drawer';

type SortField = 'title' | 'score' | 'impact' | 'probability' | 'status' | 'created_at';
type SortDir = 'asc' | 'desc';
type ViewMode = 'table' | 'card';

interface FilterState {
  status?: string;
  minScore?: number;
  maxScore?: number;
  framework?: string;
  tag?: string;
  assignedTo?: string;
  createdBy?: string;
  dateFrom?: string;
  dateTo?: string;
}

export const RiskListPage = () => {
  const { t } = useI18n();
  const { risks, total, page, pageSize, isLoading, selectedRisk, setSelectedRisk, fetchRisks } =
    useRiskStore();
  const { success, error } = useToast();

  // State
  const [localPage, setLocalPage] = useState(1);
  const [localPageSize, setLocalPageSize] = useState(20);
  const [sortBy, setSortBy] = useState<SortField>('score');
  const [sortDir, setSortDir] = useState<SortDir>('desc');
  const [view, setView] = useState<ViewMode>(() => {
    const saved = localStorage.getItem('riskView');
    return (saved as ViewMode) || 'table';
  });
  const [searchQuery, setSearchQuery] = useState('');
  const [filters, setFilters] = useState<FilterState>({});
  const [showFilters, setShowFilters] = useState(false);
  const [selectedRisks, setSelectedRisks] = useState<Set<string>>(new Set());
  const [editRisk, setEditRisk] = useState<Risk | null>(null);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showRiskDrawer, setShowRiskDrawer] = useState(false);

  const debouncedSearch = useDebounce(searchQuery, 300);

  // Save view preference
  useEffect(() => {
    localStorage.setItem('riskView', view);
  }, [view]);

  // Fetch risks on params change
  useEffect(() => {
    fetchRisks({
      page: localPage,
      limit: localPageSize,
      sort_by: sortBy,
      sort_dir: sortDir,
      q: debouncedSearch || undefined,
      ...filters,
    });
  }, [localPage, localPageSize, sortBy, sortDir, debouncedSearch, filters, fetchRisks]);

  // SSE integration for real-time updates
  const handleSSEMessage = useCallback(
    (event: any) => {
      if (event.type === 'risk.created') {
        success(t('messages.riskCreatedSuccess'));
        fetchRisks();
      } else if (event.type === 'risk.updated') {
        success(t('messages.riskUpdatedSuccess'));
        fetchRisks();
      } else if (event.type === 'risk.score_updated') {
        // Silently update score in UI
        fetchRisks();
      }
    },
    [success, fetchRisks, t]
  );

  useSSE({
    url: '/api/v1/risks/events',
    enabled: true,
    onMessage: handleSSEMessage,
  });

  // Keyboard shortcuts
  useKeyboardShortcuts([
    {
      options: { key: 'n', ctrl: true },
      callback: () => setShowCreateModal(true),
    },
    {
      options: { key: 'Escape' },
      callback: () => {
        setShowCreateModal(false);
        setShowRiskDrawer(false);
      },
    },
  ]);

  // Calculations
  const totalPages = useMemo(() => Math.max(1, Math.ceil(total / localPageSize)), [total, localPageSize]);
  const hasSelectedRisks = selectedRisks.size > 0;

  // Toggle risk selection
  const toggleRiskSelection = (riskId: string) => {
    setSelectedRisks((prev) => {
      const next = new Set(prev);
      if (next.has(riskId)) next.delete(riskId);
      else next.add(riskId);
      return next;
    });
  };

  // Bulk actions
  const handleBulkDelete = async () => {
    if (!confirm(`${t('common.delete')} ${selectedRisks.size} risks?`)) return;

    try {
      // TODO: Implement bulk delete endpoint
      await Promise.all(
        Array.from(selectedRisks).map((id) =>
          fetch(`/api/v1/risks/${id}`, { method: 'DELETE' })
        )
      );
      success(`${selectedRisks.size} ${t('common.delete')}d`);
      setSelectedRisks(new Set());
      fetchRisks();
    } catch (err) {
      error(t('errors.failedToDeleteRisk'));
    }
  };

  const handleExport = async () => {
    try {
      const response = await fetch(
        `/api/v1/risks/export?format=csv&${new URLSearchParams(filters as any)}`
      );
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `risks-${new Date().toISOString().split('T')[0]}.csv`;
      a.click();
      window.URL.revokeObjectURL(url);
      success(t('messages.exportCompleted'));
    } catch (err) {
      error(t('errors.failedToExportRisks'));
    }
  };

  // Render table header cell
  const renderHeaderCell = (field: SortField, label: string) => (
    <button
      type="button"
      className="flex items-center gap-2 focus:outline-none hover:text-white transition-colors"
      onClick={() => {
        if (sortBy === field) setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
        setSortBy(field);
      }}
      aria-sort={sortBy === field ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'}
    >
      <span>{label}</span>
      {sortBy === field && (sortDir === 'asc' ? <ChevronUp size={14} /> : <ChevronDown size={14} />)}
    </button>
  );

  return (
    <div className="max-w-full min-h-screen bg-gradient-to-b from-zinc-950 to-black">
      <div className="max-w-7xl mx-auto p-6">
        {/* Header */}
        <div className="mb-8">
          <div className="flex items-center justify-between mb-6">
            <div>
              <h1 className="text-4xl font-bold text-white">{t('risks.title')}</h1>
              <p className="text-zinc-400 mt-2">{t('risks.description')}</p>
            </div>
            <div className="flex items-center gap-2">
              <Button onClick={handleExport} variant="ghost" className="gap-2">
                <Download size={16} />
                {t('common.export')}
              </Button>
              <Button
                onClick={() => (window.location.href = '/import-risks')}
                variant="ghost"
                className="gap-2"
              >
                <Upload size={16} />
                {t('risks.import')}
              </Button>
              <Button
                onClick={() => setShowCreateModal(true)}
                variant="secondary"
                className="gap-2"
              >
                <Plus size={16} />
                {t('risks.createNew')}
              </Button>
            </div>
          </div>

          {/* Controls */}
          <div className="flex items-center gap-4 flex-wrap">
            {/* Search */}
            <div className="flex-1 min-w-xs relative">
              <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-zinc-500" size={16} />
              <Input
                placeholder={t('common.search')}
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-10"
              />
            </div>

            {/* View toggle */}
            <ViewToggle view={view} onViewChange={setView} />

            {/* Filters toggle */}
            <Button
              onClick={() => setShowFilters(!showFilters)}
              variant={showFilters ? 'secondary' : 'ghost'}
              className="gap-2"
            >
              <Settings2 size={16} />
              {t('common.filter')}
            </Button>

            {/* Page size selector */}
            <select
              value={localPageSize}
              onChange={(e) => {
                setLocalPageSize(Number(e.target.value));
                setLocalPage(1);
              }}
              className="bg-zinc-900 border border-zinc-700 rounded-md px-3 py-2 text-sm text-zinc-300 hover:border-zinc-600 transition-colors"
            >
              <option value={10}>10</option>
              <option value={20}>20</option>
              <option value={50}>50</option>
              <option value={100}>100</option>
            </select>
          </div>
        </div>

        {/* Filters panel */}
        <AnimatePresence>
          {showFilters && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              className="mb-6 p-4 bg-zinc-900/50 border border-zinc-800 rounded-lg space-y-4"
            >
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                {/* Status filter */}
                <div>
                  <label className="text-xs text-zinc-400 uppercase mb-2 block">{t('filters.status')}</label>
                  <select
                    value={filters.status || ''}
                    onChange={(e) => setFilters((prev) => ({ ...prev, status: e.target.value || undefined }))}
                    className="w-full bg-zinc-800 border border-zinc-700 rounded px-3 py-2 text-sm text-zinc-300"
                  >
                    <option value="">All</option>
                    <option value="open">{t('statuses.open')}</option>
                    <option value="in_progress">{t('statuses.in_progress')}</option>
                    <option value="mitigated">{t('statuses.mitigated')}</option>
                    <option value="accepted">{t('statuses.accepted')}</option>
                    <option value="closed">{t('statuses.closed')}</option>
                  </select>
                </div>

                {/* Score range */}
                <div>
                  <label className="text-xs text-zinc-400 uppercase mb-2 block">{t('filters.scoreRange')}</label>
                  <input
                    type="number"
                    min={0}
                    max={100}
                    placeholder="Min"
                    value={filters.minScore || ''}
                    onChange={(e) => setFilters((prev) => ({ ...prev, minScore: e.target.value ? Number(e.target.value) : undefined }))}
                    className="w-full bg-zinc-800 border border-zinc-700 rounded px-3 py-2 text-sm text-zinc-300 placeholder-zinc-500"
                  />
                </div>

                {/* Framework */}
                <div>
                  <label className="text-xs text-zinc-400 uppercase mb-2 block">{t('filters.framework')}</label>
                  <select
                    value={filters.framework || ''}
                    onChange={(e) => setFilters((prev) => ({ ...prev, framework: e.target.value || undefined }))}
                    className="w-full bg-zinc-800 border border-zinc-700 rounded px-3 py-2 text-sm text-zinc-300"
                  >
                    <option value="">All</option>
                    <option value="iso27001">{t('frameworks.iso27001')}</option>
                    <option value="cis">{t('frameworks.cis')}</option>
                    <option value="nist">{t('frameworks.nist')}</option>
                    <option value="owasp">{t('frameworks.owasp')}</option>
                  </select>
                </div>

                {/* Clear button */}
                <div className="flex items-end">
                  <Button
                    onClick={() => setFilters({})}
                    variant="ghost"
                    className="w-full text-xs"
                  >
                    {t('filters.clearAll')}
                  </Button>
                </div>
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        {/* Content */}
        {isLoading ? (
          <SkeletonTable rows={5} columns={6} />
        ) : risks.length === 0 ? (
          <EmptyState
            icon="🎯"
            title={t('risks.noRisks')}
            description={t('risks.noRisksDescription')}
            action={{ label: t('risks.createFirstRisk'), onClick: () => setShowCreateModal(true) }}
          />
        ) : view === 'table' ? (
          // Table view
          <div className="bg-zinc-900/50 border border-zinc-800 rounded-lg overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead className="border-b border-zinc-700 bg-zinc-900/80">
                  <tr>
                    <th className="px-4 py-3 text-left w-8">
                      <input
                        type="checkbox"
                        checked={selectedRisks.size === risks.length && risks.length > 0}
                        onChange={(e) => {
                          if (e.target.checked) {
                            setSelectedRisks(new Set(risks.map((r) => r.id)));
                          } else {
                            setSelectedRisks(new Set());
                          }
                        }}
                        className="rounded"
                      />
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-medium text-zinc-400 uppercase">
                      {renderHeaderCell('title', t('risks.riskName'))}
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-medium text-zinc-400 uppercase">
                      {renderHeaderCell('score', t('risks.riskScore'))}
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-medium text-zinc-400 uppercase">
                      {renderHeaderCell('status', t('risks.riskStatus'))}
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-medium text-zinc-400 uppercase">
                      {t('risks.riskFramework')}
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-medium text-zinc-400 uppercase">
                      {t('risks.riskAssignedTo')}
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-medium text-zinc-400 uppercase">
                      {renderHeaderCell('created_at', t('common.date'))}
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-medium text-zinc-400 uppercase">
                      {t('common.actions')}
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-700/50">
                  <AnimatePresence>
                    {risks.map((risk: Risk) => (
                      <motion.tr
                        key={risk.id}
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        className="hover:bg-zinc-800/30 transition-colors"
                      >
                        <td className="px-4 py-3">
                          <input
                            type="checkbox"
                            checked={selectedRisks.has(risk.id)}
                            onChange={() => toggleRiskSelection(risk.id)}
                            className="rounded"
                          />
                        </td>
                        <td className="px-4 py-3">
                          <div>
                            <p className="font-medium text-white hover:text-primary cursor-pointer transition-colors"
                               onClick={() => { setSelectedRisk(risk); setShowRiskDrawer(true); }}>
                              {risk.title}
                            </p>
                            <p className="text-xs text-zinc-400 mt-1 truncate">
                              {risk.description?.slice(0, 80)}
                            </p>
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          <ScoreMeter score={risk.score} maxScore={100} size="sm" showLabel={false} />
                        </td>
                        <td className="px-4 py-3">
                          <StatusDot status={risk.status as any} size="sm" withLabel />
                        </td>
                        <td className="px-4 py-3 text-sm text-zinc-300">
                          {risk.frameworks?.[0] || '-'}
                        </td>
                        <td className="px-4 py-3">
                          {risk.assigned_to ? (
                            <UserAvatar name={risk.assigned_to} size="sm" />
                          ) : (
                            <span className="text-xs text-zinc-400">-</span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-sm text-zinc-400">
                          {new Date(risk.created_at || '').toLocaleDateString()}
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex gap-2">
                            <Button
                              onClick={() => { setSelectedRisk(risk); setShowRiskDrawer(true); }}
                              variant="ghost"
                              className="text-xs"
                            >
                              {t('common.edit')}
                            </Button>
                          </div>
                        </td>
                      </motion.tr>
                    ))}
                  </AnimatePresence>
                </tbody>
              </table>
            </div>
          </div>
        ) : (
          // Card view
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            <AnimatePresence>
              {risks.map((risk: Risk) => (
                <motion.div
                  key={risk.id}
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0 }}
                  whileHover={{ y: -4 }}
                  onClick={() => { setSelectedRisk(risk); setShowRiskDrawer(true); }}
                  className="bg-zinc-900/50 border border-zinc-800 rounded-lg p-6 hover:border-primary/50 transition-all cursor-pointer group"
                >
                  <div className="flex items-start justify-between mb-4">
                    <div className="flex-1">
                      <input
                        type="checkbox"
                        checked={selectedRisks.has(risk.id)}
                        onChange={(e) => {
                          e.stopPropagation();
                          toggleRiskSelection(risk.id);
                        }}
                        className="mr-2 rounded"
                      />
                      <h3 className="font-semibold text-white group-hover:text-primary transition-colors inline">
                        {risk.title}
                      </h3>
                    </div>
                    <div className="ml-4">
                      <RiskBadge level={getRiskLevel(risk.score)} size="sm" />
                    </div>
                  </div>

                  <p className="text-xs text-zinc-500 mb-4">{risk.description?.slice(0, 100)}</p>

                  <div className="space-y-3 mb-4 border-t border-zinc-700 pt-4">
                    <div className="flex items-center justify-between">
                      <span className="text-xs text-zinc-400">{t('risks.riskScore')}</span>
                      <span className="text-lg font-bold text-primary">{risk.score.toFixed(1)}</span>
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-xs text-zinc-400">{t('risks.riskStatus')}</span>
                      <StatusDot status={risk.status as any} size="sm" />
                    </div>
                  </div>

                  {risk.tags && risk.tags.length > 0 && (
                    <div className="flex flex-wrap gap-2 mb-4">
                      {risk.tags.slice(0, 2).map((tag) => (
                        <span key={tag} className="text-xs bg-primary/20 text-primary px-2 py-1 rounded">
                          {tag}
                        </span>
                      ))}
                    </div>
                  )}

                  <Button onClick={() => setSelectedRisk(risk)} variant="ghost" className="w-full text-xs">
                    {t('common.edit')}
                  </Button>
                </motion.div>
              ))}
            </AnimatePresence>
          </div>
        )}

        {/* Pagination */}
        {risks.length > 0 && (
          <div className="mt-8 flex items-center justify-between">
            <div className="text-sm text-zinc-400">
              {t('common.search')}: {total} {t('risks.title')}
            </div>
            <div className="flex items-center gap-2">
              <Button
                onClick={() => setLocalPage((p) => Math.max(1, p - 1))}
                disabled={localPage === 1}
                variant="ghost"
              >
                <ChevronLeft size={16} />
              </Button>
              <div className="px-4 text-sm text-zinc-300">
                {localPage} / {totalPages}
              </div>
              <Button
                onClick={() => setLocalPage((p) => Math.min(totalPages, p + 1))}
                disabled={localPage === totalPages}
                variant="ghost"
              >
                <ChevronRight size={16} />
              </Button>
            </div>
          </div>
        )}
      </div>

      {/* Floating bulk action bar */}
      <FloatingBulkBar
        selectedCount={selectedRisks.size}
        isVisible={hasSelectedRisks}
        onCancel={() => setSelectedRisks(new Set())}
        onConfirm={handleBulkDelete}
        isLoading={false}
        actions={[{ id: 'delete', label: t('common.delete'), icon: <X size={16} />, variant: 'danger' as const }]}
      />

      {/* Modals */}
      <CreateRiskModal isOpen={showCreateModal} onClose={() => setShowCreateModal(false)} />
      <EditRiskModal isOpen={!!editRisk} onClose={() => setEditRisk(null)} risk={editRisk} />

      {/* Drawer */}
      <Drawer isOpen={showRiskDrawer} onClose={() => setShowRiskDrawer(false)} title={selectedRisk?.title}>
        {selectedRisk && <RiskDetails risk={selectedRisk} onClose={() => setShowRiskDrawer(false)} />}
      </Drawer>
    </div>
  );
};

// Helper function
function getRiskLevel(score: number): 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' {
  if (score >= 80) return 'CRITICAL';
  if (score >= 60) return 'HIGH';
  if (score >= 40) return 'MEDIUM';
  return 'LOW';
}

export default RiskListPage;
