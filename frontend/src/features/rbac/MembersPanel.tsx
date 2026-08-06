// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Members & access, rendered inside Settings › Members.
//
// This was a page of its own at /settings/roles, which is how "Invite a member"
// came to land on a screen called "Roles & permissions": inviting someone and
// deciding what they can do were filed apart, even though they are one job. The
// invite lives here now, with the member list and the role assignment, and
// Settings › Members is the single place that answers "who has access".
//
// Two views remain, as a local toggle rather than two destinations:
//  - Members: the tenant's members with org role + business role, an inline
//    selector to (re)assign, and the invite action.
//  - Roles: the business-role catalog and its permission matrix — reference
//    material for the assignment, not a separate errand.
//
// ?action=invite opens the invite dialog on arrival, so anything that wants to
// send a user here to invite somebody can link straight to the action.

import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router';
import { toast } from 'sonner';
import { ShieldCheck, Users, KeyRound, UserPlus, X, Copy } from 'lucide-react';
import { Card, Chip, SkeletonRows, EmptyState } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { useRbacCatalog, useRbacMembers, useAssignBusinessRole, useInviteMember } from './useRbac';
import type { BusinessRole, PermissionDef, MemberView, InviteMemberResult } from './rbacService';
import { useOnboarding } from '../onboarding/onboardingStore';

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

// Declared at module scope: a component defined inside a render is a NEW type
// on every render, so React unmounts and remounts it, discarding focus and any
// state it holds.
function TabBtn({ id, label, tab, onSelect }: { id: Tab; label: string; tab: Tab; onSelect: (t: Tab) => void }) {
  return (
    <button
      onClick={() => onSelect(id)}
      className="h-9 px-3.5 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5"
      style={{
        background: tab === id ? 'var(--accent)' : 'transparent',
        color: tab === id ? 'var(--text-on-solid)' : 'var(--text-secondary)',
        border: tab === id ? 'none' : '1px solid var(--border-strong)',
      }}
    >
      {label}
    </button>
  );
}

export function MembersPanel() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  // Members first: it is what people come here to do. The permission matrix is
  // reference material behind it.
  const [tab, setTab] = useState<Tab>('members');

  return (
    <>
      <div className="flex gap-2 mb-4 flex-wrap">
        <TabBtn id="members" label={tr('Membres', 'Members')} tab={tab} onSelect={setTab} />
        <TabBtn id="roles" label={tr('Rôles & permissions', 'Roles & permissions')} tab={tab} onSelect={setTab} />
      </div>

      {tab === 'roles' && <RolesView />}
      {tab === 'members' && <MembersView />}
    </>
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
  // ?action=invite opens the dialog on arrival (spec §3), so anything that wants
  // to send a user here to invite somebody can link straight to the action.
  //
  // Derived from the URL rather than synced into state by an effect: the URL is
  // already the source of truth, and mirroring it into state means two things
  // that can disagree. Closing clears the param (replace, so Back does not
  // re-open it) and the local flag together.
  const [params, setParams] = useSearchParams();
  const [manualInvite, setManualInvite] = useState(false);
  const inviteOpen = manualInvite || params.get('action') === 'invite';
  const closeInvite = () => {
    setManualInvite(false);
    if (params.get('action')) {
      setParams((prev) => {
        const n = new URLSearchParams(prev);
        n.delete('action');
        return n;
      }, { replace: true });
    }
  };

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

  const inviteBar = (
    <div className="flex justify-end mb-3">
      <button
        onClick={() => setManualInvite(true)}
        data-testid="invite-member"
        className="h-9 px-3.5 rounded-[9px] text-[12.5px] font-semibold text-text-primary inline-flex items-center gap-1.5"
        style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))', boxShadow: '0 3px 12px var(--accent-glow)' }}
      >
        <UserPlus size={15} /> {tr('Inviter un membre', 'Invite a member')}
      </button>
    </div>
  );
  const modal = inviteOpen && <InviteModal catalog={catalog} onClose={closeInvite} />;

  if (isLoading)
    return (
      <>
        {inviteBar}
        <SkeletonRows rows={5} />
        {modal}
      </>
    );
  if (members.length === 0)
    return (
      <>
        {inviteBar}
        <EmptyState
          icon={Users}
          title={tr('Aucun membre', 'No members')}
          description={tr('Invitez votre premier membre pour collaborer.', 'Invite your first member to collaborate.')}
        />
        {modal}
      </>
    );

  return (
    <>
    {inviteBar}
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
    {modal}
    </>
  );
}

// ---------------------------------------------------------------------------
// Invite modal — provisions a real member (user + org membership) and shows the
// one-time temporary password to share.
// ---------------------------------------------------------------------------
function InviteModal({ catalog, onClose }: { catalog: ReturnType<typeof useRbacCatalog>['data']; onClose: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const invite = useInviteMember();
  const markInvited = useOnboarding((s) => s.markInvited);
  const [email, setEmail] = useState('');
  const [fullName, setFullName] = useState('');
  const [role, setRole] = useState(''); // '' = no preset; '__admin__' = full admin; else preset key
  const [result, setResult] = useState<InviteMemberResult | null>(null);

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const input =
      role === ADMIN_OPTION
        ? { email, full_name: fullName, member_role: 'admin' as const }
        : { email, full_name: fullName, member_role: 'user' as const, business_role: role };
    invite.mutate(input, {
      onSuccess: (res) => {
        setResult(res);
        markInvited(); // completes the onboarding "invite a teammate" step
      },
      onError: (err) => {
        const status = (err as { response?: { status?: number; data?: { error?: string } } })?.response;
        toast.error(
          status?.status === 409
            ? tr('Un compte existe déjà avec cet email.', 'An account already exists for this email.')
            : status?.data?.error || tr("L'invitation a échoué.", 'The invite failed.'),
        );
      },
    });
  };

  const copy = () => {
    if (result) navigator.clipboard?.writeText(result.temp_password);
    toast.success(tr('Copié', 'Copied'));
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'var(--surface-overlay)' }} onClick={onClose}>
      <div className="w-full max-w-[440px] rounded-[16px] p-5" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-overlay)' }} onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-4">
          <div className="text-[16px] font-bold text-ink">{tr('Inviter un membre', 'Invite a member')}</div>
          <button onClick={onClose} className="w-8 h-8 rounded-[8px] flex items-center justify-center text-ink-muted hover:bg-hover" aria-label={tr('Fermer', 'Close')}><X size={17} /></button>
        </div>

        {result ? (
          <div>
            <div className="text-[13.5px] text-ink mb-2">
              {tr('Membre créé :', 'Member created:')} <b>{result.member.email}</b>
            </div>
            <div className="text-[12.5px] text-ink-soft mb-2">
              {tr(
                'Partagez ce mot de passe temporaire avec le membre (affiché une seule fois) :',
                'Share this one-time temporary password with the member (shown only once):',
              )}
            </div>
            <div className="flex items-center gap-2 p-3 rounded-[10px] mb-4" style={{ background: 'var(--bg-hover)', border: '1px solid var(--border-strong)' }}>
              <code className="mono text-[14px] text-ink flex-1 break-all">{result.temp_password}</code>
              <button onClick={copy} className="w-8 h-8 rounded-[8px] flex items-center justify-center text-ink-muted hover:bg-hover shrink-0" aria-label={tr('Copier', 'Copy')}><Copy size={16} /></button>
            </div>
            <button onClick={onClose} className="w-full h-[42px] rounded-[10px] text-[14px] font-semibold text-text-primary" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}>{tr('Terminé', 'Done')}</button>
          </div>
        ) : (
          <form onSubmit={submit}>
            <label className="block text-[12px] font-semibold text-ink-soft mb-1.5" htmlFor="inv-name">{tr('Nom complet', 'Full name')}</label>
            <input id="inv-name" value={fullName} onChange={(e) => setFullName(e.target.value)} autoFocus className="w-full h-[42px] px-3.5 rounded-[11px] text-[14px] text-ink outline-none mb-3" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-app)' }} />
            <label className="block text-[12px] font-semibold text-ink-soft mb-1.5" htmlFor="inv-email">{tr('Email', 'Email')}</label>
            <input id="inv-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} className="w-full h-[42px] px-3.5 rounded-[11px] text-[14px] text-ink outline-none mb-3" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-app)' }} />
            <label className="block text-[12px] font-semibold text-ink-soft mb-1.5" htmlFor="inv-role">{tr('Rôle', 'Role')}</label>
            <select id="inv-role" value={role} onChange={(e) => setRole(e.target.value)} className="w-full h-[42px] px-3 rounded-[11px] text-[14px] text-ink outline-none mb-4" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-app)' }}>
              <option value="">{tr('— Aucun rôle métier —', '— No business role —')}</option>
              {catalog?.business_roles.map((r) => (
                <option key={r.key} value={r.key}>{lang === 'fr' ? r.label_fr : r.label_en}</option>
              ))}
              <option value={ADMIN_OPTION}>{tr('Administrateur (accès complet)', 'Administrator (full access)')}</option>
            </select>
            <button type="submit" disabled={invite.isPending} className="w-full h-[42px] rounded-[10px] text-[14px] font-semibold text-text-primary" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))', opacity: invite.isPending ? 0.7 : 1 }}>
              {invite.isPending ? '…' : tr('Créer le membre', 'Create member')}
            </button>
          </form>
        )}
      </div>
    </div>
  );
}

function RoleBadge({ role }: { role: MemberView['org_role'] }) {
  const map: Record<string, { label: string; color: string }> = {
    root: { label: 'Root', color: 'var(--critical)' },
    admin: { label: 'Admin', color: 'var(--accent)' },
    user: { label: 'User', color: 'var(--text-secondary)' },
  };
  const m = map[role] ?? map.user;
  return (
    <Chip label={m.label} color={m.color} />
  );
}
