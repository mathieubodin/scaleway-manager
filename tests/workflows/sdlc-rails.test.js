import assert from "node:assert/strict";
import { test } from "node:test";
import { runWorkflow } from "./harness.js";

const SCRIPT = ".claude/workflows/sdlc-rails.js";
const STORIES = ["stories/1-api.md", "stories/2-button.md"];
const LONG = { mode: "long", items: STORIES, testWriterPrompt: "Write tests." };
const BUG = {
  mode: "bug",
  items: ["work/bug-report.md"],
  testWriterPrompt: "Reproduce.",
};
const t = (file) => ({ file, covers: "AC1" });

// Scripted answers by label: "write <n>", "write <n> again", "run first",
// "run second", "freeze". `writes[label]` gives the tests of a writer (a
// function throws to make it die); `passes` lists files that pass when run.
const respond =
  ({
    writes = {},
    passes = [],
    freeze = { ok: true, output: "ok" },
    runs = {},
  }) =>
  ({ prompt, opts }) => {
    if (opts.label.startsWith("write")) {
      const w = writes[opts.label] ?? [];
      return typeof w === "function" ? w() : { tests: w };
    }
    if (opts.label.startsWith("run")) {
      if (runs[opts.label]) return runs[opts.label]();
      const files = prompt.split("\n").slice(1);
      return {
        results: files.map((file) => ({
          file,
          failed: !passes.includes(file),
          message: passes.includes(file) ? "passed" : `${file} fails`,
        })),
      };
    }
    return typeof freeze === "function" ? freeze() : freeze;
  };

test("invalid args are refused", async () => {
  const bad = [
    [{}, /mode/],
    [{ ...LONG, mode: "short" }, /mode/],
    [{ ...LONG, items: [] }, /items/],
    [{ ...LONG, items: "stories/1-api.md" }, /items/],
    [{ ...LONG, items: [STORIES[0], STORIES[0]] }, /twice/],
    [{ ...BUG, items: ["a.md", "b.md"] }, /one item/],
    [{ ...LONG, testWriterPrompt: "" }, /testWriterPrompt/],
  ];
  for (const [args, message] of bad) {
    await assert.rejects(runWorkflow(SCRIPT, { args }), message);
  }
  await assert.rejects(runWorkflow(SCRIPT), /mode/);
});

test("long: one writer per story, all tests fail, one freeze", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: LONG,
    respond: respond({
      writes: { "write 1": [t("tests/a.sh")], "write 2": [t("tests/b.sh")] },
    }),
  });
  assert.deepEqual(run.result, {
    mode: "long",
    frozen: [
      {
        item: STORIES[0],
        file: "tests/a.sh",
        covers: "AC1",
        message: "tests/a.sh fails",
      },
      {
        item: STORIES[1],
        file: "tests/b.sh",
        covers: "AC1",
        message: "tests/b.sh fails",
      },
    ],
    suspect: [],
    failed: [],
  });
  const writers = run.calls.filter((c) => c.opts.phase === "Write");
  assert.equal(writers.length, 2);
  assert.ok(writers[0].prompt.startsWith("Write tests.\n\n"));
  assert.ok(writers[0].prompt.endsWith(`Story spec: ${STORIES[0]}`));
  assert.doesNotMatch(writers[0].prompt, /2-button/);
  assert.match(
    run.calls.at(-1).prompt,
    /make freeze FILES='tests\/a\.sh tests\/b\.sh'/,
  );
});

test("long: a passing test goes back once; rewritten and failing, it is frozen", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: LONG,
    respond: respond({
      writes: {
        "write 1": [t("tests/a.sh"), t("tests/a2.sh")],
        "write 2": [t("tests/b.sh")],
        "write 1 again": [t("tests/a3.sh")],
      },
      passes: ["tests/a2.sh"],
    }),
  });
  assert.deepEqual(
    run.result.frozen.map((x) => x.file),
    ["tests/a.sh", "tests/a3.sh", "tests/b.sh"],
  );
  assert.deepEqual(run.result.suspect, []);
  const again = run.calls.find((c) => c.opts.label === "write 1 again");
  assert.match(again.prompt, /Your earlier tests tests\/a2\.sh already pass/);
  assert.ok(again.prompt.endsWith(`Story spec: ${STORIES[0]}`));
  const second = run.calls.find((c) => c.opts.label === "run second");
  assert.equal(second.prompt.split("\n").slice(1).join(","), "tests/a3.sh");
});

test("long: still passing after the rewrite, the test is suspect, not frozen", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: LONG,
    respond: respond({
      writes: {
        "write 1": [t("tests/a.sh")],
        "write 2": [t("tests/b.sh")],
        // Same file rewritten in place, plus a file owned by story 2 and a
        // file name with a space: both ignored.
        "write 1 again": [t("tests/a.sh"), t("tests/b.sh"), t("tests/a b.sh")],
      },
      passes: ["tests/a.sh"],
    }),
  });
  assert.deepEqual(
    run.result.suspect.map((x) => [x.file, x.message]),
    [["tests/a.sh", "passed"]],
  );
  assert.deepEqual(
    run.result.frozen.map((x) => x.file),
    ["tests/b.sh"],
  );
});

test("long: a dead rewriter leaves the passing tests suspect", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: LONG,
    respond: respond({
      writes: {
        "write 1": [t("tests/a.sh")],
        "write 2": [t("tests/b.sh")],
        "write 1 again": () => {
          throw new Error("dead");
        },
      },
      passes: ["tests/a.sh"],
    }),
  });
  assert.deepEqual(
    run.result.suspect.map((x) => x.file),
    ["tests/a.sh"],
  );
  assert.equal(
    run.calls.some((c) => c.opts.label === "run second"),
    false,
  );
});

test("long: dead, empty or badly named writers fail their item", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { ...LONG, items: [...STORIES, "stories/3-x.md"] },
    respond: respond({
      writes: {
        "write 1": () => {
          throw new Error("dead");
        },
        "write 2": [],
        "write 3": [t("tests/c d.sh")],
      },
    }),
  });
  assert.deepEqual(run.result, {
    mode: "long",
    frozen: [],
    suspect: [],
    failed: [
      { item: STORIES[0], reason: "test writer died" },
      { item: STORIES[1], reason: "no test written" },
      { item: "stories/3-x.md", reason: "unsupported file name: tests/c d.sh" },
    ],
  });
  assert.equal(run.calls.length, 3);
});

test("long: one test file written for two stories stops the run", async () => {
  await assert.rejects(
    runWorkflow(SCRIPT, {
      args: LONG,
      respond: respond({
        writes: { "write 1": [t("tests/x.sh")], "write 2": [t("tests/x.sh")] },
      }),
    }),
    /tests\/x\.sh written for both stories\/1-api\.md and stories\/2-button\.md/,
  );
});

test("long: an unreported file counts as passing", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { ...LONG, items: [STORIES[0]] },
    respond: respond({
      writes: {
        "write 1": [t("tests/a.sh")],
        "write 1 again": [t("tests/a.sh")],
      },
      runs: {
        "run first": () => ({ results: [] }),
        "run second": () => ({ results: [] }),
      },
    }),
  });
  assert.deepEqual(
    run.result.suspect.map((x) => x.message),
    ["not reported"],
  );
});

test("a dead test runner or a failed freeze stops the run", async () => {
  const args = { ...LONG, items: [STORIES[0]] };
  const writes = { "write 1": [t("tests/a.sh")] };
  await assert.rejects(
    runWorkflow(SCRIPT, {
      args,
      respond: respond({
        writes,
        runs: {
          "run first": () => {
            throw new Error("dead");
          },
        },
      }),
    }),
    /the agent running the tests \(first\) died/,
  );
  await assert.rejects(
    runWorkflow(SCRIPT, {
      args,
      respond: respond({
        writes,
        freeze: { ok: false, output: "no such file" },
      }),
    }),
    /make freeze failed: no such file/,
  );
  await assert.rejects(
    runWorkflow(SCRIPT, {
      args,
      respond: respond({
        writes,
        freeze: () => {
          throw new Error("dead");
        },
      }),
    }),
    /make freeze failed: agent died/,
  );
});

test("bug: one writer, tests run with their messages, nothing frozen", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: BUG,
    respond: respond({
      writes: { "write 1": [t("tests/bug.sh"), t("tests/bug2.sh")] },
      passes: ["tests/bug2.sh"],
    }),
  });
  assert.deepEqual(run.result, {
    mode: "bug",
    tests: [
      {
        file: "tests/bug.sh",
        covers: "AC1",
        failed: true,
        message: "tests/bug.sh fails",
      },
      {
        file: "tests/bug2.sh",
        covers: "AC1",
        failed: false,
        message: "passed",
      },
    ],
    failed: [],
  });
  assert.ok(run.calls[0].prompt.endsWith("Bug report: work/bug-report.md"));
  assert.equal(
    run.calls.some((c) => c.opts.phase === "Freeze"),
    false,
  );
});

test("bug: a dead writer returns no test", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: BUG,
    respond: respond({
      writes: {
        "write 1": () => {
          throw new Error("dead");
        },
      },
    }),
  });
  assert.deepEqual(run.result, {
    mode: "bug",
    tests: [],
    failed: [{ item: "work/bug-report.md", reason: "test writer died" }],
  });
});
