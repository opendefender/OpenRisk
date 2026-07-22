// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Design fixtures ported verbatim from OpenRisk.dc.html. The prototype is
// fixture-driven; these mirror it so the reskinned screens match the handoff
// pixel-for-pixel. Real API data is layered in per-screen where a backend exists
// (Dashboard KPIs already read the live risk store).

import type { Criticality } from './riskColors';
import type { RiskStatus } from './ui';

export interface FxRisk {
  id: string; name: string; crit: Criticality; score: number; prob: number; impact: number; ac: number;
  asset: string; fw: string; status: RiskStatus; owner: string; ownerName: string; mod: string; desc?: string;
}

export const RISKS: FxRisk[] = [
  { id: 'RSK-1042', name: 'Exposition RDP non filtrée sur serveur de paie', crit: 'critical', score: 9.2, prob: 0.9, impact: 9.5, ac: 1.2, asset: 'srv-paie-01', fw: 'ISO27001', status: 'open', owner: 'AD', ownerName: 'Amir Diallo', mod: 'il y a 2h', desc: 'Le port RDP (3389) du serveur de paie est exposé sur Internet sans restriction d’IP ni MFA, offrant une surface d’attaque directe sur des données financières sensibles.' },
  { id: 'RSK-1039', name: 'Absence de MFA sur comptes administrateurs cloud', crit: 'critical', score: 8.6, prob: 0.8, impact: 9.0, ac: 1.2, asset: 'aws-prod', fw: 'SOC2', status: 'progress', owner: 'FS', ownerName: 'Fatou Sy', mod: 'il y a 5h', desc: 'Plusieurs comptes IAM à privilèges élevés ne disposent pas de MFA activée.' },
  { id: 'RSK-1031', name: 'Chiffrement TLS obsolète sur passerelle bancaire', crit: 'high', score: 6.8, prob: 0.6, impact: 8.0, ac: 1.0, asset: 'gw-bank-02', fw: 'BCEAO', status: 'progress', owner: 'KM', ownerName: 'Kofi Mensah', mod: 'hier' },
  { id: 'RSK-1024', name: 'Sauvegardes non testées depuis 90 jours', crit: 'high', score: 6.2, prob: 0.7, impact: 6.5, ac: 1.0, asset: 'backup-nas', fw: 'ISO27001', status: 'open', owner: 'AD', ownerName: 'Amir Diallo', mod: 'hier' },
  { id: 'RSK-1018', name: 'Dépendances npm vulnérables (CVE-2024-4032)', crit: 'medium', score: 4.4, prob: 0.5, impact: 5.0, ac: 1.0, asset: 'portail-client', fw: 'NIST', status: 'mitigated', owner: 'LT', ownerName: 'Léa Traoré', mod: 'il y a 3j' },
  { id: 'RSK-1009', name: 'Journalisation insuffisante des accès DB', crit: 'medium', score: 3.8, prob: 0.4, impact: 5.0, ac: 1.0, asset: 'db-core-01', fw: 'DORA', status: 'progress', owner: 'FS', ownerName: 'Fatou Sy', mod: 'il y a 4j' },
  { id: 'RSK-0994', name: 'Politique de mots de passe non conforme', crit: 'low', score: 1.9, prob: 0.3, impact: 3.0, ac: 0.9, asset: 'ad-domain', fw: 'ANSSI', status: 'accepted', owner: 'KM', ownerName: 'Kofi Mensah', mod: 'il y a 6j' },
  { id: 'RSK-0987', name: 'Certificat SSL expirant dans 21 jours', crit: 'low', score: 1.6, prob: 0.4, impact: 2.5, ac: 0.8, asset: 'www-public', fw: 'ISO27001', status: 'open', owner: 'LT', ownerName: 'Léa Traoré', mod: 'la semaine dernière' },
];

export interface FxMiti {
  id: string; title: string; risk: string; owner: string; deadline: string; progress: number; crit: Criticality; overdue?: boolean;
}
export const MITIGATIONS: Record<'todo' | 'progress' | 'review' | 'done', FxMiti[]> = {
  todo: [
    { id: 'M1', title: 'Restreindre RDP par liste blanche IP', risk: 'RSK-1042', owner: 'AD', deadline: '12 juil.', progress: 0, overdue: true, crit: 'critical' },
    { id: 'M2', title: 'Renouveler certificat SSL www-public', risk: 'RSK-0987', owner: 'LT', deadline: '28 juil.', progress: 0, crit: 'low' },
  ],
  progress: [
    { id: 'M3', title: 'Déployer MFA sur comptes IAM AWS', risk: 'RSK-1039', owner: 'FS', deadline: '18 juil.', progress: 45, crit: 'critical' },
    { id: 'M4', title: 'Migrer passerelle vers TLS 1.3', risk: 'RSK-1031', owner: 'KM', deadline: '22 juil.', progress: 60, crit: 'high' },
  ],
  review: [
    { id: 'M5', title: 'Tester procédure de restauration NAS', risk: 'RSK-1024', owner: 'AD', deadline: '15 juil.', progress: 90, crit: 'high' },
  ],
  done: [
    { id: 'M6', title: 'Mettre à jour dépendances npm', risk: 'RSK-1018', owner: 'LT', deadline: '02 juil.', progress: 100, crit: 'medium' },
    { id: 'M7', title: 'Activer journalisation avancée DB', risk: 'RSK-1009', owner: 'FS', deadline: '30 juin', progress: 100, crit: 'medium' },
  ],
};
export const ALL_MITIGATIONS: FxMiti[] = [...MITIGATIONS.todo, ...MITIGATIONS.progress, ...MITIGATIONS.review, ...MITIGATIONS.done];
