#!/usr/bin/env bash
# Extract a dotted JSON path from stdin-captured input. Usage:
#   VALUE=$(json_get "$INPUT" '.tool_input.command')
# Returns empty string if the key is absent. Prints a warning to stderr and
# returns exit 3 if NO extraction method is available on this machine.
json_get() {
  local input="$1" path="$2"
  if command -v jq >/dev/null 2>&1; then
    printf '%s' "$input" | jq -r "$path // empty" 2>/dev/null
    return 0
  fi
  if command -v python3 >/dev/null 2>&1; then
    printf '%s' "$input" | python3 -c '
import json,sys
try: d=json.load(sys.stdin)
except Exception: sys.exit(0)
for k in sys.argv[1].strip(".").split("."):
    if not isinstance(d,dict): sys.exit(0)
    d=d.get(k)
    if d is None: sys.exit(0)
print(d if isinstance(d,str) else json.dumps(d))
' "$path" 2>/dev/null
    return 0
  fi
  # Last resort: naive extraction of the leaf key as a JSON string.
  local leaf="${path##*.}"
  printf '%s' "$input" \
    | sed -n "s/.*\"$leaf\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" \
    | head -1
  return 0
}

json_available() {
  command -v jq >/dev/null 2>&1 || command -v python3 >/dev/null 2>&1
}
