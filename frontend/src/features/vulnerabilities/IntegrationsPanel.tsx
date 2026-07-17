// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Integration configuration screen: wire each scanner (API credentials + inbound
// webhook + live-pull schedule + auto-risk/auto-ticket toggles) and the tenant
// ITSM (Jira/ServiceNow). Credentials are write-only — the API returns only
// has_credentials, so existing secrets show as "configured" and are preserved
// unless re-typed.

import { useMemo, useState } from 'react';
import { toast } from 'sonner';
import {
  X, Server, Cpu, Cloud, Radio, Upload, Webhook, Copy, RefreshCw, Trash2, Save,
  ChevronRight, ChevronLeft, Ticket, PlayCircle, CheckCircle2, AlertTriangle,
} from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import {
  useVulnIntegrations, useVulnIntegrationMutations, useVulnTicketing, useVulnTicketingMutations,
} from './useVulnIntegrations';
import type { VulnIntegration, VulnTicketProvider } from './vulnIntegrationsService';
import { INTEGRATION_META, TICKETING_META, type SourceMeta } from './vulnIntegrationMeta';
import type { VulnSource } from './vulnerabilityService';

type ConfigurableSource = keyof typeof INTEGRATION_META;
const SOURCES = Object.keys(INTEGRATION_META) as ConfigurableSource[];
const CAT_ICON = { network_scanner: Server, edr: Cpu, cloud: Cloud } as const;

export function IntegrationsPanel({ isOpen, onClose, onImport }: { isOpen: boolean; onClose: () => void; onImport: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const canWrite = useAuthStore((s) => s.hasPermission('vulnerabilities:update'));
  const { data: integrations } = useVulnIntegrations();
  const [view, setView] = useState<'list' | ConfigurableSource | 'ticketing'>('list');

  const bySource = useMemo(() => {
    const m: Partial<Record<VulnSource, VulnIntegration>> = {};
    (integrations ?? []).forEach((i) => { m[i.source] = i; });
    return m;
  }, [integrations]);

  if (!isOpen) return null;

  const title = view === 'list'
    ? tr('Intégrations de scanners', 'Scanner integrations')
    : view === 'ticketing'
      ? tr('Ticketing (ITSM)', 'Ticketing (ITSM)')
      : INTEGRATION_META[view].label;

  return (
    <div className="fixed inset-0 z-[80] flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,.5)', backdropFilter: 'blur(3px)' }} onClick={onClose}>
      <div onClick={(e) => e.stopPropagation()} className="w-full max-w-[640px] rounded-[16px] flex flex-col" style={{ maxHeight: '92vh', background: 'var(--bg-secondary)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)' }}>
        <div className="flex items-center gap-2 px-5 py-4" style={{ borderBottom: '1px solid var(--border)' }}>
          {view !== 'list' && (
            <button onClick={() => setView('list')} className="w-8 h-8 rounded-[9px] flex items-center justify-center text-ink-soft" style={{ background: 'var(--bg-hover)' }}><ChevronLeft size={18} /></button>
          )}
          <div className="text-[15px] font-bold text-ink flex-1">{title}</div>
          <button onClick={onClose} className="w-8 h-8 rounded-[9px] flex items-center justify-center text-ink-soft" style={{ background: 'var(--bg-hover)' }}><X size={18} /></button>
        </div>

        {view === 'list' && (
          <>
            <div className="flex-1 overflow-y-auto px-5 py-4 space-y-2.5">
              {SOURCES.map((src) => {
                const meta = INTEGRATION_META[src];
                const Icon = CAT_ICON[meta.category];
                const cfg = bySource[src];
                return (
                  <button key={src} onClick={() => setView(src)} className="w-full flex items-center gap-3.5 rounded-[12px] p-3.5 text-left transition-colors hover:bg-hover" style={{ border: '1px solid var(--border)' }}>
                    <div className="w-10 h-10 rounded-[10px] flex items-center justify-center shrink-0" style={{ background: 'var(--accent-soft)', color: 'var(--accent)' }}><Icon size={18} /></div>
                    <div className="flex-1 min-w-0">
                      <div className="text-[13.5px] font-semibold text-ink">{meta.label}</div>
                      <div className="text-[12px] text-ink-muted flex items-center gap-1.5 mt-0.5">
                        {cfg ? (
                          <>
                            <span style={{ color: cfg.enabled ? 'var(--low)' : 'var(--text-muted)' }}>{cfg.enabled ? tr('Actif', 'Enabled') : tr('Désactivé', 'Disabled')}</span>
                            {cfg.webhook_enabled && <span className="inline-flex items-center gap-1"><Webhook size={11} /> webhook</span>}
                            {cfg.live_pull_enabled && <span className="inline-flex items-center gap-1"><Radio size={11} /> live</span>}
                          </>
                        ) : (
                          <span>{tr('Non configuré', 'Not configured')}</span>
                        )}
                      </div>
                    </div>
                    <div className="flex items-center gap-1.5 shrink-0">
                      {meta.livePull && <span className="inline-flex items-center gap-1 h-[22px] px-2 rounded-full text-[10.5px] font-semibold" style={{ color: 'var(--accent)', background: 'var(--accent-soft)' }}><Radio size={11} /> Live</span>}
                      {cfg?.has_credentials && <CheckCircle2 size={16} style={{ color: 'var(--low)' }} />}
                      <ChevronRight size={16} className="text-ink-muted" />
                    </div>
                  </button>
                );
              })}
            </div>
            <div className="px-5 py-4 flex items-center gap-2" style={{ borderTop: '1px solid var(--border)' }}>
              <button onClick={() => setView('ticketing')} className="h-9 px-4 rounded-[9px] text-[13px] font-semibold inline-flex items-center gap-1.5" style={{ border: '1px solid var(--border-strong)', color: 'var(--text-secondary)' }}>
                <Ticket size={15} /> {tr('Ticketing', 'Ticketing')}
              </button>
              <button onClick={onImport} className="ml-auto h-9 px-4 rounded-[9px] text-[13px] font-semibold text-white inline-flex items-center gap-1.5" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}>
                <Upload size={15} /> {tr('Import manuel', 'Manual import')}
              </button>
            </div>
          </>
        )}

        {view !== 'list' && view !== 'ticketing' && (
          <IntegrationForm source={view} meta={INTEGRATION_META[view]} existing={bySource[view]} canWrite={canWrite} onDone={() => setView('list')} />
        )}
        {view === 'ticketing' && <TicketingForm canWrite={canWrite} />}
      </div>
    </div>
  );
}

/* ---------- toggle + field primitives ---------- */
function Toggle({ on, onChange, disabled }: { on: boolean; onChange: (v: boolean) => void; disabled?: boolean }) {
  return (
    <button type="button" disabled={disabled} onClick={() => onChange(!on)} className="relative w-[42px] h-[24px] rounded-full transition-colors disabled:opacity-50" style={{ background: on ? 'var(--accent)' : 'var(--border-strong)' }}>
      <span className="absolute top-[3px] w-[18px] h-[18px] rounded-full bg-white transition-all" style={{ left: on ? '21px' : '3px' }} />
    </button>
  );
}

function Row({ label, sub, children }: { label: string; sub?: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-3 py-2.5">
      <div className="flex-1 min-w-0">
        <div className="text-[13px] font-medium text-ink">{label}</div>
        {sub && <div className="text-[11.5px] text-ink-muted mt-0.5">{sub}</div>}
      </div>
      {children}
    </div>
  );
}

const inputCls = 'w-full h-9 px-3 rounded-[9px] text-[13px] text-ink outline-none';
const inputStyle = { border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' } as const;

/* ---------- per-source config form ---------- */
function IntegrationForm({ source, meta, existing, canWrite, onDone }: {
  source: ConfigurableSource; meta: SourceMeta; existing?: VulnIntegration; canWrite: boolean; onDone: () => void;
}) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const pick = (t: readonly [string, string]) => (lang === 'fr' ? t[0] : t[1]);
  const { save, remove, pull } = useVulnIntegrationMutations();

  const [name, setName] = useState(existing?.name ?? meta.label);
  const [baseUrl, setBaseUrl] = useState(existing?.base_url ?? '');
  const [creds, setCreds] = useState<Record<string, string>>({});
  const [enabled, setEnabled] = useState(existing?.enabled ?? true);
  const [livePull, setLivePull] = useState(existing?.live_pull_enabled ?? false);
  const [schedule, setSchedule] = useState(existing?.schedule_minutes ?? 0);
  const [webhook, setWebhook] = useState(existing?.webhook_enabled ?? false);
  const [autoRisk, setAutoRisk] = useState(existing?.auto_create_risk ?? false);
  const [autoTicket, setAutoTicket] = useState(existing?.auto_create_ticket ?? false);

  const webhookUrl = existing?.webhook_token
    ? `${window.location.origin}/api/v1/vulnerabilities/webhook/${source}?token=${existing.webhook_token}`
    : '';

  const submit = async (regenerate = false) => {
    const enteredCreds = Object.fromEntries(Object.entries(creds).filter(([, v]) => v.trim() !== ''));
    try {
      await save.mutateAsync({
        source: source as VulnSource,
        name, enabled, base_url: baseUrl,
        credentials: Object.keys(enteredCreds).length ? enteredCreds : undefined,
        live_pull_enabled: livePull, schedule_minutes: Number(schedule) || 0,
        webhook_enabled: webhook, regenerate_webhook_token: regenerate,
        auto_create_risk: autoRisk, auto_create_ticket: autoTicket,
      });
      toast.success(tr('Intégration enregistrée', 'Integration saved'));
      setCreds({});
      onDone();
    } catch {
      toast.error(tr('Échec de l’enregistrement', 'Save failed'));
    }
  };

  const runPull = async () => {
    if (!existing) return;
    try {
      const res = await pull.mutateAsync(existing.id);
      toast.success(tr(`Pull terminé — ${res.received} findings`, `Pull complete — ${res.received} findings`));
    } catch (e) {
      const msg = e instanceof Error ? e.message : tr('Échec du pull', 'Pull failed');
      toast.error(msg);
    }
  };

  const del = async () => {
    if (!existing || !window.confirm(tr('Supprimer cette intégration ?', 'Delete this integration?'))) return;
    try {
      await remove.mutateAsync(existing.id);
      toast.success(tr('Supprimée', 'Deleted'));
      onDone();
    } catch {
      toast.error(tr('Échec', 'Failed'));
    }
  };

  return (
    <>
      <div className="flex-1 overflow-y-auto px-5 py-4">
        {!meta.livePull && (
          <div className="flex items-start gap-2 rounded-[10px] p-3 mb-4 text-[12px]" style={{ background: 'color-mix(in srgb,var(--medium) 10%,transparent)', color: 'var(--text-secondary)' }}>
            <AlertTriangle size={15} style={{ color: 'var(--medium)' }} className="mt-0.5 shrink-0" />
            <span>{tr('Ce connecteur ne fait pas de live-pull REST (protocole non-REST ou SDK). Utilisez le webhook ou l’import.', 'This connector has no REST live-pull (non-REST protocol or SDK). Use the webhook or import.')}</span>
          </div>
        )}

        <label className="block text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1">{tr('Nom', 'Name')}</label>
        <input className={inputCls + ' mb-3'} style={inputStyle} value={name} onChange={(e) => setName(e.target.value)} disabled={!canWrite} />

        {meta.baseUrl && (
          <>
            <label className="block text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1">{pick(meta.baseUrl.label)}{meta.baseUrl.required && ' *'}</label>
            <input className={inputCls + ' mb-3'} style={inputStyle} placeholder={meta.baseUrl.placeholder} value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} disabled={!canWrite} />
          </>
        )}

        <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1.5 mt-1">{tr('Identifiants API', 'API credentials')}</div>
        {meta.creds.map((f) => (
          <div key={f.key} className="mb-2.5">
            <label className="block text-[12px] text-ink-soft mb-1">{pick(f.label)}</label>
            <input
              className={inputCls} style={inputStyle}
              type={f.secret ? 'password' : 'text'}
              placeholder={existing?.has_credentials ? tr('•••••• (configuré — laisser vide pour conserver)', '•••••• (configured — leave blank to keep)') : ''}
              value={creds[f.key] ?? ''}
              onChange={(e) => setCreds((c) => ({ ...c, [f.key]: e.target.value }))}
              disabled={!canWrite}
            />
          </div>
        ))}

        <div className="h-px my-3" style={{ background: 'var(--border)' }} />

        <Row label={tr('Activée', 'Enabled')}><Toggle on={enabled} onChange={setEnabled} disabled={!canWrite} /></Row>

        {meta.livePull && (
          <Row label={tr('Live-pull (polling API)', 'Live-pull (API polling)')} sub={tr('Interroge l’API du scanner périodiquement', 'Polls the scanner API periodically')}>
            <Toggle on={livePull} onChange={setLivePull} disabled={!canWrite} />
          </Row>
        )}
        {meta.livePull && livePull && (
          <Row label={tr('Fréquence (minutes)', 'Frequency (minutes)')} sub={tr('0 = manuel uniquement', '0 = manual only')}>
            <input type="number" min={0} className="w-[80px] h-9 px-2.5 rounded-[9px] text-[13px] text-ink text-right outline-none" style={inputStyle} value={schedule} onChange={(e) => setSchedule(Number(e.target.value))} disabled={!canWrite} />
          </Row>
        )}

        <Row label={tr('Webhook entrant', 'Inbound webhook')} sub={tr('Le scanner pousse ses findings vers OpenRisk', 'The scanner pushes findings to OpenRisk')}>
          <Toggle on={webhook} onChange={setWebhook} disabled={!canWrite} />
        </Row>
        {webhook && webhookUrl && (
          <div className="rounded-[10px] p-2.5 mb-2 flex items-center gap-2" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}>
            <Webhook size={14} className="text-ink-muted shrink-0" />
            <code className="flex-1 text-[11px] text-ink-soft truncate">{webhookUrl}</code>
            <button onClick={() => { navigator.clipboard.writeText(webhookUrl); toast.success(tr('Copié', 'Copied')); }} className="shrink-0 text-ink-muted hover:text-ink"><Copy size={14} /></button>
            {canWrite && <button onClick={() => submit(true)} title={tr('Régénérer', 'Regenerate')} className="shrink-0 text-ink-muted hover:text-ink"><RefreshCw size={14} /></button>}
          </div>
        )}
        {webhook && !webhookUrl && (
          <div className="text-[11.5px] text-ink-muted mb-2">{tr('Enregistrez pour générer l’URL du webhook.', 'Save to generate the webhook URL.')}</div>
        )}

        <div className="h-px my-3" style={{ background: 'var(--border)' }} />
        <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1">{tr('Automatisation', 'Automation')}</div>
        <Row label={tr('Créer un risque (P1/KEV)', 'Auto-create risk (P1/KEV)')} sub={tr('Une vuln critique sur un actif connu devient un risque', 'A critical vuln on a known asset becomes a risk')}>
          <Toggle on={autoRisk} onChange={setAutoRisk} disabled={!canWrite} />
        </Row>
        <Row label={tr('Ouvrir un ticket (P1/KEV)', 'Auto-open ticket (P1/KEV)')} sub={tr('Nécessite un ITSM configuré', 'Requires ITSM configured')}>
          <Toggle on={autoTicket} onChange={setAutoTicket} disabled={!canWrite} />
        </Row>
      </div>

      {canWrite && (
        <div className="px-5 py-3.5 flex items-center gap-2" style={{ borderTop: '1px solid var(--border)' }}>
          {existing && (
            <button onClick={del} className="h-9 px-3 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5" style={{ color: 'var(--critical)', background: 'color-mix(in srgb,var(--critical) 12%,transparent)' }}>
              <Trash2 size={14} /> {tr('Supprimer', 'Delete')}
            </button>
          )}
          {existing && meta.livePull && (
            <button onClick={runPull} disabled={pull.isPending} className="h-9 px-3 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 disabled:opacity-60" style={{ border: '1px solid var(--border-strong)', color: 'var(--accent)' }}>
              <PlayCircle size={14} /> {tr('Pull maintenant', 'Pull now')}
            </button>
          )}
          <button onClick={() => submit(false)} disabled={save.isPending} className="ml-auto h-9 px-4 rounded-[9px] text-[13px] font-semibold text-white inline-flex items-center gap-1.5 disabled:opacity-60" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}>
            <Save size={15} /> {tr('Enregistrer', 'Save')}
          </button>
        </div>
      )}
    </>
  );
}

/* ---------- ticketing (ITSM) config ---------- */
function TicketingForm({ canWrite }: { canWrite: boolean }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const pick = (t: readonly [string, string]) => (lang === 'fr' ? t[0] : t[1]);
  const { data: cfg } = useVulnTicketing();
  const { save } = useVulnTicketingMutations();

  const [provider, setProvider] = useState<VulnTicketProvider>(cfg?.provider ?? '');
  const [enabled, setEnabled] = useState(cfg?.enabled ?? false);
  const [baseUrl, setBaseUrl] = useState(cfg?.base_url ?? '');
  const [project, setProject] = useState(cfg?.project_or_table ?? '');
  const [issueType, setIssueType] = useState(cfg?.default_issue_type ?? 'Bug');
  const [creds, setCreds] = useState<Record<string, string>>({});

  const meta = provider === 'jira' ? TICKETING_META.jira : provider === 'servicenow' ? TICKETING_META.servicenow : null;

  const submit = async () => {
    const enteredCreds = Object.fromEntries(Object.entries(creds).filter(([, v]) => v.trim() !== ''));
    try {
      await save.mutateAsync({
        provider, enabled, base_url: baseUrl, project_or_table: project, default_issue_type: issueType,
        credentials: Object.keys(enteredCreds).length ? enteredCreds : undefined,
      });
      toast.success(tr('Ticketing enregistré', 'Ticketing saved'));
      setCreds({});
    } catch {
      toast.error(tr('Échec de l’enregistrement', 'Save failed'));
    }
  };

  return (
    <>
      <div className="flex-1 overflow-y-auto px-5 py-4">
        <label className="block text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1.5">{tr('Fournisseur', 'Provider')}</label>
        <div className="flex gap-2 mb-4">
          {(['', 'jira', 'servicenow'] as VulnTicketProvider[]).map((p) => (
            <button key={p || 'none'} onClick={() => canWrite && setProvider(p)} className="h-9 px-3.5 rounded-[9px] text-[12.5px] font-semibold" style={{ border: `1px solid ${provider === p ? 'var(--accent)' : 'var(--border-strong)'}`, color: provider === p ? 'var(--accent)' : 'var(--text-secondary)', background: provider === p ? 'var(--accent-soft)' : 'transparent' }}>
              {p === '' ? tr('Aucun', 'None') : p === 'jira' ? 'Jira' : 'ServiceNow'}
            </button>
          ))}
        </div>

        {meta && (
          <>
            <Row label={tr('Activé', 'Enabled')}><Toggle on={enabled} onChange={setEnabled} disabled={!canWrite} /></Row>

            <label className="block text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1 mt-2">{pick(meta.baseUrl.label)}</label>
            <input className={inputCls + ' mb-3'} style={inputStyle} placeholder={meta.baseUrl.placeholder} value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} disabled={!canWrite} />

            <label className="block text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1">{pick(meta.projectLabel)}</label>
            <input className={inputCls + ' mb-3'} style={inputStyle} placeholder={meta.projectPlaceholder} value={project} onChange={(e) => setProject(e.target.value)} disabled={!canWrite} />

            {provider === 'jira' && (
              <>
                <label className="block text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1">{tr('Type de ticket', 'Issue type')}</label>
                <input className={inputCls + ' mb-3'} style={inputStyle} placeholder="Bug" value={issueType} onChange={(e) => setIssueType(e.target.value)} disabled={!canWrite} />
              </>
            )}

            <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1.5 mt-1">{tr('Identifiants', 'Credentials')}</div>
            {meta.creds.map((f) => (
              <div key={f.key} className="mb-2.5">
                <label className="block text-[12px] text-ink-soft mb-1">{pick(f.label)}</label>
                <input className={inputCls} style={inputStyle} type={f.secret ? 'password' : 'text'}
                  placeholder={cfg?.has_credentials ? tr('•••••• (configuré)', '•••••• (configured)') : ''}
                  value={creds[f.key] ?? ''} onChange={(e) => setCreds((c) => ({ ...c, [f.key]: e.target.value }))} disabled={!canWrite} />
              </div>
            ))}
          </>
        )}
      </div>
      {canWrite && (
        <div className="px-5 py-3.5 flex justify-end" style={{ borderTop: '1px solid var(--border)' }}>
          <button onClick={submit} disabled={save.isPending} className="h-9 px-4 rounded-[9px] text-[13px] font-semibold text-white inline-flex items-center gap-1.5 disabled:opacity-60" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}>
            <Save size={15} /> {tr('Enregistrer', 'Save')}
          </button>
        </div>
      )}
    </>
  );
}
