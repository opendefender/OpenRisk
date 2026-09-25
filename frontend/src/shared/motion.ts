// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The only way into framer-motion (#751). Import `motion`, `AnimatePresence` and
// friends from here, never from 'framer-motion' directly — eslint enforces it.
//
// Why a module and not <MotionConfig reducedMotion="user"> at the root: the root
// is on the preloaded path, and framer-motion lives in its own lazy `motion`
// chunk (vite.config.ts). A provider in main.tsx would pull that whole chunk into
// the initial bundle. This module is only ever imported by code that already
// imports framer-motion, so it costs nothing until motion is actually needed.
//
// Under `prefers-reduced-motion: reduce` every framer-motion animation is
// skipped: the value jumps to its final keyframe and exit animations complete
// at once. That is the same contract as the global CSS rule in index.css and
// features/auth/motion.ts — reduced motion means none, not a shorter version.
// It also covers opacity, which MotionConfig's "user" mode would still animate.

// eslint-disable-next-line no-restricted-imports -- this module is the wrapper the rule points to
import { MotionGlobalConfig } from 'framer-motion';

const REDUCED_MOTION_QUERY = '(prefers-reduced-motion: reduce)';

/**
 * Point framer-motion at the OS preference and keep following it, so turning
 * the setting on mid-session takes effect without a reload. Runs once, when the
 * motion chunk is first evaluated; exported for the test.
 */
export function syncReducedMotion(): void {
  if (typeof window === 'undefined' || !window.matchMedia) return;
  const mq = window.matchMedia(REDUCED_MOTION_QUERY);
  MotionGlobalConfig.skipAnimations = mq.matches;
  mq.addEventListener('change', (e: MediaQueryListEvent) => {
    MotionGlobalConfig.skipAnimations = e.matches;
  });
}

syncReducedMotion();

// eslint-disable-next-line no-restricted-imports -- this module is the wrapper the rule points to
export { AnimatePresence, motion, type Variants } from 'framer-motion';
