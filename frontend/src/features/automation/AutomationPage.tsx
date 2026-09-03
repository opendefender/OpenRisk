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
  Workflow,
  Plus,
  Pencil,
  Trash2,
  Timer,
  AlertTriangle,
  Siren,
  Activity,
  ChevronRight,
  ArrowRight,
  Bell,
  MessageSquare,
  Mail,
  FlaskConical,
  Pause,
  PlayCircle,
  RotateCcw,
  Sparkles,
  Webhook,
  Smartphone,
  CheckCircle2,
  XCircle,
  Loader2,
  HelpCircle,
} from 'lucide-react';
import { PageFrame, PageHeader, Btn, Card, SkeletonRows, EmptyState } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import {
  useAutomationRules,
  useAutomationExecutions,
  useSLATrackers,
  useSLAStats,
  useChannelConfig,
  useAutomationMutations,
  useAutomationState,
  useAutomationTemplates,
  useChannelCatalogue,
} from './useAutomation';
import type {
  AutomationRule,
  RuleHealth,
  NotifyChannel,
  ChannelTestResult,
} from './automationService';
import { DryRunPanel } from './DryRunPanel';
import {
  TRIGGER_META,
  ACTION_META,
  EXEC_STATUS_META,
  SLA_STATUS_META,
  SEVERITY_COLOR,
  pick,
  fmtMinutes,
} from './automationMeta';
import { RuleEditorModal } from './RuleEditorModal';

type Tab = 'rules' | 'sla' | 'history' | 'channels';

// One sentence per tab, shown above its content. A tab whose purpose you have
// to infer from its contents is a tab people click once.
const TAB_PURPOSE: Record<Tab, { fr: string; en: string }> = {
  rules: {
    fr: 'Une règle dit : quand ceci arrive, si ces conditions sont réunies, alors faire cela.',
    en: 'A rule says: when this happens, if these conditions hold, then do that.',
  },
  sla: {
    fr: 'Le compte à rebours de chaque remédiation en cours, et ce qui se passe quand il expire.',
    en: 'The countdown on every remediation in flight, and what happens when it runs out.',
  },
  history: {
    fr: 'Chaque déclenchement passé : ce qui est entré, ce qui en est sorti, et qui en répond.',
    en: 'Every past firing: what went in, what came out, and who is answerable for it.',
  },
  channels: {
    fr: 'Où partent les alertes. Testez chaque canal séparément pour savoir lequel est muet.',
    en: 'Where alerts go. Test each channel on its own to find out which one is silent.',
  },
};

// Health colours for the live state indicator.
const HEALTH_META: Record<RuleHealth, { color: string; fr: string; en: string }> = {
  ok: { color: 'var(--low)', fr: 'OK', en: 'OK' },
  degraded: { color: 'var(--medium)', fr: 'Dégradée', en: 'Degraded' },
  failing: { color: 'var(--critical)', fr: 'En échec', en: 'Failing' },
  suspended: { color: 'var(--fg-secondary)', fr: 'Suspendue', en: 'Suspended' },
  idle: { color: 'var(--accent-500)', fr: 'En attente', en: 'Waiting' },
};

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

  const openNew = () => {
    setEditorRule(null);
    setEditorOpen(true);
  };
  const openEdit = (r: AutomationRule) => {
    setEditorRule(r);
    setEditorOpen(true);
  };

  const TabBtn = ({ id, label, count }: { id: Tab; label: string; count?: number }) => (
    <button
      onClick={() => setTab(id)}
      className="h-9 px-3.5 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5"
      style={{
        background: tab === id ? 'var(--accent)' : 'transparent',
        color: tab === id ? '#fff' : 'var(--fg-secondary)',
        border: tab === id ? 'none' : '1px solid var(--border-strong)',
      }}
    >
      {label}
      {typeof count === 'number' && <span className="mono opacity-80">{count}</span>}
    </button>
  );

  return (
    <PageFrame wide>
      <PageHeader
        title={tr('Automatisation', 'Automation')}
        count={`${rules.length} ${tr('règles', 'rules')}`}
        actions={
          canWrite ? (
            <Btn label={tr('Nouvelle règle', 'New rule')} icon={Plus} primary onClick={openNew} />
          ) : undefined
        }
      />

      <div className="flex gap-2 mb-4 flex-wrap">
        <TabBtn id="rules" label={tr('Règles', 'Rules')} count={rules.length} />
        <TabBtn
          id="sla"
          label={tr('SLA en cours', 'Live SLA')}
          count={stats ? stats.open + stats.breached + stats.escalated : undefined}
        />
        <TabBtn id="history" label={tr('Historique', 'History')} />
        <TabBtn id="channels" label={tr('Canaux', 'Channels')} />
      </div>

      <p className="text-[12.5px] mb-3" style={{ color: 'var(--fg-secondary)' }}>
        {tr(TAB_PURPOSE[tab].fr, TAB_PURPOSE[tab].en)}
      </p>

      {tab === 'rules' && (
        <RulesView
          rules={rules}
          loading={rulesLoading}
          canWrite={canWrite}
          onEdit={openEdit}
          onNew={openNew}
        />
      )}
      {tab === 'sla' && <SLAView />}
      {tab === 'history' && <HistoryView />}
      {tab === 'channels' && <ChannelsView canWrite={canWrite} />}

      {editorOpen && (
        <RuleEditorModal
          rule={editorRule}
          isOpen={editorOpen}
          onClose={() => setEditorOpen(false)}
        />
      )}
    </PageFrame>
  );
}

// ---------------------------------------------------------------------------
// Rules — each rule renders its trigger → action chain (the workflow view).
// ---------------------------------------------------------------------------
function RulesView({
  rules,
  loading,
  canWrite,
  onEdit,
  onNew,
}: {
  rules: AutomationRule[];
  loading: boolean;
  canWrite: boolean;
  onEdit: (r: AutomationRule) => void;
  onNew: () => void;
}) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { deleteRule, enableRule, suspendRule } = useAutomationMutations();
  const { data: state } = useAutomationState();
  const [testing, setTesting] = useState<AutomationRule | null>(null);
  const [showTemplates, setShowTemplates] = useState(false);

  if (loading && rules.length === 0)
    return (
      <Card style={{ padding: 12 }}>
        <SkeletonRows rows={4} />
      </Card>
    );

  if (rules.length === 0)
    return (
      <>
        <EmptyState
          icon={Workflow}
          title={tr('Aucune règle pour l’instant', 'No rules yet')}
          description={tr(
            'Une règle enchaîne des actions — ouvrir un risque, alerter, démarrer un SLA — dès qu’un événement survient. Partez d’un modèle prêt à l’emploi, testez-le sur vos vraies données, puis activez-le.',
            'A rule chains actions — open a risk, alert, start an SLA — the moment an event happens. Start from a ready-made template, test it on your real data, then switch it on.',
          )}
          primaryAction={
            canWrite ? (
              <Btn
                label={tr('Partir d’un modèle', 'Start from a template')}
                icon={Sparkles}
                primary
                onClick={() => setShowTemplates(true)}
              />
            ) : undefined
          }
          secondaryAction={
            canWrite ? (
              <Btn label={tr('Créer de zéro', 'Build from scratch')} icon={Plus} onClick={onNew} />
            ) : undefined
          }
        />
        {showTemplates && <TemplateGallery onClose={() => setShowTemplates(false)} />}
      </>
    );

  const stateOf = (id: string) => state?.rules.find((r) => r.rule_id === id);

  const toggle = async (r: AutomationRule) => {
    try {
      if (r.enabled) {
        // Suspending demands a reason: the next person needs to know why before
        // switching it back on.
        const reason = window.prompt(
          tr(
            'Pourquoi suspendre cette règle ? (visible par toute l’équipe)',
            'Why are you suspending this rule? (visible to the whole team)',
          ),
        );
        if (reason === null) return;
        if (!reason.trim()) {
          toast.error(tr('Une raison est requise', 'A reason is required'));
          return;
        }
        await suspendRule.mutateAsync({ id: r.id, reason: reason.trim() });
        toast.success(tr('Règle suspendue', 'Rule suspended'));
      } else {
        await enableRule.mutateAsync(r.id);
        toast.success(tr('Règle activée', 'Rule enabled'));
      }
    } catch {
      toast.error(tr('Échec', 'Failed'));
    }
  };

  const remove = async (r: AutomationRule) => {
    if (!confirm(tr(`Supprimer « ${r.name} » ?`, `Delete "${r.name}"?`))) return;
    try {
      await deleteRule.mutateAsync(r.id);
      toast.success(tr('Supprimée', 'Deleted'));
    } catch {
      toast.error(tr('Échec', 'Failed'));
    }
  };

  return (
    <div className="space-y-3">
      {/* Live state strip — the "indicateur d'état temps réel". */}
      {state && (
        <div className="flex items-center gap-2 flex-wrap text-[12px]">
          <span
            className="inline-flex items-center gap-1.5"
            style={{ color: 'var(--fg-secondary)' }}
          >
            <span className="w-1.5 h-1.5 rounded-full" style={{ background: 'var(--low)' }} />
            {tr('État en direct', 'Live state')}
          </span>
          {(
            [
              ['active', state.active, 'var(--low)', tr('actives', 'active')],
              ['failing', state.failing, 'var(--critical)', tr('en échec', 'failing')],
              ['degraded', state.degraded, 'var(--medium)', tr('dégradées', 'degraded')],
              ['suspended', state.suspended, 'var(--fg-secondary)', tr('suspendues', 'suspended')],
              ['idle', state.idle, 'var(--accent)', tr('en attente', 'waiting')],
            ] as const
          )
            .filter(([, n]) => n > 0)
            .map(([k, n, color, label]) => (
              <span
                key={k}
                className="px-2 py-0.5 rounded-[7px] font-semibold"
                style={{ background: `color-mix(in srgb, ${color} 12%, transparent)`, color }}
              >
                {n} {label}
              </span>
            ))}
          {canWrite && (
            <button
              onClick={() => setShowTemplates(true)}
              className="ml-auto inline-flex items-center gap-1.5 text-[12px] font-semibold"
              style={{ color: 'var(--accent-500)' }}
            >
              <Sparkles size={13} /> {tr('Modèles prêts à l’emploi', 'Ready-made templates')}
            </button>
          )}
        </div>
      )}

      {rules.map((r, idx) => {
        const t = TRIGGER_META[r.trigger];
        const TIcon = t.icon;
        const live = stateOf(r.id);
        const health = live ? HEALTH_META[live.health] : null;
        return (
          <Card
            key={r.id}
            className="or-fadeup"
            style={{ padding: '14px 16px', animationDelay: `${Math.min(idx * 40, 240)}ms` }}
          >
            <div className="flex items-start gap-3 flex-wrap">
              <div className="flex-1 min-w-[240px]">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="text-[14px] font-bold text-ink">{r.name}</span>
                  {health && (
                    <span
                      title={live?.health_detail}
                      className="h-5 px-2 rounded-full text-[10.5px] font-semibold inline-flex items-center gap-1.5"
                      style={{
                        background: `color-mix(in srgb, ${health.color} 14%, transparent)`,
                        color: health.color,
                      }}
                    >
                      <span
                        className="w-1.5 h-1.5 rounded-full"
                        style={{ background: health.color }}
                      />
                      {pick({ fr: health.fr, en: health.en }, lang)}
                    </span>
                  )}
                  {r.trigger_count > 0 && (
                    <span className="text-[11px] text-ink-muted inline-flex items-center gap-1">
                      <Activity size={11} /> {r.trigger_count}×
                    </span>
                  )}
                </div>

                {/* The rule as a sentence — what the near-natural-language
                    builder produced, read back. */}
                {live?.sentence && (
                  <div className="text-[12.5px] mt-1" style={{ color: 'var(--fg-primary)' }}>
                    {live.sentence}
                  </div>
                )}
                {live?.health_detail && live.health !== 'ok' && (
                  <div className="text-[11.5px] mt-0.5" style={{ color: health?.color }}>
                    {live.health_detail}
                  </div>
                )}

                {/* Workflow chain: trigger → actions */}
                <div className="flex items-center gap-1.5 flex-wrap mt-2.5">
                  <span
                    className="h-7 px-2.5 rounded-[8px] text-[11.5px] font-semibold inline-flex items-center gap-1.5"
                    style={{ background: 'var(--bg-hover)', color: 'var(--fg-secondary)' }}
                  >
                    <TIcon size={12} /> {pick(t.label, lang)}
                  </span>
                  {r.actions.map((a, i) => {
                    const m = ACTION_META[a.type];
                    const AIcon = m.icon;
                    return (
                      <span key={i} className="inline-flex items-center gap-1.5">
                        <ArrowRight size={13} className="text-ink-muted" />
                        <span
                          className="h-7 px-2.5 rounded-[8px] text-[11.5px] font-medium inline-flex items-center gap-1.5"
                          style={{ border: `1px solid ${m.color}`, color: m.color }}
                        >
                          <AIcon size={12} /> {pick(m.label, lang)}
                          {a.channels && a.channels.length > 0 && (
                            <span className="opacity-70">· {a.channels.join('/')}</span>
                          )}
                        </span>
                      </span>
                    );
                  })}
                </div>
              </div>

              <div className="flex items-center gap-1.5">
                {/* Testing is read-only, so it is offered to readers too. */}
                <button
                  title={tr('Tester sans rien exécuter', 'Test without running anything')}
                  onClick={() => setTesting(r)}
                  className="h-8 px-2.5 rounded-[8px] flex items-center gap-1.5 text-[12px] font-semibold"
                  style={{ background: 'var(--bg-hover)', color: 'var(--accent-500)' }}
                >
                  <FlaskConical size={14} /> {tr('Tester', 'Test')}
                </button>
                {canWrite && (
                  <>
                    <button
                      title={r.enabled ? tr('Suspendre', 'Suspend') : tr('Activer', 'Enable')}
                      onClick={() => toggle(r)}
                      className="w-8 h-8 rounded-[8px] flex items-center justify-center"
                      style={{
                        background: 'var(--bg-hover)',
                        color: r.enabled ? 'var(--medium)' : 'var(--low)',
                      }}
                    >
                      {r.enabled ? <Pause size={14} /> : <PlayCircle size={14} />}
                    </button>
                    <button
                      title={tr('Modifier', 'Edit')}
                      onClick={() => onEdit(r)}
                      className="w-8 h-8 rounded-[8px] flex items-center justify-center text-ink-soft"
                      style={{ background: 'var(--bg-hover)' }}
                    >
                      <Pencil size={14} />
                    </button>
                    <button
                      title={tr('Supprimer', 'Delete')}
                      onClick={() => remove(r)}
                      className="w-8 h-8 rounded-[8px] flex items-center justify-center"
                      style={{ background: 'var(--bg-hover)', color: 'var(--critical)' }}
                    >
                      <Trash2 size={14} />
                    </button>
                  </>
                )}
              </div>
            </div>

            {!r.enabled && r.suspended_reason && (
              <div
                className="mt-2.5 text-[12px] rounded-[8px] px-2.5 py-1.5"
                style={{ background: 'var(--bg-hover)', color: 'var(--fg-secondary)' }}
              >
                {tr('Suspendue : ', 'Suspended: ')}
                {r.suspended_reason}
              </div>
            )}
          </Card>
        );
      })}

      {testing && <DryRunPanel rule={testing} onClose={() => setTesting(null)} />}
      {showTemplates && <TemplateGallery onClose={() => setShowTemplates(false)} />}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Ready-made templates. Each one adopts as a SUSPENDED rule so it can be
// dry-run before it ever fires — the whole point of shipping them.
// ---------------------------------------------------------------------------
function TemplateGallery({ onClose }: { onClose: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data: templates = [], isLoading } = useAutomationTemplates();
  const { adoptTemplate } = useAutomationMutations();

  const adopt = async (key: string, name: string) => {
    try {
      await adoptTemplate.mutateAsync({ key });
      toast.success(
        tr(
          `« ${name} » ajoutée — suspendue. Testez-la, puis activez-la.`,
          `"${name}" added — suspended. Test it, then switch it on.`,
        ),
      );
      onClose();
    } catch {
      toast.error(tr('Échec', 'Failed'));
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: 'rgba(0,0,0,.35)' }}
    >
      <div
        className="w-full max-w-[720px] max-h-[85vh] flex flex-col rounded-[14px] or-scalein"
        style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)' }}
      >
        <div
          className="px-5 py-4 flex items-center justify-between"
          style={{ borderBottom: '1px solid var(--border)' }}
        >
          <div>
            <h2 className="text-[15px] font-bold text-ink inline-flex items-center gap-2">
              <Sparkles size={16} style={{ color: 'var(--accent-500)' }} />
              {tr('Modèles prêts à l’emploi', 'Ready-made templates')}
            </h2>
            <p className="text-[12px] mt-0.5" style={{ color: 'var(--fg-secondary)' }}>
              {tr(
                'Ajoutées suspendues : testez-les sur vos données avant qu’elles ne se déclenchent.',
                'Added suspended: test them against your data before they ever fire.',
              )}
            </p>
          </div>
          <button onClick={onClose} className="p-1.5 rounded-lg">
            <XCircle size={18} />
          </button>
        </div>
        <div className="p-5 space-y-3 overflow-y-auto">
          {isLoading && <SkeletonRows rows={3} />}
          {templates.map(({ template, sentence }) => (
            <Card key={template.key} style={{ padding: '14px 16px' }}>
              <div className="flex items-start gap-3 flex-wrap">
                <div className="flex-1 min-w-[240px]">
                  <div className="text-[13.5px] font-bold text-ink">
                    {lang === 'fr' ? template.name : template.name_en}
                  </div>
                  <p className="text-[12px] mt-0.5" style={{ color: 'var(--fg-secondary)' }}>
                    {lang === 'fr' ? template.use_case : template.use_case_en}
                  </p>
                  <p className="text-[12.5px] mt-1.5" style={{ color: 'var(--fg-primary)' }}>
                    {sentence}
                  </p>
                  {(template.requires_channels || template.requires_ticketing) && (
                    <p
                      className="text-[11.5px] mt-1.5 inline-flex items-center gap-1.5"
                      style={{ color: 'var(--medium)' }}
                    >
                      <AlertTriangle size={12} />
                      {tr('Nécessite : ', 'Needs: ')}
                      {[
                        template.requires_channels && tr('un canal d’alerte', 'an alert channel'),
                        template.requires_ticketing &&
                          tr('une intégration ITSM', 'an ITSM integration'),
                      ]
                        .filter(Boolean)
                        .join(' · ')}
                    </p>
                  )}
                </div>
                <Btn
                  label={tr('Ajouter', 'Add')}
                  icon={Plus}
                  primary
                  onClick={() =>
                    adopt(template.key, lang === 'fr' ? template.name : template.name_en)
                  }
                />
              </div>
            </Card>
          ))}
        </div>
      </div>
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
      <div className="mono text-[24px] font-bold mt-1" style={{ color }}>
        {value}
      </div>
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
          <EmptyState
            icon={Timer}
            title={tr('Aucun SLA en cours', 'No live SLA')}
            description={tr(
              'Les compteurs SLA démarrent quand une règle déclenche l’action « Démarrer un SLA ».',
              'SLA countdowns start when a rule fires the “Start SLA” action.',
            )}
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-[13px]" style={{ minWidth: 640 }}>
              <thead>
                <tr className="text-ink-muted text-[11px] uppercase tracking-[.04em]">
                  <th className="text-left font-semibold px-3 py-2">{tr('Sujet', 'Subject')}</th>
                  <th className="text-left font-semibold px-3 py-2">
                    {tr('Sévérité', 'Severity')}
                  </th>
                  <th className="text-left font-semibold px-3 py-2">{tr('Statut', 'Status')}</th>
                  <th className="text-left font-semibold px-3 py-2 w-[42%]">
                    {tr('Échéance', 'Deadline')}
                  </th>
                </tr>
              </thead>
              <tbody>
                {trackers.map((t) => {
                  const sm = SLA_STATUS_META[t.status];
                  const overdue = t.remaining_minutes < 0;
                  const barColor = overdue
                    ? 'var(--critical)'
                    : t.remaining_minutes < 60
                      ? 'var(--high)'
                      : 'var(--accent)';
                  // Fraction of the budget consumed (created_at → due_at).
                  const totalMin =
                    (new Date(t.due_at).getTime() - new Date(t.created_at).getTime()) / 60000;
                  const consumed = totalMin > 0 ? (totalMin - t.remaining_minutes) / totalMin : 1;
                  const pct = overdue ? 100 : Math.max(4, Math.min(100, consumed * 100));
                  return (
                    <tr key={t.id} style={{ borderTop: '1px solid var(--border)' }}>
                      <td className="px-3 py-2.5">
                        <div className="font-medium text-ink truncate max-w-[240px]">
                          {t.title || t.subject_id}
                        </div>
                        <div className="text-[11px] text-ink-muted">
                          {t.subject_type}
                          {t.escalation_level > 0 &&
                            ` · ${tr('escalade', 'escalation')} L${t.escalation_level}`}
                        </div>
                      </td>
                      <td className="px-3 py-2.5">
                        <span
                          className="h-5 px-2 rounded-full text-[10.5px] font-semibold uppercase"
                          style={{
                            background: 'var(--bg-hover)',
                            color: SEVERITY_COLOR[t.severity] ?? 'var(--fg-secondary)',
                          }}
                        >
                          {t.severity}
                        </span>
                      </td>
                      <td className="px-3 py-2.5">
                        <span
                          className="h-5 px-2 rounded-full text-[10.5px] font-semibold inline-flex items-center"
                          style={{ background: 'var(--bg-hover)', color: sm.color }}
                        >
                          {pick(sm.label, lang)}
                        </span>
                      </td>
                      <td className="px-3 py-2.5">
                        <div className="flex items-center gap-2">
                          <div
                            className="flex-1 h-1.5 rounded-full overflow-hidden"
                            style={{ background: 'var(--bg-hover)' }}
                          >
                            <div
                              className="h-full rounded-full"
                              style={{ width: `${pct}%`, background: barColor }}
                            />
                          </div>
                          <span
                            className="mono text-[11.5px] shrink-0"
                            style={{ color: overdue ? 'var(--critical)' : 'var(--fg-secondary)' }}
                          >
                            {fmtMinutes(t.remaining_minutes, lang)}
                          </span>
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
// History — what went in, what came out, how long it took, and who is
// answerable for it. Everything a run is asked about after the fact.
// ---------------------------------------------------------------------------
function HistoryView() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data: execs = [], isLoading } = useAutomationExecutions();
  const { replayExecution } = useAutomationMutations();
  const [openId, setOpenId] = useState<string | null>(null);
  const canWrite = useAuthStore((s) => s.hasPermission)('automation:write');

  if (isLoading && execs.length === 0)
    return (
      <Card style={{ padding: 12 }}>
        <SkeletonRows rows={5} />
      </Card>
    );
  if (execs.length === 0)
    return (
      <EmptyState
        icon={Activity}
        title={tr('Aucune exécution', 'No runs yet')}
        description={tr(
          'Chaque fois qu’une règle se déclenche, sa trace apparaît ici : ce qui l’a déclenchée, chaque étape avec sa durée, le résultat, et qui l’a lancée. Vous pourrez rejouer une exécution à l’identique.',
          'Every time a rule fires, its trace lands here: what triggered it, each step with its duration, the outcome, and who started it. You will be able to replay a run exactly as it was.',
        )}
      />
    );

  const stepColor = (st: string) =>
    st === 'success' ? 'var(--low)' : st === 'failed' ? 'var(--critical)' : 'var(--fg-secondary)';
  const modeLabel = (m: string) =>
    m === 'replay'
      ? tr('rejeu', 'replay')
      : m === 'manual'
        ? tr('manuel', 'manual')
        : tr('événement', 'live');

  const replay = async (id: string) => {
    try {
      const exec = await replayExecution.mutateAsync(id);
      toast.success(tr(`Rejoué : ${exec.status}`, `Replayed: ${exec.status}`));
    } catch {
      toast.error(tr('Le rejeu a échoué', 'Replay failed'));
    }
  };

  return (
    <div className="space-y-2">
      {execs.map((e) => {
        const sm = EXEC_STATUS_META[e.status];
        const open = openId === e.id;
        return (
          <Card key={e.id} style={{ padding: '10px 14px' }}>
            <div className="flex items-center gap-2.5">
              <button
                onClick={() => setOpenId(open ? null : e.id)}
                className="flex-1 min-w-0 flex items-center gap-2.5 text-left"
              >
                <ChevronRight
                  size={14}
                  className="text-ink-muted transition-transform"
                  style={{ transform: open ? 'rotate(90deg)' : 'none' }}
                />
                <span
                  className="h-5 px-2 rounded-full text-[10.5px] font-semibold"
                  style={{ background: 'var(--bg-hover)', color: sm.color }}
                >
                  {pick(sm.label, lang)}
                </span>
                <span className="text-[13px] font-semibold text-ink truncate">{e.rule_name}</span>
                {e.step_summary && (
                  <span className="text-[11px] text-ink-muted shrink-0">{e.step_summary}</span>
                )}
                <span
                  className="text-[10.5px] px-1.5 py-0.5 rounded shrink-0"
                  style={{ background: 'var(--bg-hover)', color: 'var(--fg-secondary)' }}
                >
                  {modeLabel(e.mode)}
                </span>
                <span className="mono text-[11px] text-ink-muted shrink-0">{e.duration_ms} ms</span>
                <span className="text-[11px] text-ink-muted truncate hidden md:inline">
                  {e.actor_email || e.trigger_ref}
                </span>
                <span className="text-[11px] text-ink-muted shrink-0 ml-auto">
                  {new Date(e.started_at).toLocaleString()}
                </span>
              </button>
              {canWrite && (
                <button
                  title={tr('Rejouer avec les mêmes données', 'Replay with the same input')}
                  onClick={() => replay(e.id)}
                  className="w-8 h-8 rounded-[8px] flex items-center justify-center shrink-0"
                  style={{ background: 'var(--bg-hover)', color: 'var(--accent-500)' }}
                >
                  <RotateCcw size={14} />
                </button>
              )}
            </div>

            {open && (
              <div className="mt-3 pl-6 space-y-3">
                {e.error && (
                  <div
                    className="text-[12px] rounded-[8px] px-2.5 py-1.5"
                    style={{
                      background: 'color-mix(in srgb, var(--critical) 8%, transparent)',
                      color: 'var(--critical)',
                    }}
                  >
                    {e.error}
                  </div>
                )}
                <div className="grid gap-3 md:grid-cols-2">
                  <div>
                    <p
                      className="text-[11px] font-bold uppercase tracking-wide mb-1"
                      style={{ color: 'var(--fg-secondary)' }}
                    >
                      {tr('Entrée', 'Input')}
                    </p>
                    <pre
                      className="text-[11px] mono overflow-x-auto p-2 rounded-[8px]"
                      style={{ background: 'var(--bg)', color: 'var(--fg-primary)' }}
                    >
                      {JSON.stringify(e.input ?? {}, null, 2)}
                    </pre>
                  </div>
                  <div>
                    <p
                      className="text-[11px] font-bold uppercase tracking-wide mb-1"
                      style={{ color: 'var(--fg-secondary)' }}
                    >
                      {tr('Sortie', 'Output')}
                    </p>
                    <pre
                      className="text-[11px] mono overflow-x-auto p-2 rounded-[8px]"
                      style={{ background: 'var(--bg)', color: 'var(--fg-primary)' }}
                    >
                      {JSON.stringify(e.output ?? {}, null, 2)}
                    </pre>
                  </div>
                </div>
                <div className="space-y-1">
                  {(e.steps ?? []).map((st, i) => (
                    <div key={i} className="flex items-start gap-2 text-[12px]">
                      <span className="mono text-[10.5px] w-4 text-ink-muted">{i + 1}</span>
                      <span
                        className="font-semibold shrink-0"
                        style={{ color: stepColor(st.status), minWidth: 96 }}
                      >
                        {st.action}
                      </span>
                      <span className="text-ink-muted flex-1">{st.detail}</span>
                      <span className="mono text-[10.5px] text-ink-muted shrink-0">
                        {st.duration_ms} ms
                      </span>
                    </div>
                  ))}
                  {(e.steps ?? []).length === 0 && (
                    <div className="text-[12px] text-ink-muted">
                      {tr('Aucune étape', 'No steps')}
                    </div>
                  )}
                </div>
                {e.replayed_from && (
                  <p className="text-[11.5px]" style={{ color: 'var(--fg-secondary)' }}>
                    {tr('Rejeu d’une exécution antérieure.', 'Replay of an earlier run.')}
                  </p>
                )}
              </div>
            )}
          </Card>
        );
      })}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Channels — configuration AND a real per-channel delivery test. "Alerts don't
// arrive" is the hardest automation bug to chase; testing one channel at a time
// removes the guessing.
// ---------------------------------------------------------------------------
const CHANNEL_ROWS: {
  key: NotifyChannel;
  icon: typeof Bell;
  title: string;
  hint: { fr: string; en: string };
}[] = [
  {
    key: 'in_app',
    icon: Bell,
    title: 'In-app',
    hint: {
      fr: 'Toujours disponible — la cloche de l’application.',
      en: 'Always available — the in-app bell.',
    },
  },
  {
    key: 'email',
    icon: Mail,
    title: 'Email',
    hint: {
      fr: 'Adresse de repli quand une alerte par rôle ne trouve personne.',
      en: 'Fallback address when a role-based alert resolves to nobody.',
    },
  },
  {
    key: 'slack',
    icon: MessageSquare,
    title: 'Slack',
    hint: { fr: 'Webhook entrant Slack.', en: 'Slack incoming webhook.' },
  },
  {
    key: 'teams',
    icon: MessageSquare,
    title: 'Microsoft Teams',
    hint: { fr: 'Webhook entrant Teams.', en: 'Teams incoming webhook.' },
  },
  {
    key: 'webhook',
    icon: Webhook,
    title: 'Webhook',
    hint: {
      fr: 'N’importe quel endpoint HTTPS. Signé (HMAC-SHA256) si un secret est fourni.',
      en: 'Any HTTPS endpoint. Signed (HMAC-SHA256) when a secret is set.',
    },
  },
  {
    key: 'sms',
    icon: Smartphone,
    title: 'SMS',
    hint: {
      fr: 'Passerelle HTTP générique — les noms de champs sont configurables, aucun opérateur n’est imposé.',
      en: 'Generic HTTP gateway — field names are configurable, no operator is assumed.',
    },
  },
];

function ChannelsView({ canWrite }: { canWrite: boolean }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data: cfg } = useChannelConfig();
  const { data: catalogue = [] } = useChannelCatalogue();
  const { saveChannels, testChannel } = useAutomationMutations();

  const [form, setForm] = useState<Record<string, string>>({});
  const [toggles, setToggles] = useState<Record<string, boolean>>({});
  const [results, setResults] = useState<Partial<Record<NotifyChannel, ChannelTestResult>>>({});
  const [testingCh, setTestingCh] = useState<NotifyChannel | null>(null);

  const on = (k: NotifyChannel): boolean => {
    if (toggles[k] !== undefined) return toggles[k];
    switch (k) {
      case 'slack':
        return cfg?.slack_enabled ?? false;
      case 'teams':
        return cfg?.teams_enabled ?? false;
      case 'email':
        return cfg?.email_enabled ?? true;
      case 'webhook':
        return cfg?.webhook_enabled ?? false;
      case 'sms':
        return cfg?.sms_enabled ?? false;
      default:
        return true;
    }
  };
  const val = (k: string, fallback = '') => form[k] ?? fallback;
  const set = (k: string, v: string) => setForm((f) => ({ ...f, [k]: v }));

  const inputCls = 'w-full h-9 px-3 rounded-[9px] text-[13px] text-ink bg-transparent outline-none';
  const inputStyle = {
    border: '1px solid var(--border-strong)',
    background: 'var(--bg-elevated)',
  } as const;

  const save = async () => {
    try {
      await saveChannels.mutateAsync({
        slack_enabled: on('slack'),
        slack_webhook_url: form.slack_url || undefined,
        teams_enabled: on('teams'),
        teams_webhook_url: form.teams_url || undefined,
        email_enabled: on('email'),
        default_email: val('default_email', cfg?.default_email ?? ''),
        webhook_enabled: on('webhook'),
        webhook_url: form.webhook_url || undefined,
        webhook_secret: form.webhook_secret || undefined,
        sms_enabled: on('sms'),
        sms_gateway_url: form.sms_gateway_url || undefined,
        sms_api_key: form.sms_api_key || undefined,
        sms_sender: val('sms_sender', cfg?.sms_sender ?? ''),
        sms_recipients: val('sms_recipients', cfg?.sms_recipients ?? ''),
        sms_to_field: val('sms_to_field', cfg?.sms_to_field ?? ''),
        sms_text_field: val('sms_text_field', cfg?.sms_text_field ?? ''),
        sms_sender_field: val('sms_sender_field', cfg?.sms_sender_field ?? ''),
      });
      toast.success(tr('Canaux enregistrés', 'Channels saved'));
      // Secrets are write-only: clear them from the form so the UI never
      // pretends to hold a value it can no longer read back.
      setForm((f) => ({
        ...f,
        slack_url: '',
        teams_url: '',
        webhook_url: '',
        webhook_secret: '',
        sms_gateway_url: '',
        sms_api_key: '',
      }));
    } catch {
      toast.error(tr('Échec', 'Failed'));
    }
  };

  const runTest = async (ch: NotifyChannel) => {
    setTestingCh(ch);
    try {
      const res = await testChannel.mutateAsync(ch);
      setResults((r) => ({ ...r, [ch]: res }));
      if (res.delivered) toast.success(tr('Message de test envoyé', 'Test message delivered'));
      else toast.error(res.error || res.detail);
    } catch {
      toast.error(tr('Le test n’a pas pu s’exécuter', 'The test could not run'));
    } finally {
      setTestingCh(null);
    }
  };

  const configured = (ch: NotifyChannel) =>
    catalogue.find((c) => c.channel === ch)?.configured ?? false;

  const fields = (ch: NotifyChannel) => {
    switch (ch) {
      case 'email':
        return (
          <input
            className={inputCls}
            style={inputStyle}
            placeholder={tr('Email de repli (SOC)', 'Fallback email (SOC)')}
            value={val('default_email', cfg?.default_email ?? '')}
            onChange={(e) => set('default_email', e.target.value)}
            disabled={!canWrite}
          />
        );
      case 'slack':
        return (
          <input
            className={inputCls}
            style={inputStyle}
            placeholder={
              cfg?.has_slack
                ? tr('Configuré — coller pour remplacer', 'Configured — paste to replace')
                : 'https://hooks.slack.com/services/…'
            }
            value={form.slack_url ?? ''}
            onChange={(e) => set('slack_url', e.target.value)}
            disabled={!canWrite}
          />
        );
      case 'teams':
        return (
          <input
            className={inputCls}
            style={inputStyle}
            placeholder={
              cfg?.has_teams
                ? tr('Configuré — coller pour remplacer', 'Configured — paste to replace')
                : 'https://outlook.office.com/webhook/…'
            }
            value={form.teams_url ?? ''}
            onChange={(e) => set('teams_url', e.target.value)}
            disabled={!canWrite}
          />
        );
      case 'webhook':
        return (
          <div className="space-y-2">
            <input
              className={inputCls}
              style={inputStyle}
              placeholder={
                cfg?.has_webhook
                  ? tr('Configuré — coller pour remplacer', 'Configured — paste to replace')
                  : 'https://example.com/hooks/openrisk'
              }
              value={form.webhook_url ?? ''}
              onChange={(e) => set('webhook_url', e.target.value)}
              disabled={!canWrite}
            />
            <input
              className={inputCls}
              style={inputStyle}
              placeholder={tr('Secret de signature (optionnel)', 'Signing secret (optional)')}
              value={form.webhook_secret ?? ''}
              onChange={(e) => set('webhook_secret', e.target.value)}
              disabled={!canWrite}
            />
          </div>
        );
      case 'sms':
        return (
          <div className="space-y-2">
            <input
              className={inputCls}
              style={inputStyle}
              placeholder={
                cfg?.has_sms
                  ? tr(
                      'Passerelle configurée — coller pour remplacer',
                      'Gateway configured — paste to replace',
                    )
                  : 'https://api.operator.example/sms'
              }
              value={form.sms_gateway_url ?? ''}
              onChange={(e) => set('sms_gateway_url', e.target.value)}
              disabled={!canWrite}
            />
            <input
              className={inputCls}
              style={inputStyle}
              placeholder={tr('Clé API', 'API key')}
              value={form.sms_api_key ?? ''}
              onChange={(e) => set('sms_api_key', e.target.value)}
              disabled={!canWrite}
            />
            <input
              className={inputCls}
              style={inputStyle}
              placeholder={tr(
                'Destinataires (+237…, séparés par des virgules)',
                'Recipients (+237…, comma separated)',
              )}
              value={val('sms_recipients', cfg?.sms_recipients ?? '')}
              onChange={(e) => set('sms_recipients', e.target.value)}
              disabled={!canWrite}
            />
            <div className="grid grid-cols-3 gap-2">
              <input
                className={inputCls}
                style={inputStyle}
                placeholder={tr('champ « to »', '"to" field')}
                value={val('sms_to_field', cfg?.sms_to_field ?? '')}
                onChange={(e) => set('sms_to_field', e.target.value)}
                disabled={!canWrite}
              />
              <input
                className={inputCls}
                style={inputStyle}
                placeholder={tr('champ « message »', '"message" field')}
                value={val('sms_text_field', cfg?.sms_text_field ?? '')}
                onChange={(e) => set('sms_text_field', e.target.value)}
                disabled={!canWrite}
              />
              <input
                className={inputCls}
                style={inputStyle}
                placeholder={tr('expéditeur', 'sender')}
                value={val('sms_sender', cfg?.sms_sender ?? '')}
                onChange={(e) => set('sms_sender', e.target.value)}
                disabled={!canWrite}
              />
            </div>
          </div>
        );
      default:
        return null;
    }
  };

  return (
    <div className="max-w-[680px] space-y-3">
      <div className="text-[12.5px] text-ink-muted">
        {tr(
          'Les secrets sont en écriture seule : ils ne sont jamais réaffichés. « Tester » envoie un vrai message et vous rend l’erreur exacte s’il n’arrive pas.',
          'Secrets are write-only: they are never shown again. "Test" sends a real message and hands you the exact error if it does not land.',
        )}
      </div>

      {CHANNEL_ROWS.map(({ key, icon: Icon, title, hint }) => {
        const res = results[key];
        const isTesting = testingCh === key;
        return (
          <div
            key={key}
            className="rounded-[12px] p-3.5"
            style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
          >
            <div className="flex items-center gap-2.5 flex-wrap">
              {key !== 'in_app' ? (
                <input
                  type="checkbox"
                  checked={on(key)}
                  disabled={!canWrite}
                  onChange={(e) => setToggles((t) => ({ ...t, [key]: e.target.checked }))}
                />
              ) : (
                <span className="w-[13px]" />
              )}
              <Icon size={16} className="text-ink-soft" />
              <span className="text-[13px] font-semibold text-ink">{title}</span>
              <span
                className="h-5 px-2 rounded-full text-[10.5px] font-semibold"
                style={{
                  background: configured(key)
                    ? 'color-mix(in srgb, var(--low) 14%, transparent)'
                    : 'var(--bg-hover)',
                  color: configured(key) ? 'var(--low)' : 'var(--fg-secondary)',
                }}
              >
                {configured(key)
                  ? tr('configuré', 'configured')
                  : tr('non configuré', 'not configured')}
              </span>
              <button
                onClick={() => runTest(key)}
                disabled={isTesting}
                className="ml-auto h-8 px-2.5 rounded-[8px] inline-flex items-center gap-1.5 text-[12px] font-semibold"
                style={{ background: 'var(--bg-hover)', color: 'var(--accent-500)' }}
              >
                {isTesting ? (
                  <Loader2 size={13} className="animate-spin" />
                ) : (
                  <FlaskConical size={13} />
                )}
                {tr('Tester', 'Test')}
              </button>
            </div>
            <p className="text-[11.5px] mt-1 ml-[27px]" style={{ color: 'var(--fg-secondary)' }}>
              {pick(hint, lang)}
            </p>
            {fields(key) && <div className="mt-2.5 ml-[27px]">{fields(key)}</div>}
            {res && (
              <div
                className="mt-2.5 ml-[27px] text-[12px] flex items-start gap-1.5"
                style={{ color: res.delivered ? 'var(--low)' : 'var(--critical)' }}
              >
                {res.delivered ? (
                  <CheckCircle2 size={14} className="shrink-0 mt-0.5" />
                ) : (
                  <XCircle size={14} className="shrink-0 mt-0.5" />
                )}
                <span>
                  {res.detail}
                  {res.recipients ? ` · ${res.recipients}` : ''}
                  {res.error ? ` — ${res.error}` : ''}
                </span>
              </div>
            )}
          </div>
        );
      })}

      {canWrite && (
        <div className="flex justify-end">
          <Btn label={tr('Enregistrer', 'Save')} primary onClick={save} />
        </div>
      )}
    </div>
  );
}
