import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

// Bug #48: on the long route, the trunk merge rule ("make check after each
// merge") stops the trunk when frozen tests of later stories are red, and
// nothing after Rails maps each frozen test to the runner that reaches it.

const ROOT = spawnSync("git", ["rev-parse", "--show-toplevel"], {
  encoding: "utf8",
}).stdout.trim();
const read = (path) => readFileSync(join(ROOT, path), "utf8");

const between = (text, start, end, path) => {
  const from = text.indexOf(start);
  assert.ok(from >= 0, `${path}: '${start}' not found`);
  const to = text.indexOf(end, from + start.length);
  assert.ok(to > from, `${path}: '${end}' not found after '${start}'`);
  return text.slice(from, to);
};

// Rails and Build regions of each file that describes the long route.
const REGIONS = [
  {
    path: "docs/development-process.md",
    rails: ["- Rails:", "- Build:"],
    build: ["- Build:", "- Verification:"],
  },
  {
    path: "skills/sdlc-work/references/long.md",
    rails: ["## Rails (step `rails`)", "## Build (step `build`)"],
    build: ["## Build (step `build`)", "## Review (step `review`)"],
  },
  {
    path: ".claude/skills/sdlc-work/references/long.md",
    rails: ["## Rails (step `rails`)", "## Build (step `build`)"],
    build: ["## Build (step `build`)", "## Review (step `review`)"],
  },
];

const sentences = (text) =>
  text
    .split(/(?<=[.;:])\s+|\n/)
    .map((s) => s.trim())
    .filter(Boolean);

for (const { path, rails } of REGIONS) {
  test(`AC1: ${path} maps each frozen test to its runner and story right after Rails`, () => {
    const region = between(read(path), rails[0], rails[1], path);
    const map = sentences(region).find(
      (s) =>
        /frozen-tests\.sha256/.test(s) &&
        /\brunners?\b/i.test(s) &&
        /\bstor(y|ies)\b/i.test(s),
    );
    assert.ok(
      map,
      `${path}: Rails describes no map of each file of frozen-tests.sha256 to its runner and its story`,
    );
    const orphan = sentences(region).find(
      (s) => /\b(no|without an?)\s+runner\b/i.test(s) && /\breport/i.test(s),
    );
    assert.ok(
      orphan,
      `${path}: Rails does not report the frozen files without a runner before the build`,
    );
  });
}

for (const { path, build } of REGIONS) {
  test(`AC2: ${path} states the trunk merge rule with expected failures of stories not built yet`, () => {
    const region = between(read(path), build[0], build[1], path);
    assert.match(region, /make check-frozen/, `${path}: no make check-frozen`);
    assert.match(
      region,
      /not (been )?built yet/i,
      `${path}: Build does not accept make check failures that are frozen tests of a story not built yet`,
    );
    assert.match(
      region,
      /\bmap\b/i,
      `${path}: Build does not judge make check failures against the runner map`,
    );
    assert.match(
      region,
      /memlog/i,
      `${path}: Build does not list the expected failures in the memlog merge line`,
    );
    assert.match(
      region,
      /builder ?prompt/i,
      `${path}: Build does not name the expected failures in the builder prompt`,
    );
    assert.match(
      region,
      /\bblocks?\b/i,
      `${path}: Build does not say that any other make check failure blocks the merge`,
    );
    assert.match(
      region,
      /last (trunk )?story[^.]*green|green[^.]*last (trunk )?story/i,
      `${path}: Build does not require make check fully green after the last story`,
    );
  });
}

test("AC1, AC2: the sequence diagram shows the runner map after Rails and the expected failures at each trunk merge", () => {
  const path = "docs/development-process.md";
  const diagram = between(
    read(path),
    "Note over H,G: Rails, build, verification, review",
    "W->>F: sdlc-review",
    path,
  );
  const loop = diagram.indexOf("loop Each trunk story");
  assert.ok(loop >= 0, `${path}: trunk loop missing from the diagram`);
  assert.match(
    diagram.slice(0, loop),
    /runner/i,
    `${path}: the diagram has no runner map step between Rails and the build`,
  );
  assert.match(
    diagram.slice(loop),
    /not built/i,
    `${path}: the diagram's trunk merge does not accept failures of stories not built yet`,
  );
});

test("AC3: the installed long.md is identical to its module source", () => {
  assert.equal(
    read(".claude/skills/sdlc-work/references/long.md"),
    read("skills/sdlc-work/references/long.md"),
  );
});
