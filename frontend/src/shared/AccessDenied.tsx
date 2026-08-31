// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Access denied — names the missing permission and who can grant it.
//
// "Accès restreint" on its own is a dead end twice over: the user cannot act on
// it, and neither can the admin they forward it to, because nothing on screen
// says which grant is missing. This page names the permission verbatim (the same
// string the backend guard checks, so it can be pasted straight into the role
// editor), says who to ask, and always offers a way out — the parent page if the
// route has one, the dashboard otherwise.

import { Link, useNavigate } from 'react-router';
import { Lock, ArrowLeft, Copy } from 'lucide-react';
import { toast } from 'sonner';
import { useUIStore } from '../store/uiStore';
import { parentHref } from './routeModel';
import { Btn } from './ui';

interface AccessDeniedProps {
  /** The permission string the route requires, e.g. "compliance:audits:read". */
  permission: string;
  /** The path that was refused, used to offer its parent as a way out. */
  pathname: string;
}

export function AccessDenied({ permission, pathname }: AccessDeniedProps) {
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const back = parentHref(pathname) ?? '/';

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(permission);
      toast.success(tr('Permission copiée', 'Permission copied'));
    } catch {
      toast.error(tr('Copie impossible', 'Could not copy'));
    }
  };

  return (
    <div
      data-testid="access-denied"
      className="flex-1 flex items-center justify-center p-8"
      style={{ background: 'var(--bg-primary)' }}
    >
      <div className="max-w-md w-full text-center">
        <div
          className="w-16 h-16 rounded-2xl flex items-center justify-center mb-5 mx-auto"
          style={{ background: 'var(--bg-hover)', color: 'var(--fg-muted)' }}
        >
          <Lock size={28} strokeWidth={1.7} />
        </div>

        <h1 className="text-md font-semibold text-ink mb-2">
          {tr('Vous n’avez pas accès à cette page', 'You do not have access to this page')}
        </h1>

        <p className="text-sm text-ink-soft leading-relaxed mb-4">
          {tr(
            'Elle demande une permission qui n’est pas attribuée à votre rôle.',
            'It requires a permission that is not granted to your role.',
          )}
        </p>

        {/* The exact string the backend guard checks — pasteable into the role
            editor by whoever grants it. */}
        <div
          className="flex items-center justify-center gap-2 rounded-md px-3 py-2.5 mb-4"
          style={{ background: 'var(--bg-hover)', border: '1px solid var(--border)' }}
        >
          <span className="text-2xs uppercase tracking-wide text-ink-muted shrink-0">
            {tr('Permission requise', 'Required permission')}
          </span>
          <code data-testid="missing-permission" className="mono text-xs font-semibold text-ink truncate">
            {permission}
          </code>
          <button
            onClick={copy}
            aria-label={tr('Copier la permission', 'Copy permission')}
            className="shrink-0 text-ink-muted hover:text-ink transition-colors"
          >
            <Copy size={14} />
          </button>
        </div>

        <p className="text-xs text-ink-muted leading-relaxed mb-6">
          {tr(
            'Demandez-la à un administrateur de votre organisation : il peut vous l’attribuer depuis Paramètres › Membres.',
            'Ask an administrator of your organisation: they can grant it from Settings › Members.',
          )}
        </p>

        <div className="flex items-center justify-center gap-2.5 flex-wrap">
          <Btn label={tr('Retour', 'Go back')} icon={ArrowLeft} onClick={() => navigate(back)} />
          <Link
            to="/settings/members"
            className="h-9 px-3.5 rounded-md text-sm font-semibold inline-flex items-center gap-[7px] transition-colors text-ink hover:bg-hover"
            style={{ border: '1px solid var(--border-strong)' }}
          >
            {tr('Voir les administrateurs', 'View administrators')}
          </Link>
        </div>
      </div>
    </div>
  );
}
