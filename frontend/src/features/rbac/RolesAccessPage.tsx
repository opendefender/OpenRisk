// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Roles & access (Tenant Admin). Two views:
//  - Roles: the business-role catalog with its permission matrix — what each GRC
//    job role (RSSI, Risk Manager, Auditor, …) can do, grouped by domain.
//  - Members: the tenant's members with their org role + business role, and an
//    inline selector to (re)assign a business role. Admin-gated (route + nav).

import { useMemo, useState } from 'react';
import { toast } from 'sonner';
import { ShieldCheck, Users, KeyRound } from 'lucide-react';
import { PageFrame, PageHeader, Card, Chip, SkeletonRows, EmptyState } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { useRbacCatalog, useRbacMembers, useAssignBusinessRole } from './useRbac';
import type { BusinessRole, PermissionDef, MemberView } from './rbacService';

type Tab = 'roles' | 'members';

const GROUP_LABELS: Record<string, { fr: string; en: string }> = {
  risks: { fr: 'Risques', en: 'Risks' },
  assets: { fr: 'Actifs', en: 'Assets' },
  mitigations: { fr: 'Traitements', en: 'Mitigations' },
  vulnerabilities: { fr: 'Vulnérabilités', en: 'Vulnerabilities' },
  incidents: { fr: 'Incidents', en: 'Incidents' },
  compliance: { fr: 'Conformité', en: 'Compliance' },
  scanner: { fr: 'Scanner', en: 'Scanner' },
  automation: { fr: 'Automatisation', en: 'Automation' },
  reports: { fr: 'Rapports', en: 'Reports' },
};

export function RolesAccessPage() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const [tab, setTab] = useState<Tab>('roles');

  const TabBtn = ({ id, label }: { id: Tab; label: string }) => (
    <button
      onClick={() => setTab(id)}
      className="h-9 px-3.5 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5"
      style={{
        background: tab === id ? 'var(--accent)' : 'transparent',
        color: tab === id ? '#fff' : 'var(--text-secondary)',
        border: tab === id ? 'none' : '1px solid var(--border-strong)',
      }}
    >
      {label}
    </button>
  );

  return (
    <PageFrame wide>
      <PageHeader
        title={tr('Rôles & accès', 'Roles & access')}
        count={tr(
          'Rôles métiers GRC · matrice de permissions · affectation',
          'GRC business roles · permission matrix · assignment'
        )}
      />
      <div className="flex gap-2 mb-4 flex-wrap">
        <TabBtn id="roles" label={tr('Rôles & permissions', 'Roles & permissions')} />
        <TabBtn id="members" label={tr('Membres', 'Members')} />
      </div>

      {tab === 'roles' && <RolesView />}
      {tab === 'members' && <MembersView />}
    </PageFrame>
  );
}

// ---------------------------------------------------------------------------
// Roles view — the business-role catalog + permission matrix.
// ---------------------------------------------------------------------------
function RolesView() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data, isLoading } = useRbacCatalog();

  const byGroup = useMemo(() => {
    const map: Record<string, PermissionDef[]> = {};
    for (const p of data?.permissions ?? []) (map[p.group] ??= []).push(p);
    return map;
  }, [data]);

  if (isLoading) return <SkeletonRows rows={6} />;
  if (!data) return <EmptyState icon={ShieldCheck} title={tr('Aucun rôle', 'No roles')} />;

  const label = (r: BusinessRole) => (lang === 'fr' ? r.label_fr : r.label_en);
  const desc = (r: BusinessRole) => (lang === 'fr' ? r.description_fr : r.description_en);
  const permLabel = (key: string): string => {
    const def = data.permissions.find((p) => p.key === key);
    if (!def) return key;
    return lang === 'fr' ? def.label_fr : def.label_en;
  };

  return (
    <div className="grid gap-4" style={{ gridTemplateColumns: 'repeat(auto-fill,minmax(340px,1fr))' }}>
      {data.business_roles.map((role) => {
        const groups = Object.keys(byGroup).filter((g) =>
          role.permissions.some((p) => byGroup[g]?.some((pd) => pd.key === p))
        );
        return (
          <Card key={role.key} className="p-4">
            <div className="flex items-center justify-between gap-2 mb-1">
              <div className="text-[15px] font-bold text-ink">{label(role)}</div>
              <span className="text-[10px] mono px-1.5 py-0.5 rounded" style={{ background: 'var(--accent-soft)', color: 'var(--accent)' }}>
                {role.permissions.length} {tr('perms', 'perms')}
              </span>
            </div>
            <div className="text-[12px] text-ink-soft mb-3 leading-snug">{desc(role)}</div>
            <div className="space-y-2.5">
              {groups.map((g) => {
                const granted = role.permissions.filter((p) => byGroup[g]?.some((pd) => pd.key === p));
                return (
                  <div key={g}>
                    <div className="text-[10.5px] uppercase tracking-wide text-ink-muted font-semibold mb-1">
                      {GROUP_LABELS[g] ? tr(GROUP_LABELS[g].fr, GROUP_LABELS[g].en) : g}
                    </div>
                    <div className="flex flex-wrap gap-1.5">
                      {granted.map((p) => (
                        <span
                          key={p}
                          title={p}
                          className="text-[11px] px-2 py-0.5 rounded-[7px]"
                          style={{ background: 'var(--bg-hover)', color: 'var(--text-secondary)' }}
                        >
                          {permLabel(p)}
                        </span>
                      ))}
                    </div>
                  </div>
                );
              })}
            </div>
          </Card>
        );
      })}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Members view — assign a business role to each member.
// ---------------------------------------------------------------------------
const ADMIN_OPTION = '__admin__';

function MembersView() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data: catalog } = useRbacCatalog();
  const { data: members = [], isLoading } = useRbacMembers();
  const assign = useAssignBusinessRole();

  const roleLabel = (key: string): string => {
    const r = catalog?.business_roles.find((b) => b.key === key);
    if (!r) return key;
    return lang === 'fr' ? r.label_fr : r.label_en;
  };

  const onChange = (m: MemberView, value: string) => {
    const input =
      value === ADMIN_OPTION
        ? { business_role: '', member_role: 'admin' as const }
        : { business_role: value, member_role: 'user' as const };
    assign.mutate(
      { userId: m.user_id, input },
      {
        onSuccess: () => toast.success(tr('Rôle mis à jour', 'Role updated')),
        onError: () => toast.error(tr('Échec de la mise à jour', 'Update failed')),
      }
    );
  };

  if (isLoading) return <SkeletonRows rows={5} />;
  if (members.length === 0) return <EmptyState icon={Users} title={tr('Aucun membre', 'No members')} />;

  return (
    <Card className="p-0 overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full text-[13px]" style={{ minWidth: 640 }}>
          <thead>
            <tr className="text-left text-ink-muted text-[11px] uppercase tracking-wide" style={{ borderBottom: '1px solid var(--border)' }}>
              <th className="px-4 py-3 font-semibold">{tr('Membre', 'Member')}</th>
              <th className="px-4 py-3 font-semibold">{tr("Rôle d'organisation", 'Org role')}</th>
              <th className="px-4 py-3 font-semibold">{tr('Rôle métier', 'Business role')}</th>
              <th className="px-4 py-3 font-semibold text-right">{tr('Accès', 'Access')}</th>
            </tr>
          </thead>
          <tbody>
            {members.map((m) => {
              const isOwner = m.org_role === 'root';
              const isAdmin = m.org_role === 'admin';
              const selectValue = isAdmin ? ADMIN_OPTION : m.business_role ?? '';
              const permCount = m.permissions.includes('*') ? '∞' : String(m.permissions.length);
              return (
                <tr key={m.user_id} style={{ borderBottom: '1px solid var(--border)' }}>
                  <td className="px-4 py-3">
                    <div className="font-semibold text-ink">{m.full_name || m.email}</div>
                    <div className="text-[11.5px] text-ink-muted">{m.email}</div>
                  </td>
                  <td className="px-4 py-3">
                    <RoleBadge role={m.org_role} />
                  </td>
                  <td className="px-4 py-3">
                    {isOwner ? (
                      <span className="text-[12px] text-ink-muted">{tr('Propriétaire', 'Owner')}</span>
                    ) : (
                      <select
                        value={selectValue}
                        disabled={assign.isPending}
                        onChange={(e) => onChange(m, e.target.value)}
                        className="h-8 px-2 rounded-[8px] text-[12.5px] bg-transparent"
                        style={{ border: '1px solid var(--border-strong)', color: 'var(--text-primary)' }}
                      >
                        <option value={ADMIN_OPTION}>{tr('Administrateur (accès complet)', 'Administrator (full access)')}</option>
                        <option value="">{tr('— Aucun rôle —', '— No role —')}</option>
                        {catalog?.business_roles.map((r) => (
                          <option key={r.key} value={r.key}>
                            {roleLabel(r.key)}
                          </option>
                        ))}
                      </select>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <span className="mono text-[12px] text-ink-soft">{permCount} {tr('perms', 'perms')}</span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <div className="px-4 py-2.5 text-[11.5px] text-ink-muted flex items-center gap-1.5" style={{ borderTop: '1px solid var(--border)' }}>
        <KeyRound size={13} />
        {tr(
          "Les rôles métiers ne s'appliquent qu'aux membres « utilisateur » ; les administrateurs ont un accès complet.",
          'Business roles only apply to "user" members; administrators have full access.'
        )}
      </div>
    </Card>
  );
}

function RoleBadge({ role }: { role: MemberView['org_role'] }) {
  const map: Record<string, { label: string; color: string }> = {
    root: { label: 'Root', color: 'var(--crit, #dc2626)' },
    admin: { label: 'Admin', color: 'var(--accent)' },
    user: { label: 'User', color: 'var(--text-secondary)' },
  };
  const m = map[role] ?? map.user;
  return (
    <Chip label={m.label} color={m.color} />
  );
}
