---
name: sprint
description: Run several OpenRisk issues in parallel with an agent team — spawns teammates that own disjoint file sets and coordinate through the shared task list. Use only for issues that touch different layers. Expensive; use deliberately.
argument-hint: <issue numbers, comma separated>
---

# Sprint on issues: $ARGUMENTS

You are the team lead. Agent teams cost several times a single session — only
run this when the issues genuinely parallelize.

## Refuse to spawn if

- Two issues touch the same files. Run them sequentially with `/work` instead.
- Any issue is not `status:ready`.
- There are more than four issues. Do the first four.

Say which check failed and stop. Do not spawn anyway.

## Before spawning

Read each issue, produce a table: issue · agent · **files owned** · dependencies.
Two teammates must never own the same file. Show me the table and wait for my
go-ahead.

## Spawn

Name each teammate after its issue so I can address it:
- teammate `be-<issue>` using the `backend-go` agent type
- teammate `fe-<issue>` using the `frontend-react` agent type
- teammate `qa` using the `qa-automation` agent type
- teammate `sec` using the `devsecops` agent type

Each spawn prompt must contain the full issue context — teammates inherit
`CLAUDE.md` and skills but **not this conversation**. Paste what they need.

Require plan approval. Approve only plans that name the files they will touch
and include a cross-tenant test.

## During

`qa` starts once the first implementer reports a green build. `sec` runs last
and may send work back. Wait for every teammate to report before synthesizing —
do not implement tasks yourself while they work.

Each teammate posts its checkpoint comment on its own issue before going idle.

## Close

Synthesize: what shipped · what is blocked · what regressed · next actions.
Then shut down each teammate by name.
