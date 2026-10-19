export const meta = {
  name: "sdlc-build",
  description:
    "Build each ticket (story spec or bug report) in its own worktree against the frozen tests, with the two-failure rule; return the finished branches and the failed tickets",
  whenToUse:
    "Launched by sdlc-work at Build, once per trunk story in order, then once for the leaves (long route), or once for the bug, with args { mode, items, baseSha, builderPrompt }.",
  phases: [{ title: "Build" }, { title: "Verify" }, { title: "Improve" }],
};

// args:
// - mode: "long" (items: story spec paths) or "bug" (items: [bug-report.md]);
// - baseSha: the work branch SHA every worktree must start from (U25);
// - builderPrompt: sdlc_builder_prompt resolved from the bmad-build override.
// Two-failure rule (docs/development-process.md): after a failure an agent
// improves the ticket and a new builder starts; after a second failure the
// ticket goes back to Plan (long) or to the human (bug). Decided with the
// author on 2026-10-19: at most MAX_BUILDERS builders at the same time.
// A builder that reports ok is checked by another agent (commit at the tip of
// its branch, descending from baseSha, no frozen test changed). A builder that
// started from another base is not a ticket failure: the item is returned as
// "wrong base", without improving the ticket.
const MAX_BUILDERS = 3;
const SHA = /^[0-9a-f]{7,40}$/;

const BUILT = {
  type: "object",
  properties: {
    ok: { type: "boolean" },
    branch: { type: "string" },
    commit: { type: "string" },
    worktree: { type: "string" },
    wrongBase: { type: "boolean" },
    report: { type: "string" },
  },
  required: ["ok", "branch", "commit", "worktree", "wrongBase", "report"],
};
const CHECKED = {
  type: "object",
  properties: { ok: { type: "boolean" }, report: { type: "string" } },
  required: ["ok", "report"],
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
// agent() call; one that rejects (budget exhausted) frees its slot and counts
// as a dead agent.
const slots = (n) => {
  let active = 0;
  const queue = [];
  const next = () => {
    if (active >= n || queue.length === 0) return;
    active++;
    const { task, resolve } = queue.shift();
    Promise.resolve()
      .then(task)
      // agent() under the test harness never rejects.
      /* c8 ignore next */
      .catch(() => null)
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

const build = (item, i, earlier) =>
  slot(() =>
    agent(
      `${prompt.trim()}\n\nAlways report the path of this worktree (\`git rev-parse --show-toplevel\`). Before anything else, run \`git rev-parse HEAD\`: if it does not start with ${baseSha}, change nothing and report a failure with wrongBase true. Work only in this worktree. When the ticket's tests and \`make check-frozen\` pass, commit your work on the current branch and report ok with the branch name (\`git branch --show-current\`) and the commit SHA; otherwise report a failure with what blocked you, and an empty branch and commit.${earlier ?? ""}\n\n${label}: ${item}`,
      {
        isolation: "worktree",
        schema: BUILT,
        phase: "Build",
        label: `build ${i + 1}${earlier ? " again" : ""}`,
      },
    ),
  );

const check = (r, i, again) =>
  agent(
    `From the repository root, change no file and check a builder's result: branch ${JSON.stringify(r.branch)}, commit ${r.commit}. Report ok only if all hold: \`git rev-parse --verify\` finds the commit; it is the tip of that branch; \`git merge-base --is-ancestor ${baseSha} ${r.commit}\` succeeds; and \`git diff --name-only ${baseSha} ${r.commit}\` lists no file named in frozen-tests.sha256 as it is at ${baseSha}. Otherwise report what failed.`,
    {
      schema: CHECKED,
      phase: "Verify",
      label: `check ${i + 1}${again ? " again" : ""}`,
    },
  );

// One attempt: build, then check what the builder claims. A dead builder, one
// that reports ok without a branch and a commit SHA, or one whose commit the
// checker rejects, has failed.
const attempt = async (item, i, earlier) => {
  const r = await build(item, i, earlier);
  if (r == null) return { r, ok: false, reason: "builder died" };
  if (!r.ok) return { r, ok: false, reason: r.report };
  if (r.branch === "" || !SHA.test(r.commit)) {
    return { r, ok: false, reason: `incomplete report: ${r.report}` };
  }
  const c = await check(r, i, earlier != null);
  if (c?.ok) return { r, ok: true };
  return {
    r,
    ok: false,
    reason: `check failed: ${c?.report ?? "checker died"}`,
  };
};

const improve = (item, i, failure) =>
  agent(
    `${
      mode === "long"
        ? `A builder failed on the story spec ${item}. Its report: ${failure}\nImprove the story spec so that a new builder, given only this file and the frozen tests, can succeed: clarify and complete it in place. Do not change its acceptance criteria, any test, or any other file.`
        : `A builder failed on the bug report ${item}. Its report: ${failure}\nAdd what this attempt taught to ${item}, under a section "Builder attempts", without changing any other file or test.`
    } Do not commit. Return the full ticket as it now is and a one-line summary of the changes.`,
    { schema: IMPROVED, phase: "Improve", label: `improve ${i + 1}` },
  );

phase("Build");
const outcomes = await parallel(
  items.map((item, i) => async () => {
    const first = await attempt(item, i, null);
    if (first.ok) return { item, ok: true, r: first.r, attempts: 1 };
    if (first.r?.wrongBase) {
      return {
        item,
        ok: false,
        outcome: "wrong base",
        reasons: [first.reason],
        worktrees: [first.r.worktree],
      };
    }
    const improved = await improve(item, i, first.reason);
    const second = await attempt(
      item,
      i,
      improved
        ? `\n\nAn earlier builder failed on this ticket. The improved ticket below supersedes the file named after it:\n\n${improved.ticket}`
        : `\n\nAn earlier builder failed on this ticket with: ${first.reason}. The ticket file stays the reference.`,
    );
    if (second.ok) {
      return {
        item,
        ok: true,
        r: second.r,
        attempts: 2,
        ticketImproved: improved != null,
      };
    }
    return {
      item,
      ok: false,
      outcome: second.r?.wrongBase
        ? "wrong base"
        : mode === "long"
          ? "back to Plan"
          : "ask the human",
      reasons: [first.reason, second.reason],
      improvement: improved?.changes ?? "improver died",
      worktrees: [first.r?.worktree, second.r?.worktree].filter(Boolean),
    };
  }),
);

const finished = outcomes
  .filter((o) => o.ok)
  .map((o) => ({
    item: o.item,
    branch: o.r.branch,
    commit: o.r.commit,
    attempts: o.attempts,
    ticketImproved: o.ticketImproved === true,
  }));
// Failed items name the worktrees their builders left, for inspection.
const failed = outcomes
  .filter((o) => !o.ok)
  .map(({ item, outcome, reasons, improvement, worktrees }) => ({
    item,
    outcome,
    reasons,
    improvement: improvement ?? "",
    worktrees,
  }));
log(`${finished.length} finished, ${failed.length} failed`);
return {
  mode,
  finished,
  failed,
  backToPlan: failed.some((f) => f.outcome === "back to Plan"),
};
