# Agent token baseline, before the agent stack v2

Issue #1003, part of epic #1002. Measured on 2026-10-15 on the owner's machine,
before any v2 change landed. The same command will be run again on five issues
worked under v2, and the comparison posted on #1002.

## Command

```
python3 -I scripts/agents/token_usage.py --last 5 --merged-only
```

The script reads the local Claude Code transcripts of this project
(`~/.claude/projects/<project>/`) and prints aggregates only: session ids, dates,
models, agent types, PR numbers and token counts. It never prints a prompt, a
command or a tool result. `scripts/agents/test_token_usage.py` checks that with a
sentinel string planted in prompts, commands, tool results, timestamps and the
subagent metadata.

## Result

The five latest sessions that opened at least one merged PR. All 16 PRs are
merged.

| session | date | PRs opened | input | cache_write | cache_read | output |
|---|---|---|---|---|---|---|
| d64a0c1b | 2026-10-14 | #963 #967 #968 #969 #970 #971 | 792 | 1 675 136 | 77 446 523 | 209 472 |
| d6f2fc97 | 2026-10-14 | #964 | 176 | 137 803 | 11 056 852 | 31 086 |
| ec8d15aa | 2026-10-14 | #976 #980 #981 #982 #983 | 772 | 876 154 | 70 534 554 | 198 534 |
| e8e9ecf4 | 2026-10-14 | #984 #987 | 104 | 131 688 | 5 009 571 | 43 715 |
| 684aa2d1 | 2026-10-14 | #993 #994 | 258 | 565 921 | 14 444 415 | 73 888 |

By agent, over these sessions:

| agent | input | cache_write | cache_read | output |
|---|---|---|---|---|
| main | 1 498 | 2 570 324 | 159 095 438 | 545 724 |
| tech-lead | 434 | 516 369 | 15 330 615 | 10 146 * |
| devsecops | 158 | 269 554 | 3 936 778 | 748 * |
| po-openrisk | 12 | 30 455 | 129 084 | 77 * |

\* a floor, not a count. See limit 1.

**Baseline, per merged PR (16 PRs):**

| input | cache_write | cache_read | output |
|---|---|---|---|
| 131 | 211 668 | 11 155 744 | 34 793 |

## What the numbers say

- **Cache reads dominate by volume.** Per merged PR, cache reads are about 320
  times the output and about 50 times the cache writes. Every turn of a
  session re-reads the whole context from cache, so the volume grows with
  session length times turn count.
- **The main session carries most of it.** It accounts for 89% of the cache
  reads (159 M of 178 M) and 76% of the cache writes (2.57 M of 3.39 M). The
  two reviewers, tech-lead and devsecops, account for about 11% of the cache
  reads.
- **Long sessions with several PRs cost the most.** The sessions that opened 5
  and 6 PRs read 70 to 77 M tokens from cache. `/work` says "one issue per
  session" (`.claude/skills/work/SKILL.md`), and these sessions did not follow
  it.
- **What this suggests for v2.** Keeping the orchestrating session short and
  delegating reads to subagents is likely to move the total more than the
  number of reviewers does. The v2 re-run will test that. This is a reading of
  five sessions, not a proven result.

The cost of each category in currency is out of scope: the repository has no
sourced price table, and the categories are billed at different rates.

## Limits

1. **Subagent output is undercounted.** Main-session transcripts write a
   reply's final usage on every line. Subagent transcripts write a growing
   usage: input and cache figures are the same on every line, but the output
   count grows and the last line written is not always the final count. The
   script keeps the highest value per reply, which is still a floor. v2 moves
   work into subagents (scout, qa, reviewers), so a comparison on output alone
   would flatter v2. **Compare on input, cache_write and cache_read, which are
   complete for every agent.** Read output for the main session only.
2. **Attribution is per session, not per issue.** A session's total is divided
   by the PRs it opened. Work that opened no PR (refinement, aborted attempts)
   is in no row.
3. **Coverage.** The oldest transcript on disk starts on 2026-08-14. Of the 245
   PRs GitHub lists as created since then, the script finds 130:
   - October: all 56;
   - September: 67 of 151;
   - August: 7 of 38.

   The missed ones were not traced one by one. Four are on `claude/…` branches,
   which come from sessions that did not run on this machine. For the rest, the
   likely cause is that their transcripts are no longer on disk. **Run the v2
   comparison before the v2 transcripts age out.** The baseline sessions are
   all from October, where coverage is complete.
4. **PR detection.** A PR counts as opened when a command that starts with
   `gh pr create`, or with a `gh api` POST to the pulls collection, returned
   its URL. A PR opened any other way is missed. A review or comment POST does
   not count.
5. **Skipped lines.** 1 line was skipped in the 5 baseline sessions, and 7 over
   all 77 sessions (malformed, or an assistant reply without usage).

## Second signal for v2

The desktop app reports the plan's usage windows (5-hour and weekly) in whole
percent. That is too coarse to rebuild history, but recording it before and
after each queue-runner issue (#1008) gives a figure that includes subagent
output.
