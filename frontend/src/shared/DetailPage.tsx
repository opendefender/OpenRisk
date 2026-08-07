// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The shell every detail route renders inside.
//
// Detail views used to be component state — a drawer opened over a list. That
// made them unaddressable (no link to share), invisible to Back, and dependent
// on whatever ad-hoc close affordance the parent happened to draw. Promoting
// them to routes fixes all three, but only if each one actually renders a way
// back; this shell is what guarantees it, so no detail page can forget.
//
// It provides: a back control wired to useBackTo (history when there is any,
// the route tree's parent otherwise), the loading / error / not-found states,
// and registration of the record's name for the breadcrumb.

import type { ReactNode } from 'react';
import { ArrowLeft } from 'lucide-react';
import { useLocation } from 'react-router';
import { useBackTo } from './useBackTo';
import { useRegisterCrumb } from './crumbLabels';
import { EmptyState } from './EmptyState';
import { SkeletonRows, Btn } from './ui';
import { useUIStore } from '../store/uiStore';

interface DetailPageProps {
  /** Record name, shown as the title and used as the breadcrumb's leaf label. */
  title?: string | null;
  /** Rendered under the title — reference, status chips, dates. */
  subtitle?: ReactNode;
  /** Right-aligned page actions. */
  actions?: ReactNode;
  loading?: boolean;
  /** True when the record does not exist (or is not this tenant's). */
  notFound?: boolean;
  /** Non-null when loading failed, as distinct from the record not existing. */
  error?: unknown;
  /** Label for the back control, e.g. "Audits". Defaults to a generic "Back". */
  backLabel?: string;
  children?: ReactNode;
}

export function DetailPage({
  title, subtitle, actions, loading, notFound, error, backLabel, children,
}: DetailPageProps) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { pathname } = useLocation();
  const { goBack, parent } = useBackTo();

  // Sharpens the breadcrumb's last crumb from "Audit" to the audit's own title.
  useRegisterCrumb(pathname, title ?? null);

  const back = (
    // An anchor, not a button: middle-click and "open in new tab" are how people
    // navigate back up a tree they intend to return to.
    <a
      href={parent}
      onClick={(e) => {
        if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
        e.preventDefault();
        goBack();
      }}
      data-testid="detail-back"
      className="inline-flex items-center gap-1.5 text-[12.5px] font-semibold text-ink-soft hover:text-ink mb-3 transition-colors"
    >
      <ArrowLeft size={15} /> {backLabel ?? tr('Retour', 'Back')}
    </a>
  );

  return (
    <div className="flex-1 overflow-y-auto">
      <div className="mx-auto px-5 sm:px-7 pt-6 pb-10 max-w-[1100px]" style={{ animation: 'or-fadeup .3s ease' }}>
        {back}

        {loading ? (
          <SkeletonRows rows={5} height={56} />
        ) : error ? (
          <EmptyState
            variant="error"
            title={tr('Chargement impossible', 'Could not load')}
            description={tr('Réessayez, ou contactez un administrateur si le problème persiste.', 'Retry, or contact an administrator if the problem persists.')}
            primaryAction={<Btn label={tr('Réessayer', 'Retry')} onClick={() => window.location.reload()} />}
          />
        ) : notFound ? (
          <EmptyState
            variant="no-results"
            title={tr('Introuvable', 'Not found')}
            description={tr('Cet élément n’existe plus ou ne vous est pas accessible.', 'This item no longer exists or is not accessible to you.')}
            primaryAction={<Btn label={backLabel ?? tr('Retour', 'Back')} onClick={goBack} />}
          />
        ) : (
          <>
            <div className="flex items-start justify-between flex-wrap gap-3 mb-5">
              <div className="min-w-0">
                <h1 className="disp text-[22px] font-bold tracking-tight text-ink truncate">{title}</h1>
                {subtitle && <div className="text-[13px] text-ink-soft mt-1.5 flex items-center gap-2 flex-wrap">{subtitle}</div>}
              </div>
              {actions && <div className="flex items-center gap-2 shrink-0">{actions}</div>}
            </div>
            {children}
          </>
        )}
      </div>
    </div>
  );
}

/** A labelled field row, the common building block of these detail bodies. */
export function DetailField({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="py-3" style={{ borderBottom: '1px solid var(--border)' }}>
      <div className="text-[11px] uppercase tracking-wide text-ink-muted mb-1">{label}</div>
      <div className="text-[13.5px] text-ink">{children || <span className="text-ink-muted">—</span>}</div>
    </div>
  );
}
