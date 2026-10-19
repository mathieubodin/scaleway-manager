export const meta = {
  name: "sdlc-cold-start",
  description:
    "Give each story spec alone to a fresh agent, as a builder would get it; return what each one is missing to implement it, blocking items apart",
  whenToUse:
    "Launched by sdlc-work during Plan, once per cold-start round, with args { storyPaths }.",
  phases: [{ title: "Cold start" }],
};

// args: { storyPaths: story spec paths }. One round per run; the loop, the
// fixes and the questions stay in sdlc-work. Rule validated with the author on
// 2026-10-19: only blocking items keep the round red; useful ones are listed
// with the plan gate page.
const MISSING = {
  type: "object",
  properties: {
    missing: {
      type: "array",
      items: {
        type: "object",
        properties: {
          item: { type: "string" },
          why: { type: "string" },
          blocking: { type: "boolean" },
        },
        required: ["item", "why", "blocking"],
      },
    },
  },
  required: ["missing"],
};

const storyPaths = args?.storyPaths;
if (!Array.isArray(storyPaths) || storyPaths.length === 0) {
  throw new Error("args.storyPaths is required: a list of story spec paths");
}
if (new Set(storyPaths).size < storyPaths.length) {
  throw new Error("args.storyPaths lists a story twice");
}

phase("Cold start");
const reports = await parallel(
  storyPaths.map(
    (path, i) => () =>
      agent(
        `You are about to implement one story, as its builder will: you get the story spec ${path} and the files it names, and you may read the code of this repository. Open nothing else under _bmad-output/ unless the story spec names it: no plan, other story specs, SPEC.md, stories.yaml, .memlog.md, define-notes.md, forged-idea.md, gate pages or briefs. Stories this one depends on are built before it, so code they create may not exist yet: what you need from them must be described in this spec (names, interfaces, behavior); report it only when the spec does not describe it. Tests for its acceptance criteria will be written and frozen before you start: their absence now is not a missing item. Implement nothing and change no file; do not invoke any skill or spawn subagents. List what you would be missing to start and finish this story without guessing: each item, why you need it, and whether it is blocking (without it you would have to guess, and could build something else) or only useful (you could start, but the spec would be better with it). Return an empty list when nothing is missing.`,
        { schema: MISSING, phase: "Cold start", label: `story ${i + 1}` },
      ),
  ),
);

const stories = storyPaths.map((path, i) => {
  const missing = reports[i]?.missing ?? [];
  return {
    path,
    dead: reports[i] == null,
    blocking: missing.filter((m) => m.blocking),
    useful: missing.filter((m) => !m.blocking),
  };
});
// A story whose agent died was not checked: it keeps the round red.
const green = stories.every((s) => !s.dead && s.blocking.length === 0);
log(
  `${stories.filter((s) => s.blocking.length > 0 || s.dead).length} of ${stories.length} stories red`,
);
return { green, stories };
