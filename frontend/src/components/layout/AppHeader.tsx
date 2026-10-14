// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Global header (October 2026 redesign, #900): breadcrumb · ⌘K search ·
// connection status · language · density · theme · notifications · help. It
// keeps the redesign's layout (search, theme, bell, help on a solid canvas) and
// adds the controls the redesign does not draw but the product relies on: the
// live connection dot, the language switch and table density. On < lg it
// collapses to a hamburger.
//
// Two controls were removed rather than kept as decoration (docs/ui/dead-controls.md):
// the "Voice assistant" microphone (no speech feature exists anywhere in the
// product) and the old "View all notifications" footer, which merely closed the
// panel. The redesign's footer link goes somewhere real, the activity feed, so
// it is back with that destination. The pulsing green dot,
// which claimed "Realtime" on every tenant regardless of anything, is now a real
// connection indicator driven by lib/connection.

import { lazy, Suspense, useSyncExternalStore, useState, useEffect } from 'react';
import { Search, Bell, Sun, Moon, Menu, Rows2, Rows3, Rows4, CircleHelp } from 'lucide-react';
import { cn, useExitTimer } from '../../shared/ds';
import { useUIStore } from '../../store/uiStore';
import { useUIStrings } from '../../shared/uiStrings';
import { Hint } from '../../shared/Hint';
import { Breadcrumbs } from '../../shared/Breadcrumbs';
import {
  getConnectionStatus,
  subscribeConnection,
  type ConnectionState,
} from '../../lib/connection';
import { useRealtimeStatus } from '../../features/realtime/useRealtime';
import { useUnreadCount } from '../../features/notifications/useNotifications';
import type { LocaleCode } from '../../i18n/locales';
import { ENABLED_LOCALES, localeDefinition } from '../../i18n/locales';
import { NOTIF_EXIT_MS } from './notifMotion';

// The panel and everything it draws load on the first open, not with the shell
// (#966). Hovering or focusing the bell starts the fetch, so by the time the
// click lands the module is usually already there.
const loadNotifPanel = () => import('./NotifPanel');
const NotifPanel = lazy(() => loadNotifPanel().then((m) => ({ default: m.NotifPanel })));

interface AppHeaderProps {
  onOpenMobileNav: () => void;
}

const iconBtn =
  'w-[34px] h-[34px] rounded-[10px] flex items-center justify-center text-fg-secondary hover:bg-surface-3 hover:text-ink transition-colors';

export const AppHeader = ({ onOpenMobileNav }: AppHeaderProps) => {
  const setCmdkOpen = useUIStore((s) => s.setCmdkOpen);
  const toggleTheme = useUIStore((s) => s.toggleTheme);
  const toggleLang = useUIStore((s) => s.toggleLang);
  const cycleDensity = useUIStore((s) => s.cycleDensity);
  const density = useUIStore((s) => s.density);
  const theme = useUIStore((s) => s.theme);
  const lang = useUIStore((s) => s.lang);
  const L = useUIStrings();
  // The switcher names the language it will switch *to*, in that language's own
  // words, and reads the list from the registry — so enabling a third locale
  // needs no edit here.
  const nextLocaleLabel = (() => {
    const offered = ENABLED_LOCALES;
    const next = offered[(offered.indexOf(lang) + 1) % offered.length];
    return `${localeDefinition(next).nativeName} — ${localeDefinition(lang).nativeName}`;
  })();

  const densityMeta = {
    comfort: { Icon: Rows3, label: lang === 'fr' ? 'Densité : Confort' : 'Density: Comfort' },
    compact: { Icon: Rows4, label: lang === 'fr' ? 'Densité : Compact' : 'Density: Compact' },
    spacious: { Icon: Rows2, label: lang === 'fr' ? 'Densité : Spacieux' : 'Density: Spacious' },
  }[density];
  const [notifOpen, setNotifOpen] = useState(false);
  // The panel keeps rendering for --dur-fast after notifOpen goes false, so
  // its close animation (see .notif-panel in index.css) gets a frame to play
  // instead of the panel just vanishing.
  const notifMounted = useExitTimer(notifOpen, NOTIF_EXIT_MS);
  const { count: unreadCount, isFetched: unreadFetched } = useUnreadCount();
  // Gates the badge's entrance animation on the query's own data identity
  // rather than on this component's mount (#751 phase 4 spec, "armed follows
  // data identity"). Otherwise a page load — where unreadCount goes from
  // nothing to a real number in the very first render pass — would always
  // play the "just arrived" entrance, on every tenant, on every login.
  const badgeArmed = useArmedBadge(unreadFetched);

  return (
    <header className="h-14 shrink-0 flex items-center gap-2.5 pl-3 pr-3 sm:pl-6 sm:pr-4 border-b border-border-subtle bg-surface-0 sticky top-0 z-50">
      {/* Mobile hamburger */}
      <button
        onClick={onOpenMobileNav}
        className={cn(iconBtn, 'lg:hidden')}
        aria-label="Open navigation"
      >
        <Menu size={18} />
      </button>

      {/* Breadcrumb — a real clickable trail derived from the route tree, so
          every page at depth >= 2 renders its own way back. */}
      <div className="flex-1 min-w-0">
        <Breadcrumbs />
      </div>

      {/* ⌘K search trigger */}
      <button
        data-tour="search"
        onClick={() => setCmdkOpen(true)}
        className="hidden sm:flex items-center gap-2 h-[34px] w-[300px] max-w-[32vw] shrink-0 pl-2.5 pr-1.5 rounded-[10px] border border-border-default bg-surface-1 text-ink-muted text-[13px] hover:border-border-strong hover:text-ink-soft transition-colors"
      >
        <Search size={15} strokeWidth={1.8} />
        <span className="flex-1 min-w-0 text-left whitespace-nowrap overflow-hidden text-ellipsis">
          {L.searchHint}
        </span>
        <span className="mono text-[11px] px-1.5 py-0.5 rounded-[5px] border border-border-default text-ink-muted">
          ⌘K
        </span>
      </button>

      {/* Actions */}
      <div className="flex items-center gap-0.5 shrink-0">
        <button
          onClick={() => setCmdkOpen(true)}
          className={cn(iconBtn, 'sm:hidden')}
          aria-label="Search"
        >
          <Search size={18} />
        </button>

        <ConnectionDot lang={lang} />

        <button
          onClick={toggleLang}
          className={iconBtn}
          title={nextLocaleLabel}
          aria-label={nextLocaleLabel}
        >
          <span className="mono text-[11px] font-semibold">{lang.toUpperCase()}</span>
        </button>

        <Hint
          id="header-density"
          side="bottom"
          text={
            lang === 'fr'
              ? 'Ajustez la densité des tables et listes : Confort · Compact · Spacieux.'
              : 'Adjust table & list density: Comfort · Compact · Spacious.'
          }
        >
          <button
            onClick={cycleDensity}
            className={cn(iconBtn, 'hidden sm:flex')}
            title={densityMeta.label}
            aria-label={densityMeta.label}
          >
            <densityMeta.Icon size={17} strokeWidth={1.7} />
          </button>
        </Hint>

        <button
          onClick={toggleTheme}
          className={iconBtn}
          title={theme === 'dark' ? L.themeToLight : L.themeToDark}
          /* Names what pressing it DOES, not a static "toggle theme" — the
             accessible name tracks state the same way the icon does. */
          aria-label={theme === 'dark' ? L.themeToLight : L.themeToDark}
        >
          {/* Both icons stacked in one grid cell and cross-faded on
              --motion-hover (opacity + a slight scale) — no rotation, no pop. */}
          <span className="grid">
            <Sun
              aria-hidden="true"
              size={17}
              strokeWidth={1.7}
              className={cn(
                '[grid-area:1/1] transition-[opacity,transform] duration-fast ease-out',
                theme === 'dark' ? 'opacity-100 scale-100' : 'opacity-0 scale-75',
              )}
            />
            <Moon
              aria-hidden="true"
              size={17}
              strokeWidth={1.7}
              className={cn(
                '[grid-area:1/1] transition-[opacity,transform] duration-fast ease-out',
                theme === 'dark' ? 'opacity-0 scale-75' : 'opacity-100 scale-100',
              )}
            />
          </span>
        </button>

        {/* Notifications */}
        <div className="relative">
          <button
            onClick={() => setNotifOpen((v) => !v)}
            onPointerEnter={() => void loadNotifPanel()}
            onFocus={() => void loadNotifPanel()}
            className={cn(iconBtn, 'relative', notifOpen && 'bg-surface-3')}
            title={L.notifTitle}
            // The count is real server data (polled, never invented), so the
            // accessible name carries it — the true number, not the "9+" the
            // badge caps its own text at. A screen-reader user is entitled to
            // "142 unread", the same information a sighted user reads off the
            // badge glyph itself.
            aria-label={
              unreadCount > 0
                ? lang === 'fr'
                  ? `Notifications, ${unreadCount} non lues`
                  : `Notifications, ${unreadCount} unread`
                : L.notifTitle
            }
            aria-expanded={notifOpen}
            aria-haspopup="dialog"
          >
            <Bell size={17} strokeWidth={1.7} />
            <NotifBadge count={unreadCount} armed={badgeArmed} />
          </button>
          {notifMounted && (
            <Suspense fallback={null}>
              <NotifPanel open={notifOpen} onClose={() => setNotifOpen(false)} />
            </Suspense>
          )}
        </div>

        {/* Help: the keyboard shortcuts sheet ("?" opens it from anywhere). */}
        <button
          onClick={() => window.dispatchEvent(new CustomEvent('openrisk:shortcuts'))}
          className={cn(iconBtn, 'hidden sm:flex')}
          title={L.helpShortcutsTitle}
          aria-label={L.helpShortcuts}
          data-testid="header-help"
        >
          <CircleHelp size={17} strokeWidth={1.7} />
        </button>
      </div>
    </header>
  );
};

/* ---------- Notification badge (#751 phase 4) ---------- */
// Replaces the old presence dot: the server already sends a real integer
// (useUnreadCount), so throwing it away and drawing a dot invented nothing —
// it just hid data the API already provides. Capped at "9+" for the glyph
// itself; the bell's aria-label above carries the true number regardless of
// the cap.
//
// Motion: always mounted, gated by data-armed/data-visible (see .notif-badge
// in index.css) rather than a JS timer, so a poll that repeats the same count
// is structurally silent — no attribute changes, so no transition fires.
function NotifBadge({ count, armed }: { count: number; armed: boolean }) {
  const visible = count > 0;
  return (
    <span
      aria-hidden="true"
      data-armed={armed}
      data-visible={visible}
      className="notif-badge absolute top-[5px] right-[5px] min-w-[15px] h-[15px] px-1 rounded-full flex items-center justify-center text-[9.5px] font-bold tabular-nums"
      style={{
        background: 'var(--danger-solid)',
        color: 'var(--fg-on-solid)',
        boxShadow: '0 0 0 2px var(--surface-0)',
      }}
    >
      {count > 9 ? '9+' : count}
    </span>
  );
}

/**
 * True once `useUnreadCount`'s first fetch has resolved, flipped a macrotask
 * later so the browser always paints one "unarmed" frame before it — even
 * when the query resolves from cache and `isFetched` is already true on the
 * very first render. Without that deferral, mount and "armed" would land in
 * the same paint and the badge's very first appearance would animate exactly
 * like every later one, which is what the spec forbids ("no animation on
 * first data"). Not a visual duration, so no prefers-reduced-motion branch —
 * it gates a state, not a timing.
 */
function useArmedBadge(isFetched: boolean): boolean {
  const [armed, setArmed] = useState(false);
  useEffect(() => {
    if (armed || !isFetched) return;
    const timer = setTimeout(() => setArmed(true), 0);
    return () => clearTimeout(timer);
  }, [armed, isFetched]);
  return armed;
}

/* ---------- connection status ---------- */
// Reads lib/connection, which is fed by the browser's online/offline events and
// by the outcome of every axios call, AND the realtime stream's own state.
// Green = the API answered; amber = a request could not reach it; grey = the
// browser reports no network. It only ever reports an observation, never an
// assumption.
//
// The two signals are combined rather than shown as two dots, and the API's
// verdict wins when it is bad: a browser that cannot reach the API at all is
// not usefully told that its event stream is also down. When the API is fine,
// the dot reports whether live updates are actually arriving — which is the
// difference between a screen that refreshes itself and one the user has to
// reload, and the user is entitled to know which they are looking at.
function ConnectionDot({ lang }: { lang: LocaleCode }) {
  const status = useSyncExternalStore(
    subscribeConnection,
    getConnectionStatus,
    getConnectionStatus,
  );
  const live = useRealtimeStatus();
  const fr = lang === 'fr';

  const meta: Record<ConnectionState, { color: string; label: [string, string]; pulse: boolean }> =
    {
      online: {
        color: 'var(--low)',
        label: ['Connecté au serveur', 'Connected to the server'],
        pulse: true,
      },
      degraded: {
        color: 'var(--high)',
        label: ['Serveur injoignable', 'Server unreachable'],
        pulse: false,
      },
      offline: { color: 'var(--fg-muted)', label: ['Hors ligne', 'Offline'], pulse: false },
    };
  let m = meta[status.state];
  // `live` is the realtime stream's own state, reported SEPARATELY from the
  // connection state below. It used to overwrite `data-state`, which meant the
  // attribute never read "online" while the API was reachable — it always
  // carried a live-* value instead. That silently broke the contract
  // tests/e2e/dead-controls.spec.ts reads ("the status dot reports the REAL
  // connection"), and conflated two questions a user asks separately: can the
  // app reach the server, and are updates arriving by themselves.
  //
  // The colour and the label still combine both, because one dot is the right
  // amount of chrome. Only the machine-readable attributes are split.
  let liveState: string = 'none';

  if (status.state === 'online') {
    switch (live.state) {
      case 'CONNECTED':
        m = {
          color: 'var(--low)',
          label: ['Mises à jour en direct', 'Live updates on'],
          pulse: true,
        };
        liveState = 'live';
        break;
      case 'RECONNECTING':
      case 'INITIALIZING':
        m = {
          color: 'var(--medium)',
          label: ['Reconnexion au flux…', 'Reconnecting to the live stream…'],
          pulse: false,
        };
        liveState = 'live-reconnecting';
        break;
      case 'RESYNCING':
        m = {
          color: 'var(--medium)',
          label: ['Resynchronisation en cours', 'Resynchronising'],
          pulse: false,
        };
        liveState = 'live-resyncing';
        break;
      case 'FORBIDDEN':
        // Said plainly rather than shown as a fault: nothing is broken, this
        // account simply may not hold a stream.
        m = {
          color: 'var(--fg-muted)',
          label: [
            'Direct non autorisé pour ce compte',
            'Live updates not permitted for this account',
          ],
          pulse: false,
        };
        liveState = 'live-forbidden';
        break;
      case 'ERROR':
      case 'DISCONNECTED':
        m = {
          color: 'var(--fg-muted)',
          label: [
            'Direct interrompu — rechargez pour actualiser',
            'Live updates stopped — reload to refresh',
          ],
          pulse: false,
        };
        liveState = 'live-off';
        break;
    }
  }

  const label = m.label[fr ? 0 : 1];
  return (
    <div
      className="flex items-center px-2"
      title={label}
      data-testid="connection-status"
      data-state={status.state}
      data-live={liveState}
    >
      <span
        className="w-[7px] h-[7px] rounded-full"
        style={{ background: m.color, animation: m.pulse ? 'or-pulsedot 2.4s infinite' : 'none' }}
      />
      {/*
        One polite live region for the CONNECTION STATE, which changes rarely.
        Individual events are deliberately not announced: a screen reader
        narrating every risk update in a busy tenant would make the page
        unusable, and the state is the only part a user needs read aloud.
      */}
      <span className="sr-only" role="status" aria-live="polite">
        {label}
      </span>
    </div>
  );
}
