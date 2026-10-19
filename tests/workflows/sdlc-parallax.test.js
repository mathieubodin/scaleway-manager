import assert from "node:assert/strict";
import { test } from "node:test";
import { runWorkflow } from "./harness.js";

const SCRIPT = ".claude/workflows/sdlc-parallax.js";
const ARGS = { ideaPath: "_bmad-output/forge/x/forged-idea.md" };
const DIV = {
  topic: "file layout",
  positions: [
    { readers: [1, 3], position: "one file" },
    { readers: [2], position: "one file per resource" },
  ],
  question: "One YAML file, or one file per resource?",
  weight: "important",
  proposal: "One file.",
};
const DETAIL = { ...DIV, topic: "name", weight: "detail", proposal: "x.yaml" };

// Readers return "spec <n>" unless `dead` holds their number.
const respond =
  ({ dead = [], divergences = [DIV] } = {}) =>
  ({ opts }) => {
    if (opts.label === "divergences") return { divergences };
    const n = Number(opts.label.split(" ")[1]);
    if (dead.includes(n)) throw new Error("dead");
    return { spec: `spec ${n}` };
  };

test("missing idea path is refused", async () => {
  await assert.rejects(runWorkflow(SCRIPT, { args: {} }), /ideaPath/);
  await assert.rejects(runWorkflow(SCRIPT), /ideaPath/);
});

test("3 readers with the same prompt, then one comparison", async () => {
  const run = await runWorkflow(SCRIPT, { args: ARGS, respond: respond() });
  assert.deepEqual(run.result, {
    readers: 3,
    deadReaders: [],
    important: [DIV],
    details: [],
  });
  const readers = run.calls.filter((c) => c.opts.phase === "Readers");
  assert.equal(readers.length, 3);
  assert.equal(new Set(readers.map((c) => c.prompt)).size, 1);
  assert.match(readers[0].prompt, /forge\/x\/forged-idea\.md/);
  const compare = run.calls.at(-1).prompt;
  assert.match(compare, /--- Reader 1 ---\nspec 1/);
  assert.match(compare, /--- Reader 3 ---\nspec 3/);
});

test("no divergence: empty lists", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: ARGS,
    respond: respond({ divergences: [] }),
  });
  assert.deepEqual([run.result.important, run.result.details], [[], []]);
});

test("details apart from important points, order kept", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: ARGS,
    respond: respond({
      divergences: [DETAIL, DIV, { ...DETAIL, topic: "order" }],
    }),
  });
  assert.deepEqual(run.result.important, [DIV]);
  assert.deepEqual(
    run.result.details.map((d) => d.topic),
    ["name", "order"],
  );
  assert.match(run.calls.at(-1).prompt, /"important" when/);
});

test("one dead reader: the two others compared under their own numbers", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: ARGS,
    respond: respond({
      dead: [2],
      divergences: [
        {
          topic: "t",
          positions: [
            { readers: [1, 2], position: "a" },
            { readers: [3, 4, 0], position: "b" },
          ],
          question: "q?",
          weight: "important",
          proposal: "a",
        },
      ],
    }),
  });
  assert.deepEqual(run.result.deadReaders, [2]);
  assert.deepEqual(
    run.result.important[0].positions.map((p) => p.readers),
    [[1], [3]],
  );
  const compare = run.calls.at(-1).prompt;
  assert.match(compare, /^2 agents/);
  assert.match(compare, /--- Reader 3 ---\nspec 3/);
  assert.doesNotMatch(compare, /Reader 2/);
});

test("fewer than 2 specs: the run stops, nothing compared", async () => {
  for (const dead of [
    [1, 3],
    [1, 2, 3],
  ]) {
    await assert.rejects(
      runWorkflow(SCRIPT, { args: ARGS, respond: respond({ dead }) }),
      new RegExp(`only ${3 - dead.length} spec written`),
    );
  }
});

test("a dead comparison agent stops the run", async () => {
  await assert.rejects(
    runWorkflow(SCRIPT, {
      args: ARGS,
      respond: (call) => {
        if (call.opts.label === "divergences") throw new Error("dead");
        return respond()(call);
      },
    }),
    /comparing the specs died/,
  );
});
