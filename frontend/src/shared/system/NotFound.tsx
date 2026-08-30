// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { useNavigate } from 'react-router';
import { Compass, ArrowLeft, Home } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { SystemMessage } from './SystemMessage';

export function NotFound() {
  const navigate = useNavigate();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  return (
    <SystemMessage
      icon={Compass}
      code="404"
      title={tr('Page introuvable', 'Page not found')}
      message={tr(
        "Cette page n'existe pas ou a été déplacée. Vérifiez l'adresse, ou revenez à un point connu.",
        "This page doesn't exist or has moved. Check the address, or head back to a known spot.",
      )}
      actions={
        <>
          <button
            onClick={() => navigate(-1)}
            className="h-10 px-4 rounded-[11px] inline-flex items-center gap-2 text-[13.5px] font-semibold"
            style={{ border: '1px solid var(--border)', color: 'var(--ink)' }}
          >
            <ArrowLeft size={16} /> {tr('Retour', 'Back')}
          </button>
          <button
            onClick={() => navigate('/')}
            className="h-10 px-5 rounded-[11px] inline-flex items-center gap-2 text-[13.5px] font-semibold text-text-primary"
            style={{ background: 'var(--accent-solid)', color: 'var(--text-on-solid)' }}
          >
            <Home size={16} /> {tr('Tableau de bord', 'Dashboard')}
          </button>
        </>
      }
    />
  );
}
