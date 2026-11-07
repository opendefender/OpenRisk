#!/usr/bin/env node
// Development dataset for the local stack (#1337).
//
//   make seed-dev                       # against the compose stack
//   SEED_ADMIN_PASSWORD=… node scripts/seed-dev.mjs
//
// Fills one organisation with enough data to open every screen on something
// other than its empty state, plus a second, small organisation to check tenant
// isolation by hand. Everything goes through the public API as the seed admin,
// so scores, bands, audit trail, activity and notifications are produced by the
// real use cases. Nothing is written to the database behind the API's back: a
// record the API refuses is reported in the summary with the server's answer.
//
// Idempotent: every record is looked up by its natural key (title, name, email)
// before it is created, so a second run creates nothing.
//
// The persona passwords live in dev/fixtures/dev-dataset.json and are printed
// at the end, which is the point of the script. That is why it refuses any API
// that is not on this machine unless SEED_DEV_ALLOW_REMOTE=true.

import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const DATASET_FILE = path.resolve(HERE, '../dev/fixtures/dev-dataset.json');
const STATE_FILE = path.resolve(HERE, '../dev/.seed-dev-state.json');

const API_URL = (process.env.SEED_API_URL || 'http://localhost:8080/api/v1').replace(/\/$/, '');
const ADMIN_EMAIL = process.env.SEED_ADMIN_EMAIL || 'admin@opendefender.io';
const ADMIN_PASSWORD = process.env.SEED_ADMIN_PASSWORD || '';

const dataset = JSON.parse(fs.readFileSync(DATASET_FILE, 'utf8'));

/* ------------------------------------------------------------------ guards */

function assertLocalTarget() {
  const host = new URL(API_URL).hostname;
  // Literal loopback only. A DNS name such as the compose service `backend`
  // could resolve through a search domain to another machine.
  const local = ['localhost', '127.0.0.1', '::1', '[::1]'].includes(host);
  if (!local && process.env.SEED_DEV_ALLOW_REMOTE !== 'true') {
    fail(
      `refusing to seed ${host}: this script creates accounts with published passwords. ` +
        'Set SEED_DEV_ALLOW_REMOTE=true if this really is a throwaway stack.',
    );
  }
}

function fail(msg) {
  console.error(`[seed-dev] ${msg}`);
  process.exit(1);
}

/* -------------------------------------------------------------- reporting */

const summary = new Map(); // module -> {created, existing, skipped: []}
function tally(module) {
  if (!summary.has(module)) summary.set(module, { created: 0, existing: 0, skipped: [] });
  return summary.get(module);
}
const created = (m) => tally(m).created++;
const existing = (m) => tally(m).existing++;
function skipped(m, what, res) {
  const why = res
    ? `${res.status} ${typeof res.body === 'string' ? res.body : JSON.stringify(res.body)}`.slice(0, 220)
    : '';
  tally(m).skipped.push(why ? `${what}: ${why}` : what);
}

/* --------------------------------------------------------------------- api */

let TOKEN = '';

async function api(method, endpoint, body, opts = {}) {
  const headers = { ...(opts.token ?? TOKEN ? { Authorization: `Bearer ${opts.token ?? TOKEN}` } : {}) };
  let payload;
  if (body instanceof FormData) {
    payload = body;
  } else if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
    payload = JSON.stringify(body);
  }
  for (let attempt = 1; ; attempt++) {
    // No redirects: a proxy in the way must not forward passwords elsewhere.
    const res = await fetch(`${API_URL}${endpoint}`, { method, headers, body: payload, redirect: 'manual' });
    const text = await res.text();
    let json;
    try {
      json = text ? JSON.parse(text) : null;
    } catch {
      json = text;
    }
    // The API rate-limits writes; a seed is a burst by nature.
    if (res.status === 429 && attempt < 12) {
      const retryAfter = Number(res.headers.get('retry-after'));
      await sleep(Number.isFinite(retryAfter) && retryAfter > 0 ? retryAfter * 1000 : attempt * 3000);
      continue;
    }
    return { status: res.status, ok: res.ok, body: json };
  }
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

/** The list endpoints do not agree on an envelope. */
function asArray(body) {
  if (Array.isArray(body)) return body;
  if (!body || typeof body !== 'object') return [];
  for (const k of [
    'data', 'items', 'results', 'risks', 'assets', 'users', 'teams', 'frameworks', 'controls',
    'vulnerabilities', 'incidents', 'mitigations', 'evidence', 'audits', 'remediations', 'vendors',
    'templates', 'assessments', 'reports', 'rules', 'workflows', 'approvals', 'delegations',
    'fields', 'custom_fields', 'views', 'dependencies', 'members', 'actions', 'categories',
  ]) {
    if (Array.isArray(body[k])) return body[k];
    if (body[k] && typeof body[k] === 'object' && Array.isArray(body[k].items)) return body[k].items;
  }
  return [];
}

function idOf(body) {
  if (!body || typeof body !== 'object') return undefined;
  if (body.id !== undefined && body.id !== null) return String(body.id);
  for (const v of Object.values(body)) {
    if (v && typeof v === 'object' && !Array.isArray(v) && v.id !== undefined) return String(v.id);
  }
  return undefined;
}

/**
 * Reads every page of a list endpoint. They paginate three ways (page and
 * page_size, limit and offset, a capped limit), and a seed that reads only the
 * first page of the risk register creates the second page again on every run.
 */
async function listAll(endpoint) {
  const sep = endpoint.includes('?') ? '&' : '?';
  const size = 100;
  const out = [];
  for (let page = 1; page <= 50; page++) {
    const q = `page=${page}&page_size=${size}&limit=${size}&offset=${(page - 1) * size}`;
    const res = await api('GET', `${endpoint}${sep}${q}`);
    if (!res.ok) break;
    const items = asArray(res.body);
    out.push(...items);
    const total = typeof res.body?.total === 'number' ? res.body.total : undefined;
    if (!items.length) break;
    // A list that caps its page below `size` still reports the real total, so
    // trust the total when there is one; a short page ends only an untotalled list.
    if (total !== undefined ? out.length >= total : items.length < size) break;
  }
  return out;
}

const isoIn = (days) => new Date(Date.now() + days * 86400000).toISOString();
const dateIn = (days) => isoIn(days).slice(0, 10);

/* -------------------------------------------------------------------- auth */

function totp(base32Secret) {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  let bits = '';
  for (const ch of base32Secret.replace(/=+$/, '').toUpperCase()) {
    const idx = alphabet.indexOf(ch);
    if (idx >= 0) bits += idx.toString(2).padStart(5, '0');
  }
  const key = Buffer.from((bits.match(/.{8}/g) ?? []).map((b) => parseInt(b, 2)));
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30000)));
  const h = crypto.createHmac('sha1', key).update(counter).digest();
  const o = h[h.length - 1] & 0x0f;
  return String((h.readUInt32BE(o) & 0x7fffffff) % 1_000_000).padStart(6, '0');
}

/** Signs in and returns an access token, answering an MFA challenge if one is set. */
async function signIn(email, password, totpSecret) {
  const res = await api('POST', '/auth/login', { email, password }, { token: '' });
  if (!res.ok) return { error: res };
  if (res.body?.token_pair?.access_token) return { token: res.body.token_pair.access_token, body: res.body };
  if (res.body?.mfa_required) {
    if (!totpSecret) return { error: { status: 'mfa', body: 'MFA challenge and no TOTP secret' } };
    const ch = await api('POST', '/auth/mfa/challenge', { code: totp(totpSecret) }, { token: res.body.mfa_token });
    if (ch.ok && ch.body?.token_pair?.access_token) return { token: ch.body.token_pair.access_token, body: ch.body };
    return { error: ch };
  }
  return { error: { status: 'mfa', body: res.body } };
}

/* ------------------------------------------------------------------ state */

const ids = {
  users: {}, // persona key -> user id
  assets: {}, // asset name -> id
  risks: {}, // title -> id
  frameworks: {}, // name -> id
  controls: {}, // framework name -> [control]
  audits: {},
  templates: {},
};

/* ================================================================ modules */

async function seedOrganization() {
  const m = 'organisation';
  const org = await api('GET', '/organization');
  const current = org.body?.organization?.name ?? org.body?.name;
  if (current === dataset.organization.name) existing(m);
  else {
    const res = await api('PUT', '/organization', { name: dataset.organization.name });
    res.ok ? created(m) : skipped(m, 'rename', res);
  }
  const pol = await api('GET', '/security/mfa-policy');
  if (pol.body?.grace_days === dataset.organization.mfa_grace_days) existing(m);
  else {
    const res = await api('PUT', '/security/mfa-policy', { grace_days: dataset.organization.mfa_grace_days });
    res.ok ? created(m) : skipped(m, 'MFA grace period', res);
  }
}

async function seedPersonas() {
  const m = 'membres';
  const users = await listAll('/users');
  const byEmail = new Map(users.map((u) => [u.email, u]));
  for (const p of dataset.personas) {
    const found = byEmail.get(p.email);
    if (found) {
      ids.users[p.key] = found.id;
      existing(m);
      continue;
    }
    const res = await api('POST', '/users', {
      email: p.email,
      username: p.username,
      full_name: p.full_name,
      password: p.password,
      role: p.role,
      business_role: p.business_role,
      department: p.department,
    });
    if (res.ok) {
      ids.users[p.key] = idOf(res.body);
      created(m);
    } else skipped(m, p.email, res);
  }
  const me = await api('GET', '/auth/me');
  ids.users.admin = me.body?.user?.id ?? me.body?.id;
  for (const p of dataset.personas) await completeOnboarding(p);
}

/**
 * Onboarding state is per user. Without this, every persona's first sign-in
 * lands on the setup wizard instead of the screen being tested. Done through
 * the persona's own session, as the wizard's last step would.
 */
async function completeOnboarding(p) {
  const m = 'onboarding des comptes';
  const session = await signIn(p.email, p.password);
  if (session.error) {
    skipped(m, `${p.email} cannot sign in`, session.error);
    return;
  }
  const state = await api('GET', '/onboarding/state', undefined, { token: session.token });
  if (state.body?.completed) {
    existing(m);
    return;
  }
  const res = await api('POST', '/onboarding/complete', {}, { token: session.token });
  res.ok ? created(m) : skipped(m, p.email, res);
}

const userId = (key) => (key ? ids.users[key] : undefined);

async function seedTeams() {
  const m = 'équipes';
  const teams = await listAll('/teams');
  for (const t of dataset.teams) {
    let team = teams.find((x) => x.name === t.name);
    if (team) existing(m);
    else {
      const res = await api('POST', '/teams', { name: t.name, description: t.description });
      if (!res.ok) {
        skipped(m, t.name, res);
        continue;
      }
      team = { id: idOf(res.body) };
      created(m);
    }
    const detail = await api('GET', `/teams/${team.id}`);
    const memberIds = new Set(
      asArray(detail.body?.members ?? detail.body?.team?.members ?? []).map((x) => x.user_id ?? x.id),
    );
    for (const key of t.members) {
      const uid = userId(key);
      if (!uid || memberIds.has(uid)) continue;
      const res = await api('POST', `/teams/${team.id}/members/${uid}`, {});
      if (!res.ok && res.status !== 409) skipped(m, `${t.name} ← ${key}`, res);
    }
  }
}

async function seedCustomFields() {
  const m = 'champs personnalisés';
  const fields = await listAll('/custom-fields');
  for (const f of dataset.custom_fields) {
    if (fields.some((x) => x.name === f.name)) {
      existing(m);
      continue;
    }
    const res = await api('POST', '/custom-fields', f);
    res.ok ? created(m) : skipped(m, f.name, res);
  }
}

async function seedAssets(assetsSpec, module = 'actifs') {
  const assets = await listAll('/assets');
  const byName = new Map(assets.map((a) => [a.name, a]));
  for (const a of assetsSpec) {
    const found = byName.get(a.name);
    if (found) {
      ids.assets[a.name] = found.id;
      existing(module);
      continue;
    }
    const owner = dataset.personas.find((p) => p.key === a.owner)?.email ?? a.owner;
    const res = await api('POST', '/assets', {
      name: a.name,
      type: a.type,
      category: a.category,
      criticality: a.criticality,
      owner,
      attributes: a.attributes,
    });
    if (res.ok) {
      ids.assets[a.name] = idOf(res.body);
      created(module);
    } else skipped(module, a.name, res);
  }
}

async function seedDependencies() {
  const m = 'dépendances';
  const deps = await listAll('/asset-dependencies');
  const key = (s, t, ty) => `${s}|${t}|${ty}`;
  const have = new Set(deps.map((d) => key(d.source_asset_id, d.target_asset_id, d.type)));
  for (const d of dataset.dependencies) {
    const s = ids.assets[d.source];
    const t = ids.assets[d.target];
    if (!s || !t) {
      skipped(m, `${d.source} → ${d.target}: asset missing`);
      continue;
    }
    if (have.has(key(s, t, d.type))) {
      existing(m);
      continue;
    }
    const res = await api('POST', '/asset-dependencies', {
      source_asset_id: s,
      target_asset_id: t,
      type: d.type,
      description: d.description,
    });
    if (res.ok) created(m);
    else if (res.status === 409) existing(m);
    else skipped(m, `${d.source} → ${d.target}`, res);
  }
}

async function seedVendorLinks() {
  const m = 'fournisseurs ↔ actifs';
  for (const l of dataset.vendor_links) {
    const v = ids.assets[l.vendor];
    const a = ids.assets[l.asset];
    if (!v || !a) {
      skipped(m, `${l.vendor} → ${l.asset}: missing`);
      continue;
    }
    const res = await api('POST', `/vendors/${v}/assets`, { asset_id: a, verb: l.verb, description: l.description });
    if (res.ok) created(m);
    else if (res.status === 409) existing(m);
    else skipped(m, `${l.vendor} → ${l.asset}`, res);
  }
}

/* ---- risks and their lifecycle ---- */

// Same table as domain.riskStateTransitions (backend/internal/domain/risk_state.go).
const STATE_EDGES = {
  draft: ['identified'],
  identified: ['assessed', 'closed'],
  assessed: ['treatment_planned', 'closed'],
  treatment_planned: ['in_treatment', 'assessed', 'closed'],
  in_treatment: ['mitigated', 'residual_accepted', 'treatment_planned'],
  residual_accepted: ['closed', 'in_treatment', 'assessed'],
  mitigated: ['closed', 'in_treatment'],
  closed: ['reopened'],
  reopened: ['assessed', 'treatment_planned', 'in_treatment', 'closed'],
};

function statePath(from, to) {
  if (from === to) return [];
  const prev = { [from]: null };
  const queue = [from];
  while (queue.length) {
    const cur = queue.shift();
    for (const next of STATE_EDGES[cur] ?? []) {
      if (next in prev) continue;
      prev[next] = cur;
      if (next === to) {
        const out = [to];
        for (let s = cur; s !== from; s = prev[s]) out.unshift(s);
        return out;
      }
      queue.push(next);
    }
  }
  return null;
}

// The stored state is not on every risk payload under the same name.
const stateOf = (r) => r?.state ?? r?.lifecycle_state ?? r?.risk_state;

async function seedRisks() {
  const m = 'risques';
  const risks = await listAll('/risks');
  const byTitle = new Map(risks.map((r) => [r.title, r]));
  const fin = [
    'sle_xaf', 'aro', 'downtime_hours', 'hourly_downtime_cost_xaf', 'data_loss_cost_xaf', 'fines_xaf',
    'other_direct_cost_xaf', 'remediation_cost_xaf', 'mitigation_effectiveness',
  ];
  for (const r of dataset.risks) {
    const found = byTitle.get(r.title);
    if (found) {
      ids.risks[r.title] = found.id;
      existing(m);
      continue;
    }
    const body = {
      title: r.title,
      description: r.description,
      probability: r.probability,
      impact: r.impact,
      tags: r.tags,
      frameworks: r.frameworks,
      asset_ids: r.assets.map((n) => ids.assets[n]).filter(Boolean),
    };
    for (const k of fin) if (r[k] !== undefined) body[k] = r[k];
    if (r.custom) body.custom_fields = r.custom;
    const res = await api('POST', '/risks', body);
    if (!res.ok) {
      skipped(m, r.title, res);
      continue;
    }
    ids.risks[r.title] = idOf(res.body);
    created(m);
    const owner = userId(r.owner);
    if (owner) {
      const o = await api('PATCH', `/risks/${ids.risks[r.title]}`, { owner_id: owner });
      if (!o.ok) skipped('propriétaires de risque', r.title, o);
    }
  }
}

/** Moves each risk to its target state. Runs after the mitigations exist. */
async function seedRiskStates() {
  const m = 'cycle de vie des risques';
  for (const r of dataset.risks) {
    const id = ids.risks[r.title];
    if (!id || !r.state) continue;
    const cur = await api('GET', `/risks/${id}`);
    const from = stateOf(cur.body?.risk ?? cur.body) ?? 'identified';
    const steps = statePath(from, r.state);
    if (steps === null) {
      skipped(m, `${r.title}: no path ${from} → ${r.state}`);
      continue;
    }
    if (!steps.length) {
      existing(m);
      if (r.state === 'residual_accepted') existing('approbations d\'acceptation');
      continue;
    }
    let ok = true;
    for (const to of steps) {
      if (to === 'residual_accepted') {
        const why = await approveAcceptance(id, r.title);
        if (why) {
          skipped(m, `${r.title}: ${why}`);
          ok = false;
          break;
        }
      }
      const body = { to, comment: 'Jeu de données de développement' };
      if (to === 'residual_accepted' && r.acceptance_expires_at) {
        const days = Number(String(r.acceptance_expires_at).replace('+', '').replace('d', ''));
        body.acceptance_expires_at = dateIn(days);
      }
      const res = await api('POST', `/risks/${id}/transition`, body);
      if (!res.ok) {
        skipped(m, `${r.title}: ${from} → ${to}`, res);
        ok = false;
        break;
      }
    }
    if (ok) created(m);
  }
}

/**
 * Residual risk can only be accepted on a validated governance approval, and
 * the requester may not sign it (four-eyes). The admin raises the request, the
 * RSSI persona signs it with her own session. Returns why it failed, or ''.
 */
async function approveAcceptance(riskId, title) {
  const approverKey = dataset.acceptance_approver;
  const approver = dataset.personas.find((p) => p.key === approverKey);
  if (!approver) return 'no acceptance approver persona';
  const session = await signIn(approver.email, approver.password);
  if (session.error) return `approver ${approver.email} cannot sign in (${session.error.status})`;

  const res = await api('POST', '/governance/approvals', {
    entity_type: 'risk_acceptance',
    entity_id: riskId,
    action: 'accept',
    title: `Acceptation du risque résiduel — ${title}`,
    description: 'Demande créée par le jeu de données de développement.',
  });
  if (!res.ok) return `approval request: ${res.status} ${JSON.stringify(res.body).slice(0, 160)}`;
  const reqId = idOf(res.body);
  for (let i = 0; i < 3; i++) {
    const d = await api(
      'POST',
      `/governance/approvals/${reqId}/decide`,
      { decision: 'approve', comment: 'Risque résiduel compris et accepté.' },
      { token: session.token },
    );
    if (!d.ok) return `decide: ${d.status} ${JSON.stringify(d.body).slice(0, 160)}`;
    const status = d.body?.status ?? d.body?.request?.status;
    if (status === 'approved') break;
  }
  created('approbations d\'acceptation');
  return '';
}

async function seedMitigations() {
  const m = 'mitigations';
  const all = await listAll('/mitigations');
  for (const mi of dataset.mitigations) {
    const riskId = ids.risks[mi.risk];
    if (!riskId) {
      skipped(m, `${mi.title}: risk missing`);
      continue;
    }
    let mit = all.find((x) => x.title === mi.title);
    if (mit) {
      existing(m);
      continue;
    }
    const assignee = userId(mi.assignee);
    const res = await api('POST', `/risks/${riskId}/mitigations`, {
      title: mi.title,
      assigned_to: assignee ? [assignee] : [],
      assignee_id: assignee,
      owner_id: assignee,
      due_date: isoIn(mi.due_in_days ?? 30),
      sub_actions: (mi.sub_actions ?? []).map((s) => ({ title: s.title })),
    });
    if (!res.ok) {
      skipped(m, mi.title, res);
      continue;
    }
    created(m);
    const mid = idOf(res.body);
    const patch = { progress: mi.progress ?? 0 };
    if (mi.cost) patch.cost = mi.cost;
    const p = await api('PATCH', `/mitigations/${mid}`, patch);
    if (!p.ok) skipped('avancement des mitigations', mi.title, p);
    const done = (mi.sub_actions ?? []).filter((s) => s.done).map((s) => s.title);
    if (done.length) {
      const detail = await api('GET', `/mitigations/${mid}`);
      const subs = asArray(detail.body?.sub_actions ?? detail.body?.mitigation?.sub_actions ?? []);
      for (const s of subs.filter((x) => done.includes(x.title))) {
        const c = await api('POST', `/mitigations/${mid}/sub-actions/${s.id}/complete`, {});
        if (!c.ok) skipped('sous-actions', s.title, c);
      }
    }
  }
}

/* ---- compliance ---- */

async function seedFrameworks() {
  const m = 'référentiels';
  const fws = await listAll('/compliance/frameworks');
  for (const f of dataset.frameworks) {
    let fw = fws.find((x) => x.name === f.name);
    if (fw) existing(m);
    else {
      const res = await api('POST', '/compliance/frameworks', { name: f.name, version: f.version, description: f.description });
      if (!res.ok) {
        skipped(m, f.name, res);
        continue;
      }
      fw = { id: idOf(res.body) };
      created(m);
    }
    ids.frameworks[f.name] = fw.id;
    let controls = await listAll(`/compliance/frameworks/${fw.id}/controls`);
    if (!controls.length) {
      const imp = await api('POST', `/compliance/frameworks/${fw.id}/import-catalog`, { catalog_key: f.catalog_key });
      if (!imp.ok) skipped('catalogues de contrôles', `${f.name} (${f.catalog_key})`, imp);
      controls = await listAll(`/compliance/frameworks/${fw.id}/controls`);
    }
    ids.controls[f.name] = controls;
  }
}

/** Picks a status per control from the mix, by position, so it is stable run to run. */
function plannedStatus(i) {
  const mix = Object.entries(dataset.control_status_mix);
  const at = (i * 0.618) % 1; // golden-ratio walk: spread out, deterministic
  let acc = 0;
  for (const [s, share] of mix) {
    acc += share;
    if (at < acc) return s;
  }
  return mix[mix.length - 1][0];
}

/**
 * Sets a spread of control statuses so the coverage bars are not all grey.
 *
 * A control cannot be declared implemented without valid evidence
 * (update_control.go), so each framework first gets one statement of
 * applicability, linked to the controls about to be marked implemented. That is
 * also how an auditor would expect it to be substantiated.
 */
async function seedControlStatuses() {
  const m = 'statuts des contrôles';
  const library = await listAll('/evidence');
  for (const f of dataset.frameworks) {
    const controls = ids.controls[f.name] ?? [];
    if (!controls.length) continue;
    const implemented = controls.filter((_, i) => plannedStatus(i) === 'implemented').map((c) => c.id);
    const soaTitle = `Déclaration d'applicabilité — ${f.name}`;
    // Counted the same way on every run, so a second run prints the first run's
    // totals under "existants" (#1337, criterion 2).
    if (implemented.length && library.some((e) => e.title === soaTitle)) existing('preuves');
    if (controls.some((c) => c.status && c.status !== 'not_implemented')) {
      tally(m).existing += controls.filter((c) => c.status && c.status !== 'not_implemented').length;
      continue;
    }
    if (implemented.length && !library.some((e) => e.title === soaTitle)) {
      const res = await uploadEvidence({
        title: soaTitle,
        type: 'attestation',
        description: `Contrôles déclarés en place pour ${f.name}, signée par le RSSI.`,
        valid_days: 365,
        controlIds: implemented,
      });
      if (!res.ok) skipped('preuves', soaTitle, res);
      else created('preuves');
    }
    let set = 0;
    for (let i = 0; i < controls.length; i++) {
      const status = plannedStatus(i);
      if (status === 'not_implemented') continue;
      const res = await api('PATCH', `/compliance/controls/${controls[i].id}`, { status });
      if (!res.ok) {
        skipped(m, `${controls[i].reference_code ?? controls[i].name} → ${status}`, res);
        continue;
      }
      set++;
    }
    tally(m).created += set;
    ids.controls[f.name] = await listAll(`/compliance/frameworks/${ids.frameworks[f.name]}/controls`);
  }
}

async function seedControlMappings() {
  const m = 'correspondances entre référentiels';
  const iso = ids.controls['ISO 27001'] ?? [];
  const cobac = ids.controls.COBAC ?? [];
  const pci = ids.controls['PCI DSS'] ?? [];
  const pairs = [];
  for (let i = 0; i < 4 && i < iso.length && i < cobac.length; i++) pairs.push([iso[i], cobac[i], 'partial']);
  for (let i = 0; i < 3 && i + 4 < iso.length && i < pci.length; i++) pairs.push([iso[i + 4], pci[i], 'related']);
  for (const [s, t, relation] of pairs) {
    const res = await api('POST', '/compliance/control-mappings', {
      source_control_id: s.id,
      target_control_id: t.id,
      relation,
      rationale: `Les deux contrôles portent sur la même exigence (${s.name ?? s.reference_code}).`,
      note: 'Jeu de données de développement',
    });
    if (res.ok) created(m);
    else if (res.status === 409) existing(m);
    else skipped(m, `${s.reference_code} ↔ ${t.reference_code}`, res);
  }
  // Two risks mapped onto a real control, so the "Référentiel" badge has a target.
  const rm = 'risques ↔ contrôles';
  const links = [
    ['MFA non imposée sur le fournisseur d\'identité', 'ISO 27001', 5],
    ['Revue des accès expirée pour trois applications', 'COBAC', 2],
    ['Script analytique tiers sur la page de paiement', 'PCI DSS', 3],
  ];
  for (const [title, fw, idx] of links) {
    const rid = ids.risks[title];
    const ctl = ids.controls[fw]?.[idx];
    if (!rid || !ctl) continue;
    const res = await api('POST', `/risks/${rid}/control-mappings`, { framework_id: ids.frameworks[fw], control_id: ctl.id });
    if (res.ok) created(rm);
    else if (res.status === 409) existing(rm);
    else skipped(rm, title, res);
  }
}

/** Multipart upload of a small text file as evidence, linked to the given controls. */
function uploadEvidence({ title, type, description, valid_days, controlIds }) {
  const form = new FormData();
  form.set('title', title);
  form.set('type', type);
  form.set('description', description ?? '');
  form.set('source', 'manual');
  form.set('collected_at', dateIn(-15));
  form.set('valid_until', dateIn(valid_days ?? 180));
  form.set('control_ids', controlIds.join(','));
  const text = `${title}\n\n${description ?? ''}\n\nFichier de développement généré par scripts/seed-dev.mjs.\n`;
  form.set('file', new Blob([text], { type: 'text/plain' }), `${title.replace(/[^\p{L}\p{N}]+/gu, '-')}.txt`);
  return api('POST', '/evidence', form);
}

async function seedEvidence() {
  const m = 'preuves';
  const all = await listAll('/evidence');
  for (const e of dataset.evidence) {
    if (all.some((x) => x.title === e.title)) {
      existing(m);
      continue;
    }
    // Skip the first controls: the statement of applicability covers those.
    const pool = ids.controls[e.framework] ?? [];
    const controlIds = pool.filter((_, i) => plannedStatus(i) !== 'implemented').slice(0, e.controls ?? 1).map((c) => c.id);
    const res = await uploadEvidence({ ...e, controlIds });
    if (!res.ok) {
      skipped(m, e.title, res);
      continue;
    }
    created(m);
    if (e.review) {
      const id = idOf(res.body);
      const r = await api('POST', `/evidence/${id}/review`, {
        decision: e.review,
        comment: e.review === 'reject' ? 'La capture ne montre pas le royaume de production.' : 'Conforme.',
      });
      if (!r.ok) skipped('revue des preuves', e.title, r);
    }
  }
}

async function seedAudits() {
  const m = 'audits';
  const all = await listAll('/compliance/audits');
  for (const a of dataset.audits) {
    let audit = all.find((x) => x.title === a.title);
    if (audit) {
      ids.audits[a.title] = audit.id;
      existing(m);
      continue;
    }
    const auditor = dataset.personas.find((p) => p.key === a.auditor)?.full_name ?? a.auditor;
    const res = await api('POST', '/compliance/audits', {
      title: a.title,
      type: a.type,
      framework_id: ids.frameworks[a.framework],
      auditor,
      scope: a.scope,
      scheduled_start: isoIn(a.start_in_days),
      scheduled_end: isoIn(a.end_in_days),
    });
    if (!res.ok) {
      skipped(m, a.title, res);
      continue;
    }
    const id = idOf(res.body);
    ids.audits[a.title] = id;
    created(m);
    if (a.status && a.status !== 'planned') {
      const steps = a.status === 'completed' ? ['in_progress', 'completed'] : [a.status];
      for (const status of steps) {
        const patch = { status };
        if (status === 'completed') Object.assign(patch, { compliance_score: a.compliance_score, summary: a.summary });
        const p = await api('PATCH', `/compliance/audits/${id}`, patch);
        if (!p.ok) {
          skipped('statut des audits', `${a.title} → ${status}`, p);
          break;
        }
      }
    }
    if (a.generate_remediations) {
      const g = await api('POST', `/compliance/audits/${id}/generate-remediations`, {});
      if (!g.ok) skipped('remédiations générées', a.title, g);
    }
  }
}

async function seedRemediations() {
  const m = 'plans de remédiation';
  const all = await listAll('/compliance/remediations');
  for (const r of dataset.remediations) {
    if (all.some((x) => x.title === r.title)) {
      existing(m);
      continue;
    }
    const res = await api('POST', '/compliance/remediations', {
      title: r.title,
      description: r.description,
      audit_id: r.audit ? ids.audits[r.audit] : undefined,
      priority: r.priority,
      assigned_to: userId(r.assignee),
      due_date: isoIn(r.due_in_days),
    });
    if (!res.ok) {
      skipped(m, r.title, res);
      continue;
    }
    created(m);
    if (r.status) {
      const p = await api('PATCH', `/compliance/remediations/${idOf(res.body)}`, { status: r.status });
      if (!p.ok) skipped('statut des remédiations', r.title, p);
    }
  }
}

/* ---- security ---- */

async function seedVulnerabilities() {
  const m = 'vulnérabilités';
  const before = await listAll('/vulnerabilities');
  const seen = new Set(before.flatMap((v) => [v.cve_id, v.cve, v.title].filter(Boolean)));
  const todo = dataset.vulnerabilities.filter((v) => !seen.has(v.cve ?? v.title) && !seen.has(v.title));
  tally(m).existing += dataset.vulnerabilities.length - todo.length;
  if (!todo.length) return;
  const res = await api('POST', '/vulnerabilities/ingest', {
    source: 'manual',
    findings: todo.map((v) => ({
      title: v.title,
      description: v.description ?? v.title,
      cve: v.cve,
      cvss: v.cvss,
      severity: v.severity,
      kev: v.kev ?? false,
      exploit_available: v.exploit_available ?? false,
      host: v.host,
      hostname: v.host,
    })),
  });
  if (res.ok) tally(m).created += todo.length;
  else skipped(m, `${todo.length} findings`, res);
}

async function seedIncidents() {
  const m = 'incidents';
  const all = await listAll('/incidents');
  for (const i of dataset.incidents) {
    let inc = all.find((x) => x.title === i.title);
    if (inc) {
      existing(m);
      continue;
    }
    const res = await api('POST', '/incidents', {
      title: i.title,
      description: i.description,
      incident_type: i.incident_type,
      severity: i.severity,
      source: 'internal',
      asset_ids: (i.assets ?? []).map((n) => ids.assets[n]).filter(Boolean),
      risk_ids: (i.risks ?? []).map((t) => ids.risks[t]).filter(Boolean),
    });
    if (!res.ok) {
      skipped(m, i.title, res);
      continue;
    }
    created(m);
    const id = idOf(res.body);
    for (const a of i.actions ?? []) {
      const r = await api('POST', `/incidents/${id}/actions`, {
        title: a.title,
        assigned_to: userId(a.assignee) ?? a.assignee,
        due_date: isoIn(a.due_in_days ?? 1),
      });
      if (!r.ok) skipped('actions d\'incident', a.title, r);
    }
    if (i.post_mortem) {
      const { publish, ...pm } = i.post_mortem;
      pm.timeline = (pm.timeline ?? []).map((t) => ({ ...t, at: isoIn(Number(t.at.replace('__', '').replace('d', ''))) }));
      pm.corrective_actions = (pm.corrective_actions ?? []).map(({ owner, due_in_days, ...c }) => ({
        ...c,
        id: crypto.randomUUID(),
        owner_id: userId(owner),
        due_date: isoIn(due_in_days ?? 30),
      }));
      const r = await api('PUT', `/incidents/${id}/post-mortem`, pm);
      if (!r.ok) skipped('post-mortems', i.title, r);
      else if (publish) {
        const p = await api('POST', `/incidents/${id}/post-mortem/publish`, {});
        if (!p.ok) skipped('post-mortems publiés', i.title, p);
      }
    }
    if (i.status && i.status !== 'open') {
      const u = await api('PUT', `/incidents/${id}`, {
        status: i.status,
        resolution: ['resolved', 'closed'].includes(i.status) ? 'Traité, voir le post-mortem.' : undefined,
      });
      if (!u.ok) skipped('statut des incidents', `${i.title} → ${i.status}`, u);
    }
  }
}

/* ---- third parties ---- */

async function seedQuestionnaires() {
  const m = 'questionnaires fournisseurs';
  const all = await listAll('/vendor-questionnaire-templates');
  for (const t of dataset.questionnaire_templates) {
    const found = all.find((x) => x.name === t.name);
    if (found) {
      ids.templates[t.name] = found.id;
      existing(m);
      continue;
    }
    const res = await api('POST', '/vendor-questionnaire-templates', t);
    if (res.ok) {
      ids.templates[t.name] = idOf(res.body);
      created(m);
    } else skipped(m, t.name, res);
  }
  const am = 'évaluations fournisseurs';
  for (const a of dataset.vendor_assessments) {
    const vendorId = ids.assets[a.vendor];
    const templateId = ids.templates[a.template];
    if (!vendorId || !templateId) {
      skipped(am, `${a.vendor}: vendor or template missing`);
      continue;
    }
    const list = await listAll(`/vendors/${vendorId}/assessments`);
    if (list.some((x) => (x.template_id ?? x.template?.id) === templateId && x.status !== 'revoked')) {
      existing(am);
      continue;
    }
    const res = await api('POST', `/vendors/${vendorId}/assessments`, {
      template_id: templateId,
      due_at: isoIn(a.due_in_days),
      contact_email: a.contact_email,
      contact_language: a.contact_language,
    });
    res.ok ? created(am) : skipped(am, a.vendor, res);
  }
}

/* ---- reporting ---- */

async function seedReports() {
  const m = 'rapports au conseil';
  const boards = await listAll('/reports/board');
  for (const b of dataset.board_reports) {
    if (boards.some((x) => x.period_label === b.period_label)) {
      existing(m);
      continue;
    }
    const res = await api('POST', '/reports/board', { period_label: b.period_label, locale: b.locale });
    if (!res.ok) {
      skipped(m, b.period_label, res);
      continue;
    }
    created(m);
    if (b.approve) {
      const a = await api('POST', `/reports/board/${idOf(res.body)}/approve`, {});
      if (!a.ok) skipped('approbation des rapports', b.period_label, a);
    }
  }
  const jm = 'rapports générés';
  const reports = await listAll('/reports');
  for (const j of dataset.report_jobs) {
    if (reports.some((x) => x.type === j.type)) {
      existing(jm);
      continue;
    }
    const res = await api('POST', '/reports', {
      type: j.type,
      format: j.format,
      locale: j.locale,
      from: dateIn(-90),
      to: dateIn(0),
      framework_id: j.framework ? ids.frameworks[j.framework] : undefined,
    });
    res.ok ? created(jm) : skipped(jm, j.type, res);
  }
}

/* ---- governance and automation ---- */

async function seedGovernance() {
  const g = dataset.governance;
  const wm = 'circuits d\'approbation';
  const workflows = await listAll('/governance/workflows');
  for (const w of g.workflows) {
    if (workflows.some((x) => x.name === w.name)) {
      existing(wm);
      continue;
    }
    const res = await api('POST', '/governance/workflows', w);
    res.ok ? created(wm) : skipped(wm, w.name, res);
  }
  const am = 'demandes d\'approbation';
  const approvals = await listAll('/governance/approvals');
  for (const a of g.approvals) {
    if (approvals.some((x) => x.title === a.title)) {
      existing(am);
      continue;
    }
    const entityId = ids.risks[a.entity];
    if (!entityId) {
      skipped(am, `${a.title}: risk missing`);
      continue;
    }
    const res = await api('POST', '/governance/approvals', {
      entity_type: a.entity_type,
      entity_id: entityId,
      action: a.action,
      title: a.title,
      description: a.description,
    });
    res.ok ? created(am) : skipped(am, a.title, res);
  }
  const dm = 'délégations';
  const delegations = await listAll('/governance/delegations');
  for (const d of g.delegations) {
    const from = userId(d.delegator);
    const to = userId(d.delegate);
    if (delegations.some((x) => x.delegator_id === from && x.delegate_id === to && !x.revoked_at)) {
      existing(dm);
      continue;
    }
    const res = await api('POST', '/governance/delegations', {
      delegator_id: from,
      delegate_id: to,
      reason: d.reason,
      permissions: d.permissions,
      starts_at: isoIn(d.starts_in_days),
      ends_at: isoIn(d.ends_in_days),
    });
    res.ok ? created(dm) : skipped(dm, d.reason, res);
  }
}

async function seedAutomation() {
  const m = 'règles d\'automatisation';
  const rules = await listAll('/automation/rules');
  const templates = await listAll('/automation/templates');
  const wanted = templates.map((x) => x.template ?? x).slice(0, dataset.automation.templates_to_adopt);
  for (const t of wanted) {
    const name = t.name ?? t.name_fr ?? t.title ?? t.key;
    if (rules.some((r) => r.template_key === t.key || r.name === name)) {
      existing(m);
      continue;
    }
    const res = await api('POST', `/automation/templates/${t.key}/adopt`, {});
    res.ok ? created(m) : skipped(m, t.key, res);
  }
  if (!templates.length) skipped(m, 'no automation template listed');
}

async function seedSavedViews() {
  const m = 'vues enregistrées';
  const views = await listAll('/saved-views?table_id=risks');
  for (const v of dataset.saved_views) {
    if (views.some((x) => x.name === v.name)) {
      existing(m);
      continue;
    }
    const res = await api('POST', '/saved-views', v);
    res.ok ? created(m) : skipped(m, v.name, res);
  }
}

/**
 * A scan configuration so the infrastructure screen has a target. It is never
 * triggered: the seed does not scan anything.
 */
async function seedScanConfigs() {
  const m = 'configurations de scan';
  const configs = await listAll('/scanner/configs');
  for (const c of dataset.scan_configs ?? []) {
    if (configs.some((x) => x.name === c.name)) {
      existing(m);
      continue;
    }
    const res = await api('POST', '/scanner/configs', c);
    res.ok ? created(m) : skipped(m, c.name, res);
  }
}

/* ---- second tenant ---- */

async function seedSecondTenant() {
  const m = 'organisation n°2';
  const st = dataset.second_tenant;
  let session = await signIn(st.owner.email, st.owner.password);
  if (session.error) {
    const reg = await api('POST', '/auth/register', st.owner, { token: '' });
    if (!reg.ok) {
      skipped(m, st.owner.email, reg);
      return;
    }
    created(m);
    session = await signIn(st.owner.email, st.owner.password);
    if (session.error) {
      skipped(m, `${st.owner.email} cannot sign in`, session.error);
      return;
    }
  } else existing(m);
  await completeOnboarding(st.owner);

  const adminToken = TOKEN;
  TOKEN = session.token;
  try {
    const saved = ids.assets;
    ids.assets = {};
    await seedAssets(st.assets, 'organisation n°2 · actifs');
    const risks = await listAll('/risks');
    for (const r of st.risks) {
      if (risks.some((x) => x.title === r.title)) {
        existing('organisation n°2 · risques');
        continue;
      }
      const res = await api('POST', '/risks', {
        title: r.title,
        description: r.description,
        probability: r.probability,
        impact: r.impact,
        tags: r.tags,
        asset_ids: r.assets.map((n) => ids.assets[n]).filter(Boolean),
      });
      res.ok ? created('organisation n°2 · risques') : skipped('organisation n°2 · risques', r.title, res);
    }
    ids.assets = saved;
  } finally {
    TOKEN = adminToken;
  }
}

/* ============================================================ isolation */

/**
 * Reads back from the first tenant: none of the second tenant's titles may
 * show. Returns null when the check proves nothing: the read failed, or the
 * second tenant's risks do not exist, so an empty answer would be meaningless.
 */
async function checkIsolation() {
  const titles = new Set(dataset.second_tenant.risks.map((r) => r.title));
  const st = tally('organisation n°2 · risques');
  if (st.created + st.existing !== titles.size) return null;
  const res = await api('GET', '/risks?page=1&page_size=1');
  if (!res.ok) return null;
  const leaked = (await listAll('/risks')).filter((r) => titles.has(r.title));
  return leaked.length;
}

/* ================================================================== main */

async function main() {
  assertLocalTarget();
  if (!ADMIN_PASSWORD) {
    fail(
      'SEED_ADMIN_PASSWORD is not set. With the compose stack, `make seed-dev` reads it from the backend ' +
        'container (secrets/initial_admin_password). Otherwise export the INITIAL_ADMIN_PASSWORD the backend started with.',
    );
  }
  const health = await api('GET', '/health', undefined, { token: '' }).catch(() => null);
  if (!health?.ok) fail(`no API at ${API_URL} — is the stack up?`);

  const session = await signIn(ADMIN_EMAIL, ADMIN_PASSWORD, process.env.SEED_ADMIN_TOTP_SECRET);
  if (session.error) {
    const e = session.error;
    fail(
      `cannot sign in as ${ADMIN_EMAIL}: ${e.status} ${JSON.stringify(e.body).slice(0, 200)}. ` +
        'If MFA is enrolled on this account, set SEED_ADMIN_TOTP_SECRET.',
    );
  }
  TOKEN = session.token;
  console.log(`[seed-dev] ${API_URL} as ${ADMIN_EMAIL}`);

  const steps = [
    seedOrganization, seedPersonas, seedTeams, seedCustomFields,
    () => seedAssets(dataset.assets), seedDependencies, seedVendorLinks,
    // Governance before the lifecycle: accepting a residual risk needs its workflow.
    seedRisks, seedMitigations, seedGovernance, seedRiskStates,
    seedFrameworks, seedEvidence, seedControlStatuses, seedControlMappings, seedAudits, seedRemediations,
    seedVulnerabilities, seedIncidents, seedQuestionnaires, seedReports,
    seedAutomation, seedSavedViews, seedScanConfigs, seedSecondTenant,
  ];
  for (const step of steps) {
    try {
      await step();
    } catch (err) {
      skipped('erreurs', `${step.name || 'step'}: ${err.message}`);
    }
  }
  const leaks = await checkIsolation();

  printSummary(leaks);
  fs.writeFileSync(
    STATE_FILE,
    JSON.stringify({ generatedAt: new Date().toISOString(), api: API_URL, ids }, null, 2),
  );
  if (leaks !== 0) process.exit(2);
}

function printSummary(leaks) {
  const rows = [...summary.entries()];
  const w = Math.max(...rows.map(([k]) => k.length), 10);
  console.log('');
  console.log(`${'module'.padEnd(w)}  créés  existants  ignorés`);
  for (const [k, v] of rows) {
    console.log(`${k.padEnd(w)}  ${String(v.created).padStart(5)}  ${String(v.existing).padStart(9)}  ${String(v.skipped.length).padStart(7)}`);
  }
  const notes = rows.flatMap(([k, v]) => v.skipped.map((s) => `  · ${k} — ${s}`));
  if (notes.length) {
    console.log('\nNon créés (réponse de l\'API) :');
    for (const n of notes) console.log(n);
  }
  const verdict =
    leaks === null
      ? 'NON VÉRIFIÉE, les risques de l\'organisation n°2 n\'existent pas ou le registre n\'a pas pu être lu'
      : leaks
        ? `ÉCHEC, ${leaks} risque(s) de l'organisation n°2 visibles depuis Kalao`
        : 'aucun risque de l\'organisation n°2 visible depuis Kalao';
  console.log(`\nIsolation : ${verdict}`);
  console.log('\nComptes (mots de passe de développement, voir dev/fixtures/dev-dataset.json) :');
  console.log(`  ${ADMIN_EMAIL.padEnd(36)} propriétaire (root)  mot de passe du seed admin`);
  for (const p of dataset.personas) {
    // POST /users drops an admin's business role: the wildcard grants it all.
    const role = p.role === 'admin' ? 'admin (tous les droits)' : `${p.role} · ${p.business_role}`;
    console.log(`  ${p.email.padEnd(36)} ${role.padEnd(30)} ${p.password}`);
  }
  const o = dataset.second_tenant.owner;
  console.log(`  ${o.email.padEnd(36)} ${'root · Sahel Assurances'.padEnd(30)} ${o.password}`);
  console.log(
    `\nMFA : délai de grâce de ${dataset.organization.mfa_grace_days} jours sur Kalao, les comptes privilégiés se connectent par mot de passe jusque-là.`,
  );
}

main().catch((err) => fail(`FAILED: ${err.stack || err.message}`));
