// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The red line at the top of the redesigned dashboard (#901): the most severe
// critical or high incident still open, how long it has been running, and the
// way into its war room. Nothing renders when no such incident exists, and
// nothing renders for a member who may not read incidents.

import { useEffect, useState } from 'react';
import { Link } from 'react-router';
import { ArrowRight } from 'lucide-react';
import { useI18n } from '../../../hooks/useI18n';
import { usePermissions } from '../../../hooks/usePermissions';
import { useIncidents } from '../../incidents/useIncidents';
import { PRIORITY_CODE, incidentRef } from '../../incidents/incidentMeta';
import { bannerIncident } from './incidentPick';

export function IncidentBanner() {
  const { can } = usePermissions();
  if (!can('incidents:read')) return null;
  return <IncidentBannerInner />;
}

function IncidentBannerInner() {
  const { t } = useI18n();
  const { incidents } = useIncidents({ limit: 50 });
  const inc = bannerIncident(incidents ?? []);
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!inc) return;
    const timer = setInterval(() => setNow(Date.now()), 30_000);
    return () => clearInterval(timer);
  }, [inc]);

  if (!inc) return null;
  const mins = Math.max(0, Math.floor((now - new Date(inc.created_at).getTime()) / 60_000));
  const elapsed =
    mins < 60
      ? t('dashboard.incident.elapsedMin', { m: mins })
      : t('dashboard.incident.elapsed', {
          h: Math.floor(mins / 60),
          m: String(mins % 60).padStart(2, '0'),
        });

  return (
    <Link
      to={`/incidents/${inc.id}/war-room`}
      data-testid="dash-incident-banner"
      className="flex items-center gap-3 px-3.5 py-2.5 mb-4 rounded-[12px] text-[13px] hover:brightness-110"
      style={{ background: 'var(--danger-surface)', color: 'var(--danger-text)' }}
    >
      <span
        aria-hidden="true"
        className="w-2 h-2 rounded-full shrink-0 motion-safe:animate-[or-pulse_1.6s_ease-in-out_infinite]"
        style={{ background: 'var(--danger)' }}
      />
      <b className="mono font-semibold text-[12.5px] whitespace-nowrap">
        {incidentRef(inc.id)} · {PRIORITY_CODE[inc.severity]}
      </b>
      <span className="flex-1 min-w-0 truncate text-ink">
        {inc.title} — {t('dashboard.incident.running', { elapsed })}
      </span>
      <span className="font-semibold flex items-center gap-1.5 whitespace-nowrap">
        {t('dashboard.incident.join')} <ArrowRight size={14} aria-hidden="true" />
      </span>
    </Link>
  );
}
