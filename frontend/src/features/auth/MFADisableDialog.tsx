// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #754 — turning MFA off from Settings › Security.
//
// An open session is not proof of who is at the keyboard, so the server asks
// for the password again. The dialog says what is lost (the authenticator AND
// the backup codes), and a wrong password is reported on the field itself.

import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { isAxiosError } from 'axios';
import { z } from 'zod';
import { ShieldOff } from 'lucide-react';
import { toast } from 'sonner';
import { useState } from 'react';

import { Button, Field, Input, Modal } from '../../shared/ds';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useDisableMFA } from './useMfa';
import type { DisableMFAErrorBody } from './authService';

type Tr = (fr: string, en: string) => string;

function schema(tr: Tr) {
  return z.object({
    password: z.string().min(1, tr('Saisissez votre mot de passe.', 'Enter your password.')),
  });
}

type Values = z.infer<ReturnType<typeof schema>>;

export function MFADisableDialog({ onClose }: { onClose: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr: Tr = (fr, en) => (lang === 'fr' ? fr : en);
  const email = useAuthStore((s) => s.user?.email ?? '');
  const disable = useDisableMFA();
  // A refusal the password field cannot fix (role, identity provider, throttle).
  const [blocked, setBlocked] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors },
  } = useForm<Values>({
    resolver: zodResolver(schema(tr)),
    defaultValues: { password: '' },
  });

  const onSubmit = (v: Values) => {
    setBlocked(null);
    disable.mutate(
      { password: v.password, locale: lang },
      {
        onSuccess: (res) => {
          toast.success(res.message);
          onClose();
        },
        onError: (err) => {
          const body = (isAxiosError(err) ? err.response?.data : undefined) as
            DisableMFAErrorBody | undefined;
          switch (body?.code) {
            case 'wrong_password':
              setError('password', { message: body.error });
              return;
            case 'mfa_required_by_role':
            case 'no_local_password':
            case 'too_many_attempts':
              setBlocked(body.error ?? '');
              return;
            case 'not_enrolled':
              // Already off (another tab, another device): nothing left to do.
              toast.success(body.error ?? '');
              onClose();
              return;
            default:
              toast.error(
                body?.error ||
                  tr(
                    'La désactivation a échoué. Réessayez dans un instant.',
                    'Could not turn it off. Try again in a moment.',
                  ),
              );
          }
        },
      },
    );
  };

  const busy = disable.isPending;

  return (
    <Modal
      open
      onClose={onClose}
      size="sm"
      dismissable={!busy}
      closeLabel={tr('Fermer', 'Close')}
      title={tr('Désactiver le MFA', 'Turn off MFA')}
      subtitle={tr(
        'Confirmez votre mot de passe pour continuer.',
        'Confirm your password to continue.',
      )}
      leading={
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-danger-surface text-danger-text">
          <ShieldOff size={18} aria-hidden="true" />
        </span>
      }
      footer={
        <>
          <Button variant="ghost" onClick={onClose} disabled={busy}>
            {tr('Annuler', 'Cancel')}
          </Button>
          <Button
            variant="destructive"
            type="submit"
            form="mfa-disable-form"
            loading={busy}
            disabled={blocked !== null}
            data-testid="mfa-disable-submit"
          >
            {tr('Désactiver', 'Turn off')}
          </Button>
        </>
      }
    >
      <form
        id="mfa-disable-form"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="space-y-3"
      >
        <p className="text-sm leading-relaxed text-fg-secondary">
          {tr(
            "Votre application d'authentification et vos codes de secours cesseront de fonctionner. Le mot de passe seul suffira pour ouvrir votre compte. Un e-mail vous confirmera le changement.",
            'Your authenticator app and your backup codes will stop working. A password alone will open your account. You will get an email confirming the change.',
          )}
        </p>
        {/* Tells password managers which account this password belongs to. */}
        <input type="text" name="username" autoComplete="username" value={email} readOnly hidden />
        <Field
          label={tr('Mot de passe actuel', 'Current password')}
          required
          message={errors.password?.message}
          status={errors.password ? 'invalid' : 'default'}
        >
          <Input
            {...register('password')}
            type="password"
            autoComplete="current-password"
            autoFocus
            data-testid="mfa-disable-password"
          />
        </Field>
        {blocked !== null && (
          <p
            role="alert"
            className="rounded-md bg-surface-3 p-3 text-sm text-fg-secondary"
            data-testid="mfa-disable-blocked"
          >
            {blocked}
          </p>
        )}
      </form>
    </Modal>
  );
}
