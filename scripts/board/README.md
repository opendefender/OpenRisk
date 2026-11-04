# Delivery board scripts

The board is https://github.com/orgs/opendefender/projects/3 (#1247, #1248, ADR 0009 D6).
Its Status, Priority and Tier are **derived**: change the label or the code, never the field.

| Script | What it does |
|---|---|
| `setup.mjs` | Creates or repairs the project (statuses, fields, views, no built-in workflows) and writes `.github/board.json`. Run once, or after changing the statuses. |
| `sync.mjs` | Recomputes every item from the repository and writes only what differs. `--dry-run` prints the changes and the counts. |
| `derive.mjs` | The rules, as pure functions. Tested in `derive.test.mjs`. |

## Status rules (first match wins)

Closed issue: not planned or duplicate → archived · closing commit in a tag matching
`releaseTags` → **Released** · on the `staging` branch → **On staging** · merged → **On dev** ·
closed by hand with no merged change → archived.

Open issue: an owner label (`ownerLabels`) → **Needs owner** · `status:blocked` → **Blocked** ·
closing PR merged into the `dev` branch → **On dev** (and further) · an open closing PR or
`status:in-review` → **In review** · a `<type>/<n>-*` branch or `status:in-progress` →
**In progress** · `status:ready` → **Ready** · `status:needs-refinement` → **Refinement** ·
otherwise **Inbox**.

Branch names live in `.github/board.json`: today `dev` is `master` and there is no staging
branch. #1244 changes them to `develop` and `staging`.

## Running it

From a clone, logged in with `gh` (the token needs the `project` scope):

```sh
node scripts/board/sync.mjs --dry-run
node scripts/board/sync.mjs
node --test scripts/board/*.test.mjs
```

In CI, `.github/workflows/board.yml` runs it on issue, PR, branch, push and tag events and
every 30 minutes. It needs the `BOARD_TOKEN` secret, a fine-grained token with resource owner
`opendefender`, organisation permission **Projects: read and write**, and on this repository
**Contents, Issues, Pull requests, Metadata: read**. Without the secret the job is skipped with
a notice.
