// The daily brief (#1320, D-101): which three issues the owner should start next, and why.
// Pure functions, no I/O; scripts/brief/post.mjs reads GitHub and posts the result.
//
// Nothing here starts work. The owner starts each issue with /work; the brief only ranks.

import { sprintEnd } from '../board/derive.mjs';

const TRUSTED = ['OWNER', 'MEMBER', 'COLLABORATOR'];

// An issue counts as refined when a trusted account left a `<!-- refined: <date> … -->` marker
// on it (D-094). Anyone can comment on a public issue, so a stranger's marker counts for
// nothing (#1320).
export const REFINED_MARKER = /<!--\s*refined:\s*(\d{4}-\d{2}-\d{2})/;
export const trustedComments = (issue) => (issue.comments || []).filter((c) => TRUSTED.includes(c?.authorAssociation));
export const isRefined = (issue) => trustedComments(issue).some((c) => REFINED_MARKER.test(c.body || ''));

const priorityRank = (labels) => {
  const m = labels.map((l) => /^priority:P([0-3])/.exec(l)?.[1]).filter(Boolean).sort();
  return m.length ? Number(m[0]) : 9;
};
const tierRank = (labels) => {
  const m = labels.map((l) => /^tier:([0-5])-/.exec(l)?.[1]).filter(Boolean).sort();
  return m.length ? Number(m[0]) : 9;
};
// P0 first, then tier 0 first, then the oldest.
export const byUrgency = (a, b) =>
  priorityRank(a.labels) - priorityRank(b.labels) || tierRank(a.labels) - tierRank(b.labels) || a.number - b.number;
const IN_FLIGHT = ['status:in-progress', 'status:in-review', 'status:blocked'];
const SIZE = /<!--\s*refined:\s*(\d{4}-\d{2}-\d{2})[^>]*?size=([SML])/;

// issue = { number, title, labels, authorAssociation, comments: [{ body, authorAssociation }], hasOpenPr,
//           milestone: { title, dueOn } | null, sprint: { startDate, duration } | null }
export function launchable(i) {
  return TRUSTED.includes(i.authorAssociation) &&
    i.labels.includes('status:ready') &&
    !i.labels.some((l) => IN_FLIGHT.includes(l) || l.startsWith('tier:5')) &&
    !i.hasOpenPr &&
    isRefined(i);
}

export const inSprint = (sprint, today) => Boolean(sprint) && sprint.startDate <= today && sprintEnd(sprint) > today;

const due = (i) => i.milestone?.dueOn?.slice(0, 10) ?? '9999-12-31';

// Current sprint first, then priority, then tier, then the nearest milestone due date, then age.
export function rank(issues, today, n = 3) {
  return issues
    .filter(launchable)
    .sort((a, b) =>
      Number(inSprint(b.sprint, today)) - Number(inSprint(a.sprint, today)) ||
      byUrgencyNoAge(a, b) || due(a).localeCompare(due(b)) || a.number - b.number)
    .slice(0, n)
    .map((i) => ({ number: i.number, title: i.title, reasons: reasons(i, today) }));
}

// byUrgency ends on age; the brief puts the milestone due date before age.
const byUrgencyNoAge = (a, b) => byUrgency({ ...a, number: 0 }, { ...b, number: 0 });

export function reasons(i, today) {
  const out = [];
  if (inSprint(i.sprint, today)) out.push('dans le sprint en cours');
  // The taxonomy label (priority:P1-high) over a legacy bare one (priority:P1).
  const p = i.labels.filter((l) => l.startsWith('priority:')).sort((x, y) => y.length - x.length)[0];
  if (p) out.push(`priorité ${p.slice(9)}`);
  const t = i.labels.find((l) => l.startsWith('tier:'));
  if (t) out.push(`tier ${t.slice(5)}`);
  if (i.milestone) out.push(i.milestone.dueOn ? `milestone « ${i.milestone.title} », échéance le ${i.milestone.dueOn.slice(0, 10)}` : `milestone « ${i.milestone.title} »`);
  const m = [...trustedComments(i)].reverse().map((c) => SIZE.exec(c.body || '')).find(Boolean);
  if (m) out.push(`affiné le ${m[1]}, taille ${m[2]}`);
  return out;
}

// A title from GitHub, made inert in the bot's comment: no @-mention, no link, no markup (#1320).
// Anyone can write an issue or PR title, and the bot posts this every day.
export const safe = (title) => String(title).replace(/[\\`*_[\]()<>#|~!]/g, '\\$&').replace(/@/g, '@\u200b')
  .replace(/:\/\//g, ':\u200b//').replace(/\bwww\./gi, 'www\u200b.');

// The comment the bot posts. `waiting` lists what only the owner can do.
export function render({ today, owner, top, inFlight, waiting }) {
  const lines = [`### Brief du jour — ${today}`, ''];
  if (owner) lines.push(`@${owner}`, '');
  if (top.length) {
    lines.push(`**À lancer, dans l'ordre** (${top.length})`, '');
    top.forEach((t, k) => {
      lines.push(`${k + 1}. #${t.number} — ${safe(t.title)}`);
      lines.push(`   - Pourquoi : ${t.reasons.join(' · ') || 'seul ticket prêt et affiné'}`);
      lines.push(`   - Lancer : \`/work ${t.number}\``);
    });
  } else {
    lines.push("**À lancer** — aucun ticket prêt et affiné. L'affinage automatique revient avec #1326 ; d'ici là, un ticket s'affine dans une session.");
  }
  lines.push('');
  lines.push(`**En cours** — ${inFlight.length ? inFlight.map((n) => `#${n}`).join(', ') : 'rien'}`);
  lines.push('');
  if (waiting.length) {
    lines.push('**Attend ta décision**', '');
    for (const w of waiting) lines.push(`- ${w}`);
  } else {
    lines.push('**Attend ta décision** — rien.');
  }
  lines.push('', '<sub>Calculé par scripts/brief/ sans appel de modèle (#1320). Rien ne démarre sans toi.</sub>');
  return lines.join('\n');
}

// Open entries of docs/DECISIONS.md: the `### D-…` headings between `## Open` and `## Resolved`.
export function openDecisions(markdown) {
  const open = /^## Open\s*$([\s\S]*?)^## Resolved/m.exec(markdown)?.[1] ?? '';
  return [...open.matchAll(/^### (D-\d+ — .+)$/gm)].map((m) => m[1].replace(/\s+·\s+raised.*$/, ''));
}
