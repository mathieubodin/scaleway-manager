import assert from "node:assert/strict";
import { test } from "node:test";
import { runWorkflow } from "./harness.js";

const SCRIPT = ".claude/workflows/sdlc-build.js";
const SHA = "abc1234";
const STORIES = ["stories/1-a.md", "stories/2-b.md"];
const LONG = {
  mode: "long",
  items: STORIES,
  baseSha: SHA,
  builderPrompt: "Build it.",
};
const BUG = { ...LONG, mode: "bug", items: ["work/bug-report.md"] };

const ok = (n) => ({
  ok: true,
  branch: `wt-${n}`,
  commit: "def5678",
  report: "done",
});
const ko = (report) => ({ ok: false, branch: "", commit: "", report });

// Answers by label: "build <n>", "build <n> again", "improve <n>"; a function
// is called (it may throw to make the agent die).
const respond =
  (answers) =>
  ({ opts }) => {
    const a = answers[opts.label];
    if (typeof a === "function") return a();
    if (a === undefined && opts.label.startsWith("improve")) {
      return { ticket: "better ticket", changes: "added paths" };
    }
    return a;
  };

test("invalid args are refused", async () => {
  const bad = [
    [{}, /mode/],
    [{ ...LONG, mode: "short" }, /mode/],
    [{ ...LONG, items: [] }, /items/],
    [{ ...LONG, items: "stories/1-a.md" }, /items/],
    [{ ...LONG, items: [STORIES[0], STORIES[0]] }, /twice/],
    [{ ...BUG, items: ["a.md", "b.md"] }, /one item/],
    [{ ...LONG, baseSha: "main" }, /baseSha/],
    [{ ...LONG, baseSha: undefined }, /baseSha/],
    [{ ...LONG, builderPrompt: "" }, /builderPrompt/],
  ];
  for (const [args, message] of bad) {
    await assert.rejects(runWorkflow(SCRIPT, { args }), message);
  }
  await assert.rejects(runWorkflow(SCRIPT), /mode/);
});

test("success at the first attempt, in a worktree, base checked", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: LONG,
    respond: respond({ "build 1": ok(1), "build 2": ok(2) }),
  });
  assert.deepEqual(run.result, {
    mode: "long",
    finished: [
      { item: STORIES[0], branch: "wt-1", commit: "def5678", attempts: 1 },
      { item: STORIES[1], branch: "wt-2", commit: "def5678", attempts: 1 },
    ],
    failed: [],
  });
  for (const { prompt, opts } of run.calls) {
    assert.equal(opts.isolation, "worktree");
    assert.ok(prompt.startsWith("Build it.\n\n"));
    assert.match(prompt, /if it does not start with abc1234, change nothing/);
    assert.match(prompt, /make check-frozen/);
  }
  assert.ok(run.calls[0].prompt.endsWith(`Story spec: ${STORIES[0]}`));
});

test("one failure, ticket improved, second builder succeeds", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { ...LONG, items: [STORIES[0]] },
    respond: respond({
      "build 1": ko("no file named"),
      "build 1 again": ok(1),
    }),
  });
  assert.deepEqual(run.result.finished, [
    { item: STORIES[0], branch: "wt-1", commit: "def5678", attempts: 2 },
  ]);
  const improve = run.calls.find((c) => c.opts.label === "improve 1");
  assert.equal(improve.opts.isolation, undefined);
  assert.match(
    improve.prompt,
    /story spec stories\/1-a\.md\. Its report: no file named/,
  );
  assert.match(improve.prompt, /Do not change its acceptance criteria/);
  const again = run.calls.find((c) => c.opts.label === "build 1 again");
  assert.match(
    again.prompt,
    /supersedes the file named after it:\n\nbetter ticket/,
  );
});

test("long: two failures send the story back to Plan", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { ...LONG, items: [STORIES[0]] },
    respond: respond({
      "build 1": ko("first"),
      "build 1 again": () => {
        throw new Error("dead");
      },
    }),
  });
  assert.deepEqual(run.result, {
    mode: "long",
    finished: [],
    failed: [
      {
        item: STORIES[0],
        reasons: ["first", "builder died"],
        improvement: "added paths",
        outcome: "back to Plan",
      },
    ],
  });
});

test("bug: two failures ask the human; the bug report is enriched", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: BUG,
    respond: respond({
      // ok without a commit SHA is not a success
      "build 1": { ok: true, branch: "wt", commit: "", report: "said ok" },
      "build 1 again": ko("still red"),
      "improve 1": () => {
        throw new Error("dead");
      },
    }),
  });
  assert.deepEqual(run.result.failed, [
    {
      item: "work/bug-report.md",
      reasons: ["incomplete report: said ok", "still red"],
      improvement: "improver died",
      outcome: "ask the human",
    },
  ]);
  assert.ok(run.calls[0].prompt.endsWith("Bug report: work/bug-report.md"));
  const again = run.calls.find((c) => c.opts.label === "build 1 again");
  assert.match(
    again.prompt,
    /\(the ticket could not be improved\) incomplete report/,
  );
});

test("bug: the improver adds a Builder attempts section", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: BUG,
    respond: respond({ "build 1": ko("x"), "build 1 again": ok(1) }),
  });
  const improve = run.calls.find((c) => c.opts.label === "improve 1");
  assert.match(improve.prompt, /under a section "Builder attempts"/);
  assert.equal(run.result.finished[0].attempts, 2);
});

test("at most 3 builders at the same time", async () => {
  const items = ["s/1.md", "s/2.md", "s/3.md", "s/4.md", "s/5.md"];
  let active = 0;
  let peak = 0;
  const run = await runWorkflow(SCRIPT, {
    args: { ...LONG, items },
    respond: async ({ opts }) => {
      active++;
      peak = Math.max(peak, active);
      await new Promise((r) => setTimeout(r, 5));
      active--;
      return ok(opts.label.split(" ")[1]);
    },
  });
  assert.equal(peak, 3);
  assert.equal(run.result.finished.length, 5);
});
