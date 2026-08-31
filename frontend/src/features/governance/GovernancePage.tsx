// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Governance (spec §15). Four views:
//  - Audit trail: an interactive, filterable journal with before→after diffs + CSV export.
//  - Approvals: the Maker-Checker inbox — submit a request, approve/reject a step.
//  - Delegations: time-boxed grants of one user's rights to another.
//  - Workflows: the admin config of approval chains (trigger = entity_type + action).

import { useMemo, useState } from 'react';
import { useLocation, useSearchParams } from 'react-router';
import { toast } from 'sonner';
import {
  Scale, Download, ChevronRight, ChevronDown, Plus, Trash2, Check, X,
  UserPlus, ShieldCheck, Clock, ArrowRight, FileClock,
} from 'lucide-react';
import { PageFrame, PageHeader, Btn, Card, SkeletonRows, EmptyState, Chip } from '../../shared/ui';
import { DataTable, useTableState, type Column, type Facet, type RowAction } from '../../shared/datatable';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import { AccessDenied } from '../../shared/AccessDenied';
import {
  useAuditEvents, useDelegations, useApprovals, useApprovalDetail, useRequestTypes, useWorkflows, useGovernanceMutations,
} from './useGovernance';
import { AuditIntegrityPanel } from './AuditIntegrityPanel';
import { governanceService } from './governanceService';
import type {
  AuditEvent, AuditAction, Delegation, ApprovalRequest, ApprovalWorkflow, WorkflowStep,
} from './governanceService';

type Tab = 'audit' | 'approvals' | 'delegations' | 'workflows';

// Facet vocabularies for the audit trail. They mirror the values the API filters
// on; a value listed here that the API does not know would simply return zero
// rows, which is why they are kept next to the query rather than inlined in JSX.
const AUDIT_ACTIONS: AuditAction[] = ['create', 'update', 'delete', 'submit', 'approve', 'reject', 'delegate', 'revoke', 'export'];
const AUDIT_ENTITIES = ['asset', 'compliance_control', 'delegation', 'approval_request', 'audit_events'];

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

  // /governance/audit-trail must open the AUDIT TRAIL.
  //
  // It did not: the route rendered this component, which always started on
  // Approvals, so the URL — and the two legacy redirects that target it,
  // /audit-logs and /settings/audit-log — silently landed somewhere else. A
  // link that names one screen and opens another is a dead end that does not
  // look like one: the page renders fine, it is simply not the page asked for
  // (W0-05 / D13).
  const { pathname } = useLocation();
  const [params, setParams] = useSearchParams();
  const requestedAudit = pathname.endsWith('/audit-trail') || params.get('tab') === 'audit';
  const [tab, setTab] = useState<Tab>(() => {
    if (requestedAudit) return 'audit';
    const p = params.get('tab');
    return p === 'delegations' || p === 'workflows' ? p : 'approvals';
  });

  const { data: approvals = [] } = useApprovals({ status: 'pending' });

  const selectTab = (id: Tab) => {
    setTab(id);
    // Keep the URL honest about which screen is showing, so a reload or a copied
    // link reopens the same one.
    const next = new URLSearchParams(params);
    if (id === 'approvals') next.delete('tab');
    else next.set('tab', id);
    setParams(next, { replace: true });
  };

  const TabBtn = ({ id, label, count }: { id: Tab; label: string; count?: number }) => (
    <button
      onClick={() => selectTab(id)}
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
        {/* Shown to a non-admin only when they asked for it by URL — so the tab
            they landed on is visibly the one they selected, rather than the page
            quietly choosing another. */}
        {(isAdmin || tab === 'audit') && <TabBtn id="audit" label={tr('Piste d’audit', 'Audit trail')} />}
      </div>

      {tab === 'approvals' && <ApprovalsView />}
      {tab === 'delegations' && <DelegationsView />}
      {tab === 'workflows' && isAdmin && <WorkflowsView />}
      {tab === 'audit' && isAdmin && <AuditView isAdmin={isAdmin} />}
      {/* The audit trail is admin-only server-side (/governance/audit-events
          answers 403). Rendering nothing left a non-admin who followed the URL
          looking at the Approvals empty state — "nothing to approve" — which
          reads as "there is no data" when the truth is "you may not see it".
          Those two call for different actions from whoever is reading, so they
          must not look the same (W0-05 / D14). */}
      {tab === 'audit' && !isAdmin && (
        <AccessDenied permission="governance:audit:read" pathname={pathname} />
      )}
    </PageFrame>
  );
}

// ---------------------------------------------------------------------------
// Audit trail — interactive journal with before→after diff + CSV export.
// ---------------------------------------------------------------------------
// The audit trail is the one table where "what changed" matters as much as
// "what happened", so the Before → After diff moved from an inline expander into
// a drawer: <DataTable> owns the row, the drawer owns the detail. Server-side
// paging (limit/offset), facets and search all round-trip to the API.
function AuditView({ isAdmin }: { isAdmin: boolean }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const [detail, setDetail] = useState<AuditEvent | null>(null);
  const [exporting, setExporting] = useState(false);

  const table = useTableState({ defaultPageSize: 50 });
  const { state } = table;

  const filter = useMemo(() => ({
    search: state.q.trim() || undefined,
    entity_type: state.filters.entity_type?.[0],
    action: state.filters.action?.[0],
    limit: state.pageSize,
    offset: (state.page - 1) * state.pageSize,
  }), [state]);

  const { data, isLoading, isError, refetch } = useAuditEvents(filter);
  const events = useMemo(() => data?.events ?? [], [data]);

  const exportCsv = async () => {
    setExporting(true);
    try {
      const blob = await governanceService.exportAuditCsv(filter);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'audit-trail.csv';
      a.click();
      URL.revokeObjectURL(url);
      toast.success(tr('Piste d’audit exportée', 'Audit trail exported'));
    } catch {
      toast.error(tr('Échec de l’export', 'Export failed'));
    } finally {
      setExporting(false);
    }
  };

  const facets: Facet<AuditEvent>[] = useMemo(() => [
    {
      key: 'action',
      label: tr('Action', 'Action'),
      single: true,
      options: AUDIT_ACTIONS.map((a) => ({ value: a, label: a, color: ACTION_COLOR[a] })),
    },
    {
      key: 'entity_type',
      label: tr('Entité', 'Entity'),
      single: true,
      options: AUDIT_ENTITIES.map((e) => ({ value: e, label: e })),
    },
  ], [lang]); // eslint-disable-line react-hooks/exhaustive-deps

  const columns: Column<AuditEvent>[] = useMemo(() => [
    {
      key: 'action',
      header: tr('Action', 'Action'),
      hideable: false,
      exportValue: (e) => e.action,
      render: (e) => (
        <span className="text-[11px] font-bold uppercase px-2 py-0.5 rounded-md" style={{ color: 'var(--text-inverse)', background: ACTION_COLOR[e.action] }}>{e.action}</span>
      ),
    },
    {
      key: 'summary',
      header: tr('Résumé', 'Summary'),
      frozen: true,
      exportValue: (e) => e.summary || `${e.action} ${e.entity_type}`,
      render: (e) => <span className="text-[13px] block max-w-[420px] truncate">{e.summary || `${e.action} ${e.entity_type}`}</span>,
    },
    { key: 'entity', header: tr('Entité', 'Entity'), exportValue: (e) => e.entity_type, render: (e) => <span className="text-[12px] mono" style={{ color: 'var(--text-secondary)' }}>{e.entity_type}</span> },
    {
      key: 'actor',
      header: tr('Acteur', 'Actor'),
      exportValue: (e) => e.actor_email || e.actor_id || 'system',
      render: (e) => <span className="text-[12px]" style={{ color: 'var(--text-secondary)' }}>{e.actor_email || (e.actor_id ? e.actor_id.slice(0, 8) : tr('système', 'system'))}</span>,
    },
    { key: 'ip', header: 'IP', defaultHidden: true, exportValue: (e) => e.ip_address ?? '', render: (e) => <span className="text-[12px] mono" style={{ color: 'var(--text-secondary)' }}>{e.ip_address ?? '—'}</span> },
    {
      key: 'seq',
      header: '#',
      defaultHidden: true,
      exportValue: (e) => String(e.sequence),
      // The position in the chain. Hidden by default — it matters when you are
      // reconciling an export, not when you are reading the day's activity.
      render: (e) => <span className="text-[12px] mono" style={{ color: 'var(--text-secondary)' }}>{e.sequence}</span>,
    },
    {
      key: 'source',
      header: tr('Origine', 'Source'),
      defaultHidden: true,
      exportValue: (e) => e.source ?? '',
      render: (e) => <span className="text-[12px] mono" style={{ color: 'var(--text-secondary)' }}>{e.source ?? '—'}</span>,
    },
    { key: 'when', header: tr('Quand', 'When'), exportValue: (e) => e.created_at, render: (e) => <span className="text-[12px] mono" style={{ color: 'var(--text-secondary)' }}>{fmt(e.created_at)}</span> },
  ], [lang]); // eslint-disable-line react-hooks/exhaustive-deps

  const rowActions: RowAction<AuditEvent>[] = useMemo(() => [
    { key: 'diff', label: tr('Voir le détail', 'View details'), icon: FileClock, onSelect: (e) => setDetail(e) },
  ], [lang]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <>
      {/* Integrity, retention and signed export sit above the list: what makes
          this a piece of evidence rather than a log. */}
      <AuditIntegrityPanel filter={filter} isAdmin={isAdmin} />

      <DataTable
        id="audit-trail"
        ariaLabel={tr('Piste d’audit', 'Audit trail')}
        rows={events}
        total={data?.total ?? events.length}
        columns={columns}
        rowKey={(e) => e.id}
        api={table}
        mode="server"
        loading={isLoading}
        error={isError}
        onRetry={() => void refetch()}
        facets={facets}
        searchPlaceholder={tr('Rechercher (résumé, entité…)', 'Search (summary, entity…)')}
        rowActions={rowActions}
        onRowClick={(e) => setDetail(e)}
        toolbarExtra={<Btn label={exporting ? tr('Export…', 'Exporting…') : tr('Exporter CSV', 'Export CSV')} icon={Download} onClick={exportCsv} disabled={exporting} />}
        minWidth={900}
        empty={
          <EmptyState
            variant="first-use"
            icon={FileClock}
            title={tr('La piste d’audit est vide', 'The audit trail is empty')}
            description={tr('Chaque création, modification et suppression d’une entité auditée est enregistrée ici — qui, quoi, quand, et le détail Avant → Après. Elle se remplit dès votre première action.', 'Every create, update and delete on an audited entity is recorded here — who, what, when, and the before/after diff. It fills up with your first action.')}
          />
        }
      />
      {detail && <AuditDetailDrawer e={detail} onClose={() => setDetail(null)} />}
    </>
  );
}

// Before → After, field by field.
function AuditDetailDrawer({ e, onClose }: { e: AuditEvent; onClose: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const changed = e.changed_fields ?? [];

  return (
    <div className="fixed inset-0 z-70 flex justify-end" style={{ background: 'rgba(0,0,0,.45)', backdropFilter: 'blur(3px)' }} onClick={onClose}>
      <div
        onClick={(ev) => ev.stopPropagation()}
        className="h-full flex flex-col"
        style={{ width: 'min(94vw,560px)', background: 'var(--bg-secondary)', borderLeft: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)', animation: 'or-slidein .3s cubic-bezier(.2,.8,.2,1)' }}
      >
        <div className="px-[22px] pt-5 pb-3.5 flex items-start gap-3" style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="flex-1">
            <span className="text-[11px] font-bold uppercase px-2 py-0.5 rounded-md" style={{ color: 'var(--text-inverse)', background: ACTION_COLOR[e.action] }}>{e.action}</span>
            <div className="disp text-[16px] font-bold text-ink leading-snug mt-2">{e.summary || `${e.action} ${e.entity_type}`}</div>
            <div className="text-[12px] mt-1" style={{ color: 'var(--text-secondary)' }}>
              {e.entity_type} · {e.actor_email || (e.actor_id ? e.actor_id.slice(0, 8) : tr('système', 'system'))} · {fmt(e.created_at)}
            </div>
            {e.ip_address && <div className="text-[12px] mt-1" style={{ color: 'var(--text-secondary)' }}>IP {e.ip_address}{e.user_agent ? ` · ${e.user_agent}` : ''}</div>}
          </div>
          <button onClick={onClose} aria-label={tr('Fermer', 'Close')} className="w-8 h-8 rounded-[9px] flex items-center justify-center shrink-0 text-ink-soft" style={{ background: 'var(--bg-hover)' }}>
            <X size={18} />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-[22px] py-5 text-[12.5px]">
          {changed.length > 0 ? (
            <div className="space-y-2">
              {changed.map((f) => (
                <div key={f} className="flex items-start gap-2 flex-wrap">
                  <span className="mono font-semibold" style={{ minWidth: 140 }}>{f}</span>
                  <span className="mono px-1.5 rounded" style={{ background: 'color-mix(in srgb, var(--critical) 12%, transparent)', textDecoration: 'line-through', opacity: 0.8 }}>{renderVal(e.before?.[f])}</span>
                  <ArrowRight size={12} style={{ marginTop: 3, color: 'var(--text-secondary)' }} />
                  <span className="mono px-1.5 rounded" style={{ background: 'color-mix(in srgb, var(--low) 14%, transparent)' }}>{renderVal(e.after?.[f])}</span>
                </div>
              ))}
            </div>
          ) : (
            <pre className="text-[11.5px] mono overflow-x-auto p-2 rounded" style={{ background: 'var(--bg-hover)' }}>
              {JSON.stringify(e.after ?? e.before ?? {}, null, 2)}
            </pre>
          )}
        </div>
      </div>
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
    { id: 'expired', label: tr('Expirés', 'Expired') },
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
        <EmptyState
          variant="first-use"
          icon={ShieldCheck}
          title={tr('Rien à approuver', 'Nothing to approve')}
          description={tr('Les demandes soumises via un workflow Maker-Checker atterrissent ici pour validation. Vous ne pouvez jamais approuver vos propres demandes.', 'Requests submitted through a Maker-Checker workflow land here for sign-off. You can never approve your own request.')}
        />
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
  const [stepOrder, setStepOrder] = useState<number | null>(null);

  // The detail view answers "may I sign this, and why not" before the click,
  // instead of turning the question into a 403 afterwards.
  const { data: detail } = useApprovalDetail(req.status === 'pending' ? req.id : null);
  const progress = detail?.progress ?? [];
  const openSteps = progress.filter((p) => p.open);
  const parallel = req.mode === 'parallel';

  const decide = async (decision: 'approve' | 'reject') => {
    // A refusal without a reason leaves the requester knowing only that they
    // failed. The server refuses it too; catching it here saves a round trip
    // and says so next to the field.
    if (decision === 'reject' && !comment.trim()) {
      toast.error(tr('Un commentaire est obligatoire pour refuser — dites ce qui devrait changer.',
                     'A comment is required to refuse — say what would have to change.'));
      return;
    }
    try {
      await decideApproval.mutateAsync({
        id: req.id,
        input: {
          decision,
          comment: comment.trim() || undefined,
          step_order: parallel && stepOrder !== null ? stepOrder : undefined,
        },
      });
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
  const canDecide = detail?.can_decide ?? false;

  // Deadlines are a promise; show how much of it is left.
  const hoursLeft = req.expires_at
    ? Math.round((new Date(req.expires_at).getTime() - Date.now()) / 3_600_000)
    : null;

  return (
    <Card style={{ padding: '14px 16px' }}>
      <div className="flex items-start gap-3 flex-wrap">
        <div className="flex-1 min-w-[240px]">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-[11px] font-bold uppercase px-2 py-0.5 rounded-md" style={{ color: '#fff', background: STATUS_COLOR[req.status] }}>{req.status}</span>
            <span className="text-[14px] font-semibold">{req.title}</span>
            {parallel && (
              <span className="text-[10.5px] font-semibold px-1.5 py-0.5 rounded"
                style={{ background: 'var(--bg-hover)', color: 'var(--text-secondary)' }}>
                {tr('parallèle', 'parallel')}
              </span>
            )}
            {isPending && hoursLeft !== null && (
              <span className="text-[10.5px] font-semibold px-1.5 py-0.5 rounded"
                style={{
                  background: `color-mix(in srgb, ${hoursLeft <= 12 ? 'var(--critical)' : 'var(--medium)'} 14%, transparent)`,
                  color: hoursLeft <= 12 ? 'var(--critical)' : 'var(--medium)',
                }}>
                {hoursLeft > 0
                  ? tr(`expire dans ${hoursLeft} h`, `expires in ${hoursLeft}h`)
                  : tr('expiré', 'expired')}
              </span>
            )}
          </div>
          <div className="text-[12px] mt-1" style={{ color: 'var(--text-secondary)' }}>
            {req.workflow_name} · {detail?.request_type_info
              ? (lang === 'fr' ? detail.request_type_info.label : detail.request_type_info.label_en)
              : `${req.entity_type}${req.action ? `/${req.action}` : ''}`}
            {' · '}{tr('demandé par', 'by')} {req.requested_by_email || req.requested_by.slice(0, 8)}
          </div>

          {/* Step chain, with each step's real quorum standing. */}
          <div className="flex items-center gap-1.5 mt-2 flex-wrap">
            {(progress.length > 0 ? progress : req.steps.map((st, i) => ({
              order: i, name: st.name, approver_role: st.approver_role,
              required_approvals: st.min_approvals, approvals: 0,
              satisfied: i < req.current_step, rejected: false,
              open: i === req.current_step && isPending, approvers: [],
            }))).map((p, i, arr) => (
              <span key={p.order} className="inline-flex items-center gap-1.5">
                <span className="text-[11px] px-2 py-0.5 rounded-md" style={{
                  border: '1px solid var(--border-strong)',
                  background: p.satisfied
                    ? 'color-mix(in srgb, var(--good, #16a34a) 16%, transparent)'
                    : p.open ? 'color-mix(in srgb, var(--accent) 16%, transparent)' : 'transparent',
                  fontWeight: p.open ? 700 : 500,
                }}
                title={p.approvers && p.approvers.length > 0 ? p.approvers.join(', ') : undefined}>
                  {p.satisfied && <Check size={11} className="inline mr-0.5" />}
                  {p.name}{p.approver_role ? ` · ${p.approver_role}` : ''}
                  {p.required_approvals > 1 ? ` ${p.approvals}/${p.required_approvals}` : ''}
                </span>
                {!parallel && i < arr.length - 1 && <ChevronRight size={12} style={{ color: 'var(--text-secondary)' }} />}
              </span>
            ))}
          </div>
        </div>
      </div>

      {isPending && (
        <div className="mt-3">
          {/* Why the buttons are (or are not) available — stated, not implied. */}
          {detail && (
            <p className="text-[12px] mb-2" style={{ color: canDecide ? 'var(--text-secondary)' : 'var(--medium)' }}>
              {detail.verdict.reason}
              {detail.verdict.via_delegation && ` (${tr('par délégation', 'by delegation')})`}
            </p>
          )}
          <div className="flex items-center gap-2 flex-wrap">
            {parallel && openSteps.length > 1 && (
              <select value={stepOrder ?? openSteps[0]?.order ?? 0}
                onChange={(e) => setStepOrder(Number(e.target.value))}
                className="h-9 px-2 rounded-[9px] text-[13px]"
                style={{ border: '1px solid var(--border-strong)', background: 'var(--bg)', color: 'var(--text-primary)' }}>
                {openSteps.map((p) => <option key={p.order} value={p.order}>{p.name}</option>)}
              </select>
            )}
            <input value={comment} onChange={(e) => setComment(e.target.value)}
              placeholder={tr('Commentaire — obligatoire pour refuser', 'Comment — required to refuse')}
              className="flex-1 min-w-[160px] h-9 px-2.5 rounded-[9px] bg-transparent text-[13px]"
              style={{ border: '1px solid var(--border-strong)' }} />
            <Btn label={tr('Approuver', 'Approve')} icon={Check} primary onClick={() => decide('approve')} disabled={!canDecide} />
            <Btn label={tr('Rejeter', 'Reject')} icon={X} onClick={() => decide('reject')} disabled={!canDecide} />
            {isRequester && <Btn label={tr('Annuler', 'Cancel')} onClick={cancel} />}
          </div>
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
        <EmptyState
          variant="first-use"
          icon={UserPlus}
          title={tr('Aucune délégation', 'No delegations')}
          description={tr('Une délégation confie temporairement vos droits à un collègue — pendant un congé, par exemple. Elle expire d’elle-même à la date de fin.', 'A delegation temporarily hands your rights to a colleague — during leave, for instance. It expires on its own end date.')}
        />
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
        <EmptyState
          variant="first-use"
          icon={Scale}
          title={tr('Aucun workflow', 'No workflows')}
          description={tr('Un workflow impose une chaîne d’approbations sur une action sensible (accepter un risque, par exemple) : à chaque étape, un rôle habilité doit signer.', 'A workflow enforces an approval chain over a sensitive action (accepting a risk, say): at each step an eligible role must sign off.')}
        />
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
                  <div className="flex items-center gap-2 mt-0.5 flex-wrap">
                    <span className="text-[12px] mono" style={{ color: 'var(--text-secondary)' }}>{w.entity_type}{w.action ? `/${w.action}` : ''}</span>
                    <span className="text-[10.5px] px-1.5 py-0.5 rounded" style={{ background: 'var(--bg-hover)', color: 'var(--text-secondary)' }}>
                      {w.mode === 'parallel' ? tr('parallèle', 'parallel') : tr('séquentiel', 'sequential')}
                    </span>
                    {w.expires_in_hours > 0 && (
                      <span className="text-[10.5px] px-1.5 py-0.5 rounded" style={{ background: 'var(--bg-hover)', color: 'var(--text-secondary)' }}>
                        {tr(`expire après ${w.expires_in_hours} h`, `expires after ${w.expires_in_hours}h`)}
                      </span>
                    )}
                  </div>
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
  const { data: requestTypes = [] } = useRequestTypes();
  const [name, setName] = useState('');
  // The catalogue is the shared vocabulary; picking a type fills in the
  // (entity_type, action) pair the submit path matches on, so a workflow cannot
  // end up bound to a pair nothing ever submits.
  const [requestType, setRequestType] = useState('risk_acceptance');
  const [mode, setMode] = useState<'sequential' | 'parallel'>('sequential');
  const [expiresInHours, setExpiresInHours] = useState(0);
  const [steps, setSteps] = useState<Array<{ name: string; approver_role: string; min_approvals: number; quorum_percent?: number }>>([
    { name: 'Asset owner', approver_role: 'manager', min_approvals: 1 },
    { name: 'CISO sign-off', approver_role: 'admin', min_approvals: 1 },
  ]);
  const selectedType = requestTypes.find((t) => t.key === requestType);

  const addStep = () => setSteps((s) => [...s, { name: '', approver_role: '', min_approvals: 1 }]);
  const removeStep = (i: number) => setSteps((s) => s.filter((_, idx) => idx !== i));
  const setStep = (i: number, patch: Partial<{ name: string; approver_role: string; min_approvals: number; quorum_percent: number }>) =>
    setSteps((s) => s.map((st, idx) => (idx === i ? { ...st, ...patch } : st)));

  const submit = async () => {
    const clean = steps.filter((s) => s.name.trim() || s.approver_role.trim());
    if (!name || !requestType || clean.length === 0) { toast.error(tr('Nom, type de demande et au moins une étape requis', 'Name, request type and at least one step required')); return; }
    try {
      await createWorkflow.mutateAsync({
        name,
        request_type: requestType,
        entity_type: selectedType?.entity_type ?? requestType,
        action: selectedType?.action,
        mode,
        expires_in_hours: expiresInHours,
        steps: clean,
      });
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
      <div>
        <label className="text-[12px] font-semibold">{tr('Type de demande', 'Request type')}</label>
        <select value={requestType} onChange={(e) => setRequestType(e.target.value)}
          className="w-full mt-1 h-9 px-2.5 rounded-[9px] bg-transparent text-[13px]" style={{ border: '1px solid var(--border-strong)' }}>
          {requestTypes.map((t) => (
            <option key={t.key} value={t.key}>{lang === 'fr' ? t.label : t.label_en}</option>
          ))}
        </select>
        {selectedType && (
          <p className="text-[11.5px] mt-1" style={{ color: 'var(--text-secondary)' }}>
            {selectedType.description}
            {selectedType.linked_to_lifecycle && (
              <>
                {' '}
                <strong style={{ color: 'var(--medium)' }}>{selectedType.linked_to_lifecycle}</strong>
              </>
            )}
          </p>
        )}
      </div>

      <div className="flex gap-2">
        <div className="flex-1">
          <label className="text-[12px] font-semibold">{tr('Déroulement', 'Flow')}</label>
          <select value={mode} onChange={(e) => setMode(e.target.value as 'sequential' | 'parallel')}
            className="w-full mt-1 h-9 px-2.5 rounded-[9px] bg-transparent text-[13px]" style={{ border: '1px solid var(--border-strong)' }}>
            <option value="sequential">{tr('Séquentiel — une étape après l’autre', 'Sequential — one step after another')}</option>
            <option value="parallel">{tr('Parallèle — toutes les étapes en même temps', 'Parallel — every step at once')}</option>
          </select>
        </div>
        <div className="w-[170px]">
          <label className="text-[12px] font-semibold">{tr('Expire après (h)', 'Expires after (h)')}</label>
          <input type="number" min={0} value={expiresInHours}
            onChange={(e) => setExpiresInHours(Math.max(0, Number(e.target.value) || 0))}
            className="w-full mt-1 h-9 px-2.5 rounded-[9px] bg-transparent text-[13px] mono" style={{ border: '1px solid var(--border-strong)' }} />
        </div>
      </div>
      <p className="text-[11.5px] -mt-2" style={{ color: 'var(--text-secondary)' }}>
        {tr('0 = n’expire jamais. Sinon, une demande non décidée dans le délai se clôt comme « expirée » — ce qui n’est pas un refus.',
            '0 = never expires. Otherwise a request nobody decides in time closes as "expired" — which is not a refusal.')}
      </p>
      <div>
        <div className="flex items-center justify-between">
          <label className="text-[12px] font-semibold">{tr('Étapes d’approbation', 'Approval steps')}</label>
          <button onClick={addStep} className="text-[12px] inline-flex items-center gap-1" style={{ color: 'var(--accent-500)' }}><Plus size={12} /> {tr('Ajouter', 'Add')}</button>
        </div>
        <div className="space-y-2 mt-1">
          {steps.map((s, i) => (
            <div key={i} className="flex items-center gap-1.5">
              <span className="mono text-[12px]" style={{ color: 'var(--text-secondary)', width: 18 }}>{i + 1}</span>
              <input value={s.name} onChange={(e) => setStep(i, { name: e.target.value })} placeholder={tr('Nom de l’étape', 'Step name')} className="flex-1 h-8 px-2 rounded-[8px] bg-transparent text-[12.5px]" style={{ border: '1px solid var(--border-strong)' }} />
              <input value={s.approver_role} onChange={(e) => setStep(i, { approver_role: e.target.value })} placeholder={tr('rôle', 'role')} className="w-24 h-8 px-2 rounded-[8px] bg-transparent text-[12.5px]" style={{ border: '1px solid var(--border-strong)' }} />
              <input type="number" min={1} value={s.min_approvals}
                title={tr('Nombre de signatures requises', 'Signatures required')}
                onChange={(e) => setStep(i, { min_approvals: Math.max(1, Number(e.target.value) || 1) })}
                className="w-14 h-8 px-2 rounded-[8px] bg-transparent text-[12.5px]" style={{ border: '1px solid var(--border-strong)' }} />
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
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'var(--surface-overlay)' }} onClick={onClose}>
      <div className="or-scalein w-full max-w-lg flex flex-col rounded-[14px]" style={{ maxHeight: '90vh', background: 'var(--surface-2)', border: '1px solid var(--border-strong)' }} onClick={(e) => e.stopPropagation()}>
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
