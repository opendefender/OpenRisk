// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Post-victory personalization (UX-5 / directive §2). Once a user has felt the
// value (created their first risk), let them make the space theirs — theme + accent.
// Both are already wired into the token system (data-theme / data-variant), so this
// is a thin, reusable picker used in onboarding and in Settings.

import { Sun, Moon } from 'lucide-react';
import { useUIStore, type Theme, type Variant } from '../../store/uiStore';

const ACCENTS: { key: Variant; label: string; color: string }[] = [
  { key: 'azure', label: 'Azure', color: '#0a84ff' },
  { key: 'iris', label: 'Iris', color: '#7c6cff' },
];

export function PersonalizeCard({ compact }: { compact?: boolean }) {
  const theme = useUIStore((s) => s.theme);
  const setTheme = useUIStore((s) => s.setTheme);
  const variant = useUIStore((s) => s.variant);
  const setVariant = useUIStore((s) => s.setVariant);
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const themeBtn = (t: Theme, Icon: typeof Sun, label: string) => {
    const active = theme === t;
    return (
      <button
        onClick={() => setTheme(t)}
        className="flex-1 flex items-center justify-center gap-2 h-9 rounded-[10px] text-[12.5px] font-semibold transition-colors"
        style={{
          background: active ? 'var(--accent-soft)' : 'var(--bg-hover)',
          color: active ? 'var(--accent)' : 'var(--text-secondary)',
          border: `1px solid ${active ? 'var(--accent)' : 'transparent'}`,
        }}
      >
        <Icon size={15} /> {label}
      </button>
    );
  };

  return (
    <div className={compact ? '' : 'space-y-4'}>
      <div>
        <div className="text-[11px] font-semibold uppercase tracking-[.06em] text-ink-muted mb-2">{tr('Thème', 'Theme')}</div>
        <div className="flex gap-2">
          {themeBtn('light', Sun, tr('Clair', 'Light'))}
          {themeBtn('dark', Moon, tr('Sombre', 'Dark'))}
        </div>
      </div>
      <div className={compact ? 'mt-3' : ''}>
        <div className="text-[11px] font-semibold uppercase tracking-[.06em] text-ink-muted mb-2">{tr('Couleur d’accent', 'Accent color')}</div>
        <div className="flex gap-2.5">
          {ACCENTS.map((a) => {
            const active = variant === a.key;
            return (
              <button
                key={a.key}
                onClick={() => setVariant(a.key)}
                className="flex items-center gap-2 h-9 px-3 rounded-[10px] text-[12.5px] font-semibold transition-colors"
                style={{ background: 'var(--bg-hover)', border: `1px solid ${active ? a.color : 'transparent'}`, color: active ? 'var(--text-primary)' : 'var(--text-secondary)' }}
              >
                <span className="w-4 h-4 rounded-full" style={{ background: a.color, boxShadow: active ? `0 0 0 2px var(--bg-hover), 0 0 0 3px ${a.color}` : 'none' }} />
                {a.label}
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}
