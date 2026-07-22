// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
//
// Slide-in drawer to create a scan configuration. Cloud providers show their
// credential fields (encrypted server-side, never returned); on-premise shows
// the CIDR/host targets (validated ≤ /24 by the backend). Credentials only ever
// leave the browser over TLS in the create request — they are never echoed back.

import { useState } from 'react';
import { X, ShieldCheck, Info } from 'lucide-react';
import toast from 'react-hot-toast';
import { Btn } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { PROVIDERS, CLOUD_CRED_FIELDS, SCOPE_HINTS } from './scannerMeta';
import type { ScannerProvider, CreateScanConfigInput } from './scannerService';

const inputStyle: React.CSSProperties = {
  background: 'var(--bg-elevated)',
  border: '1px solid var(--border-strong)',
  borderRadius: 10,
  minHeight: 38,
  padding: '9px 12px',
  fontSize: 13,
  color: 'var(--text-primary)',
  outline: 'none',
  width: '100%',
};
const monoStyle: React.CSSProperties = { ...inputStyle, fontFamily: 'var(--font-mono, monospace)', fontSize: 12 };

export function ScanConfigDrawer({
  open,
  provider,
  onClose,
  onCreate,
  submitting,
}: {
  open: boolean;
  provider: ScannerProvider;
  onClose: () => void;
  onCreate: (input: CreateScanConfigInput) => Promise<void>;
  submitting: boolean;
}) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const meta = PROVIDERS[provider];
  const isCloud = meta.cloud;

  const [name, setName] = useState('');
  const [creds, setCreds] = useState<Record<string, string>>({});
  const [regions, setRegions] = useState('');
  const [targets, setTargets] = useState('');
  const [scheduleMin, setScheduleMin] = useState(0);

  if (!open) return null;

  const setCred = (k: string, v: string) => setCreds((c) => ({ ...c, [k]: v }));

  const submit = async () => {
    if (!name.trim()) return toast.error(tr('Un nom est requis', 'A name is required'));
    const input: CreateScanConfigInput = { name: name.trim(), provider, schedule_minutes: scheduleMin };
    if (isCloud) {
      const cleaned: Record<string, string> = {};
      for (const [k, v] of Object.entries(creds)) if (v.trim()) cleaned[k] = v.trim();
      input.credentials = cleaned;
      const regs = regions.split(',').map((r) => r.trim()).filter(Boolean);
      if (regs.length) input.regions = regs;
    } else {
      const tg = targets.split(/[\n,]/).map((t) => t.trim()).filter(Boolean);
      if (!tg.length) return toast.error(tr('Au moins une cible est requise', 'At least one target is required'));
      input.targets = tg;
    }
    try {
      await onCreate(input);
    } catch (e) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      toast.error(msg ?? tr('Échec de la création', 'Creation failed'));
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      <div className="absolute inset-0" style={{ background: 'rgba(0,0,0,.45)', backdropFilter: 'blur(2px)' }} onClick={onClose} />
      <aside
        className="relative h-full w-full max-w-[460px] flex flex-col glass-strong"
        style={{ borderLeft: '1px solid var(--border-strong)', animation: 'or-slidein .28s ease' }}
      >
        {/* header */}
        <div className="flex items-center justify-between px-5 py-4" style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-[10px] flex items-center justify-center" style={{ background: `color-mix(in srgb,${meta.color} 15%,transparent)`, color: meta.color }}>
              <meta.icon size={18} strokeWidth={1.8} />
            </div>
            <div>
              <div className="text-[15px] font-semibold text-ink">{tr('Nouvelle configuration', 'New scan config')}</div>
              <div className="text-[12px] text-ink-soft">{meta.short}</div>
            </div>
          </div>
          <button onClick={onClose} className="w-8 h-8 rounded-lg flex items-center justify-center text-ink-soft hover:bg-hover" aria-label="Close"><X size={18} /></button>
        </div>

        {/* body */}
        <div className="flex-1 overflow-y-auto px-5 py-5 flex flex-col gap-4">
          <Field label={tr('Nom', 'Name')}>
            <input style={inputStyle} value={name} onChange={(e) => setName(e.target.value)} placeholder={isCloud ? tr('Production AWS', 'Production AWS') : tr('Sweep réseau siège', 'HQ network sweep')} />
          </Field>

          {isCloud ? (
            <>
              {(CLOUD_CRED_FIELDS[provider] ?? []).map((f) => (
                <Field key={f.key} label={f.label}>
                  {f.kind === 'textarea' ? (
                    <textarea style={monoStyle} rows={4} value={creds[f.key] ?? ''} onChange={(e) => setCred(f.key, e.target.value)} placeholder={f.placeholder} />
                  ) : (
                    <input type={f.kind === 'password' ? 'password' : 'text'} autoComplete="off" style={inputStyle} value={creds[f.key] ?? ''} onChange={(e) => setCred(f.key, e.target.value)} placeholder={f.placeholder} />
                  )}
                </Field>
              ))}
              {SCOPE_HINTS[provider] && (
                <Field label={tr(SCOPE_HINTS[provider]!.fr, SCOPE_HINTS[provider]!.en)}>
                  <input style={inputStyle} value={regions} onChange={(e) => setRegions(e.target.value)} placeholder={SCOPE_HINTS[provider]!.placeholder} />
                </Field>
              )}
              <div className="flex items-start gap-2 text-[12px] text-ink-soft rounded-lg px-3 py-2.5" style={{ background: 'var(--bg-hover)' }}>
                <ShieldCheck size={15} className="shrink-0 mt-[1px]" style={{ color: 'var(--low)' }} />
                <span>{tr('Chiffré en AES-256-GCM, déchiffré uniquement au moment du scan.', 'Encrypted with AES-256-GCM, decrypted only at scan time.')}</span>
              </div>
            </>
          ) : (
            <>
              <Field label={tr('Cibles (une par ligne — CIDR ≤ /24 ou hôte)', 'Targets (one per line — CIDR ≤ /24 or host)')}>
                <textarea style={monoStyle} rows={5} value={targets} onChange={(e) => setTargets(e.target.value)} placeholder={'10.0.0.0/24\n192.168.1.10\nsrv-db-01'} />
              </Field>
              <div className="flex items-start gap-2 text-[12px] text-ink-soft rounded-lg px-3 py-2.5" style={{ background: 'var(--bg-hover)' }}>
                <Info size={15} className="shrink-0 mt-[1px]" style={{ color: 'var(--info)' }} />
                <span>{tr('Le scan est exécuté par un Agent sur site (nmap/osquery), jamais par le SaaS. Déployez un agent après la création.', 'Scans run on an on-prem Agent (nmap/osquery), never on the SaaS. Deploy an agent after creating.')}</span>
              </div>
            </>
          )}

          <Field label={tr('Fréquence automatique', 'Automatic schedule')}>
            <select value={scheduleMin} onChange={(e) => setScheduleMin(Number(e.target.value))} style={inputStyle}>
              <option value={0}>{tr('Manuel uniquement', 'Manual only')}</option>
              <option value={60}>{tr('Toutes les heures', 'Hourly')}</option>
              <option value={1440}>{tr('Quotidien', 'Daily')}</option>
              <option value={10080}>{tr('Hebdomadaire', 'Weekly')}</option>
            </select>
          </Field>
        </div>

        {/* footer */}
        <div className="flex items-center justify-end gap-2.5 px-5 py-4" style={{ borderTop: '1px solid var(--border)' }}>
          <Btn label={tr('Annuler', 'Cancel')} onClick={onClose} />
          <Btn label={submitting ? tr('Création…', 'Creating…') : tr('Créer', 'Create')} primary onClick={submit} />
        </div>
      </aside>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-[12px] font-semibold text-ink-soft">{label}</span>
      {children}
    </label>
  );
}
