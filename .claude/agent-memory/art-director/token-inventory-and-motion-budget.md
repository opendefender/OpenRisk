---
name: token-inventory-and-motion-budget
description: House motion/colour tokens (2026-09-28), the #751 phase 2 micro-interaction budget, and phase 3 decisions (KPI slot-reel counter numbers, skeleton reveal, texts-reveal scope, calendar-grid verdict)
metadata:
  type: project
---

Snapshot 2026-09-28 (verify in frontend/src/styles/{primitives,theme,tokens}.css before citing).

Durations: --dur-instant 90 · --dur-fast 120 · --dur-base 180 · --dur-slow 260 · --dur-panel 400 (panels only).
Curves: --ease-out (0.2,0.8,0.2,1; default enter/settle) · --ease-in (exit) · --ease-inout · --ease-emphasized
(overshoot 1.06, reserved for THE signature move; the signature is the 2px accent KEYLINE; the
curve had zero usages in src at this date — do not spend it on feedback micro-interactions).
Shorthands: --motion-hover/press/enter/exit/panel.
Intents: success / -surface / -text / -solid; danger same four; accent / -solid / -soft / -line;
--fg-on-solid white. White on --danger-solid #c81e14 ~5.8:1, on --success-solid #1a7f3c ~5.1:1.
Risk tokens: --risk-low/moderate/high/critical/extreme (tokens.css, dark + light sets).
Skeleton: one system only, .or-skeleton shimmer in index.css (1.4s linear infinite).
Global reduced-motion rule in index.css kills every animation+transition.
Existing shake: or-shake keyframe, 4px, used inline at 300ms `ease` in features/auth/fields.tsx.

Motion budget for anything used dozens of times an hour (decided #751 phase 2):
- max 180ms enter, 120ms exit, 260ms only for one-shot confirmations; translate <= 4px;
  scale >= 0.96; NO blur, NO rotation, NO overshoot, NO particles/glow, NO per-frame rAF JS.
- SVG draws use pathLength="1" (no getTotalLength calibration).
- Success check fires on server ack only, never on optimistic update or client-side Zod validity.
- Errors never auto-revert on a timer; they persist until the value is fixed.
- Checkbox glyph stays accent (selection), not success green (compliance meaning).

Phase 3 decisions (2026-09-28, owner D-060 = A slot-reel on 3 KPIs):
- Reel: per column var(--dur-panel) 400ms, --ease-out, 1 spin max, 40ms left-to-right stagger
  (proposed new token --stagger-step: 40ms); total window <= 520ms from data arrival. Motion blur
  DROPPED. Edge mask only while rolling. Columns fixed from target at t0, separators static.
  aria-label carries the real value; data-settled attr; print + reduced-motion = final value.
  Settle must not depend on transitionend (reduced motion kills the transition).
- Skeleton reveal: keep house shimmer, drop recipe pulse and blur; opacity cross-fade only,
  content in --motion-enter, skeleton out --motion-exit; no inter-widget stagger; skip fade on cache.
- Texts reveal: dropped on the dashboard; allowed only on empty states/onboarding, 4px, no blur.
- Contribution-style calendar grid: fit for incident/finding event density over time, NOT for
  risk (risk is a state; the P x I matrix in shared/ds/RiskMatrix.tsx stays). Future issue only.

**Why:** dense regulator-facing GRC console; the owner favours premium polish (see
po-openrisk memory owner-overrides-on-scope-and-polish), so frame cuts as risk (a11y, IME/RTL,
truth, perf), never as "less". Board-deck screenshots make the reel's wrong-figure window a truth cost.
**How to apply:** use as the default numbers when reviewing any recipe from transitions.dev or
visual reference (RareUI is reference-only, D-059).
