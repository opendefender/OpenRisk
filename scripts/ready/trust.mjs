// Who counts as one of ours (#1320, #1326). Pure functions, no I/O.
//
// People of the organisation are trusted by author association. The delivery bot is trusted
// by login, because GitHub gives its comments and issues the association CONTRIBUTOR, not
// MEMBER, so an association check alone would ignore its refinement markers and the child
// issues it opens (found in the #1326 design gate).
//
// A login alone is not enough: anyone can register a user named `opendefender-delivery-bot`
// (GraphQL shows a bot without the `[bot]` suffix). The account must also be a Bot: GraphQL
// `__typename`, REST `type`, or the `[bot]` suffix, which a human login cannot contain.

export const TRUSTED = ['OWNER', 'MEMBER', 'COLLABORATOR'];
export const BOT = 'opendefender-delivery-bot';

export const normLogin = (login) => String(login || '').replace(/\[bot\]$/i, '').toLowerCase();

// The account behind a GraphQL node (`author`) or a REST object (`user`).
const actor = (x) => x?.author ?? x?.user ?? null;
const association = (x) => x?.authorAssociation ?? x?.author_association ?? null;

export function isBot(x) {
  const a = actor(x);
  if (!a || normLogin(a.login) !== BOT) return false;
  return a.__typename === 'Bot' || a.type === 'Bot' || /\[bot\]$/i.test(a.login);
}

export const isTrusted = (x) => TRUSTED.includes(association(x)) || isBot(x);
