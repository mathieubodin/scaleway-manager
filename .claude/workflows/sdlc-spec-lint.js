export const meta = {
  name: "sdlc-spec-lint",
  description:
    "Ask each spec-lint question about each acceptance criterion of a spec to 3 independent agents; return the failing criteria",
  whenToUse:
    "Launched by sdlc-work during Define, once per spec-lint round, with args { specPath, questions }.",
  phases: [{ title: "Criteria" }, { title: "Votes" }],
};

// args: { specPath: path of SPEC.md, questions: spec_lint_questions resolved from
// the bmad-review override by sdlc-work }. Rule validated with the author on
// 2026-10-19: a criterion passes with at least MIN_YES "yes" out of VOTES for each
// question; a vote whose agent died counts as a missing "yes".
const VOTES = 3;
const MIN_YES = 2;

const CRITERIA = {
  type: "object",
  properties: {
    criteria: {
      type: "array",
      items: {
        type: "object",
        properties: { id: { type: "string" }, text: { type: "string" } },
        required: ["id", "text"],
      },
    },
  },
  required: ["criteria"],
};
const VOTE = {
  type: "object",
  properties: { yes: { type: "boolean" }, reason: { type: "string" } },
  required: ["yes", "reason"],
};

const specPath = args?.specPath;
const questions = args?.questions ?? [];
if (!specPath || questions.length === 0) {
  throw new Error("args.specPath and args.questions are required");
}

phase("Criteria");
const listed = await agent(
  `Read ${specPath}. List every acceptance criterion it states, in order, with a short id (as written in the file, or AC1, AC2...) and its exact text. Do not judge them. Return an empty list when the file has none.`,
  { schema: CRITERIA, phase: "Criteria", label: "list criteria" },
);
const criteria = listed?.criteria ?? [];
if (criteria.length === 0) {
  log("no acceptance criterion found");
  return { criteria: 0, failing: [], results: [], noCriteria: true };
}

const results = await pipeline(criteria, (c) =>
  parallel(
    questions.map((q) => async () => {
      const votes = await parallel(
        Array.from(
          { length: VOTES },
          (_, i) => () =>
            agent(
              `You judge one acceptance criterion of the spec ${specPath}, alone. Criterion ${c.id}: "${c.text}". Question: ${q} Answer yes or no from the criterion's own wording (read the spec only for context), with a one-sentence reason.`,
              {
                schema: VOTE,
                phase: "Votes",
                label: `${c.id} q${questions.indexOf(q) + 1} v${i + 1}`,
              },
            ),
        ),
      );
      const yes = votes.filter((v) => v?.yes === true).length;
      return {
        question: q,
        yes,
        votes: votes.map((v) => v ?? { yes: null, reason: "agent died" }),
        pass: yes >= MIN_YES,
      };
    }),
  ).then((answers) => ({
    id: c.id,
    text: c.text,
    answers,
    pass: answers.every((a) => a.pass),
  })),
);

const failing = results.filter((r) => !r.pass);
log(`${failing.length} of ${criteria.length} criteria failing`);
return { criteria: criteria.length, failing, results, noCriteria: false };
