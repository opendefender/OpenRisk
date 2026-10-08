---
name: website
description: Work on the opendefender-website marketing site — a codebase separate from the OpenRisk product. Every claim is verified against the product before it ships. Use for any marketing site change.
argument-hint: <what to build or fix>
---

# Website work: $ARGUMENTS

The site lives outside this repository. If it is not already in scope, add it:
```
/add-dir ~/Documents/projects/opendefender-website
```

## Order of operations — never skip step 1

1. **Truth first** — `product-verifier`: which claims involved here are
   `VERIFIED`? Anything `MOCKED`, `ABSENT` or `PARTIAL` either does not appear,
   or appears in the future tense with a visible planned marker.
2. **Positioning** — `brand-strategist` if the message is new.
3. **Copy** — `copywriter`, FR and EN, both native.
4. **Structure and structured data** — `seo-growth`.
5. **Visual direction** — `art-director` for any new token, type scale or color.
6. **Build** — `website-dev`. Reads the site's own config first; assumes nothing.
7. **Gate** — `performance-engineer` on Core Web Vitals, `qa-automation` on
   axe-core and keyboard traversal.

## Refuse to ship

- Any claim not cleared in step 1.
- Any Core Web Vitals regression.
- Any page that exists in one language only.
- Any fabricated customer, logo, testimonial or metric — including as a
  placeholder. Placeholders ship by accident.

## Output

What was built · which claims it makes and their matrix rows · the CWV numbers
before and after · what remains.
