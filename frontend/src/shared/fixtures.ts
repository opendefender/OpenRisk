// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
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

export type NodeType = 'globe' | 'server' | 'database' | 'cloud' | 'laptop';
export interface UniNode {
  id: string; name: string; type: NodeType; crit: Criticality; score: number; riskCount: number;
  cveCount: number; env: string; ip: string; owner: string; os: string; lastScan: string;
  // simulation state (mutable at runtime)
  x?: number; y?: number; vx?: number; vy?: number; fixed?: boolean;
}
const N = (id: string, name: string, type: NodeType, crit: Criticality, score: number, rc: number, cve: number, env: string, ip: string): UniNode =>
  ({ id, name, type, crit, score, riskCount: rc, cveCount: cve, env, ip, owner: 'Amir Diallo', os: 'Linux', lastScan: 'il y a 3h' });

export const UNI_NODES: UniNode[] = [
  N('n1', 'Internet', 'globe', 'low', 0, 0, 0, 'production', '—'),
  N('n2', 'fw-edge-01', 'server', 'high', 6.4, 3, 2, 'production', '10.0.0.1'),
  N('n3', 'lb-prod', 'server', 'medium', 4.1, 1, 0, 'production', '10.0.1.4'),
  N('n4', 'srv-paie-01', 'server', 'critical', 9.2, 4, 5, 'production', '10.0.2.11'),
  N('n5', 'aws-prod', 'cloud', 'critical', 8.6, 3, 4, 'production', '—'),
  N('n6', 'gw-bank-02', 'server', 'high', 6.8, 2, 1, 'production', '10.0.2.20'),
  N('n7', 'db-core-01', 'database', 'medium', 3.8, 2, 0, 'production', '10.0.3.5'),
  N('n8', 'db-replica', 'database', 'medium', 3.2, 1, 0, 'staging', '10.0.3.6'),
  N('n9', 'portail-client', 'laptop', 'medium', 4.4, 1, 3, 'production', '10.0.4.9'),
  N('n10', 'www-public', 'globe', 'low', 1.6, 1, 0, 'production', '10.0.4.1'),
  N('n11', 'backup-nas', 'database', 'high', 6.2, 1, 0, 'production', '10.0.5.2'),
  N('n12', 'ad-domain', 'server', 'low', 1.9, 1, 0, 'production', '10.0.0.10'),
  N('n13', 'k8s-node-1', 'cloud', 'medium', 4.0, 0, 2, 'production', '10.0.6.1'),
  N('n14', 'k8s-node-2', 'cloud', 'medium', 3.7, 0, 1, 'production', '10.0.6.2'),
  N('n15', 'redis-cache', 'database', 'low', 2.1, 0, 0, 'production', '10.0.3.9'),
  N('n16', 'dev-sandbox', 'laptop', 'low', 1.2, 0, 1, 'dev', '10.1.0.5'),
  N('n17', 'vpn-gw', 'server', 'high', 5.9, 1, 1, 'production', '10.0.0.2'),
  N('n18', 'mail-relay', 'server', 'medium', 3.5, 1, 0, 'production', '10.0.7.1'),
  N('n19', 'ci-runner', 'cloud', 'medium', 3.9, 0, 2, 'staging', '10.1.1.3'),
  N('n20', 'iot-badge', 'laptop', 'medium', 4.6, 1, 0, 'production', '10.0.8.4'),
  N('n21', 's3-archives', 'cloud', 'low', 2.4, 0, 0, 'production', '—'),
  N('n22', 'monitoring', 'server', 'low', 1.8, 0, 0, 'production', '10.0.9.1'),
];
export const UNI_LINKS: [string, string][] = [
  ['n1', 'n2'], ['n2', 'n3'], ['n2', 'n17'], ['n3', 'n4'], ['n3', 'n5'], ['n3', 'n6'], ['n4', 'n7'],
  ['n5', 'n13'], ['n5', 'n14'], ['n5', 'n21'], ['n6', 'n7'], ['n7', 'n8'], ['n7', 'n15'], ['n9', 'n3'],
  ['n9', 'n7'], ['n10', 'n2'], ['n11', 'n7'], ['n12', 'n2'], ['n13', 'n19'], ['n14', 'n19'], ['n17', 'n16'],
  ['n18', 'n2'], ['n20', 'n2'], ['n22', 'n5'], ['n4', 'n11'], ['n5', 'n7'],
];
