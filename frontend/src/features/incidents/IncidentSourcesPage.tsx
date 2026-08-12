// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// "D'où viennent les incidents ?"
//
// The page a user reaches after their first automatically-opened incident. It
// answers three questions in the order people ask them: what opened this, is it
// supposed to, and where do I change it. The counts are this tenant's real
// ones — a page that lists five sources while four have never fired teaches the
// wrong thing.

import { Link } from 'react-router';
import { HelpCircle, Bot, Hand, ArrowRight, Settings2 } from 'lucide-react';
import { PageFrame, PageHeader, Card, SkeletonRows } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { useIncidentOrigins } from './useIncidents';

export function IncidentSourcesPage() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data, isLoading } = useIncidentOrigins();

  const items = data?.items ?? [];
  const total = data?.total ?? 0;

  return (
    <PageFrame>
      <PageHeader title={tr('D’où viennent les incidents ?', 'Where do incidents come from?')} />
      <p className="text-[13px] -mt-2 mb-4" style={{ color: 'var(--text-secondary)' }}>
        {tr(
          'Cinq chemins mènent à un incident dans OpenRisk. Un seul part d’un jugement humain.',
          'Five paths lead to an incident in OpenRisk. Only one starts from a human judgement.',
        )}
      </p>

      <Card style={{ padding: '16px 18px', marginBottom: 16 }}>
        <div className="flex items-start gap-2.5">
          <HelpCircle size={16} style={{ color: 'var(--accent)' }} className="shrink-0 mt-0.5" />
          <p className="text-[13px]" style={{ color: 'var(--text-primary)' }}>
            {tr(
              'Tout incident ouvert sans intervention humaine porte un bandeau qui nomme la règle et la source exactes, avec un lien vers cette règle. Si un incident vous surprend, ce bandeau vous dit quoi changer — et cette page vous dit où.',
              'Any incident opened without a human carries a banner naming the exact rule and source, with a link to that rule. If an incident surprises you, that banner tells you what to change — and this page tells you where.',
            )}
          </p>
        </div>
      </Card>

      {isLoading && <Card style={{ padding: 12 }}><SkeletonRows rows={4} /></Card>}

      <div className="space-y-3">
        {items.map(({ origin, count }) => {
          const Icon = origin.automatic ? Bot : Hand;
          const share = total > 0 ? Math.round((count / total) * 100) : 0;
          return (
            <Card key={origin.key} style={{ padding: '14px 16px' }}>
              <div className="flex items-start gap-3 flex-wrap">
                <span
                  className="w-9 h-9 rounded-[10px] flex items-center justify-center shrink-0"
                  style={{
                    background: origin.automatic
                      ? 'color-mix(in srgb, var(--accent) 12%, transparent)'
                      : 'var(--bg-hover)',
                    color: origin.automatic ? 'var(--accent)' : 'var(--text-secondary)',
                  }}
                >
                  <Icon size={17} />
                </span>
                <div className="flex-1 min-w-[240px]">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="text-[14px] font-bold" style={{ color: 'var(--text-primary)' }}>
                      {lang === 'fr' ? origin.label : origin.label_en}
                    </span>
                    <span
                      className="h-5 px-2 rounded-full text-[10.5px] font-semibold"
                      style={{
                        background: origin.automatic
                          ? 'color-mix(in srgb, var(--accent) 12%, transparent)'
                          : 'var(--bg-hover)',
                        color: origin.automatic ? 'var(--accent)' : 'var(--text-secondary)',
                      }}
                    >
                      {origin.automatic ? tr('automatique', 'automatic') : tr('humain', 'human')}
                    </span>
                  </div>
                  <p className="text-[12.5px] mt-1" style={{ color: 'var(--text-secondary)' }}>
                    {origin.description}
                  </p>
                  {origin.where_to_configure && (
                    <Link
                      to={origin.where_to_configure}
                      className="text-[12px] font-semibold inline-flex items-center gap-1 mt-1.5"
                      style={{ color: 'var(--accent)' }}
                    >
                      <Settings2 size={12} /> {tr('Configurer cette source', 'Configure this source')}
                      <ArrowRight size={12} />
                    </Link>
                  )}
                </div>
                <div className="text-right shrink-0">
                  <div className="mono text-[22px] font-bold" style={{ color: 'var(--text-primary)' }}>
                    {count}
                  </div>
                  <div className="text-[11px]" style={{ color: 'var(--text-secondary)' }}>
                    {count === 0
                      ? tr('jamais déclenchée', 'never fired')
                      : `${share}% ${tr('de vos incidents', 'of your incidents')}`}
                  </div>
                </div>
              </div>
            </Card>
          );
        })}
      </div>

      <p className="text-[12px] mt-4" style={{ color: 'var(--text-secondary)' }}>
        {tr(
          'Une source manquante ici signifie qu’aucun code ne l’utilise : cette liste est dérivée des chemins qui créent réellement des incidents, pas d’une documentation à jour séparément.',
          'A source missing here means no code uses it: this list is derived from the paths that actually create incidents, not from documentation maintained separately.',
        )}
      </p>
    </PageFrame>
  );
}
