// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The configurator.
//
// Picking a report type opens THIS, in place — period, scope, language, format,
// recipients — and generating leaves the user on the report. The previous
// behaviour sent them to another screen whose tile sent them back, so the
// request never terminated on a document.
//
// The type catalogue comes from the server, so the picker cannot offer a format
// the engine would refuse on submit.

import { useEffect, useMemo, useState } from 'react';
import { X, FileText, Loader2 } from 'lucide-react';
import { Btn } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { useFrameworks } from '../compliance/useCompliance';
import { useAudits } from '../compliance/useCompliance';
import { useReportCatalogue } from './useReports';
import type {
  CreateReportInput,
  ReportFormat,
  ReportLocale,
  ReportTypeOption,
} from '../../types/report';

export function ReportConfigurator({
  onClose,
  onGenerate,
  pending,
  initialType,
}: {
  onClose: () => void;
  onGenerate: (input: CreateReportInput) => Promise<void>;
  pending?: boolean;
  initialType?: string;
}) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const { data: catalogue, isLoading } = useReportCatalogue(lang);
  const { frameworks } = useFrameworks();
  const { audits } = useAudits();

  const [typeKey, setTypeKey] = useState<string>(initialType ?? '');
  const [format, setFormat] = useState<ReportFormat>('pdf');
  // The document's language, seeded from the interface but deliberately its own
  // control: a French-speaking officer routinely produces an English report.
  const [locale, setLocale] = useState<ReportLocale>(lang === 'en' ? 'en' : 'fr');
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [frameworkId, setFrameworkId] = useState('');
  const [auditId, setAuditId] = useState('');
  const [recipients, setRecipients] = useState('');
  const [error, setError] = useState<string | null>(null);

  const selected: ReportTypeOption | undefined = useMemo(
    () => catalogue?.types.find((t) => t.type === typeKey),
    [catalogue, typeKey],
  );

  // Keep the format legal for the chosen type rather than letting the user
  // submit something the server will reject.
  useEffect(() => {
    if (!selected) return;
    if (!selected.formats.some((f) => f.key === format)) {
      setFormat(selected.formats[0]?.key ?? 'pdf');
    }
  }, [selected, format]);

  const needsFramework = selected?.scope === 'framework';
  const needsAudit = selected?.scope === 'audit';
  const canSubmit = Boolean(selected) && !pending && (!needsFramework || Boolean(frameworkId));

  async function submit() {
    if (!selected) return;
    setError(null);
    try {
      await onGenerate({
        type: selected.type,
        format,
        locale,
        from: from || undefined,
        to: to || undefined,
        framework_id: frameworkId || undefined,
        audit_id: auditId || undefined,
        recipients: recipients
          .split(',')
          .map((r) => r.trim())
          .filter(Boolean),
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <div className="relative w-full max-w-[620px] max-h-[90vh] flex flex-col rounded-xl bg-surface border border-line or-scalein">
        <header className="px-5 py-4 border-b border-line flex items-center justify-between shrink-0">
          <h2 className="text-ink font-semibold text-[15px]">
            {tr('Générer un rapport', 'Generate a report')}
          </h2>
          <button className="p-1.5 rounded-md hover:bg-surface-3 text-ink-muted" onClick={onClose}>
            <X size={16} />
          </button>
        </header>

        <div className="px-5 py-4 space-y-5 overflow-y-auto">
          {isLoading ? (
            <div className="flex items-center gap-2 text-[13px] text-ink-muted">
              <Loader2 size={14} className="animate-spin" />
              {tr('Chargement des modèles…', 'Loading templates…')}
            </div>
          ) : (
            <>
              <section>
                <h3 className="text-[13px] font-medium text-ink mb-2">{tr('Type', 'Type')}</h3>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                  {catalogue?.types.map((t) => (
                    <button
                      key={t.type}
                      onClick={() => setTypeKey(t.type)}
                      className={`text-left px-3 py-2.5 rounded-lg border transition-colors ${
                        typeKey === t.type
                          ? 'border-accent bg-accent/8'
                          : 'border-line hover:bg-surface-2'
                      }`}
                    >
                      <div className="flex items-center gap-2">
                        <FileText size={14} className="text-ink-muted shrink-0" />
                        <span className="text-[13px] text-ink font-medium">{t.title}</span>
                      </div>
                      <p className="text-[12px] text-ink-muted mt-1 leading-snug">
                        {t.description}
                      </p>
                      <p className="text-[11px] text-ink-muted/80 mt-1">
                        {t.template_key} v{t.template_version}
                      </p>
                    </button>
                  ))}
                </div>
              </section>

              {selected ? (
                <>
                  {needsFramework ? (
                    <Section label={tr('Référentiel', 'Framework')} required>
                      <select
                        value={frameworkId}
                        onChange={(e) => setFrameworkId(e.target.value)}
                        className="w-full px-3 py-2 text-[13px] rounded-lg bg-surface-2 border border-line text-ink"
                      >
                        <option value="">{tr('Choisir…', 'Choose…')}</option>
                        {frameworks.map((f) => (
                          <option key={f.id} value={f.id}>
                            {f.name} {f.version}
                          </option>
                        ))}
                      </select>
                    </Section>
                  ) : null}

                  {needsAudit ? (
                    <Section
                      label={tr('Audit', 'Audit')}
                      hint={tr(
                        'Laisser vide pour couvrir tous les audits.',
                        'Leave empty to cover every audit.',
                      )}
                    >
                      <select
                        value={auditId}
                        onChange={(e) => setAuditId(e.target.value)}
                        className="w-full px-3 py-2 text-[13px] rounded-lg bg-surface-2 border border-line text-ink"
                      >
                        <option value="">{tr('Tous les audits', 'All audits')}</option>
                        {audits.map((a) => (
                          <option key={a.id} value={a.id}>
                            {a.title}
                          </option>
                        ))}
                      </select>
                    </Section>
                  ) : null}

                  <Section
                    label={tr('Période', 'Period')}
                    hint={tr(
                      "Laisser vide pour couvrir depuis l'origine.",
                      'Leave empty to cover all time.',
                    )}
                  >
                    <div className="flex items-center gap-2">
                      <input
                        type="date"
                        value={from}
                        onChange={(e) => setFrom(e.target.value)}
                        className="flex-1 px-3 py-2 text-[13px] rounded-lg bg-surface-2 border border-line text-ink"
                      />
                      <span className="text-ink-muted text-[13px]">→</span>
                      <input
                        type="date"
                        value={to}
                        onChange={(e) => setTo(e.target.value)}
                        className="flex-1 px-3 py-2 text-[13px] rounded-lg bg-surface-2 border border-line text-ink"
                      />
                    </div>
                  </Section>

                  <div className="grid grid-cols-2 gap-4">
                    <Section
                      label={tr('Langue du document', 'Document language')}
                      hint={tr(
                        "Indépendante de la langue de l'interface.",
                        'Independent of the interface language.',
                      )}
                    >
                      <div className="flex gap-1.5">
                        {(catalogue?.locales ?? []).map((l) => (
                          <button
                            key={l.key}
                            onClick={() => setLocale(l.key)}
                            className={`px-3 py-1.5 rounded-lg text-[13px] border ${
                              locale === l.key
                                ? 'border-accent text-accent bg-accent/10'
                                : 'border-line text-ink-muted'
                            }`}
                          >
                            {l.label}
                          </button>
                        ))}
                      </div>
                    </Section>

                    <Section label={tr('Format', 'Format')}>
                      <div className="flex gap-1.5">
                        {selected.formats.map((f) => (
                          <button
                            key={f.key}
                            onClick={() => setFormat(f.key)}
                            className={`px-3 py-1.5 rounded-lg text-[13px] border ${
                              format === f.key
                                ? 'border-accent text-accent bg-accent/10'
                                : 'border-line text-ink-muted'
                            }`}
                          >
                            {f.label}
                          </button>
                        ))}
                      </div>
                    </Section>
                  </div>

                  <Section
                    label={tr('Destinataires', 'Recipients')}
                    hint={tr(
                      'Inscrits sur le document, séparés par des virgules.',
                      'Recorded on the document, comma separated.',
                    )}
                  >
                    <input
                      value={recipients}
                      onChange={(e) => setRecipients(e.target.value)}
                      placeholder="comite.audit@exemple.cm, dg@exemple.cm"
                      className="w-full px-3 py-2 text-[13px] rounded-lg bg-surface-2 border border-line text-ink"
                    />
                  </Section>
                </>
              ) : (
                <p className="text-[13px] text-ink-muted">
                  {tr('Choisissez un type pour continuer.', 'Choose a type to continue.')}
                </p>
              )}

              {error ? (
                <p className="text-[12.5px]" style={{ color: 'var(--critical)' }}>
                  {error}
                </p>
              ) : null}
            </>
          )}
        </div>

        <footer className="px-5 py-3.5 border-t border-line flex items-center justify-between gap-2 shrink-0">
          <span className="text-[12px] text-ink-muted">
            {tr(
              'La génération se fait en arrière-plan : vous pouvez quitter la page.',
              'Generation runs in the background: you can leave the page.',
            )}
          </span>
          <div className="flex gap-2">
            <Btn onClick={onClose} label={tr('Annuler', 'Cancel')} />
            <Btn
              disabled={!canSubmit}
              onClick={submit}
              label={pending ? tr('Envoi…', 'Submitting…') : tr('Générer', 'Generate')}
            />
          </div>
        </footer>
      </div>
    </div>
  );
}

function Section({
  label,
  hint,
  required,
  children,
}: {
  label: string;
  hint?: string;
  required?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div>
      <label className="block text-[13px] font-medium text-ink mb-1">
        {label}
        {required ? <span style={{ color: 'var(--critical)' }}> *</span> : null}
      </label>
      {hint ? <p className="text-[12px] text-ink-muted mb-1.5">{hint}</p> : null}
      {children}
    </div>
  );
}
