---
name: compliance-audit
description: Verify every control of a framework catalogue against its official source text — citation, obligation strength, scope, FR/EN fidelity, testability. Run before shipping any framework and before any compliance claim reaches a customer.
argument-hint: "[framework: cobac | bceao | antic-cm | iso-27001 | all]"
---

# Compliance audit: ${ARGUMENTS:-all frameworks}

Delegate to `compliance-officer`. This is the product's core differentiator —
budget real time for it.

## 1. Locate the catalogue

```
rg -l 'cobac|bceao|antic-cm|iso.?27001' --type go backend/ | head -20
```
Read the catalogue definition files, not the database.

## 2. Verify every control

Per control: citation resolves · obligation strength preserved · scope
preserved · reference code unique · FR and EN both faithful · testable ·
no duplicate obligation.

Verdicts: `SOURCED` · `PARAPHRASE-DRIFT` · `MIS-CITED` · `UNSOURCED` · `FABRICATED`.

## 3. Count check

ISO 27001:2022 Annex A has exactly 93 controls. Any other count means the
framework was polluted by another catalogue's import — this has happened in
this project before. Report the delta and the contaminating source.

## 4. Availability gate

A framework is `Available: true` only when every control is `SOURCED`, the
count is right, both locales are faithful, and the sign-off records the date
and the source document used.

`cm-loi-2024-017` remains a placeholder until the source text is supplied —
decision D-002.

## Output

Table per framework: reference · verdict · citation checked · finding · fix.
Then `COMPLIANCE: PASS` or `COMPLIANCE: FAIL — <n> unsourced, <n> fabricated`.
Open a P0 issue for every `FABRICATED`, P1 for every `UNSOURCED`/`MIS-CITED`.

Finally, hand the copyright question to `legal-counsel`: is any normative ISO
text reproduced verbatim anywhere in the catalogue?
