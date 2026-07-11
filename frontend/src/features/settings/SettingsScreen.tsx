// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Settings (OpenRisk.dc.html §6.16): a 210px internal nav + 8 distinct tabs —
// General, Members, RBAC, Integrations, Notifications, Security, Billing, Danger.

import { useState } from 'react';
import { Settings as SettingsIcon, Users, Lock, Plug, Siren, Shield, CreditCard, AlertTriangle, Plus, FileText, Check, Laptop, type LucideIcon } from 'lucide-react';
import { PageFrame, PageHeader, Btn, Card, Avatar } from '../../shared/ui';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';

type TabKey = 'general' | 'members' | 'rbac' | 'integrations' | 'notif' | 'security' | 'billing' | 'danger';

/* ---- local reusable bits ---- */
function Toggle({ on: initial }: { on: boolean }) {
  const [on, setOn] = useState(initial);
  return (
    <button onClick={() => setOn((v) => !v)} className="relative shrink-0" style={{ width: 42, height: 24, borderRadius: 20, background: on ? 'var(--accent)' : 'var(--bg-hover)', transition: 'background .2s' }} aria-pressed={on}>
      <span className="absolute rounded-full bg-white" style={{ width: 20, height: 20, top: 2, left: on ? 20 : 2, transition: 'left .2s', boxShadow: '0 1px 3px rgba(0,0,0,.3)' }} />
    </button>
  );
}
function ToggleRow({ label, sub, on }: { label: string; sub?: string | null; on: boolean }) {
  return (
    <div className="flex items-center justify-between gap-5 py-[15px]" style={{ borderBottom: '1px solid var(--border)' }}>
      <div className="flex-1">
        <div className="text-[13.5px] font-medium text-ink">{label}</div>
        {sub && <div className="text-[12px] text-ink-soft mt-0.5 leading-snug">{sub}</div>}
      </div>
      <Toggle on={on} />
    </div>
  );
}
function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="mb-[18px]">
      <label className="block text-[12px] font-semibold text-ink-soft mb-[7px]">{label}</label>
      <input defaultValue={value} className={`w-full h-[42px] px-3.5 rounded-[11px] text-[14px] text-ink outline-none ${mono ? 'mono' : ''}`} style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }} />
    </div>
  );
}
const Title = ({ children }: { children: React.ReactNode }) => <div className="text-[14px] font-semibold text-ink mb-3.5">{children}</div>;

export function SettingsScreen() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const [tab, setTab] = useState<TabKey>('general');

  const tabs: [TabKey, string, LucideIcon][] = [
    ['general', L.s_general, SettingsIcon], ['members', L.s_members, Users], ['rbac', L.s_rbac, Lock],
    ['integrations', L.s_integrations, Plug], ['notif', L.s_notif, Siren], ['security', L.s_security, Shield],
    ['billing', L.s_billing, CreditCard], ['danger', L.s_danger, AlertTriangle],
  ];

  return (
    <PageFrame>
      <PageHeader title={L.setTitle} />
      <div className="flex gap-7 items-start flex-col md:flex-row">
        <div className="w-full md:w-[210px] shrink-0 flex md:block gap-1 overflow-x-auto">
          {tabs.map(([k, lbl, Icon]) => (
            <button
              key={k}
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
          {tab === 'members' && <MembersTab L={L} />}
          {tab === 'rbac' && <RbacTab tr={tr} />}
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

type Tr = (fr: string, en: string) => string;

function GeneralTab({ tr }: { tr: Tr }) {
  return (
    <>
      <Card style={{ padding: '20px 22px', marginBottom: 16 }}>
        <Title>{tr('Profil de l’organisation', 'Organization profile')}</Title>
        <div className="flex items-center gap-4 mb-5">
          <div className="w-14 h-14 rounded-[14px] flex items-center justify-center text-[20px] font-bold" style={{ background: 'var(--accent-soft)', color: 'var(--accent)' }}>BA</div>
          <Btn label={tr('Changer le logo', 'Change logo')} />
        </div>
        <Field label={tr('Nom de l’organisation', 'Organization name')} value="Banque Atlantique" />
        <div className="flex gap-4">
          <div className="flex-1"><Field label={tr('Secteur', 'Industry')} value={tr('Banque & Finance', 'Banking & Finance')} /></div>
          <div className="flex-1"><Field label={tr('Fuseau horaire', 'Time zone')} value="GMT · Abidjan" /></div>
        </div>
      </Card>
      <Card style={{ padding: '20px 22px' }}>
        <Title>{tr('Préférences', 'Preferences')}</Title>
        <ToggleRow label={tr('Mode conformité stricte', 'Strict compliance mode')} sub={tr('Bloque la clôture d’un risque sans preuve documentée', 'Blocks closing a risk without documented evidence')} on />
        <ToggleRow label={tr('Recalcul automatique des scores', 'Automatic score recalculation')} sub={tr('Met à jour les scores à chaque scan d’infrastructure', 'Updates scores after each infrastructure scan')} on />
      </Card>
    </>
  );
}

function MembersTab({ L }: { L: ReturnType<typeof useUIStrings> }) {
  const mem: [string, string, string, string, string, string][] = [
    ['Amir Diallo', 'amir@banque-atlantique.ci', 'Admin', 'var(--accent)', 'active', 'AD'],
    ['Fatou Sy', 'fatou@banque-atlantique.ci', 'Analyste', 'var(--info)', 'active', 'FS'],
    ['Kofi Mensah', 'kofi@banque-atlantique.ci', 'Analyste', 'var(--info)', 'active', 'KM'],
    ['Léa Traoré', 'lea@banque-atlantique.ci', 'Lecteur', 'var(--text-muted)', 'active', 'LT'],
    ['Yasmine B.', 'yasmine@partner.io', 'Analyste', 'var(--info)', 'pending', 'YB'],
  ];
  const th = (t: string) => <th className="text-left text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-3 pb-[11px]">{t}</th>;
  return (
    <>
      <div className="flex items-center justify-between mb-4">
        <div className="text-[15px] font-semibold text-ink">{L.s_members} · {mem.length}</div>
        <Btn label={L.invite} icon={Plus} primary />
      </div>
      <Card style={{ padding: '8px 8px 0', overflow: 'hidden' }}>
        <div className="overflow-x-auto">
          <table className="w-full border-collapse" style={{ minWidth: 520 }}>
            <thead style={{ borderBottom: '1px solid var(--border)' }}><tr>{th(L.member)}{th(L.role)}{th(L.status)}{th('')}</tr></thead>
            <tbody>
              {mem.map(([name, email, role, rc, st, init]) => (
                <tr key={email} style={{ borderBottom: '1px solid var(--border)' }}>
                  <td className="px-3 py-3"><div className="flex items-center gap-2.5"><Avatar initials={init} size={32} /><div><div className="text-[13.5px] font-medium text-ink">{name}</div><div className="text-[12px] text-ink-muted">{email}</div></div></div></td>
                  <td className="px-3 py-3"><span className="text-[12px] font-semibold px-[9px] py-[3px] rounded-full" style={{ color: rc, background: `color-mix(in srgb,${rc} 14%,transparent)` }}>{role}</span></td>
                  <td className="px-3 py-3"><span className="inline-flex items-center gap-1.5 text-[12.5px] text-ink-soft"><span className="w-[7px] h-[7px] rounded-full" style={{ background: st === 'active' ? 'var(--low)' : 'var(--high)' }} />{st === 'active' ? L.active : L.pending}</span></td>
                  <td className="px-3 py-3 text-right"><button className="text-[12.5px] font-semibold" style={{ color: 'var(--critical)' }}>{L.revoke}</button></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
    </>
  );
}

function RbacTab({ tr }: { tr: Tr }) {
  const perms = [tr('Voir les risques', 'View risks'), tr('Créer / éditer', 'Create / edit'), tr('Supprimer', 'Delete'), tr('Gérer les membres', 'Manage members'), tr('Facturation', 'Billing')];
  const roles: [string, number[]][] = [['Admin', [1, 1, 1, 1, 1]], ['Analyste', [1, 1, 1, 0, 0]], ['Lecteur', [1, 0, 0, 0, 0]]];
  const Dot = ({ on }: { on: boolean }) => on
    ? <div className="w-[22px] h-[22px] rounded-[7px] inline-flex items-center justify-center" style={{ background: 'var(--accent)' }}><Check size={13} className="text-white" strokeWidth={3} /></div>
    : <div className="w-[22px] h-[22px] rounded-[7px] inline-block" style={{ border: '1.5px solid var(--border-strong)' }} />;
  return (
    <>
      <div className="flex items-center justify-between mb-4">
        <div className="text-[15px] font-semibold text-ink">{tr('Rôles & permissions', 'Roles & permissions')}</div>
        <Btn label={tr('Créer un rôle', 'Create role')} icon={Plus} primary />
      </div>
      <Card style={{ padding: '10px 16px', overflow: 'hidden' }}>
        <div className="overflow-x-auto">
          <table className="w-full border-collapse" style={{ minWidth: 460 }}>
            <thead style={{ borderBottom: '1px solid var(--border)' }}>
              <tr><th className="text-left text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-2.5 pb-3">{tr('Permission', 'Permission')}</th>{roles.map((r) => <th key={r[0]} className="text-center text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-2.5 pb-3">{r[0]}</th>)}</tr>
            </thead>
            <tbody>
              {perms.map((p, i) => (
                <tr key={p} style={{ borderTop: i ? '1px solid var(--border)' : 'none' }}>
                  <td className="px-2.5 py-3 text-[13.5px] text-ink">{p}</td>
                  {roles.map((r) => <td key={r[0]} className="px-2.5 py-3 text-center"><Dot on={!!r[1][i]} /></td>)}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
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
      <div className="text-[13px] text-ink-soft mb-4">{tr('Connectez OpenRisk à votre écosystème. Marketplace complet à venir.', 'Connect OpenRisk to your stack. Full marketplace coming soon.')}</div>
      <div className="grid gap-4" style={{ gridTemplateColumns: 'repeat(auto-fill,minmax(280px,1fr))' }}>
        {ints.map(([name, col, desc, on]) => (
          <Card key={name} style={{ padding: 18 }}>
            <div className="flex items-center gap-3 mb-3">
              <div className="w-10 h-10 rounded-[11px] flex items-center justify-center" style={{ background: `color-mix(in srgb,${col} 16%,transparent)`, color: col }}><Plug size={20} /></div>
              <div className="flex-1 text-[14px] font-semibold text-ink">{name}</div>
              <Toggle on={on} />
            </div>
            <div className="text-[12.5px] text-ink-soft leading-snug">{desc}</div>
          </Card>
        ))}
      </div>
    </>
  );
}

function NotifTab({ tr }: { tr: Tr }) {
  return (
    <>
      <Card style={{ padding: '20px 22px', marginBottom: 16 }}>
        <Title>{tr('Canaux', 'Channels')}</Title>
        <ToggleRow label="Email" sub="amir@banque-atlantique.ci" on />
        <ToggleRow label="Slack" sub="#soc-alerts" on />
        <ToggleRow label="SMS" sub={tr('Uniquement incidents critiques', 'Critical incidents only')} on={false} />
      </Card>
      <Card style={{ padding: '20px 22px' }}>
        <Title>{tr('M’alerter quand…', 'Notify me when…')}</Title>
        <ToggleRow label={tr('Un risque critique est créé', 'A critical risk is created')} on />
        <ToggleRow label={tr('Un score augmente de +10 %', 'A score rises by +10%')} on />
        <ToggleRow label={tr('Une War Room est déclenchée', 'A War Room is triggered')} on />
        <ToggleRow label={tr('Une mitigation m’est assignée', 'A mitigation is assigned to me')} on />
        <ToggleRow label={tr('Résumé hebdomadaire', 'Weekly digest')} on={false} />
      </Card>
    </>
  );
}

function SecurityTab({ tr }: { tr: Tr }) {
  const sessions: [string, string, boolean][] = [
    [tr('MacBook Pro · Abidjan', 'MacBook Pro · Abidjan'), tr('Session actuelle', 'Current session'), true],
    ['iPhone 15 · Abidjan', 'Chrome · iOS', false],
    [tr('Windows · Dakar', 'Windows · Dakar'), tr('il y a 2 jours', '2 days ago'), false],
  ];
  return (
    <>
      <Card style={{ padding: '20px 22px', marginBottom: 16 }}>
        <Title>{tr('Authentification', 'Authentication')}</Title>
        <ToggleRow label={tr('MFA obligatoire (TOTP)', 'Mandatory MFA (TOTP)')} sub={tr('Imposée à tous les membres de l’organisation', 'Enforced for all organization members')} on />
        <ToggleRow label="SSO SAML 2.0" sub={tr('Connexion via votre fournisseur d’identité', 'Sign in via your identity provider')} on />
        <ToggleRow label={tr('Expiration de session (8 h)', 'Session timeout (8 h)')} on />
      </Card>
      <Card style={{ padding: '20px 22px' }}>
        <Title>{tr('Sessions actives', 'Active sessions')}</Title>
        {sessions.map(([name, meta, cur], i) => (
          <div key={name} className="flex items-center gap-3 py-3" style={{ borderTop: i ? '1px solid var(--border)' : 'none' }}>
            <div className="w-9 h-9 rounded-[10px] flex items-center justify-center text-ink-soft shrink-0" style={{ background: 'var(--bg-hover)' }}><Laptop size={18} /></div>
            <div className="flex-1"><div className="text-[13.5px] font-medium text-ink">{name}</div><div className="text-[12px] mt-0.5" style={{ color: cur ? 'var(--low)' : 'var(--text-muted)' }}>{meta}</div></div>
            {!cur && <button className="text-[12.5px] font-semibold" style={{ color: 'var(--critical)' }}>{tr('Déconnecter', 'Revoke')}</button>}
          </div>
        ))}
      </Card>
    </>
  );
}

function BillingTab({ tr }: { tr: Tr }) {
  const usage: [string, string, number][] = [
    [tr('Membres', 'Members'), '18 / 50', 36], [tr('Actifs surveillés', 'Monitored assets'), '142 / 500', 28], [tr('Simulations / mois', 'Simulations / mo'), '23 / 100', 23],
  ];
  return (
    <>
      <Card style={{ padding: '22px 24px', marginBottom: 16 }}>
        <div className="flex items-start justify-between flex-wrap gap-3.5">
          <div>
            <div className="flex items-center gap-2.5 mb-1.5">
              <span className="disp text-[20px] font-bold text-ink">Enterprise</span>
              <span className="text-[11px] font-semibold px-[9px] py-[3px] rounded-full" style={{ color: 'var(--low)', background: 'color-mix(in srgb,var(--low) 14%,transparent)' }}>{tr('Actif', 'Active')}</span>
            </div>
            <div className="text-[13px] text-ink-soft">{tr('Facturation annuelle · renouvellement le 1 janv. 2027', 'Annual billing · renews Jan 1, 2027')}</div>
          </div>
          <div className="text-right"><span className="disp mono text-[26px] font-bold text-ink">€24k</span><span className="text-[12px] text-ink-muted">{tr('/ an', '/ yr')}</span></div>
        </div>
        <div className="flex gap-2.5 mt-[18px]">
          <Btn label={tr('Gérer le plan', 'Manage plan')} primary />
          <Btn label={tr('Voir les factures', 'View invoices')} icon={FileText} />
        </div>
      </Card>
      <Card style={{ padding: '20px 22px' }}>
        <Title>{tr('Consommation', 'Usage')}</Title>
        {usage.map(([lbl, val, pct]) => (
          <div key={lbl} className="mb-4">
            <div className="flex justify-between mb-[7px]"><span className="text-[13px] text-ink">{lbl}</span><span className="mono text-[12.5px] text-ink-soft">{val}</span></div>
            <div className="h-1.5 rounded-md overflow-hidden" style={{ background: 'var(--bg-hover)' }}><div className="h-full rounded-md" style={{ width: `${pct}%`, background: 'var(--accent)' }} /></div>
          </div>
        ))}
      </Card>
    </>
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
