// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Infrastructure — the live scan engine console. Provider cards (AWS/Azure/GCP/
// On-Premise), scan configurations, connected on-prem Agents, and recent scans
// whose completed previews open the ScanPreviewPage. Wired to /scanner/* — the
// pipeline never writes assets/risks; results wait in a preview to be imported.

import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Plus, Play, Trash2, Radar, ChevronRight, Server, DownloadCloud,
  ShieldOff, Boxes, Bug, Loader2,
} from 'lucide-react';
import toast from 'react-hot-toast';
import { PageFrame, PageHeader, Btn, Card, Skeleton, EmptyState, ErrorState } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { useUIStrings } from '../../shared/uiStrings';
import { useAuthStore } from '../../hooks/useAuthStore';
import { ScanConfigDrawer } from './ScanConfigDrawer';
import { AgentDeployModal } from './AgentDeployModal';
import { useScanConfigs, useScannerAgents, useScanJobs } from './useScanner';
import { PROVIDERS, jobStatusColor, agentStatusColor, timeAgo } from './scannerMeta';
import type { ScanConfig, ScannerProvider, CreateScanConfigInput } from './scannerService';

export function InfrastructurePage() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const navigate = useNavigate();
  const canWrite = useAuthStore((s) => s.hasPermission)('scanner:create');

  const { configs, isLoading, error, createConfig, deleteConfig, triggerScan } = useScanConfigs();
  const { agents, revokeAgent } = useScannerAgents();
  const { jobs } = useScanJobs();

  const doRevoke = (id: string) =>
    revokeAgent.mutate(id, {
      onSuccess: () => toast.success(tr('Agent révoqué', 'Agent revoked')),
      onError: () => toast.error(tr('Échec de la révocation', 'Revoke failed')),
    });

  const [drawerProvider, setDrawerProvider] = useState<ScannerProvider | null>(null);
  const [deployConfig, setDeployConfig] = useState<ScanConfig | null>(null);

  const onlineAgents = agents.filter((a) => a.status === 'online' || a.status === 'scanning');

  const providerCards: { provider: ScannerProvider; title: string }[] = [
    { provider: 'aws', title: 'Amazon Web Services' },
    { provider: 'azure', title: 'Microsoft Azure' },
    { provider: 'gcp', title: 'Google Cloud' },
    { provider: 'nmap', title: tr('Sur site (Agent)', 'On-Premise (Agent)') },
  ];
  const countFor = (p: ScannerProvider) =>
    p === 'nmap' ? configs.filter((c) => c.provider === 'nmap' || c.provider === 'agent').length : configs.filter((c) => c.provider === p).length;

  const handleCreate = async (input: CreateScanConfigInput) => {
    await createConfig.mutateAsync(input);
    toast.success(tr('Configuration créée', 'Configuration created'));
    setDrawerProvider(null);
  };

  const handleScan = (cfg: ScanConfig) => {
    const onPrem = cfg.provider === 'nmap' || cfg.provider === 'agent';
    if (onPrem && onlineAgents.length === 0) {
      toast(tr('Aucun agent en ligne — le scan sera mis en file et lancé dès qu’un agent se connecte.', 'No agent online — the scan is queued and runs once an agent connects.'), { icon: '⏳' });
    }
    triggerScan.mutate(cfg.id, {
      onSuccess: () => toast.success(tr('Scan lancé', 'Scan started')),
      onError: (e) => toast.error((e as { response?: { data?: { error?: string } } })?.response?.data?.error ?? tr('Échec du scan', 'Scan failed')),
    });
  };

  return (
    <PageFrame wide>
      <PageHeader
        title={L.n_infra}
        count={configs.length ? `${configs.length}` : null}
        actions={
          <>
            <Btn label={tr('Vue Univers', 'Universe view')} icon={Radar} onClick={() => navigate('/assets/universe')} />
            {canWrite && <Btn label={tr('Nouvelle config', 'New config')} icon={Plus} primary onClick={() => setDrawerProvider('aws')} />}
          </>
        }
      />

      {/* Provider cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3.5 mb-6">
        {providerCards.map(({ provider, title }) => {
          const m = PROVIDERS[provider];
          const n = countFor(provider);
          return (
            <button
              key={provider}
              onClick={() => canWrite && setDrawerProvider(provider)}
              className="text-left rounded-2xl p-4 transition-all hover:brightness-[1.06] disabled:opacity-70"
              disabled={!canWrite}
              style={{ border: '1px solid var(--border)', background: 'var(--bg-panel)', cursor: canWrite ? 'pointer' : 'default' }}
            >
              <div className="flex items-center justify-between mb-3">
                <div className="w-10 h-10 rounded-xl flex items-center justify-center" style={{ background: `color-mix(in srgb,${m.color} 15%,transparent)`, color: m.color }}>
                  <m.icon size={20} strokeWidth={1.8} />
                </div>
                {provider === 'nmap' && (
                  <span className="text-[11px] font-semibold inline-flex items-center gap-1.5" style={{ color: onlineAgents.length ? 'var(--low)' : 'var(--text-muted)' }}>
                    <span className="w-1.5 h-1.5 rounded-full" style={{ background: onlineAgents.length ? 'var(--low)' : 'var(--text-muted)' }} />
                    {onlineAgents.length} {tr('en ligne', 'online')}
                  </span>
                )}
              </div>
              <div className="text-[13.5px] font-semibold text-ink leading-tight">{title}</div>
              <div className="text-[12px] text-ink-soft mt-1">{n} {n === 1 ? tr('configuration', 'config') : tr('configurations', 'configs')}{canWrite && <span className="ml-1" style={{ color: m.color }}>· {tr('ajouter', 'add')}</span>}</div>
            </button>
          );
        })}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
        {/* Left: configs + recent scans */}
        <div className="lg:col-span-2 flex flex-col gap-5">
          <Card style={{ padding: 0 }}>
            <SectionHead icon={Boxes} title={tr('Configurations de scan', 'Scan configurations')} />
            {isLoading ? (
              <div className="p-4 flex flex-col gap-2">{[0, 1, 2].map((i) => <Skeleton key={i} style={{ height: 56 }} />)}</div>
            ) : error ? (
              <ErrorState title={tr('Chargement impossible', 'Could not load')} />
            ) : configs.length === 0 ? (
              <EmptyState icon={Boxes} title={tr('Aucune configuration', 'No configurations yet')} sub={tr('Ajoutez un cloud ou un scan sur site pour commencer.', 'Add a cloud or on-prem scan to get started.')} cta={canWrite ? <Btn label={tr('Nouvelle config', 'New config')} icon={Plus} primary onClick={() => setDrawerProvider('aws')} /> : undefined} />
            ) : (
              <div>
                {configs.map((c, i) => {
                  const m = PROVIDERS[c.provider];
                  const onPrem = c.provider === 'nmap' || c.provider === 'agent';
                  const scanning = triggerScan.isPending && triggerScan.variables === c.id;
                  return (
                    <div key={c.id} className="flex items-center gap-3 px-4 py-3.5" style={{ borderTop: i ? '1px solid var(--border)' : 'none' }}>
                      <div className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0" style={{ background: `color-mix(in srgb,${m.color} 14%,transparent)`, color: m.color }}>
                        <m.icon size={16} strokeWidth={1.8} />
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="text-[13.5px] font-semibold text-ink truncate">{c.name}</div>
                        <div className="text-[11.5px] text-ink-soft truncate">
                          {onPrem ? (c.targets?.join(', ') || tr('aucune cible', 'no targets')) : (c.regions?.length ? c.regions.join(', ') : tr('toutes régions', 'all regions'))}
                        </div>
                      </div>
                      <span className="text-[11px] font-semibold px-2 py-[3px] rounded-md shrink-0" style={{ color: m.color, background: `color-mix(in srgb,${m.color} 13%,transparent)` }}>{m.short}</span>
                      {canWrite && (
                        <div className="flex items-center gap-1.5 shrink-0">
                          {onPrem && (
                            <button onClick={() => setDeployConfig(c)} title={tr('Déployer un agent', 'Deploy agent')} className="w-8 h-8 rounded-lg flex items-center justify-center text-ink-soft hover:bg-hover"><DownloadCloud size={16} /></button>
                          )}
                          <button onClick={() => handleScan(c)} disabled={scanning} title={tr('Scanner maintenant', 'Scan now')} className="h-8 px-3 rounded-lg inline-flex items-center gap-1.5 text-[12px] font-semibold" style={{ background: 'var(--accent-soft)', color: 'var(--accent)' }}>
                            {scanning ? <Loader2 size={14} className="animate-spin" /> : <Play size={14} />}{tr('Scanner', 'Scan')}
                          </button>
                          <button onClick={() => { if (confirm(tr('Supprimer cette configuration ?', 'Delete this configuration?'))) deleteConfig.mutate(c.id, { onSuccess: () => toast.success(tr('Supprimée', 'Deleted')) }); }} title={tr('Supprimer', 'Delete')} className="w-8 h-8 rounded-lg flex items-center justify-center text-ink-soft hover:bg-hover"><Trash2 size={15} /></button>
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </Card>

          <Card style={{ padding: 0 }}>
            <SectionHead icon={Bug} title={tr('Scans récents', 'Recent scans')} />
            {jobs.length === 0 ? (
              <EmptyState icon={Bug} title={tr('Aucun scan pour l’instant', 'No scans yet')} sub={tr('Lancez un scan depuis une configuration.', 'Run a scan from a configuration.')} />
            ) : (
              <div>
                {jobs.slice(0, 8).map((j, i) => {
                  const m = PROVIDERS[j.provider];
                  const done = j.status === 'completed';
                  const active = j.status === 'running' || j.status === 'queued' || j.status === 'claimed';
                  return (
                    <button
                      key={j.id}
                      onClick={() => done && navigate(`/infrastructure/scans/${j.id}`)}
                      className="w-full text-left flex items-center gap-3 px-4 py-3.5 transition-colors hover:bg-hover"
                      style={{ borderTop: i ? '1px solid var(--border)' : 'none', cursor: done ? 'pointer' : 'default' }}
                    >
                      <span className="w-2 h-2 rounded-full shrink-0" style={{ background: jobStatusColor(j.status), animation: active ? 'or-pulsedot 1.4s infinite' : 'none' }} />
                      <span className="text-[11px] font-semibold px-2 py-[3px] rounded-md shrink-0" style={{ color: m.color, background: `color-mix(in srgb,${m.color} 13%,transparent)` }}>{m.short}</span>
                      <div className="min-w-0 flex-1">
                        <div className="text-[13px] font-medium text-ink capitalize">{j.status}{j.error ? ` · ${j.error.slice(0, 48)}` : ''}</div>
                        <div className="text-[11.5px] text-ink-soft">{timeAgo(j.created_at, lang)}</div>
                      </div>
                      <span className="mono text-[12px] text-ink-soft shrink-0">{j.assets_found} {tr('actifs', 'assets')} · {j.findings_found} {tr('vulns', 'findings')}</span>
                      {done && <ChevronRight size={16} className="text-ink-soft shrink-0" />}
                    </button>
                  );
                })}
              </div>
            )}
          </Card>
        </div>

        {/* Right: agents */}
        <div className="flex flex-col gap-5">
          <Card style={{ padding: 0 }}>
            <SectionHead icon={Server} title={tr('Agents connectés', 'Connected agents')} right={<span className="text-[12px] font-semibold" style={{ color: onlineAgents.length ? 'var(--low)' : 'var(--text-muted)' }}>{onlineAgents.length}/{agents.length}</span>} />
            {agents.length === 0 ? (
              <div className="px-4 pb-5 pt-1">
                <EmptyState icon={ShieldOff} title={tr('Aucun agent', 'No agents')} sub={tr('Déployez un agent depuis une config sur site.', 'Deploy one from an on-prem config.')} />
              </div>
            ) : (
              <div>
                {agents.map((a, i) => (
                  <div key={a.id} className="flex items-center gap-3 px-4 py-3.5" style={{ borderTop: i ? '1px solid var(--border)' : 'none' }}>
                    <span className="w-2 h-2 rounded-full shrink-0" style={{ background: agentStatusColor(a.status), animation: a.status === 'scanning' ? 'or-pulsedot 1.4s infinite' : 'none' }} />
                    <div className="min-w-0 flex-1">
                      <div className="text-[13px] font-semibold text-ink truncate">{a.name || a.hostname}</div>
                      <div className="text-[11.5px] text-ink-soft truncate">{a.os || '—'} · v{a.version || '?'} · {timeAgo(a.last_heartbeat, lang)}</div>
                    </div>
                    <span className="text-[11px] font-semibold capitalize shrink-0" style={{ color: agentStatusColor(a.status) }}>{a.status}</span>
                    {canWrite && a.status !== 'revoked' && (
                      <button onClick={() => { if (confirm(tr('Révoquer cet agent ? Son jeton sera invalidé immédiatement.', 'Revoke this agent? Its token is invalidated immediately.'))) doRevoke(a.id); }} title={tr('Révoquer', 'Revoke')} className="w-7 h-7 rounded-lg flex items-center justify-center text-ink-soft hover:bg-hover"><ShieldOff size={14} /></button>
                    )}
                  </div>
                ))}
              </div>
            )}
          </Card>
        </div>
      </div>

      {drawerProvider && (
        <ScanConfigDrawer
          open
          provider={drawerProvider}
          submitting={createConfig.isPending}
          onClose={() => setDrawerProvider(null)}
          onCreate={handleCreate}
        />
      )}
      {deployConfig && <AgentDeployModal config={deployConfig} onClose={() => setDeployConfig(null)} />}
    </PageFrame>
  );
}

function SectionHead({ icon: Icon, title, right }: { icon: typeof Server; title: string; right?: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between px-4 py-3.5" style={{ borderBottom: '1px solid var(--border)' }}>
      <div className="flex items-center gap-2.5">
        <Icon size={16} strokeWidth={1.9} className="text-ink-soft" />
        <span className="text-[13.5px] font-semibold text-ink">{title}</span>
      </div>
      {right}
    </div>
  );
}
