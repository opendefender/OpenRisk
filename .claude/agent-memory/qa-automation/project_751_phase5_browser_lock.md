---
name: project_751_phase5_browser_lock
description: Shared playwright-mcp Chrome profile can be locked by a concurrent sibling agent, blocking live verification entirely
metadata:
  type: project
---

During the #751 phase 5 QA pass (2026-09-29), `mcp__playwright__browser_navigate`
failed on every attempt (~10 tries over several minutes, interleaved with code
review) with `Browser is already in use for
/home/alex/.cache/ms-playwright-mcp/mcp-chrome-130bedd, use --isolated to run
multiple instances of the same browser`. `Default/` mtimes inside that profile
kept advancing in real time, confirming a sibling agent (likely motion-designer
or frontend-react doing their own review pass on the same phase) genuinely held
the lock, not a stale lockfile. No `--isolated` flag or alternate tool
(`ToolSearch` / `plugin_playwright_playwright` set mentioned in the task brief)
was actually available in the tool list handed to this agent.

**Why:** OpenRisk phase reviews run several agents (frontend-react,
motion-designer, qa-automation) against the same live dev stack in the same
orchestrated pass. They can collide on the one playwright-mcp browser profile.

**How to apply:** When a task brief promises an isolated/alternate Playwright
tool set via `ToolSearch`, verify it is actually present in the tool list
before relying on it — don't assume the brief's tooling claim is current. If
`browser_navigate` reports the profile in use, check `Default/` mtimes to
distinguish a live collision from a stale lock before waiting it out; do not
kill the browser process to force access, since another agent's in-progress
review may be using it. If it never frees up within a reasonable number of
retries, report the live-verification gap honestly (per [[project_751_phase4_defects]]'s
sibling entries) rather than substituting code/test review silently as if it
were live proof.
