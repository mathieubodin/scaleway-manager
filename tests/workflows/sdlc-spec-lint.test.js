import assert from "node:assert/strict";
import { test } from "node:test";
import { runWorkflow } from "./harness.js";

const SCRIPT = ".claude/workflows/sdlc-spec-lint.js";
const Q = ["Says how it is verified?", "One reading only?"];
const ARGS = { specPath: "SPEC.md", questions: Q };

// Scripted answers: criteria list, then votes keyed by "<id> <question> <vote>".
const respond =
  (criteria, vote) =>
  ({ opts }) => {
    if (opts.label === "list criteria") return { criteria };
    const [id, q, v] = opts.label.split(" ");
    return vote(id, q, v);
  };

test("missing args are refused", async () => {
  await assert.rejects(runWorkflow(SCRIPT, { args: {} }), /required/);
  await assert.rejects(
    runWorkflow(SCRIPT, { args: { specPath: "SPEC.md", questions: [] } }),
  );
  await assert.rejects(runWorkflow(SCRIPT), /required/);
});

test("no criterion: reported, no vote", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: ARGS,
    respond: respond([], () => ({ yes: true, reason: "" })),
  });
  assert.deepEqual(run.result, {
    criteria: 0,
    failing: [],
    results: [],
    noCriteria: true,
  });
  assert.equal(run.calls.length, 1);
});

test("criteria list agent died: same as no criterion", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: ARGS,
    respond: () => {
      throw new Error("dead");
    },
  });
  assert.equal(run.result.noCriteria, true);
});

test("3 votes per question per criterion; 2 yes out of 3 pass", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: ARGS,
    respond: respond(
      [
        { id: "AC1", text: "Export keeps project-id, checked by a test." },
        { id: "AC2", text: "Export is fast." },
      ],
      (id, q, v) => ({
        yes:
          id === "AC1" ? !(q === "q2" && v === "v3") : q === "q2" && v === "v1",
        reason: "r",
      }),
    ),
  });
  assert.equal(run.calls.length, 1 + 2 * 2 * 3);
  assert.equal(run.result.criteria, 2);
  assert.deepEqual(
    run.result.failing.map((f) => f.id),
    ["AC2"],
  );
  const ac1 = run.result.results[0];
  assert.equal(ac1.pass, true);
  assert.deepEqual(
    ac1.answers.map((a) => a.yes),
    [3, 2],
  );
  const ac2 = run.result.failing[0];
  assert.deepEqual(
    ac2.answers.map((a) => [a.yes, a.pass]),
    [
      [0, false],
      [1, false],
    ],
  );
  assert.deepEqual(run.logs, ["1 of 2 criteria failing"]);
});

test("a dead voting agent counts as a missing yes", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: ARGS,
    respond: respond([{ id: "AC1", text: "t" }], (_id, q, v) => {
      if (q === "q1" && v !== "v1") throw new Error("dead");
      return { yes: true, reason: "r" };
    }),
  });
  const [q1, q2] = run.result.results[0].answers;
  assert.equal(q1.yes, 1);
  assert.equal(q1.pass, false);
  assert.deepEqual(q1.votes[1], { yes: null, reason: "agent died" });
  assert.equal(q2.pass, true);
  assert.deepEqual(
    run.result.failing.map((f) => f.id),
    ["AC1"],
  );
});

const WORK = "work/issue-1-x";
const ARGS_WORK = { ...ARGS, workFolder: WORK };
const yesVote = () => ({ yes: true, reason: "r" });
// Answers the agent that reads define-notes.md with `read`, every other agent with `rest`.
const withRead = (read, rest) => (call) =>
  call.opts.label === "read acquired" ? read(call) : rest(call);
const byLabel = (run, label) => run.calls.find((c) => c.opts.label === label);

test("without workFolder, define-notes.md is not read", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: ARGS,
    respond: respond([{ id: "AC1", text: "t" }], yesVote),
  });
  assert.equal(byLabel(run, "read acquired"), undefined);
  assert.equal(run.calls.length, 1 + 2 * 3);
  assert.doesNotMatch(byLabel(run, "list criteria").prompt, /already acquired/);
});

test("each kind of agent runs on its pinned model and effort", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: ARGS_WORK,
    respond: withRead(
      () => ({ present: true, acquired: [] }),
      respond([{ id: "AC1", text: "t" }], yesVote),
    ),
  });
  const read = byLabel(run, "read acquired");
  assert.match(read.prompt, /work\/issue-1-x\/define-notes\.md/);
  assert.equal(read.opts.model, "claude-haiku-5-5");
  assert.equal(read.opts.effort, "medium");
  const list = byLabel(run, "list criteria");
  assert.equal(list.opts.model, "claude-sonnet-5-5");
  assert.equal(list.opts.effort, "high");
  const votes = run.calls.filter((c) => c.opts.phase === "Votes");
  assert.equal(votes.length, 2 * 3);
  for (const v of votes) {
    assert.equal(v.opts.model, "claude-opus-5-5");
    assert.equal(v.opts.effort, "high");
  }
});

test("acquired criteria are given to the agent that lists the criteria", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: ARGS_WORK,
    respond: withRead(
      () => ({
        present: true,
        acquired: [
          { id: "CAP-1", text: "T1" },
          { id: "CAP-3", text: "T3" },
        ],
      }),
      respond([{ id: "CAP-2", text: "T2" }], yesVote),
    ),
  });
  const prompt = byLabel(run, "list criteria").prompt;
  assert.match(prompt, /already acquired/);
  assert.match(prompt, /has not changed in meaning/);
  assert.match(prompt, /\n- CAP-1: T1\n- CAP-3: T3$/);
  assert.deepEqual(run.logs, [
    "acquired criteria, not listed again: CAP-1, CAP-3",
    "0 of 1 criteria failing",
  ]);
  assert.deepEqual(
    run.result.results.map((r) => r.id),
    ["CAP-2"],
  );
});

test("no acquired criterion: the list prompt carries no note", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: ARGS_WORK,
    respond: withRead(
      () => ({ present: false, acquired: [] }),
      respond([{ id: "AC1", text: "t" }], yesVote),
    ),
  });
  assert.doesNotMatch(byLabel(run, "list criteria").prompt, /already acquired/);
  assert.equal(run.logs[0], "acquired criteria, not listed again: none");
});

test("reading agent died: nothing is acquired, every criterion is listed", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: ARGS_WORK,
    respond: withRead(
      () => {
        throw new Error("dead");
      },
      respond([{ id: "AC1", text: "t" }], yesVote),
    ),
  });
  assert.doesNotMatch(byLabel(run, "list criteria").prompt, /already acquired/);
  assert.equal(run.logs[0], "acquired criteria, not listed again: none");
  assert.equal(run.result.criteria, 1);
});

test("every criterion acquired: a green round, no vote", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: ARGS_WORK,
    respond: withRead(
      () => ({ present: true, acquired: [{ id: "CAP-1", text: "T1" }] }),
      respond([], yesVote),
    ),
  });
  assert.deepEqual(run.result, {
    criteria: 0,
    failing: [],
    results: [],
    noCriteria: false,
  });
  assert.equal(run.calls.length, 2);
  assert.ok(run.logs.includes("every criterion is acquired"));
});

test("listing agent died with acquired criteria: reported as no criterion", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: ARGS_WORK,
    respond: withRead(
      () => ({ present: true, acquired: [{ id: "CAP-1", text: "T1" }] }),
      () => {
        throw new Error("dead");
      },
    ),
  });
  assert.equal(run.result.noCriteria, true);
  assert.ok(run.logs.includes("no acceptance criterion found"));
});
