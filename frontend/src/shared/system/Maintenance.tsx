// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { Wrench, RefreshCw } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { SystemMessage } from './SystemMessage';

export function Maintenance() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  return (
    <SystemMessage
      icon={Wrench}
      tone="var(--medium)"
      title={tr('Maintenance en cours', 'Under maintenance')}
      message={tr(
        "OpenRisk est momentanément indisponible pour une opération de maintenance planifiée. Merci de réessayer dans quelques minutes.",
        'OpenRisk is briefly unavailable for planned maintenance. Please try again in a few minutes.',
      )}
      actions={
        <button
          onClick={() => window.location.reload()}
          className="h-10 px-5 rounded-[11px] inline-flex items-center gap-2 text-[13.5px] font-semibold text-text-primary"
          style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}
        >
          <RefreshCw size={16} /> {tr('Réessayer', 'Retry')}
        </button>
      }
    />
  );
}
