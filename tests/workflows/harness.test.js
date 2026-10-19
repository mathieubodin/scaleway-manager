import assert from "node:assert/strict";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { after, test } from "node:test";
import { runWorkflow } from "./harness.js";

const SAMPLE = "tests/workflows/fixtures/sample.js";
const META = 'export const meta = { name: "x", description: "x" };\n';

const dir = mkdtempSync(join(tmpdir(), "harness-"));
after(() => rmSync(dir, { recursive: true, force: true }));
const tmpScript = (name, body) => {
  const path = join(dir, `${name}.js`);
  writeFileSync(path, body);
  return path;
};

test("no question: logs and returns early", async () => {
  const run = await runWorkflow(SAMPLE, { args: {} });
  assert.deepEqual(run.result, { accepted: [], checked: [] });
  assert.deepEqual(run.logs, ["no question"]);
  assert.equal(run.calls.length, 0);
  assert.equal(run.meta.name, "sample");
});

test("no args at all", async () => {
  const run = await runWorkflow(SAMPLE);
  assert.deepEqual(run.result.accepted, []);
});

test("votes: majority accepts, a dead agent counts as a missing vote", async () => {
  const run = await runWorkflow(SAMPLE, {
    args: { questions: ["a", "b"] },
    respond: ({ prompt }) => {
      if (prompt.startsWith("check")) return "ok";
      if (prompt === "b (vote 0)") throw new Error("agent died");
      return { yes: prompt.startsWith("a") || prompt === "b (vote 1)" };
    },
  });
  assert.deepEqual(run.result, { accepted: ["a"], checked: ["a: ok"] });
  assert.deepEqual(run.phases, ["Vote", "Check"]);
  assert.equal(run.calls.length, 7);
  assert.deepEqual(
    run.calls.filter((c) => c.phase === "Check").map((c) => c.prompt),
    ["check a"],
  );
});

test("pipeline: a throwing stage drops the item to null", async () => {
  const run = await runWorkflow(SAMPLE, {
    args: { questions: ["a", "b"] },
    respond: ({ prompt }) => {
      if (prompt === "check b") return Promise.reject(new Error("boom"));
      return prompt.startsWith("check") ? "ok" : { yes: true };
    },
  });
  assert.deepEqual(run.result.checked, ["a: ok", null]);
});

test("agent returns null when the subagent dies", async () => {
  const path = tmpScript("dead", `${META}return await agent("p");\n`);
  const run = await runWorkflow(path, {
    respond: () => {
      throw new Error("dead");
    },
  });
  assert.equal(run.result, null);
});

test("budget and workflow hooks", async () => {
  const path = tmpScript(
    "hooks",
    `${META}return [budget.total, budget.spent(), budget.remaining(), await workflow("child", 1)];\n`,
  );
  const run = await runWorkflow(path, {
    budget: 100,
    workflow: (name, a) => `${name}:${a}`,
  });
  assert.deepEqual(run.result, [100, 0, 100, "child:1"]);
  const noBudget = tmpScript(
    "nobudget",
    `${META}return [budget.total, budget.remaining()];\n`,
  );
  assert.deepEqual((await runWorkflow(noBudget)).result, [null, Infinity]);
  const noFake = tmpScript("nofake", `${META}return await workflow("x");\n`);
  await assert.rejects(runWorkflow(noFake), /without a fake/);
});

test("Date.now, Math.random, Date() and argless new Date throw", async () => {
  for (const expr of ["Date.now()", "Math.random()", "new Date()", "Date()"]) {
    const path = tmpScript("forbid", `${META}return ${expr};\n`);
    await assert.rejects(
      runWorkflow(path),
      /not available in workflow scripts/,
    );
  }
  const path = tmpScript(
    "date",
    `${META}return [new Date(0).getTime(), Date.UTC(1970, 0, 1), new Date(0) instanceof Date];\n`,
  );
  assert.deepEqual((await runWorkflow(path)).result, [0, 0, true]);
});

test("a script that does not begin with meta is refused", async () => {
  await assert.rejects(
    runWorkflow(tmpScript("nometa", "return 1;\n")),
    /does not begin with/,
  );
  await assert.rejects(
    runWorkflow(tmpScript("latemeta", `const a = 1;\n${META}return a;\n`)),
    /does not begin with/,
  );
});

test("errors report the source line", async () => {
  const path = tmpScript("line", `${META}\nthrow new Error("here");\n`);
  await assert.rejects(runWorkflow(path), (err) => {
    assert.match(err.stack, new RegExp(`${path.replaceAll(".", "\\.")}:3:`));
    return true;
  });
});
