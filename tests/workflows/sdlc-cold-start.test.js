import assert from "node:assert/strict";
import { test } from "node:test";
import { runWorkflow } from "./harness.js";

const SCRIPT = ".claude/workflows/sdlc-cold-start.js";
const PATHS = ["stories/1-api.md", "stories/2-button.md"];
const item = (blocking) => ({ item: "x", why: "y", blocking });

// Answers keyed by story number; a function throws to make that agent die.
const respond =
  (byStory) =>
  ({ opts }) => {
    const answer = byStory[Number(opts.label.split(" ")[1])] ?? [];
    if (typeof answer === "function") return answer();
    return { missing: answer };
  };

test("missing story paths are refused", async () => {
  await assert.rejects(runWorkflow(SCRIPT, { args: {} }), /storyPaths/);
  await assert.rejects(
    runWorkflow(SCRIPT, { args: { storyPaths: [] } }),
    /storyPaths/,
  );
  await assert.rejects(runWorkflow(SCRIPT), /storyPaths/);
  await assert.rejects(
    runWorkflow(SCRIPT, { args: { storyPaths: "stories/1-api.md" } }),
    /list of story spec paths/,
  );
  await assert.rejects(
    runWorkflow(SCRIPT, { args: { storyPaths: [PATHS[0], PATHS[0]] } }),
    /twice/,
  );
});

test("one agent per story, given its path only", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { storyPaths: PATHS },
    respond: respond({}),
  });
  assert.equal(run.calls.length, 2);
  PATHS.forEach((path, i) => {
    const { prompt, opts } = run.calls[i];
    assert.ok(prompt.includes(`the story spec ${path} and the files it names`));
    assert.doesNotMatch(prompt, new RegExp(PATHS[1 - i]));
    assert.match(prompt, /Implement nothing and change no file/);
    assert.match(prompt, /code they create may not exist yet/);
    assert.match(prompt, /their absence now is not a missing item/);
    assert.equal(opts.phase, "Cold start");
    assert.equal(JSON.stringify(opts.schema.required), '["missing"]');
  });
  assert.deepEqual(run.result, {
    green: true,
    stories: PATHS.map((path) => ({
      path,
      dead: false,
      blocking: [],
      useful: [],
    })),
  });
});

test("useful items only: green, items listed", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { storyPaths: PATHS },
    respond: respond({ 2: [item(false)] }),
  });
  assert.equal(run.result.green, true);
  assert.deepEqual(run.result.stories[1].useful, [item(false)]);
});

test("a blocking item makes the round red", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { storyPaths: PATHS },
    respond: respond({ 1: [item(true), item(false)] }),
  });
  assert.equal(run.result.green, false);
  assert.deepEqual(run.result.stories[0].blocking, [item(true)]);
  assert.deepEqual(run.result.stories[0].useful, [item(false)]);
  assert.match(run.logs.at(-1), /^1 of 2 stories red/);
});

test("a dead agent leaves its story unchecked and the round red", async () => {
  const run = await runWorkflow(SCRIPT, {
    args: { storyPaths: PATHS },
    respond: respond({
      2: () => {
        throw new Error("dead");
      },
    }),
  });
  assert.equal(run.result.green, false);
  assert.equal(run.result.stories[1].dead, true);
  assert.deepEqual(run.result.stories[0], {
    path: PATHS[0],
    dead: false,
    blocking: [],
    useful: [],
  });
  assert.match(run.logs.at(-1), /^1 of 2 stories red/);
});
