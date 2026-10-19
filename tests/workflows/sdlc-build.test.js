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

const built = (n, more = {}) => ({
  ok: true,
  branch: `wt-${n}`,
  commit: "def5678",
  worktree: `/wt/${n}`,
  wrongBase: false,
  report: "done",
  ...more,
});
const ko = (report, more = {}) => ({
  ok: false,
  branch: "",
  commit: "",
  worktree: "/wt/x",
  wrongBase: false,
  report,
  ...more,
});

// Answers by label: "build <n>", "build <n> again", "check <n>", "check <n>
// again", "improve <n>". Checks and improvements succeed unless given; a
// function is called (it may throw to make the agent die).
const respond =
  (answers) =>
  ({ opts }) => {
    const a = answers[opts.label];
    if (typeof a === "function") return a();
    if (a !== undefined) return a;
    if (opts.label.startsWith("check")) return { ok: true, report: "ok" };
    if (opts.label.startsWith("improve")) {
      return { ticket: "better ticket", changes: "added paths" };
    }
    return undefined;
  };
const dead = () => {
  throw new Error("dead");
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

test("success at the first attempt, in a worktree, base checked, result checked", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: LONG,
    respond: respond({ "build 1": built(1), "build 2": built(2) }),
  });
  assert.deepEqual(run.result, {
    mode: "long",
    finished: [
      {
        item: STORIES[0],
        branch: "wt-1",
        commit: "def5678",
        attempts: 1,
        ticketImproved: false,
      },
      {
        item: STORIES[1],
        branch: "wt-2",
        commit: "def5678",
        attempts: 1,
        ticketImproved: false,
      },
    ],
    failed: [],
    backToPlan: false,
  });
  const builds = run.calls.filter((c) => c.opts.phase === "Build");
  for (const { prompt, opts } of builds) {
    assert.equal(opts.isolation, "worktree");
    assert.ok(prompt.startsWith("Build it.\n\n"));
    assert.match(prompt, /if it does not start with abc1234, change nothing/);
    assert.match(prompt, /make check-frozen/);
  }
  assert.ok(builds[0].prompt.endsWith(`Story spec: ${STORIES[0]}`));
  const check = run.calls.find((c) => c.opts.label === "check 1");
  assert.equal(check.opts.isolation, undefined);
  assert.match(check.prompt, /branch "wt-1", commit def5678/);
  assert.match(check.prompt, /git merge-base --is-ancestor abc1234 def5678/);
  assert.match(check.prompt, /frozen-tests\.sha256/);
});

test("one failure, ticket improved, second builder succeeds", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { ...LONG, items: [STORIES[0]] },
    respond: respond({
      "build 1": ko("no file named"),
      "build 1 again": built(1),
    }),
  });
  assert.deepEqual(run.result.finished, [
    {
      item: STORIES[0],
      branch: "wt-1",
      commit: "def5678",
      attempts: 2,
      ticketImproved: true,
    },
  ]);
  const improve = run.calls.find((c) => c.opts.label === "improve 1");
  assert.equal(improve.opts.isolation, undefined);
  assert.match(
    improve.prompt,
    /story spec stories\/1-a\.md\. Its report: no file named/,
  );
  assert.match(improve.prompt, /Do not change its acceptance criteria/);
  assert.match(improve.prompt, /Do not commit\./);
  const again = run.calls.find((c) => c.opts.label === "build 1 again");
  assert.match(
    again.prompt,
    /supersedes the file named after it:\n\nbetter ticket/,
  );
});

test("long: two failures send the story back to Plan, worktrees named", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { ...LONG, items: [STORIES[0]] },
    respond: respond({ "build 1": ko("first"), "build 1 again": dead }),
  });
  assert.deepEqual(run.result, {
    mode: "long",
    finished: [],
    failed: [
      {
        item: STORIES[0],
        outcome: "back to Plan",
        reasons: ["first", "builder died"],
        improvement: "added paths",
        worktrees: ["/wt/x"],
      },
    ],
    backToPlan: true,
  });
});

test("a commit the checker rejects, or a dead checker, is a failure", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { ...LONG, items: [STORIES[0]] },
    respond: respond({
      "build 1": built(1),
      "check 1": { ok: false, report: "frozen test changed" },
      "build 1 again": built(1),
      "check 1 again": dead,
    }),
  });
  assert.deepEqual(run.result.failed[0].reasons, [
    "check failed: frozen test changed",
    "check failed: checker died",
  ]);
});

test("wrong base: no improvement, no second builder", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { ...LONG, items: [STORIES[0]] },
    respond: respond({ "build 1": ko("HEAD is 999", { wrongBase: true }) }),
  });
  assert.deepEqual(run.result.failed, [
    {
      item: STORIES[0],
      outcome: "wrong base",
      reasons: ["HEAD is 999"],
      improvement: "",
      worktrees: ["/wt/x"],
    },
  ]);
  assert.equal(run.result.backToPlan, false);
  assert.equal(run.calls.length, 1);
});

test("wrong base at the second attempt is reported as such", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { ...LONG, items: [STORIES[0]] },
    respond: respond({
      "build 1": ko("red"),
      "build 1 again": ko("moved", { wrongBase: true }),
    }),
  });
  assert.equal(run.result.failed[0].outcome, "wrong base");
  assert.equal(run.result.backToPlan, false);
});

test("bug: two failures ask the human; a dead improver keeps the file as the ticket", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: BUG,
    respond: respond({
      // ok without a commit SHA is not a success
      "build 1": built(1, { commit: "" }),
      "build 1 again": ko("still red"),
      "improve 1": dead,
    }),
  });
  assert.deepEqual(run.result.failed, [
    {
      item: "work/bug-report.md",
      outcome: "ask the human",
      reasons: ["incomplete report: done", "still red"],
      improvement: "improver died",
      worktrees: ["/wt/1", "/wt/x"],
    },
  ]);
  const builds = run.calls.filter((c) => c.opts.phase === "Build");
  assert.ok(builds[0].prompt.endsWith("Bug report: work/bug-report.md"));
  assert.match(
    builds[1].prompt,
    /failed on this ticket with: incomplete report: done\. The ticket file stays the reference\./,
  );
  assert.doesNotMatch(builds[1].prompt, /supersedes/);
});

test("bug: the improver adds a Builder attempts section", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: BUG,
    respond: respond({ "build 1": ko("x"), "build 1 again": built(1) }),
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
      if (opts.label.startsWith("check")) return { ok: true, report: "ok" };
      active++;
      peak = Math.max(peak, active);
      await new Promise((r) => setTimeout(r, 5));
      active--;
      return built(opts.label.split(" ")[1]);
    },
  });
  assert.equal(peak, 3);
  assert.equal(run.result.finished.length, 5);
});
