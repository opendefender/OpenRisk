// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Governance (spec §15). Four views:
//  - Audit trail: an interactive, filterable journal with before→after diffs + CSV export.
//  - Approvals: the Maker-Checker inbox — submit a request, approve/reject a step.
//  - Delegations: time-boxed grants of one user's rights to another.
//  - Workflows: the admin config of approval chains (trigger = entity_type + action).

import { useMemo, useState } from 'react';
import { toast } from 'sonner';
import {
  Scale, Search, Download, ChevronRight, ChevronDown, Plus, Trash2, Check, X,
  UserPlus, ShieldCheck, Clock, ArrowRight, FileClock,
} from 'lucide-react';
import { PageFrame, PageHeader, Btn, Card, SkeletonRows, EmptyState, Chip } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import {
  useAuditEvents, useDelegations, useApprovals, useWorkflows, useGovernanceMutations,
} from './useGovernance';
import { governanceService } from './governanceService';
import type {
  AuditEvent, AuditAction, Delegation, ApprovalRequest, ApprovalWorkflow, WorkflowStep,
} from './governanceService';

type Tab = 'audit' | 'approvals' | 'delegations' | 'workflows';

const ACTION_COLOR: Record<AuditAction, string> = {
  create: 'var(--good, #16a34a)', update: 'var(--accent)', delete: 'var(--crit, #dc2626)',
  submit: 'var(--accent)', approve: 'var(--good, #16a34a)', reject: 'var(--crit, #dc2626)',
  delegate: 'var(--med, #d97706)', revoke: 'var(--crit, #dc2626)',
  login: 'var(--text-secondary)', export: 'var(--text-secondary)',
};

const STATUS_COLOR: Record<string, string> = {
  pending: 'var(--med, #d97706)', approved: 'var(--good, #16a34a)',
  rejected: 'var(--crit, #dc2626)', cancelled: 'var(--text-secondary)',
  active: 'var(--good, #16a34a)', revoked: 'var(--crit, #dc2626)', expired: 'var(--text-secondary)',
};

function fmt(ts?: string | null): string {
  if (!ts) return '—';
  const d = new Date(ts);
  return isNaN(d.getTime()) ? '—' : d.toLocaleString();
}

export function GovernancePage() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const isAdmin = useAuthStore((s) => s.hasPermission)('*');
  const [tab, setTab] = useState<Tab>('approvals');

  const { data: approvals = [] } = useApprovals({ status: 'pending' });

  const TabBtn = ({ id, label, count }: { id: Tab; label: string; count?: number }) => (
    <button
      onClick={() => setTab(id)}
      className="h-9 px-3.5 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5"
      style={{
        background: tab === id ? 'var(--accent)' : 'transparent',
        color: tab === id ? '#fff' : 'var(--text-secondary)',
        border: tab === id ? 'none' : '1px solid var(--border-strong)',
      }}
    >
      {label}
      {typeof count === 'number' && count > 0 && <span className="mono opacity-80">{count}</span>}
    </button>
  );

  return (
    <PageFrame wide>
      <PageHeader
        title={tr('Gouvernance', 'Governance')}
        count={tr('Piste d’audit · Approbations · Délégations', 'Audit trail · Approvals · Delegations')}
      />

      <div className="flex gap-2 mb-4 flex-wrap">
        <TabBtn id="approvals" label={tr('Approbations', 'Approvals')} count={approvals.length} />
        <TabBtn id="delegations" label={tr('Délégations', 'Delegations')} />
        {isAdmin && <TabBtn id="workflows" label={tr('Workflows', 'Workflows')} />}
        {isAdmin && <TabBtn id="audit" label={tr('Piste d’audit', 'Audit trail')} />}
      </div>

      {tab === 'approvals' && <ApprovalsView />}
      {tab === 'delegations' && <DelegationsView />}
      {tab === 'workflows' && isAdmin && <WorkflowsView />}
      {tab === 'audit' && isAdmin && <AuditView />}
    </PageFrame>
  );
}

// ---------------------------------------------------------------------------
// Audit trail — interactive journal with before→after diff + CSV export.
// ---------------------------------------------------------------------------
function AuditView() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const [search, setSearch] = useState('');
  const [entityType, setEntityType] = useState('');
  const [action, setAction] = useState('');
  const filter = useMemo(
    () => ({ search: search || undefined, entity_type: entityType || undefined, action: action || undefined, limit: 100 }),
    [search, entityType, action],
  );
  const { data, isLoading } = useAuditEvents(filter);
  const events = data?.events ?? [];

  const exportCsv = async () => {
    try {
      const blob = await governanceService.exportAuditCsv(filter);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'audit-trail.csv';
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      toast.error(tr('Échec de l’export', 'Export failed'));
    }
  };

  const ACTIONS: AuditAction[] = ['create', 'update', 'delete', 'submit', 'approve', 'reject', 'delegate', 'revoke', 'export'];

  return (
    <div className="space-y-3">
      <Card style={{ padding: 12 }}>
        <div className="flex items-center gap-2 flex-wrap">
          <div className="flex items-center gap-2 flex-1 min-w-[220px] px-2.5 h-9 rounded-[9px]" style={{ border: '1px solid var(--border-strong)' }}>
            <Search size={15} style={{ color: 'var(--text-secondary)' }} />
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={tr('Rechercher (résumé, entité…)', 'Search (summary, entity…)')}
              className="bg-transparent outline-none text-[13px] flex-1"
            />
          </div>
          <select value={entityType} onChange={(e) => setEntityType(e.target.value)} className="h-9 px-2.5 rounded-[9px] text-[12.5px] bg-transparent" style={{ border: '1px solid var(--border-strong)', color: 'var(--text-secondary)' }}>
            <option value="">{tr('Toutes entités', 'All entities')}</option>
            <option value="asset">asset</option>
            <option value="compliance_control">compliance_control</option>
            <option value="delegation">delegation</option>
            <option value="approval_request">approval_request</option>
            <option value="audit_events">audit_events</option>
          </select>
          <select value={action} onChange={(e) => setAction(e.target.value)} className="h-9 px-2.5 rounded-[9px] text-[12.5px] bg-transparent" style={{ border: '1px solid var(--border-strong)', color: 'var(--text-secondary)' }}>
            <option value="">{tr('Toutes actions', 'All actions')}</option>
            {ACTIONS.map((a) => <option key={a} value={a}>{a}</option>)}
          </select>
          <Btn label={tr('Exporter CSV', 'Export CSV')} icon={Download} onClick={exportCsv} />
        </div>
      </Card>

      {isLoading && events.length === 0 ? (
        <Card style={{ padding: 12 }}><SkeletonRows rows={6} /></Card>
      ) : events.length === 0 ? (
        <EmptyState icon={FileClock} title={tr('Aucun évènement', 'No events')} sub={tr('Les mutations des entités auditées apparaîtront ici.', 'Mutations of audited entities will appear here.')} />
      ) : (
        <Card style={{ padding: 0, overflow: 'hidden' }}>
          {events.map((e) => <AuditRow key={e.id} e={e} />)}
        </Card>
      )}
    </div>
  );
}

function AuditRow({ e }: { e: AuditEvent }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const [open, setOpen] = useState(false);
  const changed = e.changed_fields ?? [];
  const hasDiff = (e.before || e.after) && (e.action === 'update' || e.action === 'create' || e.action === 'delete');

  return (
    <div style={{ borderBottom: '1px solid var(--border)' }}>
      <button onClick={() => setOpen((o) => !o)} className="w-full flex items-center gap-3 px-4 py-2.5 text-left">
        {hasDiff ? (open ? <ChevronDown size={14} /> : <ChevronRight size={14} />) : <span style={{ width: 14 }} />}
        <span className="text-[11px] font-bold uppercase px-2 py-0.5 rounded-md" style={{ color: '#fff', background: ACTION_COLOR[e.action] }}>{e.action}</span>
        <span className="text-[13px] flex-1 min-w-0 truncate">{e.summary || `${e.action} ${e.entity_type}`}</span>
        <span className="text-[12px] mono hidden md:inline" style={{ color: 'var(--text-secondary)' }}>{e.entity_type}</span>
        <span className="text-[12px] hidden lg:inline" style={{ color: 'var(--text-secondary)' }}>{e.actor_email || (e.actor_id ? e.actor_id.slice(0, 8) : tr('système', 'system'))}</span>
        <span className="text-[12px] mono" style={{ color: 'var(--text-secondary)' }}>{fmt(e.created_at)}</span>
      </button>

      {open && hasDiff && (
        <div className="px-10 pb-3 pt-1 text-[12.5px]">
          {e.ip_address && <div className="mb-2" style={{ color: 'var(--text-secondary)' }}>IP {e.ip_address}{e.user_agent ? ` · ${e.user_agent}` : ''}</div>}
          {changed.length > 0 ? (
            <div className="space-y-1">
              {changed.map((f) => (
                <div key={f} className="flex items-start gap-2 flex-wrap">
                  <span className="mono font-semibold" style={{ minWidth: 140 }}>{f}</span>
                  <span className="mono px-1.5 rounded" style={{ background: 'color-mix(in srgb, var(--crit, #dc2626) 12%, transparent)', textDecoration: 'line-through', opacity: 0.8 }}>{renderVal(e.before?.[f])}</span>
                  <ArrowRight size={12} style={{ marginTop: 3, color: 'var(--text-secondary)' }} />
                  <span className="mono px-1.5 rounded" style={{ background: 'color-mix(in srgb, var(--good, #16a34a) 14%, transparent)' }}>{renderVal(e.after?.[f])}</span>
                </div>
              ))}
            </div>
          ) : (
            <pre className="text-[11.5px] mono overflow-x-auto p-2 rounded" style={{ background: 'var(--surface-2, rgba(127,127,127,0.06))' }}>
              {JSON.stringify(e.after ?? e.before ?? {}, null, 2)}
            </pre>
          )}
        </div>
      )}
    </div>
  );
}

function renderVal(v: unknown): string {
  if (v === undefined || v === null) return '∅';
  if (typeof v === 'object') return JSON.stringify(v);
  return String(v);
}

// ---------------------------------------------------------------------------
// Approvals — the Maker-Checker inbox.
// ---------------------------------------------------------------------------
function ApprovalsView() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const [statusFilter, setStatusFilter] = useState('pending');
  const { data: requests = [], isLoading } = useApprovals({ status: statusFilter === 'all' ? undefined : statusFilter });
  const [showSubmit, setShowSubmit] = useState(false);

  const FILTERS = [
    { id: 'pending', label: tr('En attente', 'Pending') },
    { id: 'approved', label: tr('Approuvés', 'Approved') },
    { id: 'rejected', label: tr('Rejetés', 'Rejected') },
    { id: 'all', label: tr('Tous', 'All') },
  ];

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2 flex-wrap">
        {FILTERS.map((f) => <Chip key={f.id} label={f.label} active={statusFilter === f.id} onClick={() => setStatusFilter(f.id)} />)}
        <div className="flex-1" />
        <Btn label={tr('Demander une approbation', 'Request approval')} icon={Plus} primary onClick={() => setShowSubmit(true)} />
      </div>

      {isLoading && requests.length === 0 ? (
        <Card style={{ padding: 12 }}><SkeletonRows rows={4} /></Card>
      ) : requests.length === 0 ? (
        <EmptyState icon={ShieldCheck} title={tr('Rien à approuver', 'Nothing to approve')} sub={tr('Les demandes soumises via un workflow apparaissent ici.', 'Requests submitted through a workflow appear here.')} />
      ) : (
        <div className="space-y-3">{requests.map((r) => <ApprovalCard key={r.id} req={r} />)}</div>
      )}

      {showSubmit && <SubmitModal onClose={() => setShowSubmit(false)} />}
    </div>
  );
}

function ApprovalCard({ req }: { req: ApprovalRequest }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { decideApproval, cancelApproval } = useGovernanceMutations();
  const user = useAuthStore((s) => s.user);
  const [comment, setComment] = useState('');
  const [expanded, setExpanded] = useState(false);

  const decide = async (decision: 'approve' | 'reject') => {
    try {
      await decideApproval.mutateAsync({ id: req.id, input: { decision, comment: comment || undefined } });
      toast.success(decision === 'approve' ? tr('Étape approuvée', 'Step approved') : tr('Demande rejetée', 'Request rejected'));
      setComment('');
    } catch (err) {
      const e = err as { response?: { data?: { error?: string } } };
      toast.error(e.response?.data?.error || tr('Action refusée', 'Action refused'));
    }
  };
  const cancel = async () => {
    try { await cancelApproval.mutateAsync(req.id); toast.success(tr('Annulée', 'Cancelled')); }
    catch { toast.error(tr('Échec', 'Failed')); }
  };

  const isPending = req.status === 'pending';
  const isRequester = user?.id === req.requested_by;

  return (
    <Card style={{ padding: '14px 16px' }}>
      <div className="flex items-start gap-3 flex-wrap">
        <div className="flex-1 min-w-[240px]">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-[11px] font-bold uppercase px-2 py-0.5 rounded-md" style={{ color: '#fff', background: STATUS_COLOR[req.status] }}>{req.status}</span>
            <span className="text-[14px] font-semibold">{req.title}</span>
          </div>
          <div className="text-[12px] mt-1" style={{ color: 'var(--text-secondary)' }}>
            {req.workflow_name} · {req.entity_type}{req.action ? `/${req.action}` : ''} · {tr('demandé par', 'by')} {req.requested_by_email || req.requested_by.slice(0, 8)}
          </div>
          {/* Step chain */}
          <div className="flex items-center gap-1.5 mt-2 flex-wrap">
            {req.steps.map((s, i) => {
              const done = i < req.current_step || req.status === 'approved';
              const current = i === req.current_step && isPending;
              return (
                <span key={i} className="inline-flex items-center gap-1.5">
                  <span className="text-[11px] px-2 py-0.5 rounded-md" style={{
                    border: '1px solid var(--border-strong)',
                    background: done ? 'color-mix(in srgb, var(--good, #16a34a) 16%, transparent)' : current ? 'color-mix(in srgb, var(--accent) 16%, transparent)' : 'transparent',
                    fontWeight: current ? 700 : 500,
                  }}>
                    {done && <Check size={11} className="inline mr-0.5" />}
                    {s.name}{s.approver_role ? ` · ${s.approver_role}` : ''}{s.min_approvals > 1 ? ` ×${s.min_approvals}` : ''}
                  </span>
                  {i < req.steps.length - 1 && <ChevronRight size={12} style={{ color: 'var(--text-secondary)' }} />}
                </span>
              );
            })}
          </div>
        </div>
      </div>

      {isPending && (
        <div className="flex items-center gap-2 mt-3 flex-wrap">
          <input value={comment} onChange={(e) => setComment(e.target.value)} placeholder={tr('Commentaire (optionnel)', 'Comment (optional)')} className="flex-1 min-w-[160px] h-9 px-2.5 rounded-[9px] bg-transparent text-[13px]" style={{ border: '1px solid var(--border-strong)' }} />
          <Btn label={tr('Approuver', 'Approve')} icon={Check} primary onClick={() => decide('approve')} />
          <Btn label={tr('Rejeter', 'Reject')} icon={X} onClick={() => decide('reject')} />
          {isRequester && <Btn label={tr('Annuler', 'Cancel')} onClick={cancel} />}
        </div>
      )}

      {(req.decisions?.length ?? 0) > 0 && (
        <button className="text-[12px] mt-2 inline-flex items-center gap-1" style={{ color: 'var(--text-secondary)' }} onClick={() => setExpanded((x) => !x)}>
          <Clock size={12} /> {req.decisions.length} {tr('décision(s)', 'decision(s)')} {expanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
        </button>
      )}
      {expanded && (
        <div className="mt-2 space-y-1 pl-5">
          {req.decisions.map((d, i) => (
            <div key={i} className="text-[12px]" style={{ color: 'var(--text-secondary)' }}>
              <span style={{ color: d.decision === 'approve' ? 'var(--good, #16a34a)' : 'var(--crit, #dc2626)', fontWeight: 700 }}>{d.decision}</span>
              {' '}· {tr('étape', 'step')} {d.step_order + 1} · {d.approver_email || d.approver_id.slice(0, 8)} · {fmt(d.decided_at)}{d.comment ? ` — “${d.comment}”` : ''}
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

function SubmitModal({ onClose }: { onClose: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { submitApproval } = useGovernanceMutations();
  const { data: workflows = [] } = useWorkflows();
  const [entityType, setEntityType] = useState('');
  const [action, setAction] = useState('');
  const [entityId, setEntityId] = useState('');
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');

  const pick = (w: ApprovalWorkflow) => { setEntityType(w.entity_type); setAction(w.action); };

  const submit = async () => {
    if (!entityType || !title) { toast.error(tr('Type d’entité et titre requis', 'Entity type and title required')); return; }
    try {
      await submitApproval.mutateAsync({ entity_type: entityType, action: action || undefined, entity_id: entityId || undefined, title, description: description || undefined });
      toast.success(tr('Demande soumise', 'Request submitted'));
      onClose();
    } catch (err) {
      const e = err as { response?: { data?: { error?: string } } };
      toast.error(e.response?.data?.error || tr('Échec', 'Failed'));
    }
  };

  return (
    <ModalShell title={tr('Demander une approbation', 'Request approval')} onClose={onClose} onSubmit={submit} submitLabel={tr('Soumettre', 'Submit')}>
      {workflows.length > 0 && (
        <div>
          <label className="text-[12px] font-semibold">{tr('Workflow', 'Workflow')}</label>
          <div className="flex gap-1.5 flex-wrap mt-1">
            {workflows.filter((w) => w.enabled).map((w) => (
              <Chip key={w.id} label={`${w.entity_type}${w.action ? `/${w.action}` : ''}`} active={entityType === w.entity_type && action === w.action} onClick={() => pick(w)} />
            ))}
          </div>
        </div>
      )}
      <Field label={tr('Type d’entité', 'Entity type')} value={entityType} onChange={setEntityType} placeholder="risk_acceptance" />
      <Field label="Action" value={action} onChange={setAction} placeholder="accept" />
      <Field label={tr('ID de l’entité (optionnel)', 'Entity ID (optional)')} value={entityId} onChange={setEntityId} placeholder="risk-uuid" />
      <Field label={tr('Titre', 'Title')} value={title} onChange={setTitle} placeholder={tr('Accepter le risque résiduel Log4Shell', 'Accept Log4Shell residual risk')} />
      <Field label={tr('Description', 'Description')} value={description} onChange={setDescription} textarea />
    </ModalShell>
  );
}

// ---------------------------------------------------------------------------
// Delegations
// ---------------------------------------------------------------------------
function DelegationsView() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data: delegations = [], isLoading } = useDelegations();
  const { revokeDelegation } = useGovernanceMutations();
  const [showCreate, setShowCreate] = useState(false);

  const revoke = async (d: Delegation) => {
    if (!confirm(tr('Révoquer cette délégation ?', 'Revoke this delegation?'))) return;
    try { await revokeDelegation.mutateAsync(d.id); toast.success(tr('Révoquée', 'Revoked')); }
    catch { toast.error(tr('Échec', 'Failed')); }
  };

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between flex-wrap gap-2">
        <div className="text-[12.5px]" style={{ color: 'var(--text-secondary)' }}>
          {tr('Confiez temporairement vos droits à un collègue (absence, congés).', 'Temporarily hand your rights to a colleague (leave, absence).')}
        </div>
        <Btn label={tr('Nouvelle délégation', 'New delegation')} icon={UserPlus} primary onClick={() => setShowCreate(true)} />
      </div>

      {isLoading && delegations.length === 0 ? (
        <Card style={{ padding: 12 }}><SkeletonRows rows={3} /></Card>
      ) : delegations.length === 0 ? (
        <EmptyState icon={UserPlus} title={tr('Aucune délégation', 'No delegations')} sub={tr('Créez une délégation temporaire de droits.', 'Create a temporary delegation of rights.')} />
      ) : (
        <Card style={{ padding: 0, overflow: 'hidden' }}>
          {delegations.map((d) => (
            <div key={d.id} className="flex items-center gap-3 px-4 py-3 flex-wrap" style={{ borderBottom: '1px solid var(--border)' }}>
              <span className="text-[11px] font-bold uppercase px-2 py-0.5 rounded-md" style={{ color: '#fff', background: STATUS_COLOR[d.status] }}>{d.status}</span>
              <div className="flex-1 min-w-[200px]">
                <div className="text-[13px]">
                  <span className="font-semibold">{d.delegator_email || d.delegator_id.slice(0, 8)}</span>
                  <ArrowRight size={12} className="inline mx-1.5" style={{ color: 'var(--text-secondary)' }} />
                  <span className="font-semibold">{d.delegate_email || d.delegate_id.slice(0, 8)}</span>
                </div>
                <div className="text-[12px] mt-0.5" style={{ color: 'var(--text-secondary)' }}>
                  {d.permissions.join(', ')} · {fmt(d.starts_at)} → {fmt(d.ends_at)}{d.reason ? ` · ${d.reason}` : ''}
                </div>
              </div>
              {d.status === 'active' && <Btn label={tr('Révoquer', 'Revoke')} icon={Trash2} onClick={() => revoke(d)} />}
            </div>
          ))}
        </Card>
      )}

      {showCreate && <CreateDelegationModal onClose={() => setShowCreate(false)} />}
    </div>
  );
}

function CreateDelegationModal({ onClose }: { onClose: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { createDelegation } = useGovernanceMutations();
  const [delegateId, setDelegateId] = useState('');
  const [permissions, setPermissions] = useState('risks:read, risks:update');
  const [reason, setReason] = useState('');
  const [endsAt, setEndsAt] = useState('');

  const submit = async () => {
    const perms = permissions.split(',').map((p) => p.trim()).filter(Boolean);
    if (!delegateId || perms.length === 0 || !endsAt) { toast.error(tr('Délégataire, permissions et date de fin requis', 'Delegate, permissions and end date required')); return; }
    try {
      await createDelegation.mutateAsync({ delegate_id: delegateId, permissions: perms, reason: reason || undefined, ends_at: new Date(endsAt).toISOString() });
      toast.success(tr('Délégation créée', 'Delegation created'));
      onClose();
    } catch (err) {
      const e = err as { response?: { data?: { error?: string } } };
      toast.error(e.response?.data?.error || tr('Échec', 'Failed'));
    }
  };

  return (
    <ModalShell title={tr('Nouvelle délégation', 'New delegation')} onClose={onClose} onSubmit={submit} submitLabel={tr('Créer', 'Create')}>
      <Field label={tr('ID de l’utilisateur délégataire', 'Delegate user ID')} value={delegateId} onChange={setDelegateId} placeholder="user-uuid" />
      <Field label={tr('Permissions (séparées par des virgules, ou *)', 'Permissions (comma-separated, or *)')} value={permissions} onChange={setPermissions} />
      <Field label={tr('Raison', 'Reason')} value={reason} onChange={setReason} placeholder={tr('Congés annuels', 'Annual leave')} />
      <div>
        <label className="text-[12px] font-semibold">{tr('Fin de la délégation', 'Delegation ends')}</label>
        <input type="datetime-local" value={endsAt} onChange={(e) => setEndsAt(e.target.value)} className="w-full mt-1 h-9 px-2.5 rounded-[9px] bg-transparent text-[13px]" style={{ border: '1px solid var(--border-strong)' }} />
      </div>
    </ModalShell>
  );
}

// ---------------------------------------------------------------------------
// Workflows (admin config)
// ---------------------------------------------------------------------------
function WorkflowsView() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data: workflows = [], isLoading } = useWorkflows();
  const { deleteWorkflow } = useGovernanceMutations();
  const [showCreate, setShowCreate] = useState(false);

  const remove = async (w: ApprovalWorkflow) => {
    if (!confirm(tr(`Supprimer « ${w.name} » ?`, `Delete "${w.name}"?`))) return;
    try { await deleteWorkflow.mutateAsync(w.id); toast.success(tr('Supprimé', 'Deleted')); }
    catch { toast.error(tr('Échec', 'Failed')); }
  };

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between flex-wrap gap-2">
        <div className="text-[12.5px]" style={{ color: 'var(--text-secondary)' }}>
          {tr('Définissez des chaînes d’approbation (ex : accepter un risque = propriétaire + CISO).', 'Define approval chains (e.g. accept a risk = owner + CISO).')}
        </div>
        <Btn label={tr('Nouveau workflow', 'New workflow')} icon={Plus} primary onClick={() => setShowCreate(true)} />
      </div>

      {isLoading && workflows.length === 0 ? (
        <Card style={{ padding: 12 }}><SkeletonRows rows={3} /></Card>
      ) : workflows.length === 0 ? (
        <EmptyState icon={Scale} title={tr('Aucun workflow', 'No workflows')} sub={tr('Créez une chaîne Maker-Checker.', 'Create a Maker-Checker chain.')} />
      ) : (
        <div className="space-y-3">
          {workflows.map((w) => (
            <Card key={w.id} style={{ padding: '14px 16px' }}>
              <div className="flex items-start gap-3 flex-wrap">
                <div className="flex-1 min-w-[240px]">
                  <div className="flex items-center gap-2">
                    <span className="text-[14px] font-semibold">{w.name}</span>
                    {!w.enabled && <span className="text-[11px] px-2 py-0.5 rounded-md" style={{ border: '1px solid var(--border-strong)', color: 'var(--text-secondary)' }}>{tr('désactivé', 'disabled')}</span>}
                  </div>
                  <div className="text-[12px] mt-0.5 mono" style={{ color: 'var(--text-secondary)' }}>{w.entity_type}{w.action ? `/${w.action}` : ''}</div>
                  <div className="flex items-center gap-1.5 mt-2 flex-wrap">
                    {w.steps.map((s: WorkflowStep, i) => (
                      <span key={i} className="inline-flex items-center gap-1.5">
                        <span className="text-[11px] px-2 py-0.5 rounded-md" style={{ border: '1px solid var(--border-strong)' }}>
                          {s.name}{s.approver_role ? ` · ${s.approver_role}` : ''}{s.min_approvals > 1 ? ` ×${s.min_approvals}` : ''}
                        </span>
                        {i < w.steps.length - 1 && <ChevronRight size={12} style={{ color: 'var(--text-secondary)' }} />}
                      </span>
                    ))}
                  </div>
                </div>
                <Btn label={tr('Supprimer', 'Delete')} icon={Trash2} onClick={() => remove(w)} />
              </div>
            </Card>
          ))}
        </div>
      )}

      {showCreate && <CreateWorkflowModal onClose={() => setShowCreate(false)} />}
    </div>
  );
}

function CreateWorkflowModal({ onClose }: { onClose: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { createWorkflow } = useGovernanceMutations();
  const [name, setName] = useState('');
  const [entityType, setEntityType] = useState('risk_acceptance');
  const [action, setAction] = useState('accept');
  const [steps, setSteps] = useState<Array<{ name: string; approver_role: string; min_approvals: number }>>([
    { name: 'Asset owner', approver_role: 'manager', min_approvals: 1 },
    { name: 'CISO sign-off', approver_role: 'admin', min_approvals: 1 },
  ]);

  const addStep = () => setSteps((s) => [...s, { name: '', approver_role: '', min_approvals: 1 }]);
  const removeStep = (i: number) => setSteps((s) => s.filter((_, idx) => idx !== i));
  const setStep = (i: number, patch: Partial<{ name: string; approver_role: string; min_approvals: number }>) =>
    setSteps((s) => s.map((st, idx) => (idx === i ? { ...st, ...patch } : st)));

  const submit = async () => {
    const clean = steps.filter((s) => s.name.trim() || s.approver_role.trim());
    if (!name || !entityType || clean.length === 0) { toast.error(tr('Nom, entité et au moins une étape requis', 'Name, entity and at least one step required')); return; }
    try {
      await createWorkflow.mutateAsync({ name, entity_type: entityType, action: action || undefined, steps: clean });
      toast.success(tr('Workflow créé', 'Workflow created'));
      onClose();
    } catch (err) {
      const e = err as { response?: { data?: { error?: string } } };
      toast.error(e.response?.data?.error || tr('Échec', 'Failed'));
    }
  };

  return (
    <ModalShell title={tr('Nouveau workflow', 'New workflow')} onClose={onClose} onSubmit={submit} submitLabel={tr('Créer', 'Create')}>
      <Field label={tr('Nom', 'Name')} value={name} onChange={setName} placeholder={tr('Acceptation de risque', 'Risk acceptance')} />
      <div className="flex gap-2">
        <div className="flex-1"><Field label={tr('Type d’entité', 'Entity type')} value={entityType} onChange={setEntityType} /></div>
        <div className="flex-1"><Field label="Action" value={action} onChange={setAction} /></div>
      </div>
      <div>
        <div className="flex items-center justify-between">
          <label className="text-[12px] font-semibold">{tr('Étapes d’approbation', 'Approval steps')}</label>
          <button onClick={addStep} className="text-[12px] inline-flex items-center gap-1" style={{ color: 'var(--accent)' }}><Plus size={12} /> {tr('Ajouter', 'Add')}</button>
        </div>
        <div className="space-y-2 mt-1">
          {steps.map((s, i) => (
            <div key={i} className="flex items-center gap-1.5">
              <span className="mono text-[12px]" style={{ color: 'var(--text-secondary)', width: 18 }}>{i + 1}</span>
              <input value={s.name} onChange={(e) => setStep(i, { name: e.target.value })} placeholder={tr('Nom de l’étape', 'Step name')} className="flex-1 h-8 px-2 rounded-[8px] bg-transparent text-[12.5px]" style={{ border: '1px solid var(--border-strong)' }} />
              <input value={s.approver_role} onChange={(e) => setStep(i, { approver_role: e.target.value })} placeholder={tr('rôle', 'role')} className="w-24 h-8 px-2 rounded-[8px] bg-transparent text-[12.5px]" style={{ border: '1px solid var(--border-strong)' }} />
              <input type="number" min={1} value={s.min_approvals} onChange={(e) => setStep(i, { min_approvals: Math.max(1, Number(e.target.value) || 1) })} className="w-14 h-8 px-2 rounded-[8px] bg-transparent text-[12.5px]" style={{ border: '1px solid var(--border-strong)' }} />
              <button onClick={() => removeStep(i)} style={{ color: 'var(--crit, #dc2626)' }}><X size={14} /></button>
            </div>
          ))}
        </div>
      </div>
    </ModalShell>
  );
}

// ---------------------------------------------------------------------------
// Small shared bits
// ---------------------------------------------------------------------------
function Field({ label, value, onChange, placeholder, textarea }: { label: string; value: string; onChange: (v: string) => void; placeholder?: string; textarea?: boolean }) {
  return (
    <div>
      <label className="text-[12px] font-semibold">{label}</label>
      {textarea ? (
        <textarea value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} rows={3} className="w-full mt-1 px-2.5 py-2 rounded-[9px] bg-transparent text-[13px]" style={{ border: '1px solid var(--border-strong)' }} />
      ) : (
        <input value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} className="w-full mt-1 h-9 px-2.5 rounded-[9px] bg-transparent text-[13px]" style={{ border: '1px solid var(--border-strong)' }} />
      )}
    </div>
  );
}

function ModalShell({ title, onClose, onSubmit, submitLabel, children }: { title: string; onClose: () => void; onSubmit: () => void; submitLabel: string; children: React.ReactNode }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.5)' }} onClick={onClose}>
      <div className="or-scalein w-full max-w-lg flex flex-col rounded-[14px]" style={{ maxHeight: '90vh', background: 'var(--surface, #fff)', border: '1px solid var(--border-strong)' }} onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between px-5 py-3.5" style={{ borderBottom: '1px solid var(--border)' }}>
          <span className="text-[15px] font-semibold">{title}</span>
          <button onClick={onClose}><X size={18} /></button>
        </div>
        <div className="px-5 py-4 space-y-3 overflow-y-auto">{children}</div>
        <div className="flex items-center justify-end gap-2 px-5 py-3.5" style={{ borderTop: '1px solid var(--border)' }}>
          <Btn label={tr('Annuler', 'Cancel')} onClick={onClose} />
          <Btn label={submitLabel} primary onClick={onSubmit} />
        </div>
      </div>
    </div>
  );
}
