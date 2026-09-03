// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Live password strength meter.
//
// Two-tier by design. zxcvbn runs in the browser on every keystroke so the bar
// moves as you type; the server is then asked (debounced) for the verdict that
// actually decides, because it knows two things the browser cannot: whether the
// password appears in the HaveIBeenPwned corpus, and what the enforced policy
// is right now. The client estimate is a preview — never the gate.

import { useEffect, useMemo, useRef, useState } from 'react';

import { useUIStore } from '../../store/uiStore';
import { authCopy, strengthLabel } from './authStrings';
import { checkPassword, reasonText, type PasswordAssessment } from './authService';

// zxcvbn's ranked dictionaries are ~1 MB gzipped — far too heavy to sit in the
// initial bundle, since the login page (the app's very first paint) would pay for
// them even though the meter only matters on sign-up. So the estimator is loaded
// ON DEMAND, the first time a password is actually typed, as its own async chunk.
// Until it resolves, the meter falls back to the (authoritative) server verdict.
type Estimator = { check: (pw: string, inputs?: string[]) => { score: number } };
let estimatorPromise: Promise<Estimator> | null = null;
function loadEstimator(): Promise<Estimator> {
  if (!estimatorPromise) {
    estimatorPromise = Promise.all([
      import('@zxcvbn-ts/core'),
      import('@zxcvbn-ts/language-common'),
      import('@zxcvbn-ts/language-en'),
      import('@zxcvbn-ts/language-fr'),
    ]).then(
      ([core, common, en, fr]) =>
        new core.ZxcvbnFactory({
          // Both language dictionaries: a French user may pick an English word and
          // vice versa; the dictionary must catch it whichever language it came from.
          graphs: common.adjacencyGraphs,
          dictionary: { ...common.dictionary, ...en.dictionary, ...fr.dictionary },
        }),
    );
  }
  return estimatorPromise;
}

/** Mirrors pkg/pwpolicy.MinScore — the bar the server enforces. */
const MIN_SCORE = 3;

/** Mirrors pkg/pwpolicy — kept in sync via the server assessment it returns. */
const MIN_LENGTH = 12;

interface Props {
  password: string;
  /** Identity strings fed to zxcvbn so a password built from the account scores low. */
  email?: string;
  name?: string;
  /** Called whenever the acceptability verdict changes. */
  onVerdict?: (acceptable: boolean) => void;
  /** Server assessment to display instead of asking again (e.g. a 422 body). */
  override?: PasswordAssessment | null;
}

export function PasswordStrength({ password, email, name, onVerdict, override }: Props) {
  const lang = useUIStore((s) => s.lang);
  const copy = authCopy(lang);

  // The server verdict is stored WITH the password it was computed for.
  //
  // That pairing is what makes staleness derivable: on every keystroke the
  // stored `forPassword` stops matching and the result is simply ignored. The
  // alternative — clearing it from an effect — means a synchronous setState in
  // the effect body, an extra render pass, and a window where the old verdict is
  // still on screen under the new password.
  const [server, setServer] = useState<{ forPassword: string; result: PasswordAssessment } | null>(
    null,
  );
  const abortRef = useRef<AbortController | null>(null);

  // --- Client estimate: instant once the estimator has loaded --------------
  // Kick off the (lazy) dictionary load the first time there is a password.
  const [estimator, setEstimator] = useState<Estimator | null>(null);
  useEffect(() => {
    if (password && !estimator) {
      loadEstimator()
        .then(setEstimator)
        .catch(() => {
          /* offline / chunk load failed — server verdict still gates submit */
        });
    }
  }, [password, estimator]);

  const local = useMemo(() => {
    if (!password || !estimator) return null;
    const inputs = [email, name].filter((v): v is string => Boolean(v));
    return estimator.check(password, inputs);
  }, [password, email, name, estimator]);

  // --- Server verdict: debounced, authoritative ----------------------------
  useEffect(() => {
    if (override || !password) return; // nothing to ask about

    // Cancel the previous in-flight check: without this, a fast typist's earlier
    // request can land after a later one and show a stale verdict.
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    const timer = setTimeout(() => {
      checkPassword(password, { email, name }, controller.signal)
        .then((result) => setServer({ forPassword: password, result }))
        .catch(() => {
          // Offline or the endpoint is unreachable: keep the local estimate on
          // screen rather than blanking the meter. Submit is still gated by the
          // server, so nothing weak gets through on the strength of this.
        });
    }, 350);

    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [password, email, name, override]);

  // Only honour a verdict computed for the password currently in the field.
  const fresh = server?.forPassword === password ? server.result : null;
  const assessment = override ?? fresh;

  // Derived rather than stored: we are waiting exactly when there is something to
  // ask about and no answer for it yet.
  const checking = !override && password.length > 0 && fresh === null;

  // Prefer the server's score once we have one; fall back to the local estimate.
  const score = assessment?.score ?? local?.score ?? 0;
  const longEnough = password.length >= MIN_LENGTH;
  const acceptable = assessment ? assessment.ok : score >= MIN_SCORE && longEnough;

  useEffect(() => {
    onVerdict?.(acceptable);
  }, [acceptable, onVerdict]);

  if (!password) return null;

  const blocking = assessment?.blocking ?? [];
  const advisory = assessment?.advisory ?? [];

  return (
    <div className="mt-2" aria-live="polite">
      {/* Four segments, matching zxcvbn's 0..4 scale. */}
      <div
        className="flex gap-1.5"
        role="img"
        aria-label={`${copy.strengthLabel}: ${strengthLabel(copy, score)}`}
      >
        {[0, 1, 2, 3].map((i) => (
          <div
            key={i}
            className="flex-1 h-1 rounded transition-colors"
            style={{
              background:
                i < score ? (acceptable ? 'var(--low)' : 'var(--high)') : 'var(--bg-hover)',
              // Bounded well under the 400 ms ceiling; a strength bar that lags
              // the keystroke reads as unresponsive.
              transitionDuration: '180ms',
            }}
          />
        ))}
      </div>

      <div className="flex items-center justify-between mt-1.5 text-[11.5px]">
        <span style={{ color: acceptable ? 'var(--low)' : 'var(--fg-muted)' }}>
          {copy.strengthLabel} : {strengthLabel(copy, score)}
        </span>
        {checking && <span className="text-ink-muted">{copy.strengthChecking}</span>}
      </div>

      {/* Actionable guidance — the whole point. "Invalid" produces retry loops;
          "add another word" tells someone what to do next. */}
      {blocking.length > 0 && (
        <ul className="mt-1.5 space-y-1">
          {blocking.map((r) => (
            <li
              key={r.code}
              className="text-[11.5px] leading-snug"
              style={{ color: 'var(--high)' }}
            >
              {reasonText(r, lang)}
            </li>
          ))}
        </ul>
      )}

      {blocking.length === 0 && advisory.length > 0 && (
        <ul className="mt-1.5 space-y-1">
          {advisory.map((r) => (
            <li key={r.code} className="text-[11.5px] leading-snug text-ink-muted">
              {reasonText(r, lang)}
            </li>
          ))}
        </ul>
      )}

      {/* Only shown when the corpus was actually consulted. On a HIBP outage the
          server reports breach_check_skipped and we stay quiet rather than
          implying the password was cleared. */}
      {assessment?.breach_check_skipped && blocking.length === 0 && (
        <div className="mt-1.5 text-[11px] text-ink-muted">
          {lang === 'fr'
            ? 'Vérification des fuites indisponible pour le moment.'
            : 'Breach check unavailable right now.'}
        </div>
      )}
    </div>
  );
}

export default PasswordStrength;
