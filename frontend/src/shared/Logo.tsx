// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// OpenRisk brand mark — a shield enclosing a radar target (ring + center dot),
// evoking "risk radar / defense". Filled with currentColor; the ring reads as
// negative space (evenodd), so it looks crisp on the gradient brand tiles and in
// monochrome. Replaces the generic lucide Shield used as the app's logo.

interface LogoProps {
  size?: number;
  className?: string;
  title?: string;
}

// Single evenodd path: shield (fill) → outer ring (hole) → center dot (fill).
const LOGO_PATH =
  'M12 2 L20 5 V11 C20 16 16.4 19.2 12 21.4 C7.6 19.2 4 16 4 11 V5 Z ' +
  'M8 12 a4 4 0 1 1 8 0 a4 4 0 1 1 -8 0 Z ' +
  'M10.7 12 a1.3 1.3 0 1 1 2.6 0 a1.3 1.3 0 1 1 -2.6 0 Z';

export function OpenRiskLogo({ size = 20, className, title = 'OpenRisk' }: LogoProps) {
  return (
    <svg viewBox="0 0 24 24" width={size} height={size} className={className} role="img" aria-label={title}>
      <title>{title}</title>
      <path d={LOGO_PATH} fill="currentColor" fillRule="evenodd" clipRule="evenodd" />
    </svg>
  );
}

export default OpenRiskLogo;
