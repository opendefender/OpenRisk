// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// "Déclarer les actifs exposés" (#907): exposure paths start from assets
// declared internet-exposed, so a tenant that has declared none sees no path.
// This dialog lets an editor declare them from the topology, where the need is
// felt. Only assets whose type carries the attribute are offered (server,
// application, network, cloud); those in a DMZ or perimeter zone are ticked as
// likely. Each save merges the flag into the asset's own attributes, and a
// refusal (a tenant schema without the attribute) is reported per asset.

import { useMemo, useState } from 'react';

import { useI18n } from '../../hooks/useI18n';
import { Button, Checkbox, Modal } from '../../shared/ds';
import { assetService } from '../../services/assetService';
import type { TopologyNode } from './topologyTypes';

const EXPOSABLE = new Set(['server', 'application', 'network', 'cloud']);
const LIKELY_ZONES = new Set(['dmz', 'perimetre']);

export function ExposureDeclareDialog({
  nodes,
  onClose,
  onSaved,
}: {
  nodes: readonly TopologyNode[];
  onClose: () => void;
  /** Called after a save with how many were declared; the page refreshes. */
  onSaved: (declared: number) => void;
}) {
  const { t } = useI18n();
  const candidates = useMemo(
    () =>
      nodes
        .filter((n) => !n.internet_exposed && EXPOSABLE.has((n.category ?? '') as string))
        .map((n) => ({ n, likely: LIKELY_ZONES.has((n.zone ?? '') as string) }))
        .sort(
          (a, b) =>
            Number(b.likely) - Number(a.likely) || (a.n.name ?? '').localeCompare(b.n.name ?? ''),
        ),
    [nodes],
  );
  const [picked, setPicked] = useState<Set<string>>(
    () => new Set(candidates.filter((c) => c.likely).map((c) => c.n.id as string)),
  );
  const [saving, setSaving] = useState(false);
  const [failed, setFailed] = useState<string[]>([]);

  const save = async () => {
    setSaving(true);
    const errors: string[] = [];
    let ok = 0;
    for (const id of picked) {
      try {
        const asset = await assetService.getAsset(id);
        await assetService.updateAsset(id, {
          attributes: { ...(asset.attributes ?? {}), internet_exposed: true },
        });
        ok++;
      } catch {
        errors.push(candidates.find((c) => c.n.id === id)?.n.name ?? id);
      }
    }
    setSaving(false);
    setFailed(errors);
    if (ok > 0) onSaved(ok);
    // With refusals the dialog stays open on the list of what failed.
    if (errors.length === 0) onClose();
  };

  return (
    <Modal
      open
      onClose={onClose}
      dismissable={!saving}
      title={t('topology.declare.title')}
      subtitle={t('topology.declare.sub')}
      closeLabel={t('topology.close')}
      footer={
        <>
          <Button variant="ghost" onClick={onClose} disabled={saving}>
            {t('topology.declare.cancel')}
          </Button>
          <Button
            variant="primary"
            loading={saving}
            disabled={picked.size === 0}
            onClick={() => void save()}
          >
            {t('topology.declare.save', { count: picked.size })}
          </Button>
        </>
      }
    >
      {candidates.length === 0 ? (
        <p className="m-0 text-[13px] text-ink-soft">{t('topology.declare.none')}</p>
      ) : (
        <ul
          className="m-0 p-0 list-none grid gap-1 max-h-[50vh] overflow-auto"
          data-testid="declare-list"
        >
          {candidates.map(({ n, likely }) => {
            const id = n.id as string;
            return (
              <li key={id} className="flex items-center gap-3 py-1.5">
                <Checkbox
                  checked={picked.has(id)}
                  onChange={(e) => {
                    const on = e.target.checked;
                    setPicked((s) => {
                      const next = new Set(s);
                      if (on) next.add(id);
                      else next.delete(id);
                      return next;
                    });
                  }}
                  label={n.name ?? id}
                />
                {likely && (
                  <span className="text-[11px] px-1.5 py-0.5 rounded bg-surface-2 text-ink-soft">
                    {t('topology.declare.likely', {
                      zone: t(`assetAnalysis.zones.${n.zone ?? ''}`, (n.zone ?? '') as string),
                    })}
                  </span>
                )}
              </li>
            );
          })}
        </ul>
      )}
      {failed.length > 0 && (
        <p role="alert" className="m-0 mt-3 text-[12.5px] text-danger-text">
          {t('topology.declare.failed', { names: failed.join(', ') })}
        </p>
      )}
    </Modal>
  );
}
