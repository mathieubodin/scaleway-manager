export const meta = {
  name: "sample",
  description:
    "Harness fixture: three votes per question, then a check per accepted question",
  phases: [{ title: "Vote" }, { title: "Check" }],
};

const VOTE = {
  type: "object",
  properties: { yes: { type: "boolean" } },
  required: ["yes"],
};

phase( "Vote" )
const questions = args?.questions ?? [];
if (questions.length === 0) {
  log("no question");
  return { accepted: [], checked: [] };
}

const tallies = await parallel(
  questions.map((q) => async () => {
    const votes = await parallel(
      [0, 1, 2].map((i) => () => agent(`${q} (vote ${i})`, { schema: VOTE })),
    );
    const yes = votes.filter(Boolean).filter((v) => v.yes).length;
    return { q, yes };
  }),
);
const accepted = tallies.filter((t) => t.yes >= 2).map((t) => t.q);

phase("Check");
const checked = await pipeline(
  accepted,
  (q) => agent(`check ${q}`, { phase: "Check" }),
  (text, q) => {
    if (text === null) throw new Error(`no check for ${q}`);
    return `${q}: ${text}`;
  },
);
return { accepted, checked };
