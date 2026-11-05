// Risk class of a change, as pure functions (#1250, ADR 0009 D3). No I/O here.
//
// A change is red when it touches a red path, adds a destructive migration line, or comes
// from a fork. Anything the caller could not read is red too: the classifier fails closed.

// parseDiff turns `git diff --no-renames -U0` output into [{ path, added: [lines] }].
export function parseDiff(text) {
  const files = [];
  let cur = null;
  for (const line of String(text).split('\n')) {
    const m = /^diff --git a\/(.+?) b\/(.+)$/.exec(line);
    if (m) {
      cur = { path: m[2], added: [] };
      files.push(cur);
      continue;
    }
    if (!cur) continue;
    if (line.startsWith('+++ ') || line.startsWith('--- ')) continue;
    if (line.startsWith('+')) cur.added.push(line.slice(1));
  }
  return files;
}

function pathRule(rule, path) {
  if (rule.except && path.startsWith(rule.except)) return false;
  if (rule.match) return path === rule.match || path.startsWith(rule.match);
  if (rule.suffix) return path.endsWith(rule.suffix);
  if (rule.regex) return new RegExp(rule.regex).test(path);
  return false;
}

// classify returns { cls: 'green' | 'red', reasons: string[] }.
// input: { files: [{ path, added }], fork: boolean, error?: string }
export function classify(input, rules) {
  const reasons = [];
  if (input.error) reasons.push(`could not read the change (${input.error}): failing closed`);
  if (!rules || !Array.isArray(rules.red_paths)) reasons.push('no readable rule file: failing closed');
  if (input.fork) reasons.push('the PR comes from a fork');

  for (const f of input.files || []) {
    for (const r of rules?.red_paths || []) {
      if (pathRule(r, f.path)) reasons.push(`${f.path}: ${r.why}`);
    }
    for (const c of rules?.red_content || []) {
      if (!new RegExp(c.files).test(f.path)) continue;
      if (c.except_files && new RegExp(c.except_files).test(f.path)) continue;
      const re = new RegExp(c.added_line, 'i');
      const hit = f.added.find((l) => re.test(l));
      if (hit) reasons.push(`${f.path}: ${c.why} (\`${hit.trim().slice(0, 80)}\`)`);
    }
  }
  return { cls: reasons.length ? 'red' : 'green', reasons: [...new Set(reasons)] };
}

// Gate integrity (#1250 criterion 6): a change to the gates must not drop a required
// context. `workflowsText` is the concatenated text of the head's .github/workflows/*.yml.
export function missingContexts(workflowsText, contexts) {
  return contexts.filter((c) => {
    const esc = c.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    return !new RegExp(`^\\s*name:\\s*['"]?${esc}['"]?\\s*$`, 'm').test(workflowsText);
  });
}
