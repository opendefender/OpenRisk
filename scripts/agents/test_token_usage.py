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

sys.path.insert(0, str(Path(__file__).resolve().parent))
import token_usage  # noqa: E402

SENTINEL = "SENTINEL-do-not-print-7f3a"
SESSION = "11111111-aaaa-bbbb-cccc-000000000001"


def line(**entry):
    return json.dumps(entry) + "\n"


def assistant(msg_id, usage, blocks, model="claude-opus-5-5", ts="2026-10-10T10:00:00Z"):
    return line(type="assistant", timestamp=ts,
                message={"id": msg_id, "model": model, "usage": usage, "content": blocks})


def bash_call(tool_id, command):
    return {"type": "tool_use", "id": tool_id, "name": "Bash", "input": {"command": command}}


def result(tool_id, text, ts="2026-10-10T10:00:05Z"):
    return line(type="user", timestamp=ts, message={"role": "user", "content": [
        {"type": "tool_result", "tool_use_id": tool_id, "content": text}]})


USAGE = {"input_tokens": 10, "cache_creation_input_tokens": 100,
         "cache_read_input_tokens": 1000, "output_tokens": 5}


class TokenUsageTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.dir = Path(self.tmp.name)
        main = self.dir / f"{SESSION}.jsonl"
        main.write_text("".join([
            line(type="user", timestamp="2026-10-10T09:59:00Z",
                 message={"role": "user", "content": f"please do it {SENTINEL}"}),
            # One message written as two lines (thinking + tool_use): counted once.
            assistant("msg_1", USAGE, [{"type": "thinking", "thinking": SENTINEL}]),
            assistant("msg_1", USAGE, [bash_call("t1", f"gh pr create --title '{SENTINEL}'")]),
            result("t1", f"https://github.com/opendefender/OpenRisk/pull/901\n{SENTINEL}"),
            # A URL from `gh pr view` is not an opening.
            assistant("msg_2", USAGE, [bash_call("t2", "gh pr view 777")]),
            result("t2", "https://github.com/opendefender/OpenRisk/pull/777"),
            # A REST creation is an opening.
            assistant("msg_3", USAGE, [bash_call(
                "t3", "gh api -X POST repos/opendefender/OpenRisk/pulls -F body=@pr.md "
                      "--jq .html_url")]),
            result("t3", [{"type": "text",
                           "text": "https://github.com/opendefender/OpenRisk/pull/902"}]),
            "{this is not json\n",
            line(type="assistant", timestamp="2026-10-10T10:01:00Z",
                 message={"id": "msg_4", "model": "claude-opus-5-5", "content": []}),
        ]))
        sub = self.dir / SESSION / "subagents"
        sub.mkdir(parents=True)
        (sub / "agent-a1.jsonl").write_text(assistant(
            "msg_sub", USAGE, [{"type": "text", "text": SENTINEL}], model="claude-sonnet-5-5"))
        (sub / "agent-a1.meta.json").write_text(json.dumps(
            {"agentType": "backend-go-12", "customAgentType": "backend-go"}))

    def tearDown(self):
        self.tmp.cleanup()

    def run_main(self, *args):
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            code = token_usage.main(["--projects-dir", str(self.dir), "--no-gh", *args])
        return code, out.getvalue()

    def test_TokenUsage_Success(self):
        stats = token_usage.load_sessions(self.dir)[SESSION]
        # msg_1 counted once, msg_2, msg_3 and the subagent's message: 4 messages.
        self.assertEqual(stats.total("input_tokens"), 40)
        self.assertEqual(stats.total("cache_read_input_tokens"), 4000)
        self.assertEqual(stats.tokens["claude-sonnet-5-5"]["output_tokens"], 5)
        self.assertEqual(stats.by_agent["backend-go"]["input_tokens"], 10)
        self.assertEqual(stats.by_agent["main"]["input_tokens"], 30)
        self.assertEqual({n for _, _, n in stats.prs}, {901, 902})

    def test_TokenUsage_SkipsMalformedLines(self):
        stats = token_usage.load_sessions(self.dir)[SESSION]
        # The broken JSON line and msg_4 without usage.
        self.assertEqual(stats.skipped, 2)
        code, out = self.run_main()
        self.assertEqual(code, 0)
        self.assertIn("2 lines skipped", out)

    def test_TokenUsage_NeverPrintsContent(self):
        for args in ((), ("--last", "5"), ("--since", "2026-10-01")):
            _, out = self.run_main(*args)
            self.assertNotIn(SENTINEL, out, f"content leaked with {args}")
            self.assertNotIn("gh pr create", out)

    def test_TokenUsage_LastSummarisesPerPR(self):
        _, out = self.run_main("--last", "5")
        self.assertIn("#901 #902", out)
        self.assertIn("output tokens are a floor", out)
        # 40 input tokens over 2 PRs.
        self.assertIn("| 2 | 20 | 200 | 2 000 | 10 |", out)

    def test_TokenUsage_MergedOnlyDividesByMergedPRs(self):
        real = token_usage.merged_state
        token_usage.merged_state = lambda pr: "merged" if pr[2] == 901 else "not merged"
        try:
            out = io.StringIO()
            with contextlib.redirect_stdout(out):
                code = token_usage.main(["--projects-dir", str(self.dir),
                                         "--last", "5", "--merged-only"])
        finally:
            token_usage.merged_state = real
        self.assertEqual(code, 0)
        self.assertIn("| 1/2 |", out.getvalue())
        # 40 input tokens over the one merged PR.
        self.assertIn("| 1 | 40 | 400 | 4 000 | 20 |", out.getvalue())

    def test_TokenUsage_Unauthorized(self):
        # --merged-only needs GitHub; refusing beats a silent wrong denominator.
        err = io.StringIO()
        with contextlib.redirect_stderr(err):
            code = token_usage.main(["--projects-dir", str(self.dir), "--no-gh",
                                     "--last", "5", "--merged-only"])
        self.assertEqual(code, 2)
        self.assertIn("needs GitHub", err.getvalue())

    def test_TokenUsage_SinceFilters(self):
        _, out = self.run_main("--since", "2026-10-11")
        self.assertIn("0 sessions", out)

    def test_TokenUsage_NotFound(self):
        err = io.StringIO()
        with contextlib.redirect_stderr(err):
            code = token_usage.main(["--projects-dir", str(self.dir / "missing")])
        self.assertEqual(code, 2)
        self.assertIn("not found", err.getvalue())


if __name__ == "__main__":
    unittest.main(verbosity=2)
