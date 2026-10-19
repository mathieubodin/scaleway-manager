export const meta = {
  name: "sdlc-cold-start",
  description:
    "Give each story spec alone to a fresh agent, as a builder would get it; return what each one is missing to start, blocking items apart",
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

const storyPaths = args?.storyPaths ?? [];
if (storyPaths.length === 0) throw new Error("args.storyPaths is required");

phase("Cold start");
const reports = await parallel(
  storyPaths.map(
    (path, i) => () =>
      agent(
        `You are about to implement one story, and the story spec ${path} is all you are given. Read it, and the code and files of this repository it names or that you need to find your way; do not read any other planning document (no plan.md, SPEC.md, stories.yaml, other story specs, memlog, define notes, forge or brainstorm files). Implement nothing and change no file; do not invoke any skill or spawn subagents. List what you would be missing to start and finish this story without guessing: each item, why you need it, and whether it is blocking (without it you would have to guess, and could build something else) or only useful (you could start, but the spec would be better with it). Return an empty list when nothing is missing.`,
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
