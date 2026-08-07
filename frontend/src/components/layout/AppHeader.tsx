// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Global glass header (OpenRisk.dc.html §5): breadcrumb · ⌘K search · realtime
// dot · language · voice · notifications · theme. Sticky, backdrop-blurred, sits
// above the scrollable body. On < lg it collapses to a hamburger + brand.

import { useState } from 'react';
import { useNavigate } from 'react-router';
import {
  Search, Bell, Sun, Moon, Mic, Menu,
  Rows2, Rows3, Rows4, Keyboard,
} from 'lucide-react';
import { cn } from '../ui/Button';
import { useUIStore } from '../../store/uiStore';
import { useUIStrings } from '../../shared/uiStrings';
import { Hint } from '../../shared/Hint';
import { categoryMeta, categoryForType, type NotifCategory } from '../../shared/notificationCategory';
import { Breadcrumbs } from '../../shared/Breadcrumbs';
import { useNotifications, useUnreadCount, useNotificationActions } from '../../features/notifications/useNotifications';
import { EmptyState } from '../../shared/EmptyState';
import { Btn, SkeletonRows } from '../../shared/ui';

interface AppHeaderProps {
  onOpenMobileNav: () => void;
}

const iconBtn =
  'w-9 h-9 rounded-[9px] flex items-center justify-center text-ink-muted hover:bg-hover hover:text-ink transition-colors';

export const AppHeader = ({ onOpenMobileNav }: AppHeaderProps) => {
  const setCmdkOpen = useUIStore((s) => s.setCmdkOpen);
  const toggleTheme = useUIStore((s) => s.toggleTheme);
  const toggleLang = useUIStore((s) => s.toggleLang);
  const cycleDensity = useUIStore((s) => s.cycleDensity);
  const density = useUIStore((s) => s.density);
  const theme = useUIStore((s) => s.theme);
  const lang = useUIStore((s) => s.lang);
  const L = useUIStrings();
  const densityMeta = {
    comfort: { Icon: Rows3, label: lang === 'fr' ? 'Densité : Confort' : 'Density: Comfort' },
    compact: { Icon: Rows4, label: lang === 'fr' ? 'Densité : Compact' : 'Density: Compact' },
    spacious: { Icon: Rows2, label: lang === 'fr' ? 'Densité : Spacieux' : 'Density: Spacious' },
  }[density];
  const [notifOpen, setNotifOpen] = useState(false);
  const unreadCount = useUnreadCount();

  return (
    <header
      className="h-[58px] shrink-0 flex items-center gap-3 px-3 sm:px-[18px] border-b border-border sticky top-0 z-50 glass"
    >
      {/* Mobile hamburger */}
      <button onClick={onOpenMobileNav} className={cn(iconBtn, 'lg:hidden')} aria-label="Open navigation">
        <Menu size={18} />
      </button>

      {/* Breadcrumb — a real clickable trail derived from the route tree, so
          every page at depth >= 2 renders its own way back. */}
      <Breadcrumbs />

      <div className="flex-1" />

      {/* ⌘K search trigger */}
      <button
        onClick={() => setCmdkOpen(true)}
        className="hidden sm:flex items-center gap-[9px] h-[34px] px-3 rounded-[9px] text-ink-muted min-w-[230px] text-[13px] transition-colors"
        style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}
      >
        <Search size={15} strokeWidth={1.8} />
        <span className="flex-1 text-left">{L.search}</span>
        <span className="mono text-[10.5px] px-1.5 py-0.5 rounded-[5px]" style={{ background: 'var(--bg-hover)', border: '1px solid var(--border)' }}>
          ⌘K
        </span>
      </button>

      <div className="flex-1" />

      {/* Actions */}
      <div className="flex items-center gap-0.5">
        <button onClick={() => setCmdkOpen(true)} className={cn(iconBtn, 'sm:hidden')} aria-label="Search">
          <Search size={18} />
        </button>

        {/* Realtime pulse */}
        <div className="flex items-center px-2" title="Realtime">
          <span className="w-[7px] h-[7px] rounded-full" style={{ background: 'var(--low)', animation: 'or-pulsedot 2.4s infinite' }} />
        </div>

        <button onClick={toggleLang} className={iconBtn} title="Language" aria-label="Toggle language">
          <span className="mono text-[11px] font-semibold">{lang.toUpperCase()}</span>
        </button>

        <Hint
          id="header-density"
          side="bottom"
          text={lang === 'fr'
            ? 'Ajustez la densité des tables et listes : Confort · Compact · Spacieux.'
            : 'Adjust table & list density: Comfort · Compact · Spacious.'}
        >
          <button onClick={cycleDensity} className={cn(iconBtn, 'hidden sm:flex')} title={densityMeta.label} aria-label={densityMeta.label}>
            <densityMeta.Icon size={18} strokeWidth={1.7} />
          </button>
        </Hint>

        <Hint
          id="header-shortcuts"
          side="bottom"
          text={lang === 'fr'
            ? 'Raccourcis clavier — ou appuyez sur « ? » à tout moment.'
            : 'Keyboard shortcuts — or press “?” anytime.'}
        >
          <button
            onClick={() => window.dispatchEvent(new CustomEvent('openrisk:shortcuts'))}
            className={cn(iconBtn, 'hidden sm:flex')}
            title={lang === 'fr' ? 'Raccourcis clavier (?)' : 'Keyboard shortcuts (?)'}
            aria-label={lang === 'fr' ? 'Raccourcis clavier' : 'Keyboard shortcuts'}
          >
            <Keyboard size={18} strokeWidth={1.7} />
          </button>
        </Hint>

        <button className={cn(iconBtn, 'hidden sm:flex')} title="Voice assistant" aria-label="Voice assistant">
          <Mic size={18} strokeWidth={1.7} />
        </button>

        {/* Notifications */}
        <div className="relative">
          <button onClick={() => setNotifOpen((v) => !v)} className={cn(iconBtn, 'relative')} title={L.notifTitle} aria-label={L.notifTitle}>
            <Bell size={18} strokeWidth={1.7} />
            {/* Lit only when the server reports unread items. This was static
                markup, so it glowed on tenants with no notifications at all. */}
            {unreadCount > 0 && (
              <span
                className="absolute top-[5px] right-[5px] w-[7px] h-[7px] rounded-full"
                style={{ background: 'var(--critical)', border: '1.5px solid var(--glass)' }}
              />
            )}
          </button>
          {notifOpen && <NotifPanel onClose={() => setNotifOpen(false)} />}
        </div>

        <button onClick={toggleTheme} className={iconBtn} title="Theme" aria-label="Toggle theme">
          {theme === 'dark' ? <Sun size={18} strokeWidth={1.7} /> : <Moon size={18} strokeWidth={1.7} />}
        </button>
      </div>
    </header>
  );
};

/* ---------- Notifications panel (glass, anchored right) ---------- */
// Reads the real /notifications feed. This panel used to render four invented
// notifications on every tenant, which is how a fresh install came to report
// incidents it had never had.
function NotifPanel({ onClose }: { onClose: () => void }) {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const [filter, setFilter] = useState<NotifCategory | 'all'>('all');
  const { notifications, isLoading, isError } = useNotifications(20);
  const { markRead, markAllRead } = useNotificationActions();

  const items = notifications.map((n) => {
    const category = categoryForType(n.type);
    return {
      id: n.id,
      category,
      color: categoryMeta(category).color,
      icon: categoryMeta(category).icon,
      title: n.subject || n.message,
      body: n.subject ? n.message : (n.description ?? ''),
      time: relativeTime(n.created_at, lang),
      unread: !n.read_at,
    };
  });
  const shown = filter === 'all' ? items : items.filter((it) => it.category === filter);
  const cats: (NotifCategory | 'all')[] = ['all', ...Array.from(new Set(items.map((it) => it.category)))];

  return (
    <>
      {/* invisible backdrop closes on outside click */}
      <div className="fixed inset-0 z-[65]" onClick={onClose} />
      <div
        onClick={(e) => e.stopPropagation()}
        className="glass-strong absolute top-[44px] right-0 w-[352px] rounded-[16px] overflow-hidden shadow-card-lg z-[70]"
        style={{ animation: 'or-scalein .16s cubic-bezier(.2,.8,.2,1)' }}
      >
        <div className="flex items-center justify-between px-[17px] py-[15px] border-b border-border">
          <span className="text-[14px] font-semibold text-ink">{L.notifTitle}</span>
          {items.some((it) => it.unread) && (
            <button onClick={() => markAllRead.mutate()} className="text-[12px] font-medium text-accent hover:brightness-110">
              {L.notifAll}
            </button>
          )}
        </div>

        {/* category filter — separate the contexts (UX-6). Only rendered when
            there is something to filter; chips over an empty list are noise. */}
        {cats.length > 1 && (
          <div className="flex gap-1.5 px-[15px] py-2.5 border-b border-border overflow-x-auto">
            {cats.map((c) => {
              const active = filter === c;
              const label = c === 'all' ? tr('Tout', 'All') : categoryMeta(c).label[lang];
              return (
                <button
                  key={c}
                  onClick={() => setFilter(c)}
                  className="shrink-0 h-[26px] px-2.5 rounded-full text-[11.5px] font-semibold transition-colors"
                  style={{ background: active ? 'var(--accent-soft)' : 'var(--bg-hover)', color: active ? 'var(--accent)' : 'var(--text-secondary)' }}
                >
                  {label}
                </button>
              );
            })}
          </div>
        )}

        <div className="max-h-[340px] overflow-y-auto">
          {isLoading ? (
            <div className="p-3"><SkeletonRows rows={3} height={44} /></div>
          ) : isError ? (
            <EmptyState
              variant="error"
              title={tr('Notifications indisponibles', 'Notifications unavailable')}
              description={tr('Impossible de charger vos notifications.', 'Could not load your notifications.')}
              className="py-10"
            />
          ) : shown.length === 0 ? (
            <EmptyState
              variant="first-use"
              icon={Bell}
              title={tr('Aucune notification', 'No notifications')}
              description={tr('Vous serez alerté ici des risques critiques, des SLA dépassés et des tâches qui vous sont assignées.', 'You will be alerted here about critical risks, breached SLAs and tasks assigned to you.')}
              primaryAction={<Btn label={tr('Régler mes alertes', 'Tune my alerts')} onClick={() => { onClose(); navigate('/settings'); }} />}
              className="py-10"
            />
          ) : (
            shown.map((it) => {
              const Icon = it.icon;
              return (
                <div
                  key={it.id}
                  onClick={() => { if (it.unread) markRead.mutate(it.id); }}
                  className="flex gap-3 px-[17px] py-[13px] border-b border-border cursor-pointer hover:bg-hover transition-colors"
                  style={{ background: it.unread ? 'color-mix(in srgb,var(--accent) 4%,transparent)' : 'transparent' }}
                >
                  <div
                    className="w-[34px] h-[34px] rounded-[10px] flex items-center justify-center shrink-0"
                    style={{ background: `color-mix(in srgb,${it.color} 14%,transparent)`, color: it.color }}
                  >
                    <Icon size={17} strokeWidth={1.7} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="text-[13px] font-semibold text-ink mb-0.5">{it.title}</div>
                    {it.body && <div className="text-[12px] text-ink-soft leading-snug">{it.body}</div>}
                    <div className="text-[11px] text-ink-muted mt-1 flex items-center gap-1.5">
                      {it.time}
                      <span className="px-1.5 py-[1px] rounded-full text-[10px] font-semibold" style={{ color: categoryMeta(it.category).color, background: `color-mix(in srgb, ${categoryMeta(it.category).color} 14%, transparent)` }}>
                        {categoryMeta(it.category).label[lang]}
                      </span>
                    </div>
                  </div>
                  {it.unread && <span className="w-[7px] h-[7px] rounded-full shrink-0 mt-1.5" style={{ background: 'var(--accent)' }} />}
                </div>
              );
            })
          )}
        </div>

        {shown.length > 0 && (
          <button onClick={onClose} className="w-full py-[13px] text-[13px] font-semibold text-accent hover:brightness-110">
            {L.notifViewAll}
          </button>
        )}
      </div>
    </>
  );
}

/** Compact relative time ("il y a 8 min"), no dependency on a date library. */
function relativeTime(iso: string, lang: 'fr' | 'en'): string {
  const then = new Date(iso).getTime();
  if (!Number.isFinite(then)) return '';
  const mins = Math.max(0, Math.floor((Date.now() - then) / 60000));
  if (mins < 1) return lang === 'fr' ? "à l'instant" : 'just now';
  if (mins < 60) return lang === 'fr' ? `il y a ${mins} min` : `${mins} min ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return lang === 'fr' ? `il y a ${hours} h` : `${hours} h ago`;
  const days = Math.floor(hours / 24);
  if (days === 1) return lang === 'fr' ? 'hier' : 'yesterday';
  return lang === 'fr' ? `il y a ${days} j` : `${days} d ago`;
}
