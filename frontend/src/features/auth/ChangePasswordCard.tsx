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
import { useI18n } from '../../hooks/useI18n';
import { useAuthStore } from '../../hooks/useAuthStore';
import { setAccessToken } from '../../lib/session';
import { PasswordStrength } from './PasswordStrength';
import {
  changePassword,
  type ChangePasswordErrorBody,
  type PasswordAssessment,
} from './authService';

type T = (key: string) => string;

function schema(t: T) {
  return z
    .object({
      current: z.string().min(1, t('accountSecurity.currentRequired')),
      next: z.string().min(12, t('accountSecurity.minLength')),
      confirm: z.string(),
    })
    .refine((v) => v.next === v.confirm, {
      path: ['confirm'],
      message: t('accountSecurity.mismatch'),
    })
    .refine((v) => v.next !== v.current, {
      path: ['next'],
      message: t('accountSecurity.sameAsCurrent'),
    });
}

type Values = z.infer<ReturnType<typeof schema>>;

export function ChangePasswordCard() {
  const lang = useUIStore((s) => s.lang);
  const { t } = useI18n();
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
    resolver: zodResolver(schema(t)),
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
            toast.error(t('accountSecurity.tooManyAttempts'));
            return;
          }
          toast.error(body?.error || t('accountSecurity.failed'));
      }
    }
  };

  const type = visible ? 'text' : 'password';
  const toggle = (
    <button
      type="button"
      onClick={() => setVisible((x) => !x)}
      className="text-ink-muted hover:text-ink"
      aria-label={visible ? t('accountSecurity.hidePasswords') : t('accountSecurity.showPasswords')}
      aria-pressed={visible}
    >
      {visible ? <EyeOff size={15} /> : <Eye size={15} />}
    </button>
  );

  return (
    <section aria-labelledby="change-password-title" data-testid="change-password-card">
      <div className="flex items-center justify-between mb-3.5">
        <h3 id="change-password-title" className="text-[14px] font-semibold text-ink">
          {t('accountSecurity.title')}
        </h3>
        {!managedByIdp && toggle}
      </div>

      {managedByIdp !== null ? (
        <p
          className="text-[12.5px] text-ink-soft leading-relaxed"
          data-testid="password-managed-by-idp"
        >
          {managedByIdp || t('accountSecurity.managedByIdp')}
        </p>
      ) : (
        <form onSubmit={handleSubmit(onSubmit)} noValidate className="grid gap-4 max-w-[440px]">
          {/* Tells password managers which account this password belongs to,
              so they update the right entry. Hidden, read-only, never sent. */}
          <input
            type="text"
            name="username"
            autoComplete="username"
            value={user?.email ?? ''}
            readOnly
            hidden
          />
          <Field
            label={t('accountSecurity.current')}
            required
            message={errors.current?.message}
            status={errors.current ? 'invalid' : 'default'}
          >
            <Input {...register('current')} type={type} autoComplete="current-password" />
          </Field>
          <Field
            label={t('accountSecurity.new')}
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
            label={t('accountSecurity.confirm')}
            required
            message={errors.confirm?.message}
            status={errors.confirm ? 'invalid' : 'default'}
          >
            <Input {...register('confirm')} type={type} autoComplete="new-password" />
          </Field>
          <p className="text-[11.5px] text-ink-muted">{t('accountSecurity.note')}</p>
          <div>
            <Button
              type="submit"
              variant="primary"
              disabled={isSubmitting || (next.length > 0 && !acceptable)}
              data-testid="change-password-submit"
            >
              {isSubmitting ? t('accountSecurity.submitting') : t('accountSecurity.title')}
            </Button>
          </div>
        </form>
      )}
    </section>
  );
}
