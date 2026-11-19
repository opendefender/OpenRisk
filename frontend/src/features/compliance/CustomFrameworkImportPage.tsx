// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Import of a custom compliance framework from a JSON file (#307).
// Steps: choose a file -> client-side parse and preview -> one POST. The server
// writes the framework and every control or nothing; this page reports exactly
// what it answered. There is no optimistic update: the new framework's id only
// exists once the server has written it.

import { useRef, useState } from 'react';
import { Link, useNavigate } from 'react-router';
import axios from 'axios';
import { AlertCircle, ArrowLeft, FileText, Upload, X } from 'lucide-react';

import { Button, Skeleton, cn } from '../../shared/ds';
import { useI18n } from '../../hooks/useI18n';
import { useToast } from '../../hooks/useToast';
import { useImportFramework } from './useCompliance';
import {
  frameworkFileSchema,
  importRejectedSchema,
  parseFrameworkDocument,
  type Problem,
  type T,
} from './customImportSchema';
import type { FrameworkExportDocument } from '../../types/compliance';

type Failure =
  | { kind: 'rejected'; message: string; problems: Problem[] }
  | { kind: 'conflict' | 'limit' | 'forbidden' | 'unauthorized' | 'generic' };

const PREVIEW_ROWS = 5;

function toFailure(err: unknown): Failure {
  if (axios.isAxiosError(err) && err.response) {
    const { status, data } = err.response;
    switch (status) {
      case 400: {
        const parsed = importRejectedSchema.safeParse(data);
        return {
          kind: 'rejected',
          message: parsed.success ? (parsed.data.error ?? '') : '',
          problems: parsed.success ? (parsed.data.errors ?? []) : [],
        };
      }
      case 401:
        return { kind: 'unauthorized' };
      case 402:
        return { kind: 'limit' };
      case 403:
        return { kind: 'forbidden' };
      case 409:
        return { kind: 'conflict' };
      default:
        return { kind: 'generic' };
    }
  }
  return { kind: 'generic' };
}

function failureTitle(failure: Failure | null, hasRows: boolean, t: T): string {
  if (!failure) return t('compliance.customImport.fileRefused');
  switch (failure.kind) {
    case 'rejected':
      return hasRows
        ? t('compliance.customImport.rejectedRows')
        : t('compliance.customImport.fileRefused');
    case 'conflict':
      return t('compliance.customImport.conflict');
    case 'limit':
      return t('compliance.customImport.limit');
    case 'forbidden':
      return t('compliance.customImport.forbidden');
    case 'unauthorized':
      return t('compliance.customImport.unauthorized');
    case 'generic':
    default:
      return t('compliance.customImport.serverFailed');
  }
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export const CustomFrameworkImportPage = () => {
  const { t } = useI18n();
  const { success } = useToast();
  const navigate = useNavigate();
  const importFramework = useImportFramework();

  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [document, setDocument] = useState<FrameworkExportDocument | null>(null);
  const [clientProblems, setClientProblems] = useState<Problem[]>([]);
  const [truncated, setTruncated] = useState(false);
  const [failure, setFailure] = useState<Failure | null>(null);
  const [dragging, setDragging] = useState(false);

  const submitting = importFramework.isPending;

  const reset = () => {
    setFile(null);
    setFileError(null);
    setDocument(null);
    setClientProblems([]);
    setTruncated(false);
    setFailure(null);
    if (inputRef.current) inputRef.current.value = '';
  };

  const pick = async (f: File | undefined) => {
    if (!f) return;
    reset();
    const checked = frameworkFileSchema(t).safeParse(f);
    if (!checked.success) {
      setFileError(checked.error.issues[0]?.message ?? t('compliance.customImport.invalidFile'));
      return;
    }
    setFile(f);
    let text: string;
    try {
      text = await f.text();
    } catch {
      setFile(null);
      setFileError(t('compliance.customImport.unreadable'));
      return;
    }
    const result = parseFrameworkDocument(text, t);
    if (result.ok) {
      setDocument(result.document);
    } else {
      setClientProblems(result.problems);
      setTruncated(result.truncated);
    }
  };

  const submit = () => {
    if (!document || submitting) return;
    setFailure(null);
    importFramework.mutate(document, {
      onSuccess: (created) => {
        success(t('compliance.customImport.created', { count: created.controls_created }));
        navigate(`/compliance/frameworks/${created.id}`);
      },
      onError: (err) => setFailure(toFailure(err)),
    });
  };

  const problems = failure?.kind === 'rejected' ? failure.problems : clientProblems;
  const showTable = problems.length > 0;
  const showError = !submitting && (showTable || (failure !== null && failure.kind !== 'rejected'));

  return (
    <div className="max-w-4xl mx-auto p-6">
      <div className="mb-6">
        <Link
          to="/compliance"
          className="inline-flex items-center gap-1.5 text-sm font-medium text-fg-secondary hover:text-fg-primary transition-colors mb-3"
        >
          <ArrowLeft size={15} aria-hidden /> {t('compliance.customImport.back')}
        </Link>
        <h1 className="text-2xl font-semibold text-fg-primary mb-1">
          {t('compliance.customImport.title')}
        </h1>
        <p className="text-sm text-fg-secondary">{t('compliance.customImport.intro')}</p>
      </div>

      <div
        role="button"
        tabIndex={0}
        aria-describedby={fileError ? 'framework-import-file-error' : undefined}
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
          void pick(e.dataTransfer.files[0]);
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
        <p className="font-medium text-fg-primary">{t('compliance.customImport.dropHere')}</p>
        <p className="mt-1 text-xs text-fg-tertiary">{t('compliance.customImport.limits')}</p>
      </div>
      <input
        ref={inputRef}
        type="file"
        accept=".json,application/json"
        aria-label={t('compliance.customImport.dropHere')}
        className="hidden"
        tabIndex={-1}
        onChange={(e) => void pick(e.target.files?.[0])}
        data-testid="framework-import-file-input"
      />
      {fileError && (
        <p id="framework-import-file-error" role="alert" className="mt-2 text-sm text-danger-text">
          {fileError}
        </p>
      )}

      {/* Empty state: nothing chosen yet. The CTA opens the picker. */}
      {!file && !fileError && (
        <div
          className="mt-4 rounded-lg border border-border-subtle bg-surface-1 p-4 text-sm"
          data-testid="framework-import-empty"
        >
          <p className="font-medium text-fg-primary">{t('compliance.customImport.emptyTitle')}</p>
          <p className="mt-1 text-fg-secondary">{t('compliance.customImport.emptyHint')}</p>
          <Button
            variant="secondary"
            size="sm"
            className="mt-3 gap-1.5"
            onClick={() => inputRef.current?.click()}
          >
            <Upload size={14} aria-hidden /> {t('compliance.customImport.choose')}
          </Button>
        </div>
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
              aria-label={t('compliance.customImport.removeFile')}
            >
              <X size={16} aria-hidden />
            </Button>
            <Button
              variant="primary"
              onClick={submit}
              loading={submitting}
              disabled={!document}
              className="gap-1.5"
            >
              <Upload size={15} aria-hidden />
              {t('compliance.customImport.submit')}
            </Button>
          </div>
        </div>
      )}

      {document && !submitting && (
        <section
          aria-labelledby="framework-import-preview"
          className="mt-4 rounded-lg border border-border-subtle bg-surface-1 p-4 text-sm"
          data-testid="framework-import-preview"
        >
          <h2 id="framework-import-preview" className="font-medium text-fg-primary">
            {t('compliance.customImport.previewTitle')}
          </h2>
          <dl className="mt-3 grid grid-cols-1 gap-x-6 gap-y-2 sm:grid-cols-[max-content_1fr] text-fg-secondary">
            <dt className="text-fg-tertiary">{t('compliance.customImport.previewName')}</dt>
            <dd className="text-fg-primary">{document.framework.name}</dd>
            <dt className="text-fg-tertiary">{t('compliance.customImport.previewVersion')}</dt>
            <dd className="text-fg-primary">{document.framework.version || '—'}</dd>
            <dt className="text-fg-tertiary">{t('compliance.customImport.previewControls')}</dt>
            <dd className="text-fg-primary">
              {t('compliance.customImport.controlsCount', { count: document.controls.length })}
            </dd>
          </dl>
          <ul className="mt-3 divide-y divide-border-subtle rounded-md border border-border-subtle bg-surface-0">
            {document.controls.slice(0, PREVIEW_ROWS).map((c) => (
              <li key={c.reference_code} className="flex gap-3 px-3 py-2">
                <span className="font-mono text-fg-secondary">{c.reference_code}</span>
                <span className="min-w-0 truncate text-fg-primary">{c.name}</span>
              </li>
            ))}
          </ul>
          {document.controls.length > PREVIEW_ROWS && (
            <p className="mt-2 text-xs text-fg-tertiary">
              {t('compliance.customImport.previewMore', {
                count: document.controls.length - PREVIEW_ROWS,
              })}
            </p>
          )}
        </section>
      )}

      {/* Importing: skeleton of the preview, never a full-page spinner. */}
      {submitting && (
        <div
          className="mt-4 rounded-lg border border-border-subtle bg-surface-1 p-4"
          role="status"
          data-testid="framework-import-skeleton"
        >
          <span className="sr-only">{t('compliance.customImport.importing')}</span>
          <Skeleton className="mb-3 h-5 w-1/3" />
          <Skeleton className="mb-2 h-4 w-2/3" />
          <Skeleton className="mb-2 h-4 w-1/2" />
          <Skeleton className="h-4 w-3/5" />
        </div>
      )}

      <div aria-live="polite">
        {showError && (
          <div
            role="alert"
            className="mt-6 rounded-lg border border-danger/50 bg-danger/10 p-4"
            data-testid="framework-import-error"
          >
            <div className="flex items-start gap-3">
              <AlertCircle size={22} className="mt-0.5 shrink-0 text-danger-text" aria-hidden />
              <div className="min-w-0 flex-1">
                <p className="font-medium text-fg-primary">{failureTitle(failure, showTable, t)}</p>
                {failure?.kind === 'rejected' && failure.message && !showTable && (
                  <p className="mt-1 text-sm text-fg-secondary">{failure.message}</p>
                )}
                {failure?.kind === 'limit' && (
                  <p className="mt-1 text-sm">
                    <Link to="/settings?tab=billing" className="text-accent hover:underline">
                      {t('compliance.customImport.seePlans')}
                    </Link>
                  </p>
                )}
                {showTable && (
                  <>
                    <p className="mt-1 text-sm text-fg-secondary">
                      {t('compliance.customImport.fixRows')}
                    </p>
                    <div className="mt-3 max-h-80 overflow-auto rounded-md border border-border-subtle bg-surface-0">
                      <table className="w-full text-sm">
                        <thead className="sticky top-0 bg-surface-1 text-left text-xs text-fg-secondary">
                          <tr>
                            <th scope="col" className="px-3 py-2 font-medium">
                              {t('compliance.customImport.row')}
                            </th>
                            <th scope="col" className="px-3 py-2 font-medium">
                              {t('compliance.customImport.field')}
                            </th>
                            <th scope="col" className="px-3 py-2 font-medium">
                              {t('compliance.customImport.problem')}
                            </th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-border-subtle">
                          {problems.map((p, i) => (
                            <tr key={`${p.index ?? 'doc'}-${p.field}-${i}`}>
                              <td className="px-3 py-2 tabular-nums text-fg-primary">
                                {p.index === null
                                  ? t('compliance.customImport.document')
                                  : p.index + 1}
                              </td>
                              <td className="px-3 py-2 font-mono text-fg-secondary">{p.field}</td>
                              <td className="px-3 py-2 text-fg-secondary">{p.message}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                    {truncated && failure === null && (
                      <p className="mt-2 text-xs text-fg-tertiary">
                        {t('compliance.customImport.truncated', { max: problems.length })}
                      </p>
                    )}
                  </>
                )}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
