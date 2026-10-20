import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import {
  chmodSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";

// Story 4 of #41: the process files list the five checks added by the
// application skeleton (lint-go, test-go, check-go-mod, lint-chart, build-image).

const ROOT = spawnSync("git", ["rev-parse", "--show-toplevel"], {
  encoding: "utf8",
}).stdout.trim();
const read = (path) => readFileSync(join(ROOT, path), "utf8");

const OLD_CHECKS = [
  "lint-docs",
  "lint-workflows",
  "test-workflows",
  "check-frozen",
];
const NEW_CHECKS = [
  "lint-go",
  "test-go",
  "check-go-mod",
  "lint-chart",
  "build-image",
];

// Runs scripts/evidence.sh with a stub `make` first on PATH, in a throwaway
// work folder, so that no real check runs. Returns the evidence record and the
// targets the stub received, in order.
const runEvidence = () => {
  const issue = String(900000 + (process.pid % 99999));
  const work = join(ROOT, "_bmad-output", "work", `issue-${issue}-test`);
  const bin = mkdtempSync(join(tmpdir(), "evidence-stub-"));
  const calls = join(bin, "calls.txt");
  try {
    mkdirSync(work, { recursive: true });
    writeFileSync(
      join(bin, "make"),
      `#!/usr/bin/env bash\nprintf '%s\\n' "$*" >>"${calls}"\necho "stub ok"\n`,
    );
    chmodSync(join(bin, "make"), 0o755);
    const run = spawnSync("bash", ["scripts/evidence.sh", issue], {
      cwd: ROOT,
      encoding: "utf8",
      env: { ...process.env, PATH: `${bin}:${process.env.PATH}` },
    });
    assert.equal(
      run.status,
      0,
      `evidence.sh failed:\n${run.stdout}${run.stderr}`,
    );
    const record = readFileSync(join(work, "evidence.md"), "utf8");
    const targets = readFileSync(calls, "utf8")
      .trim()
      .split("\n")
      .map((line) =>
        line
          .split(" ")
          .filter((w) => !w.startsWith("-") && !w.includes("="))
          .join(" "),
      );
    return { issue, record, targets };
  } finally {
    rmSync(work, { recursive: true, force: true });
    rmSync(bin, { recursive: true, force: true });
  }
};

test("AC1: make evidence runs the ten checks, the five new ones after check-frozen", () => {
  const { targets } = runEvidence();
  assert.deepEqual(targets, [...OLD_CHECKS, ...NEW_CHECKS, "check-migration"]);
});

test("AC1: evidence.md has a section for each of the ten checks", () => {
  const { issue, record } = runEvidence();
  const all = [...OLD_CHECKS, ...NEW_CHECKS, `check-migration ISSUE=${issue}`];
  for (const check of all) {
    assert.ok(
      record.includes(`### \`make ${check}\``),
      `no section for make ${check}`,
    );
  }
  const sections = record.match(/^### `make /gm) ?? [];
  assert.equal(sections.length, 10);
});

test("AC1: evidence.md lists only coverage, mutation testing and the behavior diff as not run", () => {
  const { record } = runEvidence();
  const after = record.split(/^Not run:$/m)[1];
  assert.ok(after, "no 'Not run:' list");
  const items = [];
  for (const line of after.trim().split("\n")) {
    if (!line.startsWith("- ")) break;
    items.push(line);
  }
  assert.deepEqual(items, [
    "- Coverage, mutation testing: no target yet (#14)",
    "- Behavior diff: no target yet (#15)",
  ]);
});

test("AC2: no 'four checks' nor 'U12, stack' left in the process doc and the evidence script", () => {
  for (const path of ["docs/development-process.md", "scripts/evidence.sh"]) {
    const text = read(path);
    assert.ok(!/four checks/.test(text), `${path} still says four checks`);
    assert.ok(!/U12, stack/.test(text), `${path} still says U12, stack`);
  }
});

test("AC3: the installed deliver.md is identical to the module source", () => {
  assert.equal(
    read(".claude/skills/sdlc-work/references/deliver.md"),
    read("skills/sdlc-work/references/deliver.md"),
  );
});

test("AC3: deliver.md section 4 carries the required-checks reminder after the PR link line", () => {
  for (const path of [
    "skills/sdlc-work/references/deliver.md",
    ".claude/skills/sdlc-work/references/deliver.md",
  ]) {
    const text = read(path);
    const link = text.indexOf("Give the user the PR link");
    assert.ok(link >= 0, `${path}: PR link line missing`);
    const rest = text.slice(link);
    assert.match(rest, /\.github\/workflows\/pr-checks\.yml/);
    assert.match(
      rest,
      /repos\/\{owner\}\/\{repo\}\/branches\/main\/protection\/required_status_checks\/contexts/,
    );
    assert.match(rest, /required-checks reminder: /);
  }
});

test("AC4: docs/migrations/next/41.md is exactly 'Migration: no impact'", () => {
  assert.equal(
    read("docs/migrations/next/41.md").replace(/\n$/, ""),
    "Migration: no impact",
  );
});

test("AC4: the Markdown files of the story pass markdownlint", () => {
  const run = spawnSync(
    join(ROOT, "node_modules", ".bin", "markdownlint-cli2"),
    [
      "docs/development-process.md",
      "docs/migrations/next/41.md",
      "skills/sdlc-work/references/deliver.md",
      ".claude/skills/sdlc-work/references/deliver.md",
    ],
    { cwd: ROOT, encoding: "utf8" },
  );
  assert.equal(run.status, 0, `${run.stdout}${run.stderr}`);
});
