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
