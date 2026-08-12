// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// "Créé automatiquement par la règle X depuis la source Y."
//
// An incident that appeared on its own is unnerving until you can see what
// opened it. That unease is what turns automatic alerts into muted ones, so the
// banner is not decoration: it names the rule, links to it, and gives the
// concrete evidence. It renders only for automatic origins — a human
// declaration needs no explanation.

import { Link } from 'react-router';
import { Bot, ExternalLink, HelpCircle } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import type { Incident, IncidentOrigin } from './incidentService';

const AUTOMATIC: IncidentOrigin[] = ['automation', 'scanner', 'cti', 'integration'];

const ORIGIN_LABEL: Record<IncidentOrigin, { fr: string; en: string }> = {
  manual: { fr: 'déclaration manuelle', en: 'manual declaration' },
  automation: { fr: 'une règle d’automatisation', en: 'an automation rule' },
  scanner: { fr: 'le scanner d’infrastructure', en: 'the infrastructure scanner' },
  cti: { fr: 'le renseignement sur les menaces', en: 'threat intelligence' },
  integration: { fr: 'un outil externe', en: 'an external tool' },
};

export function IncidentProvenance({ incident }: { incident: Incident }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  if (!incident.origin || !AUTOMATIC.includes(incident.origin)) return null;

  const source = ORIGIN_LABEL[incident.origin] ?? ORIGIN_LABEL.integration;

  return (
    <div
      className="rounded-[10px] p-3 flex items-start gap-2.5"
      style={{
        background: 'color-mix(in srgb, var(--accent) 8%, transparent)',
        border: '1px solid color-mix(in srgb, var(--accent) 30%, transparent)',
      }}
    >
      <Bot size={16} style={{ color: 'var(--accent)' }} className="shrink-0 mt-0.5" />
      <div className="min-w-0 flex-1">
        <p className="text-[12.5px]" style={{ color: 'var(--text-primary)' }}>
          {incident.origin_rule_name ? (
            <>
              {tr('Créé automatiquement par la règle ', 'Created automatically by the rule ')}
              <strong>« {incident.origin_rule_name} »</strong>
              {tr(' depuis ', ' from ')}
              {tr(source.fr, source.en)}
              {tr('.', '.')}
            </>
          ) : (
            <>
              {tr('Créé automatiquement depuis ', 'Created automatically from ')}
              {tr(source.fr, source.en)}
              {tr('. Aucune personne ne l’a déclaré.', '. Nobody declared it.')}
            </>
          )}
        </p>
        {incident.origin_detail && (
          <p className="text-[12px] mt-0.5 mono" style={{ color: 'var(--text-secondary)' }}>
            {incident.origin_detail}
          </p>
        )}
        <div className="flex items-center gap-3 mt-1.5 flex-wrap">
          {incident.origin_rule_id && (
            <Link
              to={`/automation?tab=rules&focus=${incident.origin_rule_id}`}
              className="text-[12px] font-semibold inline-flex items-center gap-1"
              style={{ color: 'var(--accent)' }}
            >
              {tr('Voir la règle', 'View the rule')} <ExternalLink size={12} />
            </Link>
          )}
          <Link
            to="/incidents/sources"
            className="text-[12px] inline-flex items-center gap-1"
            style={{ color: 'var(--text-secondary)' }}
          >
            <HelpCircle size={12} /> {tr('D’où viennent les incidents ?', 'Where do incidents come from?')}
          </Link>
        </div>
      </div>
    </div>
  );
}

/** A compact origin chip for the register table. */
export function OriginChip({ incident }: { incident: Incident }) {
  const lang = useUIStore((s) => s.lang);
  const automatic = incident.origin && AUTOMATIC.includes(incident.origin);
  if (!automatic) return null;
  const source = ORIGIN_LABEL[incident.origin] ?? ORIGIN_LABEL.integration;
  return (
    <span
      title={
        incident.origin_rule_name
          ? `${lang === 'fr' ? 'Règle' : 'Rule'}: ${incident.origin_rule_name}`
          : undefined
      }
      className="h-5 px-1.5 rounded-full text-[10.5px] font-semibold inline-flex items-center gap-1"
      style={{ background: 'color-mix(in srgb, var(--accent) 12%, transparent)', color: 'var(--accent)' }}
    >
      <Bot size={10} /> {lang === 'fr' ? source.fr : source.en}
    </span>
  );
}
