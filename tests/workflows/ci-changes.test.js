// #65: scripts/ci-changes.sh and the changes job of pr-checks.yml. A push that only
// changes work memlogs and cost.json skips the jobs that need changes; any doubt runs them.
import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import {
  chmodSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { delimiter, dirname, join } from "node:path";
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

const SKIPPABLE = [
  "lint-docs",
  "lint-workflows",
  "test-workflows",
  "check-frozen",
  "lint-go",
  "test-go",
  "check-go-mod",
  "lint-chart",
  "build-image",
];

// A fake gh: logs its arguments, prints FAKE_GH_RUNS (the lines its --jq would print),
// exits with FAKE_GH_EXIT.
const fakeBin = join(repo, ".fake-bin");
const ghLog = join(repo, ".gh.log");
mkdirSync(fakeBin);
writeFileSync(
  join(fakeBin, "gh"),
  `#!/bin/sh
echo "$*" >> "${ghLog}"
printf '%s' "$FAKE_GH_RUNS"
exit "\${FAKE_GH_EXIT:-0}"
`,
);
chmodSync(join(fakeBin, "gh"), 0o755);
const allPassed = SKIPPABLE.map((name) => `${name} success\n`).join("");

const run = (env) => {
  const r = spawnSync(SCRIPT, [], {
    cwd: repo,
    encoding: "utf8",
    env: {
      PATH: `${fakeBin}${delimiter}${process.env.PATH}`,
      REPO: "owner/name",
      FAKE_GH_RUNS: allPassed,
      ...env,
    },
  });
  assert.equal(r.status, 0, r.stderr);
  return r.stdout.trim();
};

git("init", "-q");
writeFileSync(join(repo, ".git/info/exclude"), ".fake-bin/\n.gh.log\n");
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
  writeFileSync(ghLog, "");
  assert.equal(
    run({ ACTION: "synchronize", BEFORE: base, AFTER: memlogOnly }),
    "run=false",
  );
  assert.equal(
    readFileSync(ghLog, "utf8"),
    `api repos/owner/name/commits/${base}/check-runs?filter=latest&per_page=100 --jq .check_runs[] | "\\(.name) \\(.conclusion)"\n`,
  );
});

test("skippable jobs skipped on before (a chain of memlog pushes): run=false", () => {
  const FAKE_GH_RUNS = SKIPPABLE.map((name) => `${name} skipped\n`).join("");
  assert.equal(
    run({
      ACTION: "synchronize",
      BEFORE: base,
      AFTER: memlogOnly,
      FAKE_GH_RUNS,
    }),
    "run=false",
  );
});

test("a skippable job of before failed, cancelled, running or missing: run=true", () => {
  for (const last of [
    "build-image failure\n",
    "build-image cancelled\n",
    "build-image null\n",
    "",
  ]) {
    const FAKE_GH_RUNS =
      SKIPPABLE.slice(0, -1)
        .map((name) => `${name} success\n`)
        .join("") + last;
    assert.equal(
      run({
        ACTION: "synchronize",
        BEFORE: base,
        AFTER: memlogOnly,
        FAKE_GH_RUNS,
      }),
      "run=true",
      last,
    );
  }
});

test("gh fails or the repository is not owner/name: run=true", () => {
  assert.equal(
    run({
      ACTION: "synchronize",
      BEFORE: base,
      AFTER: memlogOnly,
      FAKE_GH_EXIT: "1",
    }),
    "run=true",
  );
  for (const REPO of ["", "owner", "owner/name/x", "owner/na me"]) {
    assert.equal(
      run({ ACTION: "synchronize", BEFORE: base, AFTER: memlogOnly, REPO }),
      "run=true",
      REPO,
    );
  }
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
  assert.deepEqual(
    names.filter((name) => !["changes", "check-migration"].includes(name)),
    SKIPPABLE,
  );
  assert.match(jobs[0], /run: scripts\/ci-changes\.sh >> "\$GITHUB_OUTPUT"/);
  assert.match(jobs[0], /fetch-depth: 0/);
  assert.match(jobs[0], /permissions:\n {6}contents: read\n {6}checks: read\n/);
  assert.match(jobs[0], /REPO: \$\{\{ github\.repository \}\}/);
  assert.match(jobs[0], /GH_TOKEN: \$\{\{ github\.token \}\}/);
});

test("ci-changes.sh lists the same skippable jobs", () => {
  const script = readFileSync(SCRIPT, "utf8");
  assert.match(script, new RegExp(`^SKIPPABLE="${SKIPPABLE.join(" ")}"$`, "m"));
});

test("a source file renamed into a work memlog: run=true", () => {
  const lines = Array.from({ length: 20 }, (_, i) => `line ${i}\n`).join("");
  const withSrc = commit({ "src/x.go": lines });
  mkdirSync(join(repo, "_bmad-output/work/issue-2-b"), { recursive: true });
  git("mv", "src/x.go", "_bmad-output/work/issue-2-b/.memlog.md");
  const renamed = commit({});
  assert.equal(
    run({ ACTION: "synchronize", BEFORE: withSrc, AFTER: renamed }),
    "run=true",
  );
});
