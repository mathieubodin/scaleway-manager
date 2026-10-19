export const meta = {
  name: "sdlc-rails",
  description:
    "Write the tests of each story before its code (long mode, then freeze them), or the tests that reproduce a bug (bug mode, run but not frozen)",
  whenToUse:
    "Launched by sdlc-work at Rails (long route) or after the bug tests are confirmed (bug route), with args { mode, items, testWriterPrompt }.",
  phases: [{ title: "Write" }, { title: "Check" }, { title: "Freeze" }],
};

// args:
// - mode: "long" (items: story spec paths) or "bug" (items: [bug-report.md]);
// - testWriterPrompt: sdlc_test_writer_prompt (long) or
//   sdlc_bug_test_writer_prompt (bug), resolved from the bmad-build override.
// Long mode, decided with the author on 2026-10-19: every new test is run
// before the freeze; a test that already passes goes back once to a new writer
// for its story; if it still passes, it is not frozen and is reported.
// Bug mode: the tests are run and returned with their failure messages; no
// freeze, the human first confirms the failure reason (bug G2).
const FILE = /^[\w./-]+$/;

const WRITTEN = {
  type: "object",
  properties: {
    tests: {
      type: "array",
      items: {
        type: "object",
        properties: { file: { type: "string" }, covers: { type: "string" } },
        required: ["file", "covers"],
      },
    },
  },
  required: ["tests"],
};
const RUN = {
  type: "object",
  properties: {
    results: {
      type: "array",
      items: {
        type: "object",
        properties: {
          file: { type: "string" },
          failed: { type: "boolean" },
          message: { type: "string" },
        },
        required: ["file", "failed", "message"],
      },
    },
  },
  required: ["results"],
};
const DONE = {
  type: "object",
  properties: { ok: { type: "boolean" }, output: { type: "string" } },
  required: ["ok", "output"],
};

const mode = args?.mode;
const items = args?.items;
const prompt = args?.testWriterPrompt;
if (mode !== "long" && mode !== "bug") {
  throw new Error('args.mode must be "long" or "bug"');
}
if (!Array.isArray(items) || items.length === 0) {
  throw new Error(
    "args.items is required: story spec paths, or the bug report",
  );
}
if (mode === "bug" && items.length !== 1) {
  throw new Error("bug mode takes one item: the bug report");
}
if (new Set(items).size < items.length) {
  throw new Error("args.items lists an item twice");
}
if (!prompt) throw new Error("args.testWriterPrompt is required");

const label = mode === "long" ? "Story spec" : "Bug report";
const write = (item, i, extra = "") =>
  agent(
    `${prompt.trim()}\n\nOther test writers may work at the same time on other items: write only new test files for this one, never edit an existing file, and use only file names made of letters, digits, . _ / -.${extra}\n\n${label}: ${item}`,
    {
      schema: WRITTEN,
      phase: "Write",
      label: `write ${i + 1}${extra ? " again" : ""}`,
    },
  );
const runTests = async (files, round) => {
  const r = await agent(
    `From the repository root, run each of these new test files on its own, with the project's test commands, and change no file. For each, report whether it failed and its failure message (or "passed").\n${files.join("\n")}`,
    { schema: RUN, phase: "Check", label: `run ${round}` },
  );
  if (r == null) throw new Error(`the agent running the tests (${round}) died`);
  // A file the agent did not report counts as passed: it showed no failure.
  return new Map(
    files.map((f) => {
      const x = r.results.find((y) => y.file === f);
      return [f, x ?? { file: f, failed: false, message: "not reported" }];
    }),
  );
};

phase("Write");
const written = await parallel(items.map((item, i) => () => write(item, i)));
const failed = [];
let byItem = items.map((item, i) => {
  const tests = written[i]?.tests ?? [];
  const bad = tests.filter((t) => !FILE.test(t.file));
  if (written[i] == null || tests.length === 0 || bad.length > 0) {
    failed.push({
      item,
      reason:
        written[i] == null
          ? "test writer died"
          : tests.length === 0
            ? "no test written"
            : `unsupported file name: ${bad.map((t) => t.file).join(", ")}`,
    });
    return { item, tests: [] };
  }
  return { item, tests };
});
const owners = new Map();
for (const { item, tests } of byItem) {
  for (const t of tests) {
    if (owners.has(t.file)) {
      throw new Error(
        `${t.file} written for both ${owners.get(t.file)} and ${item}`,
      );
    }
    owners.set(t.file, item);
  }
}

if (owners.size === 0) {
  log("no test written");
  return mode === "bug"
    ? { mode, tests: [], failed }
    : { mode, frozen: [], suspect: [], failed };
}

phase("Check");
let results = await runTests([...owners.keys()], "first");

if (mode === "bug") {
  const tests = byItem[0].tests.map((t) => ({ ...t, ...results.get(t.file) }));
  log(
    `${tests.filter((t) => t.failed).length} of ${tests.length} bug tests fail`,
  );
  return { mode, tests, failed };
}

// Long mode: one rewrite for the items whose tests already pass.
const passing = byItem.filter((x) =>
  x.tests.some((t) => !results.get(t.file).failed),
);
if (passing.length > 0) {
  phase("Write");
  const again = await parallel(
    passing.map((x) => () => {
      const list = x.tests
        .filter((t) => !results.get(t.file).failed)
        .map((t) => t.file)
        .join(", ");
      return write(
        x.item,
        items.indexOf(x.item),
        ` Your earlier tests ${list} already pass although the story is not implemented: they check nothing new. Delete those files (they are yours to delete) and write tests that fail until the story is implemented.`,
      );
    }),
  );
  byItem = byItem.map((x) => {
    const k = passing.indexOf(x);
    // A dead rewriter leaves the item as it was: its passing tests are suspect.
    if (k < 0 || again[k] == null) return x;
    // The failing tests of the first writer are kept; new ones are added
    // unless another item already owns their file.
    const kept = x.tests.filter((t) => results.get(t.file).failed);
    const added = again[k].tests.filter(
      (t) =>
        FILE.test(t.file) &&
        !kept.some((y) => y.file === t.file) &&
        (owners.get(t.file) ?? x.item) === x.item,
    );
    return { item: x.item, tests: [...kept, ...added] };
  });
  const rewritten = byItem
    .filter((x) => {
      const k = passing.findIndex((p) => p.item === x.item);
      return k >= 0 && again[k] != null;
    })
    .flatMap((x) => x.tests.map((t) => t.file))
    .filter((f) => !results.has(f) || !results.get(f).failed);
  phase("Check");
  if (rewritten.length > 0) {
    const second = await runTests(rewritten, "second");
    results = new Map([...results, ...second]);
  }
}

const frozen = [];
const suspect = [];
for (const { item, tests } of byItem) {
  for (const t of tests) {
    const r = results.get(t.file);
    (r.failed ? frozen : suspect).push({
      item,
      file: t.file,
      covers: t.covers,
      message: r.message,
    });
  }
}

if (frozen.length > 0) {
  phase("Freeze");
  const files = frozen.map((t) => t.file).join(" ");
  const done = await agent(
    `From the repository root, run \`make freeze FILES='${files}'\` and change nothing else. Report whether it exited 0, with its output.`,
    { schema: DONE, phase: "Freeze", label: "freeze" },
  );
  if (!done?.ok) {
    throw new Error(`make freeze failed: ${done?.output ?? "agent died"}`);
  }
}
log(
  `${frozen.length} tests frozen, ${suspect.length} suspect, ${failed.length} items failed`,
);
return { mode, frozen, suspect, failed };
