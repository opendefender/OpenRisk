// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
//
// Security Automation / SOAR (spec §10 « Automatisation »). Four views:
//  - Rules: the workflow builder — each rule shows its trigger → action chain.
//  - SLA: the live remediation countdown dashboard (open/breached/escalated).
//  - History: the execution audit trail with per-step outcomes.
//  - Channels: the tenant Slack/Teams/email alert configuration.

import { useState } from 'react';
import { toast } from 'sonner';
import {
  Workflow, Plus, Play, Pencil, Trash2, Power, Timer, AlertTriangle, Siren,
  Activity, ChevronRight, ArrowRight, Bell, MessageSquare, Mail,
} from 'lucide-react';
import { PageFrame, PageHeader, Btn, Card, SkeletonRows, EmptyState } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import {
  useAutomationRules, useAutomationExecutions, useSLATrackers, useSLAStats,
  useChannelConfig, useAutomationMutations,
} from './useAutomation';
import type { AutomationRule } from './automationService';
import {
  TRIGGER_META, ACTION_META, EXEC_STATUS_META, SLA_STATUS_META, SEVERITY_COLOR,
  pick, fmtMinutes,
} from './automationMeta';
import { RuleEditorModal } from './RuleEditorModal';

type Tab = 'rules' | 'sla' | 'history' | 'channels';

export function AutomationPage() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const canWrite = useAuthStore((s) => s.hasPermission)('automation:write');

  // Deep-linkable tab: /automation?tab=sla focuses the SLA dashboard.
  const initialTab = (new URLSearchParams(window.location.search).get('tab') as Tab) || 'rules';
  const [tab, setTab] = useState<Tab>(
    ['rules', 'sla', 'history', 'channels'].includes(initialTab) ? initialTab : 'rules',
  );
  const [editorRule, setEditorRule] = useState<AutomationRule | null>(null);
  const [editorOpen, setEditorOpen] = useState(false);

  const { data: rules = [], isLoading: rulesLoading } = useAutomationRules();
  const { data: stats } = useSLAStats();

  const openNew = () => { setEditorRule(null); setEditorOpen(true); };
  const openEdit = (r: AutomationRule) => { setEditorRule(r); setEditorOpen(true); };

  const TabBtn = ({ id, label, count }: { id: Tab; label: string; count?: number }) => (
    <button onClick={() => setTab(id)} className="h-9 px-3.5 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5"
      style={{ background: tab === id ? 'var(--accent)' : 'transparent', color: tab === id ? '#fff' : 'var(--text-secondary)', border: tab === id ? 'none' : '1px solid var(--border-strong)' }}>
      {label}{typeof count === 'number' && <span className="mono opacity-80">{count}</span>}
    </button>
  );

  return (
    <PageFrame wide>
      <PageHeader
        title={tr('Automatisation', 'Automation')}
        count={`${rules.length} ${tr('règles', 'rules')}`}
        actions={canWrite ? <Btn label={tr('Nouvelle règle', 'New rule')} icon={Plus} primary onClick={openNew} /> : undefined}
      />

      <div className="flex gap-2 mb-4 flex-wrap">
        <TabBtn id="rules" label={tr('Règles', 'Rules')} count={rules.length} />
        <TabBtn id="sla" label={tr('SLA en cours', 'Live SLA')} count={stats ? stats.open + stats.breached + stats.escalated : undefined} />
        <TabBtn id="history" label={tr('Historique', 'History')} />
        <TabBtn id="channels" label={tr('Canaux', 'Channels')} />
      </div>

      {tab === 'rules' && (
        <RulesView rules={rules} loading={rulesLoading} canWrite={canWrite} onEdit={openEdit} onNew={openNew} />
      )}
      {tab === 'sla' && <SLAView />}
      {tab === 'history' && <HistoryView />}
      {tab === 'channels' && <ChannelsView canWrite={canWrite} />}

      {editorOpen && <RuleEditorModal rule={editorRule} isOpen={editorOpen} onClose={() => setEditorOpen(false)} />}
    </PageFrame>
  );
}

// ---------------------------------------------------------------------------
// Rules — each rule renders its trigger → action chain (the workflow view).
// ---------------------------------------------------------------------------
function RulesView({
  rules, loading, canWrite, onEdit, onNew,
}: { rules: AutomationRule[]; loading: boolean; canWrite: boolean; onEdit: (r: AutomationRule) => void; onNew: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { deleteRule, updateRule, testRule } = useAutomationMutations();

  if (loading && rules.length === 0) return <Card style={{ padding: 12 }}><SkeletonRows rows={4} /></Card>;
  if (rules.length === 0)
    return (
      <EmptyState icon={Workflow}
        title={tr('Aucune automatisation', 'No automations')}
        sub={tr('Chaînez des actions (scan, risque, ticket, alerte, SLA) déclenchées par un événement.', 'Chain actions (scan, risk, ticket, alert, SLA) triggered by an event.')}
        cta={canWrite ? <Btn label={tr('Nouvelle règle', 'New rule')} icon={Plus} primary onClick={onNew} /> : undefined} />
    );

  const toggle = async (r: AutomationRule) => {
    try {
      await updateRule.mutateAsync({ id: r.id, input: { ...r, enabled: !r.enabled } });
    } catch { toast.error(tr('Échec', 'Failed')); }
  };
  const runTest = async (r: AutomationRule) => {
    try {
      const exec = await testRule.mutateAsync({ id: r.id, input: { severity: 'critical', cve_id: 'CVE-0000-TEST', cvss: 9.8, kev: true, priority_tier: 'P1', asset_name: 'test-asset' } });
      toast.success(tr(`Test : ${exec.status} · ${exec.steps?.length ?? 0} étapes`, `Test: ${exec.status} · ${exec.steps?.length ?? 0} steps`));
    } catch { toast.error(tr('Échec du test', 'Test failed')); }
  };
  const remove = async (r: AutomationRule) => {
    if (!confirm(tr(`Supprimer « ${r.name} » ?`, `Delete "${r.name}"?`))) return;
    try { await deleteRule.mutateAsync(r.id); toast.success(tr('Supprimée', 'Deleted')); }
    catch { toast.error(tr('Échec', 'Failed')); }
  };

  return (
    <div className="space-y-3">
      {rules.map((r, idx) => {
        const t = TRIGGER_META[r.trigger];
        const TIcon = t.icon;
        return (
          <Card key={r.id} className="or-fadeup" style={{ padding: '14px 16px', animationDelay: `${Math.min(idx * 40, 240)}ms` }}>
            <div className="flex items-start gap-3 flex-wrap">
              <div className="flex-1 min-w-[240px]">
                <div className="flex items-center gap-2">
                  <span className="text-[14px] font-bold text-ink">{r.name}</span>
                  <span className="h-5 px-2 rounded-full text-[10.5px] font-semibold inline-flex items-center"
                    style={{ background: r.enabled ? 'rgba(46,160,90,.14)' : 'var(--bg-hover)', color: r.enabled ? 'var(--low)' : 'var(--text-secondary)' }}>
                    {r.enabled ? tr('Active', 'Active') : tr('Inactive', 'Inactive')}
                  </span>
                  {r.trigger_count > 0 && (
                    <span className="text-[11px] text-ink-muted inline-flex items-center gap-1"><Activity size={11} /> {r.trigger_count}×</span>
                  )}
                </div>
                {r.description && <div className="text-[12px] text-ink-muted mt-0.5">{r.description}</div>}

                {/* Workflow chain: trigger → actions */}
                <div className="flex items-center gap-1.5 flex-wrap mt-2.5">
                  <span className="h-7 px-2.5 rounded-[8px] text-[11.5px] font-semibold inline-flex items-center gap-1.5" style={{ background: 'var(--bg-hover)', color: 'var(--text-secondary)' }}>
                    <TIcon size={12} /> {pick(t.label, lang)}
                  </span>
                  {r.actions.map((a, i) => {
                    const m = ACTION_META[a.type];
                    const AIcon = m.icon;
                    return (
                      <span key={i} className="inline-flex items-center gap-1.5">
                        <ArrowRight size={13} className="text-ink-muted" />
                        <span className="h-7 px-2.5 rounded-[8px] text-[11.5px] font-medium inline-flex items-center gap-1.5" style={{ border: `1px solid ${m.color}`, color: m.color }}>
                          <AIcon size={12} /> {pick(m.label, lang)}
                          {a.channels && a.channels.length > 0 && <span className="opacity-70">· {a.channels.join('/')}</span>}
                        </span>
                      </span>
                    );
                  })}
                </div>
              </div>

              {canWrite && (
                <div className="flex items-center gap-1.5">
                  <button title={tr('Tester', 'Test')} onClick={() => runTest(r)} className="w-8 h-8 rounded-[8px] flex items-center justify-center text-ink-soft" style={{ background: 'var(--bg-hover)' }}><Play size={14} /></button>
                  <button title={r.enabled ? tr('Désactiver', 'Disable') : tr('Activer', 'Enable')} onClick={() => toggle(r)} className="w-8 h-8 rounded-[8px] flex items-center justify-center" style={{ background: 'var(--bg-hover)', color: r.enabled ? 'var(--low)' : 'var(--text-secondary)' }}><Power size={14} /></button>
                  <button title={tr('Modifier', 'Edit')} onClick={() => onEdit(r)} className="w-8 h-8 rounded-[8px] flex items-center justify-center text-ink-soft" style={{ background: 'var(--bg-hover)' }}><Pencil size={14} /></button>
                  <button title={tr('Supprimer', 'Delete')} onClick={() => remove(r)} className="w-8 h-8 rounded-[8px] flex items-center justify-center" style={{ background: 'var(--bg-hover)', color: 'var(--critical)' }}><Trash2 size={14} /></button>
                </div>
              )}
            </div>
          </Card>
        );
      })}
    </div>
  );
}

// ---------------------------------------------------------------------------
// SLA — live remediation countdown dashboard.
// ---------------------------------------------------------------------------
function SLAView() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data: trackers = [], isLoading } = useSLATrackers();
  const { data: stats } = useSLAStats();

  const kpi = (label: string, value: number, color: string, Icon: typeof Timer) => (
    <Card style={{ padding: '14px 16px', flex: 1, minWidth: 130 }}>
      <div className="flex items-center gap-2 text-ink-muted text-[11px] font-semibold uppercase tracking-[.04em]">
        <Icon size={13} style={{ color }} /> {label}
      </div>
      <div className="mono text-[24px] font-bold mt-1" style={{ color }}>{value}</div>
    </Card>
  );

  return (
    <>
      <div className="flex gap-3 mb-4 flex-wrap">
        {kpi(tr('En cours', 'Open'), stats?.open ?? 0, 'var(--accent)', Timer)}
        {kpi(tr('À risque', 'At risk'), stats?.at_risk ?? 0, 'var(--medium)', AlertTriangle)}
        {kpi(tr('Dépassés', 'Breached'), stats?.breached ?? 0, 'var(--high)', AlertTriangle)}
        {kpi(tr('Escaladés', 'Escalated'), stats?.escalated ?? 0, 'var(--critical)', Siren)}
        {kpi(tr('Respectés', 'Met'), stats?.met ?? 0, 'var(--low)', Timer)}
      </div>

      <Card style={{ padding: '8px 8px 4px' }}>
        {isLoading && trackers.length === 0 ? (
          <SkeletonRows rows={5} />
        ) : trackers.length === 0 ? (
          <EmptyState icon={Timer} title={tr('Aucun SLA en cours', 'No live SLA')} sub={tr('Les compteurs SLA démarrent quand une règle déclenche l’action « Démarrer un SLA ».', 'SLA countdowns start when a rule fires the “Start SLA” action.')} />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-[13px]" style={{ minWidth: 640 }}>
              <thead>
                <tr className="text-ink-muted text-[11px] uppercase tracking-[.04em]">
                  <th className="text-left font-semibold px-3 py-2">{tr('Sujet', 'Subject')}</th>
                  <th className="text-left font-semibold px-3 py-2">{tr('Sévérité', 'Severity')}</th>
                  <th className="text-left font-semibold px-3 py-2">{tr('Statut', 'Status')}</th>
                  <th className="text-left font-semibold px-3 py-2 w-[42%]">{tr('Échéance', 'Deadline')}</th>
                </tr>
              </thead>
              <tbody>
                {trackers.map((t) => {
                  const sm = SLA_STATUS_META[t.status];
                  const overdue = t.remaining_minutes < 0;
                  const barColor = overdue ? 'var(--critical)' : t.remaining_minutes < 60 ? 'var(--high)' : 'var(--accent)';
                  // Fraction of the budget consumed (created_at → due_at).
                  const totalMin = (new Date(t.due_at).getTime() - new Date(t.created_at).getTime()) / 60000;
                  const consumed = totalMin > 0 ? (totalMin - t.remaining_minutes) / totalMin : 1;
                  const pct = overdue ? 100 : Math.max(4, Math.min(100, consumed * 100));
                  return (
                    <tr key={t.id} style={{ borderTop: '1px solid var(--border)' }}>
                      <td className="px-3 py-2.5">
                        <div className="font-medium text-ink truncate max-w-[240px]">{t.title || t.subject_id}</div>
                        <div className="text-[11px] text-ink-muted">{t.subject_type}{t.escalation_level > 0 && ` · ${tr('escalade', 'escalation')} L${t.escalation_level}`}</div>
                      </td>
                      <td className="px-3 py-2.5">
                        <span className="h-5 px-2 rounded-full text-[10.5px] font-semibold uppercase" style={{ background: 'var(--bg-hover)', color: SEVERITY_COLOR[t.severity] ?? 'var(--text-secondary)' }}>{t.severity}</span>
                      </td>
                      <td className="px-3 py-2.5">
                        <span className="h-5 px-2 rounded-full text-[10.5px] font-semibold inline-flex items-center" style={{ background: 'var(--bg-hover)', color: sm.color }}>{pick(sm.label, lang)}</span>
                      </td>
                      <td className="px-3 py-2.5">
                        <div className="flex items-center gap-2">
                          <div className="flex-1 h-1.5 rounded-full overflow-hidden" style={{ background: 'var(--bg-hover)' }}>
                            <div className="h-full rounded-full" style={{ width: `${pct}%`, background: barColor }} />
                          </div>
                          <span className="mono text-[11.5px] shrink-0" style={{ color: overdue ? 'var(--critical)' : 'var(--text-secondary)' }}>{fmtMinutes(t.remaining_minutes, lang)}</span>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </>
  );
}

// ---------------------------------------------------------------------------
// History — execution audit trail.
// ---------------------------------------------------------------------------
function HistoryView() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data: execs = [], isLoading } = useAutomationExecutions();
  const [openId, setOpenId] = useState<string | null>(null);

  if (isLoading && execs.length === 0) return <Card style={{ padding: 12 }}><SkeletonRows rows={5} /></Card>;
  if (execs.length === 0)
    return <EmptyState icon={Activity} title={tr('Aucune exécution', 'No executions')} sub={tr('Les exécutions apparaissent quand une règle se déclenche.', 'Executions appear when a rule fires.')} />;

  const stepColor = (s: string) => (s === 'success' ? 'var(--low)' : s === 'failed' ? 'var(--critical)' : 'var(--text-secondary)');

  return (
    <div className="space-y-2">
      {execs.map((e) => {
        const sm = EXEC_STATUS_META[e.status];
        const open = openId === e.id;
        return (
          <Card key={e.id} style={{ padding: '10px 14px' }}>
            <button onClick={() => setOpenId(open ? null : e.id)} className="w-full flex items-center gap-2.5 text-left">
              <ChevronRight size={14} className="text-ink-muted transition-transform" style={{ transform: open ? 'rotate(90deg)' : 'none' }} />
              <span className="h-5 px-2 rounded-full text-[10.5px] font-semibold" style={{ background: 'var(--bg-hover)', color: sm.color }}>{pick(sm.label, lang)}</span>
              <span className="text-[13px] font-semibold text-ink flex-1 truncate">{e.rule_name}</span>
              <span className="mono text-[11px] text-ink-muted truncate max-w-[180px]">{e.trigger_ref}</span>
              <span className="text-[11px] text-ink-muted shrink-0">{new Date(e.started_at).toLocaleString()}</span>
            </button>
            {open && (
              <div className="mt-2.5 pl-6 space-y-1">
                {(e.steps ?? []).map((s, i) => (
                  <div key={i} className="flex items-start gap-2 text-[12px]">
                    <span className="mono text-[10.5px] w-4 text-ink-muted">{i + 1}</span>
                    <span className="font-semibold" style={{ color: stepColor(s.status), minWidth: 96 }}>{s.action}</span>
                    <span className="text-ink-muted">{s.detail}</span>
                  </div>
                ))}
                {(e.steps ?? []).length === 0 && <div className="text-[12px] text-ink-muted">{tr('Aucune étape', 'No steps')}</div>}
              </div>
            )}
          </Card>
        );
      })}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Channels — tenant Slack/Teams/email configuration.
// ---------------------------------------------------------------------------
function ChannelsView({ canWrite }: { canWrite: boolean }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data: cfg } = useChannelConfig();
  const { saveChannels } = useAutomationMutations();

  const [slackEnabled, setSlackEnabled] = useState<boolean | null>(null);
  const [teamsEnabled, setTeamsEnabled] = useState<boolean | null>(null);
  const [emailEnabled, setEmailEnabled] = useState<boolean | null>(null);
  const [slackUrl, setSlackUrl] = useState('');
  const [teamsUrl, setTeamsUrl] = useState('');
  const [defaultEmail, setDefaultEmail] = useState<string | null>(null);

  const slackOn = slackEnabled ?? cfg?.slack_enabled ?? false;
  const teamsOn = teamsEnabled ?? cfg?.teams_enabled ?? false;
  const emailOn = emailEnabled ?? cfg?.email_enabled ?? true;
  const email = defaultEmail ?? cfg?.default_email ?? '';

  const inputCls = 'w-full h-9 px-3 rounded-[9px] text-[13px] text-ink bg-transparent outline-none';
  const inputStyle = { border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' } as const;

  const save = async () => {
    try {
      await saveChannels.mutateAsync({
        slack_enabled: slackOn,
        slack_webhook_url: slackUrl || undefined,
        teams_enabled: teamsOn,
        teams_webhook_url: teamsUrl || undefined,
        email_enabled: emailOn,
        default_email: email,
      });
      toast.success(tr('Canaux enregistrés', 'Channels saved'));
      setSlackUrl(''); setTeamsUrl('');
    } catch { toast.error(tr('Échec', 'Failed')); }
  };

  const row = (Icon: typeof Bell, title: string, on: boolean, setOn: (v: boolean) => void, extra?: React.ReactNode) => (
    <div className="rounded-[12px] p-3.5" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}>
      <label className="flex items-center gap-2.5">
        <input type="checkbox" checked={on} disabled={!canWrite} onChange={(e) => setOn(e.target.checked)} />
        <Icon size={16} className="text-ink-soft" />
        <span className="text-[13px] font-semibold text-ink flex-1">{title}</span>
      </label>
      {extra && <div className="mt-2.5">{extra}</div>}
    </div>
  );

  return (
    <div className="max-w-[620px] space-y-3">
      <div className="text-[12.5px] text-ink-muted mb-1">
        {tr('Configurez où les automatisations envoient leurs alertes. Les URL de webhook sont en écriture seule (jamais réaffichées).',
          'Configure where automations send alerts. Webhook URLs are write-only (never shown again).')}
      </div>
      {row(MessageSquare, 'Slack', slackOn, setSlackEnabled,
        <input className={inputCls} style={inputStyle} placeholder={cfg?.has_slack ? tr('Configuré — coller pour remplacer', 'Configured — paste to replace') : 'https://hooks.slack.com/services/…'} value={slackUrl} onChange={(e) => setSlackUrl(e.target.value)} disabled={!canWrite} />)}
      {row(MessageSquare, 'Microsoft Teams', teamsOn, setTeamsEnabled,
        <input className={inputCls} style={inputStyle} placeholder={cfg?.has_teams ? tr('Configuré — coller pour remplacer', 'Configured — paste to replace') : 'https://outlook.office.com/webhook/…'} value={teamsUrl} onChange={(e) => setTeamsUrl(e.target.value)} disabled={!canWrite} />)}
      {row(Mail, 'Email', emailOn, setEmailEnabled,
        <input className={inputCls} style={inputStyle} placeholder={tr('Email de repli (SOC)', 'Fallback email (SOC)')} value={email} onChange={(e) => setDefaultEmail(e.target.value)} disabled={!canWrite} />)}
      {canWrite && <div className="flex justify-end"><Btn label={tr('Enregistrer', 'Save')} primary onClick={save} /></div>}
    </div>
  );
}
