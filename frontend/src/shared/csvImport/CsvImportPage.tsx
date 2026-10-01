// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// The CSV import page (#755), written once so every import gets the same
// contract. The server validates every row and writes all of them or none. This page reports exactly what the server answered: how many records
// were created, or, when the file was refused, the line, column and reason of
// every error. It never says "imported" unless the server created at least one.

import { useRef, useState } from 'react';
import { Link } from 'react-router';
import axios from 'axios';
import { AlertCircle, ArrowLeft, CheckCircle2, Download, FileText, Upload, X } from 'lucide-react';

import { Button, cn } from '../ds';
import { api } from '../../lib/api';
import { useToast } from '../../hooks/useToast';
import { useI18n } from '../../hooks/useI18n';
import {
  importErrorMessage,
  importFileSchema,
  importLimitSchema,
  importRejectedSchema,
  importSuccessSchema,
  type ImportRowError,
  type T,
} from './csvImportSchema';

/** What differs from one import to the next. Strings are already
 * in the reader's language: the page builds its config with its own t. */
export interface CsvImportConfig {
  endpoint: string;
  back: { to: string; label: string };
  title: string;
  intro: string;
  columns: ReadonlyArray<{ name: string; help: string }>;
  note?: string;
  template: string;
  templateFilename: string;
  /** Where the created records can be seen. */
  open: { to: string; label: string };
  created: (n: number) => string;
  emptyFile: string;
  forbidden: string;
  limitTitle: string;
  limitDetail: (requested: number, remaining: number) => string;
  /** Called after a successful import, e.g. to refresh a store. */
  onCreated?: () => void;
}

type Outcome =
  | { kind: 'created'; created: number }
  | { kind: 'rejected'; rejected: number; errors: ImportRowError[] }
  | { kind: 'limit'; requested?: number; remaining?: number }
  | { kind: 'failed'; message: string };

export const CsvImportPage = ({ config }: { config: CsvImportConfig }) => {
  const { t } = useI18n();
  const { success } = useToast();

  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [dragging, setDragging] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  const pick = (f: File | undefined) => {
    setOutcome(null);
    if (!f) return;
    const parsed = importFileSchema(t).safeParse(f);
    if (!parsed.success) {
      setFile(null);
      setFileError(parsed.error.issues[0]?.message ?? t('csvImport.invalidFile'));
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
      const res = await api.post<unknown>(config.endpoint, fd, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });
      const parsed = importSuccessSchema.safeParse(res.data);
      if (!parsed.success) {
        setOutcome({
          kind: 'failed',
          message: t('csvImport.unexpectedResponse'),
        });
        return;
      }
      // The server never answers 200 with zero rows, but if it ever did this
      // page must not dress it up as a success.
      if (parsed.data.created === 0) {
        setOutcome({ kind: 'failed', message: config.emptyFile });
        return;
      }
      setOutcome({ kind: 'created', created: parsed.data.created });
      success(config.created(parsed.data.created));
      config.onCreated?.();
    } catch (err) {
      setOutcome(toOutcome(err, t, config.forbidden));
    } finally {
      setSubmitting(false);
    }
  };

  const downloadTemplate = () => {
    const blob = new Blob([config.template + '\n'], { type: 'text/csv;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = config.templateFilename;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="max-w-4xl mx-auto p-6">
      <div className="mb-6">
        <Link
          to={config.back.to}
          className="inline-flex items-center gap-1.5 text-sm font-medium text-fg-secondary hover:text-fg-primary transition-colors mb-3"
        >
          <ArrowLeft size={15} /> {config.back.label}
        </Link>
        <h1 className="text-2xl font-semibold text-fg-primary mb-1">{config.title}</h1>
        <p className="text-sm text-fg-secondary">{config.intro}</p>
      </div>

      {/* Format reference */}
      <section
        aria-labelledby="import-format"
        className="mb-6 rounded-lg border border-border-subtle bg-surface-1 p-4 text-sm"
      >
        <div className="flex flex-wrap items-start justify-between gap-3">
          <h2 id="import-format" className="font-medium text-fg-primary">
            {t('csvImport.expectedFormat')}
          </h2>
          <Button variant="ghost" size="sm" onClick={downloadTemplate} className="gap-1.5">
            <Download size={14} />
            {t('csvImport.downloadTemplate')}
          </Button>
        </div>
        <dl className="mt-3 grid grid-cols-1 gap-x-6 gap-y-2 sm:grid-cols-[max-content_1fr] text-fg-secondary">
          {config.columns.map((c) => (
            <div key={c.name} className="contents">
              <dt className="font-mono text-fg-primary">{c.name}</dt>
              <dd>{c.help}</dd>
            </div>
          ))}
        </dl>
        {config.note && <p className="mt-3 text-xs text-fg-tertiary">{config.note}</p>}
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
        <p className="font-medium text-fg-primary">{t('csvImport.dropHere')}</p>
        <p className="mt-1 text-xs text-fg-tertiary">{t('csvImport.limits')}</p>
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
              aria-label={t('csvImport.removeFile')}
            >
              <X size={16} />
            </Button>
            <Button variant="primary" onClick={submit} loading={submitting} className="gap-1.5">
              <Upload size={15} />
              {t('csvImport.import')}
            </Button>
          </div>
        </div>
      )}

      <div aria-live="polite">
        {outcome && <OutcomePanel outcome={outcome} config={config} t={t} />}
      </div>
    </div>
  );
};

function OutcomePanel({ outcome, config, t }: { outcome: Outcome; config: CsvImportConfig; t: T }) {
  if (outcome.kind === 'created') {
    return (
      <div className="mt-6 flex items-start gap-3 rounded-lg border border-success/50 bg-success/10 p-4">
        <CheckCircle2 size={22} className="mt-0.5 shrink-0 text-success-text" aria-hidden />
        <div>
          <p className="font-medium text-fg-primary">{config.created(outcome.created)}.</p>
          <Link to={config.open.to} className="text-sm text-accent hover:underline">
            {config.open.label}
          </Link>
        </div>
      </div>
    );
  }

  const title =
    outcome.kind === 'rejected'
      ? outcome.rejected > 0
        ? t('csvImport.rejectedRows', { errors: outcome.errors.length, rows: outcome.rejected })
        : // The file as a whole was refused (header, scale, size): no row to count.
          t('csvImport.fileRefused')
      : outcome.kind === 'limit'
        ? config.limitTitle
        : t('csvImport.nothingImported');

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
                ? config.limitDetail(outcome.requested, outcome.remaining) + ' '
                : ''}
              <Link to="/settings?tab=billing" className="text-accent hover:underline">
                {t('csvImport.seePlans')}
              </Link>
            </p>
          )}
          {outcome.kind === 'failed' && (
            <p className="mt-1 text-sm text-fg-secondary">{outcome.message}</p>
          )}
          {outcome.kind === 'rejected' && (
            <>
              <p className="mt-1 text-sm text-fg-secondary">{t('csvImport.fixRows')}</p>
              <div className="mt-3 max-h-80 overflow-auto rounded-md border border-border-subtle bg-surface-0">
                <table className="w-full text-sm">
                  <thead className="sticky top-0 bg-surface-1 text-left text-xs text-fg-secondary">
                    <tr>
                      <th scope="col" className="px-3 py-2 font-medium">
                        {t('csvImport.line')}
                      </th>
                      <th scope="col" className="px-3 py-2 font-medium">
                        {t('csvImport.column')}
                      </th>
                      <th scope="col" className="px-3 py-2 font-medium">
                        {t('csvImport.problem')}
                      </th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border-subtle">
                    {outcome.errors.map((e, i) => (
                      <tr key={`${e.line}-${e.column ?? ''}-${i}`}>
                        <td className="px-3 py-2 tabular-nums text-fg-primary">
                          {e.line > 0 ? e.line : t('csvImport.file')}
                        </td>
                        <td className="px-3 py-2 font-mono text-fg-secondary">{e.column ?? '—'}</td>
                        <td className="px-3 py-2 text-fg-secondary">{importErrorMessage(e, t)}</td>
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

function toOutcome(err: unknown, t: T, forbidden: string): Outcome {
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
      return { kind: 'failed', message: forbidden };
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
    message: t('csvImport.serverFailed'),
  };
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
