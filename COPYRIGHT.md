# Copyright Notice

## OpenRisk — Open-Source Risk Management Platform

**Copyright © 2024-2026 OpenDefender Contributors**

---

## License

OpenRisk is **open-core** and ships as two editions in one repository, plus one
permissively-licensed layer that both editions share:

| Edition / layer | License | SPDX header |
|---|---|---|
| **Community Edition (CE)** — the core GRC platform | **GNU AGPL v3.0** ([`LICENSE`](./LICENSE)) | `AGPL-3.0-only` |
| **Enterprise Edition (EE)** — commercial add-ons | **OpenRisk Commercial License** ([`LICENSE.commercial`](./LICENSE.commercial)) | `LicenseRef-OpenRisk-Commercial` |
| **Design system** — `frontend/design-system/` and `frontend/src/shared/ds/` | **Apache License 2.0** ([`frontend/design-system/LICENSE`](./frontend/design-system/LICENSE)) | `Apache-2.0` |

Every source file declares its license in its `SPDX-License-Identifier` header.
[`LICENSING.md`](./LICENSING.md) is the **authoritative boundary** between the three;
where this file and `LICENSING.md` disagree, `LICENSING.md` wins.

The design system is deliberately permissive — it is meant to be copied and
extended, including by commercial code — and its attribution notice is
[`frontend/design-system/NOTICE`](./frontend/design-system/NOTICE), which covers
both directories. The "CANNOT relicense" and AGPL §13 points below apply to the
Community Edition, **not** to those two directories.

**Key points for the Community Edition (AGPLv3):**

- ✅ You **CAN** use, modify and distribute it — including commercially.
- ✅ You **CAN** self-host it, for yourself or inside your organisation.
- ⚠️ You **MUST** preserve the copyright and license notices.
- ⚠️ You **MUST** release your modified source under the AGPL if you distribute it.
- ⚠️ You **MUST** also release it if you let others **use your modified version over a
  network** — this is AGPL §13, and it is the clause that distinguishes the AGPL from
  the GPL.
- ❌ You **CANNOT** relicense the Community Edition under different terms.

Enterprise Edition files are **not** covered by the AGPL. They may only be used,
copied or deployed under a Commercial Agreement — see `LICENSE.commercial`.

---

## Copyright Ownership

Copyright is held collectively by the contributors who have submitted code to the
repository. By contributing to OpenRisk, you agree that:

1. Your contributions are submitted under the license declared by the
   `SPDX-License-Identifier` header of the file you edit — `AGPL-3.0-only` for the
   Community Edition, `Apache-2.0` for the two design-system directories
2. You have the right to submit the contribution
3. Your contribution becomes part of the collective work
4. You agree to the [Contributor License Agreement](./CLA.md)

The CLA grants OpenDefender the rights it needs to **dual-license** the collective
work, which is what makes a commercial Enterprise Edition possible alongside an AGPL
core. You retain ownership of your contributions.

---

## Trademark Notice

**"OpenDefender"** and **"OpenRisk"** are trademarks of the OpenDefender project.

The code is free software; the trademarks are not. The AGPL grants no trademark
rights. Use of the marks requires compliance with our trademark policy:

- ✅ Permitted: Referring to the original project, linking, academic citations
- ✅ Permitted: Unmodified redistribution with clear attribution
- ⚠️ Requires Permission: Derivative project names, endorsements, implying affiliation
- ❌ Prohibited: Misleading use, implying official status without authorization

Note that redistributing a **modified** build under the OpenRisk name is a trademark
question, not a licensing one: the AGPL permits the fork, the trademark policy governs
what you may call it.

---

## Third-Party Components

This project includes third-party libraries and components, each with their own
copyright and license terms. All third-party components bundled into the Community
Edition are compatible with `AGPL-3.0-only`. See individual component documentation
for specific copyright notices.

---

## Attribution Requirements

When using or distributing OpenRisk, you must:

1. **Preserve all copyright and SPDX notices** in source files
2. **Include the LICENSE file** in distributions
3. **Credit the OpenDefender project**
4. **Link to the original repository**: https://github.com/opendefender/OpenRisk
5. **Indicate modifications** if you have changed the code
6. **Offer the corresponding source** to anyone who interacts with your instance over
   a network (AGPL §13)

---

## DMCA & Copyright Violations

If you believe your copyright has been violated, contact us at:

- **GitHub Issues**: https://github.com/opendefender/OpenRisk/issues
- **Takedown Requests**: Follow GitHub's DMCA process

---

## Commercial Use

The AGPL permits commercial use, including offering OpenRisk as a hosted service —
**provided** you release your modifications under the AGPL to the users of that
service. Organisations that cannot accept the AGPL's source-disclosure obligations,
or that need the Enterprise Edition, require a commercial license.

For alternative licensing arrangements: **licensing@opendefender.io**

---

## Contact

For licensing questions or permissions:

- **Licensing**: licensing@opendefender.io
- **Repository**: https://github.com/opendefender/OpenRisk
- **Issues**: https://github.com/opendefender/OpenRisk/issues

---

*Last Updated: 2026-08-31*
