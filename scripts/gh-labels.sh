#!/usr/bin/env bash
# Creates the fixed OpenRisk label taxonomy. Idempotent.
set -uo pipefail
mk() { gh label create "$1" --color "$2" --description "$3" --force >/dev/null 2>&1 \
       && echo "  ok $1" || echo "  skip $1"; }

echo "type:"
mk "type:feature"  "0E8A16" "New user-facing capability"
mk "type:bug"      "D73A4A" "Something is broken"
mk "type:chore"    "CFD3D7" "Maintenance, no user impact"
mk "type:security" "B60205" "Security defect or hardening"
mk "type:docs"     "0075CA" "Documentation"
mk "type:design"   "D876E3" "Design or UX work"
mk "type:debt"     "FBCA04" "Technical debt"

echo "area:"
mk "area:backend"   "1D76DB" "Go, /internal, /pkg"
mk "area:frontend"  "5319E7" "React, /src"
mk "area:infra"     "006B75" "Docker, K8s, CI"
mk "area:design"    "E99695" "Design system, tokens"
mk "area:marketing" "F9D0C4" "Site, copy, SEO"
mk "area:docs"      "C5DEF5" "README, ROADMAP, docs"
mk "area:db"        "BFD4F2" "Schema, migrations"

echo "priority:"
mk "priority:P0-critical" "B60205" "Production broken or exposed — work now"
mk "priority:P1-high"     "D93F0B" "Blocks a milestone"
mk "priority:P2-medium"   "FBCA04" "Normal milestone work"
mk "priority:P3-low"      "C2E0C6" "Nice to have"

echo "status:"
mk "status:needs-refinement" "E4E669" "Not workable yet"
mk "status:ready"            "0E8A16" "Meets the ready definition"
mk "status:in-progress"      "1D76DB" "An agent is working it"
mk "status:blocked"          "B60205" "Waiting on a decision or another issue"
mk "status:in-review"        "5319E7" "PR open"

echo "Done."
