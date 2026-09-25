# Security policy

OpenRisk is a governance, risk and compliance platform. The people who run it
store their weakest points in it, so we treat a vulnerability in OpenRisk as a
vulnerability in every organisation that uses it.

If you think you have found one, please tell us privately first. Do not open a
public issue, pull request or discussion for it.

Vous pouvez nous écrire en français ou en anglais.

## How to report

Use either channel:

- **GitHub private vulnerability reporting**: go to the repository's
  [Security tab](https://github.com/opendefender/OpenRisk/security) and choose
  *Report a vulnerability*. The report stays private between you and the
  maintainers, and it is where the advisory will be drafted.
- **Email**: [security@opendefender.io](mailto:security@opendefender.io).

We do not publish a PGP key yet. If what you need to send should not travel by
plain email, say so in a first short message and we will agree on a channel.

A useful report tells us:

- what the problem is, and what an attacker gains from it;
- the version or commit you tested (`GET /api/v1/health` returns both);
- the steps to reproduce it, ideally with a proof of concept;
- whether it needs an account, and with which role;
- how you would like to be credited, if at all.

A rough report is better than no report. We will ask for what is missing.

## What happens next

| Step | Our target |
|---|---|
| We confirm we received your report | 3 business days |
| We tell you whether we can reproduce it, and how severe we think it is | 10 business days |
| We keep you informed until it is fixed | at least every 14 days |
| We ship a fix | as fast as the severity requires, and within 90 days of the report |

These are targets for a small maintainer team, not contractual commitments. If
we are going to miss one, we will tell you before it passes rather than after.

Severity is assessed with CVSS v3.1. A flaw that lets one tenant read or change
another tenant's data is always treated as critical, whatever its score.

## Coordinated disclosure

We ask you to keep the details private until a fix is released, or until 90
days after your report, whichever comes first. If a fix needs longer, we will
explain why and agree a new date with you. If we stop answering, you are free
to publish after that 90-day deadline.

When the fix ships we publish a
[GitHub Security Advisory](https://github.com/opendefender/OpenRisk/security/advisories),
request a CVE when the issue warrants one, and describe the fix in
[CHANGELOG.md](CHANGELOG.md). You are credited in the advisory unless you ask
us not to be. We will never share your identity without your consent.

We do not run a paid bug bounty.

## Scope

In scope:

- the code in this repository: backend (`backend/`), frontend (`frontend/`),
  the SQL migrations, the install script, the Docker images and compose files,
  and the Helm chart (`helm/`);
- the release artifacts built from it.

Out of scope:

- instances that someone else runs. Report a flaw in their configuration to
  them. If the flaw is in our code, report it to us;
- denial of service by volume, spam, and social engineering of maintainers,
  contributors or users;
- findings that need an already compromised server, browser or device;
- scanner output with no demonstrated impact, including missing headers or
  cookie flags on their own;
- vulnerabilities in third-party dependencies that OpenRisk's use does not make
  exploitable. Report those upstream. If OpenRisk does expose one, it is in
  scope.

## Safe harbour

If you act in good faith under this policy, we will consider your research
authorised. We will not take legal action against you or ask anyone else to,
and if a third party does, we will make it known that you acted under this
policy.

Good faith means that you:

- test only against an instance you run yourself, or against your own account
  and data;
- do not access, change, keep or share anyone else's data. If you come across
  some by accident, stop, do not keep it, and tell us;
- do not degrade the service for other people, and do not use automated
  scanning that does;
- give us reasonable time to fix the problem before you disclose it, as
  described above;
- do not ask for payment in exchange for not disclosing.

If you are unsure whether something is allowed, ask us first.

## Supported versions

Security fixes are made on `master` and shipped in the next release. Only the
most recent release line receives them.

| Version | Security fixes |
|---|---|
| 1.1.0 release candidates (latest: see [`VERSION`](VERSION)) | Yes |
| 1.0.x and earlier | No. Upgrade to the latest release. |

Releases are cut from `master` as described in
[docs/VERSIONING.md](docs/VERSIONING.md). A critical fix gets its own release as
soon as it is ready and does not wait for the next planned one.

## Advisories

Security fixes shipped so far are listed under **Security** in each release of
[CHANGELOG.md](CHANGELOG.md). From now on, vulnerabilities reported under this
policy are also published on the repository's
[advisories page](https://github.com/opendefender/OpenRisk/security/advisories).

## Verifying a release

Releases built by the current release pipeline attach, next to the binary, the
frontend archive and the Helm chart, the files below. Releases published before
that pipeline have none of them.

- three SBOMs in CycloneDX JSON, generated by Trivy:
  `openrisk-backend-X.Y.Z.cdx.json` (Go modules),
  `openrisk-frontend-X.Y.Z.cdx.json` (npm production dependencies) and
  `openrisk-image-X.Y.Z.cdx.json` (the published image, OS packages included);
- `SHA256SUMS`, covering every file of the release.

Download every file of the release into one directory, then:

```sh
sha256sum -c SHA256SUMS
```

Every line must end in `OK`. The release notes give the container image,
`ghcr.io/opendefender/openrisk:X.Y.Z`, and its `sha256:` digest; pull by digest
to get exactly the image that was scanned. What can be rebuilt bit for bit from
the tag, and how to check it, is in
[docs/security/REPRODUCIBLE_BUILDS.md](docs/security/REPRODUCIBLE_BUILDS.md).

These files are not signed yet, so `SHA256SUMS` proves a download is intact, not
who produced it. Signing is tracked in
[#795](https://github.com/opendefender/OpenRisk/issues/795).

## What is and is not in place

We would rather tell you than let you assume:

- No external penetration test has been carried out yet.
- Known vulnerabilities in dependencies are checked by one blocking gate,
  [`.github/workflows/dependency-gate.yml`](.github/workflows/dependency-gate.yml),
  on every pull request, on `master` and once a day. It covers backend Go
  modules, frontend production dependencies and the images we ship, and fails
  on any HIGH or CRITICAL finding that has no exception in
  [`security/vulnerability-exceptions.yaml`](security/vulnerability-exceptions.yaml).
  Each exception names an owner and an issue, and expires within 90 days. A
  release is not published while the gate fails.
- Secret and static-analysis scans are configured in
  [`.github/workflows/security.yml`](.github/workflows/security.yml) and
  [`.github/workflows/security-scanning.yml`](.github/workflows/security-scanning.yml).
- Release files and images are not signed yet (tracked in
  [#795](https://github.com/opendefender/OpenRisk/issues/795)).
- OpenRisk ships no default password. The first administrator gets the
  password you set in `INITIAL_ADMIN_PASSWORD`, or one generated on first boot
  and written to a `0600` file (see
  [docs/SELF_HOSTING.md](docs/SELF_HOSTING.md#the-first-administrator)). An
  instance first started before this change may still use `admin123`, and the
  backend logs a warning at every boot until that password is changed.
