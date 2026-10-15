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
sentinel string.

## Result

The five latest sessions that opened at least one merged PR:

| session | date | PRs opened | merged | input | cache_write | cache_read | output |
|---|---|---|---|---|---|---|---|
| d64a0c1b | 2026-10-14 | #963 #967 #968 #969 #970 #971 | 6/6 | 792 | 1 675 136 | 77 446 523 | 209 472 |
| d6f2fc97 | 2026-10-14 | #964 | 1/1 | 176 | 137 803 | 11 056 852 | 31 086 |
| ec8d15aa | 2026-10-14 | #976 #980 #981 #982 #983 | 5/5 | 772 | 876 154 | 70 534 554 | 195 534 |
| e8e9ecf4 | 2026-10-14 | #984 #987 | 2/2 | 104 | 131 688 | 5 009 571 | 43 715 |
| 684aa2d1 | 2026-10-14 | #993 #994 | 2/2 | 258 | 565 921 | 14 444 415 | 73 699 |

By agent, over these sessions:

| agent | input | cache_write | cache_read | output |
|---|---|---|---|---|
| main | 1 498 | 2 570 324 | 159 095 438 | 545 724 |
| tech-lead | 434 | 516 369 | 15 330 615 | 6 957 * |
| devsecops | 158 | 269 554 | 3 936 778 | 748 * |
| po-openrisk | 12 | 30 455 | 129 084 | 77 * |

\* a floor, not a count. See the limits below.

**Baseline, per merged PR (16 PRs):**

| input | cache_write | cache_read | output |
|---|---|---|---|
| 131 | 211 668 | 11 155 744 | 34 594 |

## What the numbers say

- **Cache reads dominate by volume.** Per merged PR, cache reads are about 320
  times the output and 50 times the cache writes. Every turn of a session
  re-reads the whole context from cache, so volume grows with session length
  times turn count.
- **The main session carries most of it.** It accounts for 89% of the cache
  reads (159 M of 178 M) and 76% of the cache writes (2.57 M of 3.39 M). The
  reviewers, tech-lead and devsecops, account for about 11% of the cache reads.
- **Long sessions with several PRs cost the most.** The two sessions that
  opened 5 and 6 PRs read 70 to 77 M tokens from cache. CLAUDE.md and `/work`
  say "one issue per session", and these sessions did not follow it.
- **What this means for v2.** Keeping the orchestrating session short and
  delegating reads to subagents is likely to move the total more than the
  number of reviewers does. The v2 re-run will test that. It is a reading of
  five sessions, not a proven result.

The cost of each category in currency is out of scope: the repository has no
sourced price table, and the categories are billed at different rates.

## Limits, stated plainly

1. **Subagent output is undercounted.** Subagent transcripts keep the usage of
   the first streamed chunk of each reply: input and cache counts are complete,
   but output reads 8 or 16 whatever the reply's length. Main-session output is
   final. v2 moves work into subagents (scout, qa, reviewers), so a comparison
   on output alone would flatter v2. **Compare on input, cache_write and
   cache_read, which are complete for every agent**, and read output for the
   main session only.
2. **Attribution is per session, not per issue.** A session's total is divided
   by the PRs it opened. Work that opened no PR (refinement, aborted attempts)
   is in no row.
3. **Only transcripts still on disk count.** Transcripts start on 2026-08-20.
   PRs opened outside a Claude Code session do not appear.
4. **PR detection.** A PR counts as opened when a `gh pr create`, or a POST to
   `…/pulls`, returned its URL. A PR opened another way is missed.
5. **7 lines were skipped** (malformed, or an assistant message without usage).

## Second signal for v2

The desktop app reports the plan's usage windows (5-hour and weekly, in whole
percent). It is too coarse to rebuild history, but recording it before and
after each queue-runner issue (#1008) gives a real figure that includes
subagent output.
