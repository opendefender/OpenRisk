#!/usr/bin/env bash
# Tenant tripwire (ABSOLUTE RULE 2): report a Go query added without a tenant
# filter, or a tenant filter removed.
#
#   tenant-tripwire.sh <file.go> [<base-rev>]
#
# Compares the file against <base-rev> (default HEAD; an untracked file counts
# as all added) and prints one line per finding. Exit 1 when there is one.
# Used by format-and-lint.sh on every Edit/Write and by gate-task-complete.sh on
# the whole branch, so edits made through Bash and deletions are caught too.
#
# On an ADDED line it looks for a GORM call that reads or writes rows:
#   Where Or Not Raw Exec · First/Find/Take/Last/Delete/Save(&…) ·
#   Update Updates UpdateColumn(s) Count Pluck
# and accepts it when a tenant reference is part of the SAME statement (the
# chain of lines joined by a trailing or leading dot, comma or open paren).
# The func signature and comments never count: a tenantID parameter two lines
# up says nothing about whether this query uses it (#958 review).
# .Or( always needs a justification, since it can step out of a tenant scope
# that sits earlier on the same chain.
#
# The escape hatch is a comment right above the statement, added in the same
# change, that contains "tenant:" and a reason:
#     // tenant: schema_migrations is global, it has no tenant column
#
# On a REMOVED line it reports a tenant reference that the change does not
# add back anywhere in the file: a dropped .Where("tenant_id = ?") or Scopes().
#
# This is an aid, not the control. The isolation registry tests and the
# tech-lead/devsecops review are.
set -uo pipefail
FILE="$1"; BASE="${2:-HEAD}"
[ -f "$FILE" ] || exit 0
DIR=$(dirname "$FILE")

if git -C "$DIR" ls-files --error-unmatch "$FILE" >/dev/null 2>&1; then
  DIFF=$(git -C "$DIR" diff -U0 "$BASE" -- "$FILE" 2>/dev/null)
else
  DIFF=$(awk '{ print "+" $0 }' "$FILE" | sed '1i @@ -0,0 +1 @@')
  NEWFILE=1
fi
[ -n "$DIFF" ] || exit 0

ADDED=$(printf '%s\n' "$DIFF" | awk -v newfile="${NEWFILE:-0}" '
  /^@@/ { split($3, a, ","); ln = substr(a[1], 2) + 0; next }
  /^\+\+\+/ { next }
  /^\+/ { if (newfile == "1") print NR - 1; else print ln; ln++; next }
')
REMOVED=$(printf '%s\n' "$DIFF" | grep -E '^-[^-]' | sed 's/^-//')
ADDED_TEXT=$(printf '%s\n' "$DIFF" | grep -E '^\+[^+]' | sed 's/^+//')

FINDINGS=$(printf '%s\n' "$ADDED" | awk -v file="$FILE" '
  FNR == NR { if ($1 != "") added[$1] = 1; next }
  { line[FNR] = $0 }
  function iscomment(s) { return s ~ /^[[:space:]]*\/\// }
  function isfunc(s)    { return s ~ /^[[:space:]]*func[[:space:](]/ }
  function cont_after(s) { return s ~ /[.(,][[:space:]]*$/ }
  function cont_before(s) { return s ~ /^[[:space:]]*[.)]/ }
  # The part of a line that can hold a query: for a func line, only its body
  # after the first "{", so a tenantID parameter never counts.
  # Without a trailing // comment, so "// tenant checked by caller" is no scope.
  function body(s,   i) {
    sub(/[[:space:]]\/\/[^"`]*$/, "", s)
    if (!isfunc(s)) return s
    i = index(s, "{"); return i ? substr(s, i + 1) : ""
  }
  # The local variable a statement queries through, or "" (r.db, a field,
  # does not count: that is the unscoped IDOR shape).
  function receiver(s, e,   k, txt, m) {
    txt = ""
    for (k = s; k <= e; k++) txt = txt " " body(line[k])
    if (match(txt, /(^|[^A-Za-z0-9_.])[a-z][A-Za-z0-9_]*[[:space:]]*\.[[:space:]]*(Where|Or|Not|Raw|Exec|First|Find|Take|Last|Delete|Save|Updates?|UpdateColumns?|Count|Pluck|Order|Limit|Offset|Model|Select|Omit|Joins|Preload|Session|WithContext|Scopes|Table|Distinct|Group)\(/)) {
      m = substr(txt, RSTART, RLENGTH)
      sub(/^[^a-z]*/, "", m); sub(/[[:space:]]*\..*/, "", m)
      return m
    }
    return ""
  }
  # Walk back through the func for the assignments to v. Scoped as soon as one
  # carries a tenant reference; unscoped at its declaration without one.
  function builder_scoped(v, s,   j, ej, k, decl, assign) {
    assign = "(^|[^A-Za-z0-9_.])" v "[[:space:]]*(:=|=)[^=]"
    decl = "(^|[^A-Za-z0-9_.])" v "[[:space:]]*:=|var[[:space:]]+" v "[[:space:]]"
    for (j = s - 1; j >= 1 && !isfunc(line[j]); j--) {
      if (iscomment(line[j]) || line[j] !~ assign) continue
      ej = j
      while (ej < FNR && (cont_after(line[ej]) || cont_before(line[ej+1]))) ej++
      for (k = j; k <= ej; k++) if (!iscomment(line[k]) && body(line[k]) ~ TENANT_RE) return 1
      if (line[j] ~ decl) return 0
      # Only q = q.<more chain> keeps the scope of an earlier assignment;
      # q = r.db.Model(...) starts over from an unscoped handle.
      if (line[j] !~ ("=[[:space:]]*" v "[[:space:]]*\\.")) return 0
    }
    return 0
  }
  BEGIN { TENANT_RE = "tenant_id|organization_id|OrganizationID|[tT]enant(ID|Id)?\\(|[tT]enant(ID|Id)([^A-Za-z0-9_]|$)|[tT]enant[A-Za-z]*Scope|[sS]cope[A-Za-z]*Tenant|(For|By|In)Tenant|[tT]enantDB" }
  END {
    # Delete/Save without & only on a db/tx receiver or after a call, so a
    # service method like svc.Save(ctx, x) is not taken for a query.
    q = "\\.(Where|Or|Not|Raw|Exec|First|Find|Take|Last)\\(|\\.(Delete|Save)\\(&|([dD][bB]|tx|\\))\\.(Delete|Save)\\(|\\.(Updates?|UpdateColumns?|Count|Pluck)\\("
    # Standard-library calls that share a name with a GORM method.
    stdlib = "(^|[^A-Za-z0-9_.])(bytes|strings|cmp|slices|maps|utf8|regexp|re)\\.(Count|Or|Find)[A-Za-z]*\\("
    # What counts as scoping: the column, or a tenant id, accessor or scope.
    # Not the bare word: &tenantInc or Preload("Tenant") scope nothing.
    t = TENANT_RE
    for (n = 1; n <= FNR; n++) {
      if (!(n in added) || iscomment(line[n]) || line[n] !~ q) continue
      probe = line[n]; gsub(stdlib, "", probe)
      if (probe !~ q) continue
      # Statement extent.
      s = n
      while (s > 1 && !isfunc(line[s-1]) && !iscomment(line[s-1]) && (cont_after(line[s-1]) || cont_before(line[s]))) s--
      e = n
      while (e < FNR && !isfunc(line[e+1]) && !iscomment(line[e+1]) && (cont_after(line[e]) || cont_before(line[e+1]))) e++
      # A raw SQL string opened on the query line runs until its closing
      # backtick; the tenant_id is often in there.
      if (gsub(/`/, "`", line[n]) % 2 == 1) {
        for (k = n + 1; k <= FNR; k++) { if (line[k] ~ /`/) { if (k > e) e = k; break } }
        while (e < FNR && (cont_after(line[e]) || cont_before(line[e+1]))) e++
      }
      ok = 0
      if (line[n] !~ /\.Or\(/) {
        for (k = s; k <= e; k++) if (!iscomment(line[k]) && body(line[k]) ~ t) ok = 1
      }
      # A local builder (q := db.Where("tenant_id = ?", t) ... q.Count(&n))
      # is scoped when its assignment earlier in the same func is.
      if (!ok && line[n] !~ /\.Or\(/) {
        v = receiver(s, e)
        if (v != "") ok = builder_scoped(v, s)
      }
      # Justification: a tenant: comment directly above the statement, added
      # in this change (an old comment does not cover a new query).
      j = s - 1
      while (j >= 1 && line[j] ~ /^[[:space:]]*$/) j--
      # The comment must give a reason: "tenant:" then at least ten characters.
      if (j >= 1 && (j in added) && iscomment(line[j]) && tolower(line[j]) ~ /tenant:[[:space:]]*[^[:space:]].........../) ok = 1
      if (!ok) printf "%s:%d: query without a tenant filter in its statement: %s\n", file, n, line[n]
    }
  }' - "$FILE")

if [ -n "$REMOVED" ]; then
  while IFS= read -r r; do
    case "$r" in *[tT]enant*) ;; *) continue ;; esac
    printf '%s' "$r" | grep -qE '^[[:space:]]*//' && continue
    printf '%s' "$r" | grep -qE '\.(Where|Scopes|Joins|Raw|Exec)\(|tenant_id' || continue
    norm=$(printf '%s' "$r" | tr -d '[:space:]')
    if ! printf '%s\n' "$ADDED_TEXT" | tr -d ' \t' | grep -qxF -- "$norm"; then
      FINDINGS+="${FINDINGS:+$'\n'}$FILE: tenant filter removed and not added back: $(printf '%s' "$r" | sed 's/^[[:space:]]*//')"
    fi
  done <<< "$REMOVED"
fi

[ -z "$FINDINGS" ] && exit 0
printf '%s\n' "$FINDINGS"
exit 1
