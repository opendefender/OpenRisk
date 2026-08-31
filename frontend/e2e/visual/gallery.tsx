// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Design-system gallery.
 *
 * Every primitive, in every variant and every state, on one page per group, in
 * both themes. Playwright screenshots each and runs axe over it.
 *
 * Why a gallery rather than testing primitives through the screens that use
 * them: a screen exercises the two states it happens to be in. A disabled
 * destructive button, an invalid select, a warning badge and a permission-denied
 * panel are all real states that no single screen shows, so nothing would catch
 * them going unreadable in one theme — which is exactly the class of bug the
 * token layer exists to prevent, and exactly the class that only appears in the
 * theme the author was not using.
 *
 * Groups are separate pages rather than one long one because a single diff
 * covering every primitive tells you something changed, not what.
 */

import { StrictMode, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { AlertTriangle, Bug, Download, Plus, Search, Trash2 } from 'lucide-react';

import '../../src/index.css';

import { Badge } from '../../src/shared/ds/Badge';
import { Button, type ButtonVariant } from '../../src/shared/ds/Button';
import { Field, Input, Select, Textarea } from '../../src/shared/ds/Field';
import { Tabs, TabPanel } from '../../src/shared/ds/Tabs';
import {
  AuditEntry,
  ErrorState,
  LoadingState,
  PermissionDenied,
  SkeletonRows,
} from '../../src/shared/ds/States';
import { EmptyState } from '../../src/shared/EmptyState';
import { categorical, chartAxis, graph, severity } from '../../src/shared/ds/chart';

/** See harness.tsx — the UI store applies its own theme at module load. */
function applyRequestedTheme() {
  const params = new URLSearchParams(window.location.search);
  document.documentElement.setAttribute('data-theme', params.get('theme') ?? 'dark');
  document.documentElement.setAttribute('data-variant', 'azure');
}
applyRequestedTheme();

const noop = () => {};

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-2 border-b border-subtle py-4 last:border-b-0">
      <span className="text-2xs font-semibold uppercase tracking-caps text-text-muted">{label}</span>
      <div className="flex flex-wrap items-center gap-3">{children}</div>
    </div>
  );
}

const VARIANTS: ButtonVariant[] = ['primary', 'secondary', 'ghost', 'destructive', 'link'];

function Controls() {
  return (
    <>
      {VARIANTS.map((variant) => (
        <Row key={variant} label={variant}>
          <Button variant={variant}>Default</Button>
          <Button variant={variant} icon={Plus}>
            With icon
          </Button>
          <Button variant={variant} disabled>
            Disabled
          </Button>
          <Button variant={variant} loading>
            Loading
          </Button>
          <Button variant={variant} feedback="success">
            Saved
          </Button>
          <Button variant={variant} feedback="error">
            Failed
          </Button>
        </Row>
      ))}
      <Row label="sizes">
        <Button size="sm">Small</Button>
        <Button size="md">Medium</Button>
        <Button size="lg">Large</Button>
      </Row>
      <Row label="icon only">
        <Button size="sm" icon={Trash2} aria-label="Delete, small" />
        <Button size="md" icon={Download} aria-label="Export, medium" />
        <Button size="lg" icon={Bug} variant="primary" aria-label="Report a bug, large" />
      </Row>
      <Row label="badges">
        {(['neutral', 'accent', 'success', 'warning', 'danger', 'info', 'experimental', 'unavailable'] as const).map(
          (intent) => (
            <Badge key={intent} intent={intent} dot>
              {intent}
            </Badge>
          ),
        )}
      </Row>
      <Row label="badge sizes">
        <Badge size="sm" intent="danger">
          Critical
        </Badge>
        <Badge size="md" intent="danger">
          Critical
        </Badge>
      </Row>
    </>
  );
}

function Forms() {
  return (
    <div className="grid max-w-2xl gap-5">
      <Field label="Asset name" description="How this asset appears in the inventory" required>
        <Input placeholder="web-prod-01" />
      </Field>
      <Field label="Filled">
        <Input defaultValue="db-replica-02" />
      </Field>
      <Field label="With icons">
        <Input placeholder="Search assets" leadingIcon={<Search size={14} />} loading />
      </Field>
      <Field label="Invalid" message="An asset with that name already exists" status="invalid">
        <Input defaultValue="web-prod-01" />
      </Field>
      <Field label="Warning" message="This asset has had no scan in 90 days" status="warning">
        <Input defaultValue="legacy-fileshare" />
      </Field>
      <Field label="Success" message="Reachable from the scanner" status="success">
        <Input defaultValue="10.0.4.18" />
      </Field>
      <Field label="Disabled" disabled>
        <Input defaultValue="Managed by the cloud connector" disabled />
      </Field>
      <Field label="Read only">
        <Input defaultValue="i-0a1b2c3d4e5f" readOnly />
      </Field>
      <Field label="Criticality" description="Drives the score engine">
        <Select defaultValue="high">
          <option value="critical">Critical</option>
          <option value="high">High</option>
          <option value="medium">Medium</option>
          <option value="low">Low</option>
        </Select>
      </Field>
      <Field label="Invalid select" status="invalid" message="Pick a criticality">
        <Select defaultValue="">
          <option value="">Choose…</option>
          <option value="high">High</option>
        </Select>
      </Field>
      <Field label="Notes">
        <Textarea placeholder="Anything the next responder should know" />
      </Field>
    </div>
  );
}

function States() {
  const [tab, setTab] = useState('empty');
  const items = [
    { id: 'empty', label: 'Empty' },
    { id: 'loading', label: 'Loading', count: 6 },
    { id: 'error', label: 'Error' },
    { id: 'denied', label: 'Denied' },
    { id: 'audit', label: 'Audit', count: 3 },
  ] as const;

  return (
    <div className="flex flex-col gap-4">
      <Tabs id="states" items={items} value={tab} onChange={setTab} label="State examples" />

      <TabPanel tabsId="states" id="empty" active={tab === 'empty'}>
        <EmptyState
          variant="first-use"
          title="No risks yet"
          description="Create your first risk to see its financial exposure and a suggested treatment."
          primaryAction={
            <Button variant="primary" icon={Plus} onClick={noop}>
              Create a risk
            </Button>
          }
        />
      </TabPanel>

      <TabPanel tabsId="states" id="loading" active={tab === 'loading'}>
        <SkeletonRows rows={6} />
        <LoadingState label="Loading the register" />
      </TabPanel>

      <TabPanel tabsId="states" id="error" active={tab === 'error'}>
        <ErrorState
          title="Could not load the risk register"
          description="The server did not answer. Retry, or contact an administrator if this persists."
          detail="GET /api/v1/risks — 504 Gateway Timeout"
          onRetry={noop}
        />
      </TabPanel>

      <TabPanel tabsId="states" id="denied" active={tab === 'denied'}>
        <PermissionDenied resource="the audit trail" requiredPermission="governance:read" />
      </TabPanel>

      <TabPanel tabsId="states" id="audit" active={tab === 'audit'}>
        <div className="rounded-lg border border-subtle bg-surface-1 px-4">
          <AuditEntry
            actor="admin@opendefender.io"
            action="changed the lifecycle state to"
            target="IN_TREATMENT"
            timestamp="2026-08-25 09:14"
            isoTimestamp="2026-08-25T09:14:00Z"
            detail="Guard: 1 active mitigation"
          />
          <AuditEntry
            actor="scanner"
            action="auto-completed sub-action"
            target="Patch Log4j"
            timestamp="2026-08-25 08:02"
            isoTimestamp="2026-08-25T08:02:00Z"
          />
          <AuditEntry
            actor="rssi@opendefender.io"
            action="approved the residual-risk acceptance"
            timestamp="2026-08-24 17:40"
            isoTimestamp="2026-08-24T17:40:00Z"
          />
        </div>
      </TabPanel>
    </div>
  );
}

/**
 * The visualisation palette, drawn as bare SVG rather than through Recharts.
 *
 * The point is the COLOURS and their contrast against the card, which is what
 * the light/dark pair has to prove; pulling in a chart library would make the
 * snapshot sensitive to that library's own rendering and to its label
 * placement, neither of which this is testing.
 */
function Charts() {
  return (
    <div className="grid gap-6">
      <div>
        <p className="mb-2 text-2xs font-semibold uppercase tracking-caps text-text-muted">
          Categorical series
        </p>
        <svg width="100%" height="120" role="img" aria-label="The eight categorical series colours">
          <line x1="0" y1="110" x2="100%" y2="110" stroke={chartAxis.stroke} />
          {categorical.map((colour, i) => (
            <g key={colour}>
              <rect x={i * 56 + 8} y={110 - (i + 3) * 10} width="40" height={(i + 3) * 10} fill={colour} rx="3" />
              <text x={i * 56 + 8} y={118} fill="var(--chart-label)" fontSize="10">
                {i + 1}
              </text>
            </g>
          ))}
        </svg>
      </div>

      <div>
        <p className="mb-2 text-2xs font-semibold uppercase tracking-caps text-text-muted">
          Severity encodings (the risk scale, not the categorical ramp)
        </p>
        <div className="flex flex-wrap gap-3">
          {Object.entries(severity).map(([name, colour]) => (
            <span key={name} className="inline-flex items-center gap-2 text-xs text-text-secondary">
              <span className="h-3 w-8 rounded-xs" style={{ background: colour }} />
              {name}
            </span>
          ))}
        </div>
      </div>

      <div>
        <p className="mb-2 text-2xs font-semibold uppercase tracking-caps text-text-muted">
          Topology graph
        </p>
        <svg width="100%" height="140" role="img" aria-label="Graph node and edge colours">
          <line x1="60" y1="70" x2="200" y2="40" stroke={graph.edge} strokeWidth="1.5" />
          <line x1="60" y1="70" x2="200" y2="100" stroke={graph.edgeActive} strokeWidth="2" />
          <circle cx="60" cy="70" r="18" fill={graph.node} stroke={graph.nodeStroke} strokeWidth="1.5" />
          <circle cx="200" cy="40" r="14" fill={graph.node} stroke={graph.nodeStroke} strokeWidth="1.5" opacity={graph.dimmed} />
          <circle cx="200" cy="100" r="14" fill={severity.critical} stroke={graph.nodeStroke} strokeWidth="1.5" />
        </svg>
      </div>
    </div>
  );
}

function Feedback() {
  return (
    <div className="grid max-w-xl gap-3">
      <div className="flex items-start gap-2 rounded-md border border-default bg-danger-surface p-3">
        <AlertTriangle size={16} className="mt-0.5 shrink-0 text-danger-text" aria-hidden="true" />
        <p className="text-sm text-danger-text">
          Destructive consequence, stated in the semantic surface pair.
        </p>
      </div>
      <div className="rounded-md border border-default bg-warning-surface p-3 text-sm text-warning-text">
        Warning surface with its matching text token.
      </div>
      <div className="rounded-md border border-default bg-success-surface p-3 text-sm text-success-text">
        Success surface with its matching text token.
      </div>
      <div className="rounded-md border border-default bg-info-surface p-3 text-sm text-info-text">
        Info surface with its matching text token.
      </div>
      <div className="grid grid-cols-4 gap-2">
        {['surface-0', 'surface-1', 'surface-2', 'surface-3'].map((surface) => (
          <div
            key={surface}
            className="rounded-md border border-subtle p-3 text-2xs text-text-secondary"
            style={{ background: `var(--${surface})` }}
          >
            {surface}
          </div>
        ))}
      </div>
    </div>
  );
}

export const GALLERIES: Record<string, () => JSX.Element> = {
  controls: Controls,
  forms: Forms,
  states: States,
  charts: Charts,
  feedback: Feedback,
};

function Gallery() {
  const id = new URLSearchParams(window.location.search).get('gallery') ?? '';
  const Render = GALLERIES[id];

  return (
    <main className="min-h-screen bg-surface-0 p-8 text-text-primary">
      <h1 className="mb-1 text-xl font-semibold tracking-display">{id || 'Design system'}</h1>
      <p className="mb-6 text-sm text-text-secondary">
        Every variant and state, rendered against the real stylesheet.
      </p>
      {Render ? (
        <Render />
      ) : (
        <ul className="text-sm text-text-secondary">
          {Object.keys(GALLERIES).map((key) => (
            <li key={key}>{key}</li>
          ))}
        </ul>
      )}
    </main>
  );
}

queueMicrotask(applyRequestedTheme);

createRoot(document.getElementById('gallery-root')!).render(
  <StrictMode>
    <Gallery />
  </StrictMode>,
);
