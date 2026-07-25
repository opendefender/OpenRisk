// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Settings (OpenRisk.dc.html §6.16) — the consolidation point for every admin
// feature. Internal nav + tabs: General, Members (real /users), RBAC, API Tokens
// (real /tokens), Organizations, Audit log, Custom Fields (real /custom-fields),
// Integrations, Notifications, Security, Billing, Danger. Endpoints whose tables
// aren't migrated yet (roles/tenants/audit) degrade to an honest unavailable state.

import { useMemo, useState } from 'react';
import { toast } from 'sonner';
import {
  Settings as SettingsIcon, Users, Lock, KeyRound, Building2, ScrollText, SlidersHorizontal, Plug,
  Siren, Shield, CreditCard, AlertTriangle, Plus, FileText, Check, Laptop, Trash2, Copy, Database,
  type LucideIcon,
} from 'lucide-react';
import { PageFrame, PageHeader, Btn, Card, Avatar, SkeletonRows, EmptyState } from '../../shared/ui';
import { ImpactDialog } from '../../shared/ImpactDialog';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import { relTime } from '../risks/riskMap';
import { useUsers, useTokens, useCustomFields, useRoles, useAuditLogs, useTenants } from './adminData';
import { useSettingsPrefs, type PrefKey } from './settingsPrefs';

type TabKey = 'general' | 'members' | 'rbac' | 'tokens' | 'orgs' | 'audit' | 'fields' | 'integrations' | 'notif' | 'security' | 'billing' | 'danger';
type Tr = (fr: string, en: string) => string;

/* ---- reusable bits ---- */
function Toggle({ on: initial, label }: { on: boolean; label?: string }) {
  const [on, setOn] = useState(initial);
  return (
    <button onClick={() => setOn((v) => !v)} className="relative shrink-0" style={{ width: 42, height: 24, borderRadius: 20, background: on ? 'var(--accent)' : 'var(--bg-hover)', transition: 'background .2s' }} aria-pressed={on} aria-label={label}>
      <span className="absolute rounded-full bg-white" style={{ width: 20, height: 20, top: 2, left: on ? 20 : 2, transition: 'left .2s', boxShadow: '0 1px 3px rgba(0,0,0,.3)' }} />
    </button>
  );
}
function ToggleRow({ label, sub, on }: { label: string; sub?: string | null; on: boolean }) {
  return (
    <div className="flex items-center justify-between gap-5 py-[15px]" style={{ borderBottom: '1px solid var(--border)' }}>
      <div className="flex-1"><div className="text-[13.5px] font-medium text-ink">{label}</div>{sub && <div className="text-[12px] text-ink-soft mt-0.5 leading-snug">{sub}</div>}</div>
      <Toggle on={on} label={label} />
    </div>
  );
}
/** Toggle bound to a persisted preference — autosaves and shows "Saved ✓"
 *  briefly (UX-23). Survives a reload. */
function SavedToggleRow({ label, sub, prefKey, tr }: { label: string; sub?: string | null; prefKey: PrefKey; tr: Tr }) {
  const on = useSettingsPrefs((s) => s.prefs[prefKey]);
  const setPref = useSettingsPrefs((s) => s.setPref);
  const [saved, setSaved] = useState(false);
  const toggle = () => {
    setPref(prefKey, !on);
    setSaved(true);
    window.setTimeout(() => setSaved(false), 1600);
  };
  return (
    <div className="flex items-center justify-between gap-5 py-[15px]" style={{ borderBottom: '1px solid var(--border)' }}>
      <div className="flex-1"><div className="text-[13.5px] font-medium text-ink">{label}</div>{sub && <div className="text-[12px] text-ink-soft mt-0.5 leading-snug">{sub}</div>}</div>
      <div className="flex items-center gap-2.5 shrink-0">
        <span className="text-[11.5px] font-semibold transition-opacity" style={{ color: 'var(--low)', opacity: saved ? 1 : 0 }}>{tr('Enregistré ✓', 'Saved ✓')}</span>
        <button onClick={toggle} className="relative shrink-0" style={{ width: 42, height: 24, borderRadius: 20, background: on ? 'var(--accent)' : 'var(--bg-hover)', transition: 'background .2s' }} aria-pressed={on} aria-label={label}>
          <span className="absolute rounded-full bg-white" style={{ width: 20, height: 20, top: 2, left: on ? 20 : 2, transition: 'left .2s', boxShadow: '0 1px 3px rgba(0,0,0,.3)' }} />
        </button>
      </div>
    </div>
  );
}
let fieldSeq = 0;
function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  const id = useMemo(() => `field-${(fieldSeq += 1)}`, []);
  return (
    <div className="mb-[18px]">
      <label htmlFor={id} className="block text-[12px] font-semibold text-ink-soft mb-[7px]">{label}</label>
      <input id={id} aria-label={label} defaultValue={value} className={`w-full h-[42px] px-3.5 rounded-[11px] text-[14px] text-ink outline-none ${mono ? 'mono' : ''}`} style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }} />
    </div>
  );
}
const Title = ({ children }: { children: React.ReactNode }) => <div className="text-[14px] font-semibold text-ink mb-3.5">{children}</div>;

/** Honest state for endpoints whose backing tables aren't provisioned yet. */
function Unavailable({ tr }: { tr: Tr }) {
  return (
    <Card>
      <EmptyState
        icon={Database}
        title={tr('Bientôt disponible', 'Not available yet')}
        sub={tr('Ce module nécessite une migration de base de données (tables non provisionnées dans cet environnement).', 'This module needs a database migration (tables are not provisioned in this environment).')}
      />
    </Card>
  );
}

export function SettingsScreen() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const tr: Tr = (fr, en) => (lang === 'fr' ? fr : en);
  const [tab, setTab] = useState<TabKey>('general');

  const tabs: [TabKey, string, LucideIcon][] = [
    ['general', L.s_general, SettingsIcon],
    ['members', L.s_members, Users],
    ['rbac', L.s_rbac, Lock],
    ['tokens', tr('Jetons API', 'API Tokens'), KeyRound],
    ['orgs', tr('Organisations', 'Organizations'), Building2],
    ['audit', tr('Journal d’audit', 'Audit log'), ScrollText],
    ['fields', tr('Champs personnalisés', 'Custom fields'), SlidersHorizontal],
    ['integrations', L.s_integrations, Plug],
    ['notif', L.s_notif, Siren],
    ['security', L.s_security, Shield],
    ['billing', L.s_billing, CreditCard],
    ['danger', L.s_danger, AlertTriangle],
  ];

  return (
    <PageFrame>
      <PageHeader title={L.setTitle} />
      <div className="flex gap-7 items-start flex-col md:flex-row">
        <div className="w-full md:w-[210px] shrink-0 flex md:block gap-1 overflow-x-auto">
          {tabs.map(([k, lbl, Icon]) => (
            <button
              key={k}
              data-testid={`settings-tab-${k}`}
              onClick={() => setTab(k)}
              className="w-full flex items-center gap-2.5 px-3 py-[9px] rounded-[9px] mb-0.5 text-[13px] text-left whitespace-nowrap transition-colors"
              style={{ background: tab === k ? 'var(--accent-soft)' : 'transparent', color: tab === k ? 'var(--text-primary)' : 'var(--text-secondary)', fontWeight: tab === k ? 600 : 500 }}
            >
              <span style={{ color: tab === k ? 'var(--accent)' : 'var(--text-muted)' }} className="flex"><Icon size={17} /></span>
              {lbl}
            </button>
          ))}
        </div>
        <div className="flex-1 min-w-0 w-full">
          {tab === 'general' && <GeneralTab tr={tr} />}
          {tab === 'members' && <MembersTab L={L} tr={tr} lang={lang} />}
          {tab === 'rbac' && <RbacTab tr={tr} />}
          {tab === 'tokens' && <TokensTab tr={tr} lang={lang} />}
          {tab === 'orgs' && <OrgsTab tr={tr} />}
          {tab === 'audit' && <AuditTab tr={tr} lang={lang} />}
          {tab === 'fields' && <CustomFieldsTab tr={tr} />}
          {tab === 'integrations' && <IntegrationsTab tr={tr} />}
          {tab === 'notif' && <NotifTab tr={tr} />}
          {tab === 'security' && <SecurityTab tr={tr} />}
          {tab === 'billing' && <BillingTab tr={tr} />}
          {tab === 'danger' && <DangerTab L={L} tr={tr} />}
        </div>
      </div>
    </PageFrame>
  );
}

/* ==================== real tabs ==================== */

function MembersTab({ L, tr, lang }: { L: ReturnType<typeof useUIStrings>; tr: Tr; lang: 'fr' | 'en' }) {
  const { users, isLoading, isError, setStatus, remove } = useUsers();
  const roleColor = (r: string) => (r === 'admin' || r === 'root' ? 'var(--accent)' : r ? 'var(--info)' : 'var(--text-muted)');
  const th = (t: string) => <th className="text-left text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-3 pb-[11px]">{t}</th>;

  // Revoking a member is important + irreversible → impact radiography (UX-11) with a
  // non-destructive escape hatch (deactivate keeps the account + their ownership).
  const [toRevoke, setToRevoke] = useState<{ id: string; name: string; active: boolean } | null>(null);
  const confirmRevoke = () => {
    if (!toRevoke) return;
    const id = toRevoke.id;
    setToRevoke(null);
    remove.mutate(id, {
      onSuccess: () => toast.success(tr('Membre révoqué', 'Member revoked')),
      onError: () => toast.error(tr('Action échouée', 'Action failed')),
    });
  };
  const deactivateInstead = () => {
    if (!toRevoke) return;
    const id = toRevoke.id;
    setToRevoke(null);
    setStatus.mutate({ id, is_active: false }, {
      onSuccess: () => toast.success(tr('Membre désactivé', 'Member deactivated')),
      onError: () => toast.error(tr('Action échouée', 'Action failed')),
    });
  };
  const toggle = (id: string, active: boolean) =>
    setStatus.mutate({ id, is_active: !active }, { onError: () => toast.error(tr('Action échouée', 'Action failed')) });

  return (
    <>
      <div className="flex items-center justify-between mb-4">
        <div className="text-[15px] font-semibold text-ink">{L.s_members} · {users.length}</div>
        <Btn label={L.invite} icon={Plus} primary onClick={() => toast(tr('Invitation par e-mail — bientôt', 'Email invites — coming soon'))} />
      </div>
      <Card style={{ padding: '8px 8px 0', overflow: 'hidden' }}>
        {isLoading ? (
          <SkeletonRows rows={4} />
        ) : isError ? (
          <EmptyState icon={Users} title={tr('Membres indisponibles', 'Members unavailable')} sub={tr('Impossible de charger les membres.', 'Could not load members.')} />
        ) : users.length === 0 ? (
          <EmptyState icon={Users} title={tr('Aucun membre', 'No members')} />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full border-collapse" style={{ minWidth: 560 }}>
              <thead style={{ borderBottom: '1px solid var(--border)' }}><tr>{th(L.member)}{th(L.role)}{th(L.status)}{th('')}</tr></thead>
              <tbody>
                {users.map((u) => (
                  <tr key={u.id} style={{ borderBottom: '1px solid var(--border)' }}>
                    <td className="px-3 py-3">
                      <div className="flex items-center gap-2.5">
                        <Avatar initials={(u.full_name || u.email).slice(0, 2).toUpperCase()} size={32} />
                        <div><div className="text-[13.5px] font-medium text-ink">{u.full_name || u.username}</div><div className="text-[12px] text-ink-muted">{u.email}</div></div>
                      </div>
                    </td>
                    <td className="px-3 py-3"><span className="text-[12px] font-semibold px-[9px] py-[3px] rounded-full capitalize" style={{ color: roleColor(u.role), background: `color-mix(in srgb,${roleColor(u.role)} 14%,transparent)` }}>{u.role || tr('—', '—')}</span></td>
                    <td className="px-3 py-3">
                      <button onClick={() => toggle(u.id, u.is_active)} className="inline-flex items-center gap-1.5 text-[12.5px] text-ink-soft hover:text-ink transition-colors">
                        <span className="w-[7px] h-[7px] rounded-full" style={{ background: u.is_active ? 'var(--low)' : 'var(--text-muted)' }} />{u.is_active ? L.active : tr('Inactif', 'Inactive')}
                      </button>
                    </td>
                    <td className="px-3 py-3 text-right"><button onClick={() => setToRevoke({ id: u.id, name: u.full_name || u.email, active: u.is_active })} className="text-[12.5px] font-semibold" style={{ color: 'var(--critical)' }}>{L.revoke}</button></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
      <div className="text-[11.5px] text-ink-muted mt-2.5">{tr('Astuce : cliquez sur le statut pour activer / désactiver un membre.', 'Tip: click a status to enable / disable a member.')}</div>

      <ImpactDialog
        open={!!toRevoke}
        title={tr('Révoquer ce membre ?', 'Revoke this member?')}
        subject={toRevoke?.name ?? ''}
        description={tr('Action irréversible. Voici ce qui se passe :', 'This cannot be undone. Here is what happens:')}
        impacts={[
          { label: tr('Perte immédiate de tout accès', 'Loses all access immediately') },
          { label: tr('Actifs & risques dont il est responsable → sans responsable', 'Assets & risks they own → left unassigned') },
        ]}
        alternatives={toRevoke?.active ? [
          {
            label: tr('Désactiver le compte au lieu de révoquer', 'Deactivate the account instead'),
            description: tr('Coupe l’accès mais garde le membre et ses responsabilités — réversible.', 'Cuts access but keeps the member and their ownership — reversible.'),
            onClick: deactivateInstead,
          },
        ] : []}
        confirmLabel={tr('Révoquer définitivement', 'Revoke permanently')}
        cancelLabel={tr('Annuler', 'Cancel')}
        loading={remove.isPending}
        onConfirm={confirmRevoke}
        onClose={() => setToRevoke(null)}
      />
    </>
  );
}

function TokensTab({ tr, lang }: { tr: Tr; lang: 'fr' | 'en' }) {
  const { tokens, isLoading, isError, create, revoke } = useTokens();
  const [name, setName] = useState('');

  const doCreate = () => {
    const n = name.trim() || tr('Nouveau jeton', 'New token');
    create.mutate(n, {
      onSuccess: (res) => {
        setName('');
        const secret = res.data?.token;
        if (secret) { navigator.clipboard?.writeText(secret).catch(() => {}); toast.success(tr('Jeton créé et copié dans le presse-papiers', 'Token created and copied to clipboard')); }
        else toast.success(tr('Jeton créé', 'Token created'));
      },
      onError: () => toast.error(tr('Création échouée', 'Creation failed')),
    });
  };

  return (
    <>
      <div className="flex items-center gap-2.5 mb-4 flex-wrap">
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder={tr('Nom du jeton (ex. CI/CD)', 'Token name (e.g. CI/CD)')} className="flex-1 min-w-[200px] h-9 px-3.5 rounded-[10px] text-[13px] text-ink outline-none" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }} />
        <Btn label={tr('Générer un jeton', 'Generate token')} icon={Plus} primary onClick={doCreate} />
      </div>
      <Card style={{ padding: '8px 8px 0', overflow: 'hidden' }}>
        {isLoading ? (
          <SkeletonRows rows={3} />
        ) : isError ? (
          <EmptyState icon={KeyRound} title={tr('Jetons indisponibles', 'Tokens unavailable')} />
        ) : tokens.length === 0 ? (
          <EmptyState icon={KeyRound} title={tr('Aucun jeton API', 'No API tokens')} sub={tr('Créez un jeton pour authentifier vos intégrations et scripts.', 'Create a token to authenticate your integrations and scripts.')} />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full border-collapse" style={{ minWidth: 560 }}>
              <thead style={{ borderBottom: '1px solid var(--border)' }}>
                <tr>{[tr('Nom', 'Name'), tr('Préfixe', 'Prefix'), tr('Créé', 'Created'), tr('Dernière util.', 'Last used'), ''].map((t, i) => <th key={i} className="text-left text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-3 pb-[11px]">{t}</th>)}</tr>
              </thead>
              <tbody>
                {tokens.map((t) => (
                  <tr key={t.id} style={{ borderBottom: '1px solid var(--border)', opacity: t.revoked ? 0.5 : 1 }}>
                    <td className="px-3 py-3 text-[13.5px] font-medium text-ink">{t.name}</td>
                    <td className="px-3 py-3"><span className="mono text-[12px] text-ink-soft inline-flex items-center gap-1.5">{t.token_prefix ? `${t.token_prefix}…` : '—'}{t.token_prefix && <Copy size={12} className="text-ink-muted" />}</span></td>
                    <td className="px-3 py-3 text-[12px] text-ink-soft">{relTime(t.created_at, lang)}</td>
                    <td className="px-3 py-3 text-[12px] text-ink-soft">{t.last_used_at ? relTime(t.last_used_at, lang) : tr('jamais', 'never')}</td>
                    <td className="px-3 py-3 text-right">
                      {!t.revoked && <button onClick={() => revoke.mutate(t.id, { onSuccess: () => toast.success(tr('Jeton révoqué', 'Token revoked')) })} className="inline-flex items-center gap-1 text-[12.5px] font-semibold" style={{ color: 'var(--critical)' }}><Trash2 size={13} /> {tr('Révoquer', 'Revoke')}</button>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </>
  );
}

function CustomFieldsTab({ tr }: { tr: Tr }) {
  const { fields, isLoading, isError } = useCustomFields();
  if (isError) return <Unavailable tr={tr} />;
  return (
    <Card style={{ padding: '8px 8px 0', overflow: 'hidden' }}>
      {isLoading ? (
        <SkeletonRows rows={3} />
      ) : fields.length === 0 ? (
        <EmptyState icon={SlidersHorizontal} title={tr('Aucun champ personnalisé', 'No custom fields')} sub={tr('Ajoutez des champs sur mesure aux risques et aux actifs pour coller à votre méthodologie.', 'Add bespoke fields to risks and assets to match your methodology.')} cta={<Btn label={tr('Nouveau champ', 'New field')} icon={Plus} primary onClick={() => toast(tr('Éditeur de champs — bientôt', 'Field editor — coming soon'))} />} />
      ) : (
        <div className="p-3 flex flex-col gap-2">
          {fields.map((f) => (
            <div key={f.id} className="flex items-center gap-3 px-3 py-2.5 rounded-[10px]" style={{ border: '1px solid var(--border)' }}>
              <SlidersHorizontal size={16} className="text-ink-muted" />
              <div className="flex-1"><div className="text-[13.5px] font-medium text-ink">{f.label || f.name}</div><div className="text-[11.5px] text-ink-muted">{f.field_type} · {f.entity_type}</div></div>
              {f.required && <span className="text-[11px] font-semibold" style={{ color: 'var(--high)' }}>{tr('requis', 'required')}</span>}
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

function AuditTab({ tr, lang }: { tr: Tr; lang: 'fr' | 'en' }) {
  const { logs, isLoading, isError } = useAuditLogs();
  if (isError) return <Unavailable tr={tr} />;
  return (
    <Card style={{ padding: '8px 14px' }}>
      {isLoading ? <SkeletonRows rows={5} /> : logs.length === 0 ? (
        <EmptyState icon={ScrollText} title={tr('Journal vide', 'No audit entries')} />
      ) : (
        logs.slice(0, 50).map((e, i) => (
          <div key={e.id ?? i} className="flex items-center gap-3 py-2.5 px-1" style={{ borderTop: i ? '1px solid var(--border)' : 'none' }}>
            <span className="w-[7px] h-[7px] rounded-full shrink-0" style={{ background: 'var(--accent)' }} />
            <div className="flex-1 min-w-0"><div className="text-[13px] text-ink">{e.action ?? '—'} <span className="text-ink-muted">· {e.resource ?? ''}</span></div><div className="text-[11.5px] text-ink-muted">{e.actor ?? e.user_email ?? ''}</div></div>
            <span className="text-[11.5px] text-ink-muted">{relTime(e.created_at ?? e.timestamp, lang)}</span>
          </div>
        ))
      )}
    </Card>
  );
}

function OrgsTab({ tr }: { tr: Tr }) {
  const { tenants, isLoading, isError } = useTenants();
  if (isError) return <Unavailable tr={tr} />;
  return (
    <Card style={{ padding: '8px 8px 0', overflow: 'hidden' }}>
      {isLoading ? <SkeletonRows rows={3} /> : tenants.length === 0 ? (
        <EmptyState icon={Building2} title={tr('Aucune organisation', 'No organizations')} />
      ) : (
        <div className="p-3 flex flex-col gap-2">
          {tenants.map((t, i) => (
            <div key={t.id ?? i} className="flex items-center gap-3 px-3 py-2.5 rounded-[10px]" style={{ border: '1px solid var(--border)' }}>
              <div className="w-8 h-8 rounded-[9px] flex items-center justify-center text-[11px] font-bold" style={{ background: 'var(--accent-soft)', color: 'var(--accent)' }}>{(t.name ?? '?').slice(0, 2).toUpperCase()}</div>
              <div className="flex-1"><div className="text-[13.5px] font-medium text-ink">{t.name}</div><div className="mono text-[11.5px] text-ink-muted">{t.slug ?? t.id}</div></div>
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

function RbacTab({ tr }: { tr: Tr }) {
  const { roles, isLoading } = useRoles();
  const levelColor = (lvl: number) => (lvl >= 9 ? 'var(--accent)' : lvl >= 6 ? 'var(--high)' : lvl >= 3 ? 'var(--info)' : 'var(--text-muted)');
  const perms = [tr('Voir les risques', 'View risks'), tr('Créer / éditer', 'Create / edit'), tr('Supprimer', 'Delete'), tr('Gérer les membres', 'Manage members'), tr('Facturation', 'Billing')];
  const stdRoles: [string, number[]][] = [['Admin', [1, 1, 1, 1, 1]], ['Analyste', [1, 1, 1, 0, 0]], ['Lecteur', [1, 0, 0, 0, 0]]];
  const Dot = ({ on }: { on: boolean }) => on
    ? <div className="w-[22px] h-[22px] rounded-[7px] inline-flex items-center justify-center" style={{ background: 'var(--accent)' }}><Check size={13} className="text-white" strokeWidth={3} /></div>
    : <div className="w-[22px] h-[22px] rounded-[7px] inline-block" style={{ border: '1.5px solid var(--border-strong)' }} />;
  return (
    <>
      <div className="flex items-center justify-between mb-4">
        <div className="text-[15px] font-semibold text-ink">{tr('Rôles & permissions', 'Roles & permissions')}</div>
        <Btn label={tr('Créer un rôle', 'Create role')} icon={Plus} primary onClick={() => toast(tr('Éditeur de rôles — bientôt', 'Role editor — coming soon'))} />
      </div>

      {/* real roles from /rbac/roles */}
      <Card style={{ padding: 8, marginBottom: 16 }}>
        {isLoading ? (
          <SkeletonRows rows={4} />
        ) : roles.length === 0 ? (
          <EmptyState icon={Lock} title={tr('Aucun rôle', 'No roles')} />
        ) : (
          <div className="flex flex-col gap-1.5 p-1.5">
            {roles.map((r) => (
              <div key={r.id} className="flex items-center gap-3 px-3 py-2.5 rounded-[10px]" style={{ border: '1px solid var(--border)' }}>
                <div className="w-8 h-8 rounded-[9px] flex items-center justify-center shrink-0" style={{ background: `color-mix(in srgb,${levelColor(r.level)} 14%,transparent)`, color: levelColor(r.level) }}><Lock size={16} /></div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-[13.5px] font-semibold text-ink">{r.name}</span>
                    {r.is_predefined && <span className="text-[10px] font-semibold px-1.5 py-0.5 rounded uppercase tracking-[.04em]" style={{ color: 'var(--text-muted)', background: 'var(--bg-hover)' }}>{tr('système', 'system')}</span>}
                  </div>
                  <div className="text-[12px] text-ink-muted truncate">{r.description}</div>
                </div>
                <span className="text-[11.5px] font-semibold px-2 py-[3px] rounded-full shrink-0" style={{ color: levelColor(r.level), background: `color-mix(in srgb,${levelColor(r.level)} 14%,transparent)` }}>{tr('Niveau', 'Level')} {r.level}</span>
              </div>
            ))}
          </div>
        )}
      </Card>

      <div className="text-[12px] font-semibold text-ink-soft mb-2 px-1">{tr('Matrice de permissions', 'Permission matrix')}</div>
      <Card style={{ padding: '10px 16px', overflow: 'hidden' }}>
        <div className="overflow-x-auto">
          <table className="w-full border-collapse" style={{ minWidth: 460 }}>
            <thead style={{ borderBottom: '1px solid var(--border)' }}>
              <tr><th className="text-left text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-2.5 pb-3">{tr('Permission', 'Permission')}</th>{stdRoles.map((r) => <th key={r[0]} className="text-center text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-2.5 pb-3">{r[0]}</th>)}</tr>
            </thead>
            <tbody>
              {perms.map((p, i) => (
                <tr key={p} style={{ borderTop: i ? '1px solid var(--border)' : 'none' }}>
                  <td className="px-2.5 py-3 text-[13.5px] text-ink">{p}</td>
                  {stdRoles.map((r) => <td key={r[0]} className="px-2.5 py-3 text-center"><Dot on={!!r[1][i]} /></td>)}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
    </>
  );
}

/* ==================== static tabs ==================== */

function GeneralTab({ tr }: { tr: Tr }) {
  const user = useAuthStore((s) => s.user);
  const orgName = user?.org_name?.trim() || tr('Mon organisation', 'My organization');
  const orgInitials =
    orgName
      .split(/\s+/)
      .map((w) => w[0])
      .slice(0, 2)
      .join('')
      .toUpperCase() || 'OR';
  const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  return (
    <>
      <Card style={{ padding: '20px 22px', marginBottom: 16 }}>
        <Title>{tr('Profil de l’organisation', 'Organization profile')}</Title>
        <div className="flex items-center gap-4 mb-5">
          <div className="w-14 h-14 rounded-[14px] flex items-center justify-center text-[20px] font-bold" style={{ background: 'var(--accent-soft)', color: 'var(--accent)' }}>{orgInitials}</div>
          <Btn label={tr('Changer le logo', 'Change logo')} />
        </div>
        <Field label={tr('Nom de l’organisation', 'Organization name')} value={orgName} />
        <Field label={tr('Fuseau horaire', 'Time zone')} value={tz} />
      </Card>
      <Card style={{ padding: '20px 22px' }}>
        <Title>{tr('Préférences', 'Preferences')}</Title>
        <SavedToggleRow prefKey="strict_compliance" tr={tr} label={tr('Mode conformité stricte', 'Strict compliance mode')} sub={tr('Bloque la clôture d’un risque sans preuve documentée', 'Blocks closing a risk without documented evidence')} />
        <SavedToggleRow prefKey="auto_recalc" tr={tr} label={tr('Recalcul automatique des scores', 'Automatic score recalculation')} sub={tr('Met à jour les scores à chaque scan d’infrastructure', 'Updates scores after each infrastructure scan')} />
      </Card>
    </>
  );
}

function IntegrationsTab({ tr }: { tr: Tr }) {
  const ints: [string, string, string, boolean][] = [
    ['Slack', 'var(--info)', tr('Alertes incidents dans vos canaux', 'Incident alerts to your channels'), true],
    ['Microsoft Teams', '#7c6cff', tr('Notifications & rapports', 'Notifications & reports'), true],
    ['Jira', '#0a84ff', tr('Créer des tickets depuis un risque', 'Create tickets from a risk'), false],
    ['ServiceNow', 'var(--low)', tr('Synchronisation CMDB', 'CMDB synchronization'), false],
    ['Splunk', 'var(--high)', tr('Ingestion des logs SIEM', 'SIEM log ingestion'), true],
    ['Webhook', 'var(--text-secondary)', tr('Événements sortants personnalisés', 'Custom outbound events'), false],
  ];
  return (
    <>
      <div className="text-[13px] text-ink-soft mb-4">{tr('Connectez OpenRisk à votre écosystème (marketplace complet à venir).', 'Connect OpenRisk to your stack (full marketplace coming soon).')}</div>
      <div className="grid gap-4" style={{ gridTemplateColumns: 'repeat(auto-fill,minmax(280px,1fr))' }}>
        {ints.map(([name, col, desc, on]) => (
          <Card key={name} style={{ padding: 18 }}>
            <div className="flex items-center gap-3 mb-3">
              <div className="w-10 h-10 rounded-[11px] flex items-center justify-center" style={{ background: `color-mix(in srgb,${col} 16%,transparent)`, color: col }}><Plug size={20} /></div>
              <div className="flex-1 text-[14px] font-semibold text-ink">{name}</div>
              <Toggle on={on} label={name} />
            </div>
            <div className="text-[12.5px] text-ink-soft leading-snug">{desc}</div>
          </Card>
        ))}
      </div>
    </>
  );
}

function NotifTab({ tr }: { tr: Tr }) {
  const user = useAuthStore((s) => s.user);
  return (
    <>
      <Card style={{ padding: '20px 22px', marginBottom: 16 }}>
        <Title>{tr('Canaux', 'Channels')}</Title>
        <SavedToggleRow prefKey="notif_email" tr={tr} label="Email" sub={user?.email || tr('Adresse du compte', 'Account address')} />
        <SavedToggleRow prefKey="notif_slack" tr={tr} label="Slack" sub={tr('Non configuré — connectez Slack dans Intégrations', 'Not configured — connect Slack under Integrations')} />
        <SavedToggleRow prefKey="notif_sms" tr={tr} label="SMS" sub={tr('Uniquement incidents critiques', 'Critical incidents only')} />
      </Card>
      <Card style={{ padding: '20px 22px' }}>
        <Title>{tr('M’alerter quand…', 'Notify me when…')}</Title>
        <SavedToggleRow prefKey="alert_critical_risk" tr={tr} label={tr('Un risque critique est créé', 'A critical risk is created')} />
        <SavedToggleRow prefKey="alert_score_up" tr={tr} label={tr('Un score augmente de +10 %', 'A score rises by +10%')} />
        <SavedToggleRow prefKey="alert_warroom" tr={tr} label={tr('Une War Room est déclenchée', 'A War Room is triggered')} />
        <SavedToggleRow prefKey="alert_mitigation" tr={tr} label={tr('Une mitigation m’est assignée', 'A mitigation is assigned to me')} />
        <SavedToggleRow prefKey="alert_digest" tr={tr} label={tr('Résumé hebdomadaire', 'Weekly digest')} />
      </Card>
    </>
  );
}

// Minimal, honest browser label from the UA — no invented device history.
function currentDeviceLabel(): string {
  if (typeof navigator === 'undefined') return 'Session';
  const ua = navigator.userAgent;
  const browser = /Edg/.test(ua) ? 'Edge' : /Chrome/.test(ua) ? 'Chrome' : /Firefox/.test(ua) ? 'Firefox' : /Safari/.test(ua) ? 'Safari' : 'Browser';
  const os = /Windows/.test(ua) ? 'Windows' : /Mac/.test(ua) ? 'macOS' : /Linux/.test(ua) ? 'Linux' : /Android/.test(ua) ? 'Android' : /iPhone|iPad/.test(ua) ? 'iOS' : '';
  return os ? `${browser} · ${os}` : browser;
}

function SecurityTab({ tr }: { tr: Tr }) {
  const user = useAuthStore((s) => s.user);
  return (
    <>
      <Card style={{ padding: '20px 22px', marginBottom: 16 }}>
        <Title>{tr('Authentification', 'Authentication')}</Title>
        <div className="text-[13px] text-ink-soft leading-relaxed">
          {tr(
            'La MFA (TOTP) et les codes de secours sont disponibles par utilisateur (POST /auth/mfa). La politique MFA/SSO au niveau de l’organisation est configurée par un administrateur — sa gestion depuis cet écran arrive prochainement.',
            'MFA (TOTP) and backup codes are available per user (POST /auth/mfa). Organization-wide MFA/SSO policy is configured by an administrator — managing it from this screen is coming soon.'
          )}
        </div>
      </Card>
      <Card style={{ padding: '20px 22px' }}>
        <Title>{tr('Session active', 'Active session')}</Title>
        <div className="flex items-center gap-3 py-2">
          <div className="w-9 h-9 rounded-[10px] flex items-center justify-center text-ink-soft shrink-0" style={{ background: 'var(--bg-hover)' }}><Laptop size={18} /></div>
          <div className="flex-1">
            <div className="text-[13.5px] font-medium text-ink">{currentDeviceLabel()}</div>
            <div className="text-[12px] mt-0.5" style={{ color: 'var(--low)' }}>{tr('Session actuelle', 'Current session')} · {user?.email}</div>
          </div>
        </div>
        <div className="text-[11.5px] text-ink-muted mt-1.5">{tr('L’historique multi-appareils arrivera avec la gestion des sessions.', 'Multi-device history will arrive with session management.')}</div>
      </Card>
    </>
  );
}

function BillingTab({ tr }: { tr: Tr }) {
  return (
    <Card style={{ padding: '22px 24px' }}>
      <div className="flex items-center gap-2.5 mb-1.5">
        <span className="disp text-[20px] font-bold text-ink">{tr('Édition open-source', 'Open-source edition')}</span>
        <span className="text-[11px] font-semibold px-[9px] py-[3px] rounded-full" style={{ color: 'var(--low)', background: 'color-mix(in srgb,var(--low) 14%,transparent)' }}>AGPL-3.0</span>
      </div>
      <div className="text-[13px] text-ink-soft leading-relaxed max-w-[60ch]">
        {tr(
          'Cette instance est l’édition communautaire OpenRisk — aucune facturation n’est configurée. Les offres managées et le suivi d’usage apparaîtront ici lorsqu’ils seront disponibles.',
          'This instance runs the OpenRisk community edition — no billing is configured. Managed plans and usage metering will appear here when available.'
        )}
      </div>
    </Card>
  );
}

function DangerTab({ L, tr }: { L: ReturnType<typeof useUIStrings>; tr: Tr }) {
  return (
    <div className="rounded-[16px] p-[22px]" style={{ border: '1px solid rgba(255,69,58,.35)', background: 'color-mix(in srgb,var(--critical) 5%,transparent)' }}>
      <div className="text-[15px] font-semibold mb-1.5" style={{ color: 'var(--critical)' }}>{L.s_danger}</div>
      <div className="text-[13px] text-ink-soft mb-[18px]">{tr('Ces actions sont irréversibles.', 'These actions are irreversible.')}</div>
      <button className="h-[38px] px-4 rounded-[10px] text-[13px] font-semibold" style={{ border: '1px solid rgba(255,69,58,.4)', color: 'var(--critical)' }}>{tr('Supprimer l’organisation', 'Delete organization')}</button>
    </div>
  );
}
