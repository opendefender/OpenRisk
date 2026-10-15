#!/usr/bin/env python3
"""Token use per Claude Code session, and per PR those sessions opened (#1003).

Reads the local transcripts Claude Code keeps for this project:

    ~/.claude/projects/<project>/<session>.jsonl
    ~/.claude/projects/<project>/<session>/subagents/<agent>.jsonl (+ .meta.json)

and prints aggregates only: session ids, dates, model names, agent types, PR
numbers and token counts. It never prints a prompt, a command or a tool result,
because transcripts hold code, private text and sometimes secrets.

    python3 -I scripts/agents/token_usage.py --since 2026-10-01
    python3 -I scripts/agents/token_usage.py --last 5 --merged-only

Standard library only. Run with -I so nothing from the working directory is
imported.
"""

import argparse
import json
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
DEFAULT_REPO = ("opendefender", "OpenRisk")

PR_URL = re.compile(r"https://github\.com/([\w.-]+)/([\w.-]+)/pull/(\d+)")

# A PR counts as opened by a session only when the command that produced the
# URL is a creation: `gh pr create`, or a POST to the pulls collection. A URL in
# the output of `gh pr view`, of a search, or of a review POST is not an opening.
#
# `gh` must start a shell command: at the start, after a separator (; & | newline),
# an opening ( { $( or do/then/else, and optional VAR=value assignments. So a
# mention such as `rg 'gh pr create' docs/` is not a creation.
_COMMAND_START = r"(?:^|[;&|({\n]|\$\(|\bdo\b|\bthen\b|\belse\b)\s*(?:\w+=\S*\s+)*"
PR_CREATE = re.compile(_COMMAND_START + r"gh\s+pr\s+create\b")
GH_API = re.compile(_COMMAND_START + r"gh\s+api\b")
# The pulls collection itself, not /pulls/5/reviews or /pulls/5/requested_reviewers.
PULLS_COLLECTION = re.compile(r"repos/[\w.-]+/[\w.-]+/pulls(?=[\s\"'?]|$)")
EXPLICIT_POST = re.compile(r"(?:-X\s*|--method[\s=])POST\b")
EXPLICIT_GET = re.compile(r"(?:-X\s*|--method[\s=])GET\b")
# gh api switches to POST by itself when it is given fields or an input body.
FIELD_FLAGS = re.compile(r"(?:^|\s)(?:-[fF]|--field|--raw-field|--input)(?:[\s=]|$)")

# Main-session transcripts write the final usage on every line of a reply.
# Subagent transcripts write a growing usage: input and cache are the same on
# every line, output grows, and the last line written is not always the final
# count. The script keeps the highest value seen per reply. Checked on this
# machine's transcripts on 2026-10-15 (#1003).
SUBAGENT_OUTPUT_NOTE = (
    "Note: subagent output tokens are a floor, not a count. Their transcripts do "
    "not always keep a reply's final output count; input and cache figures are complete."
)


def opens_pr(command):
    if PR_CREATE.search(command):
        return True
    if not (GH_API.search(command) and PULLS_COLLECTION.search(command)):
        return False
    if EXPLICIT_GET.search(command):
        return False
    return bool(EXPLICIT_POST.search(command) or FIELD_FLAGS.search(command))


class Stats:
    def __init__(self):
        # replies[msg_id] = [model, agent, {category: highest value seen}]
        self.replies = {}
        self.anonymous = 0  # replies without an id, counted as they come
        self.tokens = defaultdict(lambda: defaultdict(int))
        self.by_agent = defaultdict(lambda: defaultdict(int))
        self.prs = set()  # PR numbers in the target repository
        self.start = None
        self.skipped = 0

    def add(self, msg_id, model, agent, usage):
        if msg_id is None:
            self.anonymous += 1
            msg_id = ("anonymous", self.anonymous)
        reply = self.replies.setdefault(msg_id, [model, agent, defaultdict(int)])
        for category in CATEGORIES:
            value = usage.get(category)
            if isinstance(value, int) and value > reply[2][category]:
                reply[2][category] = value

    def finish(self):
        for model, agent, usage in self.replies.values():
            for category in CATEGORIES:
                self.tokens[model][category] += usage[category]
                self.by_agent[agent][category] += usage[category]

    def total(self, category):
        return sum(m[category] for m in self.tokens.values())


def main_checkout():
    try:
        out = subprocess.run(
            ["git", "worktree", "list", "--porcelain"],
            capture_output=True, text=True, check=True,
        ).stdout
    except (OSError, subprocess.CalledProcessError):
        return None
    for line in out.splitlines():
        if line.startswith("worktree "):
            return line[len("worktree "):]
    return None


def default_projects_dir():
    """The transcript folder of the main checkout, even from a worktree."""
    main = main_checkout()
    if main is None:
        return None
    slug = re.sub(r"[^A-Za-z0-9]", "-", main)
    return Path.home() / ".claude" / "projects" / slug


def tool_result_text(block):
    content = block.get("content")
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        return "\n".join(
            part.get("text", "") for part in content
            if isinstance(part, dict) and isinstance(part.get("text"), str)
        )
    return ""


def read_transcript(path, agent, stats, repo):
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
            blocks = [b for b in content if isinstance(b, dict)] if isinstance(content, list) else []

            if entry.get("type") == "assistant":
                for block in blocks:
                    if block.get("type") != "tool_use":
                        continue
                    tool_input = block.get("input")
                    command = tool_input.get("command") if isinstance(tool_input, dict) else None
                    tool_id = block.get("id")
                    if isinstance(command, str) and isinstance(tool_id, str):
                        commands[tool_id] = command
                usage = message.get("usage")
                if not isinstance(usage, dict):
                    stats.skipped += 1
                    continue
                msg_id = message.get("id")
                model = message.get("model")
                stats.add(msg_id if isinstance(msg_id, str) else None,
                          model if isinstance(model, str) else "unknown", agent, usage)

            elif entry.get("type") == "user":
                for block in blocks:
                    if block.get("type") != "tool_result":
                        continue
                    tool_id = block.get("tool_use_id")
                    command = commands.get(tool_id, "") if isinstance(tool_id, str) else ""
                    if not opens_pr(command):
                        continue
                    for owner, name, number in PR_URL.findall(tool_result_text(block)):
                        if (owner, name) == repo:
                            stats.prs.add(int(number))


def agent_name(meta_path):
    try:
        meta = json.loads(meta_path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return "subagent"
    if not isinstance(meta, dict):
        return "subagent"
    # Only the type keys: `name` and `description` hold task text.
    for key in ("customAgentType", "spawnedAgentType", "agentType"):
        value = meta.get(key)
        if isinstance(value, str) and re.fullmatch(r"[\w.-]{1,64}", value):
            return value
    return "subagent"


def load_sessions(projects_dir, repo=DEFAULT_REPO):
    sessions = {}
    for path in sorted(projects_dir.glob("*.jsonl")):
        session_id = path.stem
        stats = Stats()
        read_transcript(path, "main", stats, repo)
        sub_dir = projects_dir / session_id / "subagents"
        if sub_dir.is_dir():
            for sub in sorted(sub_dir.glob("*.jsonl")):
                read_transcript(sub, agent_name(sub.with_suffix(".meta.json")), stats, repo)
        stats.finish()
        sessions[session_id] = stats
    return sessions


def merged_state(repo, number):
    owner, name = repo
    try:
        out = subprocess.run(
            ["gh", "api", f"repos/{owner}/{name}/pulls/{number}",
             "--jq", "if .merged_at then \"merged\" else .state end"],
            capture_output=True, text=True, timeout=30,
        )
    except (OSError, subprocess.TimeoutExpired):
        return "?"
    state = out.stdout.strip()
    if out.returncode != 0 or state not in ("merged", "open", "closed"):
        return "?"
    return state


def parse_repo(value):
    match = re.fullmatch(r"([\w-]+(?:\.[\w-]+)*)/([\w-]+(?:\.[\w-]+)*)", value or "")
    return (match.group(1), match.group(2)) if match else None


def fmt(n):
    return f"{n:,}".replace(",", " ")


def print_session(session_id, stats):
    prs = " ".join(f"#{n}" for n in sorted(stats.prs)) or "-"
    print(f"{session_id[:8]}  {(stats.start or '?')[:10]}  PRs {len(stats.prs)}: {prs}")
    for model in sorted(stats.tokens):
        if not any(stats.tokens[model].values()):
            continue  # `<synthetic>` replies carry no tokens
        row = "  ".join(f"{SHORT[c]} {fmt(stats.tokens[model][c])}" for c in CATEGORIES)
        print(f"    model {model:<20} {row}")
    for agent in sorted(stats.by_agent):
        row = "  ".join(f"{SHORT[c]} {fmt(stats.by_agent[agent][c])}" for c in CATEGORIES)
        print(f"    agent {agent:<20} {row}")


def positive(value):
    n = int(value)
    if n < 1:
        raise argparse.ArgumentTypeError("must be 1 or more")
    return n


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    parser.add_argument("--projects-dir", type=Path,
                        help="transcript folder (default: this repo's, from the main checkout)")
    parser.add_argument("--repo", default="/".join(DEFAULT_REPO),
                        help="owner/name whose PRs are counted (default: %(default)s)")
    parser.add_argument("--since", help="only sessions started on or after YYYY-MM-DD")
    parser.add_argument("--last", type=positive, metavar="N",
                        help="summarise the N latest sessions that opened a PR")
    parser.add_argument("--no-gh", action="store_true",
                        help="do not ask GitHub whether each PR is merged")
    parser.add_argument("--merged-only", action="store_true",
                        help="with --last: keep sessions that opened a merged PR, "
                             "and divide by merged PRs only")
    args = parser.parse_args(argv)

    repo = parse_repo(args.repo)
    if repo is None:
        print("--repo must look like owner/name", file=sys.stderr)
        return 2
    projects_dir = args.projects_dir or default_projects_dir()
    if projects_dir is None or not projects_dir.is_dir():
        print(f"transcript folder not found: {projects_dir}", file=sys.stderr)
        return 2
    if args.merged_only and args.no_gh:
        print("--merged-only needs GitHub; drop --no-gh", file=sys.stderr)
        return 2

    sessions = load_sessions(projects_dir, repo)
    if args.since:
        sessions = {k: v for k, v in sessions.items()
                    if v.start is not None and v.start[:10] >= args.since}
    ordered = sorted(sessions.items(), key=lambda kv: kv[1].start or "")

    if args.last is None:
        for session_id, stats in ordered:
            print_session(session_id, stats)
        skipped = sum(s.skipped for s in sessions.values())
        first = next((s.start[:10] for _, s in ordered if s.start), "?")
        print(f"\n{len(ordered)} sessions since {first}, "
              f"{skipped} lines skipped (malformed or without usage)")
        return 0

    states = {}
    chosen = []
    # Newest first, so --merged-only asks GitHub about as few PRs as it can.
    for session_id, stats in reversed(ordered):
        if len(chosen) == args.last:
            break
        if not stats.prs:
            continue
        for number in stats.prs:
            states[number] = "?" if args.no_gh else merged_state(repo, number)
        if args.merged_only and not any(states[n] == "merged" for n in stats.prs):
            continue
        chosen.append((session_id, stats))
    chosen.reverse()

    kind = "a merged PR" if args.merged_only else "a PR"
    print(f"Latest {len(chosen)} sessions that opened {kind} in {repo[0]}/{repo[1]}\n")
    print("| session | date | PRs opened | merged | input | cache_write | cache_read | output |")
    print("|---|---|---|---|---|---|---|---|")
    totals = defaultdict(int)
    agents = defaultdict(lambda: defaultdict(int))
    pr_count = 0
    for session_id, stats in chosen:
        prs = " ".join(f"#{n} ({states[n]})" for n in sorted(stats.prs))
        merged = sum(1 for n in stats.prs if states[n] == "merged")
        cells = [fmt(stats.total(c)) for c in CATEGORIES]
        print(f"| {session_id[:8]} | {(stats.start or '?')[:10]} | {prs} | "
              f"{merged}/{len(stats.prs)} | " + " | ".join(cells) + " |")
        for c in CATEGORIES:
            totals[c] += stats.total(c)
        for agent, row in stats.by_agent.items():
            for c in CATEGORIES:
                agents[agent][c] += row[c]
        pr_count += merged if args.merged_only else len(stats.prs)
    if chosen:
        print("\nBy agent, over these sessions:\n")
        print("| agent | input | cache_write | cache_read | output |")
        print("|---|---|---|---|---|")
        for agent in sorted(agents, key=lambda a: -agents[a]["cache_read_input_tokens"]):
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
    skipped = sum(s.skipped for _, s in chosen)
    print(f"\n{skipped} lines skipped in these sessions (malformed or without usage)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
