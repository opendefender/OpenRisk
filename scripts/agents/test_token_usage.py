#!/usr/bin/env python3
"""Tests for token_usage.py (#1003).

    python3 -I scripts/agents/test_token_usage.py
"""

import contextlib
import io
import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent))
import token_usage  # noqa: E402

SENTINEL = "SENTINEL-do-not-print-7f3a"
SESSION = "11111111-aaaa-bbbb-cccc-000000000001"
REPO = "https://github.com/opendefender/OpenRisk/pull/"


def line(**entry):
    return json.dumps(entry) + "\n"


def assistant(msg_id, usage, blocks, model="claude-opus-5-5", ts="2026-10-10T10:00:00Z"):
    message = {"id": msg_id, "model": model, "content": blocks}
    if usage is not None:
        message["usage"] = usage
    return line(type="assistant", timestamp=ts, message=message)


def bash_call(tool_id, command):
    return {"type": "tool_use", "id": tool_id, "name": "Bash", "input": {"command": command}}


def result(tool_id, text, ts="2026-10-10T10:00:05Z"):
    return line(type="user", timestamp=ts, message={"role": "user", "content": [
        {"type": "tool_result", "tool_use_id": tool_id, "content": text}]})


def call(tool_id, command, text):
    """One assistant reply running `command`, and its tool result `text`."""
    return assistant(f"msg_{tool_id}", USAGE, [bash_call(tool_id, command)]) + result(tool_id, text)


USAGE = {"input_tokens": 10, "cache_creation_input_tokens": 100,
         "cache_read_input_tokens": 1000, "output_tokens": 5}


class TokenUsageTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.dir = Path(self.tmp.name)
        main = self.dir / f"{SESSION}.jsonl"
        main.write_text("".join([
            line(type="user", timestamp="2026-10-10T09:59:00Z" + SENTINEL,
                 message={"role": "user", "content": f"please do it {SENTINEL}"}),
            # One reply written as two lines (thinking + tool_use): counted once.
            assistant("msg_1", USAGE, [{"type": "thinking", "thinking": SENTINEL}]),
            assistant("msg_1", USAGE, [bash_call("t1", f"gh pr create --title '{SENTINEL}'")]),
            result("t1", f"{REPO}901\n{SENTINEL}"),
            # Openings: a REST POST with -X, and one where -F alone makes it a POST.
            call("t2", "gh api -X POST repos/opendefender/OpenRisk/pulls --jq .html_url",
                 [{"type": "text", "text": f"{REPO}902"}]),
            call("t3", "cd ../wt && gh api repos/opendefender/OpenRisk/pulls -f title=x "
                       "-F body=@pr.md --jq .html_url", f"{REPO}903"),
            # Openings from a shell function, a loop and an env-prefixed call.
            call("t9", 'mk(){ gh pr create --base "$1" --title "$2" 2>&1 | tail -1; }; mk master x',
                 f"{REPO}904"),
            call("t10", "for b in a c; do gh pr create --head $b; done", f"{REPO}905"),
            call("t11", "GH_PROMPT_DISABLED=1 gh pr create --fill", f"{REPO}906"),
            # Not openings.
            call("t12", 'echo "run gh pr create later"', f"{REPO}780"),
            call("t4", "gh pr view 777", f"{REPO}777"),
            call("t5", "rg 'gh pr create' docs/", f"{REPO}778"),
            call("t6", "gh api -X POST repos/opendefender/OpenRisk/pulls/5/reviews -F event=COMMENT",
                 f"{REPO}5#pullrequestreview-1"),
            call("t7", "gh api repos/opendefender/OpenRisk/pulls --method GET -f state=open",
                 f"{REPO}779"),
            call("t8", "gh pr create --repo opendefender/opendefender-website",
                 "https://github.com/opendefender/opendefender-website/pull/3"),
            "{this is not json\n",
            assistant("msg_nousage", None, []),
        ]))
        sub = self.dir / SESSION / "subagents"
        sub.mkdir(parents=True)
        # A subagent reply whose output grows from line to line: the highest counts.
        (sub / "agent-a1.jsonl").write_text(
            assistant("msg_sub", dict(USAGE, output_tokens=8),
                      [{"type": "text", "text": SENTINEL}], model="claude-sonnet-5-5")
            + assistant("msg_sub", dict(USAGE, output_tokens=120),
                        [{"type": "tool_use", "id": "s1", "input": {"command": SENTINEL}}],
                        model="claude-sonnet-5-5")
            + assistant("msg_sub", dict(USAGE, output_tokens=60), [], model="claude-sonnet-5-5"))
        (sub / "agent-a1.meta.json").write_text(json.dumps({
            "agentType": "backend-go-12", "customAgentType": "backend-go",
            "name": SENTINEL, "description": SENTINEL}))

    def tearDown(self):
        self.tmp.cleanup()

    def run_main(self, *args):
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            code = token_usage.main(["--projects-dir", str(self.dir), *args])
        return code, out.getvalue(), err.getvalue()

    def with_states(self, states):
        real = token_usage.merged_state
        token_usage.merged_state = lambda repo, number: states.get(number, "?")
        self.addCleanup(setattr, token_usage, "merged_state", real)

    def stats(self):
        return token_usage.load_sessions(self.dir)[SESSION]

    def test_TokenUsage_Success(self):
        stats = self.stats()
        # msg_1 once, t2..t12 (11 replies), and the subagent's reply: 13 replies.
        self.assertEqual(stats.total("input_tokens"), 130)
        self.assertEqual(stats.total("cache_read_input_tokens"), 13000)
        self.assertEqual(stats.by_agent["main"]["input_tokens"], 120)
        self.assertEqual(stats.by_agent["backend-go"]["input_tokens"], 10)

    def test_TokenUsage_KeepsHighestSubagentOutput(self):
        self.assertEqual(self.stats().tokens["claude-sonnet-5-5"]["output_tokens"], 120)

    def test_TokenUsage_CountsOnlyRealOpenings(self):
        self.assertEqual(self.stats().prs, {901, 902, 903, 904, 905, 906})

    def test_TokenUsage_SkipsMalformedLines(self):
        # The broken JSON line and the reply without usage.
        self.assertEqual(self.stats().skipped, 2)
        code, out, _ = self.run_main("--no-gh")
        self.assertEqual(code, 0)
        self.assertIn("2 lines skipped", out)

    def test_TokenUsage_NeverPrintsContent(self):
        self.with_states({901: "merged", 902: "open", 903: "closed"})
        for args in (("--no-gh",), ("--no-gh", "--last", "5"), ("--no-gh", "--since", "2026-10-01"),
                     ("--last", "5", "--merged-only")):
            _, out, err = self.run_main(*args)
            for stream in (out, err):
                self.assertNotIn(SENTINEL, stream, f"content leaked with {args}")
                self.assertNotIn("gh pr create", stream)

    def test_TokenUsage_LastMarksEachPR(self):
        self.with_states({901: "merged", 902: "open", 903: "closed"})
        _, out, _ = self.run_main("--last", "5")
        self.assertIn("#901 (merged) #902 (open) #903 (closed) #904 (?)", out)
        self.assertIn("| 1/6 |", out)
        # 130 input tokens over 6 PRs opened.
        self.assertIn("| 6 | 21 | 216 | 2 166 |", out)
        self.assertIn("output tokens are a floor", out)

    def test_TokenUsage_MergedOnlyDividesByMergedPRs(self):
        self.with_states({901: "merged", 902: "open", 903: "closed"})
        code, out, _ = self.run_main("--last", "5", "--merged-only")
        self.assertEqual(code, 0)
        self.assertIn("#901 (merged) #902 (open) #903 (closed)", out)
        # 130 input tokens over the one merged PR.
        self.assertIn("| 1 | 130 | 1 300 | 13 000 |", out)

    def test_TokenUsage_MergedOnlySkipsSessionsWithoutMergedPR(self):
        self.with_states({901: "open", 902: "open", 903: "closed"})
        _, out, _ = self.run_main("--last", "5", "--merged-only")
        self.assertIn("Latest 0 sessions", out)

    def test_TokenUsage_Unauthorized(self):
        # --merged-only needs GitHub; refusing beats a silent wrong denominator.
        code, _, err = self.run_main("--no-gh", "--last", "5", "--merged-only")
        self.assertEqual(code, 2)
        self.assertIn("needs GitHub", err)

    def test_TokenUsage_RejectsBadArguments(self):
        for args in (("--last", "0"), ("--last", "-1")):
            with self.assertRaises(SystemExit):
                self.run_main(*args)
        code, _, err = self.run_main("--repo", "../x")
        self.assertEqual(code, 2)
        self.assertIn("owner/name", err)

    def test_TokenUsage_SinceFilters(self):
        _, out, _ = self.run_main("--no-gh", "--since", "2026-10-11")
        self.assertIn("0 sessions", out)

    def test_TokenUsage_NotFound(self):
        err = io.StringIO()
        with contextlib.redirect_stderr(err):
            code = token_usage.main(["--projects-dir", str(self.dir / "missing")])
        self.assertEqual(code, 2)
        self.assertIn("not found", err.getvalue())


if __name__ == "__main__":
    unittest.main(verbosity=2)
