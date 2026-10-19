import assert from "node:assert/strict";
import { test } from "node:test";
import { runWorkflow } from "./harness.js";

const SCRIPT = ".claude/workflows/sdlc-review.js";
const LAYERS = ["security", "correctness", "coherence"].map((id) => ({
  id,
  instruction: `Launch a context-free subagent with this prompt:\n\n${id} review of {diff_file} against {spec_file}.\n`,
}));
const BASE = {
  base: "abc1234",
  diffDir: "/tmp/review",
  specPath: "SPEC.md",
  layers: LAYERS,
  verifierPrompt: "Verify.",
  fixerPrompt: "Fix.",
};
const LONG = { ...BASE, dryPasses: 2, maxRounds: 5 };

const finding = (title) => ({ title, location: "a.js:1", detail: title });
const verdict = (finding, more = {}) => ({
  finding,
  real: true,
  evidence: "reproduced",
  severity: "medium",
  needsDecision: false,
  duplicateOf: null,
  seenAs: null,
  ...more,
});

// Scripted answers keyed by label "r<round> <role>". `lenses(round, id)` gives a
// lens's findings, `verify(round, prompt)` the verdicts, `fix(round, prompt)` the
// fixer's report; a function that throws makes that agent die.
const respond =
  ({
    lenses = () => [],
    verify = () => [],
    fix = (_, prompt) => ({
      results: [...prompt.matchAll(/^(F\d+):/gm)].map((m) => ({
        id: m[1],
        fixed: true,
        reason: "",
      })),
      checksPassed: true,
      checks: "make check: ok",
    }),
    diff = () => ({ ok: true, output: "1 files changed" }),
  }) =>
  ({ prompt, opts }) => {
    const [r, role] = opts.label.split(" ");
    const round = Number(r.slice(1));
    if (role === "diff") return diff(round);
    if (role === "verify") return { verdicts: verify(round, prompt) };
    if (role === "fix") return fix(round, prompt);
    return { findings: lenses(round, role) };
  };

test("invalid args are refused", async () => {
  const bad = [
    {},
    { ...LONG, base: "main" },
    { ...LONG, base: "abc1234'; rm -rf /" },
    { ...LONG, diffDir: "/tmp/x y" },
    { ...LONG, diffDir: undefined },
    { ...LONG, verifierPrompt: "" },
    { ...LONG, fixerPrompt: undefined },
    { ...LONG, layers: LAYERS.slice(0, 2) },
    { ...LONG, layers: [...LAYERS.slice(0, 2), { id: "x", instruction: "" }] },
    { ...LONG, layers: [LAYERS[0], LAYERS[0], LAYERS[1]] },
    { ...LONG, layers: undefined },
    { ...LONG, dryPasses: 0 },
    { ...LONG, dryPasses: 3, maxRounds: 2 },
    { ...LONG, maxRounds: "5" },
    { ...BASE },
  ];
  for (const args of bad) await assert.rejects(runWorkflow(SCRIPT, { args }));
  await assert.rejects(runWorkflow(SCRIPT), /base/);
});

test("lens prompt: the subagent prompt, placeholders replaced", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: {
      ...LONG,
      specPath: undefined,
      layers: [...LAYERS, { id: "plain", instruction: "Plain {diff_file}." }],
    },
    respond: respond({}),
  });
  const prompts = run.calls.map((c) => c.prompt);
  assert.match(
    prompts[0],
    /review-diff\.sh 'abc1234' '\/tmp\/review\/round-1\.diff'/,
  );
  assert.match(
    prompts[1],
    /^security review of \/tmp\/review\/round-1\.diff against none\./,
  );
  assert.doesNotMatch(prompts[1], /Launch/);
  assert.match(prompts[4], /^Plain \/tmp\/review\/round-1\.diff\./);
});

test("long: stops after 2 dry passes when nothing is found", async () => {
  const run = await runWorkflow(SCRIPT, { args: LONG, respond: respond({}) });
  assert.equal(run.result.converged, true);
  assert.equal(run.result.rounds, 2);
  assert.deepEqual(run.result.findings, []);
  assert.deepEqual(run.result.unreviewedFixes, []);
  assert.equal(run.result.checks, null);
  // diff + 3 lenses per round, no verifier, no fixer
  assert.equal(run.calls.length, 8);
});

test("long: fix in round 1, false alarms in round 2, converged at round 3", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: LONG,
    respond: respond({
      lenses: (round, id) =>
        round === 1 && id === "coherence"
          ? [finding("label mismatch"), finding("same mismatch")]
          : round === 2 && id === "security"
            ? [finding("false alarm")]
            : [],
      verify: (round) =>
        round === 1
          ? [verdict(1), verdict(2, { duplicateOf: 1 })]
          : [verdict(1, { real: false, severity: null })],
    }),
  });
  const r = run.result;
  assert.equal(r.converged, true);
  assert.equal(r.rounds, 3);
  assert.equal(r.findings.length, 1);
  assert.equal(r.findings[0].id, "F1");
  assert.equal(r.findings[0].status, "fixed");
  assert.deepEqual(r.unfixed, []);
  assert.deepEqual(r.unreviewedFixes, []);
  assert.deepEqual(r.checks, { passed: true, output: "make check: ok" });
  assert.deepEqual(
    r.log.map((l) => l.dry),
    [false, true, true],
  );
  const fixPrompt = run.calls.find((c) => c.opts.label === "r1 fix").prompt;
  assert.match(fixPrompt, /^Fix\./);
  assert.match(fixPrompt, /F1: label mismatch \(a\.js:1\)/);
  assert.doesNotMatch(fixPrompt, /same mismatch/);
});

test("long: returns at round 5 with the recurring finding", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: LONG,
    respond: respond({
      lenses: (_, id) => (id === "correctness" ? [finding("off by one")] : []),
      verify: (round, prompt) => {
        if (round > 1) assert.match(prompt, /earlier rounds:\nF1: off by one/);
        return [verdict(1, { seenAs: round === 1 ? null : "F1" })];
      },
    }),
  });
  const r = run.result;
  assert.equal(r.converged, false);
  assert.equal(r.rounds, 5);
  assert.deepEqual(r.recurring, ["F1"]);
  assert.deepEqual(r.findings[0].rounds, [1, 2, 3, 4, 5]);
  assert.equal(r.findings[0].status, "fixed");
  assert.deepEqual(r.unreviewedFixes, ["F1"]);
});

test("single pass: one round, fixes applied, no second review", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { ...BASE, singlePass: true },
    respond: respond({
      lenses: (_, id) =>
        id === "security" ? [finding("a"), finding("b")] : [],
      verify: () => [verdict(1), verdict(2)],
      fix: () => ({
        results: [
          { id: "F1", fixed: true, reason: "" },
          { id: "F2", fixed: false, reason: "two fixes possible" },
        ],
        checksPassed: false,
        checks: "lint-docs failed",
      }),
    }),
  });
  const r = run.result;
  assert.equal(r.rounds, 1);
  assert.equal(r.converged, false);
  assert.deepEqual(r.unreviewedFixes, ["F1"]);
  assert.deepEqual(
    r.unfixed.map((f) => [f.id, f.reason]),
    [["F2", "two fixes possible"]],
  );
  assert.deepEqual(r.checks, { passed: false, output: "lint-docs failed" });
});

test("bug: a dead lens makes the round not dry", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { ...BASE, dryPasses: 1, maxRounds: 2 },
    respond: (call) => {
      if (call.opts.label === "r1 coherence") throw new Error("dead");
      return respond({})(call);
    },
  });
  assert.equal(run.result.converged, true);
  assert.equal(run.result.rounds, 2);
  assert.deepEqual(run.result.log[0].failedLenses, ["coherence"]);
});

test("a dead verifier or a missing verdict leaves findings unverified, unfixed", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { ...BASE, dryPasses: 1, maxRounds: 2 },
    respond: (call) => {
      if (call.opts.label === "r1 verify") throw new Error("dead");
      return respond({
        lenses: (round, id) =>
          round === 2 && id === "security"
            ? [finding("x"), finding("y")]
            : round === 1 && id === "security"
              ? [finding("x")]
              : [],
        // finding 2 gets no verdict; out-of-range and repeated numbers ignored
        verify: () => [
          verdict(1, { real: false, severity: null }),
          verdict(1),
          verdict(9),
        ],
      })(call);
    },
  });
  const r = run.result;
  assert.equal(r.converged, false);
  assert.deepEqual(
    r.log.map((l) => l.unverified),
    [1, 1],
  );
  assert.deepEqual(r.findings, []);
  assert.equal(
    run.calls.some((c) => c.opts.label.endsWith("fix")),
    false,
  );
});

test("a finding needing a decision is set aside and does not block convergence", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: LONG,
    respond: respond({
      lenses: (_, id) =>
        id === "coherence" ? [finding("which wording?")] : [],
      verify: (round) => [
        verdict(1, { needsDecision: true, seenAs: round === 1 ? null : "F1" }),
      ],
    }),
  });
  const r = run.result;
  assert.equal(r.converged, true);
  assert.equal(r.rounds, 3);
  assert.deepEqual(
    r.unfixed.map((f) => [f.id, f.status, f.rounds]),
    [["F1", "unfixed", [1]]],
  );
  assert.equal(
    run.calls.some((c) => c.opts.label.endsWith("fix")),
    false,
  );
});

test("a dead fixer leaves the finding open; not found again once converged", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: LONG,
    respond: respond({
      lenses: (round, id) =>
        round === 1 && id === "security"
          ? [finding("leak"), finding("leak 2")]
          : [],
      verify: () => [verdict(1), verdict(2)],
      fix: () => {
        throw new Error("dead");
      },
    }),
  });
  const r = run.result;
  assert.equal(r.converged, true);
  assert.deepEqual(
    r.findings.map((f) => f.status),
    ["not found again", "not found again"],
  );
  assert.equal(r.checks, null);
});

test("fixer silent on a finding, two verdicts on one earlier finding", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { ...BASE, dryPasses: 1, maxRounds: 2 },
    respond: respond({
      lenses: (round, id) =>
        id === "security"
          ? round === 1
            ? [finding("a")]
            : [finding("a"), finding("a again")]
          : [],
      verify: (round) =>
        round === 1
          ? [verdict(1)]
          : [verdict(1, { seenAs: "F1" }), verdict(2, { seenAs: "F1" })],
      fix: () => ({ results: [], checksPassed: true, checks: "ok" }),
    }),
  });
  const r = run.result;
  assert.equal(r.converged, false);
  assert.deepEqual(r.findings[0].rounds, [1, 2]);
  assert.deepEqual(
    r.unfixed.map((f) => [f.id, f.status]),
    [["F1", "open"]],
  );
});

test("a diff that cannot be written stops the run", async () => {
  await assert.rejects(
    runWorkflow(SCRIPT, {
      args: LONG,
      respond: respond({ diff: () => ({ ok: false, output: "not a commit" }) }),
    }),
    /round 1: the diff could not be written: not a commit/,
  );
  await assert.rejects(
    runWorkflow(SCRIPT, {
      args: LONG,
      respond: (call) => {
        if (call.opts.label === "r1 diff") throw new Error("dead");
        return null;
      },
    }),
    /agent died/,
  );
});
