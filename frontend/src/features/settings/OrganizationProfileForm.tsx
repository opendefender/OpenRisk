// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Settings › General — the editable organization profile (#299).
//
// Rendered only when the server says the caller may edit (`can_edit`); the
// read-only view stays the answer for everyone else. The rules below mirror
// domain.OrganizationProfilePatch.Normalize so a value the form accepts is a
// value the API accepts — the server stays the authority and its named 400 is
// shown when the two ever disagree.

import { useEffect, useMemo } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { toast } from 'sonner';

import { Button, Field, Input, Select, Textarea } from '../../shared/ds';
import { apiErrorMessage } from '../../lib/apiError';
import { ENABLED_LOCALES, LOCALES } from '../../i18n/locales';
import { ACCENT_LABELS, ACCENT_PRESETS } from '../../shared/accentPresets';
import {
  DATE_FORMATS,
  ORG_SIZES,
  type OrganizationProfilePatch,
  type OrganizationView,
} from '../organization/organizationService';
import { useUpdateOrganization } from '../organization/useOrganization';
import {
  organizationProfileSchema,
  type OrganizationProfileValues,
} from './organizationProfileSchema';

type Tr = (fr: string, en: string) => string;

/** Every IANA zone the browser knows, or a short list where it cannot say. */
function timeZones(): string[] {
  try {
    return Intl.supportedValuesOf('timeZone');
  } catch {
    return [
      'UTC',
      'Africa/Douala',
      'Africa/Abidjan',
      'Africa/Casablanca',
      'Europe/Paris',
      'America/Montreal',
    ];
  }
}

function valuesOf(org: OrganizationView): OrganizationProfileValues {
  return {
    name: org.name ?? '',
    industry: org.industry ?? '',
    size: org.size ?? '',
    website: org.website ?? '',
    description: org.description ?? '',
    timezone: org.timezone ?? '',
    default_locale: org.default_locale ?? '',
    date_format: org.date_format ?? '',
    accent: org.accent ?? '',
  };
}

export function OrganizationProfileForm({ org, tr }: { org: OrganizationView; tr: Tr }) {
  const update = useUpdateOrganization();
  const schema = useMemo(() => organizationProfileSchema(tr), [tr]);
  const zones = useMemo(() => timeZones(), []);
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isDirty },
  } = useForm<OrganizationProfileValues>({
    resolver: zodResolver(schema),
    defaultValues: valuesOf(org),
  });

  // Re-sync when the saved profile changes underneath the form (another tab, a
  // rollback), but never while the user has unsaved edits.
  useEffect(() => {
    if (!isDirty) reset(valuesOf(org));
  }, [org, isDirty, reset]);

  const onSubmit = (values: OrganizationProfileValues) => {
    const patch: OrganizationProfilePatch = values as OrganizationProfilePatch;
    update.mutate(patch, {
      onSuccess: (saved) => {
        reset(valuesOf(saved));
        toast.success(tr('Profil de l’organisation enregistré', 'Organization profile saved'));
      },
      onError: (err) => {
        reset(valuesOf(org), { keepValues: true });
        toast.error(
          apiErrorMessage(err) || tr('Enregistrement impossible', 'Could not save the profile'),
        );
      },
    });
  };

  const status = (k: keyof OrganizationProfileValues) => (errors[k] ? 'invalid' : 'default');
  const zoneOptions =
    org.timezone && !zones.includes(org.timezone) ? [org.timezone, ...zones] : zones;

  return (
    <form
      onSubmit={handleSubmit(onSubmit)}
      noValidate
      data-testid="org-profile-form"
      className="flex flex-col gap-5"
    >
      <fieldset
        className="grid gap-4"
        style={{ gridTemplateColumns: 'repeat(auto-fit,minmax(220px,1fr))' }}
      >
        <legend className="text-[12px] font-semibold uppercase tracking-wide text-ink-muted mb-3">
          {tr('Identité', 'Identity')}
        </legend>
        <Field
          label={tr('Nom', 'Name')}
          required
          message={errors.name?.message}
          status={status('name')}
        >
          <Input {...register('name')} autoComplete="organization" />
        </Field>
        <Field
          label={tr('Secteur', 'Industry')}
          message={errors.industry?.message}
          status={status('industry')}
        >
          <Input {...register('industry')} placeholder={tr('ex. Banque', 'e.g. Banking')} />
        </Field>
        <Field label={tr('Taille', 'Size')}>
          <Select {...register('size')}>
            <option value="">{tr('Non précisée', 'Not specified')}</option>
            {ORG_SIZES.map((s) => (
              <option key={s} value={s}>
                {tr(`${s} personnes`, `${s} people`)}
              </option>
            ))}
          </Select>
        </Field>
        <Field
          label={tr('Site web', 'Website')}
          message={errors.website?.message}
          status={status('website')}
        >
          <Input {...register('website')} type="url" inputMode="url" placeholder="https://" />
        </Field>
        <Field
          label={tr('Description', 'Description')}
          message={errors.description?.message}
          status={status('description')}
          className="col-span-full"
        >
          <Textarea {...register('description')} rows={3} />
        </Field>
      </fieldset>

      <fieldset
        className="grid gap-4"
        style={{ gridTemplateColumns: 'repeat(auto-fit,minmax(220px,1fr))' }}
      >
        <legend className="text-[12px] font-semibold uppercase tracking-wide text-ink-muted mb-3">
          {tr('Régionalisation', 'Regional settings')}
        </legend>
        <Field
          label={tr('Fuseau horaire', 'Time zone')}
          description={tr(
            'Utilisé pour les échéances et les rapports.',
            'Used for due dates and reports.',
          )}
        >
          <Select {...register('timezone')}>
            <option value="">{tr('Non défini', 'Not set')}</option>
            {zoneOptions.map((z) => (
              <option key={z} value={z}>
                {z}
              </option>
            ))}
          </Select>
        </Field>
        <Field
          label={tr('Langue par défaut', 'Default language')}
          description={tr(
            'Pour les membres qui n’en ont pas choisi.',
            'For members who have not chosen one.',
          )}
        >
          <Select {...register('default_locale')}>
            <option value="">{tr('Non définie', 'Not set')}</option>
            {ENABLED_LOCALES.map((c) => (
              <option key={c} value={c}>
                {LOCALES[c].nativeName}
              </option>
            ))}
          </Select>
        </Field>
        <Field label={tr('Format de date', 'Date format')}>
          <Select {...register('date_format')}>
            <option value="">{tr('Selon la langue', 'Follow the language')}</option>
            {DATE_FORMATS.map((f) => (
              <option key={f} value={f}>
                {f}
              </option>
            ))}
          </Select>
        </Field>
      </fieldset>

      <fieldset
        className="grid gap-4"
        style={{ gridTemplateColumns: 'repeat(auto-fit,minmax(220px,1fr))' }}
      >
        <legend className="text-[12px] font-semibold uppercase tracking-wide text-ink-muted mb-3">
          {tr('Identité visuelle', 'Branding')}
        </legend>
        <Field
          label={tr('Couleur d’accent', 'Accent color')}
          description={tr(
            'Appliquée à l’interface de tous les membres. Chacun peut la changer sur son appareil.',
            'Applied to every member’s interface. Anyone can change it on their own device.',
          )}
        >
          <Select {...register('accent')} data-testid="org-accent">
            <option value="">{tr('Par défaut', 'Default')}</option>
            {ACCENT_PRESETS.map((a) => (
              <option key={a} value={a}>
                {ACCENT_LABELS[a]}
              </option>
            ))}
          </Select>
        </Field>
      </fieldset>

      <div className="flex justify-end gap-2">
        <Button
          type="button"
          variant="ghost"
          disabled={!isDirty || update.isPending}
          onClick={() => reset(valuesOf(org))}
        >
          {tr('Annuler', 'Cancel')}
        </Button>
        <Button
          type="submit"
          variant="primary"
          disabled={!isDirty || update.isPending}
          data-testid="org-profile-save"
        >
          {update.isPending ? tr('Enregistrement…', 'Saving…') : tr('Enregistrer', 'Save')}
        </Button>
      </div>
    </form>
  );
}
