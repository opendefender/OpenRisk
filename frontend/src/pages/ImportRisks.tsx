// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// CSV import of the risk register (#755).
//
// The server validates every row and writes all of them or none. This page
// reports exactly what the server answered: how many risks were created, or,
// when the file was refused, the line, column and reason of every error. It
// never says "imported" unless the server created at least one risk.

import { useRef, useState } from 'react';
import { Link } from 'react-router';
import axios from 'axios';
import { AlertCircle, ArrowLeft, CheckCircle2, Download, FileText, Upload, X } from 'lucide-react';

import { Button, cn } from '../shared/ds';
import { api } from '../lib/api';
import { useToast } from '../hooks/useToast';
import { useUIStore } from '../store/uiStore';
import { useRiskStore } from '../hooks/useRiskStore';
import {
  IMPORT_TEMPLATE,
  importFileSchema,
  importLimitSchema,
  importRejectedSchema,
  importSuccessSchema,
  importErrorMessage,
  type ImportRowError,
} from '../features/risks/importRisksSchema';

type Outcome =
  | { kind: 'created'; created: number }
  | { kind: 'rejected'; rejected: number; errors: ImportRowError[] }
  | { kind: 'limit'; requested?: number; remaining?: number }
  | { kind: 'failed'; message: string };

export const ImportRisksPage = () => {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { success } = useToast();
  const { fetchRisks } = useRiskStore();

  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [dragging, setDragging] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  const pick = (f: File | undefined) => {
    setOutcome(null);
    if (!f) return;
    const parsed = importFileSchema(tr).safeParse(f);
    if (!parsed.success) {
      setFile(null);
      setFileError(parsed.error.issues[0]?.message ?? tr('Fichier invalide.', 'Invalid file.'));
      return;
    }
    setFileError(null);
    setFile(f);
  };

  const reset = () => {
    setFile(null);
    setFileError(null);
    setOutcome(null);
    if (inputRef.current) inputRef.current.value = '';
  };

  const submit = async () => {
    if (!file) return;
    setSubmitting(true);
    setOutcome(null);
    try {
      const fd = new FormData();
      fd.append('file', file);
      const res = await api.post<unknown>('/risks/import', fd, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });
      const parsed = importSuccessSchema.safeParse(res.data);
      if (!parsed.success) {
        setOutcome({
          kind: 'failed',
          message: tr('Réponse inattendue du serveur.', 'Unexpected response from the server.'),
        });
        return;
      }
      // The server never answers 200 with zero rows, but if it ever did this
      // page must not dress it up as a success.
      if (parsed.data.created === 0) {
        setOutcome({
          kind: 'failed',
          message: tr('Le fichier ne contenait aucun risque.', 'The file contained no risks.'),
        });
        return;
      }
      setOutcome({ kind: 'created', created: parsed.data.created });
      success(
        tr(
          `${parsed.data.created} risque(s) importé(s)`,
          `${parsed.data.created} risk(s) imported`,
        ),
      );
      void fetchRisks();
    } catch (err) {
      setOutcome(toOutcome(err, tr));
    } finally {
      setSubmitting(false);
    }
  };

  const downloadTemplate = () => {
    const blob = new Blob([IMPORT_TEMPLATE + '\n'], { type: 'text/csv;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'openrisk-risks-template.csv';
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="max-w-4xl mx-auto p-6">
      <div className="mb-6">
        <Link
          to="/risks"
          className="inline-flex items-center gap-1.5 text-sm font-medium text-fg-secondary hover:text-fg-primary transition-colors mb-3"
        >
          <ArrowLeft size={15} /> {tr('Registre des risques', 'Risk register')}
        </Link>
        <h1 className="text-2xl font-semibold text-fg-primary mb-1">
          {tr('Importer des risques', 'Import risks')}
        </h1>
        <p className="text-sm text-fg-secondary">
          {tr(
            'Un fichier CSV, une ligne par risque. Toutes les lignes sont vérifiées avant l’import : si une seule est invalide, rien n’est importé et chaque erreur vous est indiquée.',
            'A CSV file, one row per risk. Every row is checked before anything is imported: if a single row is invalid, nothing is imported and every error is listed.',
          )}
        </p>
      </div>

      {/* Format reference */}
      <section
        aria-labelledby="import-format"
        className="mb-6 rounded-lg border border-border-subtle bg-surface-1 p-4 text-sm"
      >
        <div className="flex flex-wrap items-start justify-between gap-3">
          <h2 id="import-format" className="font-medium text-fg-primary">
            {tr('Format attendu', 'Expected format')}
          </h2>
          <Button variant="ghost" size="sm" onClick={downloadTemplate} className="gap-1.5">
            <Download size={14} />
            {tr('Télécharger le modèle', 'Download the template')}
          </Button>
        </div>
        <dl className="mt-3 grid grid-cols-1 gap-x-6 gap-y-2 sm:grid-cols-[max-content_1fr] text-fg-secondary">
          <dt className="font-mono text-fg-primary">title</dt>
          <dd>{tr('Obligatoire, 255 caractères au plus.', 'Required, at most 255 characters.')}</dd>
          <dt className="font-mono text-fg-primary">probability</dt>
          <dd>
            {tr('Obligatoire, entre 0 et 1 (ex. 0.6).', 'Required, between 0 and 1 (e.g. 0.6).')}
          </dd>
          <dt className="font-mono text-fg-primary">impact</dt>
          <dd>
            {tr('Obligatoire, entre 0 et 10 (ex. 8).', 'Required, between 0 and 10 (e.g. 8).')}
          </dd>
          <dt className="font-mono text-fg-primary">description, tags, frameworks</dt>
          <dd>
            {tr(
              'Facultatifs. Plusieurs valeurs séparées par « ; ».',
              'Optional. Separate several values with ";".',
            )}
          </dd>
          <dt className="font-mono text-fg-primary">assets</dt>
          <dd>
            {tr(
              'Facultatif. Noms ou identifiants d’actifs de votre inventaire, séparés par « ; ». Leur criticité entre dans le score ; un nom inconnu ou porté par plusieurs actifs fait refuser la ligne.',
              'Optional. Names or ids of assets in your inventory, separated by ";". Their criticality is part of the score; a name that is unknown or shared by several assets refuses the row.',
            )}
          </dd>
        </dl>
        <p className="mt-3 text-xs text-fg-tertiary">
          {tr(
            'Les exports Excel en « ; » avec virgule décimale sont acceptés. Les fichiers sur l’ancienne échelle 1–5 sont refusés : convertissez-les (probabilité 3/5 → 0,6 ; impact 4/5 → 8).',
            'Excel exports using ";" and a decimal comma are accepted. Files on the old 1–5 scale are refused: convert them (probability 3/5 → 0.6, impact 4/5 → 8).',
          )}
        </p>
      </section>

      {/* File picker */}
      <div
        role="button"
        tabIndex={0}
        aria-describedby={fileError ? 'import-file-error' : undefined}
        onClick={() => inputRef.current?.click()}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            inputRef.current?.click();
          }
        }}
        onDragOver={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => {
          e.preventDefault();
          setDragging(false);
          pick(e.dataTransfer.files[0]);
        }}
        className={cn(
          'rounded-xl border-2 border-dashed p-10 text-center cursor-pointer transition-colors',
          'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent',
          dragging
            ? 'border-accent-line bg-accent-soft'
            : 'border-border-default hover:bg-surface-1',
        )}
      >
        <Upload className="mx-auto mb-3 text-fg-tertiary" size={32} aria-hidden />
        <p className="font-medium text-fg-primary">
          {tr(
            'Glissez un fichier CSV ici ou cliquez pour le choisir',
            'Drop a CSV file here or click to choose one',
          )}
        </p>
        <p className="mt-1 text-xs text-fg-tertiary">
          {tr('CSV, 2 Mo et 1000 lignes au plus', 'CSV, up to 2 MB and 1000 rows')}
        </p>
        <input
          ref={inputRef}
          type="file"
          accept=".csv,text/csv"
          className="hidden"
          onChange={(e) => pick(e.target.files?.[0])}
          data-testid="import-file-input"
        />
      </div>
      {fileError && (
        <p id="import-file-error" role="alert" className="mt-2 text-sm text-danger-text">
          {fileError}
        </p>
      )}

      {file && (
        <div className="mt-4 flex items-center justify-between gap-3 rounded-lg border border-border-subtle bg-surface-1 px-4 py-3">
          <div className="flex min-w-0 items-center gap-3">
            <FileText size={20} className="shrink-0 text-fg-tertiary" aria-hidden />
            <div className="min-w-0">
              <p className="truncate text-sm font-medium text-fg-primary">{file.name}</p>
              <p className="text-xs text-fg-tertiary">{formatSize(file.size)}</p>
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={reset}
              disabled={submitting}
              aria-label={tr('Retirer le fichier', 'Remove the file')}
            >
              <X size={16} />
            </Button>
            <Button variant="primary" onClick={submit} loading={submitting} className="gap-1.5">
              <Upload size={15} />
              {tr('Importer', 'Import')}
            </Button>
          </div>
        </div>
      )}

      <div aria-live="polite">{outcome && <OutcomePanel outcome={outcome} tr={tr} />}</div>
    </div>
  );
};

function OutcomePanel({
  outcome,
  tr,
}: {
  outcome: Outcome;
  tr: (fr: string, en: string) => string;
}) {
  if (outcome.kind === 'created') {
    return (
      <div className="mt-6 flex items-start gap-3 rounded-lg border border-success/50 bg-success/10 p-4">
        <CheckCircle2 size={22} className="mt-0.5 shrink-0 text-success-text" aria-hidden />
        <div>
          <p className="font-medium text-fg-primary">
            {tr(`${outcome.created} risque(s) importé(s).`, `${outcome.created} risk(s) imported.`)}
          </p>
          <Link to="/risks" className="text-sm text-accent hover:underline">
            {tr('Voir le registre', 'Open the register')}
          </Link>
        </div>
      </div>
    );
  }

  const title =
    outcome.kind === 'rejected'
      ? outcome.rejected > 0
        ? tr(
            `Rien n’a été importé : ${outcome.errors.length} erreur(s) sur ${outcome.rejected} ligne(s).`,
            `Nothing was imported: ${outcome.errors.length} error(s) on ${outcome.rejected} row(s).`,
          )
        : // The file as a whole was refused (header, scale, size): no row to count.
          tr(
            `Rien n’a été importé : le fichier a été refusé.`,
            `Nothing was imported: the file was refused.`,
          )
      : outcome.kind === 'limit'
        ? tr(
            'Rien n’a été importé : ce fichier dépasse la limite de risques de votre plan.',
            'Nothing was imported: this file exceeds your plan’s risk limit.',
          )
        : tr('Rien n’a été importé.', 'Nothing was imported.');

  return (
    <div
      role="alert"
      className="mt-6 rounded-lg border border-danger/50 bg-danger/10 p-4"
      data-testid="import-outcome-error"
    >
      <div className="flex items-start gap-3">
        <AlertCircle size={22} className="mt-0.5 shrink-0 text-danger-text" aria-hidden />
        <div className="min-w-0 flex-1">
          <p className="font-medium text-fg-primary">{title}</p>
          {outcome.kind === 'limit' && (
            <p className="mt-1 text-sm text-fg-secondary">
              {outcome.requested !== undefined && outcome.remaining !== undefined
                ? tr(
                    `Le fichier contient ${outcome.requested} risque(s) ; votre plan en permet encore ${outcome.remaining}. `,
                    `The file has ${outcome.requested} risk(s); your plan allows ${outcome.remaining} more. `,
                  )
                : ''}
              <Link to="/settings?tab=billing" className="text-accent hover:underline">
                {tr('Voir les plans', 'See plans')}
              </Link>
            </p>
          )}
          {outcome.kind === 'failed' && (
            <p className="mt-1 text-sm text-fg-secondary">{outcome.message}</p>
          )}
          {outcome.kind === 'rejected' && (
            <>
              <p className="mt-1 text-sm text-fg-secondary">
                {tr(
                  'Corrigez ces lignes dans votre fichier puis importez-le à nouveau.',
                  'Fix these rows in your file, then import it again.',
                )}
              </p>
              <div className="mt-3 max-h-80 overflow-auto rounded-md border border-border-subtle bg-surface-0">
                <table className="w-full text-sm">
                  <thead className="sticky top-0 bg-surface-1 text-left text-xs text-fg-secondary">
                    <tr>
                      <th scope="col" className="px-3 py-2 font-medium">
                        {tr('Ligne', 'Line')}
                      </th>
                      <th scope="col" className="px-3 py-2 font-medium">
                        {tr('Colonne', 'Column')}
                      </th>
                      <th scope="col" className="px-3 py-2 font-medium">
                        {tr('Problème', 'Problem')}
                      </th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border-subtle">
                    {outcome.errors.map((e, i) => (
                      <tr key={`${e.line}-${e.column ?? ''}-${i}`}>
                        <td className="px-3 py-2 tabular-nums text-fg-primary">
                          {e.line > 0 ? e.line : tr('Fichier', 'File')}
                        </td>
                        <td className="px-3 py-2 font-mono text-fg-secondary">{e.column ?? '—'}</td>
                        <td className="px-3 py-2 text-fg-secondary">{importErrorMessage(e, tr)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function toOutcome(err: unknown, tr: (fr: string, en: string) => string): Outcome {
  if (axios.isAxiosError(err) && err.response) {
    const { status, data } = err.response;
    if (status === 422) {
      const parsed = importRejectedSchema.safeParse(data);
      if (parsed.success) {
        return { kind: 'rejected', rejected: parsed.data.rejected, errors: parsed.data.errors };
      }
    }
    if (status === 402) {
      const parsed = importLimitSchema.safeParse(data);
      if (parsed.success) {
        return {
          kind: 'limit',
          requested: parsed.data.requested,
          remaining: parsed.data.remaining,
        };
      }
    }
    if (status === 403) {
      return {
        kind: 'failed',
        message: tr(
          'Vous n’avez pas le droit de créer des risques.',
          'You are not allowed to create risks.',
        ),
      };
    }
    const message =
      typeof data === 'object' &&
      data !== null &&
      'message' in data &&
      typeof data.message === 'string'
        ? data.message
        : null;
    if (message) return { kind: 'failed', message };
  }
  return {
    kind: 'failed',
    message: tr(
      'Le serveur n’a pas pu traiter le fichier. Réessayez dans un instant.',
      'The server could not process the file. Try again in a moment.',
    ),
  };
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export default ImportRisksPage;
