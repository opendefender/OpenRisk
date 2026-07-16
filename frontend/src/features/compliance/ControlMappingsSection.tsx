// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Cross-framework control mappings ("cross-mapping entre référentiels") shown
// inside a control's drawer: the crosswalks that tie this control to equivalent
// controls in other frameworks. Lets an admin link this control to another
// framework's control so satisfying one signals coverage in the other.

import { useState } from 'react';
import { Link2, Plus, Trash2, X } from 'lucide-react';
import { toast } from 'sonner';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import { SkeletonRows } from '../../shared/ui';
import { useControlMappings, useControls, useFrameworks } from './useCompliance';
import type { ComplianceControl, MappingRelation } from '../../types/compliance';

const RELATIONS: MappingRelation[] = ['equivalent', 'partial', 'related'];

function relationLabel(r: MappingRelation, fr: boolean): string {
  if (r === 'equivalent') return fr ? 'Équivalent' : 'Equivalent';
  if (r === 'partial') return fr ? 'Partiel' : 'Partial';
  return fr ? 'Lié' : 'Related';
}

export function ControlMappingsSection({ control }: { control: ComplianceControl }) {
  const lang = useUIStore((s) => s.lang);
  const fr = lang === 'fr';
  const tr = (f: string, e: string) => (fr ? f : e);
  const hasPermission = useAuthStore((s) => s.hasPermission);
  const canEdit = hasPermission('compliance:controls:update');

  const { mappings, isLoading, create, remove } = useControlMappings(control.id);
  const { frameworks } = useFrameworks();

  const [adding, setAdding] = useState(false);
  const [targetFw, setTargetFw] = useState('');
  const [targetControl, setTargetControl] = useState('');
  const [relation, setRelation] = useState<MappingRelation>('equivalent');

  const { controls: targetControls } = useControls(targetFw || undefined);

  const submit = () => {
    if (!targetControl) return;
    create.mutate(
      { source_control_id: control.id, target_control_id: targetControl, relation },
      {
        onSuccess: () => {
          toast.success(tr('Correspondance ajoutée', 'Mapping added'));
          setAdding(false);
          setTargetFw('');
          setTargetControl('');
          setRelation('equivalent');
        },
        onError: () => toast.error(tr('Ajout impossible (doublon ?)', 'Could not add (duplicate?)')),
      }
    );
  };

  return (
    <div>
      <div className="flex items-center justify-between mt-6 mb-2.5">
        <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">
          {tr('Correspondances', 'Cross-mappings')} · {mappings.length}
        </div>
        {canEdit && !adding && (
          <button
            onClick={() => setAdding(true)}
            className="h-7 px-2.5 rounded-[8px] inline-flex items-center gap-1.5 text-[12px] font-semibold text-ink-soft hover:text-accent transition-colors"
            style={{ border: '1px solid var(--border-strong)' }}
          >
            <Plus size={13} /> {tr('Lier', 'Link')}
          </button>
        )}
      </div>

      {canEdit && adding && (
        <div className="rounded-[11px] p-3 mb-3 flex flex-col gap-2" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}>
          <div className="flex items-center justify-between">
            <div className="text-[12px] font-semibold text-ink">{tr('Lier à un autre référentiel', 'Link to another framework')}</div>
            <button onClick={() => setAdding(false)} className="text-ink-muted hover:text-ink"><X size={15} /></button>
          </div>
          <select
            value={targetFw}
            onChange={(e) => { setTargetFw(e.target.value); setTargetControl(''); }}
            className="w-full h-9 px-2.5 rounded-[9px] text-[12.5px] text-ink outline-none"
            style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-secondary)' }}
          >
            <option value="">{tr('Choisir un référentiel…', 'Choose a framework…')}</option>
            {frameworks
              .filter((f) => f.id !== control.framework_id)
              .map((f) => (
                <option key={f.id} value={f.id}>{f.name}{f.version ? ` ${f.version}` : ''}</option>
              ))}
          </select>
          {targetFw && (
            <select
              value={targetControl}
              onChange={(e) => setTargetControl(e.target.value)}
              className="w-full h-9 px-2.5 rounded-[9px] text-[12.5px] text-ink outline-none"
              style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-secondary)' }}
            >
              <option value="">{tr('Choisir un contrôle…', 'Choose a control…')}</option>
              {targetControls.map((c) => (
                <option key={c.id} value={c.id}>{c.reference_code} — {c.name}</option>
              ))}
            </select>
          )}
          <div className="flex items-center gap-2">
            <select
              value={relation}
              onChange={(e) => setRelation(e.target.value as MappingRelation)}
              className="h-9 px-2.5 rounded-[9px] text-[12.5px] text-ink outline-none flex-1"
              style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-secondary)' }}
            >
              {RELATIONS.map((r) => <option key={r} value={r}>{relationLabel(r, fr)}</option>)}
            </select>
            <button
              onClick={submit}
              disabled={!targetControl || create.isPending}
              className="h-9 px-3.5 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 disabled:opacity-60"
              style={{ background: 'var(--accent)', color: '#fff' }}
            >
              <Link2 size={13} /> {tr('Lier', 'Link')}
            </button>
          </div>
        </div>
      )}

      {isLoading ? (
        <SkeletonRows rows={1} height={48} />
      ) : mappings.length === 0 ? (
        <div className="text-center py-6 text-[12.5px] text-ink-muted">
          {tr('Aucune correspondance. Reliez ce contrôle à un référentiel équivalent.', 'No cross-mappings yet. Link this control to an equivalent framework.')}
        </div>
      ) : (
        <div className="flex flex-col gap-2">
          {mappings.map((m) => {
            // Show the OTHER side of the link relative to the current control.
            const otherIsTarget = m.source_control_id === control.id;
            const code = otherIsTarget ? m.target_code : m.source_code;
            const name = otherIsTarget ? m.target_name : m.source_name;
            const fwName = otherIsTarget ? m.target_framework_name : m.source_framework_name;
            return (
              <div key={m.id} className="flex items-center gap-3 px-3 py-2.5 rounded-[11px]" style={{ border: '1px solid var(--border)' }}>
                <div className="w-9 h-9 rounded-[9px] flex items-center justify-center shrink-0" style={{ background: 'var(--accent-soft)', color: 'var(--accent)' }}><Link2 size={16} /></div>
                <div className="flex-1 min-w-0">
                  <div className="text-[13px] font-medium text-ink truncate">
                    <span className="mono text-ink-soft">{code}</span> · {name}
                  </div>
                  <div className="text-[11.5px] text-ink-muted truncate">{fwName} · {relationLabel(m.relation, fr)}</div>
                </div>
                {canEdit && (
                  <button
                    onClick={() => { if (window.confirm(tr('Supprimer cette correspondance ?', 'Remove this mapping?'))) remove.mutate(m.id); }}
                    className="w-8 h-8 rounded-lg flex items-center justify-center transition-colors"
                    style={{ color: 'var(--critical)' }}
                    title={tr('Supprimer', 'Remove')}
                  >
                    <Trash2 size={15} />
                  </button>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
