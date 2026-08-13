// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, FileText, Save, Sparkles, Trash2, X } from 'lucide-react';
import { PageFrame, PageHeader, Btn, Card, EmptyState, SkeletonRows } from '../../shared/ui';
import { useToast } from '../../hooks/useToast';
import { useAuthStore } from '../../hooks/useAuthStore';
import { apiErrorMessage } from '../../lib/apiError';
import { riskRuleService, type DraftRisk, type VulnRiskRule } from './riskRuleService';

const CRITICALITIES: VulnRiskRule['min_asset_criticality'][] = ['', 'LOW', 'MEDIUM', 'HIGH', 'CRITICAL'];

/**
 * The vulnerability→risk rule, and the queue of drafts it proposed.
 *
 * Two guarantees this screen exists to make visible:
 *
 *  1. Nothing fires unless the rule is ON. It ships off.
 *  2. Everything it creates is a DRAFT. A machine may propose a risk; entering
 *     one into the register as live work is a human decision, taken here.
 *
 * The preview runs the SAME server-side evaluator that ingest runs, so what is
 * shown is what will happen — a preview computed by a parallel implementation
 * would be worse than none, because it would be trusted and wrong.
 */
export default function RiskRulePage() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const isAdmin = useAuthStore((s) => s.hasPermission('*'));
  const canReview = useAuthStore((s) => s.hasPermission('risks:update'));

  const { data: saved, isLoading } = useQuery({
    queryKey: ['attack-surface', 'risk-rule'],
    queryFn: () => riskRuleService.get(),
  });

  const [draft, setDraft] = useState<VulnRiskRule | null>(null);
  useEffect(() => {
    if (saved) setDraft({ ...saved });
  }, [saved]);

  const dirty = useMemo(
    () => !!draft && !!saved && JSON.stringify(draft) !== JSON.stringify(saved),
    [draft, saved]
  );

  // The preview follows the DRAFT, so the numbers answer "what happens if I save
  // this?" rather than "what happened before I started editing?".
  const { data: preview, isFetching: previewing } = useQuery({
    queryKey: ['attack-surface', 'risk-rule', 'preview', draft],
    queryFn: () => riskRuleService.preview(draft ?? undefined),
    enabled: !!draft,
  });

  const save = useMutation({
    mutationFn: (rule: VulnRiskRule) => riskRuleService.update(rule),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['attack-surface', 'risk-rule'] });
      toast.success('Règle enregistrée.');
    },
    onError: (err) => toast.error(apiErrorMessage(err) || "La règle n'a pas pu être enregistrée."),
  });

  const { data: drafts, isLoading: loadingDrafts } = useQuery({
    queryKey: ['attack-surface', 'draft-risks'],
    queryFn: () => riskRuleService.listDrafts(),
  });

  const [selected, setSelected] = useState<Set<string>>(new Set());
  const review = useMutation({
    mutationFn: ({ ids, decision }: { ids: string[]; decision: 'accept' | 'dismiss' }) =>
      riskRuleService.reviewDrafts(ids, decision),
    onSuccess: (res, vars) => {
      void queryClient.invalidateQueries({ queryKey: ['attack-surface', 'draft-risks'] });
      void queryClient.invalidateQueries({ queryKey: ['risks'] });
      setSelected(new Set());
      const n = vars.decision === 'accept' ? res.accepted?.length ?? 0 : res.dismissed?.length ?? 0;
      const failed = Object.keys(res.failed ?? {}).length;
      // Partial success is reported as partial: silently claiming success for a
      // batch where three items failed is how a review queue stops being trusted.
      if (failed > 0) {
        toast.error(
          `${n} risque(s) traité(s), ${failed} en échec : ${Object.values(res.failed ?? {}).join(', ')}`
        );
      } else {
        toast.success(
          vars.decision === 'accept'
            ? `${n} risque(s) ajouté(s) au registre.`
            : `${n} proposition(s) écartée(s).`
        );
      }
    },
    onError: (err) => toast.error(apiErrorMessage(err) || 'La revue a échoué.'),
  });

  const items = drafts?.items ?? [];
  const allSelected = items.length > 0 && selected.size === items.length;

  const set = <K extends keyof VulnRiskRule>(key: K, value: VulnRiskRule[K]) =>
    setDraft((d) => (d ? { ...d, [key]: value } : d));

  return (
    <PageFrame wide>
      <PageHeader
        title="Vulnérabilité → risque"
        count={drafts ? `${drafts.total} brouillon(s) à valider` : null}
      />

      {isLoading || !draft ? (
        <SkeletonRows rows={4} />
      ) : (
        <Card className="mb-6 p-5">
          <div className="mb-4 flex items-start justify-between gap-4">
            <div>
              <h2 className="text-[15px] font-semibold" style={{ color: 'var(--ink-1)' }}>
                Règle de création automatique
              </h2>
              <p className="mt-1 max-w-2xl text-[13px]" style={{ color: 'var(--ink-3)' }}>
                Quand une vulnérabilité remplit toutes ces conditions, un risque est créé
                en <strong>brouillon</strong>. Il n'entre jamais directement dans le
                registre : c'est vous qui décidez, plus bas.
              </p>
            </div>
            <label className="flex shrink-0 items-center gap-2 text-[13px]" style={{ color: 'var(--ink-2)' }}>
              <input
                type="checkbox"
                checked={draft.enabled}
                disabled={!isAdmin}
                onChange={(e) => set('enabled', e.target.checked)}
              />
              {draft.enabled ? 'Activée' : 'Désactivée'}
            </label>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="CVSS minimum" help="0 pour ne pas filtrer sur le CVSS.">
              <input
                type="number"
                min={0}
                max={10}
                step={0.1}
                value={draft.min_cvss}
                disabled={!isAdmin}
                onChange={(e) => set('min_cvss', Number(e.target.value))}
                className="w-full rounded-lg border px-2.5 py-1.5 text-sm"
                style={{ background: 'var(--surface-2)', borderColor: 'var(--line)', color: 'var(--ink-1)' }}
              />
            </Field>

            <Field label="Criticité de l'actif au minimum" help="Un actif sans criticité définie ne satisfait aucun seuil.">
              <select
                value={draft.min_asset_criticality}
                disabled={!isAdmin}
                onChange={(e) =>
                  set('min_asset_criticality', e.target.value as VulnRiskRule['min_asset_criticality'])
                }
                className="w-full rounded-lg border px-2.5 py-1.5 text-sm"
                style={{ background: 'var(--surface-2)', borderColor: 'var(--line)', color: 'var(--ink-1)' }}
              >
                {CRITICALITIES.map((c) => (
                  <option key={c} value={c}>
                    {c === '' ? 'Toutes' : c}
                  </option>
                ))}
              </select>
            </Field>

            <Toggle
              label="Uniquement les actifs exposés sur Internet"
              checked={draft.require_internet_exposure}
              disabled={!isAdmin}
              onChange={(v) => set('require_internet_exposure', v)}
            />
            <Toggle
              label="Uniquement les vulnérabilités au catalogue CISA KEV"
              checked={draft.require_kev}
              disabled={!isAdmin}
              onChange={(v) => set('require_kev', v)}
            />
            <Toggle
              label="Uniquement les vulnérabilités rattachées à un actif"
              help="Sans actif, il n'y a ni criticité métier ni responsable : le risque ne serait pas actionnable."
              checked={draft.require_asset}
              disabled={!isAdmin}
              onChange={(v) => set('require_asset', v)}
            />
            <Toggle
              label="Notifier les administrateurs à chaque proposition"
              checked={draft.notify_on_create}
              disabled={!isAdmin}
              onChange={(v) => set('notify_on_create', v)}
            />
          </div>

          {/* Live preview against the real register. */}
          <div
            className="mt-4 rounded-xl border p-3"
            style={{ borderColor: 'var(--line)', background: 'var(--surface-2)' }}
          >
            <div className="flex items-center gap-2 text-[13px]" style={{ color: 'var(--ink-2)' }}>
              <Sparkles size={14} />
              {previewing ? (
                'Simulation en cours…'
              ) : preview ? (
                <span>
                  Sur {preview.evaluated} vulnérabilité(s) ouvertes,{' '}
                  <strong style={{ color: 'var(--accent)' }}>{preview.would_create}</strong>{' '}
                  produiraient un brouillon
                  {preview.already_linked > 0
                    ? `, ${preview.already_linked} ont déjà un risque`
                    : ''}
                  .
                </span>
              ) : (
                'Simulation indisponible.'
              )}
            </div>

            {preview && preview.samples.length > 0 && (
              <ul className="mt-2 space-y-1">
                {preview.samples.slice(0, 5).map((s) => (
                  <li key={s.vulnerability_id} className="text-[12px]" style={{ color: 'var(--ink-3)' }}>
                    • {s.cve_id ? `${s.cve_id} — ` : ''}
                    {s.title}
                    {s.asset_name ? ` (${s.asset_name})` : ''}
                  </li>
                ))}
              </ul>
            )}

            {/* A rule producing nothing explains itself rather than just looking broken. */}
            {preview && preview.would_create === 0 && Object.keys(preview.top_rejections).length > 0 && (
              <div className="mt-2">
                <p className="text-[12px] font-medium" style={{ color: 'var(--ink-3)' }}>
                  Pourquoi rien ne se déclencherait :
                </p>
                <ul className="mt-1 space-y-0.5">
                  {Object.entries(preview.top_rejections)
                    .sort((a, b) => b[1] - a[1])
                    .slice(0, 3)
                    .map(([reason, n]) => (
                      <li key={reason} className="text-[12px]" style={{ color: 'var(--ink-3)' }}>
                        • {reason} ({n})
                      </li>
                    ))}
                </ul>
              </div>
            )}
          </div>

          {isAdmin && (
            <div className="mt-4 flex justify-end">
              <button
                onClick={() => save.mutate(draft)}
                disabled={!dirty || save.isPending}
                className="inline-flex items-center gap-1.5 rounded-lg px-3.5 py-2 text-sm font-medium disabled:opacity-40"
                style={{ background: 'var(--accent)', color: 'var(--on-accent, #fff)' }}
              >
                <Save size={15} />
                {save.isPending ? 'Enregistrement…' : 'Enregistrer la règle'}
              </button>
            </div>
          )}
        </Card>
      )}

      {/* Bulk review of the drafts. */}
      <section>
        <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
          <h2 className="text-[15px] font-semibold" style={{ color: 'var(--ink-1)' }}>
            Brouillons proposés
          </h2>
          {canReview && selected.size > 0 && (
            <div className="flex items-center gap-2">
              <span className="text-[12px]" style={{ color: 'var(--ink-3)' }}>
                {selected.size} sélectionné(s)
              </span>
              <button
                onClick={() => review.mutate({ ids: [...selected], decision: 'accept' })}
                disabled={review.isPending}
                className="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-[13px] font-medium disabled:opacity-40"
                style={{ background: 'var(--accent-soft)', color: 'var(--accent)' }}
              >
                <Check size={14} /> Ajouter au registre
              </button>
              <button
                onClick={() => review.mutate({ ids: [...selected], decision: 'dismiss' })}
                disabled={review.isPending}
                className="inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-[13px] disabled:opacity-40"
                style={{ borderColor: 'var(--line)', color: 'var(--critical)' }}
              >
                <Trash2 size={14} /> Écarter
              </button>
            </div>
          )}
        </div>

        {loadingDrafts ? (
          <SkeletonRows rows={4} />
        ) : items.length === 0 ? (
          <EmptyState
            icon={FileText}
            title="Aucun brouillon en attente"
            description="Quand la règle se déclenchera, les risques proposés apparaîtront ici pour validation."
          />
        ) : (
          <div className="space-y-2">
            <label className="flex items-center gap-2 text-[12px]" style={{ color: 'var(--ink-3)' }}>
              <input
                type="checkbox"
                checked={allSelected}
                onChange={(e) =>
                  setSelected(e.target.checked ? new Set(items.map((i) => i.id)) : new Set())
                }
              />
              Tout sélectionner
            </label>
            {items.map((r) => (
              <DraftRow
                key={r.id}
                risk={r}
                checked={selected.has(r.id)}
                onToggle={() =>
                  setSelected((prev) => {
                    const next = new Set(prev);
                    if (next.has(r.id)) next.delete(r.id);
                    else next.add(r.id);
                    return next;
                  })
                }
              />
            ))}
          </div>
        )}
      </section>
    </PageFrame>
  );
}

function DraftRow({
  risk,
  checked,
  onToggle,
}: {
  risk: DraftRisk;
  checked: boolean;
  onToggle: () => void;
}) {
  return (
    <div
      className="flex items-start gap-3 rounded-xl border px-3 py-2.5"
      style={{ borderColor: 'var(--line)', background: 'var(--surface-1)' }}
    >
      <input type="checkbox" checked={checked} onChange={onToggle} className="mt-1" />
      <div className="min-w-0 flex-1">
        <div className="text-[13.5px]" style={{ color: 'var(--ink-1)' }}>
          {risk.name || risk.title}
        </div>
        {/* The tracked origin: what proposed this, and on what grounds. Without
            it a reviewer cannot evaluate the proposal, and the honest response
            to a queue you cannot evaluate is to ignore all of it. */}
        {risk.source_rule_reason ? (
          <div className="mt-0.5 text-[11.5px]" style={{ color: 'var(--ink-3)' }}>
            {risk.source_rule_reason}
          </div>
        ) : null}
      </div>
      <div className="shrink-0 text-right">
        <div className="mono text-[13px] font-semibold" style={{ color: 'var(--ink-2)' }}>
          {risk.score?.toFixed(1) ?? '—'}
        </div>
        <div className="text-[11px]" style={{ color: 'var(--ink-3)' }}>
          {risk.criticality ?? ''}
        </div>
      </div>
      {risk.source_cve_id ? (
        <a
          href={`/vulnerabilities?focus=${risk.source_vulnerability_id ?? ''}`}
          className="mono shrink-0 text-[11px] underline"
          style={{ color: 'var(--ink-3)' }}
        >
          {risk.source_cve_id}
        </a>
      ) : null}
    </div>
  );
}

function Field({
  label,
  help,
  children,
}: {
  label: string;
  help?: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <label className="mb-1 block text-[12px] font-medium" style={{ color: 'var(--ink-2)' }}>
        {label}
      </label>
      {children}
      {help ? (
        <p className="mt-1 text-[11px]" style={{ color: 'var(--ink-3)' }}>
          {help}
        </p>
      ) : null}
    </div>
  );
}

function Toggle({
  label,
  help,
  checked,
  disabled,
  onChange,
}: {
  label: string;
  help?: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <div>
      <label className="flex items-start gap-2 text-[13px]" style={{ color: 'var(--ink-2)' }}>
        <input
          type="checkbox"
          className="mt-0.5"
          checked={checked}
          disabled={disabled}
          onChange={(e) => onChange(e.target.checked)}
        />
        <span>{label}</span>
      </label>
      {help ? (
        <p className="ml-6 mt-0.5 text-[11px]" style={{ color: 'var(--ink-3)' }}>
          {help}
        </p>
      ) : null}
    </div>
  );
}
