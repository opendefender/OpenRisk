// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The universal entity drawer's wire contract (W1-02).
//
// These types mirror internal/application/entity exactly. They are hand-written
// rather than generated because the drawer endpoints are not in openapi.yaml
// yet; every field below corresponds to a struct tag in that package, and the
// drawer renders nothing that is not in here. There is no `any`.

/** The eight drawer-addressable types. Two are aliases onto real storage:
 *  `finding` is a Vulnerability (there is no findings table) and `vendor` is an
 *  Asset of category vendor (there is no vendors table). */
export type EntityType =
  | 'asset'
  | 'risk'
  | 'vulnerability'
  | 'finding'
  | 'control'
  | 'incident'
  | 'vendor'
  | 'evidence';

export const ENTITY_TYPES: readonly EntityType[] = [
  'asset', 'risk', 'vulnerability', 'finding',
  'control', 'incident', 'vendor', 'evidence',
] as const;

export function isEntityType(v: string | null | undefined): v is EntityType {
  return !!v && (ENTITY_TYPES as readonly string[]).includes(v);
}

/** Which region of the drawer. The server says which ones a type — and this
 *  caller — actually has, so a tab that would always be empty or always 403 is
 *  never rendered. */
export type EntitySection = 'summary' | 'relations' | 'timeline' | 'audit';

/** Design-system badge intents. */
export type Tone =
  | 'critical' | 'high' | 'medium' | 'low'
  | 'success' | 'warning' | 'info' | 'neutral';

export interface Chip {
  value: string;
  label: string;
  tone: Tone;
}

export type FieldKind =
  | 'text' | 'date' | 'user' | 'badge' | 'number'
  | 'money' | 'link' | 'tags' | 'boolean' | 'multiline';

export interface EntityField {
  key: string;
  label: string;
  value: string;
  kind: FieldKind;
  tone?: string;
  values?: string[];
  href?: string;
}

/**
 * A business score the entity actually carries.
 *
 * `available: false` is the honest answer for an entity that has no score of
 * this kind — an unscored risk, a control (which has a state, not a number), an
 * artifact. The UI must render `unavailable`, never a 0.
 */
export interface EntityScore {
  available: boolean;
  key: string;
  label: string;
  value: number;
  max: number;
  tone?: Tone;
  basis?: string;
  unavailable?: string;
}

export interface EntityActor {
  id?: string;
  email?: string;
  label?: string;
}

export interface EntitySummary {
  type: EntityType;
  id: string;
  title: string;
  subtitle?: string;
  type_label: string;
  status?: Chip;
  severity?: Chip;
  score: EntityScore;
  owner?: EntityActor;
  fields: EntityField[];
  created_at?: string;
  updated_at?: string;
  /** The canonical in-app deep link that opens this drawer. */
  url: string;
  sections: EntitySection[];
}

export type ActionKind = 'primary' | 'secondary' | 'danger';

/**
 * Something the caller may do. Only ALLOWED actions come back — a control that
 * advertises a permission the user does not hold is the "button that lies" the
 * project forbids. `method` + `path` name the endpoint that performs it.
 */
export interface EntityAction {
  key: string;
  label: string;
  kind: ActionKind;
  method: string;
  path: string;
  permission: string;
}

export interface EntityView {
  summary: EntitySummary;
  actions: EntityAction[];
  sections: EntitySection[];
}

export interface EntityRelation {
  type: EntityType;
  id: string;
  title: string;
  subtitle?: string;
  status?: Chip;
  severity?: Chip;
  relation_label?: string;
  /** Opens this relation's own drawer. */
  url: string;
}

export interface RelationGroup {
  key: string;
  label: string;
  target_type: EntityType;
  items: EntityRelation[];
  total: number;
  /** True when `total` exceeds what `items` carries. */
  truncated: boolean;
  /** The caller may not read this target type. Items and total are both empty:
   *  the count alone would say how many exist. */
  denied: boolean;
  /** This one group's source failed. The rest of the drawer is still usable. */
  error?: string;
}

export interface RelationsResponse {
  groups: RelationGroup[];
}

export type TimelineSourceName =
  | 'audit' | 'risk_history' | 'incident_timeline' | 'asset_snapshot';

export interface TimelineChange {
  field: string;
  from?: string;
  to?: string;
}

export interface TimelineEvent {
  id: string;
  kind: string;
  occurred_at: string;
  actor?: EntityActor;
  target: { type?: EntityType; id: string };
  summary: string;
  /** Field NAMES only. The before/after values live in the audit tab, behind
   *  its own permission. */
  changes?: TimelineChange[];
  source: TimelineSourceName;
  target_url?: string;
}

export interface TimelinePage {
  events: TimelineEvent[];
  next_cursor?: string;
  /** Which journals contributed, so a reader can see that one was consulted and
   *  simply had nothing to add. */
  sources: TimelineSourceName[];
}

/** A raw audit record — before/after included. Distinct from a timeline event
 *  (§23) and gated by its own permission. */
export interface AuditRecord {
  id: string;
  actor_id?: string;
  actor_email?: string;
  action: string;
  entity_type: string;
  entity_id: string;
  summary: string;
  before?: Record<string, unknown>;
  after?: Record<string, unknown>;
  changed_fields?: string[];
  ip_address?: string;
  user_agent?: string;
  request_id?: string;
  method?: string;
  path?: string;
  status_code?: number;
  source?: string;
  sequence: number;
  created_at: string;
}

export interface AuditPage {
  events: AuditRecord[];
  total: number;
  limit: number;
  offset: number;
}

export interface EntityCatalogueEntry {
  type: EntityType;
  label: string;
  list_path: string;
  permission: string;
  readable: boolean;
  sections: EntitySection[];
}
