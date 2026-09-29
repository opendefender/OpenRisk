// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #751 phase 4, review fix (qa-automation, measured live) — the Demo and
// Offline banners render severity text at 12.5px on a 16% tint over
// --surface-0, and the RAW severity tokens measured under WCAG AA (4.5:1)
// there in the light theme (Demo/--high 3.99:1) and just barely cleared it
// in dark (Offline/--critical 4.68:1, "tight"). The fix mixes the banner
// TEXT toward the theme's own body colour (--fg-primary) rather than
// touching --high/--critical/--medium themselves, which stay the shared
// severity tokens (a design-system decision, not this file's to make).
//
// This reads the REAL `color:` style values out of DemoBanner.tsx /
// OfflineBanner.tsx (not a formula re-typed here from memory) and the real
// token hex values out of tokens.css, so reverting either component's fix —
// or a future token change that reintroduces a failing ratio — fails this
// test rather than shipping unnoticed.

import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, it, expect } from 'vitest';

// ---- sRGB / relative luminance / WCAG contrast ----------------------------

function hexToRgb(hex: string): [number, number, number] {
  const n = parseInt(hex.replace('#', ''), 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

function rgbToHex([r, g, b]: readonly number[]): string {
  const c = (x: number) =>
    Math.max(0, Math.min(255, Math.round(x)))
      .toString(16)
      .padStart(2, '0');
  return `#${c(r)}${c(g)}${c(b)}`;
}

function srgbToLinear(c: number): number {
  const cs = c / 255;
  return cs <= 0.04045 ? cs / 12.92 : Math.pow((cs + 0.055) / 1.055, 2.4);
}

function linearToSrgb(c: number): number {
  return c <= 0.0031308 ? c * 12.92 * 255 : (1.055 * Math.pow(c, 1 / 2.4) - 0.055) * 255;
}

function relLuminance(hex: string): number {
  const [r, g, b] = hexToRgb(hex).map(srgbToLinear);
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

/** WCAG contrast ratio between two opaque colours. */
function contrastRatio(hexA: string, hexB: string): number {
  const la = relLuminance(hexA);
  const lb = relLuminance(hexB);
  const [lighter, darker] = la > lb ? [la, lb] : [lb, la];
  return (lighter + 0.05) / (darker + 0.05);
}

/** `color-mix(in srgb, fg pct%, transparent)` composited over an opaque bg. */
function mixOverBg(hexFg: string, pct: number, hexBg: string): string {
  const fg = hexToRgb(hexFg);
  const bg = hexToRgb(hexBg);
  const a = pct / 100;
  return rgbToHex(fg.map((c, i) => c * a + bg[i] * (1 - a)));
}

// ---- OKLab, for `color-mix(in oklab, ...)` --------------------------------

function linRgbToOklab([r, g, b]: readonly number[]): [number, number, number] {
  const l = 0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b;
  const m = 0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b;
  const s = 0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b;
  const [l_, m_, s_] = [Math.cbrt(l), Math.cbrt(m), Math.cbrt(s)];
  return [
    0.2104542553 * l_ + 0.793617785 * m_ - 0.0040720468 * s_,
    1.9779984951 * l_ - 2.428592205 * m_ + 0.4505937099 * s_,
    0.0259040371 * l_ + 0.7827717662 * m_ - 0.808675766 * s_,
  ];
}

function oklabToLinRgb([L, a, b]: readonly number[]): [number, number, number] {
  const l_ = L + 0.3963377774 * a + 0.2158037573 * b;
  const m_ = L - 0.1055613458 * a - 0.0638541728 * b;
  const s_ = L - 0.0894841775 * a - 1.291485548 * b;
  const [l, m, s] = [l_ ** 3, m_ ** 3, s_ ** 3];
  return [
    4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
    -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
    -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s,
  ];
}

/** `color-mix(in oklab, hexA pctA%, hexB)`. */
function mixOklab(hexA: string, pctA: number, hexB: string): string {
  const a = linRgbToOklab(hexToRgb(hexA).map(srgbToLinear));
  const b = linRgbToOklab(hexToRgb(hexB).map(srgbToLinear));
  const t = pctA / 100;
  const mixed = a.map((v, i) => v * t + b[i] * (1 - t)) as [number, number, number];
  return rgbToHex(oklabToLinRgb(mixed).map(linearToSrgb));
}

// ---- Real token values, read from disk ------------------------------------

/** A `:root[data-theme='...']` block's custom properties, unresolved. */
function parseThemeBlock(css: string, blockPattern: RegExp): Map<string, string> {
  const block = css.match(blockPattern)?.[0];
  if (!block) {
    throw new Error(`Theme block not found for ${blockPattern} — did tokens.css change shape?`);
  }
  const props = new Map<string, string>();
  for (const m of block.matchAll(/--([\w-]+):\s*([^;]+);/g)) {
    props.set(m[1], m[2].trim());
  }
  return props;
}

/** Resolves `var(--x)` chains to a literal hex within one theme block. */
function resolveToken(name: string, props: Map<string, string>): string {
  const value = props.get(name);
  if (!value) throw new Error(`Token --${name} not found in this theme block.`);
  const varMatch = value.match(/^var\(--([\w-]+)\)$/);
  return varMatch ? resolveToken(varMatch[1], props) : value;
}

function loadThemeTokens() {
  const css = readFileSync(path.resolve(__dirname, '../../styles/tokens.css'), 'utf8');
  // The dark theme is the bare :root (also matched by :root[data-theme='dark']);
  // grabbing up to the first closing brace is safe here — these blocks hold
  // only flat `--name: value;` declarations, no nested braces.
  const dark = parseThemeBlock(css, /:root,\s*:root\[data-theme='dark'\]\s*\{[^}]*\}/);
  const light = parseThemeBlock(css, /:root\[data-theme='light'\]\s*\{[^}]*\}/);
  const pick = (props: Map<string, string>) => ({
    surface0: resolveToken('surface-0', props),
    fgPrimary: resolveToken('fg-primary', props),
    high: resolveToken('high', props),
    critical: resolveToken('critical', props),
    medium: resolveToken('medium', props),
  });
  return { light: pick(light), dark: pick(dark) };
}

// ---- The banner components' REAL style values, read from disk -------------
//
// Reads the actual `color:` style string out of the component source. If a
// change reverts either component to the bare `var(--severity)` it used
// before this fix, `styleColorRatio` below falls back to treating the raw
// token as the text colour — reproducing the pre-fix (failing) ratio rather
// than silently passing.

function readSource(relativePath: string): string {
  return readFileSync(path.resolve(__dirname, '../..', relativePath), 'utf8');
}

/** The single `color: '...'` style value in DemoBanner.tsx. */
function demoBannerColorStyle(): string {
  const source = readSource('shared/DemoBanner.tsx');
  const match = source.match(/\bcolor:\s*'([^']+)'/);
  if (!match) throw new Error("DemoBanner.tsx's color: style not found — did it move?");
  return match[1];
}

/** OfflineBanner.tsx's `color: shown.offline ? '<a>' : '<b>'` ternary. */
function offlineBannerColorStyles(): { offline: string; degraded: string } {
  const source = readSource('shared/OfflineBanner.tsx');
  const block = source.match(/\bcolor:\s*shown\.offline([\s\S]*?),\s*\n\s*borderBottom/);
  if (!block) throw new Error("OfflineBanner.tsx's color: ternary not found — did it move?");
  const strings = [...block[1].matchAll(/'([^']+)'/g)].map((m) => m[1]);
  if (strings.length < 2) {
    throw new Error(`Expected two color strings (offline, degraded), found ${strings.length}.`);
  }
  return { offline: strings[0], degraded: strings[1] };
}

/**
 * Computes the contrast ratio a `color:` style value actually produces
 * against the component's 16%-tint background over --surface-0. Understands
 * `color-mix(in oklab, var(--x) P%, var(--fg-primary))` (the fix); anything
 * else (a bare `var(--x)`, the pre-fix shape) is treated as the raw
 * severity colour with no mix at all.
 */
function styleColorRatio(
  styleValue: string,
  severityHex: string,
  surface0: string,
  fgPrimary: string,
): number {
  const mixMatch = styleValue.match(
    /color-mix\(in oklab,\s*var\(--[\w-]+\)\s*(\d+)%,\s*var\(--fg-primary\)\)/,
  );
  const text = mixMatch ? mixOklab(severityHex, Number(mixMatch[1]), fgPrimary) : severityHex;
  const bg = mixOverBg(severityHex, 16, surface0);
  return contrastRatio(text, bg);
}

const AA_SMALL_TEXT = 4.5;

describe('banner text contrast (#751 phase 4 review fix)', () => {
  const tokens = loadThemeTokens();
  const demoColor = demoBannerColorStyle();
  const offlineColors = offlineBannerColorStyles();

  it.each([
    ['light', 'Demo banner (--high)', demoColor, tokens.light.high, tokens.light],
    [
      'light',
      'Offline banner, offline (--critical)',
      offlineColors.offline,
      tokens.light.critical,
      tokens.light,
    ],
    [
      'light',
      'Offline banner, degraded (--medium)',
      offlineColors.degraded,
      tokens.light.medium,
      tokens.light,
    ],
    ['dark', 'Demo banner (--high)', demoColor, tokens.dark.high, tokens.dark],
    [
      'dark',
      'Offline banner, offline (--critical)',
      offlineColors.offline,
      tokens.dark.critical,
      tokens.dark,
    ],
    [
      'dark',
      'Offline banner, degraded (--medium)',
      offlineColors.degraded,
      tokens.dark.medium,
      tokens.dark,
    ],
  ] as const)(
    '%s theme, %s meets AA (>=4.5:1) with margin',
    (_theme, _label, styleValue, severityHex, theme) => {
      const ratio = styleColorRatio(styleValue, severityHex, theme.surface0, theme.fgPrimary);
      expect(ratio).toBeGreaterThanOrEqual(AA_SMALL_TEXT);
    },
  );

  it('the two previously-failing combinations clear AA with real margin (>=5:1)', () => {
    const lightHigh = styleColorRatio(
      demoColor,
      tokens.light.high,
      tokens.light.surface0,
      tokens.light.fgPrimary,
    );
    const darkCritical = styleColorRatio(
      offlineColors.offline,
      tokens.dark.critical,
      tokens.dark.surface0,
      tokens.dark.fgPrimary,
    );
    expect(lightHigh).toBeGreaterThanOrEqual(5);
    expect(darkCritical).toBeGreaterThanOrEqual(5);
  });
});
