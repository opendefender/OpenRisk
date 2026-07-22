// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// ⌘K / Ctrl+K command palette (OpenRisk.dc.html §8). Glass overlay, autofocused
// input, grouped Navigation + Quick actions filtered as you type. Esc / backdrop
// closes. Registers the global ⌘K keyboard shortcut itself.

import { useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, FileText, Sun, Moon, Languages, Search, type LucideIcon } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useUIStrings } from '../../shared/uiStrings';
import { NAV_GROUPS } from '../../shared/navModel';

interface CmdItem {
  label: string;
  icon: LucideIcon;
  shortcut?: string;
  run: () => void;
}
interface CmdGroup {
  label: string;
  items: CmdItem[];
}

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
  const [query, setQuery] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

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
      // Autofocus after the mount animation begins.
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }, [open]);

  const groups: CmdGroup[] = useMemo(() => {
    const close = () => setOpen(false);
    const nav: CmdItem[] = NAV_GROUPS.flatMap((g) =>
      g.items.map((it) => ({
        label: L[it.labelKey],
        icon: it.icon,
        run: () => {
          navigate(it.path);
          close();
        },
      }))
    );
    const actions: CmdItem[] = [
      { label: L.newRisk, icon: Plus, shortcut: 'N', run: () => { window.dispatchEvent(new CustomEvent('openrisk:new-risk')); close(); } },
      { label: L.genReport, icon: FileText, run: () => { navigate('/reports'); close(); } },
      {
        label: lang === 'fr' ? (theme === 'dark' ? 'Thème clair' : 'Thème sombre') : theme === 'dark' ? 'Light theme' : 'Dark theme',
        icon: theme === 'dark' ? Sun : Moon,
        run: () => { toggleTheme(); close(); },
      },
      {
        label: lang === 'fr' ? 'English' : 'Français',
        icon: Languages,
        run: () => { toggleLang(); close(); },
      },
    ];
    const q = query.trim().toLowerCase();
    const flt = (items: CmdItem[]) => (q ? items.filter((i) => i.label.toLowerCase().includes(q)) : items);
    return [
      { label: lang === 'fr' ? 'Navigation' : 'Navigation', items: flt(nav) },
      { label: lang === 'fr' ? 'Actions rapides' : 'Quick actions', items: flt(actions) },
    ].filter((g) => g.items.length > 0);
  }, [L, query, navigate, setOpen, theme, lang, toggleTheme, toggleLang]);

  if (!open) return null;

  const runFirst = () => {
    const first = groups[0]?.items[0];
    first?.run();
  };

  return (
    <div
      onClick={() => setOpen(false)}
      className="fixed inset-0 z-[80] flex items-start justify-center"
      style={{
        background: 'rgba(0,0,0,0.5)',
        backdropFilter: 'blur(6px)',
        WebkitBackdropFilter: 'blur(6px)',
        paddingTop: '14vh',
        animation: 'or-fadein .16s ease',
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="glass-strong rounded-[18px] overflow-hidden shadow-card-lg"
        style={{ width: 'min(92vw,600px)', animation: 'or-scalein .18s cubic-bezier(.2,.8,.2,1)' }}
      >
        <div className="flex items-center gap-[11px] px-[18px] py-4 border-b border-border">
          <Search size={18} strokeWidth={1.8} className="text-ink-muted" />
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && runFirst()}
            placeholder={L.cmdkPlaceholder}
            className="flex-1 bg-transparent border-none outline-none text-ink text-[15px] placeholder:text-ink-muted"
          />
          <span className="mono text-[10.5px] px-[7px] py-[3px] rounded-md text-ink-muted" style={{ background: 'var(--bg-hover)', border: '1px solid var(--border)' }}>
            ESC
          </span>
        </div>

        <div className="p-2 overflow-y-auto" style={{ maxHeight: '52vh' }}>
          {groups.map((g) => (
            <div key={g.label} className="mb-1.5">
              <div className="text-[10.5px] uppercase tracking-[0.08em] text-ink-muted font-semibold px-3 pt-2 pb-[5px]">
                {g.label}
              </div>
              {g.items.map((it, i) => {
                const Icon = it.icon;
                return (
                  <button
                    key={g.label + i}
                    onClick={it.run}
                    className="w-full flex items-center gap-3 px-3 py-[9px] rounded-[10px] hover:bg-accent-soft transition-colors text-left"
                  >
                    <span className="text-ink-soft flex">
                      <Icon size={17} strokeWidth={1.75} />
                    </span>
                    <span className="flex-1 text-[13.5px] text-ink">{it.label}</span>
                    {it.shortcut && (
                      <span className="mono text-[10.5px] px-1.5 py-0.5 rounded text-ink-muted" style={{ background: 'var(--bg-hover)', border: '1px solid var(--border)' }}>
                        {it.shortcut}
                      </span>
                    )}
                  </button>
                );
              })}
            </div>
          ))}
          {groups.length === 0 && (
            <div className="px-3 py-6 text-center text-[13px] text-ink-muted">{L.notifEmpty}</div>
          )}
        </div>

        <div className="px-4 py-2.5 border-t border-border text-[11px] text-ink-muted text-center">
          ↑↓ {L.navigate} · ↵ {L.open} · esc {L.close}
        </div>
      </div>
    </div>
  );
};
