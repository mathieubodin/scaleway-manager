export const meta = {
  name: "sdlc-parallax",
  description:
    "Have 3 independent agents each write the spec they would implement from the same idea; return every divergence between their specs as a question",
  whenToUse:
    "Launched by sdlc-work during Define, after bmad-forge-idea, with args { ideaPath } (the forged-idea.md).",
  phases: [{ title: "Readers" }, { title: "Divergences" }],
};

// args: { ideaPath: path of forged-idea.md }. Number of readers validated with
// the author on 2026-10-19: always 3. The readers' specs stay in the run's
// transcript; only the divergences come back to the session.
const READERS = 3;

const SPEC = {
  type: "object",
  properties: { spec: { type: "string" } },
  required: ["spec"],
};
const DIVERGENCES = {
  type: "object",
  properties: {
    divergences: {
      type: "array",
      items: {
        type: "object",
        properties: {
          topic: { type: "string" },
          positions: {
            type: "array",
            items: {
              type: "object",
              properties: {
                readers: { type: "array", items: { type: "integer" } },
                position: { type: "string" },
              },
              required: ["readers", "position"],
            },
          },
          question: { type: "string" },
        },
        required: ["topic", "positions", "question"],
      },
    },
  },
  required: ["divergences"],
};

const ideaPath = args?.ideaPath;
if (!ideaPath) throw new Error("args.ideaPath is required");

phase("Readers");
const written = await parallel(
  Array.from(
    { length: READERS },
    (_, i) => () =>
      agent(
        `Read ${ideaPath}, and the code of this repository it concerns, read-only. Write the spec you would implement from it, alone: scope, behavior seen by the user, data in and out, edge cases, non-goals, and acceptance criteria. Ask no question: where the idea is silent or unclear, decide yourself and mark that decision "assumed". Change no file. Return the spec as Markdown.`,
        { schema: SPEC, phase: "Readers", label: `reader ${i + 1}` },
      ),
  ),
);
const specs = written.map((w) => w?.spec).filter(Boolean);
const incomplete = specs.length < READERS;
if (specs.length < 2) {
  log(`${specs.length} spec written: nothing to compare`);
  return { readers: READERS, specs: specs.length, incomplete, divergences: [] };
}

phase("Divergences");
const listing = specs
  .map((s, i) => `--- Reader ${i + 1} ---\n${s}`)
  .join("\n\n");
const found = await agent(
  `${specs.length} agents each wrote, alone, the spec they would implement from ${ideaPath}. Compare their specs below. List every point where they differ in substance: scope, behavior, data, edge cases, non-goals, acceptance criteria; ignore wording and order. For each point give the topic, the position of each reader (by reader number; readers who said nothing on it form one position, "not addressed"), and one question for the human, in plain language, answerable in a sentence, whose answer settles the point. Do not report points where every reader agrees.\n\n${listing}`,
  { schema: DIVERGENCES, phase: "Divergences", label: "divergences" },
);
if (found == null) throw new Error("the agent comparing the specs died");

log(`${found.divergences.length} divergences between ${specs.length} specs`);
return {
  readers: READERS,
  specs: specs.length,
  incomplete,
  divergences: found.divergences,
};
