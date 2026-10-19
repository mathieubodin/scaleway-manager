export const meta = {
  name: "sdlc-build",
  description:
    "Build each ticket (story spec or bug report) in its own worktree against the frozen tests, with the two-failure rule; return the finished branches and the failed tickets",
  whenToUse:
    "Launched by sdlc-work at Build, once for the trunk and once for the leaves (long route), or once for the bug, with args { mode, items, baseSha, builderPrompt }.",
  phases: [{ title: "Build" }, { title: "Improve" }],
};

// args:
// - mode: "long" (items: story spec paths) or "bug" (items: [bug-report.md]);
// - baseSha: the work branch SHA every worktree must start from (U25);
// - builderPrompt: sdlc_builder_prompt resolved from the bmad-build override.
// Two-failure rule (docs/development-process.md): after a failure an agent
// improves the ticket and a new builder starts; after a second failure the
// ticket goes back to Plan (long) or to the human (bug). Decided with the
// author on 2026-10-19: at most MAX_BUILDERS builders at the same time.
const MAX_BUILDERS = 3;
const SHA = /^[0-9a-f]{7,40}$/;

const BUILT = {
  type: "object",
  properties: {
    ok: { type: "boolean" },
    branch: { type: "string" },
    commit: { type: "string" },
    report: { type: "string" },
  },
  required: ["ok", "branch", "commit", "report"],
};
const IMPROVED = {
  type: "object",
  properties: { ticket: { type: "string" }, changes: { type: "string" } },
  required: ["ticket", "changes"],
};

const mode = args?.mode;
const items = args?.items;
const baseSha = args?.baseSha ?? "";
const prompt = args?.builderPrompt;
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
if (!SHA.test(baseSha)) throw new Error("args.baseSha must be a commit SHA");
if (!prompt) throw new Error("args.builderPrompt is required");

const label = mode === "long" ? "Story spec" : "Bug report";

// At most n tasks at a time; the others wait for a free slot. A task is an
// agent() call, which resolves to null when its subagent dies: it never rejects.
const slots = (n) => {
  let active = 0;
  const queue = [];
  const next = () => {
    if (active >= n || queue.length === 0) return;
    active++;
    const { task, resolve } = queue.shift();
    task()
      .then(resolve)
      .finally(() => {
        active--;
        next();
      });
  };
  return (task) =>
    new Promise((resolve) => {
      queue.push({ task, resolve });
      next();
    });
};
const slot = slots(MAX_BUILDERS);

const build = (item, i, improved) =>
  slot(() =>
    agent(
      `${prompt.trim()}\n\nBefore anything else, run \`git rev-parse HEAD\`: if it does not start with ${baseSha}, change nothing and report a failure with the reason "wrong base". Work only in this worktree. When the ticket's tests and \`make check-frozen\` pass, commit your work on the current branch and report ok with the branch name (\`git branch --show-current\`) and the commit SHA; otherwise report a failure with what blocked you, and an empty branch and commit.${
        improved
          ? `\n\nAn earlier builder failed on this ticket. The improved ticket below supersedes the file named after it:\n\n${improved}`
          : ""
      }\n\n${label}: ${item}`,
      {
        isolation: "worktree",
        schema: BUILT,
        phase: "Build",
        label: `build ${i + 1}${improved ? " again" : ""}`,
      },
    ),
  );
// A dead builder, or one that reports ok without a branch and a commit SHA,
// counts as a failure.
const success = (r) => r?.ok === true && r.branch !== "" && SHA.test(r.commit);
const why = (r) =>
  r == null
    ? "builder died"
    : r.ok
      ? `incomplete report: ${r.report}`
      : r.report;

const improve = (item, i, failure) =>
  agent(
    mode === "long"
      ? `A builder failed on the story spec ${item}. Its report: ${failure}\nImprove the story spec so that a new builder, given only this file and the frozen tests, can succeed: clarify and complete it in place. Do not change its acceptance criteria, any test, or any other file. Return the full improved story spec and a one-line summary of the changes.`
      : `A builder failed on the bug report ${item}. Its report: ${failure}\nAdd what this attempt taught to ${item}, under a section "Builder attempts", without changing any other file or test. Return the full enriched bug report and a one-line summary of the changes.`,
    { schema: IMPROVED, phase: "Improve", label: `improve ${i + 1}` },
  );

phase("Build");
const outcomes = await parallel(
  items.map((item, i) => async () => {
    const first = await build(item, i, null);
    if (success(first)) {
      return {
        item,
        ok: true,
        branch: first.branch,
        commit: first.commit,
        attempts: 1,
      };
    }
    const improved = await improve(item, i, why(first));
    const second = await build(
      item,
      i,
      improved?.ticket ?? `(the ticket could not be improved) ${why(first)}`,
    );
    if (success(second)) {
      return {
        item,
        ok: true,
        branch: second.branch,
        commit: second.commit,
        attempts: 2,
      };
    }
    return {
      item,
      ok: false,
      reasons: [why(first), why(second)],
      improvement: improved?.changes ?? "improver died",
    };
  }),
);

const finished = outcomes
  .filter((o) => o.ok)
  .map(({ item, branch, commit, attempts }) => ({
    item,
    branch,
    commit,
    attempts,
  }));
const failed = outcomes
  .filter((o) => !o.ok)
  .map(({ item, reasons, improvement }) => ({
    item,
    reasons,
    improvement,
    outcome: mode === "long" ? "back to Plan" : "ask the human",
  }));
log(`${finished.length} finished, ${failed.length} failed`);
return { mode, finished, failed };
