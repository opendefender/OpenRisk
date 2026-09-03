// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Public status page (task §4). Reachable without auth; polls GET /api/v1/status
// (which probes the real dependencies) and shows overall + per-component health.
// A status page must be truthful and reachable exactly when things are degraded,
// so it depends on nothing but a fetch.

import { useEffect, useState } from 'react';
import { CheckCircle2, AlertTriangle, XCircle, RefreshCw } from 'lucide-react';
import { OpenRiskLogo } from '../../shared/Logo';
import { useUIStore } from '../../store/uiStore';

interface Component {
  name: string;
  status: string;
}
interface Status {
  status: string;
  version?: string;
  components: Component[];
}

const OVERALL: Record<
  string,
  { label: [string, string]; tone: string; icon: typeof CheckCircle2 }
> = {
  operational: {
    label: ['Tous les systèmes sont opérationnels', 'All systems operational'],
    tone: 'var(--low)',
    icon: CheckCircle2,
  },
  degraded: {
    label: ['Performances dégradées', 'Degraded performance'],
    tone: 'var(--medium)',
    icon: AlertTriangle,
  },
  major_outage: {
    label: ['Panne majeure', 'Major outage'],
    tone: 'var(--critical)',
    icon: XCircle,
  },
};

const COMPONENT_LABEL: Record<string, [string, string]> = {
  api: ['API', 'API'],
  database: ['Base de données', 'Database'],
  redis: ['Cache / files', 'Cache / queues'],
};

export function StatusPage() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const [status, setStatus] = useState<Status | null>(null);
  const [error, setError] = useState(false);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/v1/status', { headers: { Accept: 'application/json' } });
      if (!res.ok) throw new Error(String(res.status));
      setStatus(await res.json());
      setError(false);
    } catch {
      setError(true);
      setStatus(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
    const id = window.setInterval(load, 30_000);
    return () => window.clearInterval(id);
  }, []);

  const overallKey = error ? 'major_outage' : (status?.status ?? 'operational');
  const overall = OVERALL[overallKey] ?? OVERALL.operational;
  const OverallIcon = overall.icon;

  return (
    <div
      className="min-h-screen w-full flex items-start justify-center p-6"
      style={{ background: 'var(--bg-primary)', color: 'var(--fg-primary)' }}
    >
      <div className="max-w-lg w-full mt-16">
        <div className="flex items-center justify-between mb-8">
          <div className="flex items-center gap-2.5">
            <OpenRiskLogo size={26} />
            <span className="disp text-[17px] font-bold text-ink">
              OpenRisk · {tr('État', 'Status')}
            </span>
          </div>
          <button
            onClick={load}
            aria-label={tr('Actualiser', 'Refresh')}
            className="text-ink-soft hover:text-ink transition-colors"
          >
            <RefreshCw size={16} className={loading ? 'animate-spin' : ''} />
          </button>
        </div>

        <div
          role="status"
          aria-live="polite"
          className="rounded-[16px] p-5 mb-4 flex items-center gap-3"
          style={{ background: 'var(--bg-elev)', border: '1px solid var(--border)' }}
        >
          <span
            className="inline-flex items-center justify-center w-10 h-10 rounded-xl"
            style={{
              background: `color-mix(in srgb,${overall.tone} 16%,transparent)`,
              color: overall.tone,
            }}
          >
            <OverallIcon size={20} />
          </span>
          <div>
            <div className="text-[15px] font-semibold text-ink">{tr(...overall.label)}</div>
            {status?.version && (
              <div className="text-[12px] text-ink-muted mono">v{status.version}</div>
            )}
          </div>
        </div>

        {error ? (
          <div
            className="rounded-[16px] p-5 text-[13px] text-ink-soft"
            style={{ background: 'var(--bg-elev)', border: '1px solid var(--border)' }}
          >
            {tr(
              'Impossible de joindre le service. Il est probablement indisponible.',
              'Cannot reach the service. It is likely unavailable.',
            )}
          </div>
        ) : (
          <div
            className="rounded-[16px] overflow-hidden"
            style={{ background: 'var(--bg-elev)', border: '1px solid var(--border)' }}
          >
            {(status?.components ?? []).map((comp, i) => {
              const ok = comp.status === 'operational';
              const label = COMPONENT_LABEL[comp.name] ?? [comp.name, comp.name];
              return (
                <div
                  key={comp.name}
                  className="flex items-center justify-between px-5 py-3.5"
                  style={{ borderTop: i === 0 ? 'none' : '1px solid var(--border)' }}
                >
                  <span className="text-[13.5px] text-ink">{tr(...label)}</span>
                  <span
                    className="flex items-center gap-1.5 text-[12.5px] font-medium"
                    style={{ color: ok ? 'var(--low)' : 'var(--critical)' }}
                  >
                    {ok ? <CheckCircle2 size={14} /> : <XCircle size={14} />}
                    {ok ? tr('Opérationnel', 'Operational') : tr('Indisponible', 'Down')}
                  </span>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
