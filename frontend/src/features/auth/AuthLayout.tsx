// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The split-screen shell shared by every auth screen: sign in, register, forgot
// password, reset password, MFA.
//
// Left: a narrative panel — an abstract, generated visualisation plus a rotating
// attributed quote. Deliberately not a stock photo: a security product whose
// first screen is a smiling stock model in a server room is telling you what it
// thinks of your judgement. The visual is drawn from the same brand geometry as
// the logo.
//
// Right: the form.
//
// MOTION POLICY, applied throughout this file and its children:
//   - nothing runs longer than 400 ms;
//   - entry is a cascade at 80 ms per step;
//   - `prefers-reduced-motion: reduce` disables all of it — not "shortens", not
//     "keeps a subtle version": the orbit stops, the cascade becomes a plain
//     render, and the error shake becomes a static border. That preference is
//     often set because motion causes real symptoms, so a tasteful compromise is
//     still the wrong answer.

import { useEffect, useMemo, useRef, useState } from 'react';
import { Languages, Moon, Sun } from 'lucide-react';

import { useUIStore } from '../../store/uiStore';
import { OpenRiskLogo } from '../../shared/Logo';
import { authCopy, quoteAt } from './authStrings';
import { cascade, usePrefersReducedMotion } from './motion';

/** Nodes on the orbit. Colours are the severity ramp, matching the risk registers. */
const ORBIT_NODES: { color: string; angle: number; radius: number; period: number }[] = [
  { color: 'var(--critical)', angle: 20, radius: 118, period: 26 },
  { color: 'var(--high)', angle: 95, radius: 92, period: 22 },
  { color: 'var(--medium)', angle: 168, radius: 124, period: 30 },
  { color: 'var(--low)', angle: 246, radius: 78, period: 18 },
  { color: 'var(--accent-2)', angle: 310, radius: 110, period: 24 },
];

/**
 * The abstract visual: concentric rings with nodes on them.
 *
 * Rendered as SVG and animated with CSS rotation on the whole group, so it costs
 * one composited transform rather than five per-node animations. The rotation is
 * slow (18–30 s) and continuous — it reads as ambient rather than as something
 * demanding attention, which is what an animation behind a password field has to
 * be.
 */
function RiskOrbit({ reduced }: { reduced: boolean }) {
  return (
    <svg
      viewBox="-140 -140 280 280"
      className="w-full h-full"
      role="presentation"
      aria-hidden="true"
    >
      <defs>
        <radialGradient id="orbit-core">
          <stop offset="0%" stopColor="var(--accent)" stopOpacity="0.95" />
          <stop offset="100%" stopColor="var(--accent-2)" stopOpacity="0.75" />
        </radialGradient>
      </defs>

      {[52, 86, 120].map((r) => (
        // The hero panel is a fixed dark brand surface in BOTH themes (see the
        // wrapper in AuthLayout), so a theme-following token would render this
        // ring invisible in light mode. White at 14% is a hairline on that
        // surface, not a themed colour.
        // eslint-disable-next-line openrisk/no-raw-colors -- brand surface, see above
        <circle key={r} r={r} fill="none" stroke="rgba(255,255,255,.14)" strokeWidth="1" />
      ))}

      {/* Sweep line — the radar reference in the logo, made literal. */}
      {!reduced && (
        <g style={{ animation: 'or-orbit 12s linear infinite', transformOrigin: '0 0' }}>
          <line
            x1="0"
            y1="0"
            x2="126"
            y2="0"
            stroke="var(--accent)"
            strokeWidth="1.5"
            opacity="0.5"
          />
        </g>
      )}

      {ORBIT_NODES.map((n, i) => {
        const rad = (n.angle * Math.PI) / 180;
        const cx = Math.cos(rad) * n.radius;
        const cy = Math.sin(rad) * n.radius;
        return (
          <g
            key={i}
            style={
              reduced
                ? undefined
                : {
                    animation: `or-orbit ${n.period}s linear infinite`,
                    transformOrigin: '0 0',
                  }
            }
          >
            <circle cx={cx} cy={cy} r="6" fill={n.color} opacity="0.95" />
            <circle cx={cx} cy={cy} r="12" fill={n.color} opacity="0.18" />
          </g>
        );
      })}

      <circle r="26" fill="url(#orbit-core)" />
    </svg>
  );
}

// The narrative panel is a fixed dark brand surface in BOTH themes (see the
// wrapper in AuthLayout). Anything drawn on it must therefore be a FIXED light
// colour: a theme-following token such as --fg-primary flips to near-black in
// light mode and the text disappears into the panel. That is exactly what
// happened to the wordmark and the quote — legible in dark, invisible in light.
// eslint-disable-next-line openrisk/no-raw-colors -- brand surface, see above
const ON_BRAND_PANEL = '#ffffff';

/** How long each quote stays up. */
const QUOTE_INTERVAL_MS = 8000;

/**
 * The rotating quote.
 *
 * Rotation starts from the day's index, so everyone signing in on the same day
 * opens on the same line and a reload doesn't reshuffle it. It PAUSES ON HOVER
 * (and on keyboard focus): text that slides away mid-sentence is worse than no
 * rotation at all, and someone who has stopped to read has told you they want it
 * to stay.
 */
function RotatingQuote({ reduced }: { reduced: boolean }) {
  const lang = useUIStore((s) => s.lang);
  const [step, setStep] = useState(0);
  const [visible, setVisible] = useState(true);

  // The day is fixed for the lifetime of the screen; recomputing it per render
  // would make the memo below churn.
  const today = useMemo(() => new Date(), []);
  const quote = quoteAt(step, today);

  // Pause lives in a ref, not state: nothing about the rendered output depends
  // on it — only the interval callback reads it — so making it state would
  // re-render the whole panel on every mouse enter and leave for no visual
  // change. Written from the event handlers, never during render.
  const pausedRef = useRef(false);

  useEffect(() => {
    // With reduced motion the day's quote simply stays put — no crossfade, no
    // rotation.
    if (reduced) return;

    const timer = setInterval(() => {
      if (pausedRef.current) return;
      setVisible(false);
      // Half of the 300 ms crossfade, so the swap happens while invisible.
      setTimeout(() => {
        setStep((s) => s + 1);
        setVisible(true);
      }, 300);
    }, QUOTE_INTERVAL_MS);

    return () => clearInterval(timer);
  }, [reduced]);

  return (
    <figure
      className="relative m-0"
      style={{ minHeight: 104 }}
      onMouseEnter={() => {
        pausedRef.current = true;
      }}
      onMouseLeave={() => {
        pausedRef.current = false;
      }}
      onFocus={() => {
        pausedRef.current = true;
      }}
      onBlur={() => {
        pausedRef.current = false;
      }}
      tabIndex={0}
    >
      <div
        style={{
          opacity: reduced || visible ? 1 : 0,
          transform: reduced || visible ? 'translateY(0)' : 'translateY(6px)',
          transition: reduced ? undefined : 'opacity 300ms ease, transform 300ms ease',
        }}
      >
        <blockquote
          className="text-[17px] font-medium leading-relaxed m-0"
          style={{ letterSpacing: '-.01em', color: ON_BRAND_PANEL }}
        >
          “{lang === 'fr' ? quote.fr : quote.en}”
        </blockquote>
        {/* eslint-disable-next-line openrisk/no-raw-colors -- brand surface, see AuthLayout */}
        <figcaption className="text-[13px] mt-2.5" style={{ color: 'rgba(255,255,255,.55)' }}>
          — {quote.author}
        </figcaption>
      </div>
    </figure>
  );
}

interface AuthLayoutProps {
  children: React.ReactNode;
}

export function AuthLayout({ children }: AuthLayoutProps) {
  const reduced = usePrefersReducedMotion();
  const theme = useUIStore((s) => s.theme);
  const toggleTheme = useUIStore((s) => s.toggleTheme);
  const lang = useUIStore((s) => s.lang);
  const toggleLang = useUIStore((s) => s.toggleLang);
  const copy = authCopy(lang);

  return (
    <div className="flex w-full relative" style={{ minHeight: '100vh' }}>
      {/* Language + theme, available before sign-in. The language choice is
          persisted by uiStore, so it survives the round trip through an OAuth
          provider and the reset link in the email. */}
      <div className="absolute top-5 right-5 z-10 flex items-center gap-2">
        <button
          type="button"
          onClick={toggleLang}
          data-testid="auth-lang-toggle"
          className="h-10 px-3 rounded-[11px] flex items-center gap-1.5 text-ink-muted hover:text-ink transition-colors"
          style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
          title={copy.languageToggle}
          aria-label={copy.languageToggle}
        >
          <Languages size={17} />
          <span className="mono text-[12px] font-semibold uppercase">{lang}</span>
        </button>
        <button
          type="button"
          onClick={toggleTheme}
          data-testid="auth-theme-toggle"
          className="w-10 h-10 rounded-[11px] flex items-center justify-center text-ink-muted hover:text-ink transition-colors"
          style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
          title={copy.themeToggle}
          aria-label={copy.themeToggle}
        >
          {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
        </button>
      </div>

      {/* Narrative panel */}
      <div
        className="relative overflow-hidden flex-col justify-between p-11 hidden md:flex"
        // Brand asset: the narrative panel is intentionally dark in BOTH themes,
        // the way most products' auth split-screens are. Tokenising it would make
        // it flip to a light surface in light mode and destroy the contrast the
        // white type on it depends on. Everything layered on top is exempted for
        // the same reason.
        // eslint-disable-next-line openrisk/no-raw-colors -- brand surface, see above
        style={{ flex: '0 0 45%', background: 'linear-gradient(150deg,#0f0f0f,#1c1c1b)' }}
      >
        <div className="flex items-center gap-2.5 relative" style={cascade(0, reduced)}>
          <div
            className="w-[34px] h-[34px] rounded-[10px] flex items-center justify-center"
            style={{
              color: ON_BRAND_PANEL,
              background: 'var(--accent-solid)',
            }}
          >
            <OpenRiskLogo size={20} />
          </div>
          <div>
            <div
              className="disp text-[19px] font-bold leading-tight"
              style={{ color: ON_BRAND_PANEL }}
            >
              OpenRisk
            </div>
            {/* eslint-disable-next-line openrisk/no-raw-colors -- brand surface, see above */}
            <div className="text-[11.5px]" style={{ color: 'rgba(255,255,255,.45)' }}>
              {copy.productTagline}
            </div>
          </div>
        </div>

        <div
          className="relative flex-1 flex items-center justify-center"
          style={cascade(1, reduced)}
        >
          <div style={{ width: 280, height: 280 }}>
            <RiskOrbit reduced={reduced} />
          </div>
        </div>

        <div className="relative" style={cascade(2, reduced)}>
          <RotatingQuote reduced={reduced} />
        </div>
      </div>

      {/* Form panel */}
      <div
        className="flex-1 flex flex-col items-center justify-center p-8 relative"
        style={{ background: 'var(--bg-app)' }}
      >
        <div className="w-full max-w-[380px]">{children}</div>

        {/* OpenRisk is the product; OpenDefender is the company behind it. The
            link is real — a footer that names a company without letting you go
            and check is decoration. */}
        <div className="absolute bottom-5 left-0 right-0 text-center text-[11.5px] text-ink-muted">
          {copy.footerBy}{' '}
          <a
            href="https://github.com/opendefender"
            target="_blank"
            rel="noreferrer noopener"
            className="font-semibold text-ink-soft hover:text-ink transition-colors underline decoration-dotted underline-offset-2"
          >
            {copy.footerCompany}
          </a>{' '}
          · <span className="opacity-70">© {new Date().getFullYear()}</span>
        </div>
      </div>
    </div>
  );
}

export default AuthLayout;
