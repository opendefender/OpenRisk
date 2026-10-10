// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The topology's side panel (#907). With nothing selected it lists the
// exposure paths from the internet to critical assets; with a node selected it
// says what the node is, what it depends on and feeds, what threatens it (the
// asset's exposure digest, #937), and offers the compromise chain, dependency
// editing and the asset itself.

import { useState } from 'react';
import { ArrowDownRight, ArrowUpLeft, ChevronDown, Crosshair, X } from 'lucide-react';

import { useI18n } from '../../hooks/useI18n';
import { useAuthStore } from '../../hooks/useAuthStore';
import { formatNumber } from '../../i18n/format';
import { Button, Select } from '../../shared/ds';
import { CritBadge } from '../../shared/ui';
import type { Criticality } from '../../shared/riskColors';
import { DEPENDENCY_TYPES, type AssetDependency, type DependencyType } from '../../types/asset';
import { ExposureDigest } from '../entity-drawer/sections/ExposureSection';
import { useAssetAnalysis } from '../entity-drawer/useEntityDrawer';
import type { ExposurePathLike } from './topologyModel';
import type { TopologyNode } from './topologyTypes';

const panel = 'bg-surface-1 border border-border-subtle rounded-[14px] px-5 py-[18px]';

/* --------------------------------------------------------------- paths */

export function PathsPanel({
  paths,
  nameOf,
  critOf,
  anyExposed,
  activeIndex,
  onPick,
}: {
  paths: readonly ExposurePathLike[];
  nameOf: (id: string) => string;
  critOf: (id: string) => string;
  anyExposed: boolean;
  activeIndex: number | null;
  onPick: (i: number | null) => void;
}) {
  const { t } = useI18n();
  const maxHops = paths.reduce((m, p) => Math.max(m, p.hops), 0);
  return (
    <section className={panel} data-testid="topo-paths">
      <h2 className="m-0 text-[13.5px] font-semibold text-ink">{t('topology.pathsTitle')}</h2>
      <p className="m-0 mt-1.5 mb-3.5 text-[12.5px] leading-[1.55] text-ink-soft">
        {!anyExposed
          ? t('topology.pathsNoExposed')
          : t('topology.pathsSummary', { count: paths.length, hops: maxHops, max: 4 })}
      </p>
      <ol className="m-0 p-0 list-none grid gap-2">
        {paths.map((p, i) => {
          const on = activeIndex === i;
          const last = p.asset_ids[p.asset_ids.length - 1];
          return (
            <li key={p.asset_ids.join('>')}>
              <button
                type="button"
                aria-pressed={on}
                onClick={() => onPick(on ? null : i)}
                title={t('topology.pathShow')}
                data-testid="topo-path"
                className={`w-full text-left px-3 py-2.5 rounded-[10px] border text-[12.5px] leading-[1.6] transition-colors ${
                  on
                    ? 'border-accent bg-surface-2'
                    : 'border-border-subtle hover:border-border-strong'
                }`}
              >
                <span
                  className="mono text-[11px] font-semibold uppercase"
                  style={{
                    color:
                      critOf(last) === 'CRITICAL' ? 'var(--danger-text)' : 'var(--warning-text)',
                  }}
                >
                  {t('topology.pathLabel', { n: i + 1, count: p.hops })}
                </span>
                <span className="block text-ink-soft">
                  {[t('topology.internet'), ...p.asset_ids.map(nameOf)].join(' → ')}
                </span>
              </button>
            </li>
          );
        })}
      </ol>
      {paths.length > 0 && (
        <p className="m-0 mt-3 text-[11.5px] text-ink-muted">{t('topology.pathsRule')}</p>
      )}
    </section>
  );
}

/* ---------------------------------------------------------------- node */

export function NodePanel({
  node,
  zoneLabel,
  slug,
  up,
  down,
  nameOf,
  onSelect,
  onClose,
  onOpenAsset,
  chainOn,
  chainCounts,
  onChain,
  editor,
}: {
  node: TopologyNode;
  zoneLabel: string;
  slug: string;
  up: string[];
  down: string[];
  nameOf: (id: string) => string;
  onSelect: (id: string) => void;
  onClose: () => void;
  onOpenAsset: () => void;
  chainOn: boolean;
  chainCounts: { impacted: number; reachable: number } | null;
  onChain: () => void;
  editor: React.ReactNode;
}) {
  const { t, locale } = useI18n();
  const canSeeVulns = useAuthStore((s) => s.hasPermission('vulnerabilities:read'));
  const analysis = useAssetAnalysis('asset', node.id as string, canSeeVulns);
  const [editing, setEditing] = useState(false);
  const crit = ((node.criticality ?? 'LOW') as string).toLowerCase() as Criticality;

  const heading = 'text-[10.5px] tracking-[.06em] uppercase font-semibold text-ink-muted';

  return (
    <section className={panel} data-testid="topo-node">
      <div className="flex items-start gap-2">
        <div className="flex-1 min-w-0">
          {slug && <div className="mono text-[11px] text-ink-muted truncate">{slug}</div>}
          <h2 className="m-0 mt-1 text-[19px] font-semibold text-ink leading-tight">{node.name}</h2>
          <div className="text-[12.5px] text-ink-soft mt-0.5">
            {[node.type, zoneLabel].filter(Boolean).join(' · ')}
          </div>
        </div>
        <button
          type="button"
          onClick={onClose}
          aria-label={t('topology.close')}
          className="text-ink-muted hover:text-ink p-1"
        >
          <X size={16} />
        </button>
      </div>

      <div className="flex flex-wrap gap-1.5 items-center mt-3">
        <CritBadge crit={crit} />
        <span className="text-[11.5px] px-2 py-0.5 rounded-full bg-surface-2 text-ink-soft">
          {node.internet_exposed ? t('topology.exposed') : t('topology.internal')}
        </span>
      </div>

      <div className="mt-4">
        {canSeeVulns ? (
          <ExposureDigest
            data={analysis.data}
            isLoading={analysis.isLoading}
            onMore={onOpenAsset}
          />
        ) : (
          <p className="m-0 mb-4 text-[12.5px] text-ink-soft">
            {t('topology.vulns', { count: node.vuln_count ?? 0 })}
          </p>
        )}
      </div>

      <div className={`${heading} mb-2`}>{t('topology.dependsOn')}</div>
      <NeighbourList ids={up} icon={ArrowUpLeft} nameOf={nameOf} onSelect={onSelect} />
      <div className={`${heading} mt-3.5 mb-2`}>{t('topology.feeds')}</div>
      <NeighbourList ids={down} icon={ArrowDownRight} nameOf={nameOf} onSelect={onSelect} />

      <p className="m-0 mt-3.5 text-[12.5px] text-ink-soft">
        {t('topology.risks', {
          count: node.risk_count ?? 0,
          score: formatNumber(locale, node.max_risk_score ?? 0, { maximumFractionDigits: 1 }),
        })}
      </p>

      <div className="grid gap-2 mt-4">
        <Button
          variant={chainOn ? 'secondary' : 'ghost'}
          icon={Crosshair}
          onClick={onChain}
          aria-pressed={chainOn}
        >
          {chainOn ? t('topology.chainHide') : t('topology.chain')}
        </Button>
        {chainOn && chainCounts && (
          <p className="m-0 text-[12px] text-ink-muted" data-testid="topo-chain">
            {t('topology.chainResult', chainCounts)}
          </p>
        )}
        <Button variant="primary" onClick={onOpenAsset}>
          {t('topology.openAsset')}
        </Button>
        <button
          type="button"
          aria-expanded={editing}
          onClick={() => setEditing((e) => !e)}
          className="flex items-center gap-1.5 text-[12.5px] text-ink-soft hover:text-ink mt-1"
        >
          <ChevronDown
            size={14}
            className={`transition-transform ${editing ? 'rotate-180' : ''}`}
            aria-hidden="true"
          />
          {t('topology.editDeps')}
        </button>
        {editing && editor}
      </div>
    </section>
  );
}

/* ---------------------------------------------------- dependency editor */

/**
 * Add and remove dependency edges from inside the node panel: the moment you
 * notice a missing dependency is while looking at the graph.
 */
export function DependencyEditor({
  assetId,
  nodes,
  dependencies,
  canEdit,
  busy,
  onCreate,
  onDelete,
}: {
  assetId: string;
  nodes: readonly TopologyNode[];
  dependencies: readonly AssetDependency[];
  canEdit: boolean;
  busy: boolean;
  onCreate: (targetId: string, type: DependencyType) => void | Promise<void>;
  onDelete: (id: string) => void | Promise<void>;
}) {
  const { t } = useI18n();
  const [targetId, setTargetId] = useState('');
  const [type, setType] = useState<DependencyType>('depends_on');
  const nameOf = (id: string) => nodes.find((n) => n.id === id)?.name ?? id.slice(0, 8);
  const rel = (ty: string) => t(`topology.rel.${ty}`, ty.replace(/_/g, ' '));
  const outgoing = dependencies.filter((d) => d.source_asset_id === assetId);
  const incoming = dependencies.filter((d) => d.target_asset_id === assetId);

  return (
    <div className="border-t border-border-subtle pt-3" data-testid="topo-deps">
      <ul className="m-0 p-0 list-none grid gap-1 max-h-32 overflow-y-auto">
        {outgoing.map((d) => (
          <DepRow
            key={d.id}
            d={d}
            text={`→ ${nameOf(d.target_asset_id as string)}`}
            rel={rel}
            canEdit={canEdit}
            onDelete={onDelete}
          />
        ))}
        {incoming.map((d) => (
          <DepRow
            key={d.id}
            d={d}
            text={`← ${nameOf(d.source_asset_id as string)}`}
            rel={rel}
            canEdit={canEdit}
            onDelete={onDelete}
          />
        ))}
      </ul>
      {outgoing.length === 0 && incoming.length === 0 && (
        <p className="m-0 text-[12px] text-ink-muted">{t('topology.noDeps')}</p>
      )}
      {canEdit && (
        <div className="grid gap-1.5 mt-2">
          <Select
            value={targetId}
            onChange={(e) => setTargetId(e.target.value)}
            aria-label={t('topology.target')}
          >
            <option value="">{t('topology.target')}</option>
            {nodes
              .filter((n) => n.id !== assetId)
              .map((n) => (
                <option key={n.id} value={n.id as string}>
                  {n.name}
                </option>
              ))}
          </Select>
          <Select
            value={type}
            onChange={(e) => setType(e.target.value as DependencyType)}
            aria-label={t('topology.deps')}
          >
            {DEPENDENCY_TYPES.map((ty) => (
              <option key={ty} value={ty}>
                {rel(ty)}
              </option>
            ))}
          </Select>
          <Button
            variant="secondary"
            size="sm"
            disabled={!targetId}
            loading={busy}
            onClick={() => {
              void onCreate(targetId, type);
              setTargetId('');
            }}
          >
            {t('topology.add')}
          </Button>
        </div>
      )}
    </div>
  );
}

function NeighbourList({
  ids,
  icon: Icon,
  nameOf,
  onSelect,
}: {
  ids: string[];
  icon: typeof ArrowUpLeft;
  nameOf: (id: string) => string;
  onSelect: (id: string) => void;
}) {
  const { t } = useI18n();
  if (ids.length === 0)
    return <div className="text-[12.5px] text-ink-muted px-2">{t('topology.none')}</div>;
  return (
    <ul className="m-0 p-0 list-none">
      {ids.map((id) => (
        <li key={id}>
          <button
            type="button"
            onClick={() => onSelect(id)}
            className="w-full flex items-center gap-2 h-8 px-2 rounded-[8px] text-[13px] text-left text-ink hover:bg-surface-3"
          >
            <Icon size={14} className="text-ink-muted shrink-0" aria-hidden="true" />
            <span className="truncate">{nameOf(id)}</span>
          </button>
        </li>
      ))}
    </ul>
  );
}

function DepRow({
  d,
  text,
  rel,
  canEdit,
  onDelete,
}: {
  d: AssetDependency;
  text: string;
  rel: (ty: string) => string;
  canEdit: boolean;
  onDelete: (id: string) => void | Promise<void>;
}) {
  const { t } = useI18n();
  return (
    <li className="flex items-center justify-between gap-2 text-[12px]">
      <span className="text-ink-soft min-w-0 truncate">
        {text} <span className="text-ink-muted">({rel(d.type as string)})</span>
      </span>
      {canEdit && (
        <button
          type="button"
          onClick={() => void onDelete(d.id as string)}
          aria-label={t('topology.remove')}
          className="text-ink-muted hover:text-ink"
        >
          <X size={12} />
        </button>
      )}
    </li>
  );
}
