---
name: openrisk-ux-doctrine
description: OpenRisk UX doctrine — the four personas, the 15 non-negotiable rules and the 20-criterion grading grid used to design and audit every screen. Load before designing, implementing or auditing any interface.
---

# OpenRisk UX Doctrine

## The four personas

| Persona | Job to be done | Failure mode to avoid |
|---|---|---|
| Risk Manager | Maintain the register, run assessments | Data-entry friction, lost work |
| Compliance Officer | Map controls to frameworks, prove coverage | Untraceable evidence |
| CISO | See exposure at a glance, decide where to spend | Dashboards hiding the trend |
| Auditor | Verify, sample, export, cite | Anything not exportable or timestamped |

## The 15 rules

1. Every screen answers: where am I, what can I do, what happened last.
2. Routine deletion is soft with a 5s undo. Vital deletion is informed friction
   with an impact readout and a safer alternative offered as a first-class button.
3. Nothing important behind hover alone on a touch-capable viewport.
4. Any list over 50 rows: bulk actions, server pagination, export.
5. Filters persist and are URL-shareable.
6. Loading, empty, partial, error and permission-denied are all designed.
7. Error copy: what happened, why, what to do next. Never "an error occurred".
8. Keyboard-operable end to end. Escape closes. Focus returns to the trigger.
9. Focus always visible, never trapped.
10. Severity never encoded by color alone.
11. Contrast 4.5:1 text, 3:1 UI. Measured.
12. No layout shift after load. CLS budget 0.05.
13. Motion honours `prefers-reduced-motion`; transform and opacity only.
14. FR and EN both native. No untranslated string reaches a user.
15. The interface never claims a capability the backend does not provide.

## The 20-criterion grid — score 1–5, anything under 4 gets a written fix

Clarity of purpose · Information hierarchy · Navigation predictability ·
Data density · Scan-ability · Action discoverability · Feedback latency ·
Error prevention · Error recovery · State completeness · Keyboard operability ·
Screen-reader semantics · Color independence · Contrast compliance ·
Responsive integrity · Motion restraint · i18n completeness · Design-system
consistency · Performance perception · Product truthfulness.
