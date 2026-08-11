// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// /risks/unmapped — the risks nobody has tied to a compliance control yet.
//
// This screen is the counterpart of a deliberate decision: mapping is OPTIONAL
// at creation. Forcing it there only teaches people to pick the first framework
// in the list, which produces a register full of confident, meaningless
// references. Making the backlog visible afterwards is the honest version of the
// same goal — and unlike a required field, it can be worked through in one pass.
//
// Worst risks first, because that is the order in which an unmapped risk
// actually matters.

import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router';
import { toast } from 'sonner';
import { ArrowRight, BookOpen, Link2, Loader2, ShieldCheck } from 'lucide-react';
import { PageFrame, PageHeader, Card, CritBadge, SkeletonRows, EmptyState } from '../../shared/ui';
import type { Criticality } from '../../shared/riskColors';
import { critColor } from '../../shared/riskColors';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import { ComplianceMappingField, type MappingDraft } from './ComplianceMappingField';
import { useCreateMapping, useUnmappedRisks } from './useTaxonomy';

export function UnmappedRisksPage() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const navigate = useNavigate();
  const canUpdate = useAuthStore((s) => s.hasPermission('risks:update'));

  const { data: rows, isLoading, isError, refetch } = useUnmappedRisks();
  const [openId, setOpenId] = useState<string | null>(null);

  const sorted = useMemo(() => [...(rows ?? [])].sort((a, b) => b.score - a.score), [rows]);

  return (
    <PageFrame>
      <PageHeader
        title={tr('Risques non mappés', 'Unmapped risks')}
        count={sorted.length ? String(sorted.length) : null}
      />
      <p className="mb-4 max-w-[72ch] text-[13px] text-ink-muted">
        {tr(
          "Ces risques ne sont rattachés à aucun contrôle de conformité. Le mapping reste facultatif à la création — c'est ici qu'on rattrape le retard.",
          'These risks are not linked to any compliance control. Mapping stays optional at creation — this is where the backlog gets caught up.',
        )}
      </p>

      {isLoading ? (
        <Card><SkeletonRows rows={5} /></Card>
      ) : isError ? (
        <Card>
          <div className="p-6 text-center text-[13px] text-ink-muted">
            <p>{tr('Impossible de charger la liste.', 'Could not load the list.')}</p>
            <button type="button" onClick={() => refetch()} className="mt-2 text-accent underline">
              {tr('Réessayer', 'Retry')}
            </button>
          </div>
        </Card>
      ) : sorted.length === 0 ? (
        <Card>
          <EmptyState
            icon={ShieldCheck}
            title={tr('Tous les risques sont mappés', 'Every risk is mapped')}
            description={tr(
              'Chaque risque ouvert est rattaché à au moins un contrôle. Rien à rattraper.',
              'Every open risk is linked to at least one control. Nothing to catch up on.',
            )}
          />
        </Card>
      ) : (
        <Card style={{ padding: 0, overflow: 'hidden' }}>
          <div className="border-b border-border px-4 py-2.5 text-[12px] text-ink-muted">
            {tr(
              `${sorted.length} risque(s) sans référentiel — les plus exposés d'abord.`,
              `${sorted.length} risk(s) without a framework — most exposed first.`,
            )}
          </div>
          <ul className="divide-y divide-border">
            {sorted.map((r) => (
              <li key={r.id}>
                <div className="flex flex-wrap items-center gap-3 px-4 py-3">
                  <span
                    className="mono w-12 shrink-0 text-right text-[15px] font-bold"
                    style={{ color: critColor[(r.criticality?.toLowerCase() as Criticality) ?? 'low'] }}
                  >
                    {r.score.toFixed(1)}
                  </span>
                  <CritBadge crit={(r.criticality?.toLowerCase() as Criticality) ?? 'low'} />
                  <button
                    type="button"
                    onClick={() => navigate(`/risks?focus=${r.id}`)}
                    className="min-w-0 flex-1 truncate text-left text-[13.5px] font-semibold text-ink hover:text-accent"
                  >
                    {r.title}
                  </button>
                  {canUpdate ? (
                    <button
                      type="button"
                      onClick={() => setOpenId(openId === r.id ? null : r.id)}
                      className="inline-flex shrink-0 items-center gap-1.5 rounded-full border border-border px-3 py-1.5 text-[12px] font-semibold text-ink-soft hover:border-accent"
                    >
                      <Link2 size={13} />
                      {openId === r.id ? tr('Fermer', 'Close') : tr('Rattacher', 'Map')}
                    </button>
                  ) : null}
                  <button
                    type="button"
                    onClick={() => navigate(`/risks?focus=${r.id}`)}
                    aria-label={tr('Ouvrir le risque', 'Open the risk')}
                    className="shrink-0 text-ink-muted hover:text-accent"
                  >
                    <ArrowRight size={15} />
                  </button>
                </div>

                {/* Mapping happens INLINE — sending a user to another screen to
                    fix a list they are working through is how a backlog stays a
                    backlog. */}
                {openId === r.id ? (
                  <div className="border-t border-border bg-app px-4 py-3">
                    <InlineMapper riskId={r.id} onDone={() => setOpenId(null)} />
                  </div>
                ) : null}
              </li>
            ))}
          </ul>
        </Card>
      )}
    </PageFrame>
  );
}

function InlineMapper({ riskId, onDone }: { riskId: string; onDone: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const navigate = useNavigate();
  const [drafts, setDrafts] = useState<MappingDraft[]>([]);
  const create = useCreateMapping(riskId);

  const save = async () => {
    if (drafts.length === 0) return;
    const failed: string[] = [];
    for (const d of drafts) {
      try {
        await create.mutateAsync({ framework_id: d.framework_id, control_id: d.control_id ?? null });
      } catch {
        failed.push(d.label);
      }
    }
    if (failed.length) {
      toast.error(tr(`Échec pour : ${failed.join(', ')}`, `Failed for: ${failed.join(', ')}`));
      return;
    }
    toast.success(tr('Risque rattaché', 'Risk mapped'));
    setDrafts([]);
    // The row leaves this list on the next fetch, which the mutation already
    // invalidated — nothing to remove by hand.
    onDone();
  };

  return (
    <div className="space-y-3">
      <ComplianceMappingField
        value={drafts}
        onChange={setDrafts}
        onImportFramework={() => navigate('/compliance')}
        disabled={create.isPending}
      />
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={save}
          disabled={drafts.length === 0 || create.isPending}
          className="inline-flex items-center gap-1.5 rounded-full px-3.5 py-2 text-[12.5px] font-semibold disabled:opacity-50"
          style={{ background: 'var(--accent)', color: 'var(--on-accent, var(--text-primary))' }}
        >
          {create.isPending ? <Loader2 size={13} className="animate-spin" /> : <BookOpen size={13} />}
          {tr('Enregistrer le mapping', 'Save the mapping')}
        </button>
        <button type="button" onClick={onDone} className="text-[12.5px] text-ink-muted hover:text-ink">
          {tr('Annuler', 'Cancel')}
        </button>
      </div>
    </div>
  );
}

export default UnmappedRisksPage;
