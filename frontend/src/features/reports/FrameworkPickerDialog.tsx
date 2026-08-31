// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Framework picker for report generation.
//
// A compliance report is per-framework, so asking for one from the Reports
// screen needs a choice. Making that choice HERE is what closes the loop: the
// Compliance tile used to answer the question by navigating to /compliance,
// whose own generate button navigated back, so the user could shuttle between
// the two screens indefinitely without producing a document.

import { useUIStore } from '../../store/uiStore';
import { useEscapeToClose } from '../../shared/useBackTo';
import { EmptyState } from '../../shared/EmptyState';
import { Btn } from '../../shared/ui';
import { ClipboardCheck, X } from 'lucide-react';

interface Framework {
  id: string;
  name: string;
  version?: string;
}

interface Props {
  frameworks: Framework[];
  busy?: boolean;
  onClose: () => void;
  onPick: (frameworkId: string) => void;
}

export function FrameworkPickerDialog({ frameworks, busy, onClose, onPick }: Props) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  useEscapeToClose(true, onClose);

  return (
    <div
      className="fixed inset-0 z-90 flex items-center justify-center p-4"
      style={{ background: 'var(--surface-overlay)', backdropFilter: 'blur(6px)', animation: 'or-fadein .16s ease' }}
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      data-testid="framework-picker"
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="glass-strong rounded-[18px] shadow-card-lg flex flex-col"
        style={{ width: 'min(92vw,460px)', maxHeight: '80vh', animation: 'or-scalein .18s ease' }}
      >
        <div className="shrink-0 flex items-center justify-between px-5 py-4" style={{ borderBottom: '1px solid var(--border)' }}>
          <span className="text-[15px] font-semibold text-ink">
            {tr('Quel référentiel ?', 'Which framework?')}
          </span>
          <button onClick={onClose} aria-label={tr('Fermer', 'Close')} className="p-1 rounded-md hover:bg-hover">
            <X size={18} className="text-ink-muted" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-2">
          {frameworks.length === 0 ? (
            <EmptyState
              variant="first-use"
              icon={ClipboardCheck}
              title={tr('Aucun référentiel', 'No frameworks yet')}
              description={tr(
                'Un rapport de conformité porte sur un référentiel. Importez-en un pour pouvoir en générer.',
                'A compliance report covers a framework. Import one to be able to generate a report.',
              )}
              className="py-8"
            />
          ) : (
            frameworks.map((f) => (
              <button
                key={f.id}
                disabled={busy}
                onClick={() => onPick(f.id)}
                className="w-full text-left flex items-center gap-3 px-3 py-3 rounded-[10px] hover:bg-hover transition-colors disabled:opacity-60"
              >
                <div
                  className="w-9 h-9 rounded-[10px] flex items-center justify-center shrink-0"
                  style={{ background: 'var(--accent-soft)', color: 'var(--accent-500)' }}
                >
                  <ClipboardCheck size={17} />
                </div>
                <div className="min-w-0">
                  <div className="text-[13.5px] font-medium text-ink truncate">{f.name}</div>
                  {f.version && <div className="text-[11.5px] text-ink-muted">{f.version}</div>}
                </div>
              </button>
            ))
          )}
        </div>

        <div className="shrink-0 px-5 py-3.5 flex justify-end" style={{ borderTop: '1px solid var(--border)' }}>
          <Btn label={tr('Annuler', 'Cancel')} onClick={onClose} />
        </div>
      </div>
    </div>
  );
}
