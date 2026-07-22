// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
//
// "Deploy Agent" modal for an on-premise scan config. Mints a 24h registration
// token and shows the per-OS downloads + a ready-to-run command with the token
// embedded. The token is shown once — it enrols the agent for this tenant+config.

import { useEffect, useState } from 'react';
import { X, Copy, Check, Download, Apple, Container, Terminal, Monitor } from 'lucide-react';
import toast from 'react-hot-toast';
import { Btn } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { scannerService, type ScanConfig, type RegistrationTokenResponse } from './scannerService';

function detectOS(): 'windows' | 'macos' | 'linux' {
  const p = `${navigator.userAgent} ${navigator.platform}`.toLowerCase();
  if (p.includes('win')) return 'windows';
  if (p.includes('mac')) return 'macos';
  return 'linux';
}

export function AgentDeployModal({ config, onClose }: { config: ScanConfig; onClose: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const [data, setData] = useState<RegistrationTokenResponse | null>(null);
  const [err, setErr] = useState(false);
  const [copied, setCopied] = useState<string | null>(null);
  const os = detectOS();

  useEffect(() => {
    let alive = true;
    scannerService.registrationToken(config.id).then(
      (d) => alive && setData(d),
      () => alive && setErr(true),
    );
    return () => { alive = false; };
  }, [config.id]);

  const copy = (what: string, value: string) => {
    void navigator.clipboard.writeText(value);
    setCopied(what);
    toast.success(tr('Copié', 'Copied'));
    setTimeout(() => setCopied((c) => (c === what ? null : c)), 1500);
  };

  const dockerCmd = data
    ? `docker run -d --network host \\\n  -e OPENRISK_TOKEN=${data.registration_token.slice(0, 16)}… \\\n  ${data.downloads.docker}`
    : '';

  const osOptions: { key: 'windows' | 'macos' | 'linux' | 'docker'; label: string; icon: typeof Monitor }[] = [
    { key: 'windows', label: 'Windows (.exe)', icon: Monitor },
    { key: 'linux', label: 'Linux (binary)', icon: Terminal },
    { key: 'macos', label: 'macOS (.app)', icon: Apple },
    { key: 'docker', label: 'Docker', icon: Container },
  ];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0" style={{ background: 'rgba(0,0,0,.5)', backdropFilter: 'blur(2px)' }} onClick={onClose} />
      <div className="relative w-full max-w-[520px] max-h-[90vh] flex flex-col rounded-2xl glass-strong overflow-hidden" style={{ border: '1px solid var(--border-strong)', animation: 'or-scalein .22s ease' }}>
        <div className="flex items-center justify-between px-5 py-4" style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="text-[15px] font-semibold text-ink">{tr('Déployer un Agent', 'Deploy an Agent')}</div>
          <button onClick={onClose} className="w-8 h-8 rounded-lg flex items-center justify-center text-ink-soft hover:bg-hover" aria-label="Close"><X size={18} /></button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-5 flex flex-col gap-4">
          <p className="text-[13px] text-ink-soft leading-relaxed">
            {tr(
              `L'Agent OpenRisk s'installe sur une machine ayant accès au réseau interne, tourne en arrière-plan (service) et exécute les scans localement — aucune donnée sensible ne quitte votre infrastructure.`,
              `The OpenRisk Agent installs on a machine with internal-network access, runs in the background (as a service) and executes scans locally — no sensitive data leaves your infrastructure.`,
            )}
          </p>

          <div className="text-[12px] font-semibold text-ink-soft">{tr('Télécharger pour', 'Download for')} <span className="text-ink">{config.name}</span></div>
          <div className="grid grid-cols-2 gap-2.5">
            {osOptions.map((o) => (
              <a
                key={o.key}
                href={data ? o.key === 'docker' ? undefined : new URL(data.downloads[o.key], api_origin()).href : undefined}
                target="_blank"
                rel="noreferrer"
                onClick={o.key === 'docker' && data ? (e) => { e.preventDefault(); copy('docker', dockerCmd); } : undefined}
                className="flex items-center gap-2.5 rounded-xl px-3.5 py-3 transition-all hover:brightness-110"
                style={{
                  border: `1px solid ${o.key === os ? 'var(--accent)' : 'var(--border-strong)'}`,
                  background: o.key === os ? 'var(--accent-soft)' : 'var(--bg-elevated)',
                  cursor: data ? 'pointer' : 'not-allowed',
                  opacity: data ? 1 : 0.6,
                }}
              >
                <o.icon size={18} strokeWidth={1.7} style={{ color: o.key === os ? 'var(--accent)' : 'var(--text-secondary)' }} />
                <span className="text-[12.5px] font-semibold text-ink">{o.label}</span>
                {o.key === os && <span className="ml-auto text-[10px] font-bold uppercase" style={{ color: 'var(--accent)' }}>{tr('détecté', 'detected')}</span>}
              </a>
            ))}
          </div>

          {err && <div className="text-[12.5px]" style={{ color: 'var(--critical)' }}>{tr("Impossible de générer le jeton d'enrôlement.", 'Could not generate the enrolment token.')}</div>}

          {data && (
            <>
              <div className="flex flex-col gap-1.5">
                <div className="flex items-center justify-between">
                  <span className="text-[12px] font-semibold text-ink-soft">{tr("Jeton d'enrôlement (valide 24h)", 'Registration token (valid 24h)')}</span>
                  <button onClick={() => copy('token', data.registration_token)} className="inline-flex items-center gap-1.5 text-[11.5px] font-semibold" style={{ color: 'var(--accent)' }}>
                    {copied === 'token' ? <Check size={13} /> : <Copy size={13} />}{tr('Copier', 'Copy')}
                  </button>
                </div>
                <div className="font-mono text-[11px] rounded-lg px-3 py-2.5 break-all" style={{ background: 'var(--bg-hover)', color: 'var(--text-secondary)' }}>
                  {data.registration_token.slice(0, 44)}…
                </div>
              </div>

              <div className="flex flex-col gap-1.5">
                <span className="text-[12px] font-semibold text-ink-soft">{tr('Lancer via Docker', 'Run with Docker')}</span>
                <div className="relative">
                  <pre className="font-mono text-[11px] rounded-lg px-3 py-2.5 overflow-x-auto" style={{ background: 'var(--bg-hover)', color: 'var(--text-secondary)' }}>{dockerCmd}</pre>
                  <button onClick={() => copy('docker', dockerCmd)} className="absolute top-2 right-2 text-ink-soft hover:text-ink" aria-label="Copy">
                    {copied === 'docker' ? <Check size={14} /> : <Copy size={14} />}
                  </button>
                </div>
              </div>
            </>
          )}
        </div>

        <div className="flex items-center justify-between gap-2.5 px-5 py-4" style={{ borderTop: '1px solid var(--border)' }}>
          <span className="text-[11.5px] text-ink-muted inline-flex items-center gap-1.5"><Download size={13} /> {tr('< 15 Mo · démarre au boot', '< 15 MB · starts on boot')}</span>
          <Btn label={tr('Fermer', 'Close')} onClick={onClose} />
        </div>
      </div>
    </div>
  );
}

// The download paths are backend-relative; resolve them against the API origin
// (the Vite dev server on :5173 doesn't serve them).
function api_origin(): string {
  return 'http://localhost:8080';
}
