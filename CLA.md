# OpenRisk Contributor License Agreement

**Version 1.0 — 2026-08-31**

Thank you for your interest in contributing to OpenRisk, a project of OpenDefender
("the Project").

This Contributor License Agreement ("Agreement") documents the rights granted by
contributors to the Project. It exists for one reason: OpenRisk is **open-core**. The
Community Edition is licensed under the GNU Affero General Public License v3.0
(`AGPL-3.0-only`), and a defined set of Enterprise Edition paths is licensed
commercially (`LicenseRef-OpenRisk-Commercial`). See [`LICENSING.md`](LICENSING.md)
for the authoritative boundary. Offering the same work under two licenses requires
that the Project hold sufficient rights in every contribution. This Agreement grants
those rights. **You keep the copyright in your work.**

This is a license agreement, not an assignment and not an employment or partnership
agreement.

---

## 1. Definitions

**"You"** means the individual or legal entity agreeing to this Agreement. For a legal
entity, "You" includes any entity that controls, is controlled by, or is under common
control with that entity.

**"Contribution"** means any work of authorship — including source code, documentation,
configuration, designs, translations and regulatory control content — that You
intentionally submit to the Project for inclusion in or documentation of any of its
products. "Submit" means any form of communication sent to the Project or its
maintainers, including pull requests, patches, issue attachments and discussion posts,
but **excluding** anything You conspicuously mark in writing as *"Not a Contribution."*

---

## 2. Copyright license

You grant to OpenDefender and to recipients of software distributed by the Project a
perpetual, worldwide, non-exclusive, no-charge, royalty-free, irrevocable copyright
license to reproduce, prepare derivative works of, publicly display, publicly perform,
sublicense and distribute Your Contributions and such derivative works.

This license expressly includes the right for OpenDefender to **distribute Your
Contribution under more than one license**, including the AGPL-3.0-only Community
Edition license, the OpenRisk Commercial License, and future licenses OpenDefender may
adopt for the Project.

---

## 3. Patent license

You grant to OpenDefender and to recipients of software distributed by the Project a
perpetual, worldwide, non-exclusive, no-charge, royalty-free, irrevocable (except as
stated in this section) patent license to make, have made, use, offer to sell, sell,
import and otherwise transfer the work. This license applies only to those patent
claims licensable by You that are necessarily infringed by Your Contribution alone or
by combination of Your Contribution with the work to which it was submitted.

If any entity institutes patent litigation against You or any other entity alleging
that Your Contribution, or the work it was submitted to, constitutes direct or
contributory patent infringement, then any patent licenses granted to that entity
under this Agreement for that Contribution terminate as of the date such litigation is
filed.

---

## 4. Your representations

You represent that:

1. You are legally entitled to grant the licenses above.
2. Each Contribution is Your original creation, or You have the necessary rights to
   submit it under this Agreement.
3. If Your employer has rights to intellectual property You create, You have either
   received permission to make the Contribution on behalf of that employer, or Your
   employer has waived such rights, or Your employer has agreed to this Agreement.
4. Your Contribution does not, to Your knowledge, violate any third party's copyright,
   patent, trademark, trade secret or other proprietary right.

**Third-party work.** If You submit work that is not Your original creation, You must
submit it separately from any original Contribution, identify its source and license,
and conspicuously mark it — for example `[third-party: MIT, from <project/url>]`. Only
licenses compatible with `AGPL-3.0-only` may be introduced into the Community Edition.

**Regulatory content.** Contributions of framework control content are subject to the
Project's citation standard: every control must cite a specific article of a specific
official text. You represent that any such Contribution does not reproduce copyrighted
standards text beyond what applicable law permits.

---

## 5. No warranty and no obligation

You provide Your Contributions on an **"AS IS"** basis, without warranties or
conditions of any kind, express or implied, including any warranty of merchantability,
fitness for a particular purpose, title or non-infringement — except for the
representations You make in Section 4.

The Project is under no obligation to accept, merge, use or maintain any Contribution.
Acceptance is at the maintainers' discretion.

---

## 6. Changed circumstances

You agree to notify the Project if You become aware of any fact or circumstance that
would make any representation in Section 4 inaccurate.

---

## 7. How You accept this Agreement

You accept this Agreement by adding a `Signed-off-by` trailer to each commit in Your
pull request, using Your real name and an email address You control:

```
Signed-off-by: Jane Doe <jane@example.com>
```

`git commit -s` adds it for you. The trailer certifies that You have read this
Agreement and that Your Contribution is submitted under it.

Corporate contributors whose employer requires a countersigned agreement should contact
**licensing@opendefender.io** before submitting.

> **Status of automated enforcement.** Acceptance is currently recorded by the
> `Signed-off-by` trailer in the git history. An automated check that blocks unsigned
> pull requests is **not yet wired** into the PR workflow — see the escalation in
> [`docs/DECISIONS.md`](docs/DECISIONS.md). Until it lands, maintainers verify the
> trailer during review.

---

## 8. Governing terms

This Agreement is governed by the laws applicable to OpenDefender, without regard to
conflict-of-law principles. If any provision is held unenforceable, the remainder stays
in effect.

Questions: **licensing@opendefender.io**
