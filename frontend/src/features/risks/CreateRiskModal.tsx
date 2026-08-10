// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useEffect, useMemo } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { X, Zap, Database, ShieldAlert, Sparkles } from 'lucide-react';
import { toast } from 'sonner';
import { useAssetStore } from '../../hooks/useAssetStore';
import { riskService, type Risk } from '../../services/riskService';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { useI18n } from '../../hooks/useI18n';
import { useEscapeToClose } from '../../shared/useBackTo';
import { FieldHelp } from '../../shared/FieldHelp';
import { useUIStore } from '../../store/uiStore';
import {
  useInvalidateActivation,
  useOnboardingSuggestions,
} from '../onboarding/useActivation';
import type { RiskSuggestion } from '../../services/activationService';

const createRiskSchema = z.object({
  title: z.string().min(5, 'Le nom doit comporter au moins 5 caractères').max(100),
  description: z.string().min(10, 'La description doit comporter au moins 10 caractères'),
  impact: z.number().min(1).max(10),
  probability: z.number().min(0).max(1),
  assetCriticality: z.number().min(0.1).max(3),
  framework: z.string().optional(),
  tags: z.array(z.string()).optional(),
  asset_ids: z.array(z.string()).optional(),
});

type CreateRiskForm = z.infer<typeof createRiskSchema>;

interface CreateRiskModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreated?: (risk: Risk) => void;
}

const frameworkOptions = [
  { value: 'ISO27001', label: 'ISO 27001' },
  { value: 'CIS', label: 'CIS' },
  { value: 'NIST', label: 'NIST' },
  { value: 'OWASP', label: 'OWASP' },
];

// The instant score is P(0–1) × Impact(1–10) × AssetCriticality(0.1–3), so it lives
// on the 0–30 scale — the label must use the Score Engine bands (≥7 critical, ≥4 high,
// ≥2 medium), not the 0–100 thresholds that left it stuck on "Bas".
const scoreLabel = (score: number) => {
  if (score >= 7) return 'Critique';
  if (score >= 4) return 'Élevé';
  if (score >= 2) return 'Moyen';
  return 'Bas';
};
const scoreColor = (score: number) => {
  if (score >= 7) return 'var(--critical)';
  if (score >= 4) return 'var(--high)';
  if (score >= 2) return 'var(--medium)';
  return 'var(--low)';
};

export const CreateRiskModal = ({ isOpen, onClose, onCreated }: CreateRiskModalProps) => {
  // Esc closes this overlay (spec §2).
  useEscapeToClose(isOpen, onClose);
  const { t } = useI18n();
  const lang = useUIStore((st) => st.lang);
  const { assets, fetchAssets, isLoading: assetsLoading } = useAssetStore();

  // Sector-driven help + first-risk suggestions (spec §5/§6). The sector comes
  // from the onboarding answers; with none, the suggestions fall back to the
  // generic trio and the help to generic examples.
  const { data: suggestions } = useOnboardingSuggestions();
  const sector = suggestions?.industry ?? '';
  const refreshActivation = useInvalidateActivation();

  const {
    register,
    handleSubmit,
    watch,
    setValue,
    formState: { errors, isSubmitting },
    reset,
  } = useForm<CreateRiskForm>({
    resolver: zodResolver(createRiskSchema),
    defaultValues: {
      title: '',
      description: '',
      impact: 5,
      probability: 0.5,
      assetCriticality: 1.5,
      framework: '',
      tags: [],
      asset_ids: [],
    },
  });

  const watchedImpact = watch('impact');
  const watchedProbability = watch('probability');
  const watchedCriticality = watch('assetCriticality');
  const watchedTitle = watch('title');
  const watchedDescription = watch('description');
  const watchedTags = watch('tags') ?? [];
  const watchedAssetIds = watch('asset_ids') ?? [];
  const watchedFramework = watch('framework');

  const score = useMemo(() => {
    return Number((watchedProbability * watchedImpact * watchedCriticality).toFixed(1));
  }, [watchedImpact, watchedProbability, watchedCriticality]);

  useEffect(() => {
    if (isOpen) {
      fetchAssets();
      setTimeout(() => {
        const input = document.querySelector('input[name="title"]') as HTMLInputElement | null;
        input?.focus();
      }, 50);
    }
  }, [isOpen, fetchAssets]);

  const handleClose = () => {
    reset();
    onClose();
  };

  const onSubmit = async (values: CreateRiskForm) => {
    try {
      const payload = {
        title: values.title,
        description: values.description,
        probability: values.probability,
        impact: values.impact,
        asset_criticality: values.assetCriticality,
        framework: values.framework || undefined,
        tags: values.tags,
        asset_ids: values.asset_ids,
        source: 'manual',
      };
      const created = await riskService.createRisk(payload);
      // No client-side celebration here. The server records risk.created; the
      // checklist re-reads it and decides whether this is a milestone worth a
      // burst. Firing confetti from the client is what made it fire at random.
      refreshActivation();
      toast.success(t('messages.riskCreatedSuccess'), {
        description: 'Le risque a été créé et le score est calculé en backend.',
        icon: <Zap className="w-4 h-4 text-primary" />,
      });
      onCreated?.(created);
      handleClose();
    } catch (err) {
      console.error(err);
      toast.error(t('errors.failedToCreateRisk'), {
        description: t('errors.serverError'),
        icon: <ShieldAlert className="w-4 h-4 text-danger-text" />,
      });
    }
  };

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={handleClose}
            className="fixed inset-0 z-40 bg-surface-overlay backdrop-blur-sm"
          />

          <motion.div
            initial={{ opacity: 0, scale: 0.96, y: 40 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: 40 }}
            transition={{ duration: 0.22, type: 'spring', stiffness: 240 }}
            className="fixed inset-0 z-50 flex items-center justify-center p-4"
          >
            {/* Bounded height + scrollable body so a tall form never pushes the header
                or the submit button off-screen (the modal used to be vertically centered
                with no max-height, hiding its own actions). Header and footer stay pinned. */}
            <div className="flex max-h-[90vh] w-full max-w-2xl flex-col overflow-hidden rounded-3xl border border-border bg-elevated shadow-card-lg">
              <div className="flex shrink-0 items-center justify-between gap-4 border-b border-border px-6 py-5">
                <div>
                  <h2 className="text-2xl font-semibold text-ink">{t('risks.createRisk')}</h2>
                  <p className="text-sm text-ink-muted">Créez un risque avec score en temps réel.</p>
                </div>
                <button type="button" onClick={handleClose} className="rounded-full p-2 text-ink-soft hover:bg-hover hover:text-ink transition-colors">
                  <X size={20} />
                </button>
              </div>

              <form onSubmit={handleSubmit(onSubmit)} className="flex min-h-0 flex-1 flex-col">
                <div className="flex-1 space-y-6 overflow-y-auto px-6 py-6 scrollbar-thin">
                {/* Guided first risk (spec §5). Three drafts drawn from the
                    sector chosen at signup. We do NOT create anything: clicking
                    one fills the form, and the user adjusts and validates it —
                    which is the difference between "the product made a risk" and
                    "I made my first risk". Only shown while the form is untouched
                    so it never gets in the way of someone who knows what to type. */}
                <SuggestionPicker
                  suggestions={suggestions?.risks ?? []}
                  visible={!watchedTitle?.trim() && !watchedDescription?.trim()}
                  lang={lang}
                  onPick={(sugg) => {
                    setValue('title', sugg.title, { shouldValidate: true });
                    setValue('description', sugg.description, { shouldValidate: true });
                    setValue('probability', sugg.probability, { shouldValidate: true });
                    setValue('impact', sugg.impact, { shouldValidate: true });
                    if (sugg.suggested_tags?.length) setValue('tags', sugg.suggested_tags);
                  }}
                />
                <Input
                  label={t('risks.riskName')}
                  {...register('title')}
                  error={errors.title?.message}
                  disabled={isSubmitting}
                />
                <div className="space-y-1.5">
                  <label htmlFor="create-risk-description" className="text-xs font-semibold uppercase tracking-[0.18em] text-ink-muted">{t('risks.riskDescription')}</label>
                  <textarea
                    id="create-risk-description"
                    {...register('description')}
                    rows={5}
                    className="w-full rounded-3xl border border-border bg-elevated px-4 py-3 text-sm text-ink outline-none focus:ring-2 focus:ring-primary/40"
                    disabled={isSubmitting}
                  />
                  {errors.description && <p className="text-xs text-danger-text">{errors.description.message}</p>}
                </div>

                <div className="grid gap-4 sm:grid-cols-3">
                  <div className="space-y-2">
                    <label className="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-[0.18em] text-ink-muted">
                      {t('risks.probability')}
                      <FieldHelp field="probability" lang={lang} sector={sector} />
                    </label>
                    <input
                      type="range"
                      min={0}
                      max={1}
                      step={0.05}
                      {...register('probability', { valueAsNumber: true })}
                      className="w-full"
                    />
                    <div className="flex items-center justify-between text-xs text-ink-muted">
                      <span>0</span>
                      <span>{watchedProbability.toFixed(2)}</span>
                      <span>1</span>
                    </div>
                  </div>

                  <div className="space-y-2">
                    <label className="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-[0.18em] text-ink-muted">
                      {t('risks.impact')}
                      <FieldHelp field="impact" lang={lang} sector={sector} />
                    </label>
                    <input
                      type="range"
                      min={1}
                      max={10}
                      step={1}
                      {...register('impact', { valueAsNumber: true })}
                      className="w-full"
                    />
                    <div className="flex items-center justify-between text-xs text-ink-muted">
                      <span>1</span>
                      <span>{watchedImpact}</span>
                      <span>10</span>
                    </div>
                  </div>

                  <div className="space-y-2">
                    <label className="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-[0.18em] text-ink-muted">
                      {t('risks.riskAssetCriticality')}
                      <FieldHelp field="asset_criticality" lang={lang} sector={sector} />
                    </label>
                    <input
                      type="range"
                      min={0.1}
                      max={3}
                      step={0.1}
                      {...register('assetCriticality', { valueAsNumber: true })}
                      className="w-full"
                    />
                    <div className="flex items-center justify-between text-xs text-ink-muted">
                      <span>0.1</span>
                      <span>{watchedCriticality.toFixed(1)}</span>
                      <span>3.0</span>
                    </div>
                  </div>
                </div>

                <motion.div initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} className="rounded-3xl border border-border bg-hover p-4">
                  <div className="flex items-center justify-between gap-4">
                    <div>
                      <p className="text-xs uppercase tracking-[0.18em] text-ink-muted">Score instantané (Probabilité × Impact × Criticité)</p>
                      <p className="text-3xl font-semibold text-ink">{score}</p>
                    </div>
                    <div className="rounded-3xl px-4 py-2 text-xs font-semibold" style={{ background: `color-mix(in srgb, ${scoreColor(score)} 16%, transparent)`, color: scoreColor(score) }}>{scoreLabel(score)}</div>
                  </div>
                </motion.div>

                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="space-y-2">
                    <label className="text-xs font-semibold uppercase tracking-[0.18em] text-ink-muted">{t('risks.riskFramework')}</label>
                    <select
                      {...register('framework')}
                      className="w-full rounded-3xl border border-border bg-elevated px-4 py-3 text-sm text-ink"
                      disabled={isSubmitting}
                    >
                      <option value="">Sélectionnez un cadre</option>
                      {frameworkOptions.map((option) => (
                        <option key={option.value} value={option.value}>{option.label}</option>
                      ))}
                    </select>
                  </div>
                  <div className="space-y-2">
                    <label className="text-xs font-semibold uppercase tracking-[0.18em] text-ink-muted">{t('risks.riskAssets')}</label>
                    <div className="rounded-3xl border border-border bg-app p-3 min-h-[120px] overflow-y-auto">
                      {assetsLoading ? (
                        <p className="text-xs text-ink-muted">Chargement des assets...</p>
                      ) : assets.length === 0 ? (
                        <p className="text-xs text-ink-muted">Aucun asset disponible</p>
                      ) : (
                        <div className="grid gap-2">
                          {assets.map((asset) => (
                            <button
                              type="button"
                              key={asset.id}
                              onClick={() => {
                                const current = watchedAssetIds || [];
                                if (current.includes(asset.id)) {
                                  setValue('asset_ids', current.filter((id) => id !== asset.id), { shouldValidate: true });
                                } else {
                                  setValue('asset_ids', [...current, asset.id], { shouldValidate: true });
                                }
                              }}
                              className={`w-full rounded-2xl border px-3 py-2 text-left text-sm transition-colors ${watchedAssetIds.includes(asset.id) ? 'border-primary bg-primary/10 text-accent' : 'border-border bg-app text-ink-soft hover:border-border-strong'}`}
                            >
                              <div className="flex items-center gap-2">
                                <Database size={16} />
                                <span>{asset.name}</span>
                              </div>
                            </button>
                          ))}
                        </div>
                      )}
                    </div>
                  </div>
                </div>

                <Input
                  label={t('risks.riskTags')}
                  {...register('tags', {
                    setValueAs: (value) => typeof value === 'string' ? value.split(',').map((tag) => tag.trim()).filter(Boolean) : value,
                  })}
                  placeholder="critical, api, cloud"
                  disabled={isSubmitting}
                />

                </div>

                <div className="flex shrink-0 flex-wrap justify-end gap-3 border-t border-border bg-elevated px-6 py-4">
                  <Button type="button" variant="ghost" onClick={handleClose}>Annuler</Button>
                  <Button type="submit" variant="secondary" isLoading={isSubmitting} className="gap-2">
                    <Zap size={16} /> {t('common.save')}
                  </Button>
                </div>
              </form>
            </div>
          </motion.div>
        </>
        )}
      </AnimatePresence>
    );
};

/**
 * The three pre-filled first-risk drafts (spec §5).
 *
 * Clicking one fills the form; it does not submit, and it does not create
 * anything. The user still has to look at the numbers and press save — which is
 * exactly the point: they must feel this is THEIR risk, not one the product
 * invented on their behalf.
 */
function SuggestionPicker({
  suggestions,
  visible,
  lang,
  onPick,
}: {
  suggestions: RiskSuggestion[];
  visible: boolean;
  lang: 'fr' | 'en';
  onPick: (s: RiskSuggestion) => void;
}) {
  if (!visible || suggestions.length === 0) return null;
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  return (
    <div className="rounded-3xl border border-border bg-hover p-4" data-testid="risk-suggestions">
      <div className="flex items-center gap-2 mb-1">
        <Sparkles size={15} className="text-primary" />
        <span className="text-sm font-semibold text-ink">
          {tr('Partir d’un risque courant de votre secteur', 'Start from a common risk in your sector')}
        </span>
      </div>
      <p className="text-xs text-ink-muted mb-3">
        {tr(
          'Nous remplissons le formulaire ; à vous d’ajuster les valeurs et de valider.',
          'We fill the form in; you adjust the values and confirm.',
        )}
      </p>
      <div className="grid gap-2">
        {suggestions.map((s) => (
          <button
            key={s.key}
            type="button"
            data-testid={`risk-suggestion-${s.key}`}
            onClick={() => onPick(s)}
            className="text-left rounded-2xl border border-border bg-elevated px-3.5 py-3 transition-colors hover:border-primary"
          >
            <div className="flex items-center justify-between gap-3">
              <span className="text-[13.5px] font-semibold text-ink">{s.title}</span>
              <span className="mono shrink-0 text-[11px] text-ink-muted">
                P {s.probability.toFixed(2)} · I {s.impact}
              </span>
            </div>
            <p className="mt-1 line-clamp-2 text-[12px] leading-snug text-ink-soft">{s.description}</p>
          </button>
        ))}
      </div>
    </div>
  );
}

export default CreateRiskModal;
