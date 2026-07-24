// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Global glass header (OpenRisk.dc.html §5): breadcrumb · ⌘K search · realtime
// dot · language · voice · notifications · theme. Sticky, backdrop-blurred, sits
// above the scrollable body. On < lg it collapses to a hamburger + brand.

import { useMemo, useState } from 'react';
import { useLocation } from 'react-router-dom';
import {
  Search, Bell, Sun, Moon, Mic, Menu, ChevronRight, AlertTriangle, Siren, ShieldCheck, Trophy,
  Rows2, Rows3, Rows4,
  type LucideIcon,
} from 'lucide-react';
import { cn } from '../ui/Button';
import { useUIStore } from '../../store/uiStore';
import { useUIStrings } from '../../shared/uiStrings';
import { ALL_NAV_ITEMS } from '../../shared/navModel';

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
  const { pathname } = useLocation();
  const [notifOpen, setNotifOpen] = useState(false);

  const breadcrumb = useMemo(() => {
    let best = '';
    let bestLen = -1;
    for (const it of ALL_NAV_ITEMS) {
      const p = it.path;
      const match = p === '/' ? pathname === '/' : pathname === p || pathname.startsWith(p + '/');
      if (match && p.length > bestLen) {
        best = L[it.labelKey];
        bestLen = p.length;
      }
    }
    return best || L.soon;
  }, [pathname, L]);

  return (
    <header
      className="h-[58px] shrink-0 flex items-center gap-3 px-3 sm:px-[18px] border-b border-border sticky top-0 z-50 glass"
    >
      {/* Mobile hamburger */}
      <button onClick={onOpenMobileNav} className={cn(iconBtn, 'lg:hidden')} aria-label="Open navigation">
        <Menu size={18} />
      </button>

      {/* Breadcrumb */}
      <div className="flex items-center gap-2 text-[13px] min-w-0">
        <span className="text-ink-muted hidden sm:inline">{L.brandShort}</span>
        <ChevronRight size={13} className="text-ink-muted hidden sm:inline shrink-0" />
        <span className="text-ink font-medium whitespace-nowrap truncate">{breadcrumb}</span>
      </div>

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

        <button onClick={cycleDensity} className={cn(iconBtn, 'hidden sm:flex')} title={densityMeta.label} aria-label={densityMeta.label}>
          <densityMeta.Icon size={18} strokeWidth={1.7} />
        </button>

        <button className={cn(iconBtn, 'hidden sm:flex')} title="Voice assistant" aria-label="Voice assistant">
          <Mic size={18} strokeWidth={1.7} />
        </button>

        {/* Notifications */}
        <div className="relative">
          <button onClick={() => setNotifOpen((v) => !v)} className={cn(iconBtn, 'relative')} title={L.notifTitle} aria-label={L.notifTitle}>
            <Bell size={18} strokeWidth={1.7} />
            <span
              className="absolute top-[5px] right-[5px] w-[7px] h-[7px] rounded-full"
              style={{ background: 'var(--critical)', border: '1.5px solid var(--glass)' }}
            />
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
function NotifPanel({ onClose }: { onClose: () => void }) {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const items: {
    icon: LucideIcon; color: string; title: string; body: string; time: string; unread: boolean;
  }[] = [
    { icon: AlertTriangle, color: 'var(--critical)', title: tr('Nouveau risque critique', 'New critical risk'), body: tr('RDP exposé détecté sur srv-paie-01', 'Exposed RDP detected on srv-paie-01'), time: tr('il y a 8 min', '8 min ago'), unread: true },
    { icon: Siren, color: 'var(--critical)', title: tr('Incident déclenché', 'Incident triggered'), body: tr('INC-2026-014 · exfiltration suspectée', 'INC-2026-014 · suspected exfiltration'), time: tr('il y a 1 h', '1 h ago'), unread: true },
    { icon: ShieldCheck, color: 'var(--low)', title: tr('Mitigation auto-détectée', 'Auto-detected mitigation'), body: tr('TLS 1.3 appliqué sur gw-bank-02', 'TLS 1.3 applied on gw-bank-02'), time: tr('il y a 3 h', '3 h ago'), unread: false },
    { icon: Trophy, color: 'var(--accent)', title: tr('Badge débloqué', 'Badge unlocked'), body: tr('Vous avez atteint le rang #4', 'You reached rank #4'), time: tr('hier', 'yesterday'), unread: false },
  ];

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
          <button onClick={onClose} className="text-[12px] font-medium text-accent hover:brightness-110">
            {L.notifAll}
          </button>
        </div>
        <div className="max-h-[380px] overflow-y-auto">
          {items.map((it, i) => {
            const Icon = it.icon;
            return (
              <div
                key={i}
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
                  <div className="text-[12px] text-ink-soft leading-snug">{it.body}</div>
                  <div className="text-[11px] text-ink-muted mt-1">{it.time}</div>
                </div>
                {it.unread && <span className="w-[7px] h-[7px] rounded-full shrink-0 mt-1.5" style={{ background: 'var(--accent)' }} />}
              </div>
            );
          })}
        </div>
        <button onClick={onClose} className="w-full py-[13px] text-[13px] font-semibold text-accent hover:brightness-110">
          {L.notifViewAll}
        </button>
      </div>
    </>
  );
}
