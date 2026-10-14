---
name: openrisk-compliance-doctrine
description: OpenRisk regulatory content doctrine — control authoring format, citation standard, mapping rules between frameworks, and the evidence model. Load before writing, editing or verifying any framework control.
---

# OpenRisk Regulatory Content Doctrine

## Control authoring format

```
ReferenceCode   stable, unique per framework, never renumbered (e.g. COBAC-R16-4.2.1)
Title           the obligation in one line, active voice
Requirement     what the organization must do, in our own words
SourceCitation  <Regulation> <number>, <article/section>, <year>
SourceQuote     the operative phrase only, under 25 words, in the original language
Category        governance | access | operations | continuity | reporting | data
EvidenceType    document | configuration | log | attestation | test result
Testable        yes | no — "no" is not shippable
```

## Citation standard

- Name the instrument, its number, and the article. "COBAC regulation" is not a
  citation. "COBAC R-2016/04, art. 12" is.
- Never cite a summary, a blog post, or a consultant's commentary as the source.
- If two instruments impose the same obligation, cite the one that binds our
  target customer, and note the other in the requirement text.
- A control with no resolvable citation is not written. There is no draft state
  for regulatory content.

## Cross-framework mapping

Mappings are many-to-many and directional. `COBAC-R16-4.2.1 satisfies
ISO-A.5.15` means evidence for the first is admissible for the second — assert
that only when the obligations genuinely overlap in scope AND strength. A
partial overlap is recorded as `partial` with the gap described, never as full
coverage. Overstated mapping coverage is the fastest way to fail a customer's
real audit.

## Evidence model

Every control declares what evidence satisfies it. Evidence is tenant-scoped,
timestamped, attributed to an uploader, and immutable once attached to a
closed assessment. An auditor persona must be able to export the control, its
evidence, and its citation as one artifact.

## Framework availability gate

A framework is `Available: true` only when: every control is `SOURCED`, the
count matches the official standard where one exists, FR and EN are both
faithful, and `compliance-officer` has signed off with a date and the source
document used. Anything else is `Available: false` with an honest reason
shown in the UI.
