#!/usr/bin/env bash
# Tests for the project hooks. Each case builds a throwaway git repo, feeds the
# hook the JSON Claude Code would send, and checks the exit code.
#   bash .claude/hooks/tests/run.sh
set -uo pipefail
unset CLAUDE_PROJECT_DIR
HOOKS="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PASS=0; FAILS=0
TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT

expect() { # expect <name> <want-exit> <got-exit> [stderr]
  if [ "$2" = "$3" ]; then PASS=$((PASS + 1)); echo "ok    $1"
  else FAILS=$((FAILS + 1)); echo "FAIL  $1 (want exit $2, got $3)"; [ -n "${4:-}" ] && printf '%s\n' "$4" | head -8 | sed 's/^/      /'; fi
}

new_repo() { # a repo on master with one committed repository file
  R="$TMP/$1"; mkdir -p "$R/backend/internal/infrastructure/repository"
  git -C "$R" init -q -b master
  git -C "$R" config user.email t@t; git -C "$R" config user.name t
  cat > "$R/backend/go.mod" <<'GO'
module example.com/gatetest

go 1.25
GO
  cat > "$R/backend/internal/infrastructure/repository/thing_repo.go" <<'GO'
package repository

type db interface{ Where(q string, a ...any) db }

// Legacy query, already reviewed, no tenant filter on purpose.
func legacy(d db) db { return d.Where("kind = ?", "x") }
GO
  git -C "$R" add -A; git -C "$R" commit -qm base
  git -C "$R" checkout -qb feature
}

tripwire() { # tripwire <repo> -> runs format-and-lint on the repo file
  local f="$1/backend/internal/infrastructure/repository/thing_repo.go"
  printf '{"tool_input":{"file_path":"%s"}}' "$f" | bash "$HOOKS/format-and-lint.sh" 2>"$TMP/err"
}

add_func() { printf '\n%s\n' "$2" >> "$1/backend/internal/infrastructure/repository/thing_repo.go"; }

# --- tenant tripwire -------------------------------------------------------
new_repo t1; add_func "$R" 'func a(d db) db { return d.Where("status = ?", 1) }'
tripwire "$R"; expect "tripwire blocks an added Where without tenant_id" 2 $? "$(cat "$TMP/err")"

new_repo t2; add_func "$R" 'func a(d db) db { return d.Where("tenant_id = ? AND status = ?", 1, 2) }'
tripwire "$R"; expect "tripwire passes a Where with tenant_id" 0 $? "$(cat "$TMP/err")"

new_repo t3; add_func "$R" 'func a(d interface{ First(any, ...any) error }, id int) error { var x struct{}; return d.First(&x, id) }'
tripwire "$R"; expect "tripwire blocks First(&x, id)" 2 $? "$(cat "$TMP/err")"

new_repo t4; add_func "$R" 'func a(d interface{ Raw(string, ...any) any }) any {
	// tenant: schema_migrations is global, it has no tenant column
	return d.Raw("SELECT version FROM schema_migrations")
}'
tripwire "$R"; expect "tripwire passes Raw with a tenant: comment" 0 $? "$(cat "$TMP/err")"

new_repo t5; add_func "$R" 'func a(d interface{ Exec(string, ...any) any }) any {
	// clean up
	return d.Exec("DELETE FROM things")
}'
tripwire "$R"; expect "tripwire blocks Exec behind an unrelated comment" 2 $? "$(cat "$TMP/err")"

new_repo t6; add_func "$R" 'func b() int { return 1 }'
tripwire "$R"; expect "tripwire ignores an untouched legacy query" 0 $? "$(cat "$TMP/err")"

new_repo t7; add_func "$R" 'func a(d db, t string) db {
	return d.Where(
		"tenant_id = ?", t)
}'
tripwire "$R"; expect "tripwire passes a multi-line Where with tenant_id" 0 $? "$(cat "$TMP/err")"

new_repo t8; add_func "$R" 'func GetRisk(d db, tenantID string, id int) db {
	var x struct{}
	_ = x
	return d.Where("id = ?", id)
}'
tripwire "$R"; expect "tripwire blocks the IDOR shape with tenantID only in the signature" 2 $? "$(cat "$TMP/err")"

new_repo t9; add_func "$R" 'func list(d db, t string) db {
	return d.
		Where("tenant_id = ?", t).
		Where("status = ?", 1)
}'
tripwire "$R"; expect "tripwire passes a chain scoped earlier in the same statement" 0 $? "$(cat "$TMP/err")"

new_repo t10; add_func "$R" 'func a(d interface{ Where(string, ...any) interface{ Or(string, ...any) any } }, t string, id int) any { return d.Where("tenant_id = ?", t).Or("id = ?", id) }'
tripwire "$R"; expect "tripwire blocks .Or( even after a tenant filter" 2 $? "$(cat "$TMP/err")"

new_repo t11; add_func "$R" 'func all(d interface{ Find(any, ...any) any }) any { var xs []int; return d.Find(&xs) }'
tripwire "$R"; expect "tripwire blocks Find(&xs) with no condition" 2 $? "$(cat "$TMP/err")"

new_repo t12
F="$R/backend/internal/infrastructure/repository/thing_repo.go"
python3 - "$F" <<'PY'
import sys
p=sys.argv[1]; s=open(p).read()
s=s.replace('// Legacy query, already reviewed, no tenant filter on purpose.\n',
            '// tenant: kinds are a global catalogue\n')
open(p,'w').write(s)
PY
git -C "$R" commit -qam "justify"
python3 - "$F" <<'PY'
import sys
p=sys.argv[1]; s=open(p).read()
s=s.replace('// tenant: kinds are a global catalogue\n',
            '// tenant: kinds are a global catalogue\nfunc sneaky(d db) db { return d.Where("owner = ?", 1) }\n')
open(p,'w').write(s)
PY
tripwire "$R"; expect "tripwire does not let a new query borrow an old tenant: comment" 2 $? "$(cat "$TMP/err")"

new_repo t13
F="$R/backend/internal/infrastructure/repository/thing_repo.go"
printf '\nfunc scoped(d db, t string) db {\n\treturn d.Where("tenant_id = ?", t)\n}\n' >> "$F"
git -C "$R" commit -qam "scoped"
sed -i 's/d.Where("tenant_id = ?", t)/d/' "$F"
bash "$HOOKS/tenant-tripwire.sh" "$F" >"$TMP/err" 2>&1; expect "tripwire reports a tenant filter that was removed" 1 $? "$(cat "$TMP/err")"

new_repo t14; add_func "$R" 'func page(d db, t string, s int) db {
	q := d.Where("tenant_id = ?", t)
	if s > 0 {
		q = q.Where("status = ?", s)
	}
	return q.Where("x = ?", 1)
}'
tripwire "$R"; expect "tripwire passes a builder scoped at its declaration" 0 $? "$(cat "$TMP/err")"

new_repo t15; add_func "$R" 'func page(d db, tenantID string) db {
	q := d.Where("kind = ?", 1)
	return q.Where("x = ?", 1)
}'
tripwire "$R"; expect "tripwire blocks a builder declared without a tenant filter" 2 $? "$(cat "$TMP/err")"

new_repo t16; add_func "$R" 'func all(d interface{ Find(any, ...any) any }, out any) any { return d.Find(out) }'
tripwire "$R"; expect "tripwire blocks Find(out) without &" 2 $? "$(cat "$TMP/err")"

new_repo t17; add_func "$R" 'type repo struct{ db interface{ Delete(any, ...any) any } }
func (r repo) del(x any) any { return r.db.Delete(x) }'
tripwire "$R"; expect "tripwire blocks r.db.Delete(x)" 2 $? "$(cat "$TMP/err")"

new_repo t18; add_func "$R" 'func a(d interface{ Raw(string, ...any) any }) any {
	// tenant:
	return d.Raw("SELECT 1")
}'
tripwire "$R"; expect "tripwire refuses a tenant: comment with no reason" 2 $? "$(cat "$TMP/err")"

new_repo t19; add_func "$R" 'func n(b []byte) int { return bytes.Count(b, []byte("x")) + strings.Count("a", "b") }'
tripwire "$R"; expect "tripwire ignores bytes.Count and strings.Count" 0 $? "$(cat "$TMP/err")"

# --- completion gate -------------------------------------------------------
gate() { printf '{"cwd":"%s","agent_type":"test-%s"}' "$1" "$2" | OPENRISK_GATE_STATE_DIR="$TMP" bash "$HOOKS/gate-task-complete.sh" 2>"$TMP/err"; }

new_repo g1; gate "$R" g1; expect "gate passes when nothing changed" 0 $? "$(cat "$TMP/err")"

new_repo g2; add_func "$R" 'func broken() int { return "x" }'
gate "$R" g2; rc=$?; expect "gate blocks a Go file that does not compile" 2 $rc "$(cat "$TMP/err")"
grep -q "go build" "$TMP/err"; expect "gate names the failing command" 0 $?

new_repo g3; printf 'package repository\nfunc  ugly( ) int {return 1}\n' > "$R/backend/internal/infrastructure/repository/ugly.go"
gate "$R" g3; expect "gate blocks an unformatted Go file" 2 $? "$(cat "$TMP/err")"

new_repo g4; mkdir -p "$R/frontend/src"; echo 'export const a=1' > "$R/frontend/src/a.ts"
gate "$R" g4; rc=$?; expect "gate blocks frontend changes it cannot check" 2 $rc "$(cat "$TMP/err")"
grep -q "node_modules is missing" "$TMP/err"; expect "gate says why the frontend was not checked" 0 $?

new_repo g5; add_func "$R" 'func broken() int { return "x" }'
for _ in 1 2 3; do gate "$R" g5; done; gate "$R" g5; rc=$?
expect "gate lets the agent stop after three blocks" 0 $rc "$(cat "$TMP/err")"
grep -q "GATE RED" "$R/.claude/CHECKPOINT.md" 2>/dev/null; expect "gate records GATE RED in the checkpoint" 0 $?

new_repo g6; add_func "$R" 'func broken() int { return "x" }'
printf '{"cwd":"%s","agent_type":"tech-lead","hook_event_name":"SubagentStop"}' "$R" | OPENRISK_GATE_STATE_DIR="$TMP" bash "$HOOKS/gate-task-complete.sh" 2>"$TMP/err"
expect "gate leaves a reviewer stopping on SubagentStop alone" 0 $? "$(cat "$TMP/err")"
printf '{"cwd":"%s","agent_type":"backend-go","hook_event_name":"SubagentStop"}' "$R" | OPENRISK_GATE_STATE_DIR="$TMP" bash "$HOOKS/gate-task-complete.sh" 2>"$TMP/err"
expect "gate blocks backend-go on SubagentStop" 2 $? "$(cat "$TMP/err")"

new_repo g7
printf '\nfunc viaBash(d db) db { return d.Where("status = ?", 1) }\n' >> "$R/backend/internal/infrastructure/repository/thing_repo.go"
gate "$R" g7; rc=$?; expect "gate catches an unscoped query added outside Edit/Write" 2 $rc "$(cat "$TMP/err")"
grep -q "tenant tripwire" "$TMP/err"; expect "gate names the tenant tripwire" 0 $?

new_repo g8; add_func "$R" 'func broken() int { return "x" }'
echo 'x[$(touch '"$TMP"'/pwned)]' > "$TMP/openrisk-gate-$(printf '%s' "$R|test-g8" | cksum | cut -d' ' -f1)"
gate "$R" g8; [ ! -e "$TMP/pwned" ]; expect "gate never evaluates the content of its state file" 0 $?

new_repo g9; MAIN="$R"
git -C "$MAIN" worktree add -q "$TMP/g9-wt" -b g9-wt master
printf '\nfunc broken() int { return "x" }\n' >> "$TMP/g9-wt/backend/internal/infrastructure/repository/thing_repo.go"
python3 -c 'import json,sys; print(json.dumps({"message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":sys.argv[1]}}]}}))' \
  "$TMP/g9-wt/backend/internal/infrastructure/repository/thing_repo.go" > "$TMP/g9.jsonl"
printf '{"cwd":"%s","agent_type":"backend-go","hook_event_name":"SubagentStop","agent_transcript_path":"%s"}' "$MAIN" "$TMP/g9.jsonl" \
  | OPENRISK_GATE_STATE_DIR="$TMP" bash "$HOOKS/gate-task-complete.sh" 2>"$TMP/err"
expect "gate checks the worktree the agent edited, not the session cwd" 2 $? "$(cat "$TMP/err")"
mkdir -p "$TMP/b"
python3 -c 'import json,sys; print(json.dumps({"message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"cd "+sys.argv[1]+" && go test ./..."}}]}}))' "$TMP/g9-wt" > "$TMP/g9b.jsonl"
printf '{"cwd":"%s","agent_type":"backend-go","hook_event_name":"SubagentStop","agent_transcript_path":"%s"}' "$MAIN" "$TMP/g9b.jsonl" \
  | OPENRISK_GATE_STATE_DIR="$TMP/b" bash "$HOOKS/gate-task-complete.sh" 2>"$TMP/err"
expect "gate follows a Bash cd into the worktree" 2 $? "$(cat "$TMP/err")"

new_repo g10; git -C "$R" branch -qm master trunk; git -C "$R" checkout -q trunk 2>/dev/null; add_func "$R" 'func ok() int { return 1 }'
gate "$R" g10; rc=$?; expect "gate refuses to pass when there is no master to compare with" 2 $rc "$(cat "$TMP/err")"

# --- bash guard ------------------------------------------------------------
guard() { python3 -c 'import json,sys;print(json.dumps({"tool_input":{"command":sys.argv[1]}}))' "$1" | bash "$HOOKS/guard-bash.sh" 2>/dev/null; }
guard "gh pr $(echo merge) 12"; expect "guard blocks merging a PR" 2 $?
guard 'git status'; expect "guard lets git status through" 0 $?

echo; echo "$PASS passed, $FAILS failed"
[ "$FAILS" -eq 0 ]
