// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Settings › Security — change your password without signing out (#720).
//
// The strength meter is the same component, and defers to the same server
// policy, as registration and reset: a password this card calls acceptable is
// one the API accepts. The current password is the proof of identity, so a
// wrong one is reported on that field and nowhere else.

import { useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { isAxiosError } from 'axios';
import { z } from 'zod';
import { Eye, EyeOff } from 'lucide-react';
import { toast } from 'sonner';
import { useNavigate } from 'react-router';

import { Button, Field, Input } from '../../shared/ds';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import { setAccessToken } from '../../lib/session';
import { PasswordStrength } from './PasswordStrength';
import {
  changePassword,
  type ChangePasswordErrorBody,
  type PasswordAssessment,
} from './authService';

type Tr = (fr: string, en: string) => string;

function schema(tr: Tr) {
  return z
    .object({
      current: z
        .string()
        .min(1, tr('Saisissez votre mot de passe actuel.', 'Enter your current password.')),
      next: z.string().min(12, tr('12 caractères au moins.', 'At least 12 characters.')),
      confirm: z.string(),
    })
    .refine((v) => v.next === v.confirm, {
      path: ['confirm'],
      message: tr(
        'Les deux mots de passe ne correspondent pas.',
        'The two passwords do not match.',
      ),
    })
    .refine((v) => v.next !== v.current, {
      path: ['next'],
      message: tr(
        'Choisissez un mot de passe différent de l’actuel.',
        'Choose a password different from the current one.',
      ),
    });
}

type Values = z.infer<ReturnType<typeof schema>>;

export function ChangePasswordCard() {
  const lang = useUIStore((s) => s.lang);
  const tr: Tr = (fr, en) => (lang === 'fr' ? fr : en);
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);
  const navigate = useNavigate();
  const [visible, setVisible] = useState(false);
  const [acceptable, setAcceptable] = useState(false);
  const [assessment, setAssessment] = useState<PasswordAssessment | null>(null);
  const [managedByIdp, setManagedByIdp] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    control,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<Values>({
    resolver: zodResolver(schema(tr)),
    defaultValues: { current: '', next: '', confirm: '' },
  });
  const next = useWatch({ control, name: 'next' });

  const onSubmit = async (v: Values) => {
    setAssessment(null);
    try {
      const res = await changePassword(v.current, v.next, lang);
      reset();
      toast.success(res.message);
      if (res.token_pair?.access_token) {
        // The server ended every session, this one included, and minted a new
        // one for this device; the cookies are already replaced.
        setAccessToken(res.token_pair.access_token);
      } else if (res.reauthenticate) {
        logout();
        navigate('/login', { replace: true });
      }
    } catch (err) {
      const body = (isAxiosError(err) ? err.response?.data : undefined) as
        ChangePasswordErrorBody | undefined;
      switch (body?.code) {
        case 'wrong_current_password':
          setError('current', { message: body.error });
          return;
        case 'weak_password':
          setAssessment(body.assessment ?? null);
          setError('next', { message: body.error });
          return;
        case 'same_password':
          setError('next', { message: body.error });
          return;
        case 'no_local_password':
          setManagedByIdp(body.error ?? '');
          return;
        default:
          if (isAxiosError(err) && err.response?.status === 429) {
            toast.error(
              tr(
                'Trop de tentatives. Réessayez dans quelques minutes.',
                'Too many attempts. Try again in a few minutes.',
              ),
            );
            return;
          }
          toast.error(
            body?.error || tr('Modification impossible', 'Could not change the password'),
          );
      }
    }
  };

  const type = visible ? 'text' : 'password';
  const toggle = (
    <button
      type="button"
      onClick={() => setVisible((x) => !x)}
      className="text-ink-muted hover:text-ink"
      aria-label={
        visible
          ? tr('Masquer les mots de passe', 'Hide passwords')
          : tr('Afficher les mots de passe', 'Show passwords')
      }
      aria-pressed={visible}
    >
      {visible ? <EyeOff size={15} /> : <Eye size={15} />}
    </button>
  );

  return (
    <section aria-labelledby="change-password-title" data-testid="change-password-card">
      <div className="flex items-center justify-between mb-3.5">
        <h3 id="change-password-title" className="text-[14px] font-semibold text-ink">
          {tr('Changer le mot de passe', 'Change password')}
        </h3>
        {!managedByIdp && toggle}
      </div>

      {managedByIdp !== null ? (
        <p
          className="text-[12.5px] text-ink-soft leading-relaxed"
          data-testid="password-managed-by-idp"
        >
          {managedByIdp ||
            tr(
              'Ce compte se connecte via votre fournisseur d’identité : son mot de passe se change chez lui.',
              'This account signs in through your identity provider: its password is changed there.',
            )}
        </p>
      ) : (
        <form onSubmit={handleSubmit(onSubmit)} noValidate className="grid gap-4 max-w-[440px]">
          <Field
            label={tr('Mot de passe actuel', 'Current password')}
            required
            message={errors.current?.message}
            status={errors.current ? 'invalid' : 'default'}
          >
            <Input {...register('current')} type={type} autoComplete="current-password" />
          </Field>
          <Field
            label={tr('Nouveau mot de passe', 'New password')}
            required
            message={errors.next?.message}
            status={errors.next ? 'invalid' : 'default'}
          >
            <Input {...register('next')} type={type} autoComplete="new-password" />
          </Field>
          {next && (
            <PasswordStrength
              password={next}
              email={user?.email}
              name={user?.full_name}
              onVerdict={setAcceptable}
              override={assessment}
            />
          )}
          <Field
            label={tr('Confirmer le nouveau mot de passe', 'Confirm the new password')}
            required
            message={errors.confirm?.message}
            status={errors.confirm ? 'invalid' : 'default'}
          >
            <Input {...register('confirm')} type={type} autoComplete="new-password" />
          </Field>
          <p className="text-[11.5px] text-ink-muted">
            {tr(
              'Vos autres appareils seront déconnectés. Celui-ci reste connecté.',
              'Your other devices will be signed out. This one stays signed in.',
            )}
          </p>
          <div>
            <Button
              type="submit"
              variant="primary"
              disabled={isSubmitting || (next.length > 0 && !acceptable)}
              data-testid="change-password-submit"
            >
              {isSubmitting
                ? tr('Modification…', 'Changing…')
                : tr('Changer le mot de passe', 'Change password')}
            </Button>
          </div>
        </form>
      )}
    </section>
  );
}
