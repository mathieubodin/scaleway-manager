export const meta = {
  name: "sdlc-parallax",
  description:
    "Have 3 independent agents each write the spec they would implement from the same idea; return every divergence between their specs as a question, important ones apart from details",
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
          weight: { enum: ["important", "detail"] },
          proposal: { type: "string" },
        },
        required: ["topic", "positions", "question", "weight", "proposal"],
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
        `Read ${ideaPath}, and the code of this repository it concerns, read-only. Write the spec you would implement from it, alone: scope, behavior seen by the user, data in and out, edge cases, non-goals, and acceptance criteria. Ask no question: where the idea is silent or unclear, decide yourself and mark that decision "assumed". Do not invoke any skill, do not spawn subagents, and change no file. Return the spec as Markdown.`,
        { schema: SPEC, phase: "Readers", label: `reader ${i + 1}` },
      ),
  ),
);
// Readers keep the number of their agent label, so that a position can be
// checked against that reader's transcript even when another one died.
const specs = written
  .map((w, i) => ({ reader: i + 1, spec: w?.spec }))
  .filter((s) => s.spec);
const deadReaders = written.flatMap((w, i) => (w?.spec ? [] : [i + 1]));
if (specs.length < 2) {
  throw new Error(`only ${specs.length} spec written: nothing to compare`);
}

phase("Divergences");
const listing = specs
  .map((s) => `--- Reader ${s.reader} ---\n${s.spec}`)
  .join("\n\n");
const found = await agent(
  `${specs.length} agents each wrote, alone, the spec they would implement from ${ideaPath}. Compare their specs below. List every point where they differ in substance: scope, behavior, data, edge cases, non-goals, acceptance criteria; ignore wording and order. For each point give the topic, the position of each reader (by reader number; readers who said nothing on it form one position, "not addressed"), and one question for the human, in plain language, answerable in a sentence, whose answer settles the point. Classify each point: "important" when its answer changes what the user sees or does, the data kept, lost or exported, security, cost, or what the acceptance criteria check; "detail" otherwise (naming, ordering, wording, a status code among equivalent ones). For each point, propose the answer you would choose, in one sentence. Do not report points where every reader agrees.\n\n${listing}`,
  { schema: DIVERGENCES, phase: "Divergences", label: "divergences" },
);
if (found == null) throw new Error("the agent comparing the specs died");

// Positions may only name readers whose spec was compared.
const alive = new Set(specs.map((s) => s.reader));
const divergences = found.divergences.map((d) => ({
  ...d,
  positions: d.positions.map((p) => ({
    ...p,
    readers: p.readers.filter((r) => alive.has(r)),
  })),
}));
// Decided with the author on 2026-10-19: important points are asked one by
// one, details come as one list of proposed answers, validated or corrected
// together.
const important = divergences.filter((d) => d.weight !== "detail");
const details = divergences.filter((d) => d.weight === "detail");
log(
  `${important.length} important and ${details.length} detail divergences between ${specs.length} specs`,
);
return { readers: READERS, deadReaders, important, details };
