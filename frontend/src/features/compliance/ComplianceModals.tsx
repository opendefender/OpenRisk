// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Compliance dialogs in the dc.html design language (co-located with the
// EvidenceDrawer they sit next to): create a blank framework, import a
// regulatory framework from the catalog, or add an ad-hoc control. Kept out of
// the old zinc-styled modals so the redesigned Compliance screens stay visually
// consistent.

import { useEffect, useState, type ReactNode } from 'react';
import { z } from 'zod';
import { X, Library, Clock, Loader2, Plus, ClipboardPlus } from 'lucide-react';
import { toast } from 'sonner';
import { useUIStore } from '../../store/uiStore';
import { useCatalogs, useFrameworks, useControls, useImportCatalogAsFramework } from './useCompliance';
import type { ComplianceCatalogSummary } from '../../types/compliance';

/* ------------------------------------------------------------------ */
/* shared primitives                                                   */
/* ------------------------------------------------------------------ */

function useTr() {
  const lang = useUIStore((s) => s.lang);
  return (fr: string, en: string) => (lang === 'fr' ? fr : en);
}

// errMsg pulls the server's typed error message when present, else a fallback.
function errMsg(err: unknown, fallback: string): string {
  const e = err as { response?: { data?: { error?: string } } };
  return e?.response?.data?.error || fallback;
}

export function ModalShell({
  title,
  icon,
  onClose,
  children,
  footer,
  onSubmit,
}: {
  title: string;
  icon: ReactNode;
  onClose: () => void;
  children: ReactNode;
  footer: ReactNode;
  onSubmit?: () => void;
}) {
  // Esc to close — matches the drawer's expectations.
  useEffect(() => {
    const h = (e: KeyboardEvent) => e.key === 'Escape' && onClose();
    window.addEventListener('keydown', h);
    return () => window.removeEventListener('keydown', h);
  }, [onClose]);

  return (
    <div
      className="fixed inset-0 z-[70] flex items-center justify-center p-4"
      style={{ background: 'rgba(0,0,0,.45)', backdropFilter: 'blur(3px)', animation: 'or-fadein .2s ease' }}
      onClick={onClose}
    >
      <form
        onClick={(e) => e.stopPropagation()}
        onSubmit={(e) => {
          e.preventDefault();
          onSubmit?.();
        }}
        className="w-full max-w-[480px] max-h-[90vh] flex flex-col rounded-[16px] overflow-hidden"
        style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)', animation: 'or-scalein .22s cubic-bezier(.2,.8,.2,1)' }}
      >
        <div className="px-[22px] pt-5 pb-4 flex items-center gap-3" style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="w-9 h-9 rounded-[10px] flex items-center justify-center shrink-0" style={{ background: 'var(--accent-soft)', color: 'var(--accent)' }}>
            {icon}
          </div>
          <div className="disp text-[17px] font-bold text-ink flex-1">{title}</div>
          <button type="button" onClick={onClose} className="w-8 h-8 rounded-[9px] flex items-center justify-center shrink-0 text-ink-soft hover:text-ink transition-colors" style={{ background: 'var(--bg-hover)' }} aria-label="Close">
            <X size={18} />
          </button>
        </div>
        <div className="flex-1 overflow-y-auto p-[22px] flex flex-col gap-4">{children}</div>
        <div className="px-[22px] py-4 flex justify-end gap-2.5" style={{ borderTop: '1px solid var(--border)' }}>
          {footer}
        </div>
      </form>
    </div>
  );
}

export function Field({
  label, value, onChange, placeholder, required, error, autoFocus, type,
}: {
  label: string; value: string; onChange: (v: string) => void; placeholder?: string; required?: boolean; error?: string; autoFocus?: boolean; type?: string;
}) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">
        {label} {required && <span style={{ color: 'var(--critical)' }}>*</span>}
      </span>
      <input
        value={value}
        type={type ?? 'text'}
        autoFocus={autoFocus}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="w-full h-10 px-3.5 rounded-[10px] text-[13px] text-ink outline-none focus:border-accent transition-colors"
        style={{ border: `1px solid ${error ? 'var(--critical)' : 'var(--border-strong)'}`, background: 'var(--bg-elevated)' }}
      />
      {error && <span className="text-[11.5px]" style={{ color: 'var(--critical)' }}>{error}</span>}
    </label>
  );
}

// SelectField mirrors Field's styling for a native <select>.
export function SelectField({
  label, value, onChange, options, required,
}: {
  label: string; value: string; onChange: (v: string) => void; options: { value: string; label: string }[]; required?: boolean;
}) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">
        {label} {required && <span style={{ color: 'var(--critical)' }}>*</span>}
      </span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full h-10 px-3 rounded-[10px] text-[13px] text-ink outline-none focus:border-accent transition-colors"
        style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>{o.label}</option>
        ))}
      </select>
    </label>
  );
}

export function TextArea({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (v: string) => void; placeholder?: string }) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">{label}</span>
      <textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        rows={3}
        className="w-full px-3.5 py-2.5 rounded-[10px] text-[13px] text-ink outline-none focus:border-accent transition-colors resize-none"
        style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
      />
    </label>
  );
}

export function FooterButtons({ onCancel, submitLabel, pending }: { onCancel: () => void; submitLabel: string; pending: boolean }) {
  const tr = useTr();
  return (
    <>
      <button type="button" onClick={onCancel} className="h-9 px-3.5 rounded-[10px] text-[13px] font-semibold text-ink-soft hover:text-ink transition-colors" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}>
        {tr('Annuler', 'Cancel')}
      </button>
      <button type="submit" disabled={pending} className="h-9 px-4 rounded-[10px] text-[13px] font-semibold text-white inline-flex items-center gap-1.5 transition-all disabled:opacity-60" style={{ border: 'none', background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))', boxShadow: '0 3px 12px var(--accent-glow)' }}>
        {pending && <Loader2 size={15} className="animate-spin" />}
        {submitLabel}
      </button>
    </>
  );
}

/* ------------------------------------------------------------------ */
/* Create a blank framework                                            */
/* ------------------------------------------------------------------ */

const frameworkSchema = z.object({
  name: z.string().trim().min(2),
  version: z.string().trim().optional(),
  description: z.string().trim().optional(),
});

export function CreateFrameworkDialog({ onClose, onCreated }: { onClose: () => void; onCreated?: (id: string) => void }) {
  const tr = useTr();
  const { createFramework } = useFrameworks();
  const [name, setName] = useState('');
  const [version, setVersion] = useState('');
  const [description, setDescription] = useState('');
  const [error, setError] = useState('');

  const submit = () => {
    const parsed = frameworkSchema.safeParse({ name, version, description });
    if (!parsed.success) {
      setError(tr('Le nom doit comporter au moins 2 caractères.', 'Name must be at least 2 characters.'));
      return;
    }
    createFramework.mutate(
      { name: name.trim(), version: version.trim() || undefined, description: description.trim() || undefined },
      {
        onSuccess: (fw) => {
          toast.success(tr('Référentiel créé', 'Framework created'));
          onCreated?.(fw.id);
          onClose();
        },
        onError: (err) => toast.error(errMsg(err, tr('Création échouée', 'Creation failed'))),
      }
    );
  };

  return (
    <ModalShell title={tr('Nouveau référentiel', 'New framework')} icon={<Plus size={18} />} onClose={onClose} onSubmit={submit}
      footer={<FooterButtons onCancel={onClose} submitLabel={tr('Créer', 'Create')} pending={createFramework.isPending} />}>
      <Field label={tr('Nom', 'Name')} value={name} onChange={(v) => { setName(v); setError(''); }} required autoFocus
        placeholder={tr('ex. Politique interne SSI', 'e.g. Internal security policy')} error={error} />
      <Field label={tr('Version', 'Version')} value={version} onChange={setVersion} placeholder={tr('ex. 2024', 'e.g. 2024')} />
      <TextArea label={tr('Description', 'Description')} value={description} onChange={setDescription}
        placeholder={tr('À quoi sert ce référentiel ?', 'What is this framework for?')} />
    </ModalShell>
  );
}

/* ------------------------------------------------------------------ */
/* Import a framework from the regulatory catalog                      */
/* ------------------------------------------------------------------ */

export function ImportFrameworkDialog({ onClose, onImported }: { onClose: () => void; onImported?: (id: string) => void }) {
  const tr = useTr();
  const { data: catalogs, isLoading, error } = useCatalogs();
  const importCatalog = useImportCatalogAsFramework();

  const handleImport = (catalog: ComplianceCatalogSummary) => {
    importCatalog.mutate(catalog, {
      onSuccess: ({ framework, result }) => {
        toast.success(tr(
          `${result.imported} contrôle(s) importé(s)`,
          `${result.imported} control(s) imported`
        ));
        onImported?.(framework.id);
        onClose();
      },
      onError: (err) => toast.error(errMsg(err, tr('Import échoué', 'Import failed'))),
    });
  };

  return (
    <ModalShell title={tr('Importer un référentiel', 'Import a framework')} icon={<Library size={18} />} onClose={onClose}
      footer={
        <button type="button" onClick={onClose} className="h-9 px-3.5 rounded-[10px] text-[13px] font-semibold text-ink-soft hover:text-ink transition-colors" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}>
          {tr('Fermer', 'Close')}
        </button>
      }>
      <p className="text-[12.5px] text-ink-soft leading-relaxed -mt-1">
        {tr(
          'Choisissez un référentiel réglementaire : ses contrôles sont importés dans un nouveau référentiel prêt à suivre.',
          'Pick a regulatory framework: its controls are imported into a new, ready-to-track framework.'
        )}
      </p>
      {isLoading ? (
        <div className="flex flex-col gap-2">
          {[0, 1, 2].map((i) => <div key={i} className="h-16 rounded-[11px] animate-pulse" style={{ background: 'var(--bg-hover)' }} />)}
        </div>
      ) : error ? (
        <p className="text-[13px]" style={{ color: 'var(--critical)' }}>{tr('Erreur réseau', 'Network error')}</p>
      ) : !catalogs || catalogs.length === 0 ? (
        <p className="text-[13px] text-ink-muted">{tr('Aucun catalogue disponible.', 'No catalog available.')}</p>
      ) : (
        <div className="flex flex-col gap-2">
          {catalogs.map((catalog) => {
            const pending = importCatalog.isPending && importCatalog.variables?.key === catalog.key;
            return (
              <div key={catalog.key} className="flex items-center gap-3 px-3.5 py-3 rounded-[11px]"
                style={{ border: '1px solid var(--border)', opacity: catalog.available ? 1 : 0.6 }}>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-[13.5px] font-semibold text-ink truncate">
                      {catalog.name}{catalog.version ? ` ${catalog.version}` : ''}
                    </span>
                    {!catalog.available && (
                      <span className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider text-ink-muted" style={{ background: 'var(--bg-hover)' }}>
                        <Clock size={10} />{tr('Bientôt', 'Soon')}
                      </span>
                    )}
                  </div>
                  <div className="text-[11.5px] text-ink-muted mt-0.5 truncate">
                    {catalog.available
                      ? tr(`${catalog.control_count} contrôles`, `${catalog.control_count} controls`)
                      : tr('Contenu à venir', 'Content coming soon')}
                  </div>
                </div>
                <button
                  type="button"
                  disabled={!catalog.available || importCatalog.isPending}
                  onClick={() => handleImport(catalog)}
                  className="h-8 px-3 rounded-[9px] text-[12.5px] font-semibold shrink-0 inline-flex items-center gap-1.5 transition-all disabled:opacity-50"
                  style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)', color: 'var(--text-primary)' }}
                >
                  {pending ? <Loader2 size={14} className="animate-spin" /> : tr('Importer', 'Import')}
                </button>
              </div>
            );
          })}
        </div>
      )}
    </ModalShell>
  );
}

/* ------------------------------------------------------------------ */
/* Add an ad-hoc control to a framework                                */
/* ------------------------------------------------------------------ */

const controlSchema = z.object({
  name: z.string().trim().min(2),
  reference_code: z.string().trim().optional(),
  description: z.string().trim().optional(),
});

export function CreateControlDialog({ frameworkId, onClose }: { frameworkId: string; onClose: () => void }) {
  const tr = useTr();
  const { createControl } = useControls(frameworkId);
  const [name, setName] = useState('');
  const [referenceCode, setReferenceCode] = useState('');
  const [description, setDescription] = useState('');
  const [error, setError] = useState('');

  const submit = () => {
    const parsed = controlSchema.safeParse({ name, reference_code: referenceCode, description });
    if (!parsed.success) {
      setError(tr('Le nom doit comporter au moins 2 caractères.', 'Name must be at least 2 characters.'));
      return;
    }
    createControl.mutate(
      { name: name.trim(), reference_code: referenceCode.trim() || undefined, description: description.trim() || undefined },
      {
        onSuccess: () => {
          toast.success(tr('Contrôle ajouté', 'Control added'));
          onClose();
        },
        onError: (err) => toast.error(errMsg(err, tr('Ajout échoué', 'Failed to add control'))),
      }
    );
  };

  return (
    <ModalShell title={tr('Nouveau contrôle', 'New control')} icon={<ClipboardPlus size={18} />} onClose={onClose} onSubmit={submit}
      footer={<FooterButtons onCancel={onClose} submitLabel={tr('Ajouter', 'Add')} pending={createControl.isPending} />}>
      <Field label={tr('Code de référence', 'Reference code')} value={referenceCode} onChange={setReferenceCode}
        placeholder={tr('ex. A.5.1 (optionnel)', 'e.g. A.5.1 (optional)')} />
      <Field label={tr('Intitulé', 'Name')} value={name} onChange={(v) => { setName(v); setError(''); }} required autoFocus
        placeholder={tr('ex. Politique de sécurité', 'e.g. Security policy')} error={error} />
      <TextArea label={tr('Description', 'Description')} value={description} onChange={setDescription}
        placeholder={tr('Ce que le contrôle exige…', 'What the control requires…')} />
    </ModalShell>
  );
}
