export const meta = {
  name: "sdlc-spec-lint",
  description:
    "Ask each spec-lint question about each acceptance criterion of a spec to 3 independent agents; return the failing criteria",
  whenToUse:
    "Launched by sdlc-work during Define, once per spec-lint round, with args { specPath, questions, workFolder }.",
  phases: [{ title: "Acquired" }, { title: "Criteria" }, { title: "Votes" }],
};

// args: { specPath: path of SPEC.md, questions: spec_lint_questions resolved from
// the bmad-review override by sdlc-work, workFolder: optional, the work folder that
// holds define-notes.md }. A criterion listed there, in the section "Spec-lint", as
// "- <id>: acquired (run <run id>): <text>" is given to the agent that lists the
// criteria, which does not list it again unless its text changed. Rule validated with the author on
// 2026-10-19: a criterion passes with at least MIN_YES "yes" out of VOTES for each
// question; a vote whose agent died counts as a missing "yes".
const VOTES = 3;
const MIN_YES = 2;
// The judges (the vote agents, not the agent that lists the criteria) run on a pinned
// model and a high reasoning effort. The full model ID pins Opus 5.5; the alias `opus`
// moves to the newest Opus (https://code.claude.com/docs/en/model-config.md).
const JUDGE_MODEL = "claude-opus-5-5";
const JUDGE_EFFORT = "high";
// The agent that lists the criteria runs on a pinned Sonnet 5.5; the agent that reads
// define-notes.md runs on a pinned Haiku 5.5.
const LISTER_MODEL = "claude-sonnet-5-5";
const LISTER_EFFORT = "high";
const READER_MODEL = "claude-haiku-5-5";
const READER_EFFORT = "medium";

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

const ACQUIRED = {
  type: "object",
  properties: {
    present: { type: "boolean" },
    acquired: {
      type: "array",
      items: {
        type: "object",
        properties: { id: { type: "string" }, text: { type: "string" } },
        required: ["id", "text"],
      },
    },
  },
  required: ["present", "acquired"],
};

const specPath = args?.specPath;
const workFolder = args?.workFolder;
const questions = args?.questions ?? [];
if (!specPath || questions.length === 0) {
  throw new Error("args.specPath and args.questions are required");
}

let acquired = [];
if (workFolder) {
  phase("Acquired");
  const read = await agent(
    `Read ${workFolder}/define-notes.md. If the file does not exist, return present false and an empty acquired list. Otherwise look only at its section "Spec-lint" and return in acquired, for every line of the form "- <id>: acquired (run <run id>): <text>", its id and its text, copied exactly as written. Add nothing else.`,
    {
      schema: ACQUIRED,
      model: READER_MODEL,
      effort: READER_EFFORT,
      phase: "Acquired",
      label: "read acquired",
    },
  );
  acquired = read?.acquired ?? [];
  log(
    `acquired criteria, not listed again: ${acquired.map((a) => a.id).join(", ") || "none"}`,
  );
}

const acquiredNote =
  acquired.length === 0
    ? ""
    : ` Criteria already acquired are listed below, each with its id and the text it had when it was acquired. Do not list a criterion whose id is in this list and whose text in the spec has not changed in meaning. List it again if its text has changed. List every other criterion as usual.\n${acquired.map((a) => `- ${a.id}: ${a.text}`).join("\n")}`;

phase("Criteria");
const listed = await agent(
  `Read ${specPath}. List every acceptance criterion it states, in order, with a short id (as written in the file, or AC1, AC2...) and its exact text. Do not judge them. Return an empty list when the file has none.${acquiredNote}`,
  {
    schema: CRITERIA,
    model: LISTER_MODEL,
    effort: LISTER_EFFORT,
    phase: "Criteria",
    label: "list criteria",
  },
);
const criteria = listed?.criteria ?? [];
if (criteria.length === 0 && listed && acquired.length > 0) {
  log("every criterion is acquired");
  return { criteria: 0, failing: [], results: [], noCriteria: false };
}
if (criteria.length === 0) {
  log("no acceptance criterion found");
  return { criteria: 0, failing: [], results: [], noCriteria: true };
}

const judge = (c) =>
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
                model: JUDGE_MODEL,
                effort: JUDGE_EFFORT,
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
  }));

const results = await pipeline(criteria, judge);

const failing = results.filter((r) => !r.pass);
log(`${failing.length} of ${criteria.length} criteria failing`);
return { criteria: criteria.length, failing, results, noCriteria: false };
