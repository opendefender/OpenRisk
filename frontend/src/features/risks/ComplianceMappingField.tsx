// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The compliance mapping field on the risk form.
//
// Three rules, all of them reactions to how the previous version failed:
//
//  1. The list comes from GET /compliance/frameworks?imported=true. NEVER a
//     hard-coded array — the old <select> offered ISO27001/CIS/NIST/OWASP as
//     free strings whether or not the tenant had imported anything, so picking
//     one produced a badge that pointed at nothing.
//  2. With no imported framework, an inline EmptyState with a real way out,
//     not an empty dropdown that reads as a bug.
//  3. Importing opens a drawer OVER the form. The form is never unmounted, so
//     the risk you were describing is still there when you come back. Losing a
//     half-written risk to fix a missing framework is a worse bug than the
//     missing framework.
//
// Mapping stays OPTIONAL: a risk is creatable without one, and /risks/unmapped
// is where the backlog is caught up. Forcing it here would only teach people to
// pick the first entry in the list.

import { useState } from 'react';
import { BookOpen, Check, ExternalLink, Loader2, Plus, X } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useFrameworkControls, useImportedFrameworks } from './useTaxonomy';

export interface MappingDraft {
  framework_id: string;
  control_id?: string | null;
  /** Display only — what the chip shows before the server echoes it back. */
  label: string;
}

interface Props {
  value: MappingDraft[];
  onChange: (next: MappingDraft[]) => void;
  /** Opens the framework-import drawer without unmounting the form. */
  onImportFramework?: () => void;
  disabled?: boolean;
}

export function ComplianceMappingField({ value, onChange, onImportFramework, disabled }: Props) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const { data: frameworks, isLoading, isError, refetch } = useImportedFrameworks();
  const [frameworkId, setFrameworkId] = useState<string>('');
  const [controlId, setControlId] = useState<string>('');
  const { data: controls, isLoading: controlsLoading } = useFrameworkControls(frameworkId || null);

  const add = () => {
    if (!frameworkId) return;
    const fw = frameworks?.find((f) => f.id === frameworkId);
    const ctrl = controlId ? controls?.find((c) => c.id === controlId) : undefined;
    const label = ctrl ? `${fw?.name ?? ''} · ${ctrl.reference_code}` : (fw?.name ?? '');
    // The same statement twice is still one statement.
    if (
      value.some(
        (m) => m.framework_id === frameworkId && (m.control_id ?? null) === (controlId || null),
      )
    )
      return;
    onChange([...value, { framework_id: frameworkId, control_id: controlId || null, label }]);
    setControlId('');
  };

  const remove = (i: number) => onChange(value.filter((_, idx) => idx !== i));

  if (isLoading) {
    return <div className="h-20 animate-pulse rounded-3xl bg-app" aria-busy />;
  }

  if (isError) {
    return (
      <div className="rounded-3xl border border-border bg-app p-4 text-[13px] text-ink-muted">
        <p>{tr('Impossible de charger les référentiels.', 'Could not load the frameworks.')}</p>
        <button type="button" onClick={() => refetch()} className="mt-1.5 text-accent underline">
          {tr('Réessayer', 'Retry')}
        </button>
      </div>
    );
  }

  // Rule 2: nothing to map to yet → say so, and offer the fix inline.
  if (!frameworks || frameworks.length === 0) {
    return (
      <div className="rounded-3xl border border-dashed border-border bg-app p-4 text-center">
        <BookOpen size={20} className="mx-auto mb-2 text-ink-muted" />
        <p className="text-[13px] font-semibold text-ink">
          {tr('Aucun référentiel importé', 'No framework imported yet')}
        </p>
        <p className="mx-auto mt-1 max-w-[46ch] text-[12px] text-ink-muted">
          {tr(
            'Importez-en un pour rattacher ce risque à un contrôle réel. Vous pouvez aussi créer le risque sans mapping et le rattacher plus tard depuis « Risques non mappés ».',
            'Import one to link this risk to a real control. You can also create the risk without a mapping and link it later from "Unmapped risks".',
          )}
        </p>
        {onImportFramework ? (
          <button
            type="button"
            onClick={onImportFramework}
            disabled={disabled}
            className="mt-3 inline-flex items-center gap-1.5 rounded-full px-3.5 py-2 text-[12.5px] font-semibold"
            style={{ background: 'var(--accent)', color: 'var(--on-accent, var(--fg-primary))' }}
          >
            <Plus size={14} />
            {tr('Importer un référentiel', 'Import a framework')}
          </button>
        ) : null}
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {value.length > 0 ? (
        <div className="flex flex-wrap gap-1.5">
          {value.map((m, i) => (
            <span
              key={`${m.framework_id}-${m.control_id ?? 'fw'}`}
              className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[12px] font-semibold"
              style={{
                background: 'color-mix(in srgb, var(--accent) 14%, transparent)',
                color: 'var(--accent-500)',
              }}
            >
              <Check size={11} />
              {m.label}
              <button
                type="button"
                onClick={() => remove(i)}
                disabled={disabled}
                aria-label={tr('Retirer', 'Remove')}
                className="opacity-70 hover:opacity-100"
              >
                <X size={11} />
              </button>
            </span>
          ))}
        </div>
      ) : null}

      <div className="grid gap-2 sm:grid-cols-[1fr_1fr_auto]">
        <select
          value={frameworkId}
          onChange={(e) => {
            setFrameworkId(e.target.value);
            setControlId('');
          }}
          disabled={disabled}
          aria-label={tr('Référentiel', 'Framework')}
          className="rounded-2xl border border-border bg-elevated px-3 py-2.5 text-sm text-ink"
        >
          <option value="">{tr('Référentiel…', 'Framework…')}</option>
          {frameworks.map((f) => (
            <option key={f.id} value={f.id}>
              {f.name}
              {f.version ? ` ${f.version}` : ''}
            </option>
          ))}
        </select>

        <select
          value={controlId}
          onChange={(e) => setControlId(e.target.value)}
          disabled={disabled || !frameworkId || controlsLoading}
          aria-label={tr('Contrôle', 'Control')}
          className="rounded-2xl border border-border bg-elevated px-3 py-2.5 text-sm text-ink disabled:opacity-60"
        >
          {/* A framework-level link is a legitimate, honest answer: "this relates
              to ISO 27001, we have not pinned the clause yet". */}
          <option value="">{tr('Tout le référentiel', 'Whole framework')}</option>
          {(controls ?? []).map((c) => (
            <option key={c.id} value={c.id}>
              {c.reference_code} — {c.name}
            </option>
          ))}
        </select>

        <button
          type="button"
          onClick={add}
          disabled={disabled || !frameworkId}
          className="inline-flex items-center justify-center gap-1.5 rounded-2xl px-3.5 py-2.5 text-[12.5px] font-semibold disabled:opacity-50"
          style={{ background: 'var(--accent)', color: 'var(--on-accent, var(--fg-primary))' }}
        >
          {controlsLoading ? <Loader2 size={14} className="animate-spin" /> : <Plus size={14} />}
          {tr('Ajouter', 'Add')}
        </button>
      </div>

      <p className="text-[11px] text-ink-muted">
        {tr(
          'Le mapping est optionnel : le risque reste créable sans. Vous pourrez le rattacher plus tard.',
          'Mapping is optional: the risk is creatable without one. You can link it later.',
        )}
      </p>

      {onImportFramework ? (
        <button
          type="button"
          onClick={onImportFramework}
          disabled={disabled}
          className="inline-flex items-center gap-1 text-[12px] font-semibold text-accent hover:underline"
        >
          {tr('Importer un autre référentiel', 'Import another framework')}
          <ExternalLink size={11} />
        </button>
      ) : null}
    </div>
  );
}
