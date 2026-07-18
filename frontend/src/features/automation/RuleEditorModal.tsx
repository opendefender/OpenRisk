// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Create / edit an automation rule: trigger + conditions + an ordered action
// chain + an optional SLA policy. This is the visual workflow builder for §10.

import { useState } from 'react';
import { toast } from 'sonner';
import { X, Plus, ArrowUp, ArrowDown, Trash2, Workflow } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { Btn } from '../../shared/ui';
import { useAutomationMutations } from './useAutomation';
import {
  TRIGGER_META, ACTION_META, CHANNEL_META, pick,
} from './automationMeta';
import type {
  AutomationRule, AutomationTrigger, AutomationAction, AutomationActionType,
  NotifyChannel, RuleInput,
} from './automationService';

const TRIGGERS: AutomationTrigger[] = [
  'vulnerability_detected', 'risk_score_updated', 'risk_created', 'incident_created', 'manual',
];
const ACTIONS: AutomationActionType[] = [
  'scan_asset', 'create_risk', 'assign_owner', 'create_ticket', 'notify', 'start_sla', 'resolve_risk',
];
const CHANNELS: NotifyChannel[] = ['in_app', 'email', 'slack', 'teams'];
const SEVERITIES = ['', 'low', 'medium', 'high', 'critical'];

const lbl = 'text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1.5';
const inputCls = 'w-full h-9 px-3 rounded-[9px] text-[13px] text-ink bg-transparent outline-none';
const inputStyle = { border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' } as const;

export function RuleEditorModal({
  rule, isOpen, onClose,
}: { rule: AutomationRule | null; isOpen: boolean; onClose: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { createRule, updateRule } = useAutomationMutations();

  const [name, setName] = useState(rule?.name ?? '');
  const [description, setDescription] = useState(rule?.description ?? '');
  const [enabled, setEnabled] = useState(rule?.enabled ?? true);
  const [trigger, setTrigger] = useState<AutomationTrigger>(rule?.trigger ?? 'vulnerability_detected');
  const [minSeverity, setMinSeverity] = useState(rule?.conditions?.min_severity ?? '');
  const [minCvss, setMinCvss] = useState(rule?.conditions?.min_cvss ?? 0);
  const [kevOnly, setKevOnly] = useState(rule?.conditions?.kev_only ?? false);
  const [minTier, setMinTier] = useState(rule?.conditions?.min_priority_tier ?? '');
  const [actions, setActions] = useState<AutomationAction[]>(
    rule?.actions?.length ? rule.actions : [{ type: 'notify', channels: ['in_app'] }],
  );
  const [sla, setSla] = useState({
    critical_minutes: rule?.sla?.critical_minutes ?? 240,
    high_minutes: rule?.sla?.high_minutes ?? 480,
    medium_minutes: rule?.sla?.medium_minutes ?? 0,
    low_minutes: rule?.sla?.low_minutes ?? 0,
    escalate_after_minutes: rule?.sla?.escalate_after_minutes ?? 60,
    escalate_to_role: rule?.sla?.escalate_to_role ?? 'admin',
    escalate_channels: (rule?.sla?.escalate_channels ?? ['in_app', 'email']) as NotifyChannel[],
  });

  if (!isOpen) return null;

  const hasSLA = actions.some((a) => a.type === 'start_sla');

  const addAction = (type: AutomationActionType) =>
    setActions((prev) => [...prev, type === 'notify' ? { type, channels: ['in_app'] } : { type }]);
  const removeAction = (i: number) => setActions((prev) => prev.filter((_, idx) => idx !== i));
  const move = (i: number, dir: -1 | 1) =>
    setActions((prev) => {
      const next = [...prev];
      const j = i + dir;
      if (j < 0 || j >= next.length) return prev;
      [next[i], next[j]] = [next[j], next[i]];
      return next;
    });
  const patchAction = (i: number, patch: Partial<AutomationAction>) =>
    setActions((prev) => prev.map((a, idx) => (idx === i ? { ...a, ...patch } : a)));
  const toggleChannel = (i: number, ch: NotifyChannel) =>
    setActions((prev) =>
      prev.map((a, idx) => {
        if (idx !== i) return a;
        const set = new Set(a.channels ?? []);
        set.has(ch) ? set.delete(ch) : set.add(ch);
        return { ...a, channels: Array.from(set) };
      }),
    );

  const submit = async () => {
    if (!name.trim()) { toast.error(tr('Nom requis', 'Name required')); return; }
    if (actions.length === 0) { toast.error(tr('Au moins une action', 'At least one action')); return; }
    const input: RuleInput = {
      name: name.trim(),
      description: description.trim(),
      enabled,
      trigger,
      conditions: {
        min_severity: minSeverity || undefined,
        min_cvss: minCvss > 0 ? minCvss : undefined,
        kev_only: kevOnly || undefined,
        min_priority_tier: minTier || undefined,
      },
      actions,
      sla: hasSLA ? sla : {},
    };
    try {
      if (rule) await updateRule.mutateAsync({ id: rule.id, input });
      else await createRule.mutateAsync(input);
      toast.success(tr('Règle enregistrée', 'Rule saved'));
      onClose();
    } catch {
      toast.error(tr('Échec de l’enregistrement', 'Save failed'));
    }
  };

  return (
    <div className="fixed inset-0 z-[80] flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,.5)', backdropFilter: 'blur(3px)' }} onClick={onClose}>
      <div onClick={(e) => e.stopPropagation()} className="w-full max-w-[640px] rounded-[16px] flex flex-col or-scalein" style={{ maxHeight: '92vh', background: 'var(--bg-secondary)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)' }}>
        <div className="flex items-center justify-between px-5 py-4" style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="flex items-center gap-2 text-[15px] font-bold text-ink">
            <Workflow size={17} /> {rule ? tr('Modifier la règle', 'Edit rule') : tr('Nouvelle automatisation', 'New automation')}
          </div>
          <button onClick={onClose} className="w-8 h-8 rounded-[9px] flex items-center justify-center text-ink-soft" style={{ background: 'var(--bg-hover)' }}><X size={18} /></button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4 space-y-5">
          {/* Identity */}
          <div>
            <div className={lbl}>{tr('Nom', 'Name')}</div>
            <input className={inputCls} style={inputStyle} value={name} onChange={(e) => setName(e.target.value)} placeholder={tr('Ex. CVE critique KEV → alerte + SLA', 'e.g. Critical KEV CVE → alert + SLA')} />
          </div>
          <div>
            <div className={lbl}>{tr('Description', 'Description')}</div>
            <input className={inputCls} style={inputStyle} value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>

          {/* Trigger */}
          <div>
            <div className={lbl}>{tr('Déclencheur', 'Trigger')}</div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              {TRIGGERS.map((t) => {
                const meta = TRIGGER_META[t];
                const Icon = meta.icon;
                const active = trigger === t;
                return (
                  <button key={t} onClick={() => setTrigger(t)} className="text-left p-2.5 rounded-[10px] flex items-start gap-2"
                    style={{ border: `1px solid ${active ? 'var(--accent)' : 'var(--border-strong)'}`, background: active ? 'var(--accent-soft, rgba(90,106,207,.08))' : 'transparent' }}>
                    <Icon size={15} style={{ color: active ? 'var(--accent)' : 'var(--text-secondary)', marginTop: 1 }} />
                    <div>
                      <div className="text-[12.5px] font-semibold text-ink">{pick(meta.label, lang)}</div>
                      <div className="text-[11px] text-ink-muted">{pick(meta.hint, lang)}</div>
                    </div>
                  </button>
                );
              })}
            </div>
          </div>

          {/* Conditions */}
          <div>
            <div className={lbl}>{tr('Conditions (optionnel)', 'Conditions (optional)')}</div>
            <div className="grid grid-cols-2 gap-2">
              <label className="text-[12px] text-ink-soft">
                {tr('Sévérité min.', 'Min severity')}
                <select className={inputCls + ' mt-1'} style={inputStyle} value={minSeverity} onChange={(e) => setMinSeverity(e.target.value)}>
                  {SEVERITIES.map((s) => <option key={s} value={s}>{s || tr('— toutes —', '— any —')}</option>)}
                </select>
              </label>
              <label className="text-[12px] text-ink-soft">
                {tr('CVSS min.', 'Min CVSS')}
                <input type="number" min={0} max={10} step={0.1} className={inputCls + ' mt-1'} style={inputStyle} value={minCvss} onChange={(e) => setMinCvss(Number(e.target.value))} />
              </label>
              <label className="text-[12px] text-ink-soft">
                {tr('Tier min.', 'Min tier')}
                <select className={inputCls + ' mt-1'} style={inputStyle} value={minTier} onChange={(e) => setMinTier(e.target.value)}>
                  {['', 'P1', 'P2', 'P3', 'P4'].map((t) => <option key={t} value={t}>{t || tr('— tous —', '— any —')}</option>)}
                </select>
              </label>
              <label className="flex items-center gap-2 text-[12.5px] text-ink self-end pb-1.5">
                <input type="checkbox" checked={kevOnly} onChange={(e) => setKevOnly(e.target.checked)} />
                {tr('CISA-KEV uniquement', 'CISA-KEV only')}
              </label>
            </div>
          </div>

          {/* Action chain */}
          <div>
            <div className="flex items-center justify-between mb-1.5">
              <div className={lbl + ' mb-0'}>{tr('Chaîne d’actions', 'Action chain')}</div>
            </div>
            <div className="space-y-2">
              {actions.map((a, i) => {
                const meta = ACTION_META[a.type];
                const Icon = meta.icon;
                return (
                  <div key={i} className="rounded-[10px] p-2.5" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}>
                    <div className="flex items-center gap-2">
                      <span className="mono text-[11px] text-ink-muted w-4">{i + 1}</span>
                      <Icon size={15} style={{ color: meta.color }} />
                      <span className="text-[12.5px] font-semibold text-ink flex-1">{pick(meta.label, lang)}</span>
                      <button title={tr('Monter', 'Up')} onClick={() => move(i, -1)} className="w-6 h-6 rounded-[7px] flex items-center justify-center text-ink-soft" style={{ background: 'var(--bg-hover)' }}><ArrowUp size={13} /></button>
                      <button title={tr('Descendre', 'Down')} onClick={() => move(i, 1)} className="w-6 h-6 rounded-[7px] flex items-center justify-center text-ink-soft" style={{ background: 'var(--bg-hover)' }}><ArrowDown size={13} /></button>
                      <button title={tr('Retirer', 'Remove')} onClick={() => removeAction(i)} className="w-6 h-6 rounded-[7px] flex items-center justify-center" style={{ background: 'var(--bg-hover)', color: 'var(--critical)' }}><Trash2 size={13} /></button>
                    </div>
                    {a.type === 'notify' && (
                      <div className="mt-2 pl-6 flex flex-wrap gap-1.5 items-center">
                        {CHANNELS.map((ch) => {
                          const on = (a.channels ?? []).includes(ch);
                          return (
                            <button key={ch} onClick={() => toggleChannel(i, ch)} className="h-7 px-2.5 rounded-[7px] text-[11.5px] font-medium"
                              style={{ border: `1px solid ${on ? 'var(--accent)' : 'var(--border-strong)'}`, color: on ? 'var(--accent)' : 'var(--text-secondary)', background: on ? 'var(--accent-soft, rgba(90,106,207,.08))' : 'transparent' }}>
                              {CHANNEL_META[ch].label}
                            </button>
                          );
                        })}
                        <input className="h-7 px-2 rounded-[7px] text-[11.5px] text-ink bg-transparent outline-none ml-1" style={inputStyle} placeholder={tr('rôle/email (ex. admin)', 'role/email (e.g. admin)')} value={a.target ?? ''} onChange={(e) => patchAction(i, { target: e.target.value })} />
                      </div>
                    )}
                    {a.type === 'assign_owner' && (
                      <div className="mt-2 pl-6">
                        <input className={inputCls} style={inputStyle} placeholder={tr('cible (rôle admin, email, ou id)', 'target (role admin, email, or id)')} value={a.target ?? ''} onChange={(e) => patchAction(i, { target: e.target.value })} />
                      </div>
                    )}
                    {a.type === 'create_ticket' && (
                      <div className="mt-2 pl-6">
                        <select className={inputCls} style={inputStyle} value={a.ticket_provider ?? ''} onChange={(e) => patchAction(i, { ticket_provider: e.target.value })}>
                          <option value="">{tr('Provider par défaut du tenant', 'Tenant default provider')}</option>
                          <option value="jira">Jira</option>
                          <option value="servicenow">ServiceNow</option>
                        </select>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
            <div className="flex flex-wrap gap-1.5 mt-2">
              {ACTIONS.filter((t) => !(t === 'start_sla' && hasSLA)).map((t) => {
                const meta = ACTION_META[t];
                const Icon = meta.icon;
                return (
                  <button key={t} onClick={() => addAction(t)} className="h-7 px-2.5 rounded-[7px] text-[11.5px] inline-flex items-center gap-1.5" style={{ border: '1px dashed var(--border-strong)', color: 'var(--text-secondary)' }}>
                    <Plus size={12} /> <Icon size={12} style={{ color: meta.color }} /> {pick(meta.label, lang)}
                  </button>
                );
              })}
            </div>
          </div>

          {/* SLA policy — only when a start_sla action is present */}
          {hasSLA && (
            <div className="rounded-[12px] p-3.5" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}>
              <div className={lbl}>{tr('Politique SLA (minutes de résolution)', 'SLA policy (resolution minutes)')}</div>
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                {(['critical', 'high', 'medium', 'low'] as const).map((sev) => (
                  <label key={sev} className="text-[11.5px] text-ink-soft capitalize">
                    {sev}
                    <input type="number" min={0} className={inputCls + ' mt-1'} style={inputStyle}
                      value={sla[`${sev}_minutes` as const]}
                      onChange={(e) => setSla((s) => ({ ...s, [`${sev}_minutes`]: Number(e.target.value) }))} />
                  </label>
                ))}
              </div>
              <div className="grid grid-cols-2 gap-2 mt-2.5">
                <label className="text-[11.5px] text-ink-soft">
                  {tr('Escalade après (min. de dépassement)', 'Escalate after (min past due)')}
                  <input type="number" min={0} className={inputCls + ' mt-1'} style={inputStyle} value={sla.escalate_after_minutes} onChange={(e) => setSla((s) => ({ ...s, escalate_after_minutes: Number(e.target.value) }))} />
                </label>
                <label className="text-[11.5px] text-ink-soft">
                  {tr('Escalader vers', 'Escalate to')}
                  <select className={inputCls + ' mt-1'} style={inputStyle} value={sla.escalate_to_role} onChange={(e) => setSla((s) => ({ ...s, escalate_to_role: e.target.value }))}>
                    <option value="admin">{tr('Managers / Admins', 'Managers / Admins')}</option>
                    <option value="root">Root</option>
                  </select>
                </label>
              </div>
            </div>
          )}
        </div>

        <div className="flex items-center justify-between px-5 py-3.5" style={{ borderTop: '1px solid var(--border)' }}>
          <label className="flex items-center gap-2 text-[12.5px] text-ink">
            <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
            {tr('Activée', 'Enabled')}
          </label>
          <div className="flex items-center gap-2">
            <Btn label={tr('Annuler', 'Cancel')} onClick={onClose} />
            <Btn label={tr('Enregistrer', 'Save')} primary onClick={submit} />
          </div>
        </div>
      </div>
    </div>
  );
}
