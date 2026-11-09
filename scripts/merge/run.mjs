// Judges a list of PRs one by one (#1330). A PR whose judgement throws is printed as an
// error and the others are still judged, so one transient failure never holds the queue.
// Returns how many PRs errored.
export async function judgeAll(numbers, judge, log = console.log) {
  let errored = 0;
  for (const n of numbers) {
    let o;
    try {
      o = await judge(n);
    } catch (e) {
      errored += 1;
      log(`#${n}: error — ${e.message}`);
      continue;
    }
    log(`#${o.pr}: ${o.result}`);
    for (const r of o.reasons ?? []) log(`  - ${r}`);
  }
  return errored;
}
