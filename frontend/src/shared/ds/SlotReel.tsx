// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * SlotReel — the in-house rolling-digit counter for KPIs and gauge values.
 *
 * D-060 (2026-09-28, #751 phase 3): an in-house slot-reel replaces
 * `@number-flow/react`, which stays banned as a third-party dependency (see
 * `eslint.config.js`'s `no-restricted-imports` message).
 *
 * WHY THE NUMBER CAN NEVER BE WRONG   Every digit column is positioned at its
 * FINAL row on every single render, via a CSS `transform` computed straight
 * from `value` (or, for exactly one render right after a `rollOnMount` mount,
 * from `0` — see below) — there is no intermediate JS-counted number anywhere
 * in this component. A CSS transition only plays when a property's computed
 * value changes between two already-PAINTED renders; a fresh mount with a
 * value already in hand and `rollOnMount` unset (or absent) renders plain,
 * with no roll — a `value` that changes while the component stays mounted is
 * what rolls, old digit position to new. That is "value identity through a
 * ref, never mount": the digit position is a pure function of props, so
 * there's nothing for a remount to get wrong.
 *
 * ROLLING ON MOUNT   `rollOnMount` is for the one case that genuinely needs a
 * mount-time roll — real data landing for the first time on this page load,
 * as opposed to a value already sitting in the cache from an earlier visit
 * (D-060's "for about a second after load" cost the owner explicitly
 * accepted). The caller decides which is which — typically a TanStack Query
 * result's `isFetchedAfterMount` — because only the caller knows whether ITS
 * fetch actually ran during this mount or was served from cache. The prop is
 * read exactly once, via a lazy `useState` initializer: a LATER flip of the
 * prop (the query resolving after mount, a parent re-render) can never
 * retroactively start or cancel a roll on an instance that already decided.
 *
 * TRUTH GUARD   The digit strip is `aria-hidden`; the number a screen reader
 * or `@media print` sees is a plain `Intl.NumberFormat` string taken from
 * `value` on every render — never from animation state, because there is no
 * animation state to leak an intermediate value from.
 *
 * MOTION BUDGET   Digit columns roll over `--dur-panel`, `--ease-out`,
 * starting left to right `--stagger-step` apart, capped at three staggered
 * columns (0/1/2/3 steps) so the total never exceeds `--dur-panel` + 3 ×
 * `--stagger-step` = 520ms regardless of how many digits a value has. Reduced
 * motion is covered by the app-wide `* { transition: none !important }` rule
 * in index.css AND, belt and braces, by `motion-reduce:hidden` on the strip
 * itself (paired with `motion-reduce:not-sr-only` on the plain fallback) so a
 * reduced-motion user never has the ten-digit strip in their read order.
 */

import { useEffect, useRef, useState } from 'react';

const DIGITS = ['0', '1', '2', '3', '4', '5', '6', '7', '8', '9'] as const;

/** The last (rightmost) column's transition-delay caps at this many
 *  `--stagger-step`s, which is what keeps the total roll inside the ≤520ms
 *  budget no matter how many digits the value has. */
const MAX_STAGGER_COLUMNS = 3;

/* NOT the source of truth — primitives.css is. These are read back from the
 * DOM (see useDensityRowHeight for the same pattern) so the settle timer
 * cannot drift from what the CSS transition actually does; they only stand in
 * when the DOM can't be measured (SSR) or a token has gone missing. */
const FALLBACK_DUR_PANEL_MS = 400;
const FALLBACK_STAGGER_STEP_MS = 40;

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

type ReelPart =
  | { kind: 'literal'; key: string; text: string }
  | { kind: 'digit'; key: string; text: string; colIndex: number };

/** Splits a formatted number into one column per DIGIT and one static node
 *  per separator/sign/percent-sign — those come from `Intl.NumberFormat` and
 *  never move. A multi-digit `integer`/`fraction` part (e.g. a "42" fraction,
 *  or an ungrouped "1234") is split further, one column per character, so
 *  each digit gets its own roll. */
function toParts(value: number, formatter: Intl.NumberFormat): ReelPart[] {
  let colIndex = 0;
  const out: ReelPart[] = [];
  formatter.formatToParts(value).forEach((part, i) => {
    if (part.type === 'integer' || part.type === 'fraction') {
      for (const ch of part.value) {
        out.push({ kind: 'digit', key: `d-${i}-${colIndex}`, text: ch, colIndex: colIndex++ });
      }
    } else {
      out.push({ kind: 'literal', key: `l-${i}`, text: part.value });
    }
  });
  return out;
}

export interface SlotReelProps {
  /** The real number. Always what settles — there is no other truth. */
  value: number;
  /**
   * Roll every column from 0 the moment this instance first paints, instead
   * of rendering `value` plainly. For fresh data landing on this page load —
   * pass the caller's own signal for "did I actually fetch, or was this
   * cache", e.g. a TanStack Query result's `isFetchedAfterMount`. Read once,
   * at mount; changing it later has no effect. Default false: mounting with
   * a value already in hand renders plain.
   */
  rollOnMount?: boolean;
  /** BCP-47 locale tag. Omit to use the runtime default, matching a bare
   *  `.toLocaleString()`. */
  locale?: string;
  formatOptions?: Intl.NumberFormatOptions;
  className?: string;
}

export function SlotReel({
  value,
  rollOnMount = false,
  locale,
  formatOptions,
  className,
}: SlotReelProps) {
  const formatter = new Intl.NumberFormat(locale, formatOptions);
  const formatted = formatter.format(value);
  const parts = toParts(value, formatter);

  const timerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const mountTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const [phase, setPhase] = useState<'settled' | 'rolling'>('settled');
  // Lazily seeded to the FIRST value this instance ever renders — never
  // touched by an effect, so a genuine remount (a fresh `renderedValue`
  // matching whatever `value` already is) can never be mistaken for a change
  // to roll. This is "value identity through a ref, never mount": the thing
  // being compared is the value itself, not whether the component happened
  // to (re)mount.
  const [renderedValue, setRenderedValue] = useState(value);
  // Also lazy, also read once: whether THIS instance is mid mount-roll. A CSS
  // transition needs two genuinely PAINTED states to animate between, so this
  // can't be done the way a later value change is (adjusting state during
  // render, which collapses into a single commit and never paints the "0"
  // frame at all) — the digit strip below renders literal '0's for every
  // column while this is true, and an effect flips it after that first paint
  // has actually happened.
  const [mountRoll, setMountRoll] = useState(() => rollOnMount && !prefersReducedMotion());

  useEffect(() => {
    if (!mountRoll) return;
    // A timer, not a bare effect-body setState (the latter is a hard lint
    // error in this repo — see the eslint-plugin-react-hooks v7
    // `set-state-in-effect` rule — and, separately, would fire before the
    // browser has painted the '0' frame at all, since effects that run
    // synchronously with no async boundary happen before paint just as
    // reliably as "adjust state while rendering" does). 0ms is enough: a
    // `setTimeout` is a macrotask, so the browser paints the pending '0'
    // frame before this callback ever runs.
    mountTimerRef.current = setTimeout(() => {
      setMountRoll(false);
      setRenderedValue(value);
      setPhase('rolling');
    }, 0);
    return () => clearTimeout(mountTimerRef.current);
  }, [mountRoll, value]);

  // React-documented "adjust state while rendering": lands in THIS commit,
  // not a tick later via an effect, and is what keeps every direct setState
  // call out of a `useEffect` body — only the settle-back calls run inside
  // one, and always inside a timer callback, never the effect body itself.
  // Skipped for as long as the mount-roll above is still pending: that path
  // owns the transition from 0 to `value` on its own timer.
  if (!mountRoll && value !== renderedValue) {
    setRenderedValue(value);
    if (!prefersReducedMotion()) {
      setPhase('rolling');
    }
  }

  useEffect(() => {
    if (phase !== 'rolling') return;
    // Never settle on `transitionend` — it does not fire once the app-wide
    // reduced-motion rule has killed the transition, which would leave
    // `data-rolling` stuck true forever for a user who, by definition, never
    // sees a `transitionend` for this transform. A timer sized off the same
    // tokens the CSS uses settles correctly either way.
    const total =
      readTokenMs('--dur-panel', FALLBACK_DUR_PANEL_MS) +
      MAX_STAGGER_COLUMNS * readTokenMs('--stagger-step', FALLBACK_STAGGER_STEP_MS);
    timerRef.current = setTimeout(() => setPhase('settled'), total);
    return () => clearTimeout(timerRef.current);
  }, [phase]);

  // Reduced motion always overrides `phase` at render time — belt and braces
  // with the render-time adjustment above skipping the roll in the first
  // place, and what makes this correct even in the pathological case of the
  // OS preference changing mid-roll (a stale `phase='rolling'` can never
  // reach the DOM as long as this override is unconditional on every render,
  // not just at the moment a value change was detected).
  const reducedNow = prefersReducedMotion();
  const rolling = phase === 'rolling' && !reducedNow;
  const settled = phase === 'settled' || reducedNow;

  return (
    <span
      className={className}
      data-testid="slot-reel"
      data-rolling={rolling ? 'true' : undefined}
      data-settled={settled ? 'true' : 'false'}
    >
      <span
        aria-hidden="true"
        className="inline-flex tabular-nums print:hidden motion-reduce:hidden"
        style={
          rolling
            ? {
                maskImage:
                  'linear-gradient(to bottom, transparent 0, black 25%, black 75%, transparent 100%)',
                WebkitMaskImage:
                  'linear-gradient(to bottom, transparent 0, black 25%, black 75%, transparent 100%)',
              }
            : undefined
        }
      >
        {parts.map((part) =>
          part.kind === 'literal' ? (
            <span key={part.key}>{part.text}</span>
          ) : (
            <span
              key={part.key}
              className="relative inline-block overflow-hidden align-bottom"
              style={{ height: '1em', width: '1ch' }}
            >
              <span
                className="absolute inset-x-0 top-0 flex flex-col transition-transform duration-panel ease-out"
                style={{
                  // Every column starts at '0' during the mount-roll window
                  // (the structure — how many columns, where the separators
                  // sit — still comes from the TARGET value's formatting, per
                  // the spec's "column count is fixed from the target
                  // value"; only which digit each column shows is 0 here).
                  transform: `translateY(${-Number(mountRoll ? '0' : part.text)}em)`,
                  transitionDelay: `calc(var(--stagger-step) * ${Math.min(part.colIndex, MAX_STAGGER_COLUMNS)})`,
                }}
              >
                {DIGITS.map((d) => (
                  <span key={d} className="leading-none" style={{ height: '1em' }}>
                    {d}
                  </span>
                ))}
              </span>
            </span>
          ),
        )}
      </span>
      {/* The one place the real number always lives: hidden on screen, shown
          to a screen reader and to `@media print` (a printed/screenshotted
          page must never carry whatever digit the reel happened to be
          mid-roll on). */}
      <span
        data-testid="slot-reel-value"
        className="sr-only print:not-sr-only motion-reduce:not-sr-only"
      >
        {formatted}
      </span>
    </span>
  );
}
