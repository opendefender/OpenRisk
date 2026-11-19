// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The 5-level pickers of the drawer and the new-risk modal, and the 5×5 matrix,
// all read ONE binning rule. Our probability is 0–1 and our impact 0–10; a
// "level" k (1–5) is a bucket of width 1/5 of the range.
//
// Writing a level stores the MIDPOINT of its bucket, so that the value read
// back lands in the same bucket (and the picker shows the level that was
// clicked):  P = (2k − 1) / 10   ·   I = 2k − 1.
//
// None of this is the score. The score stays P × I × AssetCriticality, computed
// by the server; levels are only a way to enter P and I.

import type { MatrixBucket } from '../../shared/ds';
import type { Criticality } from '../../shared/riskColors';

export const LEVELS: readonly MatrixBucket[] = [1, 2, 3, 4, 5];

/** Bin a value of [0, max] into 1..5. Clamped, so a stray 0 or overflow is safe. */
export const toBucket = (v: number, max: number): MatrixBucket =>
  Math.min(5, Math.max(1, Math.ceil((v / max) * 5) || 1)) as MatrixBucket;

export const probabilityLevel = (p: number): MatrixBucket => toBucket(p, 1);
export const impactLevel = (i: number): MatrixBucket => toBucket(i, 10);

/** Midpoint of probability bucket k: 0.1, 0.3, 0.5, 0.7, 0.9. */
export const probabilityForLevel = (k: MatrixBucket): number => (2 * k - 1) / 10;

/** Midpoint of impact bucket k: 1, 3, 5, 7, 9. */
export const impactForLevel = (k: MatrixBucket): number => 2 * k - 1;

/**
 * Tint of a CELL of the 5×5 grid, from its own coordinates (bucket × bucket,
 * 1–25). This is NOT a score band: no risk's score or label is derived from it.
 * Markers carry the risk's real, server-assigned band.
 */
export function cellCrit(pBucket: number, iBucket: number): Criticality {
  const v = pBucket * iBucket;
  return v >= 15 ? 'critical' : v >= 8 ? 'high' : v >= 4 ? 'medium' : 'low';
}
