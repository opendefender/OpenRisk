#!/usr/bin/env bash
# Which reviewers a branch needs, chosen from the diff and the issue's labels,
# never by an agent's judgement (#1005). Prints one reviewer per line, sorted
# and deduplicated, so two runs on the same diff print the same text.
#
#   route-reviewers.sh <base-ref> [--labels "type:x,area:y"]
#
# Without --labels the issue number is read from the branch name
# (<type>/<n>-<slug>) and its labels from GitHub. If that fails the script
# exits 3 instead of guessing: a missing label would silently drop a reviewer.
# The branch name is chosen by whoever made the branch, so an unattended
# runner must pass --labels from its own queue, not rely on the branch.
#
# Read-only. It runs git diff/remote/rev-parse and, without --labels, one
# GET through gh. Nothing from the diff is ever executed: file names are read
# NUL-separated and diff text is only matched with grep. External diff and
# textconv drivers are switched off, since they would run configured programs.
#
# Exit: 0 printed (possibly nothing), 2 bad usage, bad base ref, no merge base
# or a failing git diff, 3 labels unknown.
set -uo pipefail
export LC_ALL=C

die() { echo "route-reviewers: $2" >&2; exit "$1"; }
# load_diff uses a nameref; on an older bash it would fail silently and drop
# every content rule (macOS /bin/bash is 3.2).
(( BASH_VERSINFO[0] > 4 || (BASH_VERSINFO[0] == 4 && BASH_VERSINFO[1] >= 3) )) \
  || die 2 "bash 4.3 or later is required"

BASE="" LABELS="" HAVE_LABELS=0
while [ $# -gt 0 ]; do
  case "$1" in
    --labels) [ $# -ge 2 ] || die 2 "--labels needs a value"; LABELS="$2"; HAVE_LABELS=1; shift 2 ;;
    --labels=*) LABELS="${1#--labels=}"; HAVE_LABELS=1; shift ;;
    -*) die 2 "unknown option: $1" ;;
    *) [ -z "$BASE" ] || die 2 "one base ref only"; BASE="$1"; shift ;;
  esac
done
[ -n "$BASE" ] || die 2 "usage: route-reviewers.sh <base-ref> [--labels a,b]"
git rev-parse --verify --quiet --end-of-options "$BASE^{commit}" >/dev/null \
  || die 2 "not a commit: $BASE"

# Paths from git diff are relative to the top level; so are the pathspecs.
cd "$(git rev-parse --show-toplevel)" || die 2 "not inside a git work tree"
G=(git -c core.fsmonitor=false --literal-pathspecs --no-pager)
# Pinned to SHAs so every git call below sees the same two commits.
RANGE="$(git rev-parse "$BASE^{commit}")...$(git rev-parse HEAD)"
# Fail closed: with no merge base (orphan branch, shallow clone) or a failing
# git diff, an empty list would mean "no reviewer" (#1005 security review).
"${G[@]}" merge-base "$BASE" HEAD >/dev/null 2>&1 \
  || die 2 "no merge base between $BASE and HEAD (shallow clone or unrelated history)"
# load_diff <var> <path>... sets <var> to the changed lines of those paths.
# Called at top level, never in $(), so die really stops the script.
load_diff() {
  local -n _out=$1; shift
  _out=$("${G[@]}" diff --no-ext-diff --no-textconv --no-color --no-renames -U0 "$RANGE" -- "$@") \
    || die 2 "git diff failed on $RANGE"
  _out=$(printf '%s\n' "$_out" | grep -E '^[+-]' | grep -vE '^(\+\+\+|---) ')
}

FILES=()
while IFS= read -r -d '' f; do FILES+=("$f"); done \
  < <("${G[@]}" diff --name-only -z --no-renames "$RANGE")
wait $! || die 2 "git diff failed on $RANGE"

if [ "$HAVE_LABELS" -eq 0 ]; then
  branch=$(git symbolic-ref --short -q HEAD || true)
  [[ "$branch" =~ ^[a-z]+/([0-9]+)- ]] \
    || die 3 "no issue number in branch '$branch'; pass --labels"
  LABELS=$(gh api "repos/{owner}/{repo}/issues/${BASH_REMATCH[1]}" --jq '[.labels[].name]|join(",")' 2>/dev/null) \
    || die 3 "cannot read the labels of #${BASH_REMATCH[1]}; pass --labels"
fi

# The marketing site is its own repository; its UI lives in src/ and content/.
WEBSITE=0
case "$(git remote get-url origin 2>/dev/null)" in *opendefender-website*) WEBSITE=1 ;; esac

OUT=()
add() { OUT+=("$1"); }

# --- labels ----------------------------------------------------------------
IFS=',' read -r -a LABEL_LIST <<< "$LABELS"
for l in "${LABEL_LIST[@]}"; do
  case "${l// /}" in
    area:design) add art-director ;;
    type:security) add devsecops ;;
  esac
done

# --- paths -----------------------------------------------------------------
# Compliance content, listed from the code on 2026-10-18 (#1005 comment):
# catalogues and crosswalks, the evidence model, audits, compliance reports.
COMPLIANCE='^backend/(pkg/compliance/|pkg/report/compliance_pdf|internal/application/(compliance|complianceaudit|evidence|report)/|internal/application/reportjob/compliance_generator|internal/domain/(compliance|control_crosswalk|evidence)[^/]*\.go|internal/infrastructure/workers/evidence_|internal/application/entity/resolver_(control|evidence)|internal/handler/(compliance_handler|compliance_audit_handler|evidence_handler|export_handler)\.go|internal/infrastructure/repository/gorm_(compliance|compliance_audit|control_crosswalk|evidence)_repository)'
# Everything CI, a release, an install or an agent session executes or loads:
# code, build and install scripts, dependency manifests, the shipped endpoint
# agent, the gate exceptions, and the agents' own instructions and memory.
INFRA='^(backend|agent|security|scripts|deploy|helm|deployment|migrations|database|\.github|\.claude/(hooks|agents|agent-memory|skills|commands))/|^frontend/(scripts/|nginx\.conf$|index\.html$|vite\.config\.[cm]?[jt]s$)|^\.claude/settings[^/]*\.json$|^(CLAUDE\.md|Makefile|\.mcp\.json|\.gitattributes|\.gitmodules|\.npmrc)$|(^|/)(package(-lock)?\.json|[^/]*\.sh|Dockerfile[^/]*|[^/]*\.[Dd]ockerfile|Containerfile[^/]*|docker-compose[^/]*\.ya?ml|compose[^/]*\.ya?ml)$'
DOCS='^docs/|(^|/)(README|ROADMAP|CHANGELOG)([.][^/]*)?$|release[-_ ]?notes|(^|/)RELEASE[^/]*\.md$'
LOCALES='^frontend/src/locales/[^/]*\.json$'
UI_SRC='^frontend/src/'
[ "$WEBSITE" -eq 1 ] && { LOCALES='^content/i18n/[^/]*\.ts$'; UI_SRC='^(src|content)/'; }
# Backend text a user reads: e-mail bodies and rendered templates.
BACKEND_TEXT='^backend/(.*/)?(templates?|emails?)/|^backend/.*\.(html|tmpl|gohtml)$|^backend/pkg/notify/email'
# Frontend files whose name says auth, sessions, tokens or uploads.
FE_SENSITIVE_PATH='auth([^o]|$)|authoriz|rbac|permission|role|session|login|logout|sign-?in|sign-?up|mfa|totp|passkey|webauthn|password|(^|[^a-z])sso|oauth|saml|invite|upload|api-?tokens?|questionnairetoken|csrf|cookie'

UI_FILES=()
for f in "${FILES[@]}"; do
  lower=$(printf '%s' "$f" | tr '[:upper:]' '[:lower:]')
  [[ "$f" == docs/* || "$lower" == *.md ]] || add code-reviewer
  [[ "$f" =~ $INFRA ]] && add devsecops
  [[ "$f" =~ $DOCS ]] && add product-verifier
  [[ "$f" =~ $COMPLIANCE ]] && add compliance-officer
  [[ "$f" =~ $LOCALES ]] && add i18n-steward
  [[ "$f" =~ $BACKEND_TEXT ]] && add i18n-steward
  if [[ "$f" =~ $UI_SRC && ! "$f" =~ $LOCALES ]] \
     && [[ ! "$f" =~ (__tests__/|\.test\.|\.spec\.|openapi\.generated\.ts$) ]]; then
    UI_FILES+=("$f")
    [[ "$lower" =~ ^frontend/src/ && "$lower" =~ $FE_SENSITIVE_PATH ]] && add devsecops
    [[ "$f" =~ (^|/)[^/]*([Ss]trings|[Cc]opy)\.tsx?$ ]] && add i18n-steward
  fi
done
[ "$WEBSITE" -eq 1 ] && [ "${#FILES[@]}" -gt 0 ] && { add website-dev; add product-verifier; }

# --- diff content ----------------------------------------------------------
if [ "${#UI_FILES[@]}" -gt 0 ]; then
  ui_diff="" tsx_diff="" fe_diff=""
  load_diff ui_diff "${UI_FILES[@]}"
  # Visible text, added or removed: t() calls, JSX text, text attributes,
  # toasts with a literal, literals in JSX braces, text-bearing properties,
  # and bare text lines inside multi-line JSX. Matched on a here-string, never
  # a pipe: grep -q exits early and pipefail would turn a match into a miss.
  if grep -qE \
       -e "\bt\([[:space:]]*['\"\`]" -e 'i18nKey=' \
       -e "(placeholder|title|aria-label|aria-description|alt|label|tooltip|header|description|message|text|emptyText|caption|subtitle|heading)[[:space:]]*[:=][[:space:]]*\{?[[:space:]]*['\"\`][^'\"\`]*[[:alpha:]]" \
       -e "\{[[:space:]]*['\"\`][^'\"\`]*[[:alpha:]]{2}" \
       -e "(toast|notify|alert|confirm)(\.[a-z]+)?\([[:space:]]*['\"\`][^'\"\`]*[[:alpha:]]" \
       -e '^[+-][[:space:]]*[[:upper:]][[:lower:]]+([[:space:]]+[^[:space:]<>{}=;()]+)+[[:space:]]*$' <<< "$ui_diff"; then
    add i18n-steward
  else
    tsx=(); for f in "${UI_FILES[@]}"; do [[ "$f" == *.tsx ]] && tsx+=("$f"); done
    [ "${#tsx[@]}" -gt 0 ] && load_diff tsx_diff "${tsx[@]}"
    grep -qE '>[^<>{}]*[[:alpha:]]{2,}[^<>{}]*<' <<< "$tsx_diff" && add i18n-steward
  fi
  # Auth, session and upload code that does not say so in its file name.
  fe=(); for f in "${UI_FILES[@]}"; do [[ "$f" == frontend/src/* ]] && fe+=("$f"); done
  [ "${#fe[@]}" -gt 0 ] && load_diff fe_diff "${fe[@]}"
  grep -qE \
    'hasRole|hasPermission|requiredRole|isAdmin|localStorage|sessionStorage|document\.cookie|Authorization|Bearer|FormData|type=["'"'"']file|multipart|dangerouslySetInnerHTML|innerHTML|accessToken|refreshToken|access_token|refresh_token|withCredentials|credentials:|window\.location|postMessage' <<< "$fe_diff" \
    && add devsecops
fi

# Locale keys that describe a capability go to product-verifier as well.
LOC=(); for f in "${FILES[@]}"; do [[ "$f" =~ $LOCALES ]] && LOC+=("$f"); done
loc_diff=""; [ "${#LOC[@]}" -gt 0 ] && load_diff loc_diff "${LOC[@]}"
grep -qiE \
  '^[+-](.*[^[:alnum:]_])?[[:alnum:]_]*(description|subtitle|tagline|intro|feature|capabilit|benefit|claim)[[:alnum:]_]*["'"'"']?[[:space:]]*:' \
  <<< "$loc_diff" && add product-verifier

[ "${#OUT[@]}" -gt 0 ] && printf '%s\n' "${OUT[@]}" | sort -u
exit 0
