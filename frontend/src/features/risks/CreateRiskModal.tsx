// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).
//
// "Nouveau risque" on the October 2026 redesign (#913): title, category, owner,
// the two 5-level pickers, the live score and an optional description, in a
// 580 px modal with a pinned footer.
//
// What the mockup does not draw and the product needs stays, compactly:
//   - "Actifs concernés": the server derives the asset-criticality term of the
//     score from the linked assets, so a risk without assets is scored with the
//     default factor. Picked here as chips.
//   - the first-risk suggestions of the onboarding (only while the form is
//     untouched);
//   - "Plus d'options": free tags and the mapping to controls, collapsed.
//
// The live figure is /score/preview, debounced: its `risk_score` and
// `risk_criticality` come from the same frozen Score Engine as the stored score
// (P × I × criticality), so what is shown here is what the register will show.
// Asset criticality is neutral until the risk is saved with its assets.

import { useEffect, useMemo, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { ChevronDown, Database, Sparkles, X } from 'lucide-react';
import { toast } from 'sonner';
import { useQueryClient } from '@tanstack/react-query';
import { useAssetStore } from '../../hooks/useAssetStore';
import { riskService, type Risk } from '../../services/riskService';
import { taxonomyService } from '../../services/taxonomyService';
import { ComplianceMappingField, type MappingDraft } from './ComplianceMappingField';
import { useRiskCategories, IMPORTED_FRAMEWORKS_KEY } from './useTaxonomy';
import { ImportFrameworkDialog } from '../compliance/ComplianceModals';
import {
  Button,
  Collapse,
  Field,
  Input,
  Listbox,
  Modal,
  TagInput,
  Textarea,
  type ListboxOption,
} from '../../shared/ds';
import { useRiskTagLabels } from './useRiskTagLabels';
import { useI18n } from '../../hooks/useI18n';
import { useScorePreview } from '../../hooks/useScore';
import { bandTextColor } from '../../services/scoreService';
import { FieldHelp } from '../../shared/FieldHelp';
import { useUIStore } from '../../store/uiStore';
import { useInvalidateActivation, useOnboardingSuggestions } from '../onboarding/useActivation';
import type { RiskSuggestion } from '../../services/activationService';
import type { LocaleCode } from '../../i18n/locales';
import { LevelPicker } from './LevelPicker';
import { formatImpact, formatProbability, formatScore } from './riskFormat';
import {
  LEVELS,
  impactForLevel,
  impactLevel,
  probabilityForLevel,
  probabilityLevel,
} from './riskLevels';
import { useRegisterMembers } from './useRiskRegisterData';
import { memberName } from './riskMembers';

const NONE = 'none';

interface CreateRiskForm {
  title: string;
  description: string;
  category_id: string;
  owner_id: string;
  probability: number;
  impact: number;
  asset_ids: string[];
  tags: string[];
}

interface CreateRiskModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreated?: (risk: Risk) => void;
  /** Assets the new risk starts linked to (a vendor's "Créer un risque", #908). */
  initialAssetIds?: string[];
  /** A starting title the user can change. */
  initialTitle?: string;
}

export const CreateRiskModal = ({
  isOpen,
  onClose,
  onCreated,
  initialAssetIds,
  initialTitle,
}: CreateRiskModalProps) => {
  const { t, locale } = useI18n();
  const lang = useUIStore((st) => st.lang);
  const { assets, fetchAssets, isLoading: assetsLoading } = useAssetStore();
  const { members, isLoading: membersLoading } = useRegisterMembers();

  // First-risk suggestions (onboarding §5/§6), drawn from the sector chosen at signup.
  const { data: suggestions } = useOnboardingSuggestions();
  // Sector-driven help on P and I: the sector chosen at signup.
  const sector = suggestions?.industry ?? '';
  const refreshActivation = useInvalidateActivation();
  const tagLabels = useRiskTagLabels();
  const { data: categories } = useRiskCategories();
  const queryClient = useQueryClient();

  const schema = useMemo(
    () =>
      z.object({
        title: z
          .string()
          .trim()
          .min(4, t('risks.new.errors.titleShort'))
          .max(100, t('risks.new.errors.titleLong')),
        description: z.string().max(5000, t('risks.new.errors.descriptionLong')),
        category_id: z.string(),
        owner_id: z.string(),
        probability: z.number().min(0).max(1),
        impact: z.number().min(0).max(10),
        asset_ids: z.array(z.string()),
        tags: z.array(z.string()),
      }),
    [t],
  );

  const {
    register,
    handleSubmit,
    watch,
    setValue,
    control,
    formState: { errors, isSubmitting },
    reset,
  } = useForm<CreateRiskForm>({
    resolver: zodResolver(schema),
    defaultValues: {
      title: initialTitle ?? '',
      description: '',
      category_id: '',
      owner_id: '',
      // Level 3 of each, as the design starts: the midpoints of their buckets.
      probability: probabilityForLevel(3),
      impact: impactForLevel(3),
      asset_ids: initialAssetIds ?? [],
      tags: [],
    },
  });

  const probability = watch('probability');
  const impact = watch('impact');
  const title = watch('title');
  const description = watch('description');
  const assetIds = watch('asset_ids');

  // Mappings are written after the risk exists (they reference its id), and the
  // import drawer must open over the form without unmounting it.
  const [mappings, setMappings] = useState<MappingDraft[]>([]);
  const [importOpen, setImportOpen] = useState(false);
  const [moreOpen, setMoreOpen] = useState(false);
  // The assets the risk can be linked to are read when the modal opens.
  useEffect(() => {
    if (isOpen) void fetchAssets();
  }, [isOpen, fetchAssets]);

  // The live figure comes from the server, debounced at 300 ms.
  const { data: preview } = useScorePreview({ scope: 'risk', probability, impact }, isOpen);

  const handleClose = () => {
    reset();
    setMappings([]);
    setMoreOpen(false);
    onClose();
  };

  const onSubmit = async (values: CreateRiskForm) => {
    try {
      const created = await riskService.createRisk({
        title: values.title.trim(),
        description: values.description.trim() || undefined,
        probability: values.probability,
        impact: values.impact,
        tags: values.tags,
        category_id: values.category_id || undefined,
        owner_id: values.owner_id || undefined,
        asset_ids: values.asset_ids,
        source: 'manual',
      });
      // Mappings are written after the risk exists. A failure here must NOT lose
      // the risk that was just created, so each one is best-effort, reported apart.
      const failed: string[] = [];
      for (const m of mappings) {
        try {
          await taxonomyService.createMapping(created.id, {
            framework_id: m.framework_id,
            control_id: m.control_id ?? null,
          });
        } catch {
          failed.push(m.label);
        }
      }
      if (failed.length) {
        toast.warning(
          t('risks.new.mappingsFailed', { count: failed.length, names: failed.join(', ') }),
        );
      }
      // The server records risk.created; the checklist re-reads it and decides
      // whether this is a milestone. No client-side celebration.
      refreshActivation();
      toast.success(t('messages.riskCreatedSuccess'));
      onCreated?.(created);
      handleClose();
    } catch (err) {
      console.error(err);
      toast.error(t('errors.failedToCreateRisk'), { description: t('errors.serverError') });
    }
  };

  const categoryOptions: ListboxOption[] = [
    { value: NONE, label: t('risks.uncategorised') },
    ...(categories ?? []).map((c) => ({ value: c.id, label: c.name })),
  ];
  const ownerOptions: ListboxOption[] = [
    { value: NONE, label: t('risks.new.ownerNone') },
    ...members.map((m) => ({ value: m.user_id, label: memberName(m) })),
  ];
  const pickableAssets: ListboxOption[] = assets
    .filter((a) => !assetIds.includes(a.id))
    .map((a) => ({ value: a.id, label: a.name }));
  const pNames = LEVELS.map((k) => t(`risks.level.probability.${k}`));
  const iNames = LEVELS.map((k) => t(`risks.level.impact.${k}`));

  // The register's own score and band, from the server (POST /score/preview).
  const band = preview?.risk_criticality;
  const bandText = band ? t(`risks.band.${band}`) : null;

  return (
    <>
      <Modal
        open={isOpen}
        onClose={handleClose}
        dismissable={!isSubmitting}
        size="md"
        className="max-w-145"
        title={t('risks.new.title')}
        subtitle={t('risks.new.subtitle')}
        closeLabel={t('common.close')}
        footer={
          <>
            <Button type="button" variant="secondary" onClick={handleClose} disabled={isSubmitting}>
              {t('common.cancel')}
            </Button>
            <Button
              type="submit"
              form="create-risk-form"
              variant="primary"
              loading={isSubmitting}
              disabled={title.trim().length < 4}
              data-testid="create-risk-submit"
            >
              {t('risks.new.submit')}
            </Button>
          </>
        }
      >
        <form
          id="create-risk-form"
          onSubmit={handleSubmit(onSubmit)}
          noValidate
          className="grid gap-4"
        >
          <Field
            label={t('risks.new.titleLabel')}
            message={errors.title?.message}
            status={errors.title?.message ? 'invalid' : 'default'}
          >
            <Input
              {...register('title')}
              autoFocus
              placeholder={t('risks.new.titlePlaceholder')}
              disabled={isSubmitting}
            />
          </Field>

          <div className="grid grid-cols-2 gap-3">
            <label className="grid gap-1.5 min-w-0">
              <span className="text-[12.5px] font-semibold text-ink">{t('risks.category')}</span>
              <Controller
                name="category_id"
                control={control}
                render={({ field }) => (
                  <Listbox
                    value={field.value || NONE}
                    options={categoryOptions}
                    onChange={(v) => field.onChange(v === NONE ? '' : v)}
                    disabled={isSubmitting}
                    data-testid="create-risk-category"
                  />
                )}
              />
            </label>
            <label className="grid gap-1.5 min-w-0">
              <span className="text-[12.5px] font-semibold text-ink">
                {t('risks.new.ownerLabel')}
              </span>
              <Controller
                name="owner_id"
                control={control}
                render={({ field }) => (
                  <Listbox
                    value={field.value || NONE}
                    options={ownerOptions}
                    onChange={(v) => field.onChange(v === NONE ? '' : v)}
                    loading={membersLoading}
                    disabled={isSubmitting}
                    data-testid="create-risk-owner"
                  />
                )}
              />
            </label>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <LevelPicker
              label={t('risks.probability')}
              level={probabilityLevel(probability)}
              levelNames={pNames}
              exact={formatProbability(locale, probability)}
              disabled={isSubmitting}
              testId="create-risk-p"
              help={<FieldHelp field="probability" lang={lang} sector={sector} />}
              onPick={(k) => setValue('probability', probabilityForLevel(k), { shouldDirty: true })}
            />
            <LevelPicker
              label={t('risks.impact')}
              level={impactLevel(impact)}
              levelNames={iNames}
              exact={formatImpact(locale, impact)}
              disabled={isSubmitting}
              testId="create-risk-i"
              help={<FieldHelp field="impact" lang={lang} sector={sector} />}
              onPick={(k) => setValue('impact', impactForLevel(k), { shouldDirty: true })}
            />
          </div>

          <div
            className="flex items-center gap-3.5 px-3.5 py-3 rounded-[10px] bg-surface-1 border border-border-subtle"
            data-testid="create-risk-score"
          >
            <span
              className="mono text-[26px] font-semibold leading-none"
              style={{ color: bandTextColor(band) }}
            >
              {formatScore(locale, preview?.risk_score)}
            </span>
            <span className="flex-1 min-w-0 text-[12.5px] text-ink-soft">
              <span className="block">{t('risks.new.liveScore')}</span>
              <span className="block text-[11.5px] text-ink-muted">
                {t('risks.new.liveScoreHint')}
              </span>
            </span>
            {bandText && (
              <span
                className="text-[11px] px-2 py-0.5 rounded-full font-semibold bg-surface-3"
                style={{ color: bandTextColor(band) }}
              >
                {bandText}
              </span>
            )}
          </div>

          <Field
            label={
              <>
                {t('risks.new.descriptionLabel')}{' '}
                <span className="font-normal text-ink-muted">— {t('risks.new.optional')}</span>
              </>
            }
            htmlFor="create-risk-description"
            message={errors.description?.message}
            status={errors.description?.message ? 'invalid' : 'default'}
          >
            <Textarea
              id="create-risk-description"
              {...register('description')}
              rows={3}
              placeholder={t('risks.new.descriptionPlaceholder')}
              disabled={isSubmitting}
            />
          </Field>

          <div className="grid gap-2">
            <span className="text-[12.5px] font-semibold text-ink">
              {t('risks.new.assetsLabel')}
            </span>
            {assetIds.length > 0 && (
              <div className="flex flex-wrap gap-1.5">
                {assetIds.map((id) => {
                  const name = assets.find((a) => a.id === id)?.name ?? id;
                  return (
                    <span
                      key={id}
                      className="inline-flex items-center gap-1.5 h-[26px] pl-2.5 pr-1 rounded-lg border border-border-default bg-surface-2 text-[12.5px] text-ink"
                    >
                      <Database size={13} aria-hidden="true" className="text-ink-muted" />
                      {name}
                      <button
                        type="button"
                        onClick={() =>
                          setValue(
                            'asset_ids',
                            assetIds.filter((x) => x !== id),
                            { shouldDirty: true },
                          )
                        }
                        aria-label={t('risks.new.removeAsset', { name })}
                        className="size-5 inline-flex items-center justify-center rounded-md text-ink-muted hover:bg-surface-3 hover:text-ink"
                      >
                        <X size={12} aria-hidden="true" />
                      </button>
                    </span>
                  );
                })}
              </div>
            )}
            <Listbox
              value=""
              options={pickableAssets}
              onChange={(id) => setValue('asset_ids', [...assetIds, id], { shouldDirty: true })}
              loading={assetsLoading}
              disabled={isSubmitting}
              placeholder={
                assets.length === 0 && !assetsLoading
                  ? t('assets.noAssets')
                  : t('risks.new.addAsset')
              }
              emptyLabel={t('risks.new.noMoreAssets')}
              aria-label={t('risks.new.addAsset')}
              data-testid="create-risk-assets"
            />
            <p className="m-0 text-[11.5px] text-ink-muted">{t('risks.new.assetsHint')}</p>
          </div>

          <SuggestionPicker
            suggestions={suggestions?.risks ?? []}
            visible={!title?.trim() && !description?.trim()}
            lang={lang}
            onPick={(sugg) => {
              setValue('title', sugg.title, { shouldValidate: true });
              setValue('description', sugg.description, { shouldValidate: true });
              setValue('probability', sugg.probability, { shouldValidate: true });
              setValue('impact', sugg.impact, { shouldValidate: true });
              if (sugg.suggested_tags?.length) setValue('tags', sugg.suggested_tags);
            }}
          />

          <div>
            <button
              type="button"
              aria-expanded={moreOpen}
              aria-controls="create-risk-more"
              onClick={() => setMoreOpen((v) => !v)}
              className="inline-flex items-center gap-1.5 text-[12.5px] font-semibold text-accent-strong hover:underline"
            >
              <ChevronDown
                size={14}
                aria-hidden="true"
                className="transition-transform duration-fast"
                style={{ transform: moreOpen ? 'rotate(180deg)' : undefined }}
              />
              {t('risks.new.more')}
            </button>
            <Collapse id="create-risk-more" open={moreOpen} className="grid gap-4 pt-3">
              <div className="grid gap-1.5">
                <label htmlFor="create-risk-tags" className="text-[12.5px] font-semibold text-ink">
                  {t('risks.riskTags')}
                </label>
                <Controller
                  name="tags"
                  control={control}
                  render={({ field }) => (
                    <TagInput
                      id="create-risk-tags"
                      value={field.value}
                      onValueChange={field.onChange}
                      labels={tagLabels}
                      disabled={isSubmitting}
                    />
                  )}
                />
              </div>
              <div className="grid gap-1.5">
                <span className="text-[12.5px] font-semibold text-ink">
                  {t('risks.riskFramework')}
                </span>
                <ComplianceMappingField
                  value={mappings}
                  onChange={setMappings}
                  disabled={isSubmitting}
                  onImportFramework={() => setImportOpen(true)}
                />
              </div>
            </Collapse>
          </div>
        </form>
      </Modal>

      {/* The import drawer opens OVER the form. The form is never unmounted, so a
          half-written risk survives the detour. */}
      {importOpen ? (
        <ImportFrameworkDialog
          onClose={() => setImportOpen(false)}
          onImported={() => {
            setImportOpen(false);
            void queryClient.invalidateQueries({ queryKey: IMPORTED_FRAMEWORKS_KEY });
          }}
        />
      ) : null}
    </>
  );
};

/**
 * The first-risk drafts (onboarding §5). Clicking one fills the form; it does
 * not submit and it does not create anything — the user still looks at the
 * numbers and confirms, so it is THEIR risk and not one the product invented.
 */
function SuggestionPicker({
  suggestions,
  visible,
  lang,
  onPick,
}: {
  suggestions: RiskSuggestion[];
  visible: boolean;
  lang: LocaleCode;
  onPick: (s: RiskSuggestion) => void;
}) {
  const { locale } = useI18n();
  const [open, setOpen] = useState(false);
  if (!visible || suggestions.length === 0) return null;
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  return (
    <div
      className="rounded-xl border border-border-default bg-surface-1 p-3"
      data-testid="risk-suggestions"
    >
      <button
        type="button"
        aria-expanded={open}
        aria-controls="risk-suggestions-list"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-2 text-left"
      >
        <Sparkles size={15} className="text-accent" aria-hidden="true" />
        <span className="flex-1 text-[13px] font-semibold text-ink">
          {tr(
            'Partir d’un risque courant de votre secteur',
            'Start from a common risk in your sector',
          )}
        </span>
        <ChevronDown
          size={14}
          aria-hidden="true"
          className="text-ink-muted transition-transform duration-fast"
          style={{ transform: open ? 'rotate(180deg)' : undefined }}
        />
      </button>
      {open && (
        <>
          <p className="text-xs text-ink-muted mb-2.5 mt-1">
            {tr(
              'Nous remplissons le formulaire ; à vous d’ajuster les valeurs et de valider.',
              'We fill the form in; you adjust the values and confirm.',
            )}
          </p>
          <div id="risk-suggestions-list" className="grid gap-2">
            {suggestions.map((s) => (
              <button
                key={s.key}
                type="button"
                data-testid={`risk-suggestion-${s.key}`}
                onClick={() => onPick(s)}
                className="text-left rounded-lg border border-border-default bg-surface-2 px-3 py-2.5 transition-colors hover:border-accent"
              >
                <div className="flex items-center justify-between gap-3">
                  <span className="text-[13px] font-semibold text-ink">{s.title}</span>
                  <span className="mono shrink-0 text-[11px] text-ink-muted">
                    P {formatProbability(locale, s.probability)} · I{' '}
                    {formatImpact(locale, s.impact)}
                  </span>
                </div>
                <p className="mt-1 mb-0 line-clamp-2 text-[12px] leading-snug text-ink-soft">
                  {s.description}
                </p>
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}

export default CreateRiskModal;
