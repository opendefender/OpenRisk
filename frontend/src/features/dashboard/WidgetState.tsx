// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// What a widget renders when it does not have its data.
//
// Five conditions, five different things to say, and the whole reason this
// component exists is that five personas collapsed them into one — usually into
// the most reassuring one available:
//
//   loading         we are asking
//   error           we asked and could not read it
//   no-permission   it exists; this account may not see it
//   empty-period    there is data, just none inside the selected window
//   empty           there is nothing to show yet
//
// Before this, the exec persona rendered every failure as `?? 0` — a cyber score
// of 0 and an annual exposure of 0 FCFA, indistinguishable from a genuinely
// measured zero. The audit persona rendered a failed compliance fetch as "No
// framework — import one to start", telling the user to fix a problem they did
// not have. The viewer persona fell back to counting one page of the register
// and printed the result as a tenant total.
//
// The distinction between the last two is the one people skip, and it is the one
// that matters most on a dashboard with a period control: "no risks were opened
// in the last 7 days" and "you have no risks" are opposite facts, and only one of
// them is a reason to press a Create button.

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { AlertTriangle, CalendarX, Lock, type LucideIcon } from 'lucide-react';

import { EmptyState } from '../../shared/EmptyState';
import { Btn, Skeleton } from '../../shared/ui';
import { isPermissionError } from './widgetError';
import type { LocaleCode } from '../../i18n/locales';

/* A skeleton that would show for under this long (cached / near-instant data)
 * never shows at all — the flash it would produce is worse than the half-beat
 * of nothing it replaces. Not a token: this is a debounce window, not a
 * motion duration: nothing animates during it. */
const SKELETON_GRACE_MS = 100;

/* NOT the source of truth — primitives.css is. Read back from the DOM (see
 * useDensityRowHeight for the same pattern) so the exit timer cannot drift
 * from what `duration-fast` actually does; this only stands in when the DOM
 * can't be measured (SSR) or the token has gone missing. */
const FALLBACK_DUR_FAST_MS = 120;

function readTokenMs(varName: string, fallback: number): number {
  if (typeof document === 'undefined') return fallback;
  const raw = getComputedStyle(document.documentElement).getPropertyValue(varName);
  const ms = Number.parseFloat(raw);
  return Number.isFinite(ms) && ms > 0 ? ms : fallback;
}

function prefersReducedMotion(): boolean {
  return (
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  );
}

/**
 * The skeleton -> content reveal's timing, kept out of the component so the
 * two guarantees it exists for are each one obvious branch:
 *
 *  - `showSkeleton` only ever becomes true `SKELETON_GRACE_MS` after loading
 *    started, so data that lands before that never produces a skeleton frame
 *    to cross-fade away from.
 *  - once content has genuinely been shown, `contentShown` latches true for
 *    the rest of this instance's life — a later `isLoading=true` (a
 *    background refetch) cannot re-arm the grace timer or bring the skeleton
 *    back, which is the #823-class trap this whole feature exists to avoid.
 *
 * `exiting` is true for exactly one `--dur-fast` after loading ends, IF a
 * skeleton was actually shown — that window is what lets the caller overlay
 * a fading skeleton on top of entering content instead of hard-cutting
 * between them. Settled on a timer, not `transitionend`: that event never
 * fires once reduced motion has killed the transition, which would leave
 * `exiting` stuck true for a user who, by definition, never sees one.
 */
function useSkeletonReveal(isLoading: boolean) {
  const graceTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const exitTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const [prevIsLoading, setPrevIsLoading] = useState(isLoading);
  const [showSkeleton, setShowSkeleton] = useState(false);
  const [exiting, setExiting] = useState(false);
  const [contentShown, setContentShown] = useState(!isLoading);

  // React-documented "adjust state while rendering": reacts to `isLoading`
  // changing in THIS commit rather than a tick later via an effect, and is
  // what keeps every direct setState call out of a `useEffect` body below —
  // both effects only ever call setState from inside a timer callback.
  if (isLoading !== prevIsLoading) {
    setPrevIsLoading(isLoading);
    if (!isLoading) {
      setContentShown(true);
      setShowSkeleton(false);
      // `showSkeleton` here is still THIS render's incoming value (the one
      // committed before this adjustment), i.e. "was the skeleton actually
      // on screen for the episode that just ended" — true only for a
      // genuine first load past the grace window, never for a background
      // refetch (which never arms the grace timer at all, see below), so a
      // refetch cannot spuriously replay the exit cross-fade.
      if (showSkeleton && !prefersReducedMotion()) {
        setExiting(true);
      }
    }
  }

  // Arms the grace timer while genuinely (still) loading for the first time;
  // settles it via the timer's OWN callback, never directly in this body.
  // Never re-arms once content has shown once — a background refetch must
  // not bring the skeleton back.
  useEffect(() => {
    clearTimeout(graceTimer.current);
    if (!isLoading || contentShown) return;
    graceTimer.current = setTimeout(() => setShowSkeleton(true), SKELETON_GRACE_MS);
    return () => clearTimeout(graceTimer.current);
  }, [isLoading, contentShown]);

  // Settles `exiting` back to false after --dur-fast, via the timer's OWN
  // callback — never on `transitionend`, which does not fire once reduced
  // motion has killed the transition (and would leave this stuck true).
  useEffect(() => {
    if (!exiting) return;
    exitTimer.current = setTimeout(
      () => setExiting(false),
      readTokenMs('--dur-fast', FALLBACK_DUR_FAST_MS),
    );
    return () => clearTimeout(exitTimer.current);
  }, [exiting]);

  return { showSkeleton, exiting, contentShown };
}

export interface WidgetStateProps {
  lang: LocaleCode;
  isLoading: boolean;
  error: unknown;
  /** True when the fetch succeeded and returned nothing to draw. */
  isEmpty?: boolean;
  /**
   * True when the window is what made it empty — there IS data outside it.
   * Renders the "widen the period" state instead of the "create your first"
   * state, because those have opposite remedies.
   */
  emptyBecauseOfPeriod?: boolean;
  /** Height of the loading skeleton, matched to the widget it stands in for. */
  skeletonHeight?: number;
  retry?: () => void;
  /** first-use copy: what this widget is for, and the action that fills it. */
  emptyTitle?: string;
  emptyDescription?: string;
  emptyIcon?: LucideIcon;
  emptyAction?: ReactNode;
  /** Widen the window — offered on the period-empty state. */
  onWidenPeriod?: () => void;
  children: ReactNode;
}

/**
 * Render `children` when there is data, and the right honest state otherwise.
 *
 * Note what it does NOT do: it never renders `children` with a placeholder
 * value substituted in. A widget either shows data it read or says why it
 * cannot — there is no third rendering where a zero stands in for an answer.
 */
export function WidgetState({
  lang,
  isLoading,
  error,
  isEmpty,
  emptyBecauseOfPeriod,
  skeletonHeight = 160,
  retry,
  emptyTitle,
  emptyDescription,
  emptyIcon,
  emptyAction,
  onWidenPeriod,
  children,
}: WidgetStateProps) {
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const reveal = useSkeletonReveal(isLoading);
  // Never shown content before, and still loading: `error`/`isEmpty` are not
  // meaningful until a fetch has actually finished at least once.
  const stillLoading = isLoading && !reveal.contentShown;

  if (stillLoading && !reveal.showSkeleton) {
    // Inside the grace window: reserve the space, draw nothing. The
    // alternative — showing the skeleton immediately — is exactly the flash
    // this window exists to avoid for cached/near-instant data.
    return <div aria-hidden="true" style={{ height: skeletonHeight }} />;
  }

  let resolved: ReactNode;
  if (stillLoading) {
    resolved = null;
  } else if (error) {
    resolved = isPermissionError(error) ? (
      <EmptyState
        variant="no-permission"
        icon={Lock}
        title={tr('Indicateur non accessible', 'Metric not available to you')}
        description={tr(
          "Votre rôle ne donne pas accès à ces données. Un administrateur de l'organisation peut vous les ouvrir.",
          'Your role does not grant access to this data. An organisation administrator can grant it.',
        )}
        className="py-8"
      />
    ) : (
      <EmptyState
        variant="error"
        icon={AlertTriangle}
        title={tr('Données indisponibles', 'Data unavailable')}
        description={tr(
          "Impossible de lire cet indicateur. Aucune valeur n'est affichée tant qu'elle n'a pas été lue — réessayez, ou contactez un administrateur si cela persiste.",
          'This metric could not be read. Nothing is shown until it has been — retry, or contact an administrator if it persists.',
        )}
        primaryAction={retry ? <Btn label={tr('Réessayer', 'Retry')} onClick={retry} /> : undefined}
        className="py-8"
      />
    );
  } else if (isEmpty && emptyBecauseOfPeriod) {
    resolved = (
      <EmptyState
        variant="no-results"
        icon={CalendarX}
        title={tr('Rien sur cette période', 'Nothing in this period')}
        description={tr(
          'Il existe des données en dehors de la fenêtre sélectionnée. Élargissez la période pour les voir.',
          'There is data outside the selected window. Widen the period to see it.',
        )}
        primaryAction={
          onWidenPeriod ? (
            <Btn label={tr('Voir tout l’historique', 'Show all time')} onClick={onWidenPeriod} />
          ) : undefined
        }
        className="py-8"
      />
    );
  } else if (isEmpty) {
    resolved = (
      <EmptyState
        variant="first-use"
        icon={emptyIcon}
        title={emptyTitle ?? tr('Rien à afficher', 'Nothing to show')}
        description={emptyDescription}
        primaryAction={emptyAction}
        className="py-8"
      />
    );
  } else {
    resolved = <>{children}</>;
  }

  // No overlay needed (the common case: no skeleton was ever shown, e.g.
  // cached data, or the exit window has already elapsed): render directly,
  // no wrapper, no animation — exactly what "renders directly" means for
  // near-instant data.
  if (!stillLoading && !reveal.exiting) {
    return <>{resolved}</>;
  }

  // `stillLoading && reveal.showSkeleton` (genuinely showing the skeleton) and
  // `reveal.exiting` (fading it back out once content has landed) render the
  // SAME wrapper shape on purpose — only the opacity target and whether
  // `resolved` is mounted yet differ. That continuity is what lets the
  // opacity actually transition: a CSS transition only plays across two
  // renders of the SAME node, never on a node's first paint, so if these two
  // states produced differently-shaped trees the "fade" would just be an
  // instant swap. The skeleton fades out on `--motion-exit`
  // (`duration-fast`/`ease-in`); the content fades in under it on
  // `--motion-enter` (`animate-or-fadein` — a keyframe, not a transition, so
  // it correctly plays on ITS first mount, once `resolved` exists). Overlaid
  // rather than stacked so the wrapper's height tracks the content, matching
  // `skeletonHeight`, with nothing shifting underneath the fading skeleton.
  return (
    <div className="relative" style={{ minHeight: skeletonHeight }}>
      <Skeleton
        aria-hidden="true"
        className="absolute inset-0 transition-opacity duration-fast ease-in"
        style={{ height: skeletonHeight, opacity: stillLoading ? 1 : 0 }}
      />
      {!stillLoading && <div className="motion-safe:animate-or-fadein">{resolved}</div>}
    </div>
  );
}
