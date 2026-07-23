// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Global keyboard shortcuts (UX-7 / directive §6). Frequent actions get a key so
// power users never reach for the mouse: N = new risk, / = search (⌘K), G then a
// letter = go to a section, ? = this help. Typing in a field is never hijacked.
// ⌘K itself is owned by CommandPalette; this layer covers the rest.

import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { X } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';

// G-then-key jump targets.
const GOTO: Record<string, string> = {
  d: '/', r: '/risks', v: '/vulnerabilities', m: '/mitigations',
  i: '/incidents', c: '/compliance', a: '/assets', s: '/settings',
};

function isTyping(el: EventTarget | null): boolean {
  const t = el as HTMLElement | null;
  if (!t) return false;
  const tag = t.tagName;
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || t.isContentEditable;
}

export function GlobalShortcuts() {
  const navigate = useNavigate();
  const setCmdkOpen = useUIStore((s) => s.setCmdkOpen);
  const [helpOpen, setHelpOpen] = useState(false);
  const gPending = useRef(false);
  const gTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.metaKey || e.ctrlKey || e.altKey) return; // leave chords (⌘K…) alone
      if (e.key === 'Escape') { setHelpOpen(false); gPending.current = false; return; }
      if (isTyping(e.target)) return;

      // Second key of a "G then X" jump.
      if (gPending.current) {
        gPending.current = false;
        if (gTimer.current) clearTimeout(gTimer.current);
        const dest = GOTO[e.key.toLowerCase()];
        if (dest) { e.preventDefault(); navigate(dest); }
        return;
      }

      switch (e.key) {
        case '?':
          e.preventDefault();
          setHelpOpen((v) => !v);
          break;
        case '/':
          e.preventDefault();
          setCmdkOpen(true);
          break;
        case 'n':
        case 'N':
          e.preventDefault();
          window.dispatchEvent(new CustomEvent('openrisk:new-risk'));
          break;
        case 'g':
        case 'G':
          e.preventDefault();
          gPending.current = true;
          if (gTimer.current) clearTimeout(gTimer.current);
          gTimer.current = setTimeout(() => { gPending.current = false; }, 1200);
          break;
      }
    };
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('keydown', onKey);
      if (gTimer.current) clearTimeout(gTimer.current);
    };
  }, [navigate, setCmdkOpen]);

  if (!helpOpen) return null;
  return <ShortcutsHelp onClose={() => setHelpOpen(false)} />;
}

function ShortcutsHelp({ onClose }: { onClose: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const groups: { title: string; items: [string, string][] }[] = [
    {
      title: tr('Actions', 'Actions'),
      items: [
        ['N', tr('Nouveau risque', 'New risk')],
        ['/', tr('Rechercher (palette)', 'Search (palette)')],
        ['⌘K', tr('Palette de commandes', 'Command palette')],
        ['?', tr('Cette aide', 'This help')],
      ],
    },
    {
      title: tr('Aller à… (G puis)', 'Go to… (G then)'),
      items: [
        ['G D', tr('Tableau de bord', 'Dashboard')],
        ['G R', tr('Risques', 'Risks')],
        ['G V', tr('Vulnérabilités', 'Vulnerabilities')],
        ['G M', tr('Mitigations', 'Mitigations')],
        ['G I', tr('Incidents', 'Incidents')],
        ['G C', tr('Conformité', 'Compliance')],
        ['G A', tr('Actifs', 'Assets')],
        ['G S', tr('Paramètres', 'Settings')],
      ],
    },
  ];

  return (
    <div
      className="fixed inset-0 z-[95] flex items-center justify-center p-4"
      style={{ background: 'rgba(0,0,0,0.5)', backdropFilter: 'blur(6px)', WebkitBackdropFilter: 'blur(6px)', animation: 'or-fadein .16s ease' }}
      onClick={onClose}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-[460px] rounded-[16px] overflow-hidden shadow-card-lg"
        style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', animation: 'or-scalein .18s cubic-bezier(.2,.8,.2,1)' }}
      >
        <div className="flex items-center justify-between px-5 py-4 border-b border-border">
          <span className="text-[15px] font-bold text-ink">{tr('Raccourcis clavier', 'Keyboard shortcuts')}</span>
          <button onClick={onClose} className="w-8 h-8 rounded-lg flex items-center justify-center text-ink-muted hover:bg-hover transition-colors" aria-label={tr('Fermer', 'Close')}><X size={16} /></button>
        </div>
        <div className="p-5 grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-4 max-h-[70vh] overflow-y-auto">
          {groups.map((g) => (
            <div key={g.title}>
              <div className="text-[10.5px] font-semibold uppercase tracking-[.07em] text-ink-muted mb-2">{g.title}</div>
              <div className="space-y-1.5">
                {g.items.map(([key, label]) => (
                  <div key={key} className="flex items-center justify-between gap-3">
                    <span className="text-[13px] text-ink-soft">{label}</span>
                    <kbd className="mono text-[11px] font-semibold px-1.5 py-0.5 rounded text-ink-muted shrink-0" style={{ background: 'var(--bg-hover)', border: '1px solid var(--border)' }}>{key}</kbd>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
