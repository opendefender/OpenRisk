# Versioning & Release Policy

OpenRisk follows [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).
This document is the contract for how a version is chosen, propagated, tagged and
released — and how the (messy) historical tags are handled.

## 1. Single source of truth

The root [`VERSION`](../VERSION) file holds the **bare** SemVer string — no `v`
prefix, no metadata — e.g.:

```
1.1.0-rc.1
```

Everything else is **derived** from it, never hand-edited independently:

| Target | How it gets the version | Verified by |
| --- | --- | --- |
| Go binary → `GET /api/v1/health` | `make build` injects `-ldflags "-X main.Version=$(cat VERSION) -X main.Commit=$(git rev-parse --short HEAD)"` | run the binary, curl `/api/v1/health` |
| Helm chart (`version` + `appVersion`) | `make sync-version` writes `VERSION` into `helm/openrisk/Chart.yaml` | `make check-version` |
| Frontend (`package.json` `version`) | `make sync-version` writes `VERSION` into `frontend/package.json` | `make check-version` |

`make check-version` fails (exit 1) if any of the three drift from `VERSION`. It is
run by the release workflow's guard job, so a release can never ship artifacts whose
reported version disagrees with the tag.

`GET /api/v1/health` returns:

```json
{ "status": "UP", "version": "1.1.0-rc.1", "commit": "e52375d3", "db": "CONNECTED" }
```

A non-release build (`go run`, `go build` without ldflags) reports
`version: "dev", commit: "none"` — deliberately obvious.

## 2. Tag convention

- Every release is a **git tag** of the form **`vMAJOR.MINOR.PATCH`**, with an
  optional pre-release suffix **`-rc.N`** — e.g. `v1.1.0`, `v1.1.0-rc.2`.
- The **`v` prefix is mandatory** on the tag. The `VERSION` file and the
  `CHANGELOG.md` headings hold the bare SemVer (`1.1.0-rc.1`); the `v` is a git-tag
  convention only. The release guard compares `v$(cat VERSION)` to the pushed tag.
- Tags are **annotated** (`git tag -a`), never lightweight.
- MAJOR / MINOR / PATCH follow SemVer: breaking / feature / fix. `-rc.N` marks a
  stabilising release candidate and is published as a GitHub **pre-release**.

## 3. Release cycle

```
1. Decide the next version (SemVer).           e.g. 1.1.0-rc.2
2. echo "1.1.0-rc.2" > VERSION
3. make sync-version                            # propagate to chart + frontend
4. Update CHANGELOG.md: move [Unreleased] items under a new
   ## [1.1.0-rc.2] - YYYY-MM-DD section + add its compare link.
5. Commit:  chore(release): 1.1.0-rc.2
6. Tag:     git tag -a v1.1.0-rc.2 -m "OpenRisk 1.1.0-rc.2"
7. Push:    git push origin v1.1.0-rc.2
8. CI (release.yml) runs the guard, builds/tests, and publishes the GitHub
   release with notes taken from the matching CHANGELOG.md section.
```

If step 6's tag does not equal `v$(cat VERSION)`, the guard fails the workflow —
by design.

## 4. Historical tags

The repository carried an inconsistent set of legacy tags **that predate the
current product**. They are **kept as-is** — deleting or moving a published tag
breaks every existing clone and every image already built from it, so we never do
that. Instead they are **requalified**:

- All GitHub releases `1.0.0`–`1.0.8` are marked **pre-release** with the header
  line: _"Historical tag — predates the current product. See CHANGELOG.md."_
- To homogenise the prefix, an **annotated alias tag `v1.0.X`** is created pointing
  at the **same commit** as each bare `1.0.X` tag. Both names coexist; nothing is
  rewritten.

The canonical line **restarts at `v1.1.0-rc.1`**.

### Tag inventory & requalification

| Legacy tag | Type | Commit | GitHub release | `v` alias created | Note |
| --- | --- | --- | --- | --- | --- |
| `1.0.0` | lightweight | `7305ff87` | 1.0.0 (pre-release) | `v1.0.0` | — |
| `1.0.1` | lightweight | `ec858b4a` | *(none)* | `v1.0.1` | **same commit as `1.0.2`**; never had a release |
| `1.0.2` | lightweight | `ec858b4a` | 1.0.2 (pre-release) | `v1.0.2` | duplicate commit of `1.0.1` |
| `1.0.3` | lightweight | `17fd1a2c` | 1.0.3 (pre-release) | `v1.0.3` | — |
| `1.0.4` | lightweight | `86e30a06` | 1.0.4 (pre-release) | `v1.0.4` | — |
| `1.0.5` | lightweight | `ac6f63ff` | 1.0.5 (pre-release) | `v1.0.5` | — |
| `1.0.6` | annotated | `6d023b77` | 1.0.6 (pre-release) | `v1.0.6` | — |
| `v1.0.7` | annotated | `758c0c7c` | 1.0.7 (pre-release) | *(already `v`)* | canonical form already |
| `1.0.8` | lightweight | `f16f5f4e` | 1.0.8 (pre-release) | `v1.0.8` | was not annotated |
| `v1.1.0-rc.1` | annotated | `82038e84` | v1.1.0-rc.1 (pre-release) | *(already `v`)* | **canonical line start** |

The `CHANGELOG.md` comparison links reference the canonical `v1.0.X` names; they
resolve once the alias tags are pushed. The duplicate `1.0.1`/`1.0.2` pair is left
intact (both are public) and documented here rather than "cleaned up".

## 5. What we never do

- **Never delete or move a published tag**, and **never force-push**. Requalify and
  document instead.
- **Never hand-edit** a downstream version (chart / package.json / binary) out of
  sync with `VERSION` — run `make sync-version` and let `make check-version` police it.
