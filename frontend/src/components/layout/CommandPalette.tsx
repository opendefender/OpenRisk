// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// ⌘K / Ctrl+K command palette, laid out as in the October 2026 redesign (#900):
// actions first, then pages ("Aller à"), then the live cross-entity results.
// ↑↓ move the highlighted row, ↵ runs it, Esc or the backdrop closes. Registers
// the global ⌘K keyboard shortcut itself.

import { useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router';
import {
  Plus,
  FileText,
  Sun,
  Moon,
  Languages,
  Search,
  ShieldAlert,
  Database,
  Bug,
  Loader2,
  ClipboardCheck,
  Scale,
  Globe,
  Users,
  type LucideIcon,
} from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useUIStrings } from '../../shared/uiStrings';
import { pickLocalized } from '../../i18n/locales';
import { visibleNavGroups } from '../../shared/navModel';
import { usePermissions } from '../../hooks/usePermissions';
import {
  universalSearch,
  type SearchResult,
  type SearchResultType,
} from '../../services/searchService';

interface CmdItem {
  label: string;
  icon: LucideIcon;
  shortcut?: string;
  /** Right-hand hint: the nav group of a page, an entity's reference. */
  hint?: string;
  hintMono?: boolean;
  badge?: { text: string; tone: string };
  run: () => void;
}
interface CmdGroup {
  label: string;
  items: CmdItem[];
}

// Entity icon + severity/criticality colour for universal-search results.
const TYPE_ICON: Record<SearchResultType, LucideIcon> = {
  risk: ShieldAlert,
  asset: Database,
  vulnerability: Bug,
  control: ClipboardCheck,
  audit: Scale,
  report: FileText,
  cve: Globe,
  user: Users,
};
// The search API sends raw keys; the palette shows words.
const SEVERITY_LABEL: Record<string, { fr: string; en: string }> = {
  critical: { fr: 'Critique', en: 'Critical' },
  high: { fr: 'Élevé', en: 'High' },
  medium: { fr: 'Moyen', en: 'Medium' },
  low: { fr: 'Faible', en: 'Low' },
  info: { fr: 'Info', en: 'Info' },
};
const STATUS_LABEL: Record<string, { fr: string; en: string }> = {
  open: { fr: 'Ouvert', en: 'Open' },
  in_progress: { fr: 'En cours', en: 'In progress' },
  mitigated: { fr: 'Atténué', en: 'Mitigated' },
  accepted: { fr: 'Accepté', en: 'Accepted' },
  closed: { fr: 'Clos', en: 'Closed' },
};

const TONE: Record<string, string> = {
  critical: 'var(--critical)',
  high: 'var(--high)',
  medium: 'var(--medium)',
  low: 'var(--low)',
  info: 'var(--fg-muted)',
};

export const CommandPalette = () => {
  const open = useUIStore((s) => s.cmdkOpen);
  const setOpen = useUIStore((s) => s.setCmdkOpen);
  const toggle = useUIStore((s) => s.toggleCmdk);
  const toggleTheme = useUIStore((s) => s.toggleTheme);
  const toggleLang = useUIStore((s) => s.toggleLang);
  const theme = useUIStore((s) => s.theme);
  const lang = useUIStore((s) => s.lang);
  const L = useUIStrings();
  const navigate = useNavigate();
  const { can, isAdmin } = usePermissions();
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [active, setActive] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  // Global ⌘K / Ctrl+K + Esc.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        toggle();
      }
      if (e.key === 'Escape') setOpen(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [toggle, setOpen]);

  useEffect(() => {
    if (open) {
      setQuery('');
      setResults([]);
      setActive(0);
      // Autofocus after the mount animation begins.
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }, [open]);

  // Live universal search (debounced, cancellable) — cross-entity hits from the
  // backend, so ⌘K finds any risk/asset/vulnerability, not just nav destinations.
  useEffect(() => {
    const q = query.trim();
    if (!open || q.length < 2) {
      setResults([]);
      setSearching(false);
      return;
    }
    setSearching(true);
    const ctrl = new AbortController();
    const t = setTimeout(() => {
      universalSearch(q, ctrl.signal).then((hits) => {
        if (!ctrl.signal.aborted) {
          setResults(hits);
          setSearching(false);
        }
      });
    }, 180);
    return () => {
      clearTimeout(t);
      ctrl.abort();
    };
  }, [query, open]);

  const groups: CmdGroup[] = useMemo(() => {
    const close = () => setOpen(false);
    const nav: CmdItem[] = visibleNavGroups(can, isAdmin()).flatMap((g) =>
      g.items.map((it) => ({
        label: L[it.labelKey],
        icon: it.icon,
        hint: it.pinned ? undefined : L[g.groupKey],
        run: () => {
          navigate(it.href ?? it.path);
          close();
        },
      })),
    );
    const actions: CmdItem[] = [
      // Offered only to a member who may create a risk (#739).
      ...(can('risks:create')
        ? [
            {
              label: L.newRisk,
              icon: Plus,
              shortcut: 'N',
              run: () => {
                window.dispatchEvent(new CustomEvent('openrisk:new-risk'));
                close();
              },
            },
          ]
        : []),
      {
        label: L.genReport,
        icon: FileText,
        run: () => {
          navigate('/reports');
          close();
        },
      },
      {
        label: theme === 'dark' ? L.themeToLight : L.themeToDark,
        icon: theme === 'dark' ? Sun : Moon,
        run: () => {
          toggleTheme();
          close();
        },
      },
      {
        label: lang === 'fr' ? 'English' : 'Français',
        icon: Languages,
        run: () => {
          toggleLang();
          close();
        },
      },
    ];
    const q = query.trim().toLowerCase();
    const flt = (items: CmdItem[]) =>
      q ? items.filter((i) => i.label.toLowerCase().includes(q)) : items;
    // Backend already matched these to the query — render as-is (no client filter).
    const resultItems: CmdItem[] = results.map((r) => ({
      label: r.title,
      icon: TYPE_ICON[r.type] ?? Search,
      hint:
        r.type === 'risk' && r.subtitle && STATUS_LABEL[r.subtitle]
          ? pickLocalized(lang, STATUS_LABEL[r.subtitle])
          : r.subtitle,
      badge: r.badge
        ? {
            text: (SEVERITY_LABEL[r.badge] && pickLocalized(lang, SEVERITY_LABEL[r.badge])) ?? r.badge,
            tone: TONE[r.badge] ?? 'var(--fg-muted)',
          }
        : undefined,
      run: () => {
        navigate(r.url);
        close();
      },
    }));
    return [
      { label: L.cmdActions, items: flt(actions) },
      {
        label: q ? L.cmdPages : L.cmdGoTo,
        // With no query the palette offers a short jump list, as the design does.
        items: q ? flt(nav) : nav.slice(0, 8),
      },
      { label: L.cmdResults, items: resultItems },
    ].filter((g) => g.items.length > 0);
  }, [L, query, results, navigate, setOpen, theme, lang, toggleTheme, toggleLang, can, isAdmin]);

  const flat = groups.flatMap((g) => g.items);
  const current = Math.min(active, Math.max(flat.length - 1, 0));

  if (!open) return null;

  const onInputKey = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      if (flat.length === 0) return;
      const next =
        e.key === 'ArrowDown' ? (current + 1) % flat.length : (current - 1 + flat.length) % flat.length;
      setActive(next);
      listRef.current
        ?.querySelector<HTMLElement>(`[data-cmd-index="${next}"]`)
        ?.scrollIntoView({ block: 'nearest' });
    } else if (e.key === 'Enter') {
      e.preventDefault();
      flat[current]?.run();
    }
  };
  let index = -1;

  return (
    <div
      onClick={() => setOpen(false)}
      className="fixed inset-0 z-80 flex items-start justify-center px-4"
      style={{
        background: 'var(--surface-overlay)',
        backdropFilter: 'blur(4px)',
        WebkitBackdropFilter: 'blur(4px)',
        paddingTop: '12vh',
        animation: 'or-fadein var(--motion-enter)',
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label={L.cmdPalette}
        onClick={(e) => e.stopPropagation()}
        className="w-[640px] max-w-full rounded-[14px] overflow-hidden bg-surface-2 border border-border-default"
        style={{ boxShadow: 'var(--elev-3)', animation: 'or-scalein var(--motion-enter)' }}
      >
        <div className="flex items-center gap-2.5 px-4 h-[52px] border-b border-border-subtle">
          {searching ? (
            <Loader2 size={17} strokeWidth={1.8} className="text-ink-muted animate-spin" />
          ) : (
            <Search size={17} strokeWidth={1.8} className="text-ink-muted" />
          )}
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setActive(0);
            }}
            onKeyDown={onInputKey}
            placeholder={L.cmdkPlaceholder}
            aria-label={L.cmdkPlaceholder}
            className="flex-1 h-full bg-transparent border-none outline-none focus-visible:outline-none text-ink text-[15px] placeholder:text-ink-muted"
          />
          <span className="mono text-[11px] px-1.5 py-0.5 rounded-[5px] border border-border-default text-ink-muted">
            {L.cmdEsc}
          </span>
        </div>

        <div ref={listRef} className="p-1.5 overflow-y-auto max-h-[400px]">
          {groups.map((g) => (
            <div key={g.label}>
              <div className="text-[10.5px] uppercase tracking-[0.06em] text-ink-muted font-semibold px-2.5 pt-2.5 pb-1.5">
                {g.label}
              </div>
              {g.items.map((it) => {
                const Icon = it.icon;
                index += 1;
                const i = index;
                const on = i === current;
                return (
                  <button
                    key={g.label + i}
                    data-cmd-index={i}
                    onClick={it.run}
                    onMouseMove={() => {
                      if (!on) setActive(i);
                    }}
                    className={`relative w-full flex items-center gap-3 h-10 px-2.5 rounded-[9px] text-left ${on ? 'bg-surface-3' : ''}`}
                  >
                    <span
                      aria-hidden="true"
                      className="absolute left-0 top-2.5 bottom-2.5 w-[2px] rounded-[2px]"
                      style={{ background: on ? 'var(--accent)' : 'transparent' }}
                    />
                    <Icon size={16} strokeWidth={1.75} className="text-ink-soft shrink-0 w-[18px]" />
                    <span className="flex-1 min-w-0 text-[13px] text-ink whitespace-nowrap overflow-hidden text-ellipsis">
                      {it.label}
                    </span>
                    {it.badge && (
                      <span
                        className="text-[11px] font-semibold px-2 py-0.5 rounded-full shrink-0"
                        style={{
                          color: it.badge.tone,
                          background: `color-mix(in srgb, ${it.badge.tone} 16%, transparent)`,
                        }}
                      >
                        {it.badge.text}
                      </span>
                    )}
                    {it.hint && (
                      <span
                        className={`text-[12px] text-ink-muted whitespace-nowrap max-w-[40%] overflow-hidden text-ellipsis ${it.hintMono ? 'mono' : ''}`}
                      >
                        {it.hint}
                      </span>
                    )}
                    {it.shortcut && (
                      <span className="text-[12px] text-ink-muted">{it.shortcut}</span>
                    )}
                  </button>
                );
              })}
            </div>
          ))}
          {groups.length === 0 && (
            <div className="p-7 text-center text-[13px] text-ink-muted">
              {searching ? L.cmdSearching : `${L.cmdNoResults} « ${query.trim()} ».`}
            </div>
          )}
        </div>

        <div className="flex gap-4 px-4 py-[9px] border-t border-border-subtle text-[11.5px] text-ink-muted">
          <span>↑↓ {L.navigate}</span>
          <span>↵ {L.open}</span>
          {can('risks:create') && (
            <span>N {L.cmdNewRiskHint}</span>
          )}
        </div>
      </div>
    </div>
  );
};
