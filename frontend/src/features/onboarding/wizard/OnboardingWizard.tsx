// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The signup wizard shell (spec §4): five routes, one visible progress bar, and
// the same promise on every step — your answers are saved, you can come back,
// and you can go back.
//
// The state lives on the server (GET/PUT /onboarding/*), so closing the tab mid
// wizard and returning on another device resumes at the same step with the same
// answers. Nothing here is persisted client-side.

import { Outlet, useNavigate, useLocation } from 'react-router';
import { Check, Loader2 } from 'lucide-react';

import { useUIStore } from '../../../store/uiStore';
import { OpenRiskLogo } from '../../../shared/Logo';
import { useOnboardingState } from '../useActivation';
import type { OnboardingStepKey } from '../../../services/activationService';

/** Step order + copy. The order mirrors domain.OnboardingStepOrder. */
export const WIZARD_STEPS: { key: OnboardingStepKey; fr: string; en: string }[] = [
  { key: 'organization', fr: 'Organisation', en: 'Organization' },
  { key: 'profile', fr: 'Profil', en: 'Profile' },
  { key: 'goal', fr: 'Objectif', en: 'Goal' },
  { key: 'framework', fr: 'Référentiel', en: 'Framework' },
  { key: 'team', fr: 'Équipe', en: 'Team' },
];

export function stepPath(step: OnboardingStepKey): string {
  return `/onboarding/${step}`;
}

export function OnboardingWizard() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const { data: state, isLoading } = useOnboardingState();

  const activeKey = (pathname.split('/')[2] ?? 'organization') as OnboardingStepKey;
  const activeIndex = Math.max(
    0,
    WIZARD_STEPS.findIndex((s) => s.key === activeKey),
  );
  // Progress reflects steps FINISHED, not the one being filled in — a bar that
  // reads 20% while you are still on step 1 is a bar that lies.
  const percent = Math.round((activeIndex / WIZARD_STEPS.length) * 100);

  return (
    <div className="min-h-screen flex flex-col" style={{ background: 'var(--bg-primary)' }}>
      <header
        className="px-5 sm:px-8 py-4 flex items-center justify-between gap-4"
        style={{ borderBottom: '1px solid var(--border-subtle)' }}
      >
        <div className="flex items-center gap-2.5">
          <OpenRiskLogo size={26} />
          <span className="text-[15px] font-bold text-ink">OpenRisk</span>
        </div>
        <div className="text-[12.5px] text-ink-soft">
          {tr('Étape', 'Step')} {activeIndex + 1}/{WIZARD_STEPS.length}
        </div>
      </header>

      {/* Progress: the bar plus a labelled step rail, so people know how much is
          left AND what is coming. */}
      <div className="px-5 sm:px-8 pt-5">
        <div
          className="h-1.5 rounded-full overflow-hidden"
          style={{ background: 'var(--bg-hover)' }}
          role="progressbar"
          aria-valuenow={percent}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-label={tr('Progression de la configuration', 'Setup progress')}
        >
          <div
            className="h-full rounded-full"
            style={{
              width: `${percent}%`,
              background: 'var(--accent)',
              transition: 'width .45s var(--ease-out, ease)',
            }}
          />
        </div>

        <ol className="flex flex-wrap items-center gap-x-4 gap-y-1.5 mt-3 list-none p-0 m-0">
          {WIZARD_STEPS.map((s, i) => {
            const done = i < activeIndex;
            const active = i === activeIndex;
            return (
              <li key={s.key}>
                <button
                  type="button"
                  // Back-navigation is allowed and encouraged; forward is not,
                  // because a step ahead has no answers to show yet.
                  disabled={i > activeIndex}
                  onClick={() => navigate(stepPath(s.key))}
                  className="inline-flex items-center gap-1.5 text-[12.5px] font-semibold disabled:cursor-default"
                  style={{
                    color: active
                      ? 'var(--accent)'
                      : done
                        ? 'var(--text-secondary)'
                        : 'var(--text-muted)',
                  }}
                >
                  <span
                    className="w-[18px] h-[18px] rounded-full inline-flex items-center justify-center text-[10px] font-bold"
                    style={{
                      background: done
                        ? 'color-mix(in srgb,var(--low) 20%,transparent)'
                        : active
                          ? 'var(--accent)'
                          : 'var(--bg-hover)',
                      color: done ? 'var(--low)' : active ? '#fff' : 'var(--text-muted)',
                    }}
                  >
                    {done ? <Check size={11} strokeWidth={3} /> : i + 1}
                  </span>
                  {tr(s.fr, s.en)}
                </button>
              </li>
            );
          })}
        </ol>
      </div>

      <main className="flex-1 px-5 sm:px-8 py-6 flex justify-center">
        <div className="w-full max-w-[640px]">
          {isLoading && !state ? (
            <div className="flex items-center gap-2 text-[13px] text-ink-soft py-10">
              <Loader2 size={15} className="animate-spin" />
              {tr('Chargement…', 'Loading…')}
            </div>
          ) : (
            <Outlet />
          )}
        </div>
      </main>
    </div>
  );
}
