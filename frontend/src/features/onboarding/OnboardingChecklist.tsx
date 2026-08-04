// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Guided onboarding (UX-01 "make them accomplish", UX-07 "always point to the next
// action", UX-17 "personalize after the Aha", UX-32 "celebrate milestones", Fogg
// B=MAT: an easy, motivated first action with a clear trigger). Runs on the
// dashboard after signup. Steps derived from real data auto-complete; the current
// step is emphasized so the user is never left facing an empty dashboard.

import { useEffect, useRef } from 'react';
import { useNavigate } from 'react-router';
import { Check, ShieldAlert, ClipboardCheck, UserPlus, Palette, Sun, Moon, ArrowRight } from 'lucide-react';
import { useUIStore, type Variant } from '../../store/uiStore';
import { useOnboarding } from './onboardingStore';
import { confetti } from '../../shared/celebrate';

interface Props {
  riskCount: number;
  frameworkCount: number;
  isAdmin: boolean;
  onCreateRisk: () => void;
}

interface Step {
  key: string;
  done: boolean;
  icon: typeof ShieldAlert;
  title: string;
  why: string;
  cta?: { label: string; run: () => void };
  personalize?: boolean;
}

export function OnboardingChecklist({ riskCount, frameworkCount, isAdmin, onCreateRisk }: Props) {
  const navigate = useNavigate();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const theme = useUIStore((s) => s.theme);
  const toggleTheme = useUIStore((s) => s.toggleTheme);
  const variant = useUIStore((s) => s.variant);
  const setVariant = useUIStore((s) => s.setVariant);
  const { invited, personalized, dismissed, markPersonalized, dismiss } = useOnboarding();

  const steps: Step[] = [
    {
      key: 'risk',
      done: riskCount > 0,
      icon: ShieldAlert,
      title: tr('Créez votre premier risque', 'Create your first risk'),
      why: tr('L’action qui prouve la valeur : votre score et votre exposition se calculent aussitôt.', 'The action that proves the value: your score and exposure compute instantly.'),
      cta: { label: tr('Créer un risque', 'Create a risk'), run: onCreateRisk },
    },
    {
      key: 'framework',
      done: frameworkCount > 0,
      icon: ClipboardCheck,
      title: tr('Importez un référentiel', 'Import a framework'),
      why: tr('ISO 27001, COBAC, BCEAO… en un clic — vos contrôles et écarts apparaissent.', 'ISO 27001, COBAC, BCEAO… in one click — your controls and gaps appear.'),
      cta: { label: tr('Choisir un référentiel', 'Choose a framework'), run: () => navigate('/compliance') },
    },
    ...(isAdmin
      ? [
          {
            key: 'invite',
            done: invited,
            icon: UserPlus,
            title: tr('Invitez un collègue', 'Invite a teammate'),
            why: tr('La GRC est un sport d’équipe — assignez des risques et des contrôles.', 'GRC is a team sport — assign risks and controls to people.'),
            cta: { label: tr('Inviter', 'Invite'), run: () => navigate('/settings?tab=members') },
          } as Step,
        ]
      : []),
    {
      key: 'personalize',
      done: personalized,
      icon: Palette,
      title: tr('Personnalisez votre espace', 'Make it yours'),
      why: tr('Thème et couleur d’accent — appropriez-vous OpenRisk.', 'Theme and accent color — make OpenRisk feel like home.'),
      personalize: true,
    },
  ];

  const total = steps.length;
  const doneCount = steps.filter((s) => s.done).length;
  const complete = doneCount === total;

  // Celebrate full completion once.
  const celebratedRef = useRef(false);
  useEffect(() => {
    if (complete && !celebratedRef.current && !dismissed) {
      celebratedRef.current = true;
      confetti(70);
    }
  }, [complete, dismissed]);

  if (dismissed) return null;

  const currentKey = steps.find((s) => !s.done)?.key;

  return (
    <div className="rounded-[16px] p-5 mb-4" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)', animation: 'or-fadeup .4s ease' }}>
      <div className="flex items-start justify-between gap-4 mb-3.5">
        <div>
          <div className="text-[15.5px] font-bold text-ink">
            {complete ? tr('🎉 Vous êtes prêt !', '🎉 You’re all set!') : tr('Prise en main', 'Get started')}
          </div>
          <div className="text-[13px] text-ink-soft mt-0.5">
            {complete
              ? tr('Votre espace est configuré. Bon travail.', 'Your workspace is set up. Nice work.')
              : tr('Quatre étapes pour tirer toute la valeur d’OpenRisk.', 'Four steps to get the full value of OpenRisk.')}
          </div>
        </div>
        <div className="flex items-center gap-3 shrink-0">
          <span className="mono text-[12.5px] font-semibold text-ink-soft">{doneCount}/{total}</span>
          <button onClick={dismiss} className="w-7 h-7 rounded-[8px] flex items-center justify-center text-ink-muted hover:bg-hover transition-colors" aria-label={tr('Masquer la prise en main', 'Dismiss onboarding')}>✕</button>
        </div>
      </div>

      {/* progress bar */}
      <div className="h-1.5 rounded-full overflow-hidden mb-4" style={{ background: 'var(--bg-hover)' }}>
        <div className="h-full rounded-full" style={{ width: `${(doneCount / total) * 100}%`, background: 'linear-gradient(90deg,var(--accent),var(--accent-2))', transition: 'width .5s var(--ease-out, ease)' }} />
      </div>

      <div className="flex flex-col gap-2">
        {steps.map((s) => {
          const isCurrent = s.key === currentKey;
          const Icon = s.icon;
          return (
            <div
              key={s.key}
              className="flex items-start gap-3 rounded-[12px] p-3 transition-colors"
              style={{
                background: isCurrent ? 'var(--accent-soft)' : 'transparent',
                border: isCurrent ? '1px solid var(--accent-line)' : '1px solid transparent',
                opacity: s.done ? 0.66 : 1,
              }}
            >
              <div
                className="w-8 h-8 rounded-[10px] flex items-center justify-center shrink-0"
                style={{ background: s.done ? 'color-mix(in srgb,var(--low) 18%,transparent)' : 'var(--bg-hover)', color: s.done ? 'var(--low)' : isCurrent ? 'var(--accent)' : 'var(--text-muted)' }}
              >
                {s.done ? <Check size={17} strokeWidth={2.5} /> : <Icon size={17} strokeWidth={1.9} />}
              </div>

              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="text-[13.5px] font-semibold text-ink" style={{ textDecoration: s.done ? 'line-through' : 'none' }}>{s.title}</span>
                  {isCurrent && !s.done && (
                    <span className="text-[10px] font-bold uppercase tracking-wide px-2 py-0.5 rounded-full" style={{ background: 'var(--accent)', color: '#fff' }}>
                      {tr('À faire maintenant', 'Do this next')}
                    </span>
                  )}
                </div>
                {!s.done && <div className="text-[12.5px] text-ink-soft mt-0.5 leading-snug">{s.why}</div>}

                {/* Personalize step: inline ghost edit (no navigation) — UX-06/17 */}
                {s.personalize && !s.done && (
                  <div className="flex items-center gap-2.5 mt-2.5 flex-wrap">
                    <button
                      onClick={() => { toggleTheme(); markPersonalized(); }}
                      className="h-9 px-3 rounded-[9px] text-[12.5px] font-semibold text-ink inline-flex items-center gap-1.5"
                      style={{ background: 'var(--bg-hover)', border: '1px solid var(--border-strong)' }}
                    >
                      {theme === 'dark' ? <Sun size={15} /> : <Moon size={15} />}
                      {theme === 'dark' ? tr('Thème clair', 'Light theme') : tr('Thème sombre', 'Dark theme')}
                    </button>
                    {(['azure', 'iris'] as Variant[]).map((v) => (
                      <button
                        key={v}
                        onClick={() => { setVariant(v); markPersonalized(); }}
                        aria-label={v}
                        title={v === 'azure' ? tr('Accent azur', 'Azure accent') : tr('Accent iris', 'Iris accent')}
                        className="w-9 h-9 rounded-[9px] flex items-center justify-center"
                        style={{ border: variant === v ? '2px solid var(--accent)' : '1px solid var(--border-strong)', background: 'var(--bg-hover)' }}
                      >
                        <span className="w-4 h-4 rounded-full" style={{ background: v === 'azure' ? 'linear-gradient(135deg,#0a84ff,#64d2ff)' : 'linear-gradient(135deg,#7c6cff,#b692ff)' }} />
                      </button>
                    ))}
                  </div>
                )}
              </div>

              {/* CTA for non-personalize steps */}
              {s.cta && !s.done && (
                <button
                  onClick={s.cta.run}
                  className="h-9 px-3.5 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 shrink-0 transition-[filter] hover:brightness-110"
                  style={
                    isCurrent
                      ? { background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))', color: '#fff', boxShadow: '0 3px 12px var(--accent-glow)' }
                      : { background: 'var(--bg-hover)', color: 'var(--text-primary)', border: '1px solid var(--border-strong)' }
                  }
                >
                  {s.cta.label} <ArrowRight size={14} />
                </button>
              )}
            </div>
          );
        })}
      </div>

      {complete && (
        <button
          onClick={dismiss}
          className="mt-3.5 w-full h-[40px] rounded-[10px] text-[13px] font-semibold text-white"
          style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}
        >
          {tr('Terminer la prise en main', 'Finish onboarding')}
        </button>
      )}
    </div>
  );
}
