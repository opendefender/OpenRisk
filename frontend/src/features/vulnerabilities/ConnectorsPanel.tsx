// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// The supported vulnerability integrations, grouped by category. Every provider
// supports normalised import today; live API polling is flagged per provider
// (implemented for AWS Inspector; the others activate when creds are configured).

import { X, Server, Cpu, Cloud, Upload, Radio } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useVulnConnectors } from './useVulnerabilities';

const CATEGORY = {
  network_scanner: { label: ['Scanner réseau', 'Network scanner'], icon: Server, color: 'var(--accent)' },
  edr: { label: ['EDR / Endpoint', 'EDR / Endpoint'], icon: Cpu, color: 'var(--high)' },
  cloud: { label: ['Cloud', 'Cloud'], icon: Cloud, color: 'var(--info)' },
} as const;

export function ConnectorsPanel({ isOpen, onClose, onImport }: { isOpen: boolean; onClose: () => void; onImport: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data: connectors, isLoading } = useVulnConnectors();
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-[80] flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,.5)', backdropFilter: 'blur(3px)' }} onClick={onClose}>
      <div onClick={(e) => e.stopPropagation()} className="w-full max-w-[620px] rounded-[16px] flex flex-col" style={{ maxHeight: '90vh', background: 'var(--bg-secondary)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)' }}>
        <div className="flex items-center justify-between px-5 py-4" style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="text-[15px] font-bold text-ink">{tr('Connecteurs de vulnérabilités', 'Vulnerability connectors')}</div>
          <button onClick={onClose} className="w-8 h-8 rounded-[9px] flex items-center justify-center text-ink-soft" style={{ background: 'var(--bg-hover)' }}><X size={18} /></button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4 space-y-2.5">
          {isLoading ? (
            <div className="text-[13px] text-ink-muted py-6 text-center">{tr('Chargement…', 'Loading…')}</div>
          ) : (
            (connectors ?? []).map((c) => {
              const cat = CATEGORY[c.category];
              const Icon = cat.icon;
              return (
                <div key={c.source} className="flex items-center gap-3.5 rounded-[12px] p-3.5" style={{ border: '1px solid var(--border)' }}>
                  <div className="w-10 h-10 rounded-[10px] flex items-center justify-center shrink-0" style={{ background: `color-mix(in srgb, ${cat.color} 14%, transparent)`, color: cat.color }}>
                    <Icon size={18} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="text-[13.5px] font-semibold text-ink">{c.label}</div>
                    <div className="text-[12px] text-ink-muted">{tr(cat.label[0], cat.label[1])} · {c.notes}</div>
                  </div>
                  <div className="flex items-center gap-1.5 shrink-0">
                    <span className="inline-flex items-center gap-1 h-[22px] px-2 rounded-full text-[10.5px] font-semibold" style={{ color: 'var(--low)', background: 'color-mix(in srgb,var(--low) 14%,transparent)' }}>
                      <Upload size={11} /> {tr('Import', 'Import')}
                    </span>
                    {c.live_pull && (
                      <span className="inline-flex items-center gap-1 h-[22px] px-2 rounded-full text-[10.5px] font-semibold" style={{ color: 'var(--accent)', background: 'var(--accent-soft)' }}>
                        <Radio size={11} /> {tr('Live', 'Live')}
                      </span>
                    )}
                  </div>
                </div>
              );
            })
          )}
        </div>

        <div className="px-5 py-4 flex justify-end gap-2" style={{ borderTop: '1px solid var(--border)' }}>
          <button onClick={onImport} className="h-9 px-4 rounded-[9px] text-[13px] font-semibold text-white inline-flex items-center gap-1.5" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}>
            <Upload size={15} /> {tr('Importer des findings', 'Import findings')}
          </button>
        </div>
      </div>
    </div>
  );
}
