// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Plain-language glossary for the security jargon a non-technical GRC user (e.g. a
// compliance officer) meets across the app (docs/UX_CHARTER UX-03/UX-14,
// OR-BUG-010). Surfaced by <Term/> as a first-hover tooltip.

export interface GlossaryEntry {
  fr: string;
  en: string;
}

export const GLOSSARY: Record<string, GlossaryEntry> = {
  CVE: {
    fr: "Common Vulnerabilities and Exposures — l'identifiant public unique d'une faille de sécurité connue (ex. CVE-2021-44228).",
    en: 'Common Vulnerabilities and Exposures — the public unique ID of a known security flaw (e.g. CVE-2021-44228).',
  },
  CVSS: {
    fr: 'Common Vulnerability Scoring System — la note de gravité d’une faille, de 0 (faible) à 10 (critique).',
    en: 'Common Vulnerability Scoring System — a flaw’s severity score, from 0 (low) to 10 (critical).',
  },
  KEV: {
    fr: 'Known Exploited Vulnerabilities (CISA) — faille dont on sait qu’elle est activement exploitée par des attaquants : à corriger en priorité.',
    en: 'CISA Known Exploited Vulnerabilities — a flaw known to be actively exploited by attackers: patch it first.',
  },
  EPSS: {
    fr: 'Exploit Prediction Scoring System — la probabilité qu’une faille soit exploitée dans les 30 prochains jours.',
    en: 'Exploit Prediction Scoring System — the probability a flaw will be exploited within the next 30 days.',
  },
  ALE: {
    fr: 'Annualized Loss Expectancy — la perte financière moyenne attendue sur un an pour un risque.',
    en: 'Annualized Loss Expectancy — the average financial loss expected over a year for a risk.',
  },
  MTTR: {
    fr: 'Mean Time To Resolve — le délai moyen pour résoudre un incident.',
    en: 'Mean Time To Resolve — the average time it takes to resolve an incident.',
  },
  KRI: {
    fr: 'Key Risk Indicator — un indicateur clé qui suit l’évolution du risque.',
    en: 'Key Risk Indicator — a headline metric that tracks how risk is trending.',
  },
};
