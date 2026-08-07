// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Settings (OpenRisk.dc.html §6.16) — the consolidation point for every admin
// feature. Internal nav + tabs: General, Members (real /users), RBAC, API Tokens
// (real /tokens), Organizations, Audit log, Custom Fields (real /custom-fields),
// Integrations, Notifications, Security, Billing, Danger. Endpoints whose tables
// aren't migrated yet (roles/tenants/audit) degrade to an honest unavailable state.

import { useEffect, useMemo, useState } from 'react';
import { useSearchParams, useNavigate, useLocation } from 'react-router';
import { toast } from 'sonner';
import {
  Settings as SettingsIcon, Users, KeyRound, Building2, SlidersHorizontal, Plug,
  Siren, Shield, CreditCard, AlertTriangle, Plus, FileText, Check, Laptop, Trash2, Copy, Database, PowerOff,
  type LucideIcon,
} from 'lucide-react';
import { PageFrame, PageHeader, Btn, Card, SkeletonRows, EmptyState } from '../../shared/ui';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import { MembersPanel } from '../rbac/MembersPanel';
import { relTime } from '../risks/riskMap';
import { api } from '../../lib/api';
import { useTokens, useCustomFields, useTenants, type ApiToken } from './adminData';
import { useSettingsPrefs, type PrefKey } from './settingsPrefs';
import { DataTable, useTableState, type Column, type Facet, type RowAction } from '../../shared/datatable';
import { DangerConfirm } from '../../shared/DangerConfirm';
import { PersonalizeCard } from '../onboarding/PersonalizeCard';

type TabKey = 'general' | 'members' | 'tokens' | 'orgs' | 'fields' | 'integrations' | 'notif' | 'security' | 'billing' | 'danger';
type Tr = (fr: string, en: string) => string;

/* ---- reusable bits ---- */
function Toggle({ on: initial, label }: { on: boolean; label?: string }) {
  const [on, setOn] = useState(initial);
  return (
    <button onClick={() => setOn((v) => !v)} className="relative shrink-0" style={{ width: 42, height: 24, borderRadius: 20, background: on ? 'var(--accent)' : 'var(--bg-hover)', transition: 'background .2s' }} aria-pressed={on} aria-label={label}>
      <span className="absolute rounded-full bg-surface-1" style={{ width: 20, height: 20, top: 2, left: on ? 20 : 2, transition: 'left .2s', boxShadow: '0 1px 3px rgba(0,0,0,.3)' }} />
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
          <span className="absolute rounded-full bg-surface-1" style={{ width: 20, height: 20, top: 2, left: on ? 20 : 2, transition: 'left .2s', boxShadow: '0 1px 3px rgba(0,0,0,.3)' }} />
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
        description={tr('Ce module nécessite une migration de base de données (tables non provisionnées dans cet environnement).', 'This module needs a database migration (tables are not provisioned in this environment).')}
      />
    </Card>
  );
}

const TAB_KEYS: TabKey[] = ['general', 'members', 'tokens', 'orgs', 'fields', 'integrations', 'notif', 'security', 'billing', 'danger'];

export function SettingsScreen() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const tr: Tr = (fr, en) => (lang === 'fr' ? fr : en);
  // Members is a route of its own (/settings/members) because things link TO it
  // — the onboarding CTA, the access-denied screen, /roles. A tab reachable only
  // by query string is not a place you can send someone. Other tabs stay on
  // ?tab= until they earn the same need.
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const [params] = useSearchParams();
  const paramTab = params.get('tab');
  const routeTab: TabKey | null = pathname === '/settings/members' ? 'members' : null;
  const [tab, setTab] = useState<TabKey>(
    routeTab ?? (TAB_KEYS.includes(paramTab as TabKey) ? (paramTab as TabKey) : 'general'),
  );
  useEffect(() => {
    if (routeTab) { setTab(routeTab); return; }
    if (paramTab && TAB_KEYS.includes(paramTab as TabKey)) setTab(paramTab as TabKey);
  }, [paramTab, routeTab]);
  const selectTab = (k: TabKey) => {
    setTab(k);
    if (k === 'members') { navigate('/settings/members'); return; }
    // Leaving Members must leave its URL too, or the route would force the tab
    // straight back on the next render.
    const base = pathname === '/settings/members' ? '/settings' : pathname;
    navigate(`${base}?tab=${k}`, { replace: pathname !== '/settings/members' });
  };

  const tabs: [TabKey, string, LucideIcon][] = [
    ['general', L.s_general, SettingsIcon],
    // Members owns invitations AND access. They are one job — "give this person
    // the right access" — and splitting them across two screens is why "Invite a
    // member" used to land on Roles & permissions.
    ['members', L.s_members, Users],
    ['tokens', tr('Jetons API', 'API Tokens'), KeyRound],
    ['orgs', tr('Organisations', 'Organizations'), Building2],
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
              onClick={() => selectTab(k)}
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
          {tab === 'members' && <MembersPanel />}
          {tab === 'tokens' && <TokensTab tr={tr} lang={lang} />}
          {tab === 'orgs' && <OrgsTab tr={tr} />}
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

// The token prefix cell used to render a Copy glyph that copied nothing. Module
// scope keeps the handler stable so the columns memo survives.
function copyPrefix(prefix: string, tr: Tr) {
  navigator.clipboard?.writeText(prefix)
    .then(() => toast.success(tr('Préfixe copié', 'Prefix copied')))
    .catch(() => toast.error(tr('Copie impossible', 'Could not copy')));
}

function TokensTab({ tr, lang }: { tr: Tr; lang: 'fr' | 'en' }) {
  const { tokens, isLoading, isError, refetch, create, revoke } = useTokens();
  const [name, setName] = useState('');
  // Revoking a token breaks any integration using it → impact-radiography confirm.
  const [revokingToken, setRevokingToken] = useState<null | { id: string; name: string; lastUsed?: string | null }>(null);
  const table = useTableState({ defaultSort: { key: 'created', dir: 'desc' }, defaultPageSize: 25, urlPrefix: 'tok_' });

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

  const facets: Facet<ApiToken>[] = useMemo(() => [
    {
      key: 'state',
      label: tr('État', 'State'),
      single: true,
      options: [
        { value: 'active', label: tr('Actifs', 'Active'), color: 'var(--low)' },
        { value: 'revoked', label: tr('Révoqués', 'Revoked'), color: 'var(--critical)' },
      ],
      matches: (t, selected) => (selected.includes('revoked') ? !!t.revoked : !t.revoked),
    },
  ], [tr]);

  const columns: Column<ApiToken>[] = useMemo(() => [
    {
      key: 'name',
      header: tr('Nom', 'Name'),
      frozen: true,
      hideable: false,
      sortValue: (t) => t.name.toLowerCase(),
      exportValue: (t) => t.name,
      render: (t) => (
        <span className="text-[13.5px] font-medium text-ink" style={{ opacity: t.revoked ? 0.55 : 1 }}>
          {t.name}{t.revoked ? ` · ${tr('révoqué', 'revoked')}` : ''}
        </span>
      ),
    },
    {
      key: 'prefix',
      header: tr('Préfixe', 'Prefix'),
      exportValue: (t) => t.token_prefix ?? '',
      render: (t) => (t.token_prefix ? (
        <button
          type="button"
          onClick={(e) => { e.stopPropagation(); copyPrefix(t.token_prefix as string, tr); }}
          className="mono text-[12px] text-ink-soft inline-flex items-center gap-1.5 rounded px-1 -mx-1 hover:bg-hover"
          title={tr('Copier le préfixe', 'Copy prefix')}
        >
          {t.token_prefix}… <Copy size={12} className="text-ink-muted" />
        </button>
      ) : <span className="text-ink-muted text-[12px]">—</span>),
    },
    { key: 'created', header: tr('Créé', 'Created'), sortValue: (t) => new Date(t.created_at ?? 0).getTime(), exportValue: (t) => t.created_at ?? '', render: (t) => <span className="text-[12px] text-ink-soft">{relTime(t.created_at, lang)}</span> },
    { key: 'used', header: tr('Dernière util.', 'Last used'), sortValue: (t) => new Date(t.last_used_at ?? 0).getTime(), exportValue: (t) => t.last_used_at ?? '', render: (t) => <span className="text-[12px] text-ink-soft">{t.last_used_at ? relTime(t.last_used_at, lang) : tr('jamais', 'never')}</span> },
  ], [tr, lang]);

  const rowActions: RowAction<ApiToken>[] = useMemo(() => [
    {
      key: 'revoke',
      label: tr('Révoquer', 'Revoke'),
      icon: Trash2,
      danger: true,
      hidden: (t) => !!t.revoked,
      onSelect: (t) => setRevokingToken({ id: t.id, name: t.name, lastUsed: t.last_used_at }),
    },
  ], [tr]);

  return (
    <>
      <div className="flex items-center gap-2.5 mb-4 flex-wrap">
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder={tr('Nom du jeton (ex. CI/CD)', 'Token name (e.g. CI/CD)')} aria-label={tr('Nom du jeton', 'Token name')} className="flex-1 min-w-[200px] h-9 px-3.5 rounded-[10px] text-[13px] text-ink outline-none" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }} />
        <Btn label={tr('Générer un jeton', 'Generate token')} icon={Plus} primary onClick={doCreate} disabled={create.isPending} />
      </div>

      <DataTable
        id="api-tokens"
        ariaLabel={tr('Jetons API', 'API tokens')}
        rows={tokens}
        columns={columns}
        rowKey={(t) => t.id}
        api={table}
        mode="client"
        loading={isLoading}
        error={isError}
        onRetry={() => void refetch()}
        facets={facets}
        clientSearch={(t, q) => `${t.name} ${t.token_prefix ?? ''}`.toLowerCase().includes(q)}
        searchPlaceholder={tr('Nom ou préfixe…', 'Name or prefix…')}
        rowActions={rowActions}
        exportFilename="jetons-api"
        minWidth={620}
        pageSizeOptions={[10, 25, 50]}
        empty={<EmptyState icon={KeyRound} title={tr('Aucun jeton API', 'No API tokens')} description={tr('Créez un jeton pour authentifier vos intégrations et scripts.', 'Create a token to authenticate your integrations and scripts.')} />}
      />

      <DangerConfirm
        open={!!revokingToken}
        onClose={() => setRevokingToken(null)}
        title={tr('Révoquer le jeton API', 'Revoke API token')}
        subject={revokingToken?.name}
        intro={tr(
          'Toute intégration ou script utilisant ce jeton cessera immédiatement de fonctionner. Cette action est irréversible.',
          'Any integration or script using this token stops working immediately. This action is irreversible.'
        )}
        impact={revokingToken ? [
          { label: tr('Dernière utilisation', 'Last used'), value: revokingToken.lastUsed ? relTime(revokingToken.lastUsed, lang) : tr('jamais', 'never') },
        ] : []}
        confirmLabel={tr('Révoquer le jeton', 'Revoke token')}
        onConfirm={() => { if (revokingToken) revoke.mutate(revokingToken.id, { onSuccess: () => { toast.success(tr('Jeton révoqué', 'Token revoked')); setRevokingToken(null); }, onError: () => toast.error(tr('Révocation échouée — réessayez.', 'Revocation failed — retry.')) }); }}
        busy={revoke.isPending}
      />
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
        <EmptyState icon={SlidersHorizontal} title={tr('Aucun champ personnalisé', 'No custom fields')} description={tr('Ajoutez des champs sur mesure aux risques et aux actifs pour coller à votre méthodologie.', 'Add bespoke fields to risks and assets to match your methodology.')} primaryAction={<Btn label={tr('Nouveau champ', 'New field')} icon={Plus} primary onClick={() => toast(tr('Éditeur de champs — bientôt', 'Field editor — coming soon'))} />} />
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
      <Card style={{ padding: '20px 22px', marginBottom: 16 }}>
        <Title>{tr('Apparence', 'Appearance')}</Title>
        <PersonalizeCard />
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

// The danger zone shipped a "Delete organization" button wired to nothing: the
// single most destructive control in the product was a <button> with no onClick.
// It now calls DELETE /rbac/tenants/:id for real, behind an impact radiography,
// and signs the user out afterwards — there is nothing left to be signed in to.
function DangerTab({ L, tr }: { L: ReturnType<typeof useUIStrings>; tr: Tr }) {
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);
  const navigate = useNavigate();
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const tenantId = user?.tenant_id;

  const deleteOrg = async () => {
    if (!tenantId) return;
    setBusy(true);
    try {
      await api.delete(`/rbac/tenants/${tenantId}`);
      toast.success(tr('Organisation supprimée', 'Organization deleted'));
      logout();
      navigate('/login');
    } catch {
      // Owner-only on the backend; an admin who is not the owner gets a 403.
      toast.error(tr(
        'Suppression refusée — seul le propriétaire de l’organisation peut la supprimer.',
        'Deletion refused — only the organization owner can delete it.',
      ));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="rounded-[16px] p-[22px]" style={{ border: '1px solid rgba(255,69,58,.35)', background: 'color-mix(in srgb,var(--critical) 5%,transparent)' }}>
      <div className="text-[15px] font-semibold mb-1.5" style={{ color: 'var(--critical)' }}>{L.s_danger}</div>
      <div className="text-[13px] text-ink-soft mb-[18px]">{tr('Ces actions sont irréversibles.', 'These actions are irreversible.')}</div>
      <button
        onClick={() => setConfirming(true)}
        disabled={!tenantId}
        data-testid="delete-org"
        title={tenantId ? undefined : tr('Organisation inconnue — reconnectez-vous.', 'Unknown organization — sign in again.')}
        className="h-[38px] px-4 rounded-[10px] text-[13px] font-semibold disabled:opacity-50 disabled:pointer-events-none"
        style={{ border: '1px solid rgba(255,69,58,.4)', color: 'var(--critical)' }}
      >
        {tr('Supprimer l’organisation', 'Delete organization')}
      </button>

      <DangerConfirm
        open={confirming}
        onClose={() => setConfirming(false)}
        title={tr('Supprimer l’organisation ?', 'Delete this organization?')}
        subject={user?.org_name || tenantId}
        intro={tr(
          'Toute l’organisation est supprimée : risques, actifs, preuves de conformité, incidents et journaux d’audit. Les membres perdent immédiatement leur accès. Cette action est irréversible.',
          'The whole organization goes: risks, assets, compliance evidence, incidents and audit logs. Members lose access immediately. This cannot be undone.',
        )}
        impact={[
          { label: tr('Membres', 'Members'), value: tr('déconnectés immédiatement', 'signed out immediately') },
          { label: tr('Registres (risques, actifs, incidents)', 'Registers (risks, assets, incidents)'), value: tr('supprimés', 'deleted') },
          { label: tr('Piste d’audit', 'Audit trail'), value: tr('supprimée', 'deleted') },
          { label: tr('Jetons API', 'API tokens'), value: tr('invalidés', 'invalidated') },
        ]}
        alternatives={[
          {
            label: tr('Exporter vos registres avant de supprimer', 'Export your registers first'),
            description: tr('Chaque registre a un bouton « Exporter la vue » (CSV).', 'Every register has an "Export view" (CSV) button.'),
            onClick: () => { setConfirming(false); navigate('/risks'); },
          },
        ]}
        confirmLabel={tr('Supprimer définitivement l’organisation', 'Permanently delete the organization')}
        onConfirm={deleteOrg}
        busy={busy}
      />
    </div>
  );
}
