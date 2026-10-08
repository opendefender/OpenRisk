// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useEffect, useMemo, useRef, useState } from 'react';
import { Link, useNavigate, useLocation } from 'react-router';
import {
  PanelLeftClose,
  PanelLeftOpen,
  Plus,
  Settings,
  LogOut,
  UserRound,
} from 'lucide-react';
import { cn } from '../../shared/ds';
import { useUIStore } from '../../store/uiStore';
import { useUIStrings } from '../../shared/uiStrings';
import { useAuthStore } from '../../hooks/useAuthStore';
import { usePermissions } from '../../hooks/usePermissions';
import { SidebarRoleLabel } from './SidebarRoleLabel';
import { OpenRiskLogo } from '../../shared/Logo';
import { OrgSwitcher } from './OrgSwitcher';
import {
  visibleNavGroups,
  pinnedItems,
  ALL_NAV_ITEMS,
  type NavItem,
  type NavBadgeTone,
} from '../../shared/navModel';
import { useScore } from '../../hooks/useScore';
import { useOrganizationBranding } from '../../features/organization/useOrganization';
import { useNavCounts } from './useNavCounts';
import { bandColor, bandLabel, bandTextColor } from '../../services/scoreService';
import { UserAvatar } from '../../shared/UserAvatar';
import { useMyProfile } from '../../features/profile/useProfile';
import { OrgLogo } from '../../features/organization/OrgLogo';
import { useNavGlide } from './useNavGlide';

// Badge colours from the redesign: a count of work is neutral, exposure is
// danger, something waiting on someone else is info.
const BADGE_TONE: Record<NavBadgeTone, { bg: string; fg: string }> = {
  neutral: { bg: 'var(--surface-3)', fg: 'var(--fg-secondary)' },
  danger: { bg: 'var(--danger-surface)', fg: 'var(--danger-text)' },
  info: { bg: 'var(--info-surface)', fg: 'var(--info-text)' },
};

interface SidebarProps {
  /** Off-canvas drawer open on mobile (< lg). Ignored on desktop, where the
   *  sidebar is always in the layout flow. */
  mobileOpen?: boolean;
  onMobileClose?: () => void;
}

function initials(name?: string, fallback = 'AD'): string {
  if (!name?.trim()) return fallback;
  const parts = name.trim().split(/\s+/);
  return ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '')).toUpperCase() || fallback;
}

export const Sidebar = ({ mobileOpen = false, onMobileClose }: SidebarProps) => {
  const collapsed = useUIStore((s) => s.sidebarCollapsed);
  const toggleCollapse = useUIStore((s) => s.toggleSidebar);
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const navigate = useNavigate();
  const { pathname, search } = useLocation();
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);
  // The avatar comes from the profile, the same query PreferencesSync reads.
  const { data: myProfile } = useMyProfile(Boolean(user));
  const { can, isAdmin } = usePermissions();
  const [menuOpen, setMenuOpen] = useState(false);
  // One hover backdrop that glides between nav entries (#751).
  const glideRef = useRef<HTMLDivElement>(null);
  const glide = useNavGlide(glideRef);
  // Real org identity + posture — replaces the former hardcoded fixtures.
  // Branding (#718) is readable by every member; the login's org_name is the
  // fallback while it loads.
  const { data: branding } = useOrganizationBranding(Boolean(user));
  const orgName =
    branding?.name?.trim() || user?.org_name?.trim() || tr('Mon organisation', 'My organization');
  // The canonical tenant score — the SAME query key the dashboard hero and the
  // dedicated page use, so all three render one object from one fetch. The
  // sidebar used to read cyber_score off the executive dashboard while the hero
  // read stats.global_risk_score: two quantities, two scales, opposite
  // directions, both labelled "score".
  const { data: tenantScore } = useScore('tenant');

  // Role-aware navigation: only surface screens the member can actually reach
  // (same permission gates the API enforces), so each business role sees a menu
  // coherent with its job.
  const navGroups = useMemo(
    () => visibleNavGroups(can, isAdmin()),
    // `can`/`isAdmin` are stable per permission set (memoized in usePermissions).
    [can, isAdmin],
  );
  // Dashboard (and any future pinned entry) is hoisted above the intention groups.
  const pinned = useMemo(() => pinnedItems(navGroups), [navGroups]);

  const handleLogout = () => {
    setMenuOpen(false);
    logout();
    navigate('/login', { replace: true });
  };

  // Close the mobile drawer whenever the route changes.
  useEffect(() => {
    onMobileClose?.();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pathname]);

  // Longest-prefix match so /assets/universe highlights Universe, not Inventory.
  const activeKey = useMemo(() => {
    let best = '';
    let bestLen = -1;
    for (const it of ALL_NAV_ITEMS) {
      const p = it.path;
      // Items sharing a pathname (Dashboard vs Executive, both "/") are told
      // apart by ?view=; an item without a `view` is active only when the param
      // is absent, so the two never light up together.
      const viewParam = new URLSearchParams(search).get('view');
      if ((it.view ?? null) !== (viewParam || null)) continue;
      const match = p === '/' ? pathname === '/' : pathname === p || pathname.startsWith(p + '/');
      if (match && p.length > bestLen) {
        best = it.key;
        bestLen = p.length;
      }
    }
    return best;
  }, [pathname, search]);

  // Posture footer. No thresholds live here any more: the band comes from the
  // server alongside the value, and bandColor maps a BAND (never a number) to a
  // token — so this cannot disagree with the server about where a cut lies.
  // An unmeasured tenant (#287) gets no number at all — not 0, not a default.
  const measuredScore = tenantScore?.measured ? tenantScore : undefined;
  const score = measuredScore ? Math.round(measuredScore.value) : undefined;
  const scoreColor = bandColor(measuredScore?.band);
  // The bar uses the fill colour; the number is small text and needs the
  // text-weight token (see bandTextColor).
  const scoreTextColor = bandTextColor(measuredScore?.band);

  // Live, tenant-scoped counters for the nav badges. A failed or refused read
  // leaves every count at zero, which renders no badge at all — the honest
  // outcome, since we do not know the number.
  const navCounts = useNavCounts(can);

  const navItem = (item: NavItem) => {
    const active = item.key === activeKey;
    const Icon = item.icon;
    return (
      // A LINK, not a button. Every nav entry used to be `<button
      // onClick={navigate(...)}>`, which renders no href — so Ctrl/Cmd+click,
      // middle-click and "copy link address" all did nothing, and a screen
      // reader announced 27 buttons where a user expects a list of links.
      //
      // A risk manager comparing two registers side by side needs a second tab,
      // and there was no way to open one. `Link` keeps the SPA navigation for a
      // plain click and lets the browser handle the modified ones itself.
      <Link
        key={item.key}
        to={item.href ?? item.path}
        data-testid={`nav-${item.key}`}
        data-nav-entry=""
        // Anchor for the product tour's third coach mark (features/onboarding/ProductTour).
        data-tour={`nav-${item.key}`}
        // The visual highlight told sighted users which page they were on; this
        // is the same fact, for everyone else.
        aria-current={active ? 'page' : undefined}
        title={L[item.labelKey]}
        className={cn(
          'group/nav relative flex items-center gap-[11px] h-[34px] px-2.5 mb-px rounded-[8px] text-[13px] transition-colors',
          active
            ? 'bg-surface-3 text-fg-primary font-semibold'
            : cn('text-fg-secondary font-medium hover:text-fg-primary', !glide.enabled && 'hover:bg-surface-3'),
          collapsed && 'justify-center px-0',
        )}
      >
        {/* Keyline on the active entry, flush with the nav's left edge. */}
        <span
          aria-hidden="true"
          className="absolute -left-2.5 top-[7px] bottom-[7px] w-[2px] rounded-r-[2px]"
          style={{ background: active ? 'var(--accent)' : 'transparent' }}
        />
        <Icon
          size={17}
          strokeWidth={1.75}
          className={cn(
            'shrink-0',
            active ? 'text-accent' : 'text-fg-muted group-hover/nav:text-fg-primary',
          )}
        />
        {!collapsed && (
          <span className="flex-1 min-w-0 whitespace-nowrap overflow-hidden text-ellipsis">
            {L[item.labelKey]}
          </span>
        )}
        {/* A badge appears only when its live count is a positive number.
            While the count is loading, errored, or refused by permission it is
            absent — a placeholder here would be indistinguishable from a real
            figure, and the whole point of removing the hardcoded '12' was that
            a counter must never state something nobody measured. Zero is also
            absent: a badge means "something is waiting", and nothing is. */}
        {item.badge &&
          navCounts[item.badge.count] > 0 &&
          (collapsed ? (
            <span
              className="absolute top-[7px] left-[24px] w-[7px] h-[7px] rounded-full"
              style={{
                background: BADGE_TONE[item.badge.tone].fg,
                boxShadow: '0 0 0 2px var(--surface-1)',
              }}
              aria-label={`${navCounts[item.badge.count]}`}
            />
          ) : (
            <span
              className="mono text-[10.5px] font-semibold min-w-[18px] h-[18px] px-[5px] rounded-[9px] flex items-center justify-center"
              style={{
                background: BADGE_TONE[item.badge.tone].bg,
                color: BADGE_TONE[item.badge.tone].fg,
              }}
              data-testid={`nav-badge-${item.key}`}
            >
              {navCounts[item.badge.count]}
            </span>
          ))}
      </Link>
    );
  };

  return (
    <>
      {/* Backdrop — mobile only. */}
      {mobileOpen && (
        <div
          onClick={onMobileClose}
          className="lg:hidden fixed inset-0 bg-surface-overlay backdrop-blur-sm z-55"
          aria-hidden="true"
        />
      )}

      <aside
        style={{ transition: 'width 260ms var(--ease-out)' }}
        className={cn(
          // Above the sticky header (z-50) when it slides in as a drawer on mobile.
          'h-screen bg-surface-1 border-r border-border-subtle flex flex-col z-60 overflow-hidden',
          'lg:static lg:shrink-0 lg:translate-x-0',
          collapsed ? 'lg:w-[64px]' : 'lg:w-[248px]',
          'fixed inset-y-0 left-0 w-[248px] max-w-[82vw]',
          mobileOpen ? 'translate-x-0 shadow-card-lg' : '-translate-x-full lg:translate-x-0',
        )}
      >
        <div className="flex flex-col h-full">
          {/* Logo + org switcher */}
          <div className="px-[14px] pt-[14px] pb-2.5">
            <div
              className={cn(
                'flex items-center gap-2.5 px-[3px] pb-[14px]',
                collapsed && 'justify-center px-0',
              )}
            >
              <div
                className="w-[30px] h-[30px] rounded-[9px] flex items-center justify-center shrink-0 text-fg-on-solid"
                style={{
                  background: 'var(--accent-solid)',
                }}
              >
                <OpenRiskLogo size={18} />
              </div>
              {!collapsed && (
                <span className="disp text-[17px] font-bold tracking-[-0.01em] whitespace-nowrap text-ink">
                  OpenRisk
                </span>
              )}
            </div>

            {!collapsed && (
              <OrgSwitcher
                orgName={orgName}
                badge={
                  <OrgLogo
                    name={orgName}
                    hasLogo={branding?.has_logo ?? false}
                    size={26}
                    radius={7}
                    neutral
                  />
                }
              />
            )}
          </div>

          {/* Quick action — only for a member who may create a risk (#739). */}
          {can('risks:create') && (
            <div className={cn('px-[14px] pb-2.5', collapsed && 'px-2.5')}>
              <button
                data-tour="new-risk"
                onClick={() => {
                  window.dispatchEvent(new CustomEvent('openrisk:new-risk'));
                  onMobileClose?.();
                }}
                className="w-full h-9 rounded-[10px] flex items-center justify-center gap-2 text-[13px] font-semibold transition-[filter,transform] hover:brightness-[1.08] active:translate-y-px"
                style={{
                  background: 'var(--accent-solid)',
                  color: 'var(--fg-on-solid)',
                }}
                title={`${L.newRisk} (N)`}
              >
                <Plus size={16} strokeWidth={2.2} />
                {!collapsed && <span>{L.newRisk}</span>}
              </button>
            </div>
          )}

          {/* Navigation */}
          <nav
            aria-label={tr('Navigation principale', 'Main navigation')}
            className="relative flex-1 overflow-y-auto overflow-x-hidden px-2.5 pt-1 pb-2.5"
            {...glide.navProps}
          >
            {/* First child, so every entry paints above it. Glides on
                --motion-hover, fades out on --motion-exit, never scales. */}
            {glide.enabled && (
              <div
                ref={glideRef}
                aria-hidden="true"
                data-testid="nav-glide"
                data-visible="false"
                className={cn(
                  'pointer-events-none absolute top-0 left-0 rounded-[8px] bg-surface-3 opacity-0',
                  'transition-opacity duration-fast ease-in',
                  'data-[visible=true]:opacity-100 data-[visible=true]:ease-out',
                  'data-[visible=true]:transition-[transform,height,opacity] data-[visible=true]:duration-fast',
                )}
              />
            )}
            {/* Pinned entries (Dashboard) hoisted above the intention groups. */}
            {pinned.length > 0 && (
              <>
                {pinned.map(navItem)}
                <div aria-hidden="true" className="h-px bg-border-subtle mt-2 mx-[2px] mb-[14px]" />
              </>
            )}
            {navGroups.map((group) => {
              const items = group.items.filter((i) => !i.pinned);
              if (items.length === 0) return null;
              return (
                <div key={group.groupKey} className="mb-[14px]">
                  {!collapsed && (
                    <div className="text-[10px] tracking-[0.09em] uppercase font-semibold text-ink-muted px-2.5 pb-1.5">
                      {L[group.groupKey]}
                    </div>
                  )}
                  {items.map(navItem)}
                </div>
              );
            })}
          </nav>

          {/* Security score footer — the canonical tenant score; hidden until
              it has loaded, "not measured" when the tenant has nothing to score. */}
          {!collapsed && tenantScore !== undefined && (
            <Link
              to="/?view=executive"
              className="block w-full text-left px-4 py-3 border-t border-border-subtle hover:bg-surface-2 transition-colors"
              title={L.openExecutive}
            >
              <div className="flex items-baseline justify-between mb-[7px]">
                <span className="text-[11px] text-ink-soft font-medium whitespace-nowrap">
                  {L.globalScore} · {bandLabel(measuredScore?.band, lang)}
                </span>
                <span
                  className={`mono text-[12px] font-semibold${score === undefined ? ' text-ink-muted' : ''}`}
                  style={score === undefined ? undefined : { color: scoreTextColor }}
                  data-testid="sidebar-score-value"
                >
                  {score === undefined ? '—' : `${score}/100`}
                </span>
              </div>
              <div className="h-1 rounded overflow-hidden bg-surface-3">
                <div
                  className="h-full rounded"
                  style={{
                    width: `${score ?? 0}%`,
                    background: scoreColor,
                    transition: 'width .8s cubic-bezier(.2,.8,.2,1)',
                  }}
                />
              </div>
            </Link>
          )}

          {/* User menu (account · settings · logout) + collapse */}
          <div className="relative px-3 py-2.5 border-t border-border-subtle">
            {menuOpen && (
              <>
                <div
                  className="fixed inset-0 z-59"
                  onClick={() => setMenuOpen(false)}
                  aria-hidden="true"
                />
                <div
                  className="absolute left-[14px] right-[14px] z-60 rounded-[12px] overflow-hidden shadow-card-lg"
                  style={{
                    bottom: 'calc(100% - 6px)',
                    background: 'var(--bg-elevated)',
                    border: '1px solid var(--border)',
                    animation: 'or-scalein var(--motion-enter)',
                  }}
                >
                  <div className="px-3 py-2.5 border-b border-border">
                    <div className="text-[12.5px] font-semibold text-ink truncate">
                      {user?.full_name || user?.username || 'Admin'}
                    </div>
                    <div className="text-[11px] text-ink-muted truncate">{user?.email}</div>
                  </div>
                  <button
                    onClick={() => {
                      setMenuOpen(false);
                      navigate('/settings/profile');
                    }}
                    className="w-full flex items-center gap-2.5 px-3 py-2.5 text-[13px] font-medium text-ink hover:bg-hover transition-colors"
                  >
                    <UserRound size={16} strokeWidth={1.8} /> {L.myProfile}
                  </button>
                  <button
                    onClick={() => {
                      setMenuOpen(false);
                      navigate('/settings');
                    }}
                    className="w-full flex items-center gap-2.5 px-3 py-2.5 text-[13px] font-medium text-ink hover:bg-hover transition-colors"
                  >
                    <Settings size={16} strokeWidth={1.8} /> {tr('Paramètres', 'Settings')}
                  </button>
                  <button
                    onClick={handleLogout}
                    className="w-full flex items-center gap-2.5 px-3 py-2.5 text-[13px] font-medium hover:bg-hover transition-colors"
                    style={{ color: 'var(--critical)' }}
                  >
                    <LogOut size={16} strokeWidth={1.8} /> {tr('Se déconnecter', 'Log out')}
                  </button>
                </div>
              </>
            )}

            <div className={cn('flex items-center gap-2', collapsed && 'justify-center')}>
              <button
                onClick={() => setMenuOpen((v) => !v)}
                title={tr('Compte', 'Account')}
                aria-label={tr('Menu du compte', 'Account menu')}
                aria-expanded={menuOpen}
                className={cn(
                  'flex items-center gap-2.5 min-w-0 rounded-[9px] p-1 hover:bg-surface-3 transition-colors',
                  !collapsed && 'flex-1',
                )}
              >
                <UserAvatar
                  userId={user?.id}
                  name={user?.full_name}
                  hasAvatar={myProfile?.has_avatar ?? false}
                  fallback={initials(user?.full_name)}
                  size={30}
                />
                {!collapsed && (
                  <div className="flex-1 min-w-0 text-left">
                    <div className="text-[12px] font-semibold text-ink truncate">
                      {user?.full_name || user?.username || 'Admin'}
                    </div>
                    <SidebarRoleLabel />
                  </div>
                )}
              </button>
              {!collapsed && (
                <button
                  onClick={toggleCollapse}
                  className="hidden lg:flex w-7 h-7 rounded-[7px] items-center justify-center text-ink-muted hover:bg-surface-3 hover:text-ink transition-colors shrink-0"
                  aria-label={L.collapseSidebar}
                  title={L.collapseSidebar}
                >
                  <PanelLeftClose size={16} strokeWidth={1.7} />
                </button>
              )}
            </div>
          </div>

          {/* Expand affordance when collapsed (desktop) */}
          {collapsed && (
            <button
              onClick={toggleCollapse}
              className="hidden lg:flex mx-auto mb-3 w-7 h-7 rounded-[7px] items-center justify-center text-ink-muted hover:bg-surface-3 hover:text-ink transition-colors"
              aria-label={L.expandSidebar}
              title={L.expandSidebar}
            >
              <PanelLeftOpen size={16} strokeWidth={1.7} />
            </button>
          )}
        </div>
      </aside>
    </>
  );
};
