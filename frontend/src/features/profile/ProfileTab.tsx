// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Settings › My profile (#719): the signed-in person's identity, avatar and
// preferences. Everything here acts on the session's own account.

import { useEffect, useMemo, useRef } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { toast } from 'sonner';

import { Button, Field, Input, Select, Textarea } from '../../shared/ds';
import { Card, ErrorState, SkeletonRows } from '../../shared/ui';
import { UserAvatar } from '../../shared/UserAvatar';
import { apiErrorMessage } from '../../lib/apiError';
import { ENABLED_LOCALES, LOCALES } from '../../i18n/locales';
import type { MyProfile, UserProfilePatch } from './profileService';
import { DATE_PATTERNS, profileSchema, THEME_MODES, type ProfileValues } from './profileSchema';
import { useDeleteAvatar, useMyProfile, useUpdateMyProfile, useUploadAvatar } from './useProfile';

type Tr = (fr: string, en: string) => string;

const MAX_AVATAR_BYTES = 1024 * 1024;
const AVATAR_TYPES = ['image/png', 'image/jpeg', 'image/webp'];

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

function valuesOf(p: MyProfile): ProfileValues {
  return {
    full_name: p.full_name ?? '',
    job_title: p.job_title ?? '',
    phone: p.phone ?? '',
    bio: p.bio ?? '',
    timezone: p.timezone ?? '',
    locale: p.locale ?? '',
    date_format: (DATE_PATTERNS as readonly string[]).includes(p.date_format)
      ? (p.date_format as ProfileValues['date_format'])
      : '',
    theme_mode: p.theme_mode ?? '',
  };
}

const Title = ({ children }: { children: React.ReactNode }) => (
  <div className="text-[14px] font-semibold text-ink mb-3.5">{children}</div>
);

export function ProfileTab({ tr }: { tr: Tr }) {
  const { data: profile, isLoading, isError, refetch } = useMyProfile();

  if (isLoading) {
    return (
      <Card style={{ padding: '20px 22px' }}>
        <SkeletonRows rows={5} />
      </Card>
    );
  }
  if (isError || !profile) {
    return (
      <Card style={{ padding: '20px 22px' }}>
        <ErrorState
          title={tr('Impossible de charger votre profil', 'Could not load your profile')}
          description={tr('Réessayez dans un instant.', 'Try again in a moment.')}
          onRetry={() => void refetch()}
        />
      </Card>
    );
  }
  return (
    <>
      <AvatarCard profile={profile} tr={tr} />
      <ProfileForm profile={profile} tr={tr} />
    </>
  );
}

function AvatarCard({ profile, tr }: { profile: MyProfile; tr: Tr }) {
  const input = useRef<HTMLInputElement>(null);
  const upload = useUploadAvatar();
  const remove = useDeleteAvatar();
  const busy = upload.isPending || remove.isPending;

  const onFile = (file: File | undefined) => {
    if (input.current) input.current.value = '';
    if (!file) return;
    if (!AVATAR_TYPES.includes(file.type)) {
      toast.error(
        tr('Choisissez une image PNG, JPEG ou WebP.', 'Choose a PNG, JPEG or WebP image.'),
      );
      return;
    }
    if (file.size > MAX_AVATAR_BYTES) {
      toast.error(tr('L’image doit faire 1 Mo au plus.', 'The image must be 1 MB or smaller.'));
      return;
    }
    upload.mutate(file, {
      onSuccess: () => toast.success(tr('Photo de profil mise à jour', 'Profile picture updated')),
      onError: (err) =>
        toast.error(apiErrorMessage(err) || tr('Téléversement impossible', 'Upload failed')),
    });
  };

  return (
    <Card style={{ padding: '20px 22px', marginBottom: 16 }}>
      <Title>{tr('Photo de profil', 'Profile picture')}</Title>
      <div className="flex items-center gap-4 flex-wrap">
        <UserAvatar
          userId={profile.id}
          name={profile.full_name || profile.email}
          hasAvatar={profile.has_avatar}
          size={64}
        />
        <div className="flex flex-col gap-2">
          <div className="flex gap-2 flex-wrap">
            <input
              ref={input}
              type="file"
              accept={AVATAR_TYPES.join(',')}
              className="sr-only"
              id="avatar-file"
              data-testid="avatar-input"
              onChange={(e) => onFile(e.target.files?.[0])}
            />
            <Button
              type="button"
              variant="secondary"
              disabled={busy}
              onClick={() => input.current?.click()}
            >
              {profile.has_avatar ? tr('Remplacer', 'Replace') : tr('Téléverser', 'Upload')}
            </Button>
            {profile.has_avatar && (
              <Button
                type="button"
                variant="ghost"
                disabled={busy}
                onClick={() =>
                  remove.mutate(undefined, {
                    onSuccess: () => toast.success(tr('Photo retirée', 'Picture removed')),
                    onError: (err) =>
                      toast.error(
                        apiErrorMessage(err) || tr('Suppression impossible', 'Could not remove'),
                      ),
                  })
                }
              >
                {tr('Retirer', 'Remove')}
              </Button>
            )}
          </div>
          <p className="text-[11.5px] text-ink-muted">
            {tr('PNG, JPEG ou WebP, 1 Mo maximum.', 'PNG, JPEG or WebP, 1 MB maximum.')}
          </p>
        </div>
      </div>
    </Card>
  );
}

function ProfileForm({ profile, tr }: { profile: MyProfile; tr: Tr }) {
  const update = useUpdateMyProfile();
  const schema = useMemo(() => profileSchema(tr), [tr]);
  const zones = useMemo(() => timeZones(), []);
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isDirty },
  } = useForm<ProfileValues>({ resolver: zodResolver(schema), defaultValues: valuesOf(profile) });

  useEffect(() => {
    if (!isDirty) reset(valuesOf(profile));
  }, [profile, isDirty, reset]);

  const onSubmit = (values: ProfileValues) => {
    const patch: UserProfilePatch = values;
    update.mutate(patch, {
      onSuccess: (saved) => {
        reset(valuesOf(saved));
        toast.success(tr('Profil enregistré', 'Profile saved'));
      },
      onError: (err) => {
        reset(valuesOf(profile), { keepValues: true });
        toast.error(apiErrorMessage(err) || tr('Enregistrement impossible', 'Could not save'));
      },
    });
  };

  const status = (k: keyof ProfileValues) => (errors[k] ? 'invalid' : 'default');
  const zoneOptions =
    profile.timezone && !zones.includes(profile.timezone) ? [profile.timezone, ...zones] : zones;
  const orgDefault = (value: string | undefined) =>
    value
      ? tr(`Selon l’organisation (${value})`, `Follow the organization (${value})`)
      : tr('Selon l’organisation', 'Follow the organization');
  const grid = { gridTemplateColumns: 'repeat(auto-fit,minmax(220px,1fr))' };

  return (
    <form onSubmit={handleSubmit(onSubmit)} noValidate data-testid="profile-form">
      <Card style={{ padding: '20px 22px', marginBottom: 16 }}>
        <Title>{tr('Identité', 'Identity')}</Title>
        <div className="grid gap-4" style={grid}>
          <Field
            label={tr('Nom complet', 'Full name')}
            required
            message={errors.full_name?.message}
            status={status('full_name')}
          >
            <Input {...register('full_name')} autoComplete="name" />
          </Field>
          <Field
            label={tr('Fonction', 'Job title')}
            message={errors.job_title?.message}
            status={status('job_title')}
          >
            <Input
              {...register('job_title')}
              autoComplete="organization-title"
              placeholder={tr('ex. RSSI', 'e.g. CISO')}
            />
          </Field>
          <Field
            label={tr('Téléphone', 'Phone')}
            message={errors.phone?.message}
            status={status('phone')}
          >
            <Input {...register('phone')} type="tel" autoComplete="tel" />
          </Field>
          <Field
            label={tr('E-mail', 'Email')}
            description={tr('Géré par votre administrateur.', 'Managed by your administrator.')}
          >
            <Input value={profile.email} readOnly disabled />
          </Field>
          <Field
            label={tr('Bio', 'Bio')}
            message={errors.bio?.message}
            status={status('bio')}
            className="col-span-full"
          >
            <Textarea {...register('bio')} rows={3} />
          </Field>
        </div>
      </Card>

      <Card style={{ padding: '20px 22px', marginBottom: 16 }}>
        <Title>{tr('Préférences', 'Preferences')}</Title>
        <div className="grid gap-4" style={grid}>
          <Field label={tr('Langue', 'Language')}>
            <Select {...register('locale')}>
              <option value="">
                {orgDefault(profile.locale ? undefined : profile.effective.locale)}
              </option>
              {ENABLED_LOCALES.map((c) => (
                <option key={c} value={c}>
                  {LOCALES[c].nativeName}
                </option>
              ))}
            </Select>
          </Field>
          <Field label={tr('Thème', 'Theme')}>
            <Select {...register('theme_mode')}>
              <option value="">{tr('Selon cet appareil', 'Follow this device')}</option>
              {THEME_MODES.map((m) => (
                <option key={m} value={m}>
                  {m === 'light'
                    ? tr('Clair', 'Light')
                    : m === 'dark'
                      ? tr('Sombre', 'Dark')
                      : tr('Système', 'System')}
                </option>
              ))}
            </Select>
          </Field>
          <Field label={tr('Fuseau horaire', 'Time zone')}>
            <Select {...register('timezone')}>
              <option value="">
                {orgDefault(profile.timezone ? undefined : profile.effective.timezone)}
              </option>
              {zoneOptions.map((z) => (
                <option key={z} value={z}>
                  {z}
                </option>
              ))}
            </Select>
          </Field>
          <Field label={tr('Format de date', 'Date format')}>
            <Select {...register('date_format')}>
              <option value="">
                {orgDefault(profile.date_format ? undefined : profile.effective.date_format)}
              </option>
              {DATE_PATTERNS.map((f) => (
                <option key={f} value={f}>
                  {f}
                </option>
              ))}
            </Select>
          </Field>
        </div>
      </Card>

      <div className="flex justify-end gap-2">
        <Button
          type="button"
          variant="ghost"
          disabled={!isDirty || update.isPending}
          onClick={() => reset(valuesOf(profile))}
        >
          {tr('Annuler', 'Cancel')}
        </Button>
        <Button
          type="submit"
          variant="primary"
          disabled={!isDirty || update.isPending}
          data-testid="profile-save"
        >
          {update.isPending ? tr('Enregistrement…', 'Saving…') : tr('Enregistrer', 'Save')}
        </Button>
      </div>
    </form>
  );
}
