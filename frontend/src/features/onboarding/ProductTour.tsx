// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The product tour (spec §8): three coach marks, non-blocking, replayable.
//
// "Non-blocking" is a design constraint with teeth, and it is why this is not a
// modal with a dimmed backdrop:
//   • no overlay intercepts clicks — the app stays fully usable underneath;
//   • Escape, "Skip" and clicking anywhere else all dismiss it;
//   • it points AT things (anchored to a real element) instead of describing them.
//
// It shows once per user and is replayable from the Help menu, which dispatches
// `openrisk:tour`. The "seen" flag is the one piece of purely cosmetic client
// state left in this feature — deliberately: unlike activation, nothing depends
// on it being right, and replaying a tour on a second device is harmless.

import { useCallback, useEffect, useLayoutEffect, useState } from 'react';
import { X, ArrowRight } from 'lucide-react';

import { useUIStore } from '../../store/uiStore';

const SEEN_KEY = 'openrisk_tour_seen_v1';

interface CoachMark {
  /** CSS selector of the element to point at. */
  anchor: string;
  fr: { title: string; body: string };
  en: { title: string; body: string };
  placement: 'right' | 'bottom';
}

const MARKS: CoachMark[] = [
  {
    anchor: '[data-tour="new-risk"]',
    placement: 'right',
    fr: {
      title: 'Commencez ici',
      body: "Ajoutez un risque : son score et son exposition financière sont calculés immédiatement. C'est le cœur d'OpenRisk.",
    },
    en: {
      title: 'Start here',
      body: 'Add a risk: its score and financial exposure are computed immediately. This is the heart of OpenRisk.',
    },
  },
  {
    anchor: '[data-tour="search"]',
    placement: 'bottom',
    fr: {
      title: 'Tout retrouver',
      body: 'Risques, actifs, vulnérabilités, contrôles, CVE — une seule recherche. Raccourci : ⌘K (ou « / »).',
    },
    en: {
      title: 'Find anything',
      body: 'Risks, assets, vulnerabilities, controls, CVEs — one search. Shortcut: ⌘K (or “/”).',
    },
  },
  {
    anchor: '[data-tour="nav-compliance"]',
    placement: 'right',
    fr: {
      title: 'Prouver la conformité',
      body: 'Importez un référentiel (ISO 27001, COBAC, BCEAO…) et vos écarts apparaissent, prêts à être remédiés.',
    },
    en: {
      title: 'Prove compliance',
      body: 'Import a framework (ISO 27001, COBAC, BCEAO…) and your gaps appear, ready to be remediated.',
    },
  },
];

function hasSeen(): boolean {
  try {
    return localStorage.getItem(SEEN_KEY) === '1';
  } catch {
    return true; // storage blocked → do not nag
  }
}

function markSeen() {
  try {
    localStorage.setItem(SEEN_KEY, '1');
  } catch {
    /* ignore quota */
  }
}

export function ProductTour() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const [step, setStep] = useState<number | null>(null);
  const [rect, setRect] = useState<DOMRect | null>(null);

  const close = useCallback(() => {
    setStep(null);
    markSeen();
  }, []);

  // Replay from the Help menu.
  useEffect(() => {
    const replay = () => setStep(0);
    window.addEventListener('openrisk:tour', replay);
    return () => window.removeEventListener('openrisk:tour', replay);
  }, []);

  // First run: only once the anchors exist, and never for someone who has seen
  // it. Deferred a beat so the shell has painted.
  useEffect(() => {
    if (hasSeen()) return;
    const timer = window.setTimeout(() => {
      if (document.querySelector(MARKS[0].anchor)) setStep(0);
    }, 900);
    return () => window.clearTimeout(timer);
  }, []);

  const mark = step === null ? null : MARKS[step];

  // Position against the live anchor, and keep up with scroll/resize.
  useLayoutEffect(() => {
    if (!mark) {
      setRect(null);
      return;
    }
    const measure = () => {
      const el = document.querySelector(mark.anchor);
      setRect(el ? el.getBoundingClientRect() : null);
    };
    measure();
    window.addEventListener('resize', measure);
    window.addEventListener('scroll', measure, true);
    return () => {
      window.removeEventListener('resize', measure);
      window.removeEventListener('scroll', measure, true);
    };
  }, [mark]);

  useEffect(() => {
    if (step === null) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') close();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [step, close]);

  if (!mark) return null;

  // A missing anchor (a route without that element, a permission-hidden nav
  // item) skips forward rather than pointing at nothing.
  if (!rect) {
    if (step !== null && step < MARKS.length - 1) {
      queueMicrotask(() => setStep(step + 1));
    } else {
      queueMicrotask(close);
    }
    return null;
  }

  const copy = lang === 'fr' ? mark.fr : mark.en;
  const isLast = step === MARKS.length - 1;

  const style: React.CSSProperties =
    mark.placement === 'right'
      ? { top: Math.max(12, rect.top - 8), left: rect.right + 12 }
      : { top: rect.bottom + 12, left: Math.max(12, rect.left - 60) };

  return (
    // No backdrop: the app must stay clickable. `pointer-events-none` on the
    // layer, re-enabled on the card only.
    <div className="fixed inset-0 z-70 pointer-events-none" data-testid="product-tour">
      {/* A ring around the anchor, so the words point at something. */}
      <div
        className="absolute rounded-[10px]"
        style={{
          top: rect.top - 4,
          left: rect.left - 4,
          width: rect.width + 8,
          height: rect.height + 8,
          border: '2px solid var(--accent)',
          transition: 'all .2s ease',
        }}
      />

      <div
        role="dialog"
        aria-label={copy.title}
        className="absolute w-[290px] max-w-[86vw] rounded-[14px] p-4 pointer-events-auto"
        style={{
          ...style,
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border-strong)',
          boxShadow: '0 16px 40px rgba(0,0,0,.28)',
          animation: 'or-scalein .18s ease',
        }}
      >
        <div className="flex items-start justify-between gap-2 mb-1.5">
          <span className="text-[13.5px] font-bold text-ink">{copy.title}</span>
          <button
            type="button"
            onClick={close}
            className="text-ink-muted hover:text-ink transition-colors"
            aria-label={tr('Fermer', 'Close')}
          >
            <X size={15} />
          </button>
        </div>

        <p className="text-[12.5px] text-ink-soft leading-snug mb-3">{copy.body}</p>

        <div className="flex items-center justify-between gap-3">
          <span className="mono text-[11px] text-ink-muted">
            {(step ?? 0) + 1}/{MARKS.length}
          </span>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={close}
              className="text-[12px] font-semibold text-ink-soft"
            >
              {tr('Passer', 'Skip')}
            </button>
            <button
              type="button"
              data-testid="tour-next"
              onClick={() => (isLast ? close() : setStep((s) => (s ?? 0) + 1))}
              className="h-8 px-3 rounded-[8px] text-[12px] font-semibold inline-flex items-center gap-1.5"
              style={{
                background: 'var(--accent-solid)',
                color: 'var(--text-on-solid)',
              }}
            >
              {isLast ? tr('Terminer', 'Done') : tr('Suivant', 'Next')}
              {!isLast && <ArrowRight size={13} />}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
