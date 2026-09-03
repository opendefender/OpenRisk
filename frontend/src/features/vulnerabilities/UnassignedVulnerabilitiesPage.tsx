// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, HelpCircle, Link2Off, ShieldQuestion, Users } from 'lucide-react';
import { PageFrame, PageHeader, Btn, EmptyState, SkeletonRows } from '../../shared/ui';
import { useToast } from '../../hooks/useToast';
import { useAuthStore } from '../../hooks/useAuthStore';
import { apiErrorMessage } from '../../lib/apiError';
import { useAssets } from '../assets/useAssets';
import { vulnerabilityService, type Vulnerability } from './vulnerabilityService';
import { SEVERITY_META } from './vulnMeta';

/**
 * Unassigned vulnerabilities — the findings the correlator refused to guess on.
 *
 * Two situations land here, and keeping them apart is the point of the screen:
 *
 *   AMBIGUOUS  several assets matched equally well. The fix is usually in the
 *              inventory (a duplicate or stale asset), and the candidates are
 *              shown so the user can see the collision.
 *   NO MATCH   nothing matched. The fix is usually a missing asset, or a
 *              missing fingerprint on an existing one.
 *
 * Attributing here is a decision, and it is pinned: the next scan will not undo
 * it.
 */
export default function UnassignedVulnerabilitiesPage() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const canWrite = useAuthStore((s) => s.hasPermission('vulnerabilities:update'));
  const { assets } = useAssets();
  const [expanded, setExpanded] = useState<string | null>(null);

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['vulnerabilities', 'unassigned'],
    queryFn: () => vulnerabilityService.listUnassigned({ limit: 100 }),
  });

  const resolve = useMutation({
    mutationFn: ({ id, assetId }: { id: string; assetId: string | null }) =>
      vulnerabilityService.resolveAsset(id, assetId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vulnerabilities'] });
    },
  });

  const items = useMemo(() => data?.items ?? [], [data]);
  const ambiguous = useMemo(() => items.filter((v) => v.match_ambiguous), [items]);
  const unmatched = useMemo(() => items.filter((v) => !v.match_ambiguous), [items]);

  const attribute = async (v: Vulnerability, assetId: string | null) => {
    try {
      await resolve.mutateAsync({ id: v.id, assetId });
      toast.success(
        assetId
          ? 'Vulnérabilité rattachée. La priorité a été recalculée avec la criticité de cet actif.'
          : 'Vulnérabilité détachée : elle ne concerne aucun actif inventorié.',
      );
      setExpanded(null);
    } catch (err) {
      toast.error(apiErrorMessage(err) || 'Le rattachement a échoué.');
    }
  };

  return (
    <PageFrame wide>
      <PageHeader
        title="Vulnérabilités non rattachées"
        count={data ? `${data.total} à traiter` : null}
        actions={<Btn label="Actualiser" onClick={() => void refetch()} />}
      />

      <p className="mb-4 text-[13px]" style={{ color: 'var(--fg-muted)' }}>
        Ces vulnérabilités n'ont pas pu être attribuées à un actif avec assez de certitude. Tant
        qu'elles ne le sont pas, elles sont priorisées sans criticité métier — c'est-à-dire mal.
        Votre décision ici est conservée : le prochain scan ne l'écrasera pas.
      </p>

      {isLoading ? (
        <SkeletonRows rows={6} />
      ) : isError ? (
        <div
          className="rounded-2xl border p-6 text-center"
          style={{ borderColor: 'var(--border)' }}
        >
          <p className="text-sm" style={{ color: 'var(--fg-secondary)' }}>
            La liste n'a pas pu être chargée.
          </p>
          <Btn label="Réessayer" onClick={() => void refetch()} />
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          icon={CheckCircle2}
          title="Tout est rattaché"
          description="Chaque vulnérabilité connue est attribuée à un actif de votre inventaire."
        />
      ) : (
        <div className="space-y-6">
          <Section
            title="Rattachement ambigu"
            icon={Users}
            hint="Plusieurs actifs correspondent aussi bien l'un que l'autre. C'est presque toujours un doublon ou une fiche obsolète dans l'inventaire."
            items={ambiguous}
            expanded={expanded}
            setExpanded={setExpanded}
            assets={assets}
            canWrite={canWrite}
            busy={resolve.isPending}
            onAttribute={attribute}
          />
          <Section
            title="Aucune correspondance"
            icon={ShieldQuestion}
            hint="Aucun actif ne porte l'empreinte signalée. Soit l'actif n'est pas inventorié, soit sa fiche ne déclare pas son nom d'hôte, son IP ou son identifiant cloud."
            items={unmatched}
            expanded={expanded}
            setExpanded={setExpanded}
            assets={assets}
            canWrite={canWrite}
            busy={resolve.isPending}
            onAttribute={attribute}
          />
        </div>
      )}
    </PageFrame>
  );
}

function Section({
  title,
  icon: Icon,
  hint,
  items,
  expanded,
  setExpanded,
  assets,
  canWrite,
  busy,
  onAttribute,
}: {
  title: string;
  icon: typeof Users;
  hint: string;
  items: Vulnerability[];
  expanded: string | null;
  setExpanded: (id: string | null) => void;
  assets: { id?: string; name?: string; criticality?: string }[];
  canWrite: boolean;
  busy: boolean;
  onAttribute: (v: Vulnerability, assetId: string | null) => void;
}) {
  if (items.length === 0) return null;
  return (
    <section>
      <h2
        className="mb-1 flex items-center gap-2 text-[14px] font-semibold"
        style={{ color: 'var(--fg-primary)' }}
      >
        <Icon size={16} />
        {title}
        <span className="font-normal" style={{ color: 'var(--fg-muted)' }}>
          ({items.length})
        </span>
      </h2>
      <p className="mb-2 text-[12px]" style={{ color: 'var(--fg-muted)' }}>
        {hint}
      </p>
      <div className="space-y-2">
        {items.map((v) => (
          <VulnRow
            key={v.id}
            vuln={v}
            open={expanded === v.id}
            onToggle={() => setExpanded(expanded === v.id ? null : v.id)}
            assets={assets}
            canWrite={canWrite}
            busy={busy}
            onAttribute={onAttribute}
          />
        ))}
      </div>
    </section>
  );
}

function VulnRow({
  vuln,
  open,
  onToggle,
  assets,
  canWrite,
  busy,
  onAttribute,
}: {
  vuln: Vulnerability;
  open: boolean;
  onToggle: () => void;
  assets: { id?: string; name?: string; criticality?: string }[];
  canWrite: boolean;
  busy: boolean;
  onAttribute: (v: Vulnerability, assetId: string | null) => void;
}) {
  const [manualAsset, setManualAsset] = useState('');

  // Candidates are only fetched when the row is opened: the list page would
  // otherwise fire one request per finding on load.
  const { data: candidates, isLoading: loadingCandidates } = useQuery({
    queryKey: ['vulnerabilities', vuln.id, 'match-candidates'],
    queryFn: () => vulnerabilityService.matchCandidates(vuln.id),
    enabled: open,
  });

  const sev = SEVERITY_META[vuln.severity]?.color ?? 'var(--fg-muted)';

  return (
    <div
      className="rounded-xl border"
      style={{ borderColor: 'var(--border)', background: 'var(--surface-1)' }}
    >
      <button
        type="button"
        onClick={onToggle}
        className="flex w-full items-center gap-3 px-3 py-2.5 text-left"
      >
        <span
          className="rounded px-1.5 py-0.5 text-[10px] font-bold uppercase"
          style={{ background: sev, color: 'var(--fg-inverse)' }}
        >
          {vuln.severity}
        </span>
        <span className="min-w-0 flex-1">
          <span className="block truncate text-[13.5px]" style={{ color: 'var(--fg-primary)' }}>
            {vuln.cve_id ? `${vuln.cve_id} — ` : ''}
            {vuln.title}
          </span>
          <span className="mono block text-[11px]" style={{ color: 'var(--fg-muted)' }}>
            {vuln.source}
            {vuln.match_confidence > 0
              ? ` · meilleure correspondance ${Math.round(vuln.match_confidence * 100)} %`
              : ' · aucune correspondance'}
            {vuln.match_method ? ` (${vuln.match_method})` : ''}
          </span>
        </span>
        <span className="mono text-[12px]" style={{ color: 'var(--fg-muted)' }}>
          {vuln.priority_tier}
        </span>
      </button>

      {open && (
        <div className="border-t px-3 py-3" style={{ borderColor: 'var(--border)' }}>
          {loadingCandidates ? (
            <SkeletonRows rows={2} height={32} />
          ) : (candidates?.length ?? 0) > 0 ? (
            <>
              <p className="mb-2 text-[12px]" style={{ color: 'var(--fg-muted)' }}>
                Actifs candidats, du plus au moins probable :
              </p>
              <div className="space-y-1.5">
                {candidates?.map((c) => (
                  <div
                    key={c.asset_id}
                    className="flex items-center gap-3 rounded-lg border px-2.5 py-2"
                    style={{ borderColor: 'var(--border)', background: 'var(--surface-2)' }}
                  >
                    <div className="min-w-0 flex-1">
                      <div className="text-[13px]" style={{ color: 'var(--fg-primary)' }}>
                        {c.asset_name}{' '}
                        <span className="text-[11px]" style={{ color: 'var(--fg-muted)' }}>
                          ({c.criticality})
                        </span>
                      </div>
                      <div className="text-[11px]" style={{ color: 'var(--fg-muted)' }}>
                        {c.reason}
                      </div>
                    </div>
                    <span
                      className="mono shrink-0 text-[12px] font-semibold"
                      style={{ color: 'var(--fg-secondary)' }}
                    >
                      {Math.round(c.confidence * 100)} %
                    </span>
                    {canWrite && (
                      <button
                        type="button"
                        disabled={busy}
                        onClick={() => onAttribute(vuln, c.asset_id)}
                        className="shrink-0 rounded-lg px-2.5 py-1 text-[12px] font-medium disabled:opacity-40"
                        style={{ background: 'var(--accent-soft)', color: 'var(--accent-500)' }}
                      >
                        Rattacher
                      </button>
                    )}
                  </div>
                ))}
              </div>
            </>
          ) : (
            <p
              className="mb-2 flex items-center gap-1.5 text-[12px]"
              style={{ color: 'var(--fg-muted)' }}
            >
              <HelpCircle size={13} />
              Aucun actif candidat. Choisissez-en un manuellement, ou détachez la vulnérabilité.
            </p>
          )}

          {canWrite && (
            <div className="mt-3 flex flex-wrap items-center gap-2">
              <select
                value={manualAsset}
                onChange={(e) => setManualAsset(e.target.value)}
                className="rounded-lg border px-2 py-1.5 text-[12px]"
                style={{
                  background: 'var(--surface-2)',
                  borderColor: 'var(--border)',
                  color: 'var(--fg-primary)',
                }}
              >
                <option value="">Choisir un autre actif…</option>
                {assets.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </select>
              <button
                type="button"
                disabled={!manualAsset || busy}
                onClick={() => onAttribute(vuln, manualAsset)}
                className="rounded-lg px-2.5 py-1.5 text-[12px] font-medium disabled:opacity-40"
                style={{ background: 'var(--accent-soft)', color: 'var(--accent-500)' }}
              >
                Rattacher
              </button>
              <button
                type="button"
                disabled={busy}
                onClick={() => onAttribute(vuln, null)}
                className="ml-auto inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-[12px] disabled:opacity-40"
                style={{ borderColor: 'var(--border)', color: 'var(--fg-secondary)' }}
                title="Cette vulnérabilité ne concerne aucun actif que nous possédons"
              >
                <Link2Off size={13} />
                Ne concerne aucun actif
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
