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
import { applyHarnessEnv } from './harnessEnv';
import { useI18n } from '../../src/hooks/useI18n';
import { MemoryRouter } from 'react-router';
import { DataTable } from '../../src/shared/datatable';
import { useTableState } from '../../src/shared/datatable/useTableState';
import { AlertDialog } from '../../src/shared/ds/AlertDialog';
import { Spinner } from '../../src/shared/ds/Spinner';
import { Popover } from '../../src/shared/ds/Popover';
import { Menu } from '../../src/shared/ds/Menu';

import { Badge } from '../../src/shared/ds/Badge';
import { Button, type ButtonVariant } from '../../src/shared/ds/Button';
import { Field, Input, Select, Textarea } from '../../src/shared/ds/Field';
import { Checkbox, CheckboxGroup } from '../../src/shared/ds/Checkbox';
import { Fieldset } from '../../src/shared/ds/Fieldset';
import { InputGroup } from '../../src/shared/ds/InputGroup';
import { Label } from '../../src/shared/ds/Label';
import { RadioGroup } from '../../src/shared/ds/RadioGroup';
import { Switch } from '../../src/shared/ds/Switch';
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
      <span className="text-2xs font-semibold uppercase tracking-caps text-fg-muted">{label}</span>
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

function FormControls() {
  const [scopes, setScopes] = useState<string[]>(['assets']);
  const [cadence, setCadence] = useState<string | null>('daily');

  return (
    <div className="grid max-w-2xl gap-6">
      <Fieldset legend="Labels" description="Standalone label, required marker and disabled tone">
        {/* Each label is attached to a real control, which is the only way it is
            ever used — and the disabled one to a genuinely disabled control, so
            the greyed pair is the exempt case WCAG 1.4.3 describes rather than
            low-contrast text floating on its own. */}
        <div className="grid gap-3 sm:grid-cols-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="fc-plain">Plain label</Label>
            <Input id="fc-plain" defaultValue="Acme" />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="fc-required" required>
              Required label
            </Label>
            <Input id="fc-required" defaultValue="Owner" />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="fc-disabled" disabled>
              Disabled label
            </Label>
            <Input id="fc-disabled" defaultValue="Locked" disabled />
          </div>
        </div>
      </Fieldset>

      <Fieldset legend="Input group" description="Prefix and suffix affixes at each control height">
        <div className="grid gap-3">
          <InputGroup size="sm" prefix="https://">
            <Input aria-label="Tenant domain" defaultValue="openrisk.io" />
          </InputGroup>
          <InputGroup size="md" prefix={<Search size={14} />} suffix="risks">
            <Input aria-label="Search risks" placeholder="Search" />
          </InputGroup>
          <InputGroup size="lg" suffix="days">
            <Input aria-label="Retention window" defaultValue="90" />
          </InputGroup>
          <InputGroup invalid suffix="%">
            <Input aria-label="Coverage target" defaultValue="140" />
          </InputGroup>
          <InputGroup disabled prefix="ID">
            <Input aria-label="Instance id" defaultValue="i-0a1b2c3d" disabled />
          </InputGroup>
        </div>
      </Fieldset>

      <Fieldset legend="Checkbox" description="Every state a checkbox can reach">
        <div className="grid gap-3">
          <Checkbox label="Unchecked" />
          <Checkbox label="Checked" defaultChecked />
          <Checkbox label="Indeterminate" indeterminate />
          <Checkbox label="With description" description="Explains the consequence of ticking it" defaultChecked />
          <Checkbox label="Disabled" disabled />
          <Checkbox label="Disabled checked" defaultChecked disabled />
        </div>
      </Fieldset>

      <CheckboxGroup
        legend="Scopes"
        description="What this API token may read"
        options={[
          { value: 'assets', label: 'Assets' },
          { value: 'risks', label: 'Risks', description: 'Includes the computed score' },
          { value: 'audit', label: 'Audit log', disabled: true },
        ]}
        value={scopes}
        onValueChange={setScopes}
      />

      <RadioGroup
        legend="Scan cadence"
        description="How often the connector re-inventories"
        options={[
          { value: 'daily', label: 'Daily', description: 'Highest freshness, highest quota use' },
          { value: 'weekly', label: 'Weekly' },
          { value: 'manual', label: 'Manual only', disabled: true },
        ]}
        value={cadence}
        onValueChange={setCadence}
      />

      <Fieldset legend="Switch" description="Both sizes, both orders, every state">
        <div className="grid gap-3">
          <Switch label="Off" />
          <Switch label="On" defaultChecked />
          <Switch size="sm" label="Small, on" defaultChecked />
          <Switch label="Control first" controlFirst defaultChecked />
          <Switch label="With description" description="Sends a digest every Monday at 08:00" defaultChecked />
          <Switch label="Disabled" disabled />
          <Switch label="Disabled on" defaultChecked disabled />
        </div>
      </Fieldset>

      <Fieldset legend="Disabled fieldset" description="Disables every control it contains" disabled>
        <div className="grid gap-3">
          <Checkbox label="Unreachable checkbox" defaultChecked />
          <Switch label="Unreachable switch" />
        </div>
      </Fieldset>
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
        <p className="mb-2 text-2xs font-semibold uppercase tracking-caps text-fg-muted">
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
        <p className="mb-2 text-2xs font-semibold uppercase tracking-caps text-fg-muted">
          Severity encodings (the risk scale, not the categorical ramp)
        </p>
        <div className="flex flex-wrap gap-3">
          {Object.entries(severity).map(([name, colour]) => (
            <span key={name} className="inline-flex items-center gap-2 text-xs text-fg-secondary">
              <span className="h-3 w-8 rounded-xs" style={{ background: colour }} />
              {name}
            </span>
          ))}
        </div>
      </div>

      <div>
        <p className="mb-2 text-2xs font-semibold uppercase tracking-caps text-fg-muted">
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
            className="rounded-md border border-subtle p-3 text-2xs text-fg-secondary"
            style={{ background: `var(--${surface})` }}
          >
            {surface}
          </div>
        ))}
      </div>
    </div>
  );
}


/**
 * The language axis. #463.
 *
 * Every label here is a REAL product string, pulled from src/locales by its key,
 * and the keys are chosen for how far French runs past English rather than at
 * random — the point is to break the layout if it is going to break:
 *
 *   common.save              "Save"       -> "Enregistrer"                 x2.75
 *   risks.riskOwner          "Owner"      -> "Propriétaire"                x2.40
 *   filters.tags             "Tags"       -> "Étiquettes"                  x2.50
 *   compliance.adminOnly     "Admin only" -> "Réservé aux administrateurs" x2.70
 *   compliance.catalog.import "Add"       -> "Ajouter"                     x2.33
 *
 * A four-character English word becoming eleven French characters is what turns
 * a comfortable button row into a wrapped one, and it happens on `Save` — the
 * most-used control in the product. Averaged strings would never show it.
 *
 * Rendering product strings rather than lorem also means this page fails when a
 * translation is DELETED: useI18n returns the key itself on a miss, so
 * "common.save" would appear on the button and the snapshot would move.
 */
function I18nPressure() {
  const { t } = useI18n();

  return (
    <div className="grid max-w-2xl gap-6">
      <div>
          <p className="mb-2 text-2xs font-semibold uppercase tracking-caps text-fg-muted">Buttons at their longest — Same row, both languages</p>
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="primary">{t('common.save')}</Button>
          <Button variant="secondary">{t('common.cancel')}</Button>
          <Button variant="secondary">{t('compliance.catalog.import')}</Button>
          <Button variant="destructive">{t('common.delete')}</Button>
          <Button variant="ghost">{t('common.filter')}</Button>
        </div>
      </div>

      <div>
          <p className="mb-2 text-2xs font-semibold uppercase tracking-caps text-fg-muted">Badges — A chip has no room to wrap</p>
        <div className="flex flex-wrap items-center gap-2">
          <Badge intent="neutral">{t('compliance.adminOnly')}</Badge>
          <Badge intent="info">{t('filters.tags')}</Badge>
          <Badge intent="success">{t('statuses.in_progress')}</Badge>
          <Badge intent="warning">{t('mitigations.deadline.today')}</Badge>
        </div>
      </div>

      <div>
          <p className="mb-2 text-2xs font-semibold uppercase tracking-caps text-fg-muted">Field labels — Label, description and error, all translated</p>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t('risks.riskOwner')} description={t('risks.dragDropHint')} required>
            <Input placeholder={t('common.search')} />
          </Field>
          <Field
            label={t('risks.acceptanceReason')}
            status="invalid"
            message={t('errors.failedToUpdateRisk')}
          >
            <Input defaultValue="" />
          </Field>
        </div>
      </div>

      <div>
          <p className="mb-2 text-2xs font-semibold uppercase tracking-caps text-fg-muted">Prose — The long strings, where wrapping is expected but overflow is not</p>
        <p className="text-sm text-fg-secondary">{t('actionCenter.emptyDescription')}</p>
        <p className="mt-2 text-sm text-fg-secondary">{t('compliance.noFrameworksDescription')}</p>
      </div>
    </div>
  );
}


/**
 * Feedback primitives — #443 PR 2.
 *
 * AlertDialog is rendered inline rather than through its trigger: the gallery
 * captures surfaces, and a dialog that has to be opened by a click is a dialog
 * whose snapshot depends on a race. Its focus and keyboard contract is asserted
 * in the unit tests, where it belongs.
 */
function Feedback2() {
  const [open, setOpen] = useState<null | 'default' | 'destructive' | 'busy'>('destructive');

  return (
    <div className="grid max-w-2xl gap-6">
      <div>
        <p className="mb-2 text-2xs font-semibold uppercase tracking-caps text-fg-muted">
          Spinner — every size, and the labelled variant
        </p>
        <div className="flex items-center gap-6 text-fg-secondary">
          <Spinner size="xs" />
          <Spinner size="sm" />
          <Spinner size="md" />
          <Spinner size="lg" />
          <Spinner size="md" label="Loading results" />
        </div>
      </div>

      <div>
        <p className="mb-2 text-2xs font-semibold uppercase tracking-caps text-fg-muted">
          LoadingState — the block that uses the atom
        </p>
        <div className="rounded-md border border-subtle">
          <LoadingState />
        </div>
      </div>

      <div>
        <p className="mb-2 text-2xs font-semibold uppercase tracking-caps text-fg-muted">
          AlertDialog — open one at a time
        </p>
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" onClick={() => setOpen('default')}>
            Default
          </Button>
          <Button variant="destructive" onClick={() => setOpen('destructive')}>
            Destructive
          </Button>
          <Button variant="secondary" onClick={() => setOpen('busy')}>
            In flight
          </Button>
        </div>
      </div>

      <AlertDialog
        open={open === 'default'}
        onCancel={() => setOpen(null)}
        onConfirm={() => setOpen(null)}
        title="Publish this framework?"
        description="Every control becomes visible to the whole tenant."
        confirmLabel="Publish"
      />
      <AlertDialog
        open={open === 'destructive'}
        onCancel={() => setOpen(null)}
        onConfirm={() => setOpen(null)}
        tone="destructive"
        title="Delete ISO 27001:2022?"
        description="This removes the framework and all 93 of its controls. It cannot be undone."
        confirmLabel="Delete framework"
      />
      <AlertDialog
        open={open === 'busy'}
        onCancel={() => setOpen(null)}
        onConfirm={() => setOpen(null)}
        tone="destructive"
        busy
        title="Revoking the token…"
        description="The dialog cannot be dismissed while the action is in flight."
        confirmLabel="Revoke"
      />
    </div>
  );
}


/**
 * Floating layers — #443 PR 3.
 *
 * Both are rendered OPEN via controlled state so the snapshot captures the
 * surface rather than a click race. Their keyboard contracts — roving tab stop,
 * typeahead, focus return — are asserted in the unit tests, where behaviour
 * belongs; what a picture can show is the panel, the separator and the
 * destructive item's colour.
 */
function Floating() {
  const [popoverOpen, setPopoverOpen] = useState(true);

  return (
    <div className="grid max-w-2xl gap-6 pb-72">
      <div>
        <p className="mb-2 text-2xs font-semibold uppercase tracking-caps text-fg-muted">
          Popover — content, not a description
        </p>
        <Popover
          open={popoverOpen}
          onOpenChange={setPopoverOpen}
          label="Filters"
          trigger={<Button variant="secondary">Filters</Button>}
        >
          <Field label="Owner">
            <Input placeholder="Anyone" />
          </Field>
        </Popover>
      </div>

      <div>
        <p className="mb-2 text-2xs font-semibold uppercase tracking-caps text-fg-muted">
          Menu — actions, with destructive last behind a separator
        </p>
        <Menu
          trigger={<Button variant="secondary">Row actions</Button>}
          items={[
            { label: 'Duplicate', onSelect: () => {}, icon: Plus },
            { label: 'Export as CSV', onSelect: () => {}, icon: Download },
            { label: 'Archive', onSelect: () => {}, disabled: true },
            { label: 'Delete risk', onSelect: () => {}, icon: Trash2, destructive: true },
          ]}
        />
      </div>
    </div>
  );
}

/**
 * The table, at whatever density `?density=` asked for. #443 PR 4.
 *
 * Small on purpose: eight rows is enough to measure a row height and see the
 * header, and a virtualised body of ten thousand would make the snapshot about
 * the virtualiser rather than about density.
 */
interface DemoRow {
  id: string;
  title: string;
  owner: string;
  score: number;
}

const DEMO_ROWS: DemoRow[] = [
  { id: 'R-001', title: 'Unpatched TLS on the edge gateway', owner: 'A. Diallo', score: 8.4 },
  { id: 'R-002', title: 'Shared admin credentials in CI', owner: 'M. Nkemi', score: 7.1 },
  { id: 'R-003', title: 'No offsite backup for the audit trail', owner: 'S. Traoré', score: 6.8 },
  { id: 'R-004', title: 'Vendor access without an NDA', owner: 'A. Diallo', score: 5.2 },
  { id: 'R-005', title: 'Laptop disk encryption not enforced', owner: 'K. Mbala', score: 4.9 },
  { id: 'R-006', title: 'Log retention below the 12-month floor', owner: 'S. Traoré', score: 3.6 },
  { id: 'R-007', title: 'Stale IAM roles after offboarding', owner: 'M. Nkemi', score: 3.1 },
  { id: 'R-008', title: 'No documented restore drill', owner: 'K. Mbala', score: 2.4 },
];

function TableDensityInner() {
  const api = useTableState();
  return (
    <div className="max-w-3xl">
      <p className="mb-2 text-2xs font-semibold uppercase tracking-caps text-fg-muted">
        DataTable — row height follows --den-row
      </p>
      <DataTable<DemoRow>
        id="gallery-density"
        rows={DEMO_ROWS}
        rowKey={(r) => r.id}
        api={api}
        mode="client"
        ariaLabel="Risk register"
        exportFilename=""
        columns={[
          { key: 'id', header: 'ID', width: 90, render: (r) => r.id },
          { key: 'title', header: 'Risk', render: (r) => r.title },
          { key: 'owner', header: 'Owner', width: 140, render: (r) => r.owner },
          {
            key: 'score',
            header: 'Score',
            width: 90,
            align: 'right',
            render: (r) => r.score.toFixed(1),
          },
        ]}
      />
    </div>
  );
}

/* useTableState reads the table's sort/page/filters from the URL, so it needs a
   router. MemoryRouter rather than the app's: the gallery has no routes, and a
   real one would make the snapshot depend on the address bar. */
function TableDensity() {
  return (
    <MemoryRouter>
      <TableDensityInner />
    </MemoryRouter>
  );
}

export const GALLERIES: Record<string, () => JSX.Element> = {
  table: TableDensity,
  floating: Floating,
  feedback2: Feedback2,
  i18n: I18nPressure,
  controls: Controls,
  forms: Forms,
  'form-controls': FormControls,
  states: States,
  charts: Charts,
  feedback: Feedback,
};

function Gallery() {
  const id = new URLSearchParams(window.location.search).get('gallery') ?? '';
  const Render = GALLERIES[id];

  return (
    <main className="min-h-screen bg-surface-0 p-8 text-fg-primary">
      <h1 className="mb-1 text-xl font-semibold tracking-display">{id || 'Design system'}</h1>
      <p className="mb-6 text-sm text-fg-secondary">
        Every variant and state, rendered against the real stylesheet.
      </p>
      {Render ? (
        <Render />
      ) : (
        <ul className="text-sm text-fg-secondary">
          {Object.keys(GALLERIES).map((key) => (
            <li key={key}>{key}</li>
          ))}
        </ul>
      )}
    </main>
  );
}

queueMicrotask(applyRequestedTheme);

/* Before render: the store owns theme AND language, so `setLang`'s applyDom
   cannot overwrite the theme the HTML stamped. See harnessEnv.ts. */
applyHarnessEnv();

createRoot(document.getElementById('gallery-root')!).render(
  <StrictMode>
    <Gallery />
  </StrictMode>,
);
