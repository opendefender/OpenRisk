// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Tamper evidence, retention, and signed export — the three things that turn an
// audit trail from a list into evidence.
//
// The verification result is stated in plain language, and a failure names the
// exact entry and what is wrong with it. "Valid" with no detail would be a
// claim; a head hash, a count and a checked-at timestamp are a receipt.

import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import {
  ShieldCheck,
  ShieldAlert,
  Loader2,
  Download,
  Clock,
  RefreshCw,
  FileJson,
  FileSpreadsheet,
} from 'lucide-react';
import { Btn, Card } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { governanceService, type AuditFilter } from './governanceService';

const KEY = ['governance', 'audit-integrity'];

function download(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = name;
  a.click();
  URL.revokeObjectURL(url);
}

export function AuditIntegrityPanel({
  filter,
  isAdmin,
}: {
  filter: AuditFilter;
  isAdmin: boolean;
}) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const qc = useQueryClient();

  const [verifying, setVerifying] = useState(false);
  const [exporting, setExporting] = useState<'csv' | 'json' | null>(null);

  const report = useQuery({
    queryKey: [...KEY, 'chain'],
    queryFn: governanceService.verifyAuditChain,
    // Re-verifying on every render would hash the whole trail; on demand and on
    // first open is the right cadence.
    staleTime: 5 * 60 * 1000,
  });

  const retention = useQuery({
    queryKey: [...KEY, 'retention'],
    queryFn: governanceService.getRetention,
    enabled: isAdmin,
  });

  const saveRetention = useMutation({
    mutationFn: (days: number) => governanceService.setRetention(days),
    onSuccess: () => {
      toast.success(tr('Rétention enregistrée', 'Retention saved'));
      void qc.invalidateQueries({ queryKey: [...KEY, 'retention'] });
    },
    onError: (e) => {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      toast.error(msg || tr('Enregistrement impossible', 'Could not save'));
    },
  });

  const applyRetention = useMutation({
    mutationFn: governanceService.applyRetention,
    onSuccess: (res) => {
      toast.success(
        res.pruned > 0
          ? tr(
              `${res.pruned} entrée(s) purgée(s) et scellée(s).`,
              `${res.pruned} entries pruned and sealed.`,
            )
          : tr('Rien à purger.', 'Nothing to prune.'),
      );
      void qc.invalidateQueries({ queryKey: KEY });
      void qc.invalidateQueries({ queryKey: ['governance', 'audit'] });
    },
  });

  const runVerify = async () => {
    setVerifying(true);
    try {
      await report.refetch();
    } finally {
      setVerifying(false);
    }
  };

  const doExport = async (format: 'csv' | 'json') => {
    setExporting(format);
    try {
      const blob =
        format === 'json'
          ? await governanceService.exportAuditJson(filter)
          : await governanceService.exportAuditCsv(filter);
      download(blob, `audit-trail-${new Date().toISOString().slice(0, 10)}.${format}`);
      toast.success(tr('Export signé téléchargé', 'Signed export downloaded'));
    } catch {
      toast.error(tr('L’export a échoué', 'The export failed'));
    } finally {
      setExporting(null);
    }
  };

  const r = report.data;
  const valid = r?.valid ?? true;
  const breaks = r?.breaks ?? [];

  const retentionDays = retention.data?.retention_days ?? 0;
  const [draftDays, setDraftDays] = useState<number | null>(null);
  const days = draftDays ?? retentionDays;

  return (
    <div className="space-y-3 mb-4">
      <Card style={{ padding: '14px 16px' }}>
        <div className="flex items-start gap-3 flex-wrap">
          {report.isLoading ? (
            <Loader2
              size={18}
              className="animate-spin shrink-0 mt-0.5"
              style={{ color: 'var(--fg-secondary)' }}
            />
          ) : valid ? (
            <ShieldCheck size={18} style={{ color: 'var(--low)' }} className="shrink-0 mt-0.5" />
          ) : (
            <ShieldAlert
              size={18}
              style={{ color: 'var(--critical)' }}
              className="shrink-0 mt-0.5"
            />
          )}

          <div className="flex-1 min-w-[280px]">
            <p
              className="text-[13.5px] font-bold"
              style={{ color: valid ? 'var(--low)' : 'var(--critical)' }}
            >
              {report.isLoading
                ? tr('Vérification de la chaîne…', 'Verifying the chain…')
                : valid
                  ? tr(
                      'Chaîne intacte — aucune altération détectée',
                      'Chain intact — no alteration detected',
                    )
                  : tr(
                      'Chaîne rompue — le journal a été altéré',
                      'Chain broken — the journal has been altered',
                    )}
            </p>
            <p className="text-[12px] mt-0.5" style={{ color: 'var(--fg-secondary)' }}>
              {tr(
                'Chaque entrée contient le hachage de la précédente. Modifier, supprimer ou réordonner une entrée casse la chaîne à partir de ce point.',
                'Each entry carries the hash of the previous one. Editing, deleting or reordering any entry breaks the chain from that point on.',
              )}
            </p>
            {r && (
              <p className="text-[11.5px] mt-1 mono" style={{ color: 'var(--fg-secondary)' }}>
                {r.verified}/{r.total_events} {tr('vérifiées', 'verified')}
                {r.seals > 0 &&
                  ` · ${r.seals} ${tr('scellé(s) de rétention', 'retention seal(s)')}`}
                {r.head_hash && ` · ${tr('tête', 'head')} ${r.head_hash.slice(0, 12)}…`}
                {` · ${new Date(r.checked_at).toLocaleString()}`}
              </p>
            )}
          </div>

          <div className="flex items-center gap-2">
            <Btn
              label={verifying ? tr('Vérification…', 'Verifying…') : tr('Vérifier', 'Verify')}
              icon={verifying ? Loader2 : RefreshCw}
              onClick={runVerify}
              disabled={verifying}
            />
            <Btn
              label={exporting === 'csv' ? '…' : 'CSV'}
              icon={FileSpreadsheet}
              onClick={() => doExport('csv')}
              disabled={exporting !== null}
            />
            <Btn
              label={exporting === 'json' ? '…' : 'JSON'}
              icon={FileJson}
              onClick={() => doExport('json')}
              disabled={exporting !== null}
            />
          </div>
        </div>

        {!valid && breaks.length > 0 && (
          <div className="mt-3 space-y-1.5">
            {breaks.slice(0, 8).map((b, i) => (
              <div
                key={i}
                className="text-[12px] rounded-[8px] px-2.5 py-1.5"
                style={{
                  background: 'color-mix(in srgb, var(--critical) 8%, transparent)',
                  color: 'var(--fg-primary)',
                }}
              >
                <span className="mono font-bold" style={{ color: 'var(--critical)' }}>
                  #{b.sequence}
                </span>{' '}
                <span className="font-semibold">{b.kind}</span> — {b.detail}
              </div>
            ))}
            {breaks.length > 8 && (
              <p className="text-[11.5px]" style={{ color: 'var(--fg-secondary)' }}>
                {tr(`… et ${breaks.length - 8} autre(s).`, `… and ${breaks.length - 8} more.`)}
              </p>
            )}
          </div>
        )}

        <p className="text-[11.5px] mt-2" style={{ color: 'var(--fg-secondary)' }}>
          {tr(
            'L’export emporte le verdict de la chaîne et une signature du déploiement : un export d’un journal altéré le dit sur sa face.',
            'The export carries the chain verdict and a deployment signature: an export of a tampered journal says so on its face.',
          )}
        </p>
      </Card>

      {isAdmin && (
        <Card style={{ padding: '14px 16px' }}>
          <div className="flex items-center gap-3 flex-wrap">
            <Clock size={16} style={{ color: 'var(--fg-secondary)' }} />
            <div className="flex-1 min-w-[240px]">
              <p className="text-[13px] font-semibold" style={{ color: 'var(--fg-primary)' }}>
                {tr('Rétention', 'Retention')}
              </p>
              <p className="text-[12px]" style={{ color: 'var(--fg-secondary)' }}>
                {days === 0
                  ? tr(
                      'Conservation illimitée — rien n’est jamais supprimé.',
                      'Kept forever — nothing is ever deleted.',
                    )
                  : tr(
                      `Les entrées de plus de ${days} jours sont purgées. La purge est scellée : la chaîne reste vérifiable de part et d’autre de la coupure.`,
                      `Entries older than ${days} days are pruned. The prune is sealed: the chain stays verifiable across the gap.`,
                    )}
                {retention.data?.last_pruned_at &&
                  ` · ${tr('dernière purge', 'last prune')} ${new Date(retention.data.last_pruned_at).toLocaleDateString()}`}
              </p>
            </div>
            <input
              type="number"
              min={0}
              max={3650}
              step={30}
              value={days}
              onChange={(e) => setDraftDays(Number(e.target.value))}
              className="h-9 w-[110px] px-2.5 rounded-[9px] text-[13px] mono"
              style={{
                border: '1px solid var(--border-strong)',
                background: 'var(--bg)',
                color: 'var(--fg-primary)',
              }}
            />
            <span className="text-[12px]" style={{ color: 'var(--fg-secondary)' }}>
              {tr('jours', 'days')}
            </span>
            <Btn
              label={tr('Enregistrer', 'Save')}
              onClick={() => saveRetention.mutate(days)}
              disabled={saveRetention.isPending}
            />
            {retentionDays > 0 && (
              <Btn
                label={tr('Purger maintenant', 'Prune now')}
                icon={Download}
                onClick={() => applyRetention.mutate()}
                disabled={applyRetention.isPending}
              />
            )}
          </div>
          <p className="text-[11.5px] mt-1.5" style={{ color: 'var(--fg-secondary)' }}>
            {tr(
              '0 = conserver indéfiniment. Minimum 30 jours si une fenêtre est définie.',
              '0 = keep forever. Minimum 30 days when a window is set.',
            )}
          </p>
        </Card>
      )}
    </div>
  );
}
