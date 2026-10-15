#!/usr/bin/env python3
"""Token use per Claude Code session, and per PR those sessions opened (#1003).

Reads the local transcripts Claude Code keeps for this project:

    ~/.claude/projects/<project>/<session>.jsonl
    ~/.claude/projects/<project>/<session>/subagents/<agent>.jsonl (+ .meta.json)

and prints aggregates only: session ids, dates, model names, agent types, PR
numbers and token counts. It never prints a prompt, a command or a tool result,
because transcripts hold code, private text and sometimes secrets.

    python3 -I scripts/agents/token_usage.py --since 2026-10-01
    python3 -I scripts/agents/token_usage.py --last 5

Standard library only. Run with -I so nothing from the working directory is
imported.
"""

import argparse
import json
import os
import re
import subprocess
import sys
from collections import defaultdict
from pathlib import Path

CATEGORIES = (
    "input_tokens",
    "cache_creation_input_tokens",
    "cache_read_input_tokens",
    "output_tokens",
)
SHORT = {
    "input_tokens": "input",
    "cache_creation_input_tokens": "cache_write",
    "cache_read_input_tokens": "cache_read",
    "output_tokens": "output",
}

PR_URL = re.compile(r"https://github\.com/([\w.-]+)/([\w.-]+)/pull/(\d+)")
# A PR counts as opened by a session only when the command that produced the
# URL is a creation: `gh pr create`, or a POST to the pulls endpoint. A URL in
# the output of `gh pr view` or `gh pr list` is not an opening.
PR_CREATE = re.compile(r"\bgh\s+pr\s+create\b")
PR_API_POST = re.compile(
    r"\bgh\s+api\b.*(-X\s*POST|--method\s+POST).*?/pulls\b"
    r"|\bgh\s+api\b.*?/pulls\b.*(-X\s*POST|--method\s+POST)",
    re.S,
)


# Subagent transcripts store the usage of the first streamed chunk of each
# reply: input and cache counts are complete there, output is not (it reads 8
# or 16 whatever the reply's length). Main-session transcripts store the final
# usage. Checked on this machine's transcripts on 2026-10-15 (#1003).
SUBAGENT_OUTPUT_NOTE = (
    "Note: subagent output tokens are a floor, not a count. Their transcripts "
    "keep the usage of the first streamed chunk; input and cache figures are complete."
)


class Stats:
    def __init__(self):
        # tokens[model][category], tokens_by_agent[agent][category]
        self.tokens = defaultdict(lambda: defaultdict(int))
        self.by_agent = defaultdict(lambda: defaultdict(int))
        self.prs = set()  # (owner, repo, number)
        self.start = None
        self.skipped = 0

    def total(self, category):
        return sum(m[category] for m in self.tokens.values())


def default_projects_dir():
    """The transcript folder of the main checkout, even from a worktree."""
    try:
        out = subprocess.run(
            ["git", "worktree", "list", "--porcelain"],
            capture_output=True, text=True, check=True,
        ).stdout
    except (OSError, subprocess.CalledProcessError):
        return None
    for line in out.splitlines():
        if line.startswith("worktree "):
            main = line[len("worktree "):]
            slug = re.sub(r"[^A-Za-z0-9]", "-", main)
            return Path.home() / ".claude" / "projects" / slug
    return None


def tool_result_text(block):
    content = block.get("content")
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        return "\n".join(
            part.get("text", "") for part in content
            if isinstance(part, dict) and part.get("type") == "text"
        )
    return ""


def read_transcript(path, agent, stats, seen):
    """Add one transcript's usage and PR openings to stats.

    `seen` holds message ids already counted: Claude Code writes one line per
    content block, and every line of a message repeats the same usage.
    """
    commands = {}  # tool_use id -> command, kept only for matching, never printed
    try:
        handle = path.open(encoding="utf-8", errors="replace")
    except OSError:
        stats.skipped += 1
        return
    with handle:
        for raw in handle:
            try:
                entry = json.loads(raw)
            except ValueError:
                stats.skipped += 1
                continue
            if not isinstance(entry, dict):
                stats.skipped += 1
                continue

            stamp = entry.get("timestamp")
            if isinstance(stamp, str) and (stats.start is None or stamp < stats.start):
                stats.start = stamp

            message = entry.get("message")
            if not isinstance(message, dict):
                continue
            content = message.get("content")
            blocks = content if isinstance(content, list) else []

            if entry.get("type") == "assistant":
                for block in blocks:
                    if isinstance(block, dict) and block.get("type") == "tool_use":
                        command = (block.get("input") or {}).get("command")
                        if isinstance(command, str):
                            commands[block.get("id")] = command
                usage = message.get("usage")
                msg_id = message.get("id")
                if not isinstance(usage, dict):
                    if msg_id is not None and msg_id not in seen:
                        stats.skipped += 1
                        seen.add(msg_id)
                    continue
                if msg_id is not None:
                    if msg_id in seen:
                        continue
                    seen.add(msg_id)
                model = message.get("model") or "unknown"
                for category in CATEGORIES:
                    value = usage.get(category)
                    if isinstance(value, int):
                        stats.tokens[model][category] += value
                        stats.by_agent[agent][category] += value

            elif entry.get("type") == "user":
                for block in blocks:
                    if not isinstance(block, dict) or block.get("type") != "tool_result":
                        continue
                    command = commands.get(block.get("tool_use_id"), "")
                    if not (PR_CREATE.search(command) or PR_API_POST.search(command)):
                        continue
                    for owner, repo, number in PR_URL.findall(tool_result_text(block)):
                        stats.prs.add((owner, repo, int(number)))


def agent_name(meta_path):
    try:
        meta = json.loads(meta_path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return "subagent"
    for key in ("customAgentType", "spawnedAgentType", "agentType"):
        value = meta.get(key)
        if isinstance(value, str) and value:
            return value
    return "subagent"


def load_sessions(projects_dir):
    sessions = {}
    for path in sorted(projects_dir.glob("*.jsonl")):
        session_id = path.stem
        stats = Stats()
        seen = set()
        read_transcript(path, "main", stats, seen)
        sub_dir = projects_dir / session_id / "subagents"
        if sub_dir.is_dir():
            for sub in sorted(sub_dir.glob("*.jsonl")):
                read_transcript(sub, agent_name(sub.with_suffix(".meta.json")), stats, seen)
        sessions[session_id] = stats
    return sessions


def merged_state(pr):
    owner, repo, number = pr
    try:
        out = subprocess.run(
            ["gh", "api", f"repos/{owner}/{repo}/pulls/{number}",
             "--jq", ".merged_at // \"\""],
            capture_output=True, text=True, timeout=30,
        )
    except (OSError, subprocess.TimeoutExpired):
        return "?"
    if out.returncode != 0:
        return "?"
    return "merged" if out.stdout.strip() else "not merged"


def fmt(n):
    return f"{n:,}".replace(",", " ")


def print_session(session_id, stats):
    prs = " ".join(f"#{n}" for _, _, n in sorted(stats.prs)) or "-"
    print(f"{session_id[:8]}  {(stats.start or '?')[:10]}  PRs {len(stats.prs)}: {prs}")
    for model in sorted(stats.tokens):
        row = "  ".join(f"{SHORT[c]} {fmt(stats.tokens[model][c])}" for c in CATEGORIES)
        print(f"    model {model:<20} {row}")
    for agent in sorted(stats.by_agent):
        row = "  ".join(f"{SHORT[c]} {fmt(stats.by_agent[agent][c])}" for c in CATEGORIES)
        print(f"    agent {agent:<20} {row}")


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    parser.add_argument("--projects-dir", type=Path,
                        help="transcript folder (default: this repo's, from the main checkout)")
    parser.add_argument("--since", help="only sessions started on or after YYYY-MM-DD")
    parser.add_argument("--last", type=int, metavar="N",
                        help="summarise the N latest sessions that opened a PR")
    parser.add_argument("--no-gh", action="store_true",
                        help="do not ask GitHub whether each PR is merged")
    parser.add_argument("--merged-only", action="store_true",
                        help="with --last: keep sessions that opened a merged PR, "
                             "and divide by merged PRs only")
    args = parser.parse_args(argv)

    projects_dir = args.projects_dir or default_projects_dir()
    if projects_dir is None or not projects_dir.is_dir():
        print(f"transcript folder not found: {projects_dir}", file=sys.stderr)
        return 2

    sessions = load_sessions(projects_dir)
    if args.since:
        sessions = {k: v for k, v in sessions.items()
                    if v.start is not None and v.start[:10] >= args.since}
    ordered = sorted(sessions.items(), key=lambda kv: kv[1].start or "")
    skipped = sum(s.skipped for s in sessions.values())

    if args.last is None:
        for session_id, stats in ordered:
            print_session(session_id, stats)
        print(f"\n{len(ordered)} sessions, {skipped} lines skipped (malformed or without usage)")
        return 0

    if args.merged_only and args.no_gh:
        print("--merged-only needs GitHub; drop --no-gh", file=sys.stderr)
        return 2
    states = {}
    with_prs = []
    # Newest first, so --merged-only asks GitHub about as few PRs as it can.
    for session_id, stats in reversed(ordered):
        if len(with_prs) == args.last:
            break
        if not stats.prs:
            continue
        for pr in stats.prs:
            states[pr] = "?" if args.no_gh else merged_state(pr)
        if args.merged_only and not any(states[pr] == "merged" for pr in stats.prs):
            continue
        with_prs.append((session_id, stats))
    with_prs.reverse()
    kind = "a merged PR" if args.merged_only else "a PR"
    print(f"Latest {len(with_prs)} sessions that opened {kind}\n")
    print("| session | date | PRs opened | merged | input | cache_write | cache_read | output |")
    print("|---|---|---|---|---|---|---|---|")
    totals = defaultdict(int)
    agents = defaultdict(lambda: defaultdict(int))
    pr_count = 0
    for session_id, stats in with_prs:
        prs = " ".join(f"#{n}" for _, _, n in sorted(stats.prs))
        merged = sum(1 for pr in stats.prs if states[pr] == "merged")
        cells = [fmt(stats.total(c)) for c in CATEGORIES]
        print(f"| {session_id[:8]} | {(stats.start or '?')[:10]} | {prs} | "
              f"{merged}/{len(stats.prs)} | " + " | ".join(cells) + " |")
        for c in CATEGORIES:
            totals[c] += stats.total(c)
        for agent, row in stats.by_agent.items():
            for c in CATEGORIES:
                agents[agent][c] += row[c]
        pr_count += merged if args.merged_only else len(stats.prs)
    if with_prs:
        print("\nBy agent, over these sessions:\n")
        print("| agent | input | cache_write | cache_read | output |")
        print("|---|---|---|---|---|")
        for agent in sorted(agents, key=lambda a: -agents[a]["output_tokens"]):
            print(f"| {agent} | " + " | ".join(fmt(agents[agent][c]) for c in CATEGORIES) + " |")
    if pr_count:
        label = "merged PR" if args.merged_only else "PR opened"
        divisor = "merged PRs" if args.merged_only else "PRs opened"
        print(f"\nPer {label} (session totals divided by {divisor}):\n")
        print("| PRs | input | cache_write | cache_read | output |")
        print("|---|---|---|---|---|")
        print(f"| {pr_count} | " + " | ".join(fmt(totals[c] // pr_count) for c in CATEGORIES) + " |")
    if any(agent != "main" for agent in agents):
        print(f"\n{SUBAGENT_OUTPUT_NOTE}")
    print(f"\n{skipped} lines skipped (malformed or without usage)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
