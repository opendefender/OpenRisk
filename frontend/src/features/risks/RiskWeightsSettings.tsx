// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/
//
// RiskWeightsSettings — admin screen to tune the eight factor weights of the
// multifactor smart-risk model (spec §8). Weights are relative; the panel shows
// the live-normalised effective share of each factor. Read-only for non-admins.

import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { SlidersHorizontal, RotateCcw, Save, ArrowLeft } from 'lucide-react';
import { PageFrame, PageHeader, Card, Btn, SkeletonRows, ErrorState } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useRiskWeights, useUpdateRiskWeights } from './useSmartScore';
import { FACTOR_KEYS, type FactorKey, type FactorWeightsInput } from './smartScoreService';

// Bilingual labels + one-line descriptions for each factor.
const FACTOR_META: Record<FactorKey, { label: [string, string]; hint: [string, string] }> = {
  business_criticality: {
    label: ['Criticité métier', 'Business criticality'],
    hint: ['Importance de l’actif pour les opérations', 'Importance of the asset to operations'],
  },
  internet_exposure: {
    label: ['Exposition Internet', 'Internet exposure'],
    hint: ['Accessible publiquement ou isolé', 'Publicly reachable vs. isolated'],
  },
  vulnerabilities: {
    label: ['Vulnérabilités', 'Vulnerabilities'],
    hint: ['Nombre et gravité (CVSS) des failles', 'Number and severity (CVSS) of findings'],
  },
  control_maturity: {
    label: ['Maturité des contrôles', 'Control maturity'],
    hint: ['Efficacité des mesures en place', 'Effectiveness of controls in place'],
  },
  incident_history: {
    label: ['Historique des incidents', 'Incident history'],
    hint: ['Fréquence des compromissions passées', 'Frequency of past compromises'],
  },
  exploitability: {
    label: ['Facilité d’exploitation', 'Exploitability'],
    hint: ['Exploit public / complexité d’attaque', 'Public exploit / attack complexity'],
  },
  financial_value: {
    label: ['Valeur financière', 'Financial value'],
    hint: ['Coût estimé en cas de perte C/I/D', 'Estimated cost of a C/I/A loss'],
  },
  threat_intel: {
    label: ['Menaces actives (CTI)', 'Active threats (CTI)'],
    hint: ['Corrélation Threat Intelligence temps réel', 'Real-time threat-intel correlation'],
  },
};

// The engine defaults (mirror pkg/scoring.defaultWeights), used by "Reset".
const DEFAULT_WEIGHTS: FactorWeightsInput = {
  business_criticality: 0.15,
  internet_exposure: 0.1,
  vulnerabilities: 0.2,
  control_maturity: 0.1,
  incident_history: 0.1,
  exploitability: 0.15,
  financial_value: 0.1,
  threat_intel: 0.1,
};

export function RiskWeightsSettings() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const navigate = useNavigate();
  const isAdmin = useAuthStore((s) => s.hasRole('admin'));
  const { data, isLoading, isError, refetch } = useRiskWeights();
  const update = useUpdateRiskWeights();

  const [w, setW] = useState<FactorWeightsInput | null>(null);
  useEffect(() => {
    if (data) {
      setW({
        business_criticality: data.business_criticality,
        internet_exposure: data.internet_exposure,
        vulnerabilities: data.vulnerabilities,
        control_maturity: data.control_maturity,
        incident_history: data.incident_history,
        exploitability: data.exploitability,
        financial_value: data.financial_value,
        threat_intel: data.threat_intel,
      });
    }
  }, [data]);

  const total = useMemo(() => (w ? FACTOR_KEYS.reduce((s, k) => s + (w[k] || 0), 0) : 0), [w]);

  if (isLoading || !w) {
    return (
      <PageFrame>
        <PageHeader title={tr('Pondération des risques', 'Risk weighting')} />
        <Card className="p-6"><SkeletonRows rows={8} /></Card>
      </PageFrame>
    );
  }
  if (isError) {
    return (
      <PageFrame>
        <PageHeader title={tr('Pondération des risques', 'Risk weighting')} />
        <ErrorState title={tr('Chargement impossible', 'Failed to load')} onRetry={() => refetch()} />
      </PageFrame>
    );
  }

  const setFactor = (k: FactorKey, val: number) => setW((prev) => (prev ? { ...prev, [k]: val } : prev));
  const dirty = FACTOR_KEYS.some((k) => Math.abs((w[k] || 0) - (data?.[k] ?? 0)) > 1e-6);

  const save = async () => {
    if (total <= 0) {
      toast.error(tr('Au moins un facteur doit être > 0.', 'At least one factor must be > 0.'));
      return;
    }
    try {
      await update.mutateAsync(w);
      toast.success(tr('Pondérations enregistrées.', 'Weights saved.'));
    } catch {
      toast.error(tr('Échec de l’enregistrement.', 'Failed to save.'));
    }
  };

  return (
    <PageFrame>
      <PageHeader
        title={tr('Pondération des risques', 'Risk weighting')}
        badge={<span className="text-[12px] text-ink-soft">{tr('Calcul de risque intelligent', 'Smart risk calculation')}</span>}
        actions={
          <div className="flex gap-2">
            <Btn label={tr('Registre des risques', 'Risk register')} icon={ArrowLeft} onClick={() => navigate('/risks')} />
            {isAdmin && <Btn label={tr('Défauts', 'Defaults')} icon={RotateCcw} onClick={() => setW({ ...DEFAULT_WEIGHTS })} />}
            {isAdmin && (
              <Btn
                label={update.isPending ? tr('Enregistrement…', 'Saving…') : tr('Enregistrer', 'Save')}
                icon={Save}
                primary
                className={!dirty || update.isPending ? 'opacity-50 pointer-events-none' : ''}
                onClick={save}
              />
            )}
          </div>
        }
      />

      <Card className="p-5 mb-4">
        <div className="flex items-start gap-3">
          <div className="w-9 h-9 rounded-[10px] flex items-center justify-center shrink-0" style={{ background: 'var(--bg-hover)' }}>
            <SlidersHorizontal size={18} className="text-ink-soft" />
          </div>
          <p className="text-[12.5px] text-ink-soft leading-snug">
            {tr(
              'Ajustez l’importance relative de chaque facteur du score intelligent. Les valeurs sont relatives : le moteur les normalise, la « part effective » ci-dessous montre le poids réel appliqué. Le score classique (Probabilité × Impact × Criticité) reste inchangé.',
              'Adjust the relative importance of each smart-score factor. Values are relative: the engine normalises them, the “effective share” below shows the real applied weight. The classic score (Probability × Impact × Criticality) is unchanged.',
            )}
          </p>
        </div>
      </Card>

      <Card className="p-5">
        {!isAdmin && (
          <div className="mb-4 text-[12.5px] text-ink-muted">
            {tr('Lecture seule — rôle administrateur requis pour modifier.', 'Read-only — the administrator role is required to edit.')}
          </div>
        )}
        <div className="space-y-5">
          {FACTOR_KEYS.map((k) => {
            const meta = FACTOR_META[k];
            const share = total > 0 ? (w[k] || 0) / total : 0;
            return (
              <div key={k}>
                <div className="flex items-baseline justify-between mb-1.5">
                  <div>
                    <span className="text-[13.5px] font-semibold text-ink">{meta.label[lang === 'fr' ? 0 : 1]}</span>
                    <span className="text-[11.5px] text-ink-muted ml-2">{meta.hint[lang === 'fr' ? 0 : 1]}</span>
                  </div>
                  <span className="mono text-[12.5px] text-ink-soft shrink-0">
                    {(w[k] || 0).toFixed(2)}
                    <span className="text-ink-muted"> · {Math.round(share * 100)}%</span>
                  </span>
                </div>
                <div className="flex items-center gap-3">
                  <input
                    type="range"
                    min={0}
                    max={1}
                    step={0.01}
                    value={w[k] || 0}
                    disabled={!isAdmin}
                    onChange={(e) => setFactor(k, Number(e.target.value))}
                    className="flex-1 accent-[var(--accent)] disabled:opacity-60"
                    style={{ accentColor: 'var(--accent)' }}
                  />
                  {/* Effective-share bar */}
                  <div className="w-24 h-1.5 rounded-full overflow-hidden shrink-0" style={{ background: 'var(--bg-hover)' }}>
                    <div className="h-full rounded-full" style={{ width: `${share * 100}%`, background: 'var(--accent)' }} />
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </Card>
    </PageFrame>
  );
}

export default RiskWeightsSettings;
