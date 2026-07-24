// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Onboarding by ACTION (UX-5 / directive §2). Not a product tour — a lightweight
// getting-started checklist that walks a new tenant straight to the Aha moment
// (create the first risk → see its exposure), then to mapping assets and importing
// a framework. Steps auto-complete from real data, so progress is honest. Shown
// only while there's something left to do and the user hasn't dismissed it; a
// tenant that's already set up never sees it. The last step is the post-victory
// reward: personalize the space (theme + accent).

import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ShieldAlert, Database, ClipboardCheck, Check, X, Sparkles, ArrowRight, ChevronDown } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useRiskStore } from '../../hooks/useRiskStore';
import { useAssets } from '../assets/useAssets';
import { useComplianceOverview } from '../compliance/complianceOverview';
import { PersonalizeCard } from './PersonalizeCard';

const DISMISS_KEY = 'openrisk_onboarding_dismissed';

export function OnboardingChecklist() {
  const navigate = useNavigate();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const risks = useRiskStore((s) => s.risks);
  const total = useRiskStore((s) => s.total);
  const { assets } = useAssets();
  const { data: fws } = useComplianceOverview();

  const [dismissed, setDismissed] = useState(() => {
    try { return localStorage.getItem(DISMISS_KEY) === '1'; } catch { return false; }
  });
  const [showPersonalize, setShowPersonalize] = useState(false);

  const riskCount = total || risks.length;
  const steps = useMemo(() => [
    {
      key: 'risk',
      icon: ShieldAlert,
      title: tr('Créez votre premier risque', 'Create your first risk'),
      desc: tr('Le cœur d’OpenRisk : voyez aussitôt son exposition financière et un plan de traitement.', 'The heart of OpenRisk: instantly see its financial exposure and a treatment plan.'),
      done: riskCount > 0,
      cta: tr('Créer un risque', 'Create a risk'),
      action: () => window.dispatchEvent(new CustomEvent('openrisk:new-risk')),
      star: true,
    },
    {
      key: 'asset',
      icon: Database,
      title: tr('Cartographiez un actif', 'Map an asset'),
      desc: tr('Serveurs, bases, services — ce que vous protégez alimente le score de risque.', 'Servers, databases, services — what you protect feeds the risk score.'),
      done: assets.length > 0,
      cta: tr('Ajouter un actif', 'Add an asset'),
      action: () => navigate('/assets'),
    },
    {
      key: 'framework',
      icon: ClipboardCheck,
      title: tr('Importez un référentiel', 'Import a framework'),
      desc: tr('ISO 27001, NIST, RGPD… suivez votre conformité en un clic.', 'ISO 27001, NIST, GDPR… track your compliance in one click.'),
      done: (fws?.length ?? 0) > 0,
      cta: tr('Importer', 'Import'),
      action: () => navigate('/compliance'),
    },
  ], [riskCount, assets.length, fws, lang]); // eslint-disable-line react-hooks/exhaustive-deps

  const doneCount = steps.filter((s) => s.done).length;
  const allDone = doneCount === steps.length;

  if (dismissed || allDone) return null;

  const dismiss = () => {
    try { localStorage.setItem(DISMISS_KEY, '1'); } catch { /* ignore */ }
    setDismissed(true);
  };

  const pct = Math.round((doneCount / steps.length) * 100);

  return (
    <div
      className="or-card mb-4 overflow-hidden"
      style={{ border: '1px solid var(--accent)', background: 'linear-gradient(180deg, var(--accent-soft), transparent 60%)', animation: 'or-fadeup .4s ease' }}
    >
      <div className="p-5">
        <div className="flex items-start justify-between gap-3 mb-4">
          <div className="flex items-center gap-2.5">
            <div className="w-9 h-9 rounded-[11px] flex items-center justify-center text-white shrink-0" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-2))' }}>
              <Sparkles size={18} />
            </div>
            <div>
              <div className="text-[15px] font-bold text-ink">{tr('Bienvenue sur OpenRisk 👋', 'Welcome to OpenRisk 👋')}</div>
              <div className="text-[12.5px] text-ink-soft">{tr(`${doneCount} sur ${steps.length} étapes — en route vers votre première victoire.`, `${doneCount} of ${steps.length} steps — on your way to a first win.`)}</div>
            </div>
          </div>
          <button onClick={dismiss} className="w-8 h-8 rounded-lg flex items-center justify-center text-ink-muted hover:bg-hover transition-colors" title={tr('Masquer', 'Dismiss')} aria-label={tr('Masquer', 'Dismiss')}>
            <X size={16} />
          </button>
        </div>

        {/* progress */}
        <div className="h-[6px] rounded-full overflow-hidden mb-4" style={{ background: 'var(--bg-hover)' }}>
          <div className="h-full rounded-full" style={{ width: `${pct}%`, background: 'linear-gradient(90deg,var(--accent),var(--accent-2))', transition: 'width .8s cubic-bezier(.2,.8,.2,1)' }} />
        </div>

        {/* steps */}
        <div className="space-y-2">
          {steps.map((s) => {
            const Icon = s.icon;
            return (
              <div
                key={s.key}
                className="flex items-center gap-3 p-3 rounded-[12px]"
                style={{ background: s.done ? 'transparent' : 'var(--bg-elevated)', border: `1px solid ${s.done ? 'transparent' : 'var(--border)'}`, opacity: s.done ? 0.7 : 1 }}
              >
                <div
                  className="w-8 h-8 rounded-full flex items-center justify-center shrink-0"
                  style={{ background: s.done ? 'var(--low)' : 'var(--accent-soft)', color: s.done ? '#fff' : 'var(--accent)' }}
                >
                  {s.done ? <Check size={16} strokeWidth={3} /> : <Icon size={16} />}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="text-[13.5px] font-semibold text-ink flex items-center gap-1.5">
                    {s.star && <span title={tr('Étape clé', 'Key step')}>⭐</span>}
                    <span className={s.done ? 'line-through' : ''}>{s.title}</span>
                  </div>
                  {!s.done && <div className="text-[12px] text-ink-muted mt-0.5">{s.desc}</div>}
                </div>
                {!s.done && (
                  <button
                    onClick={s.action}
                    className="h-8 px-3 rounded-[9px] inline-flex items-center gap-1.5 text-[12.5px] font-semibold text-white shrink-0 transition-[filter] hover:brightness-110"
                    style={{ background: 'var(--accent)' }}
                  >
                    {s.cta} <ArrowRight size={14} />
                  </button>
                )}
              </div>
            );
          })}
        </div>

        {/* post-victory personalization — unlocked once the Aha (first risk) is reached */}
        {riskCount > 0 && (
          <div className="mt-3 pt-3 border-t border-border">
            <button
              onClick={() => setShowPersonalize((v) => !v)}
              className="w-full flex items-center justify-between gap-2 text-left"
            >
              <span className="text-[13px] font-semibold text-ink inline-flex items-center gap-2"><Sparkles size={15} className="text-accent" /> {tr('Personnalisez votre espace', 'Make it yours')}</span>
              <ChevronDown size={16} className="text-ink-muted transition-transform" style={{ transform: showPersonalize ? 'rotate(180deg)' : 'none' }} />
            </button>
            {showPersonalize && <div className="mt-3"><PersonalizeCard /></div>}
          </div>
        )}
      </div>
    </div>
  );
}
