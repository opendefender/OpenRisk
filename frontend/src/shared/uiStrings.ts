// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Flat FR/EN dictionary for the redesigned app shell + screens, ported verbatim
// from the OpenRisk design handoff (OpenRisk.dc.html §10). Kept separate from the
// nested locales/*.json so the new design language can grow without disturbing the
// existing feature translations. Consume via useUIStrings().

import { useUIStore, type Lang } from '../store/uiStore';

const fr = {
  brandShort: 'OpenRisk', enterprise: 'Enterprise', ciso: 'RSSI', newRisk: 'Nouveau risque',
  search: 'Rechercher…', globalScore: 'Score de sécurité', navigate: 'naviguer', open: 'ouvrir',
  close: 'fermer', cmdkPlaceholder: 'Rechercher risques, actifs, actions…',
  g_overview: 'Aperçu', g_security: 'Sécurité', g_intel: 'Conformité & Intel', g_assets: 'Actifs',
  g_report: 'Reporting & IA', g_admin: 'Admin',
  n_dashboard: 'Tableau de bord', n_analytics: 'Analytics CISO', n_risks: 'Registre des risques',
  n_mitigations: 'Mitigations', n_incidents: 'War Room', n_infra: 'Infrastructure',
  n_compliance: 'Conformité', n_cti: 'Threat Intel', n_assets: 'Inventaire', n_universe: 'Asset Universe',
  n_reports: 'Rapports', n_ai: 'IA Advisor', n_settings: 'Paramètres', n_superadmin: 'Super Admin',
  n_simulations: 'Simulations', n_leaderboard: 'Classement',
  notifTitle: 'Notifications', notifAll: 'Tout marquer comme lu', notifEmpty: 'Vous êtes à jour',
  notifViewAll: 'Voir toutes les notifications',
  greeting: 'Bonjour Amir', dashSub: 'Voici l’état de vos risques aujourd’hui',
  genReport: 'Générer un rapport', viewDetails: 'Voir détails', since7: 'depuis 7 jours',
  kpiTotal: 'Risques totaux', kpiCrit: 'Critiques', kpiMiti: 'En mitigation', kpiResolved: 'Résolus ce mois',
  heatTitle: 'Matrice probabilité × impact', impact: 'Impact', proba: 'Probabilité',
  trendTitle: 'Tendance des risques', recentTitle: 'Activité récente', warTitle: 'Incident en cours',
  warJoin: 'Rejoindre la War Room',
  critical: 'Critique', high: 'Élevé', medium: 'Moyen', low: 'Faible',
  st_open: 'Ouvert', st_progress: 'En cours', st_mitigated: 'Mitigé', st_accepted: 'Accepté',
  riskTitle: 'Registre des risques', all: 'Tous', pendingReview: 'À revoir',
  soon: 'Bientôt disponible', soonSub: 'Ce module fait partie de la feuille de route OpenRisk.',
};

const en: typeof fr = {
  brandShort: 'OpenRisk', enterprise: 'Enterprise', ciso: 'CISO', newRisk: 'New risk',
  search: 'Search…', globalScore: 'Security score', navigate: 'navigate', open: 'open',
  close: 'close', cmdkPlaceholder: 'Search risks, assets, actions…',
  g_overview: 'Overview', g_security: 'Security', g_intel: 'Compliance & Intel', g_assets: 'Assets',
  g_report: 'Reporting & AI', g_admin: 'Admin',
  n_dashboard: 'Dashboard', n_analytics: 'CISO Analytics', n_risks: 'Risk Register',
  n_mitigations: 'Mitigations', n_incidents: 'War Room', n_infra: 'Infrastructure',
  n_compliance: 'Compliance', n_cti: 'Threat Intel', n_assets: 'Inventory', n_universe: 'Asset Universe',
  n_reports: 'Reports', n_ai: 'AI Advisor', n_settings: 'Settings', n_superadmin: 'Super Admin',
  n_simulations: 'Simulations', n_leaderboard: 'Leaderboard',
  notifTitle: 'Notifications', notifAll: 'Mark all as read', notifEmpty: 'You are all caught up',
  notifViewAll: 'View all notifications',
  greeting: 'Hello Amir', dashSub: 'Here is the state of your risks today',
  genReport: 'Generate report', viewDetails: 'View details', since7: 'over 7 days',
  kpiTotal: 'Total risks', kpiCrit: 'Critical', kpiMiti: 'In mitigation', kpiResolved: 'Resolved this month',
  heatTitle: 'Probability × impact matrix', impact: 'Impact', proba: 'Probability',
  trendTitle: 'Risk trend', recentTitle: 'Recent activity', warTitle: 'Active incident',
  warJoin: 'Join the War Room',
  critical: 'Critical', high: 'High', medium: 'Medium', low: 'Low',
  st_open: 'Open', st_progress: 'In progress', st_mitigated: 'Mitigated', st_accepted: 'Accepted',
  riskTitle: 'Risk Register', all: 'All', pendingReview: 'To review',
  soon: 'Coming soon', soonSub: 'This module is part of the OpenRisk roadmap.',
};

export type UIStrings = typeof fr;

export function uiStrings(lang: Lang): UIStrings {
  return lang === 'fr' ? fr : en;
}

/** Reactive accessor — re-renders when the header FR/EN toggle flips the store. */
export function useUIStrings(): UIStrings {
  const lang = useUIStore((s) => s.lang);
  return uiStrings(lang);
}
