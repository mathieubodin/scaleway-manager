export const meta = {
  name: "sdlc-review",
  description:
    "Review a change with the security, correctness and coherence lenses, verify each finding, fix the verified ones, and loop until enough dry passes or the round cap",
  whenToUse:
    "Launched by sdlc-work at the review step, with args { base, diffDir, specPath, layers, verifierPrompt, fixerPrompt, dryPasses, maxRounds } or singlePass: true instead of the two counts.",
  phases: [
    { title: "Diff" },
    { title: "Review" },
    { title: "Verify" },
    { title: "Fix" },
  ],
};

// args (docs/development-process.md, "Review loop"):
// - base: SHA the change is compared with (merge-base of the work branch);
// - diffDir: folder where each round's diff is written;
// - specPath: story spec, SPEC.md or bug-report.md, optional;
// - layers, verifierPrompt, fixerPrompt: review_layers, sdlc_verifier_prompt and
//   sdlc_fixer_prompt resolved from the bmad-code-review override by sdlc-work;
// - dryPasses and maxRounds (long 2 and 5, bug 1 and 2), or singlePass (short).
// Rule validated with the author on 2026-10-19: a round is dry when every lens
// answered, every finding got a verdict, and no new verified finding remains.
// A verified finding set aside for the human (its fix needs a decision) counts
// in the round that finds it, not when found again.
const MIN_LENSES = 3;

const FINDINGS = {
  type: "object",
  properties: {
    findings: {
      type: "array",
      items: {
        type: "object",
        properties: {
          title: { type: "string" },
          location: { type: "string" },
          detail: { type: "string" },
        },
        required: ["title", "location", "detail"],
      },
    },
  },
  required: ["findings"],
};
const VERDICTS = {
  type: "object",
  properties: {
    verdicts: {
      type: "array",
      items: {
        type: "object",
        properties: {
          finding: { type: "integer" },
          real: { type: "boolean" },
          evidence: { type: "string" },
          severity: { enum: ["high", "medium", "low", null] },
          needsDecision: { type: "boolean" },
          duplicateOf: { type: ["integer", "null"] },
          seenAs: { type: ["string", "null"] },
        },
        required: [
          "finding",
          "real",
          "evidence",
          "severity",
          "needsDecision",
          "duplicateOf",
          "seenAs",
        ],
      },
    },
  },
  required: ["verdicts"],
};
const FIXES = {
  type: "object",
  properties: {
    results: {
      type: "array",
      items: {
        type: "object",
        properties: {
          id: { type: "string" },
          fixed: { type: "boolean" },
          reason: { type: "string" },
        },
        required: ["id", "fixed", "reason"],
      },
    },
    checksPassed: { type: "boolean" },
    checks: { type: "string" },
  },
  required: ["results", "checksPassed", "checks"],
};
const DONE = {
  type: "object",
  properties: { ok: { type: "boolean" }, output: { type: "string" } },
  required: ["ok", "output"],
};

const a = args ?? {};
const singlePass = a.singlePass === true;
const dryPasses = singlePass ? 1 : a.dryPasses;
const maxRounds = singlePass ? 1 : a.maxRounds;
const layers = (a.layers ?? []).filter((l) => l?.id && l.instruction);
const spec = a.specPath || "none";
// Both values end up inside single quotes in a shell command.
if (!/^[0-9a-f]{7,40}$/.test(a.base ?? "")) {
  throw new Error("args.base must be a commit SHA");
}
if (!/^[\w./-]+$/.test(a.diffDir ?? "")) {
  throw new Error("args.diffDir must be a path of letters, digits, . _ / -");
}
if (!a.verifierPrompt || !a.fixerPrompt) {
  throw new Error("args.verifierPrompt and args.fixerPrompt are required");
}
if (new Set(layers.map((l) => l.id)).size < MIN_LENSES) {
  throw new Error(`args.layers must hold at least ${MIN_LENSES} active lenses`);
}
if (
  !Number.isInteger(dryPasses) ||
  !Number.isInteger(maxRounds) ||
  dryPasses < 1 ||
  maxRounds < dryPasses
) {
  throw new Error(
    "args.dryPasses and args.maxRounds must be integers, 1 <= dryPasses <= maxRounds, or singlePass: true",
  );
}

// A layer instruction launches a subagent "with this prompt:"; the agent
// started here is that subagent, so it gets the prompt itself.
const lensPrompt = (layer, diffFile) => {
  const m = layer.instruction.match(/with this prompt:[ \t]*\n([\s\S]*)$/);
  const text = (m ? m[1] : layer.instruction)
    .replaceAll("{diff_file}", diffFile)
    .replaceAll("{spec_file}", spec);
  return `${text.trim()}\n\nReturn the findings in the requested structured form.`;
};

const seen = []; // every verified finding, across rounds
const rounds = [];
let dry = 0;
let round = 0;
let checks = null;

while (round < maxRounds && dry < dryPasses) {
  round++;
  const diffFile = `${a.diffDir}/round-${round}.diff`;

  phase("Diff");
  const diff = await agent(
    `From the repository root, run \`scripts/review-diff.sh '${a.base}' '${diffFile}'\` and change nothing else. Report whether it exited 0, with its output.`,
    { schema: DONE, phase: "Diff", label: `r${round} diff` },
  );
  if (!diff?.ok) {
    throw new Error(
      `round ${round}: the diff could not be written: ${diff?.output ?? "agent died"}`,
    );
  }

  phase("Review");
  const reports = await parallel(
    layers.map(
      (l) => () =>
        agent(lensPrompt(l, diffFile), {
          schema: FINDINGS,
          phase: "Review",
          label: `r${round} ${l.id}`,
        }),
    ),
  );
  const failedLenses = layers
    .filter((_, i) => reports[i] == null)
    .map((l) => l.id);
  const raw = reports.flatMap((r, i) =>
    (r?.findings ?? []).map((f) => ({ ...f, lens: layers[i].id })),
  );

  let verdicts = [];
  if (raw.length > 0) {
    phase("Verify");
    const listing = raw
      .map(
        (f, i) =>
          `${i + 1}. [${f.lens}] ${f.title} (${f.location}): ${f.detail}`,
      )
      .join("\n");
    const known =
      seen.map((f) => `${f.id}: ${f.title} (${f.location})`).join("\n") ||
      "none";
    const v = await agent(
      `${a.verifierPrompt.trim()}\n\nDiff: ${diffFile}. Spec or bug report: ${spec}.\n\nFindings of this round:\n${listing}\n\nFindings of earlier rounds:\n${known}`,
      { schema: VERDICTS, phase: "Verify", label: `r${round} verify` },
    );
    verdicts = v?.verdicts ?? [];
  }

  // One verdict per finding number; a finding without verdict stays unverified.
  const byFinding = new Map();
  for (const v of verdicts) {
    if (
      v.finding >= 1 &&
      v.finding <= raw.length &&
      !byFinding.has(v.finding)
    ) {
      byFinding.set(v.finding, v);
    }
  }
  const unverified = raw.length - byFinding.size;

  const confirmed = [];
  for (const [n, v] of byFinding) {
    if (!v.real || v.duplicateOf != null) continue;
    const prev = seen.find((s) => s.id === v.seenAs);
    if (prev) {
      // Found again: waiting for the human, or already counted in this round.
      if (prev.status === "unfixed" || confirmed.includes(prev)) continue;
      prev.rounds.push(round);
      prev.evidence = v.evidence;
      prev.severity = v.severity;
      prev.status = "open";
      confirmed.push(prev);
      continue;
    }
    const f = raw[n - 1];
    const found = {
      id: `F${seen.length + 1}`,
      lens: f.lens,
      title: f.title,
      location: f.location,
      detail: f.detail,
      evidence: v.evidence,
      severity: v.severity,
      rounds: [round],
      status: "open",
      reason: "",
    };
    if (v.needsDecision) {
      found.status = "unfixed";
      found.reason = "its fix needs a decision by the human";
    }
    seen.push(found);
    confirmed.push(found);
  }

  const toFix = confirmed.filter((f) => f.status === "open");
  const fixed = [];
  if (toFix.length > 0) {
    phase("Fix");
    const listing = toFix
      .map(
        (f) =>
          `${f.id}: ${f.title} (${f.location}): ${f.detail} Evidence: ${f.evidence}`,
      )
      .join("\n");
    const r = await agent(
      `${a.fixerPrompt.trim()}\n\nSpec or bug report: ${spec}.\n\nVerified findings:\n${listing}`,
      { schema: FIXES, phase: "Fix", label: `r${round} fix` },
    );
    for (const f of toFix) {
      const x = r?.results?.find((y) => y.id === f.id);
      if (x?.fixed) {
        f.status = "fixed";
        fixed.push(f.id);
      } else if (x) {
        f.status = "unfixed";
        f.reason = x.reason;
      }
    }
    checks = r ? { passed: r.checksPassed, output: r.checks } : null;
  }

  const isDry =
    failedLenses.length === 0 && unverified === 0 && confirmed.length === 0;
  dry = isDry ? dry + 1 : 0;
  rounds.push({
    round,
    raw: raw.length,
    unverified,
    failedLenses,
    confirmed: confirmed.map((f) => f.id),
    fixed,
    dry: isDry,
  });
  log(
    `round ${round}: ${raw.length} raw, ${confirmed.length} verified, ${fixed.length} fixed${isDry ? ", dry" : ""}`,
  );
}

const converged = dry >= dryPasses;
if (converged) {
  // A finding whose fixer died and that no lens reported again.
  for (const f of seen) if (f.status === "open") f.status = "not found again";
}
return {
  converged,
  rounds: round,
  findings: seen,
  recurring: seen.filter((f) => f.rounds.length > 1).map((f) => f.id),
  unfixed: seen.filter((f) => f.status === "unfixed" || f.status === "open"),
  // Fixes of the last round that no later round reviewed.
  unreviewedFixes: rounds.at(-1).fixed,
  checks,
  log: rounds,
};
