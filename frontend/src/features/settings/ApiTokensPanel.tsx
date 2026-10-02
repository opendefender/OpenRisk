// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Settings › API tokens (#782). A token created here is a personal access token:
// stored hashed in PostgreSQL, bound to the current organization, and accepted
// on every /api/v1 route as `Authorization: Bearer orsk_…`.
//
// The secret exists in the browser exactly once, in the reveal panel below. The
// screen says "copied" only when the clipboard write actually resolved, and the
// panel stays until the user dismisses it, so a refused clipboard never leaves
// them with a token they cannot see.

import { useMemo, useState } from 'react';
import { toast } from 'sonner';
import { z } from 'zod';
import { Check, Copy, KeyRound, Plus, Trash2 } from 'lucide-react';

import { Btn, EmptyState } from '../../shared/ui';
import { DataTable, useTableState, type Column, type RowAction } from '../../shared/datatable';
import { DangerConfirm } from '../../shared/DangerConfirm';
import { relTime } from '../risks/riskMap';
import type { LocaleCode } from '../../i18n/locales';
import { useTokens, type ApiToken } from './adminData';

type Tr = (fr: string, en: string) => string;

const TOKEN_NAME_MAX = 100;
const tokenNameSchema = z.string().trim().max(TOKEN_NAME_MAX);

async function copyText(value: string): Promise<boolean> {
  if (!navigator.clipboard) return false;
  try {
    await navigator.clipboard.writeText(value);
    return true;
  } catch {
    return false;
  }
}

// The token prefix cell used to render a Copy glyph that copied nothing. Module
// scope keeps the handler stable so the columns memo survives.
function copyPrefix(prefix: string, tr: Tr) {
  void copyText(prefix).then((ok) =>
    ok
      ? toast.success(tr('Préfixe copié', 'Prefix copied'))
      : toast.error(tr('Copie impossible', 'Could not copy')),
  );
}

export function ApiTokensPanel({ tr, lang }: { tr: Tr; lang: LocaleCode }) {
  const { tokens, isLoading, isError, refetch, create, revoke } = useTokens();
  const [name, setName] = useState('');
  const [nameError, setNameError] = useState<string | null>(null);
  const [revealed, setRevealed] = useState<null | { name: string; value: string }>(null);
  // Revoking a token breaks any integration using it → impact-radiography confirm.
  const [revokingToken, setRevokingToken] = useState<null | {
    id: string;
    name: string;
    lastUsed?: string | null;
  }>(null);
  const table = useTableState({
    defaultSort: { key: 'created', dir: 'desc' },
    defaultPageSize: 25,
    urlPrefix: 'tok_',
  });

  const doCreate = () => {
    const parsed = tokenNameSchema.safeParse(name);
    if (!parsed.success) {
      setNameError(
        tr(
          `Le nom ne doit pas dépasser ${TOKEN_NAME_MAX} caractères.`,
          `The name must be ${TOKEN_NAME_MAX} characters or fewer.`,
        ),
      );
      return;
    }
    setNameError(null);
    const n = parsed.data || tr('Nouveau jeton', 'New token');
    create.mutate(n, {
      onSuccess: async (created) => {
        setName('');
        const secret = typeof created?.token === 'string' ? created.token : '';
        if (!secret) {
          // A 2xx without a secret is not a usable token; never call it a success.
          toast.error(
            tr(
              'Le serveur n’a renvoyé aucun jeton. Réessayez.',
              'The server returned no token. Try again.',
            ),
          );
          return;
        }
        setRevealed({ name: created.name, value: secret });
        const copied = await copyText(secret);
        if (copied)
          toast.success(
            tr(
              'Jeton créé et copié dans le presse-papiers',
              'Token created and copied to clipboard',
            ),
          );
        else
          toast.success(tr('Jeton créé — copiez-le ci-dessous', 'Token created — copy it below'));
      },
      onError: () => toast.error(tr('Création échouée', 'Creation failed')),
    });
  };

  const copyRevealed = () => {
    if (!revealed) return;
    void copyText(revealed.value).then((ok) =>
      ok
        ? toast.success(tr('Jeton copié', 'Token copied'))
        : toast.error(
            tr(
              'Copie impossible — sélectionnez le jeton et copiez-le à la main.',
              'Could not copy — select the token and copy it by hand.',
            ),
          ),
    );
  };

  const columns: Column<ApiToken>[] = useMemo(
    () => [
      {
        key: 'name',
        header: tr('Nom', 'Name'),
        frozen: true,
        hideable: false,
        sortValue: (t) => t.name.toLowerCase(),
        exportValue: (t) => t.name,
        render: (t) => <span className="text-[13.5px] font-medium text-ink">{t.name}</span>,
      },
      {
        key: 'prefix',
        header: tr('Préfixe', 'Prefix'),
        exportValue: (t) => t.token_prefix ?? '',
        render: (t) =>
          t.token_prefix ? (
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                copyPrefix(t.token_prefix, tr);
              }}
              className="mono text-[12px] text-ink-soft inline-flex items-center gap-1.5 rounded px-1 -mx-1 hover:bg-hover"
              title={tr('Copier le préfixe', 'Copy prefix')}
            >
              {t.token_prefix}… <Copy size={12} className="text-ink-muted" />
            </button>
          ) : (
            <span className="text-ink-muted text-[12px]">—</span>
          ),
      },
      {
        key: 'created',
        header: tr('Créé', 'Created'),
        sortValue: (t) => new Date(t.created_at ?? 0).getTime(),
        exportValue: (t) => t.created_at ?? '',
        render: (t) => (
          <span className="text-[12px] text-ink-soft">{relTime(t.created_at, lang)}</span>
        ),
      },
      {
        key: 'used',
        header: tr('Dernière util.', 'Last used'),
        sortValue: (t) => new Date(t.last_used_at ?? 0).getTime(),
        exportValue: (t) => t.last_used_at ?? '',
        render: (t) => (
          <span className="text-[12px] text-ink-soft">
            {t.last_used_at ? relTime(t.last_used_at, lang) : tr('jamais', 'never')}
          </span>
        ),
      },
    ],
    [tr, lang],
  );

  const rowActions: RowAction<ApiToken>[] = useMemo(
    () => [
      {
        key: 'revoke',
        label: tr('Révoquer', 'Revoke'),
        icon: Trash2,
        danger: true,
        onSelect: (t) => setRevokingToken({ id: t.id, name: t.name, lastUsed: t.last_used_at }),
      },
    ],
    [tr],
  );

  return (
    <>
      <div className="mb-4">
        <div className="flex items-center gap-2.5 flex-wrap">
          <input
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              if (nameError) setNameError(null);
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !create.isPending) doCreate();
            }}
            placeholder={tr('Nom du jeton (ex. CI/CD)', 'Token name (e.g. CI/CD)')}
            aria-label={tr('Nom du jeton', 'Token name')}
            aria-invalid={nameError ? true : undefined}
            aria-describedby={nameError ? 'api-token-name-error' : undefined}
            className="flex-1 min-w-[200px] h-9 px-3.5 rounded-[10px] text-[13px] text-ink outline-none"
            style={{
              border: `1px solid ${nameError ? 'var(--critical)' : 'var(--border-strong)'}`,
              background: 'var(--bg-elevated)',
            }}
          />
          <Btn
            label={tr('Générer un jeton', 'Generate token')}
            icon={Plus}
            primary
            onClick={doCreate}
            disabled={create.isPending}
          />
        </div>
        {nameError && (
          <p
            id="api-token-name-error"
            role="alert"
            className="mt-1.5 text-[12px]"
            style={{ color: 'var(--critical)' }}
          >
            {nameError}
          </p>
        )}
      </div>

      {revealed && (
        <section
          aria-labelledby="api-token-reveal-title"
          className="mb-4 rounded-[12px] p-4"
          style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
        >
          <h3 id="api-token-reveal-title" className="text-[13.5px] font-semibold text-ink">
            {tr(
              `Copiez le jeton « ${revealed.name} » maintenant`,
              `Copy the “${revealed.name}” token now`,
            )}
          </h3>
          <p className="mt-1 text-[12.5px] text-ink-soft">
            {tr(
              'Il ne sera plus jamais affiché. Envoyez-le dans l’en-tête Authorization: Bearer <jeton>.',
              'It will never be shown again. Send it in the Authorization: Bearer <token> header.',
            )}
          </p>
          <div className="mt-3 flex items-center gap-2 flex-wrap">
            <input
              readOnly
              value={revealed.value}
              aria-label={tr('Valeur du jeton', 'Token value')}
              onFocus={(e) => e.currentTarget.select()}
              className="mono flex-1 min-w-[240px] h-9 px-3 rounded-[10px] text-[12px] text-ink outline-none"
              style={{ border: '1px solid var(--border-strong)', background: 'var(--bg)' }}
            />
            <Btn label={tr('Copier', 'Copy')} icon={Copy} onClick={copyRevealed} />
            <Btn
              label={tr('J’ai copié le jeton', 'I have copied the token')}
              icon={Check}
              onClick={() => setRevealed(null)}
            />
          </div>
        </section>
      )}

      <DataTable
        id="api-tokens"
        ariaLabel={tr('Jetons API', 'API tokens')}
        rows={tokens}
        columns={columns}
        rowKey={(t) => t.id}
        api={table}
        mode="client"
        loading={isLoading}
        error={isError}
        onRetry={() => void refetch()}
        clientSearch={(t, q) => `${t.name} ${t.token_prefix ?? ''}`.toLowerCase().includes(q)}
        searchPlaceholder={tr('Nom ou préfixe…', 'Name or prefix…')}
        rowActions={rowActions}
        exportFilename="jetons-api"
        minWidth={620}
        pageSizeOptions={[10, 25, 50]}
        empty={
          <EmptyState
            icon={KeyRound}
            title={tr('Aucun jeton API', 'No API tokens')}
            description={tr(
              'Créez un jeton pour authentifier vos intégrations et scripts.',
              'Create a token to authenticate your integrations and scripts.',
            )}
          />
        }
      />

      <DangerConfirm
        open={!!revokingToken}
        onClose={() => setRevokingToken(null)}
        title={tr('Révoquer le jeton API', 'Revoke API token')}
        subject={revokingToken?.name}
        intro={tr(
          'Toute intégration ou script utilisant ce jeton cessera immédiatement de fonctionner. Cette action est irréversible.',
          'Any integration or script using this token stops working immediately. This action is irreversible.',
        )}
        impact={
          revokingToken
            ? [
                {
                  label: tr('Dernière utilisation', 'Last used'),
                  value: revokingToken.lastUsed
                    ? relTime(revokingToken.lastUsed, lang)
                    : tr('jamais', 'never'),
                },
              ]
            : []
        }
        confirmLabel={tr('Révoquer le jeton', 'Revoke token')}
        onConfirm={() => {
          if (!revokingToken) return;
          const target = revokingToken;
          // Optimistic: the row leaves the table now; useTokens restores it if
          // the server refuses.
          setRevokingToken(null);
          revoke.mutate(target.id, {
            onSuccess: () => toast.success(tr('Jeton révoqué', 'Token revoked')),
            onError: () =>
              toast.error(tr('Révocation échouée — réessayez.', 'Revocation failed — retry.')),
          });
        }}
        busy={revoke.isPending}
      />
    </>
  );
}
