// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The header's notifications panel. Its own module so it loads on the first
// click of the bell rather than in the entry chunk (#966): nothing in it is
// drawn until then, and it carries the category icons, the link resolvers and
// the empty states with it. The bell and its unread badge stay in AppHeader.

import { useEffect, useId, useRef, useState } from 'react';
import { Link, useNavigate } from 'react-router';
import { Bell } from 'lucide-react';
import { cn, useDismissableLayer } from '../../shared/ds';
import { useUIStore } from '../../store/uiStore';
import { useUIStrings } from '../../shared/uiStrings';
import {
  categoryMeta,
  categoryForType,
  type NotifCategory,
} from '../../shared/notificationCategory';
import {
  useNotifications,
  useNotificationActions,
} from '../../features/notifications/useNotifications';
import {
  hasNotificationTarget,
  resolveNotificationHref,
} from '../../features/notifications/notificationLinks';
import { EmptyState } from '../../shared/EmptyState';
import { Btn, SkeletonRows } from '../../shared/ui';
import type { LocaleCode } from '../../i18n/locales';
import { pickLocalized } from '../../i18n/locales';

/**
 * True once `open` is allowed to reach the DOM, which is one macrotask after
 * mount at the earliest. A freshly-mounted node cannot animate its own
 * insertion via a `transition` (the browser needs an already-rendered
 * "before" style to interpolate from), so the very first render is always
 * forced closed here regardless of `open`, then flipped to match `open` a
 * tick later — the same "paint the wrong frame first, correct it on a timer"
 * device used elsewhere in this codebase (see #751 phase 3's SlotReel
 * roll-on-mount). Skipped under reduced motion, where there is no transition
 * to prepare a from-state for and the extra frame would just be a flash.
 */
function useEnterGate(open: boolean): boolean {
  const [ready, setReady] = useState(
    () => window.matchMedia('(prefers-reduced-motion: reduce)').matches,
  );
  useEffect(() => {
    if (ready) return;
    const timer = setTimeout(() => setReady(true), 0);
    return () => clearTimeout(timer);
  }, [ready]);
  return ready && open;
}

/* ---------- Notifications panel (anchored right) ---------- */
// Reads the real /notifications feed. This panel used to render four invented
// notifications on every tenant, which is how a fresh install came to report
// incidents it had never had.
export function NotifPanel({ open, onClose }: { open: boolean; onClose: () => void }) {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const [filter, setFilter] = useState<NotifCategory | 'all'>('all');
  const { notifications, isLoading, isError } = useNotifications(20);
  const { markRead, markAllRead } = useNotificationActions();
  const titleId = useId();
  const panelRef = useRef<HTMLDivElement>(null);
  // Escape closes and returns focus to the bell; Tab is trapped inside while
  // open. `lockScroll: false` — this is a popover over content, not a modal,
  // so the page behind stays scrollable (ux spec: adopt useDismissableLayer).
  useDismissableLayer(panelRef, { open, onClose, closeOnEscape: true, lockScroll: false });
  // Gates the very first `data-open="true"` by one macrotask (review fix:
  // the panel mounts already open, and a plain CSS *transition* — needed so
  // reopening mid-exit interpolates instead of restarting a keyframe from
  // 0% — never plays on an element's initial style resolution, only on a
  // later style recalc of an already-painted node). Every later flip (close,
  // or reopening while still mid-exit) tracks `open` directly and instantly:
  // by then the node is already painted, so the transition just continues
  // from wherever it currently sits.
  const dataOpen = useEnterGate(open);

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
      linkable: hasNotificationTarget(n),
      target: { resource_type: n.resource_type, resource_id: n.resource_id },
    };
  });
  const shown = filter === 'all' ? items : items.filter((it) => it.category === filter);

  // A click marks the notification read and, when it is about something, takes
  // the user to that exact thing (features/notifications/notificationLinks.ts).
  const openItem = (it: (typeof items)[number]) => {
    if (it.unread) markRead.mutate(it.id);
    if (!it.linkable) return;
    onClose();
    void resolveNotificationHref(it.target).then((href) => {
      if (href) navigate(href);
    });
  };
  const cats: (NotifCategory | 'all')[] = [
    'all',
    ...Array.from(new Set(items.map((it) => it.category))),
  ];

  return (
    <>
      {/* invisible backdrop closes on outside click */}
      <div className="fixed inset-0 z-65" onClick={onClose} />
      <div
        ref={panelRef}
        onClick={(e) => e.stopPropagation()}
        data-open={dataOpen}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className="notif-panel absolute top-[42px] right-0 w-[400px] max-w-[calc(100vw-24px)] rounded-[14px] overflow-hidden bg-surface-2 border border-border-default z-70 outline-none"
        style={{ boxShadow: 'var(--elev-3)' }}
      >
        <div className="flex items-center justify-between px-3.5 py-3 border-b border-border-subtle">
          <span id={titleId} className="text-[13px] font-semibold text-ink">
            {L.notifTitle}
          </span>
          {items.some((it) => it.unread) && (
            <button
              onClick={() => markAllRead.mutate()}
              className="text-[12px] font-semibold text-accent-strong px-1.5 py-1 rounded-[6px] hover:bg-accent-soft transition-colors"
            >
              {L.notifAll}
            </button>
          )}
        </div>

        {/* category filter — separate the contexts (UX-6). Only rendered when
            there is something to filter; chips over an empty list are noise. */}
        {cats.length > 1 && (
          <div className="flex gap-1.5 px-3.5 py-2.5 border-b border-border-subtle overflow-x-auto">
            {cats.map((c) => {
              const active = filter === c;
              const label =
                c === 'all' ? tr('Tout', 'All') : pickLocalized(lang, categoryMeta(c).label);
              return (
                <button
                  key={c}
                  onClick={() => setFilter(c)}
                  className="shrink-0 h-[26px] px-2.5 rounded-full text-[11.5px] font-semibold transition-colors"
                  style={{
                    background: active ? 'var(--accent-soft)' : 'var(--bg-hover)',
                    color: active ? 'var(--accent)' : 'var(--fg-secondary)',
                  }}
                >
                  {label}
                </button>
              );
            })}
          </div>
        )}

        <div className="max-h-[420px] overflow-y-auto">
          {isLoading ? (
            <div className="p-3">
              <SkeletonRows rows={3} height={44} />
            </div>
          ) : isError ? (
            <EmptyState
              variant="error"
              title={tr('Notifications indisponibles', 'Notifications unavailable')}
              description={tr(
                'Impossible de charger vos notifications.',
                'Could not load your notifications.',
              )}
              className="py-10"
            />
          ) : shown.length === 0 ? (
            <EmptyState
              variant="first-use"
              icon={Bell}
              title={tr('Aucune notification', 'No notifications')}
              description={tr(
                'Vous serez alerté ici des risques critiques, des SLA dépassés et des tâches qui vous sont assignées.',
                'You will be alerted here about critical risks, breached SLAs and tasks assigned to you.',
              )}
              primaryAction={
                <Btn
                  label={tr('Régler mes alertes', 'Tune my alerts')}
                  onClick={() => {
                    onClose();
                    navigate('/settings');
                  }}
                />
              }
              className="py-10"
            />
          ) : (
            shown.map((it) => {
              const Icon = it.icon;
              return (
                <div
                  key={it.id}
                  role={it.linkable ? 'link' : 'button'}
                  tabIndex={0}
                  onClick={() => openItem(it)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      openItem(it);
                    }
                  }}
                  className="w-full text-left flex gap-3 px-3.5 py-3 border-b border-border-subtle cursor-pointer hover:bg-surface-3 focus-visible:bg-surface-3 outline-none transition-colors"
                >
                  <div
                    className="w-[30px] h-[30px] rounded-[8px] flex items-center justify-center shrink-0"
                    style={{
                      background: `color-mix(in srgb,${it.color} 14%,transparent)`,
                      color: it.color,
                    }}
                  >
                    <Icon size={15} strokeWidth={1.8} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div
                      className={cn(
                        'text-[13px] leading-[1.35] text-ink',
                        it.unread ? 'font-semibold' : 'font-medium',
                      )}
                    >
                      {it.title}
                    </div>
                    {it.body && (
                      <div className="text-[12px] text-ink-muted mt-0.5 leading-snug">
                        {it.body}
                      </div>
                    )}
                    <div className="text-[11px] text-ink-muted mt-1 flex items-center gap-1.5">
                      {it.time}
                      <span
                        className="px-1.5 py-px rounded-full text-[10px] font-semibold"
                        style={{
                          color: categoryMeta(it.category).color,
                          background: `color-mix(in srgb, ${categoryMeta(it.category).color} 14%, transparent)`,
                        }}
                      >
                        {pickLocalized(lang, categoryMeta(it.category).label)}
                      </span>
                    </div>
                  </div>
                  {it.unread && (
                    <span
                      className="w-[7px] h-[7px] rounded-full shrink-0 mt-1.5"
                      style={{ background: 'var(--accent)' }}
                    />
                  )}
                </div>
              );
            })
          )}
        </div>
        <Link
          to="/activity"
          onClick={onClose}
          className="block text-center p-2.5 text-[12.5px] font-semibold text-accent-strong hover:bg-surface-3 transition-colors"
        >
          {L.notifSeeAll}
        </Link>
      </div>
    </>
  );
}

/** Compact relative time ("il y a 8 min"), no dependency on a date library. */
function relativeTime(iso: string, lang: LocaleCode): string {
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
