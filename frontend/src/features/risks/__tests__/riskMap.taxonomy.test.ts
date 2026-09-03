// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// A regression fence around the reported bug: "étiquette affichée comme
// framework". riskMap used to compute
//
//     fw = frameworks[0] ?? tags[0]
//
// so a user's free-text label was rendered in the "Référentiel" column wearing a
// framework badge whenever the (also free-text) frameworks array was empty.

import { describe, expect, it } from 'vitest';
import { mapRisk } from '../riskMap';
import type { Risk } from '../../../hooks/useRiskStore';

function risk(partial: Partial<Risk>): Risk {
  return {
    id: 'r1',
    title: 'Bucket S3 exposé',
    description: '',
    score: 12,
    impact: 8,
    probability: 0.6,
    status: 'open',
    tags: [],
    source: 'manual',
    ...partial,
  } as Risk;
}

describe('riskMap — the three classification concepts stay apart', () => {
  it('never renders a tag as a compliance reference', () => {
    const ui = mapRisk(risk({ tags: ['cloud', 'production'] }), 'fr');

    // THE bug. A label is not a framework, whatever the column is empty.
    expect(ui.fw).toBe('—');
    expect(ui.fwHref).toBe('');
    // …and the tags are still there, in their own field.
    expect(ui.tags).toEqual(['cloud', 'production']);
  });

  it('ignores the frozen free-text frameworks column', () => {
    // `frameworks` came from a hard-coded dropdown that never consulted the
    // tenant's imported frameworks, so a value in it proves nothing. Migration
    // 0046 froze it; the UI must not read it either.
    const ui = mapRisk(risk({ frameworks: ['ISO27001'], tags: [] }), 'fr');
    expect(ui.fw).toBe('—');
  });

  it('renders a control-level mapping with its code and deep link', () => {
    const ui = mapRisk(
      risk({
        control_mappings: [
          {
            id: 'm1',
            risk_id: 'r1',
            framework_id: 'fw1',
            control_id: 'c1',
            framework_name: 'ISO 27001',
            control_code: 'A.5.1',
            control_name: 'Politiques de sécurité',
          },
        ],
      }),
      'fr',
    );

    expect(ui.fw).toBe('ISO 27001 · A.5.1');
    expect(ui.fwHref).toBe('/compliance/frameworks/fw1/controls/c1');
    expect(ui.mappings).toHaveLength(1);
  });

  it('renders a framework-level mapping as the framework, linking to its controls', () => {
    // control_id null is what the 0046 migration can honestly infer from the old
    // free-text values: "this relates to ISO 27001", no clause pinned.
    const ui = mapRisk(
      risk({
        control_mappings: [
          {
            id: 'm1',
            risk_id: 'r1',
            framework_id: 'fw1',
            control_id: null,
            framework_name: 'NIST CSF',
          },
        ],
      }),
      'fr',
    );

    expect(ui.fw).toBe('NIST CSF');
    expect(ui.fwHref).toBe('/compliance/frameworks/fw1/controls');
  });

  it('keeps the controlled category separate from both', () => {
    const ui = mapRisk(
      risk({
        tags: ['cloud'],
        category: { id: 'c1', name: 'Cybersécurité', slug: 'cybersecurite', color: 'critical' },
      }),
      'fr',
    );

    expect(ui.categoryName).toBe('Cybersécurité');
    expect(ui.categoryColor).toBe('critical');
    expect(ui.tags).toEqual(['cloud']);
    expect(ui.fw).toBe('—');
  });

  it('prefers the server-resolved owner email over the legacy free-text fields', () => {
    const ui = mapRisk(
      risk({ owner_email: 'amina.diop@openrisk.io', assigned_to: 'someone-else' } as Partial<Risk>),
      'fr',
    );
    expect(ui.ownerName).toBe('amina.diop@openrisk.io');
  });

  it('falls back to the legacy assignment for rows written before migration 0044', () => {
    const ui = mapRisk(risk({ assigned_to: 'legacy@openrisk.io' }), 'fr');
    expect(ui.ownerName).toBe('legacy@openrisk.io');
  });
});
