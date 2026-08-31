// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { AlertOctagon, RefreshCw, Home } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { SystemMessage } from './SystemMessage';

interface ServerErrorProps {
  /** Correlation id shown to the user so support can find the trace. */
  errorId?: string;
  /** Retry handler (reset the boundary); falls back to a full reload. */
  onRetry?: () => void;
}

export function ServerError({ errorId, onRetry }: ServerErrorProps) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  return (
    <SystemMessage
      icon={AlertOctagon}
      code="500"
      tone="var(--critical)"
      title={tr('Une erreur est survenue', 'Something went wrong')}
      message={tr(
        "Un problème inattendu s'est produit de notre côté. L'incident a été enregistré. Réessayez, ou revenez au tableau de bord.",
        'An unexpected problem occurred on our side. The incident was logged. Try again, or return to the dashboard.',
      )}
      actions={
        <>
          <button
            onClick={() => (onRetry ? onRetry() : window.location.reload())}
            className="h-10 px-5 rounded-[11px] inline-flex items-center gap-2 text-[13.5px] font-semibold text-text-primary"
            style={{ background: 'var(--accent-solid)', color: 'var(--text-on-solid)' }}
          >
            <RefreshCw size={16} /> {tr('Réessayer', 'Try again')}
          </button>
          <a
            href="/"
            className="h-10 px-4 rounded-[11px] inline-flex items-center gap-2 text-[13.5px] font-semibold"
            style={{ border: '1px solid var(--border)', color: 'var(--ink)' }}
          >
            <Home size={16} /> {tr('Tableau de bord', 'Dashboard')}
          </a>
        </>
      }
    >
      {errorId && (
        <div className="text-[11.5px] text-ink-muted mono">
          {tr('Référence', 'Reference')} : {errorId}
        </div>
      )}
    </SystemMessage>
  );
}
