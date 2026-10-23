// #65: scripts/ci-changes.sh and the changes job of pr-checks.yml. A push that only
// changes work memlogs and cost.json skips the jobs that need changes; any doubt runs them.
import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import {
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";

const ROOT = fileURLToPath(new URL("../..", import.meta.url));
const SCRIPT = join(ROOT, "scripts/ci-changes.sh");

const repo = mkdtempSync(join(tmpdir(), "ci-changes-"));
after(() => rmSync(repo, { recursive: true, force: true }));

const git = (...args) =>
  execFileSync("git", ["-C", repo, ...args], {
    encoding: "utf8",
    env: {
      ...process.env,
      GIT_AUTHOR_NAME: "t",
      GIT_AUTHOR_EMAIL: "t@t",
      GIT_COMMITTER_NAME: "t",
      GIT_COMMITTER_EMAIL: "t@t",
    },
  }).trim();

// Commit the given files (path -> content) and return the new SHA.
const commit = (files) => {
  for (const [path, content] of Object.entries(files)) {
    mkdirSync(dirname(join(repo, path)), { recursive: true });
    writeFileSync(join(repo, path), content);
  }
  git("add", "-A");
  git("commit", "-q", "--allow-empty", "-m", "c");
  return git("rev-parse", "HEAD");
};

const run = (env) => {
  const r = spawnSync(SCRIPT, [], {
    cwd: repo,
    encoding: "utf8",
    env: { PATH: process.env.PATH, ...env },
  });
  assert.equal(r.status, 0, r.stderr);
  return r.stdout.trim();
};

git("init", "-q");
const base = commit({ "README.md": "a\n" });
const memlogOnly = commit({
  "_bmad-output/work/issue-1-a/.memlog.md": "m\n",
  "_bmad-output/work/issue-1-a/cost.json": "{}\n",
});
const withDoc = commit({
  "_bmad-output/work/issue-1-a/.memlog.md": "m2\n",
  "docs/x.md": "x\n",
});
const otherWorkFile = commit({
  "_bmad-output/work/issue-1-a/evidence.md": "e\n",
});
const empty = commit({});

test("a synchronize push of work memlogs and cost.json only: run=false", () => {
  assert.equal(
    run({ ACTION: "synchronize", BEFORE: base, AFTER: memlogOnly }),
    "run=false",
  );
});

test("any other file in the push: run=true", () => {
  assert.equal(
    run({ ACTION: "synchronize", BEFORE: memlogOnly, AFTER: withDoc }),
    "run=true",
  );
  assert.equal(
    run({ ACTION: "synchronize", BEFORE: withDoc, AFTER: otherWorkFile }),
    "run=true",
  );
});

test("an empty diff: run=true", () => {
  assert.equal(
    run({ ACTION: "synchronize", BEFORE: otherWorkFile, AFTER: empty }),
    "run=true",
  );
});

test("not a synchronize: run=true", () => {
  for (const ACTION of ["opened", "edited", "reopened", ""]) {
    assert.equal(run({ ACTION, BEFORE: base, AFTER: memlogOnly }), "run=true");
  }
});

test("no usable before, or a force-push: run=true", () => {
  assert.equal(run({ ACTION: "synchronize", AFTER: memlogOnly }), "run=true");
  assert.equal(
    run({ ACTION: "synchronize", BEFORE: "0".repeat(40), AFTER: memlogOnly }),
    "run=true",
  );
  assert.equal(
    run({ ACTION: "synchronize", BEFORE: "not-a-sha", AFTER: memlogOnly }),
    "run=true",
  );
  // before is not an ancestor of after
  assert.equal(
    run({ ACTION: "synchronize", BEFORE: memlogOnly, AFTER: base }),
    "run=true",
  );
});

test("pr-checks.yml: every job but changes and check-migration needs changes and its guard", () => {
  const yml = readFileSync(
    join(ROOT, ".github/workflows/pr-checks.yml"),
    "utf8",
  );
  const jobs = yml
    .split("\njobs:\n")[1]
    .split(/\n(?= {2}[a-z][a-z-]*:\n)/)
    .filter((chunk) => /^ {2}[a-z]/.test(chunk));
  const names = jobs.map((j) => j.match(/^ {2}([a-z][a-z-]*):/m)[1]);
  assert.deepEqual(names, [
    "changes",
    "lint-docs",
    "lint-workflows",
    "test-workflows",
    "check-frozen",
    "lint-go",
    "test-go",
    "check-go-mod",
    "lint-chart",
    "build-image",
    "check-migration",
  ]);
  // A GitHub Actions expression, built so that it does not read as a template literal.
  const expression = "!cancelled() && needs.changes.outputs.run != 'false'";
  const guard = `\n    needs: changes\n    if: $\{{ ${expression} }}\n`;
  for (const [i, job] of jobs.entries()) {
    const skippable = !["changes", "check-migration"].includes(names[i]);
    assert.equal(job.includes(guard), skippable, names[i]);
  }
  assert.match(jobs[0], /run: scripts\/ci-changes\.sh >> "\$GITHUB_OUTPUT"/);
  assert.match(jobs[0], /fetch-depth: 0/);
});
