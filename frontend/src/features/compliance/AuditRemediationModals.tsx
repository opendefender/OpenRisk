// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Dialogs for compliance audits and remediation plans, in the dc.html design
// language. Reuse the shared modal primitives from ComplianceModals.tsx.

import { useState } from 'react';
import { CalendarClock, Wrench } from 'lucide-react';
import { toast } from 'sonner';
import { useUIStore } from '../../store/uiStore';
import { ModalShell, Field, SelectField, TextArea, FooterButtons } from './ComplianceModals';
import { useAudits, useFrameworks, useRemediations } from './useCompliance';
import type { AuditType, RemediationPriority } from '../../types/compliance';

function useTr() {
  const lang = useUIStore((s) => s.lang);
  return (fr: string, en: string) => (lang === 'fr' ? fr : en);
}

function errMsg(err: unknown, fallback: string): string {
  const e = err as { response?: { data?: { error?: string } } };
  return e?.response?.data?.error || fallback;
}

/* ------------------------------------------------------------------ */
/* Schedule an audit                                                   */
/* ------------------------------------------------------------------ */

export function CreateAuditDialog({ onClose, onCreated }: { onClose: () => void; onCreated?: (id: string) => void }) {
  const tr = useTr();
  const { createAudit } = useAudits();
  const { frameworks } = useFrameworks();
  const [title, setTitle] = useState('');
  const [type, setType] = useState<AuditType>('internal');
  const [frameworkId, setFrameworkId] = useState('');
  const [auditor, setAuditor] = useState('');
  const [scope, setScope] = useState('');
  const [start, setStart] = useState('');
  const [end, setEnd] = useState('');
  const [error, setError] = useState('');

  const typeOptions = [
    { value: 'internal', label: tr('Interne', 'Internal') },
    { value: 'external', label: tr('Externe', 'External') },
    { value: 'certification', label: tr('Certification', 'Certification') },
    { value: 'surveillance', label: tr('Surveillance', 'Surveillance') },
  ];
  const fwOptions = [{ value: '', label: tr('Programme entier', 'Whole program') }, ...frameworks.map((f) => ({ value: f.id, label: `${f.name}${f.version ? ` ${f.version}` : ''}` }))];

  const submit = () => {
    if (title.trim().length < 2) {
      setError(tr('Le titre doit comporter au moins 2 caractères.', 'Title must be at least 2 characters.'));
      return;
    }
    createAudit.mutate(
      {
        title: title.trim(),
        type,
        framework_id: frameworkId || undefined,
        auditor: auditor.trim() || undefined,
        scope: scope.trim() || undefined,
        scheduled_start: start || undefined,
        scheduled_end: end || undefined,
      },
      {
        onSuccess: (a) => {
          toast.success(tr('Audit planifié', 'Audit scheduled'));
          onCreated?.(a.id);
          onClose();
        },
        onError: (err) => toast.error(errMsg(err, tr('Création échouée', 'Creation failed'))),
      }
    );
  };

  return (
    <ModalShell title={tr('Planifier un audit', 'Schedule an audit')} icon={<CalendarClock size={18} />} onClose={onClose} onSubmit={submit}
      footer={<FooterButtons onCancel={onClose} submitLabel={tr('Planifier', 'Schedule')} pending={createAudit.isPending} />}>
      <Field label={tr('Titre', 'Title')} value={title} onChange={(v) => { setTitle(v); setError(''); }} required autoFocus
        placeholder={tr('ex. Audit interne ISO 27001 Q3', 'e.g. ISO 27001 internal audit Q3')} error={error} />
      <div className="grid grid-cols-2 gap-3">
        <SelectField label={tr('Type', 'Type')} value={type} onChange={(v) => setType(v as AuditType)} options={typeOptions} />
        <SelectField label={tr('Référentiel', 'Framework')} value={frameworkId} onChange={setFrameworkId} options={fwOptions} />
      </div>
      <Field label={tr('Auditeur', 'Auditor')} value={auditor} onChange={setAuditor} placeholder={tr('Nom ou cabinet', 'Name or firm')} />
      <div className="grid grid-cols-2 gap-3">
        <Field label={tr('Début prévu', 'Scheduled start')} value={start} onChange={setStart} type="date" />
        <Field label={tr('Fin prévue', 'Scheduled end')} value={end} onChange={setEnd} type="date" />
      </div>
      <TextArea label={tr('Périmètre', 'Scope')} value={scope} onChange={setScope} placeholder={tr('Ce que couvre l’audit…', 'What the audit covers…')} />
    </ModalShell>
  );
}

/* ------------------------------------------------------------------ */
/* Open a remediation plan                                             */
/* ------------------------------------------------------------------ */

export function CreateRemediationDialog({
  onClose, onCreated, controlId, controlLabel, auditId,
}: {
  onClose: () => void;
  onCreated?: (id: string) => void;
  controlId?: string;   // pre-linked gap
  controlLabel?: string; // human label of the linked control
  auditId?: string;
}) {
  const tr = useTr();
  const { createRemediation } = useRemediations();
  const [title, setTitle] = useState(controlLabel ? tr('Remédier : ', 'Remediate: ') + controlLabel : '');
  const [description, setDescription] = useState('');
  const [priority, setPriority] = useState<RemediationPriority>('medium');
  const [due, setDue] = useState('');
  const [error, setError] = useState('');

  const priorityOptions = [
    { value: 'low', label: tr('Basse', 'Low') },
    { value: 'medium', label: tr('Moyenne', 'Medium') },
    { value: 'high', label: tr('Haute', 'High') },
    { value: 'critical', label: tr('Critique', 'Critical') },
  ];

  const submit = () => {
    if (title.trim().length < 2) {
      setError(tr('Le titre doit comporter au moins 2 caractères.', 'Title must be at least 2 characters.'));
      return;
    }
    createRemediation.mutate(
      {
        title: title.trim(),
        description: description.trim() || undefined,
        priority,
        control_id: controlId || undefined,
        audit_id: auditId || undefined,
        due_date: due || undefined,
      },
      {
        onSuccess: (p) => {
          toast.success(tr('Plan de remédiation créé', 'Remediation plan created'));
          onCreated?.(p.id);
          onClose();
        },
        onError: (err) => toast.error(errMsg(err, tr('Création échouée', 'Creation failed'))),
      }
    );
  };

  return (
    <ModalShell title={tr('Plan de remédiation', 'Remediation plan')} icon={<Wrench size={18} />} onClose={onClose} onSubmit={submit}
      footer={<FooterButtons onCancel={onClose} submitLabel={tr('Créer', 'Create')} pending={createRemediation.isPending} />}>
      {controlLabel && (
        <div className="text-[12px] text-ink-soft px-3 py-2 rounded-[9px]" style={{ background: 'var(--bg-hover)', border: '1px solid var(--border)' }}>
          {tr('Lié au contrôle', 'Linked to control')} : <span className="font-semibold text-ink">{controlLabel}</span>
        </div>
      )}
      <Field label={tr('Titre', 'Title')} value={title} onChange={(v) => { setTitle(v); setError(''); }} required autoFocus
        placeholder={tr('ex. Déployer le MFA sur tous les accès admin', 'e.g. Roll out MFA on all admin access')} error={error} />
      <div className="grid grid-cols-2 gap-3">
        <SelectField label={tr('Priorité', 'Priority')} value={priority} onChange={(v) => setPriority(v as RemediationPriority)} options={priorityOptions} />
        <Field label={tr('Échéance', 'Due date')} value={due} onChange={setDue} type="date" />
      </div>
      <TextArea label={tr('Description', 'Description')} value={description} onChange={setDescription}
        placeholder={tr('Actions à mener pour corriger l’écart…', 'Actions to close the gap…')} />
    </ModalShell>
  );
}
