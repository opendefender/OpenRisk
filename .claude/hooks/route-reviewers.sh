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
#
# Read-only. It runs git diff/remote/rev-parse and, without --labels, one
# GET through gh. Nothing from the diff is ever executed: file names are read
# NUL-separated and diff text is only matched with grep. External diff and
# textconv drivers are switched off, since they would run configured programs.
#
# Exit: 0 printed (possibly nothing), 2 bad usage or base ref, 3 labels unknown.
set -uo pipefail
export LC_ALL=C

die() { echo "route-reviewers: $2" >&2; exit "$1"; }

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

G=(git -c core.fsmonitor=false --no-pager)
RANGE="$BASE...HEAD"
diff_text() { "${G[@]}" diff --no-ext-diff --no-textconv --no-color --no-renames -U0 "$RANGE" -- "$@" 2>/dev/null \
  | grep -E '^[+-]' | grep -vE '^(\+\+\+|---) '; }

FILES=()
while IFS= read -r -d '' f; do FILES+=("$f"); done \
  < <("${G[@]}" diff --name-only -z --no-renames "$RANGE" 2>/dev/null)

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
COMPLIANCE='^backend/(pkg/compliance/|pkg/report/compliance_pdf|internal/application/(compliance|complianceaudit|evidence|report)/|internal/application/reportjob/compliance_generator|internal/domain/(compliance|compliance_audit|control_crosswalk|evidence)\.go|internal/handler/(compliance_handler|compliance_audit_handler|evidence_handler|export_handler)\.go|internal/infrastructure/repository/gorm_(compliance|compliance_audit|control_crosswalk|evidence)_repository)'
INFRA='^(backend|deploy|helm|deployment|migrations|database|\.github|\.claude/hooks)/|^\.claude/settings[^/]*\.json$|(^|/)(Dockerfile[^/]*|[^/]*\.dockerfile|docker-compose[^/]*\.ya?ml|compose[^/]*\.ya?ml)$'
DOCS='^docs/|(^|/)(README|ROADMAP|CHANGELOG)([.][^/]*)?$|release[-_ ]?notes|(^|/)RELEASE[^/]*\.md$'
LOCALES='^(frontend/src/locales/[^/]*\.json|content/i18n/[^/]*\.ts)$'
UI_SRC='^frontend/src/'
[ "$WEBSITE" -eq 1 ] && UI_SRC='^(src|content)/'
# Frontend files whose name says auth, sessions, tokens or uploads.
FE_SENSITIVE_PATH='auth([^o]|$)|authoriz|session|login|logout|sign-?in|sign-?up|mfa|totp|passkey|webauthn|password|sso|oauth|saml|invite|upload|api-?tokens?|questionnairetoken|csrf|cookie'

UI_FILES=()
for f in "${FILES[@]}"; do
  lower=$(printf '%s' "$f" | tr '[:upper:]' '[:lower:]')
  [[ "$f" == docs/* || "$lower" == *.md ]] || add code-reviewer
  [[ "$f" =~ $INFRA ]] && add devsecops
  [[ "$f" =~ $DOCS ]] && add product-verifier
  [[ "$f" =~ $COMPLIANCE ]] && add compliance-officer
  [[ "$f" =~ $LOCALES ]] && add i18n-steward
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
  ui_diff=$(diff_text "${UI_FILES[@]}")
  # Visible text, added or removed: t() calls, JSX text, text attributes,
  # toasts with a literal, and bare text lines inside multi-line JSX.
  if printf '%s\n' "$ui_diff" | grep -qE \
       -e "\bt\([[:space:]]*['\"\`]" \
       -e "(placeholder|title|aria-label|aria-description|alt|label|tooltip)=['\"][^'\"]*[[:alpha:]]" \
       -e "(toast|notify|alert|confirm)(\.[a-z]+)?\([[:space:]]*['\"\`][^'\"\`]*[[:alpha:]]" \
       -e '^[+-][[:space:]]*[[:upper:]][[:lower:]]+([[:space:]]+[^[:space:]<>{}=;()]+)+[[:space:]]*$'; then
    add i18n-steward
  else
    tsx=(); for f in "${UI_FILES[@]}"; do [[ "$f" == *.tsx ]] && tsx+=("$f"); done
    [ "${#tsx[@]}" -gt 0 ] && diff_text "${tsx[@]}" | grep -qE '>[^<>{}]*[[:alpha:]]{2,}[^<>{}]*<' && add i18n-steward
  fi
  # Auth, session and upload code that does not say so in its file name.
  fe=(); for f in "${UI_FILES[@]}"; do [[ "$f" == frontend/src/* ]] && fe+=("$f"); done
  [ "${#fe[@]}" -gt 0 ] && diff_text "${fe[@]}" | grep -qE \
    'localStorage|sessionStorage|document\.cookie|Authorization|Bearer|FormData|type=["'"'"']file|multipart|dangerouslySetInnerHTML|innerHTML|accessToken|refreshToken|access_token|refresh_token|withCredentials|credentials:|window\.location|postMessage' \
    && add devsecops
fi

# Locale keys that describe a capability go to product-verifier as well.
LOC=(); for f in "${FILES[@]}"; do [[ "$f" =~ $LOCALES ]] && LOC+=("$f"); done
[ "${#LOC[@]}" -gt 0 ] && diff_text "${LOC[@]}" | grep -qiE \
  '^[+-](.*[^[:alnum:]_])?[[:alnum:]_]*(description|subtitle|tagline|intro|feature|capabilit|benefit|claim)[[:alnum:]_]*["'"'"']?[[:space:]]*:' \
  && add product-verifier

[ "${#OUT[@]}" -gt 0 ] && printf '%s\n' "${OUT[@]}" | sort -u
exit 0
