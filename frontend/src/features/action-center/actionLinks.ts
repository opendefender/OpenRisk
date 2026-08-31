// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Where an action item takes you, and how this panel refuses to render one that
// goes nowhere.
//
// THE RULE (CLAUDE.md rule 12, and acceptance criterion 6 of #430): no item is
// ever rendered with a dead control. The server sends `deep_link` already built;
// this module's only job is to prove that the path it sent resolves to a route
// that actually exists before an anchor is drawn around it. An item whose link
// does not resolve is dropped and logged, never shown as an inert row.
//
// It is also the trust boundary for the field. `deep_link` is server data, but
// it ends up in an href, so it is checked like any other untrusted string: a
// value that is not a same-origin absolute path — "https://…", "//evil.example",
// "javascript:…" — is rejected outright rather than turned into a link off the
// application.
//
// The six paths, per the #429 contract:
//
//   overdue_mitigation   1  /risks/mitigations/{id}
//   critical_risk        2  /risks?drawer=risk&entity={id}
//   pending_approval     3  /governance
//   open_incident        4  /incidents/{id}/war-room
//   expiring_evidence    5  /compliance/evidence?drawer=evidence&entity={id}
//   overdue_remediation  6  /compliance/remediation/{id}
//
// Risks and evidence carry ?drawer=&entity= against a LIST route on purpose:
// neither has a detail page, and the universal entity drawer is the product's
// only shareable URL for them. Rewriting those two into /risks/{id} form would
// produce a 404, which is why this module validates the pathname only and
// carries the query string through untouched.

import {
  AlertTriangle,
  CalendarClock,
  ClipboardCheck,
  ListChecks,
  ShieldCheck,
  Siren,
  Wrench,
  type LucideIcon,
} from 'lucide-react';

import { resolveRoute } from '../../shared/routeModel';
import type { ActionItem, ActionItemType } from './actionCenterService';

/**
 * A `deep_link` that has been proved to resolve, or null.
 *
 * Returns the original string when it is a same-origin absolute path whose
 * pathname matches a node in the route tree (`shared/routeModel.ts`), and null
 * otherwise. Null is the caller's signal to drop the item.
 */
export function safeDeepLink(deepLink: string | undefined | null): string | null {
  if (typeof deepLink !== 'string') return null;
  const link = deepLink.trim();

  // Same-origin absolute path only. '//host' is protocol-relative and leaves the
  // origin; anything without a leading slash could carry a scheme.
  if (!link.startsWith('/') || link.startsWith('//')) return null;

  // The route tree matches pathnames. Query and fragment are the destination
  // screen's business (the entity drawer reads ?drawer=&entity=).
  const pathname = link.split(/[?#]/, 1)[0];
  if (!resolveRoute(pathname)) return null;

  return link;
}

/** How a category is drawn. `labelKey` is a key under `actionCenter.types`. */
export interface ActionTypePresentation {
  icon: LucideIcon;
  labelKey: string;
  /** Tailwind token class for the glyph. Never a raw colour. */
  tone: string;
}

/**
 * One entry per category the contract declares. Exhaustive by type: adding a
 * seventh category to the OpenAPI enum fails the build here rather than
 * rendering a nameless row in production.
 */
export const ACTION_TYPE_PRESENTATION: Record<ActionItemType, ActionTypePresentation> = {
  overdue_mitigation: { icon: ShieldCheck, labelKey: 'overdue_mitigation', tone: 'text-danger-text' },
  critical_risk: { icon: AlertTriangle, labelKey: 'critical_risk', tone: 'text-danger-text' },
  pending_approval: { icon: ClipboardCheck, labelKey: 'pending_approval', tone: 'text-warning-text' },
  open_incident: { icon: Siren, labelKey: 'open_incident', tone: 'text-danger-text' },
  expiring_evidence: { icon: CalendarClock, labelKey: 'expiring_evidence', tone: 'text-warning-text' },
  overdue_remediation: { icon: Wrench, labelKey: 'overdue_remediation', tone: 'text-warning-text' },
};

/** The fallback for a category this build has never heard of. */
const UNKNOWN_PRESENTATION: ActionTypePresentation = {
  icon: ListChecks,
  labelKey: 'unknown',
  tone: 'text-fg-muted',
};

/**
 * The presentation for a type, tolerating a value this build does not know.
 *
 * A deployed frontend can be one release behind the API. An unrecognised type
 * is NOT a reason to hide work the user has to do: the item's title and its
 * link are the server's own, both still real, so it renders under a generic
 * label. What it may never do is render without a working link — that is
 * `safeDeepLink`'s job, applied to every item regardless of type.
 */
export function presentationFor(type: string): ActionTypePresentation {
  return (
    ACTION_TYPE_PRESENTATION[type as ActionItemType] ?? UNKNOWN_PRESENTATION
  );
}

/** An item that has earned a row: it has a link that goes somewhere real. */
export interface LinkableActionItem {
  item: ActionItem;
  href: string;
}

/**
 * Drops every item whose link does not resolve, preserving the server's order.
 *
 * The warning is deliberate and per-item: a dropped row is a contract
 * disagreement between this build and the API, and it should be visible in the
 * console of whoever hits it rather than inferred from a short list.
 */
export function linkableItems(items: readonly ActionItem[]): LinkableActionItem[] {
  const out: LinkableActionItem[] = [];
  for (const item of items) {
    const href = safeDeepLink(item.deep_link);
    if (!href) {
      console.warn(
        `[action-center] dropping item ${item.id}: deep_link "${item.deep_link}" does not resolve to a known route`,
      );
      continue;
    }
    out.push({ item, href });
  }
  return out;
}
