// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// "Exposition" in the asset drawer (#937).

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';

import { useUIStore } from '../../../store/uiStore';
import { ExposureDigest, ExposureSection } from '../sections/ExposureSection';
import type { AssetAnalysis } from '../useEntityDrawer';

const KEV_ID = '11111111-0000-4000-8000-000000000001';
const LATE_ID = '22222222-0000-4000-8000-000000000002';
const RISK_ID = '33333333-0000-4000-8000-000000000003';

function analysis(over: Partial<AssetAnalysis> = {}): AssetAnalysis {
  return {
    asset_id: 'a1',
    criticality: 'CRITICAL',
    network_zone: 'perimetre',
    internet_facing: true,
    by_severity: { critical: 0, high: 1, medium: 1, low: 0 },
    open: 2,
    kev_open: 1,
    overdue: 1,
    resolved: 1,
    last_detected_at: '2026-10-08T09:00:00Z',
    truncated: false,
    vulnerabilities: [
      {
        id: KEV_ID,
        cve_id: 'CVE-2025-55241',
        title: 'Élévation de privilèges Entra ID',
        severity: 'medium',
        cvss: 6.5,
        epss: 0.4,
        kev: true,
        status: 'triaged',
        sla_due_at: '2026-10-15T00:00:00Z',
        overdue: false,
        first_seen: '2026-10-01T00:00:00Z',
        remediation_hint: 'Appliquer le correctif de septembre.',
      },
      {
        id: LATE_ID,
        cve_id: 'CVE-2026-1000',
        title: 'Contournement d’authentification',
        severity: 'high',
        cvss: 8.1,
        epss: 0.02,
        kev: false,
        status: 'open',
        sla_due_at: '2026-10-07T00:00:00Z',
        overdue: true,
        first_seen: '2026-09-20T00:00:00Z',
      },
    ],
    risks: [
      { id: RISK_ID, title: 'Compromission AD', score: 6.4, criticality: 'high', status: 'open' },
    ],
    findings: [
      { code: 'kev_open', count: 1 },
      { code: 'overdue', count: 1, days: 3 },
      { code: 'severe_open', count: 1 },
      { code: 'internet_facing', value: 'perimetre' },
      { code: 'business_critical', value: 'CRITICAL' },
      { code: 'linked_risks', count: 1, value: '6.4' },
    ],
    next_action: { vulnerability_id: KEV_ID, reasons: ['kev', 'severity:medium', 'epss'] },
    ...over,
  };
}

function renderSection(data: AssetAnalysis | undefined, extra: { isError?: boolean } = {}) {
  const onOpen = vi.fn();
  const onRetry = vi.fn();
  render(
    <MemoryRouter>
      <ExposureSection
        data={data}
        isLoading={false}
        isError={!!extra.isError}
        onRetry={onRetry}
        onOpen={onOpen}
      />
    </MemoryRouter>,
  );
  return { onOpen, onRetry };
}

beforeEach(() => useUIStore.setState({ lang: 'fr' }));

describe('#937 — asset exposure section', () => {
  it('reads the asset as sentences from the server’s facts', () => {
    renderSection(analysis());
    const reading = screen.getByTestId('exposure-reading');
    expect(reading).toHaveTextContent('1 vulnérabilité exploitée activement (catalogue KEV).');
    expect(reading).toHaveTextContent('1 correctif en retard sur son délai, depuis 3 j.');
    expect(reading).toHaveTextContent("Exposé depuis l'extérieur (zone : périmètre).");
    expect(reading).toHaveTextContent("Actif de criticité critique pour l'activité.");
    expect(reading).toHaveTextContent('1 risque lié, score 6,4.');
  });

  it('names the finding to treat first, with why, and opens it', () => {
    const { onOpen } = renderSection(analysis());
    const next = screen.getByTestId('exposure-next');
    expect(next).toHaveTextContent('CVE-2025-55241');
    expect(next).toHaveTextContent(
      "exploitée activement · sévérité moyenne · probabilité d'exploitation élevée",
    );
    expect(next).toHaveTextContent('Appliquer le correctif de septembre.');
    fireEvent.click(within(next).getByRole('button', { name: 'Ouvrir' }));
    expect(onOpen).toHaveBeenCalledWith('vulnerability', KEV_ID);
  });

  it('lists open findings with their tags and opens a risk', () => {
    const { onOpen } = renderSection(analysis());
    const rows = screen.getAllByTestId('exposure-vuln');
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent('KEV');
    expect(rows[1]).toHaveTextContent('En retard');
    expect(rows[1]).toHaveTextContent('CVSS 8,1');
    fireEvent.click(within(screen.getByTestId('exposure-risks')).getByText('Compromission AD'));
    expect(onOpen).toHaveBeenCalledWith('risk', RISK_ID);
  });

  it('says when the asset is clean, and when it was never scanned', () => {
    renderSection(
      analysis({
        open: 0,
        kev_open: 0,
        overdue: 0,
        vulnerabilities: [],
        risks: [],
        next_action: null,
        findings: [{ code: 'clean', count: 3 }],
      }),
    );
    expect(screen.getByTestId('exposure-reading')).toHaveTextContent(
      'Aucune vulnérabilité ouverte ; 3 corrigées ou traitées. Dernière détection le',
    );
    expect(screen.queryByTestId('exposure-next')).toBeNull();
  });

  it('never scanned', () => {
    renderSection(
      analysis({
        open: 0,
        vulnerabilities: [],
        risks: [],
        next_action: null,
        last_detected_at: null,
        findings: [{ code: 'never_scanned' }],
      }),
    );
    expect(screen.getByTestId('exposure-reading')).toHaveTextContent(
      "Aucun scan n'a encore remonté de constat sur cet actif.",
    );
  });

  it('a failed load offers a retry', () => {
    const { onRetry } = renderSection(undefined, { isError: true });
    fireEvent.click(screen.getByRole('button', { name: /réessayer/i }));
    expect(onRetry).toHaveBeenCalled();
  });

  it('the Aperçu digest shows the pressing lines and leads to the analysis', () => {
    const onMore = vi.fn();
    render(<ExposureDigest data={analysis()} isLoading={false} onMore={onMore} />);
    const digest = screen.getByTestId('exposure-digest');
    expect(digest).toHaveTextContent('1 vulnérabilité exploitée activement');
    expect(digest).toHaveTextContent('1 correctif en retard');
    expect(digest).not.toHaveTextContent('risque lié');
    expect(digest).toHaveTextContent('CVE-2025-55241 — Élévation de privilèges Entra ID');
    fireEvent.click(screen.getByRole('button', { name: "Voir l'analyse" }));
    expect(onMore).toHaveBeenCalled();
  });
});
