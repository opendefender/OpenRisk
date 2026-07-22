// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useEffect, useMemo } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { X, Zap, Database, ShieldAlert } from 'lucide-react';
import { toast } from 'sonner';
import { useAssetStore } from '../../hooks/useAssetStore';
import { riskService, type Risk } from '../../services/riskService';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { useI18n } from '../../hooks/useI18n';

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

const scoreLabel = (score: number) => {
  if (score >= 80) return 'Critique';
  if (score >= 60) return 'Élevé';
  if (score >= 40) return 'Moyen';
  return 'Bas';
};

export const CreateRiskModal = ({ isOpen, onClose, onCreated }: CreateRiskModalProps) => {
  const { t } = useI18n();
  const { assets, fetchAssets, isLoading: assetsLoading } = useAssetStore();

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
        icon: <ShieldAlert className="w-4 h-4 text-red-500" />,
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
            className="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm"
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
            <div className="flex max-h-[90vh] w-full max-w-2xl flex-col overflow-hidden rounded-3xl border border-zinc-800 bg-zinc-950/95 shadow-2xl shadow-black/40">
              <div className="flex shrink-0 items-center justify-between gap-4 border-b border-zinc-800 px-6 py-5">
                <div>
                  <h2 className="text-2xl font-semibold">{t('risks.createRisk')}</h2>
                  <p className="text-sm text-zinc-500">Créez un risque avec score en temps réel.</p>
                </div>
                <button type="button" onClick={handleClose} className="rounded-full p-2 text-zinc-400 hover:bg-white/10 hover:text-white transition-colors">
                  <X size={20} />
                </button>
              </div>

              <form onSubmit={handleSubmit(onSubmit)} className="flex min-h-0 flex-1 flex-col">
                <div className="flex-1 space-y-6 overflow-y-auto px-6 py-6 scrollbar-thin">
                <Input
                  label={t('risks.riskName')}
                  {...register('title')}
                  error={errors.title?.message}
                  disabled={isSubmitting}
                />
                <div className="space-y-1.5">
                  <label className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">{t('risks.riskDescription')}</label>
                  <textarea
                    {...register('description')}
                    rows={5}
                    className="w-full rounded-3xl border border-zinc-800 bg-zinc-950 px-4 py-3 text-sm text-white outline-none focus:ring-2 focus:ring-primary/40"
                    disabled={isSubmitting}
                  />
                  {errors.description && <p className="text-xs text-red-500">{errors.description.message}</p>}
                </div>

                <div className="grid gap-4 sm:grid-cols-3">
                  <div className="space-y-2">
                    <label className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">{t('risks.probability')}</label>
                    <input
                      type="range"
                      min={0}
                      max={1}
                      step={0.05}
                      {...register('probability', { valueAsNumber: true })}
                      className="w-full"
                    />
                    <div className="flex items-center justify-between text-xs text-zinc-400">
                      <span>0</span>
                      <span>{watchedProbability.toFixed(2)}</span>
                      <span>1</span>
                    </div>
                  </div>

                  <div className="space-y-2">
                    <label className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">{t('risks.impact')}</label>
                    <input
                      type="range"
                      min={1}
                      max={10}
                      step={1}
                      {...register('impact', { valueAsNumber: true })}
                      className="w-full"
                    />
                    <div className="flex items-center justify-between text-xs text-zinc-400">
                      <span>1</span>
                      <span>{watchedImpact}</span>
                      <span>10</span>
                    </div>
                  </div>

                  <div className="space-y-2">
                    <label className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">{t('risks.riskAssetCriticality')}</label>
                    <input
                      type="range"
                      min={0.1}
                      max={3}
                      step={0.1}
                      {...register('assetCriticality', { valueAsNumber: true })}
                      className="w-full"
                    />
                    <div className="flex items-center justify-between text-xs text-zinc-400">
                      <span>0.1</span>
                      <span>{watchedCriticality.toFixed(1)}</span>
                      <span>3.0</span>
                    </div>
                  </div>
                </div>

                <motion.div initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} className="rounded-3xl border border-zinc-800 bg-zinc-900/60 p-4">
                  <div className="flex items-center justify-between gap-4">
                    <div>
                      <p className="text-xs uppercase tracking-[0.18em] text-zinc-500">Score instantané</p>
                      <p className="text-3xl font-semibold text-white">{score}</p>
                    </div>
                    <div className="rounded-3xl bg-zinc-950 px-4 py-2 text-xs text-zinc-300">{scoreLabel(score)}</div>
                  </div>
                </motion.div>

                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="space-y-2">
                    <label className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">{t('risks.riskFramework')}</label>
                    <select
                      {...register('framework')}
                      className="w-full rounded-3xl border border-zinc-800 bg-zinc-950 px-4 py-3 text-sm text-white"
                      disabled={isSubmitting}
                    >
                      <option value="">Sélectionnez un cadre</option>
                      {frameworkOptions.map((option) => (
                        <option key={option.value} value={option.value}>{option.label}</option>
                      ))}
                    </select>
                  </div>
                  <div className="space-y-2">
                    <label className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">{t('risks.riskAssets')}</label>
                    <div className="rounded-3xl border border-zinc-800 bg-zinc-950 p-3 min-h-[120px] overflow-y-auto">
                      {assetsLoading ? (
                        <p className="text-xs text-zinc-500">Chargement des assets...</p>
                      ) : assets.length === 0 ? (
                        <p className="text-xs text-zinc-500">Aucun asset disponible</p>
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
                              className={`w-full rounded-2xl border px-3 py-2 text-left text-sm transition-colors ${watchedAssetIds.includes(asset.id) ? 'border-primary bg-primary/10 text-white' : 'border-zinc-800 bg-zinc-950 text-zinc-300 hover:border-zinc-600'}`}
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

                <div className="flex shrink-0 flex-wrap justify-end gap-3 border-t border-zinc-800 bg-zinc-950/95 px-6 py-4">
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

export default CreateRiskModal;
