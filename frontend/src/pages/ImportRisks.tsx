// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useState, useRef, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Upload,
  AlertCircle,
  CheckCircle2,
  FileJson,
  FileText,
  FileSpreadsheet,
  Download,
  ArrowLeft,
  X,
} from 'lucide-react';
import { useToast } from '../hooks/useToast';
import { useI18n, interpolate } from '../hooks/useI18n';
import { Button } from '../components/ui/Button';
import { api } from '../lib/api';
import { useRiskStore } from '../hooks/useRiskStore';
import { EmptyState, SkeletonTable } from '../components/shared';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

interface ImportResult {
  success: number;
  failed: number;
  errors: Array<{ row: number; message: string }>;
}

type DragState = 'idle' | 'dragging' | 'processing';
type FileFormat = 'csv' | 'json' | 'xlsx';

export const ImportRisksPage = () => {
  const { t } = useI18n();
  const { success, error, promise } = useToast();
  const { fetchRisks } = useRiskStore();

  const [dragState, setDragState] = useState<DragState>('idle');
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<any[]>([]);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);
  const [isImporting, setIsImporting] = useState(false);
  const [mappedColumns, setMappedColumns] = useState<Record<string, string>>({});
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Get file icon based on format
  const getFileIcon = (format: FileFormat) => {
    switch (format) {
      case 'json':
        return <FileJson size={32} />;
      case 'xlsx':
        return <FileSpreadsheet size={32} />;
      default:
        return <FileText size={32} />;
    }
  };

  // Parse file and show preview
  const handleFileSelect = useCallback(async (file: File) => {
    const format = file.name.split('.').pop()?.toLowerCase() as FileFormat | undefined;

    if (!['csv', 'json', 'xlsx'].includes(format || '')) {
      error(t('errors.invalidFile'));
      return;
    }

    setSelectedFile(file);
    setDragState('processing');

    try {
      let data: any[] = [];

      if (format === 'json') {
        const text = await file.text();
        data = JSON.parse(text);
      } else if (format === 'csv') {
        // Simple CSV parser (production would use a library)
        const text = await file.text();
        const lines = text.split('\n');
        const headers = lines[0].split(',').map((h) => h.trim());
        data = lines.slice(1).map((line) => {
          const values = line.split(',');
          return headers.reduce((acc, header, i) => {
            acc[header] = values[i]?.trim() || '';
            return acc;
          }, {} as Record<string, string>);
        });
      } else if (format === 'xlsx') {
        error(t('common.loading')); // Placeholder - need excelize library
        return;
      }

      // Show first 10 rows as preview
      setPreview(data.slice(0, 10));
      setDragState('idle');
      success(t('messages.importStarted'));
    } catch (err) {
      error(interpolate(t('errors.failedToImportRisks'), {}));
      setDragState('idle');
    }
  }, [t, error, success]);

  // Handle drag and drop
  const handleDragOver = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setDragState('dragging');
  };

  const handleDragLeave = () => {
    setDragState('idle');
  };

  const handleDrop = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    const files = e.dataTransfer.files;
    if (files.length > 0) {
      handleFileSelect(files[0]);
    }
    setDragState('idle');
  };

  // Handle file input change
  const handleFileInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files?.length) {
      handleFileSelect(e.target.files[0]);
    }
  };

  // Submit import
  const handleImport = async () => {
    if (!selectedFile) return;

    setIsImporting(true);

    try {
      const formData = new FormData();
      formData.append('file', selectedFile);

      const importRequest = api.post('/risks/import', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });

      promise(importRequest, {
        loading: t('messages.importStarted'),
        success: t('messages.importCompleted'),
        error: t('errors.failedToImportRisks'),
      });

      const response = await importRequest;
      const result: ImportResult = response.data;
      setImportResult(result);

      // Refresh risks list
      await fetchRisks();
    } catch (err) {
      console.error('Import failed:', err);
    } finally {
      setIsImporting(false);
    }
  };

  // Download template
  const handleDownloadTemplate = () => {
    const template = `Title,Description,Probability,Impact,Status,Framework,Tags,Assets
"Web API Vulnerability","Unvalidated API endpoints",3,4,Open,OWASP,"API,Security","API-001"
"Database Compromise","SQL injection risk",4,5,Open,NIST,"Database","DB-001"`;

    const blob = new Blob([template], { type: 'text/csv' });
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'risks-template.csv';
    a.click();
    window.URL.revokeObjectURL(url);
  };

  return (
    <div className="max-w-5xl mx-auto p-6">
      {/* Header */}
      <div className="mb-8">
        <Link to="/risks" className="inline-flex items-center gap-1.5 text-sm font-medium text-zinc-400 hover:text-white transition-colors mb-3">
          <ArrowLeft size={15} /> {t('risks.title')}
        </Link>
        <h1 className="text-3xl font-bold text-white mb-2">{t('risks.import')}</h1>
        <p className="text-zinc-400">{t('risks.dragDropHint')}</p>
      </div>

      {/* Main content area */}
      {!importResult ? (
        <div className="space-y-6">
          {/* Drag & Drop Zone */}
          <motion.div
            animate={{
              backgroundColor:
                dragState === 'dragging' ? 'rgba(59, 130, 246, 0.1)' : 'rgba(0, 0, 0, 0)',
              borderColor:
                dragState === 'dragging' ? 'rgba(59, 130, 246, 0.5)' : 'rgba(255, 255, 255, 0.1)',
            }}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
            className={cn(
              'relative border-2 border-dashed rounded-xl p-12 text-center',
              'transition-all duration-300 cursor-pointer',
              dragState === 'dragging' && 'bg-blue-500/10 border-blue-500/50'
            )}
            onClick={() => fileInputRef.current?.click()}
          >
            {dragState === 'processing' ? (
              <div className="flex flex-col items-center gap-4">
                <div className="animate-spin">
                  <Upload className="text-blue-500" size={48} />
                </div>
                <p className="text-zinc-300">{t('common.loading')}</p>
              </div>
            ) : (
              <>
                <div className="flex justify-center gap-4 mb-4">
                  <FileJson className="text-blue-400" size={32} />
                  <FileText className="text-amber-400" size={32} />
                  <FileSpreadsheet className="text-emerald-400" size={32} />
                </div>
                <h3 className="text-lg font-semibold text-white mb-2">{t('risks.dragDropHint')}</h3>
                <p className="text-sm text-zinc-400">CSV, JSON, XLSX</p>
              </>
            )}

            <input
              ref={fileInputRef}
              type="file"
              accept=".csv,.json,.xlsx"
              onChange={handleFileInputChange}
              className="hidden"
            />
          </motion.div>

          {/* Template Download */}
          <div className="flex justify-center">
            <Button
              onClick={handleDownloadTemplate}
              variant="ghost"
              className="gap-2"
            >
              <Download size={16} />
              {t('risks.templateDownload')}
            </Button>
          </div>

          {/* Preview */}
          {preview.length > 0 && (
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              className="space-y-4"
            >
              <div className="flex items-center justify-between">
                <h3 className="text-lg font-semibold text-white">{t('risks.importPreview')}</h3>
                <Button
                  variant="ghost"
                  onClick={() => {
                    setSelectedFile(null);
                    setPreview([]);
                  }}
                  className="gap-1"
                >
                  <X size={16} />
                </Button>
              </div>

              {/* Preview Table */}
              <div className="bg-zinc-900/50 border border-zinc-800 rounded-lg overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="border-b border-zinc-700">
                    <tr>
                      {Object.keys(preview[0] || {})
                        .slice(0, 6)
                        .map((key) => (
                          <th
                            key={key}
                            className="px-4 py-3 text-left text-xs font-medium text-zinc-400 uppercase"
                          >
                            {key}
                          </th>
                        ))}
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-zinc-700">
                    {preview.slice(0, 5).map((row, i) => (
                      <tr key={i} className="hover:bg-zinc-800/50">
                        {Object.values(row)
                          .slice(0, 6)
                          .map((val, j) => (
                            <td key={j} className="px-4 py-3 text-zinc-300 truncate max-w-xs">
                              {String(val)}
                            </td>
                          ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Import Button */}
              <div className="flex justify-end gap-2">
                <Button
                  variant="secondary"
                  onClick={handleImport}
                  isLoading={isImporting}
                  className="gap-2"
                >
                  <Upload size={16} />
                  {t('common.confirm')}
                </Button>
              </div>
            </motion.div>
          )}
        </div>
      ) : (
        /* Results */
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          className="space-y-6"
        >
          {importResult.success > 0 && (
            <div className="flex items-center gap-4 p-4 rounded-lg bg-emerald-500/10 border border-emerald-500/50">
              <CheckCircle2 size={24} className="text-emerald-400 flex-shrink-0" />
              <div>
                <h4 className="font-semibold text-white">{t('messages.importCompleted')}</h4>
                <p className="text-sm text-zinc-300">
                  {interpolate(t('risks.successCount'), { count: importResult.success })}
                </p>
              </div>
            </div>
          )}

          {importResult.errors.length > 0 && (
            <div className="flex items-start gap-4 p-4 rounded-lg bg-red-500/10 border border-red-500/50">
              <AlertCircle size={24} className="text-red-400 flex-shrink-0 mt-0.5" />
              <div className="flex-1">
                <h4 className="font-semibold text-white">
                  {interpolate(t('risks.errorCount'), { count: importResult.errors.length })}
                </h4>
                <div className="mt-2 space-y-1 max-h-32 overflow-y-auto">
                  {importResult.errors.slice(0, 5).map((err, i) => (
                    <p key={i} className="text-xs text-zinc-300">
                      Row {err.row}: {err.message}
                    </p>
                  ))}
                  {importResult.errors.length > 5 && (
                    <p className="text-xs text-zinc-400">
                      ...and {importResult.errors.length - 5} more
                    </p>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* Done Button */}
          <div className="flex gap-2 justify-end">
            <Button
              onClick={() => {
                setImportResult(null);
                setSelectedFile(null);
                setPreview([]);
              }}
              variant="secondary"
            >
              {t('common.close')}
            </Button>
          </div>
        </motion.div>
      )}
    </div>
  );
};

export default ImportRisksPage;
