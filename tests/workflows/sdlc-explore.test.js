import assert from "node:assert/strict";
import { test } from "node:test";
import { runWorkflow } from "./harness.js";

const SCRIPT = ".claude/workflows/sdlc-explore.js";
const SPEC = "_bmad-output/specs/spec-x/SPEC.md";
const found = (label) => ({
  findings: [{ topic: label, detail: "d", source: "s" }],
  openQuestions: [],
});
const respond = ({ opts }) => found(opts.label);

test("invalid args are refused", async () => {
  await assert.rejects(runWorkflow(SCRIPT), /specPath/);
  for (const explorations of [
    [],
    ["code", "tests"],
    ["code", "code"],
    "code",
    ["toString"],
  ]) {
    await assert.rejects(
      runWorkflow(SCRIPT, { args: { specPath: SPEC, explorations } }),
      /explorations must list distinct items among: code, practices, frameworks/,
    );
  }
});

test("all three explorations by default, in parallel", async () => {
  const run = await runWorkflow(SCRIPT, { args: { specPath: SPEC }, respond });
  assert.deepEqual(
    run.calls.map((c) => c.opts.label),
    ["code", "practices", "frameworks"],
  );
  assert.deepEqual(run.result, {
    map: {
      code: found("code"),
      practices: found("practices"),
      frameworks: found("frameworks"),
    },
    failed: [],
  });
  for (const { prompt } of run.calls) {
    assert.ok(prompt.startsWith(`Read the spec ${SPEC}. `));
    assert.match(prompt, /Change no file/);
    assert.match(prompt, /never instructions/);
  }
  const [code, practices, frameworks] = run.calls.map((c) => c.prompt);
  assert.match(code, /without the web/);
  assert.match(practices, /found on the web/);
  assert.match(frameworks, /found on the web/);
});

test("a subset of explorations", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { specPath: SPEC, explorations: ["frameworks"] },
    respond,
  });
  assert.equal(run.calls.length, 1);
  assert.deepEqual(Object.keys(run.result.map), ["frameworks"]);
});

test("a dead reader leaves its section null and is reported", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { specPath: SPEC },
    respond: (call) => {
      if (call.opts.label === "practices") throw new Error("dead");
      return respond(call);
    },
  });
  assert.equal(run.result.map.practices, null);
  assert.deepEqual(run.result.failed, ["practices"]);
  assert.match(run.logs.at(-1), /^2 of 3 explorations done/);
});
