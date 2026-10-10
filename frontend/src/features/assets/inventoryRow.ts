// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// What one inventory row shows beyond the stored fields (#906): a short
// technical name under the asset's name, where it lives, and the icon of its
// type. Pure, so the derivation is tested without rendering.

import {
  AppWindow,
  Boxes,
  Building2,
  Cloud,
  Cpu,
  Database,
  HardDrive,
  KeyRound,
  Laptop,
  Network,
  Plug,
  Server,
  Users,
  type LucideIcon,
} from 'lucide-react';

import type { Asset } from '../../types/asset';

type Attrs = Record<string, unknown>;

function attr(a: Asset, key: string): string {
  const v = (a.attributes as Attrs | undefined)?.[key];
  return typeof v === 'string' ? v.trim() : '';
}

/** "srv-core-01": a hostname, a cloud resource id or an address, if any. */
export function slugOf(a: Asset): string {
  return (
    a.hostnames?.find((h) => h.trim()) ||
    attr(a, 'hostname') ||
    attr(a, 'resource_id') ||
    a.cloud_resource_id ||
    a.ip_addresses?.find((ip) => ip.trim()) ||
    attr(a, 'management_ip') ||
    a.external_id ||
    ''
  );
}

/** Where the asset lives, from the schema's location attributes. */
export function locationOf(a: Asset): string {
  return attr(a, 'physical_location') || attr(a, 'region') || attr(a, 'country') || '';
}

// Types are free strings (scanners and tenants name them), so the icon is
// matched on a normalised name, then on the schema category.
const BY_TYPE: Record<string, LucideIcon> = {
  server: Server,
  serveur: Server,
  application: AppWindow,
  api: Plug,
  database: Database,
  'base de données': Database,
  data: Database,
  storage: HardDrive,
  network: Network,
  réseau: Network,
  directory: KeyRound,
  annuaire: KeyRound,
  site: Building2,
  cloud: Cloud,
  saas: Cloud,
  laptop: Laptop,
  postes: Laptop,
  workstation: Laptop,
  hardware: Cpu,
  matériel: Cpu,
  user: Users,
  supplier: Building2,
};
const BY_CATEGORY: Record<string, LucideIcon> = {
  server: Server,
  application: AppWindow,
  database: Database,
  network: Network,
  cloud: Cloud,
  workstation: Laptop,
  vendor: Building2,
};

export function typeIconOf(a: Asset): LucideIcon {
  return (
    BY_TYPE[(a.type ?? '').trim().toLowerCase()] ??
    BY_CATEGORY[(a.category ?? '').toLowerCase()] ??
    Boxes
  );
}

/** Type pills: each type present, with its count, most frequent first. */
export function typeCounts(assets: Asset[]): { type: string; n: number }[] {
  const m = new Map<string, number>();
  for (const a of assets) {
    const t = (a.type ?? '').trim();
    if (t) m.set(t, (m.get(t) ?? 0) + 1);
  }
  return [...m.entries()]
    .map(([type, n]) => ({ type, n }))
    .sort((x, y) => y.n - x.n || x.type.localeCompare(y.type));
}
