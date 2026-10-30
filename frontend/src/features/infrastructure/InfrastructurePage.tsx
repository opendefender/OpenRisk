// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
//
// Infrastructure (#912, the redesign blended with the scanner console). On top,
// "Lancer un scan" picks a configuration and starts it; below, the scans with
// their progress and counts next to the agents with their last heartbeat; then
// what the old screen already did and the design leaves out: the provider
// cards, the scan configurations with their schedule, agent deployment, and
// the preview a completed scan opens. Everything is wired to /scanner/*; the
// pipeline never writes assets or risks, results wait in a preview.

import { useMemo, useState, type ReactNode } from 'react';
import { useNavigate } from 'react-router';
import { toast } from 'sonner';
import { ChevronRight, Clock, DownloadCloud, Play, Plus, Radar, ShieldOff } from 'lucide-react';

import { useI18n } from '../../hooks/useI18n';
import { useAuthStore } from '../../hooks/useAuthStore';
import { AlertDialog, Button, DeleteButton, Listbox } from '../../shared/ds';
import { PageFrame, PageHeader } from '../../shared/ui';
import { BlockEmpty, BlockError, BlockSkeleton } from '../dashboard/console/Panel';
import { ScanConfigDrawer } from './ScanConfigDrawer';
import { AgentDeployModal } from './AgentDeployModal';
import { useScanConfigs, useScannerAgents, useScanJobs } from './useScanner';
import {
  PROVIDERS,
  agentStatusColor,
  jobProgress,
  jobStatusColor,
  scanFamily,
  scheduleLabel,
  type ScanFamily,
} from './scannerMeta';
import type {
  CreateScanConfigInput,
  ScanConfig,
  ScanJob,
  ScannerAgent,
  ScannerProvider,
} from './scannerService';

// An empty title is the on-prem card, whose name is translated.
const PROVIDER_CARDS: { provider: ScannerProvider; title: string }[] = [
  { provider: 'aws', title: 'Amazon Web Services' },
  { provider: 'azure', title: 'Microsoft Azure' },
  { provider: 'gcp', title: 'Google Cloud' },
  { provider: 'kubernetes', title: 'Kubernetes' },
  { provider: 'docker', title: 'Docker' },
  { provider: 'vmware', title: 'VMware vCenter' },
  { provider: 'active_directory', title: 'Active Directory' },
  { provider: 'm365', title: 'Microsoft 365' },
  { provider: 'github', title: 'GitHub' },
  { provider: 'gitlab', title: 'GitLab' },
  { provider: 'nmap', title: '' },
];

const FAMILIES: readonly (ScanFamily | 'all')[] = ['all', 'network', 'cloud', 'discovery'];

const isOnPrem = (p: ScannerProvider) => p === 'nmap' || p === 'agent';
const isOnline = (a: ScannerAgent) => a.status === 'online' || a.status === 'scanning';

function apiError(e: unknown): string | undefined {
  return (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
}

function targetSummary(c: ScanConfig, t: (key: string) => string): string {
  if (isOnPrem(c.provider)) return c.targets?.join(', ') || t('infra.configs.noTargets');
  return c.regions?.length ? c.regions.join(', ') : t('infra.configs.allRegions');
}

function Section({
  title,
  aside,
  children,
  className = '',
}: {
  title: string;
  aside?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section
      aria-label={title}
      className={`rounded-[14px] border border-border-subtle bg-surface-1 overflow-hidden ${className}`}
    >
      <div className="flex items-center justify-between gap-3 px-[18px] pt-4 pb-2.5">
        <h2 className="m-0 text-[13.5px] font-semibold text-ink">{title}</h2>
        {aside}
      </div>
      {children}
    </section>
  );
}

/** Skeleton, error with retry, or empty sentence for a list block. */
function BlockState({
  loading,
  error,
  empty,
  onRetry,
  children,
}: {
  loading: boolean;
  error: unknown;
  empty: string | null;
  onRetry: () => void;
  children: ReactNode;
}) {
  if (loading)
    return (
      <div className="px-[18px] pb-4">
        <BlockSkeleton lines={4} height={18} />
      </div>
    );
  if (error)
    return (
      <div className="px-[18px]">
        <BlockError onRetry={onRetry} />
      </div>
    );
  if (empty !== null)
    return (
      <div className="px-[18px] pb-2">
        <BlockEmpty>{empty}</BlockEmpty>
      </div>
    );
  return <>{children}</>;
}

export function InfrastructurePage() {
  const { t, locale, fmt } = useI18n();
  const navigate = useNavigate();
  const canWrite = useAuthStore((s) => s.hasPermission)('scanner:create');

  const configsQ = useScanConfigs();
  const agentsQ = useScannerAgents();
  const jobsQ = useScanJobs();
  const { configs, createConfig, deleteConfig, triggerScan } = configsQ;
  const { agents, revokeAgent } = agentsQ;

  const [drawerProvider, setDrawerProvider] = useState<ScannerProvider | null>(null);
  const [deployConfig, setDeployConfig] = useState<ScanConfig | null>(null);
  const [toDelete, setToDelete] = useState<ScanConfig | null>(null);
  const [toRevoke, setToRevoke] = useState<ScannerAgent | null>(null);
  const [family, setFamily] = useState<ScanFamily | 'all'>('all');
  const [targetId, setTargetId] = useState('');

  const onlineAgents = agents.filter(isOnline);
  const configById = useMemo(() => new Map(configs.map((c) => [c.id, c])), [configs]);
  const targets = configs.filter((c) => family === 'all' || scanFamily(c.provider) === family);
  // The picked target falls back to the first one the type allows.
  const target = targets.find((c) => c.id === targetId) ?? targets[0];

  const countFor = (p: ScannerProvider) =>
    configs.filter((c) => (p === 'nmap' ? isOnPrem(c.provider) : c.provider === p)).length;

  const handleCreate = async (input: CreateScanConfigInput) => {
    await createConfig.mutateAsync(input);
    toast.success(t('infra.toast.created'));
    setDrawerProvider(null);
  };

  const startScan = (cfg: ScanConfig) => {
    if (isOnPrem(cfg.provider) && onlineAgents.length === 0) toast(t('infra.toast.queued'));
    triggerScan.mutate(cfg.id, {
      onSuccess: () => toast.success(t('infra.toast.started', { name: cfg.name })),
      onError: (e) => toast.error(apiError(e) ?? t('infra.toast.failed')),
    });
  };

  const confirmDelete = () => {
    if (!toDelete) return;
    deleteConfig.mutate(toDelete.id, {
      onSuccess: () => toast.success(t('infra.toast.deleted')),
      onError: (e) => toast.error(apiError(e) ?? t('infra.toast.deleteFailed')),
    });
    setToDelete(null);
  };

  const confirmRevoke = () => {
    if (!toRevoke) return;
    revokeAgent.mutate(toRevoke.id, {
      onSuccess: () => toast.success(t('infra.toast.revoked')),
      onError: () => toast.error(t('infra.toast.revokeFailed')),
    });
    setToRevoke(null);
  };

  return (
    <PageFrame wide>
      <PageHeader
        className="!mb-[18px]"
        title={t('infra.title')}
        subtitle={t('infra.subtitle')}
        actions={
          <div className="flex items-center gap-2">
            <Button variant="secondary" icon={Radar} onClick={() => navigate('/assets/topology')}>
              {t('infra.topology')}
            </Button>
            {canWrite && (
              <Button variant="primary" icon={Plus} onClick={() => setDrawerProvider('aws')}>
                {t('infra.newConfig')}
              </Button>
            )}
          </div>
        }
      />

      {canWrite && (
        <section
          aria-label={t('infra.launch.title')}
          className="rounded-[14px] border border-border-subtle bg-surface-1 px-[18px] py-4 mb-4"
        >
          <h2 className="m-0 mb-3 text-[13.5px] font-semibold text-ink">
            {t('infra.launch.title')}
          </h2>
          {configsQ.isLoading ? (
            <BlockSkeleton lines={1} height={36} />
          ) : configsQ.error ? (
            <BlockError onRetry={() => void configsQ.refetch()} />
          ) : configs.length === 0 ? (
            <BlockEmpty>{t('infra.launch.empty')}</BlockEmpty>
          ) : (
            <div className="flex gap-2 flex-wrap items-end">
              <div className="grid gap-1.5 flex-[1_1_260px] min-w-0">
                <span className="text-[12px] font-semibold text-ink-soft">
                  {t('infra.launch.target')}
                </span>
                <Listbox
                  aria-label={t('infra.launch.target')}
                  className="h-9 rounded-[10px] text-[13px]"
                  value={target?.id ?? ''}
                  onChange={setTargetId}
                  placeholder={t('infra.launch.noTarget')}
                  emptyLabel={t('infra.launch.noTarget')}
                  disabled={targets.length === 0}
                  options={targets.map((c) => ({
                    value: c.id,
                    label: c.name,
                    description: targetSummary(c, t),
                  }))}
                />
              </div>
              <div className="grid gap-1.5 flex-[0_1_220px] min-w-0">
                <span className="text-[12px] font-semibold text-ink-soft">
                  {t('infra.launch.type')}
                </span>
                <Listbox<ScanFamily | 'all'>
                  aria-label={t('infra.launch.type')}
                  className="h-9 rounded-[10px] text-[13px]"
                  value={family}
                  onChange={setFamily}
                  options={FAMILIES.map((f) => ({
                    value: f,
                    label: t(`infra.launch.family.${f}`),
                  }))}
                />
              </div>
              <Button
                variant="primary"
                icon={Play}
                disabled={!target}
                loading={triggerScan.isPending}
                onClick={() => target && startScan(target)}
              >
                {t('infra.launch.start')}
              </Button>
            </div>
          )}
        </section>
      )}

      <div className="flex gap-4 flex-wrap items-start mb-4">
        <Section title={t('infra.scans.title')} className="flex-[1.3_1_520px] min-w-0">
          <BlockState
            loading={jobsQ.isLoading}
            error={jobsQ.error}
            empty={jobsQ.jobs.length === 0 ? t('infra.scans.empty') : null}
            onRetry={() => void jobsQ.refetch()}
          >
            <ul className="m-0 p-0 list-none">
              {jobsQ.jobs.slice(0, 8).map((j) => (
                <ScanRow
                  key={j.id}
                  job={j}
                  configName={configById.get(j.config_id)?.name}
                  onOpen={() => navigate(`/infrastructure/scans/${j.id}`)}
                />
              ))}
            </ul>
          </BlockState>
        </Section>

        <Section
          title={t('infra.agents.title')}
          className="flex-[1_1_380px] min-w-0"
          aside={
            agents.length > 0 ? (
              <span
                className="text-[12px] font-semibold"
                style={{ color: onlineAgents.length ? 'var(--success)' : 'var(--fg-muted)' }}
              >
                {onlineAgents.length}/{agents.length}
              </span>
            ) : undefined
          }
        >
          <BlockState
            loading={agentsQ.isLoading}
            error={agentsQ.error}
            empty={agents.length === 0 ? t('infra.agents.empty') : null}
            onRetry={() => void agentsQ.refetch()}
          >
            <ul className="m-0 p-0 list-none">
              {agents.map((a) => {
                const host = a.hostname || a.name;
                return (
                  <li
                    key={a.id}
                    className="flex items-center gap-3 px-[18px] py-2.5 border-t border-border-subtle text-[12.5px]"
                  >
                    <span
                      aria-hidden="true"
                      className="w-2 h-2 rounded-full shrink-0"
                      style={{
                        background: agentStatusColor(a.status),
                        animation: a.status === 'scanning' ? 'or-pulsedot 1.4s infinite' : 'none',
                      }}
                    />
                    <span className="flex-1 min-w-0">
                      <span className="block mono text-[12.5px] text-ink truncate">{host}</span>
                      <span className="block text-[11.5px] text-ink-muted truncate">
                        {a.os || '—'} · {t('infra.agents.version', { version: a.version || '?' })}
                        {' · '}
                        {t(`infra.agents.status.${a.status}`)}
                      </span>
                    </span>
                    <span
                      className="text-ink-soft shrink-0"
                      title={a.last_heartbeat ? fmt.dateTime(a.last_heartbeat) : undefined}
                    >
                      {a.last_heartbeat ? fmt.relative(a.last_heartbeat) : '—'}
                    </span>
                    {canWrite && a.status !== 'revoked' && (
                      <Button
                        variant="ghost"
                        size="sm"
                        icon={ShieldOff}
                        aria-label={t('infra.agents.revoke', { name: host })}
                        title={t('infra.agents.revoke', { name: host })}
                        onClick={() => setToRevoke(a)}
                      />
                    )}
                  </li>
                );
              })}
            </ul>
          </BlockState>
        </Section>
      </div>

      <Section title={t('infra.providers.title')} className="mb-4">
        <div className="grid grid-cols-2 lg:grid-cols-4 xl:grid-cols-6 gap-2.5 px-[18px] pb-4">
          {PROVIDER_CARDS.map(({ provider, title }) => {
            const m = PROVIDERS[provider];
            const n = countFor(provider);
            return (
              <button
                key={provider}
                type="button"
                onClick={() => setDrawerProvider(provider)}
                disabled={!canWrite}
                className="text-left rounded-[12px] p-3 border border-border-subtle bg-surface-2 transition-[filter] enabled:hover:brightness-[1.06] disabled:cursor-default"
              >
                <span className="flex items-center justify-between mb-2">
                  <span
                    aria-hidden="true"
                    className="w-8 h-8 rounded-lg flex items-center justify-center"
                    style={{
                      background: `color-mix(in srgb,${m.color} 15%,transparent)`,
                      color: m.color,
                    }}
                  >
                    <m.icon size={16} strokeWidth={1.8} />
                  </span>
                  {provider === 'nmap' && (
                    <span
                      className="text-[11px] font-semibold"
                      style={{
                        color: onlineAgents.length ? 'var(--success)' : 'var(--fg-muted)',
                      }}
                    >
                      {t('infra.providers.online', { count: onlineAgents.length })}
                    </span>
                  )}
                </span>
                <span className="block text-[13px] font-semibold text-ink leading-tight">
                  {title || t('infra.providers.onPrem')}
                </span>
                <span className="block text-[11.5px] text-ink-muted mt-0.5">
                  {configsQ.isLoading ? '…' : t('infra.providers.configs', { count: n })}
                  {canWrite && (
                    <span className="ml-1" style={{ color: m.color }}>
                      · {t('infra.providers.add')}
                    </span>
                  )}
                </span>
              </button>
            );
          })}
        </div>
      </Section>

      <Section title={t('infra.configs.title')}>
        {configsQ.isLoading ? (
          <div className="px-[18px] pb-4">
            <BlockSkeleton lines={3} height={22} />
          </div>
        ) : configsQ.error ? (
          <div className="px-[18px]">
            <BlockError onRetry={() => void configsQ.refetch()} />
          </div>
        ) : configs.length === 0 ? (
          <div className="px-[18px] pb-4 grid gap-2 justify-items-start">
            <BlockEmpty>
              <strong className="font-semibold text-ink">{t('infra.configs.emptyTitle')}</strong>{' '}
              {t('infra.configs.emptyBody')}
            </BlockEmpty>
            {canWrite && (
              <Button
                variant="primary"
                size="sm"
                icon={Plus}
                onClick={() => setDrawerProvider('aws')}
              >
                {t('infra.newConfig')}
              </Button>
            )}
          </div>
        ) : (
          <ul className="m-0 p-0 list-none">
            {configs.map((c) => {
              const m = PROVIDERS[c.provider];
              const scanning = triggerScan.isPending && triggerScan.variables === c.id;
              return (
                <li
                  key={c.id}
                  className="flex items-center gap-3 px-[18px] py-3 border-t border-border-subtle"
                >
                  <span
                    aria-hidden="true"
                    className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0"
                    style={{
                      background: `color-mix(in srgb,${m.color} 14%,transparent)`,
                      color: m.color,
                    }}
                  >
                    <m.icon size={16} strokeWidth={1.8} />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block text-[13px] font-semibold text-ink truncate">
                      {c.name}
                    </span>
                    <span className="block text-[11.5px] text-ink-muted truncate">
                      {targetSummary(c, t)}
                    </span>
                  </span>
                  {c.schedule_minutes > 0 && (
                    <span
                      title={
                        c.next_run_at
                          ? t('infra.configs.next', { date: fmt.dateTime(c.next_run_at) })
                          : undefined
                      }
                      className="text-[11px] font-semibold px-2 py-[3px] rounded-md shrink-0 inline-flex items-center gap-1"
                      style={{
                        color: 'var(--info)',
                        background: 'color-mix(in srgb,var(--info) 12%,transparent)',
                      }}
                    >
                      <Clock size={11} aria-hidden="true" />
                      {scheduleLabel(c.schedule_minutes, locale)}
                    </span>
                  )}
                  <span
                    className="text-[11px] font-semibold px-2 py-[3px] rounded-md shrink-0"
                    style={{
                      color: m.color,
                      background: `color-mix(in srgb,${m.color} 13%,transparent)`,
                    }}
                  >
                    {m.short}
                  </span>
                  {canWrite && (
                    <span className="flex items-center gap-1.5 shrink-0">
                      {isOnPrem(c.provider) && (
                        <Button
                          variant="ghost"
                          size="sm"
                          icon={DownloadCloud}
                          aria-label={t('infra.configs.deploy')}
                          title={t('infra.configs.deploy')}
                          onClick={() => setDeployConfig(c)}
                        />
                      )}
                      <Button
                        variant="secondary"
                        size="sm"
                        icon={Play}
                        loading={scanning}
                        aria-label={t('infra.configs.scanNow', { name: c.name })}
                        onClick={() => startScan(c)}
                      >
                        {t('infra.configs.scan')}
                      </Button>
                      <DeleteButton
                        size="sm"
                        aria-label={t('infra.configs.delete', { name: c.name })}
                        onClick={() => setToDelete(c)}
                      />
                    </span>
                  )}
                </li>
              );
            })}
          </ul>
        )}
      </Section>

      <AlertDialog
        open={toDelete !== null}
        tone="destructive"
        title={t('infra.configs.deleteTitle')}
        description={toDelete ? t('infra.configs.deleteBody', { name: toDelete.name }) : undefined}
        confirmLabel={t('infra.configs.deleteConfirm')}
        cancelLabel={t('common.cancel')}
        onCancel={() => setToDelete(null)}
        onConfirm={confirmDelete}
      />
      <AlertDialog
        open={toRevoke !== null}
        tone="destructive"
        title={t('infra.agents.revokeTitle')}
        description={t('infra.agents.revokeBody')}
        confirmLabel={t('infra.agents.revokeConfirm')}
        cancelLabel={t('common.cancel')}
        onCancel={() => setToRevoke(null)}
        onConfirm={confirmRevoke}
      />

      {drawerProvider && (
        <ScanConfigDrawer
          open
          provider={drawerProvider}
          submitting={createConfig.isPending}
          onClose={() => setDrawerProvider(null)}
          onCreate={handleCreate}
        />
      )}
      {deployConfig && (
        <AgentDeployModal config={deployConfig} onClose={() => setDeployConfig(null)} />
      )}
    </PageFrame>
  );
}

function ScanRow({
  job,
  configName,
  onOpen,
}: {
  job: ScanJob;
  configName: string | undefined;
  onOpen: () => void;
}) {
  const { t, fmt } = useI18n();
  const m = PROVIDERS[job.provider];
  const pct = jobProgress(job.status);
  const done = job.status === 'completed';
  const color = jobStatusColor(job.status);
  const label = configName ?? (job.targets?.join(', ') || m.short);

  const body = (
    <>
      <span className="min-w-0">
        <span className="block text-[13px] font-medium text-ink truncate">{label}</span>
        <span className="block text-[11.5px] text-ink-muted truncate">
          <span className="mono">{job.id.slice(0, 8)}</span> · {m.short} ·{' '}
          {fmt.relative(job.started_at ?? job.created_at)} ·{' '}
          {t('infra.scans.assets', { count: job.assets_found })}
        </span>
        {job.error && (
          <span className="block text-[11.5px] truncate" style={{ color: 'var(--danger-text)' }}>
            {job.error}
          </span>
        )}
      </span>
      <span className="flex items-center gap-2">
        <span
          role="progressbar"
          aria-label={t('infra.scans.progress')}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={pct ?? undefined}
          className="relative flex-1 h-[5px] rounded-[5px] overflow-hidden"
          style={{ background: 'var(--chart-track)' }}
        >
          {pct === null ? (
            <span
              className="absolute inset-0 or-skeleton"
              style={{ background: `color-mix(in srgb,${color} 45%,transparent)` }}
            />
          ) : (
            <span
              className="block h-full transition-[width] duration-[400ms]"
              style={{ width: `${pct}%`, background: color }}
            />
          )}
        </span>
        <span className="mono text-[11.5px] w-[34px] text-right text-ink-soft">
          {pct === null ? '' : `${pct}%`}
        </span>
      </span>
      <span
        className="flex items-center justify-end gap-1 text-[12px] font-semibold text-ink"
        // A finished scan reads as its findings count, in ink, as in the design.
        style={done ? undefined : { color }}
      >
        {done
          ? t('infra.scans.findings', { count: job.findings_found })
          : t(`infra.status.${job.status}`)}
        {done && <ChevronRight size={14} aria-hidden="true" className="text-ink-muted" />}
      </span>
    </>
  );

  const grid =
    'grid grid-cols-[minmax(0,1fr)_120px_100px] sm:grid-cols-[minmax(0,1fr)_160px_110px] gap-3.5 items-center px-[18px] py-3 w-full text-left';

  return (
    <li className="border-t border-border-subtle">
      {done ? (
        <button
          type="button"
          onClick={onOpen}
          aria-label={`${label} · ${t('infra.scans.openPreview')}`}
          className={`${grid} hover:bg-surface-2 transition-colors`}
        >
          {body}
        </button>
      ) : (
        <div className={grid}>{body}</div>
      )}
    </li>
  );
}
