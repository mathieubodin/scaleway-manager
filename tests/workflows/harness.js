// Test harness for Claude Code workflow scripts (.claude/workflows/*.js).
//
// A workflow script is not a module: it starts with `export const meta = {...}`
// and its body uses top-level `await` and `return`. The harness compiles it as
// the body of an async function, in a fresh V8 context, with the script's own
// path as file name so that c8 reports coverage on the source file.
//
// The hooks are fakes driven by the test:
// - agent(prompt, opts) returns `respond({prompt, opts, index})`; when respond
//   throws or rejects, agent returns null, as for a subagent that died.
// - parallel(thunks) runs every thunk; a thunk that throws resolves to null.
// - pipeline(items, ...stages) runs each item through the stages without a
//   barrier; a stage that throws drops the item to null.
// - phase(title) and log(message) are recorded.
// - budget: total from the `budget` option (null by default), nothing spent.
// - workflow(nameOrRef, args) calls the `workflow` option, and throws without it.
// - Date.now(), Math.random(), Date() and new Date() without argument throw.
//
// The script's result and meta are cloned with structuredClone: they must be
// plain data, as a workflow result is.

import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import vm from "node:vm";

const HOOKS = [
  "agent",
  "parallel",
  "pipeline",
  "phase",
  "log",
  "args",
  "budget",
  "workflow",
];
// The script must begin with it; no newline inside, so that lines do not move.
const META = /^export const meta[ \t]*=/;

export async function runWorkflow(
  path,
  { args, respond = () => null, budget = null, workflow } = {},
) {
  const file = resolve(path);
  const source = readFileSync(file, "utf8");
  if (!META.test(source)) {
    throw new Error(`${path}: does not begin with "export const meta = {...}"`);
  }
  // Same length, so that line and column numbers stay those of the source.
  const body = source.replace(
    META,
    (m) => `__meta.value${" ".repeat(m.length - 14)} =`,
  );

  const calls = [];
  const phases = [];
  const logs = [];
  const __meta = {};

  const settle = (fn) =>
    Promise.resolve()
      .then(fn)
      .catch(() => null);
  const hooks = {
    agent: (prompt, opts = {}) => {
      const index = calls.length;
      calls.push({ prompt, opts, phase: opts.phase ?? phases.at(-1) ?? null });
      return settle(() => respond({ prompt, opts, index }));
    },
    parallel: (thunks) => Promise.all(thunks.map((thunk) => settle(thunk))),
    pipeline: (items, ...stages) =>
      Promise.all(
        items.map((item, index) =>
          settle(() =>
            stages.reduce(
              (prev, stage) => prev.then((value) => stage(value, item, index)),
              Promise.resolve(item),
            ),
          ),
        ),
      ),
    phase: (title) => {
      phases.push(title);
    },
    log: (message) => {
      logs.push(message);
    },
    args,
    budget: {
      total: budget,
      spent: () => 0,
      remaining: () => budget ?? Number.POSITIVE_INFINITY,
    },
    workflow: async (nameOrRef, childArgs) => {
      if (!workflow) throw new Error("workflow() called without a fake");
      return workflow(nameOrRef, childArgs);
    },
  };

  const context = vm.createContext({ __meta });
  vm.runInContext(
    `{
      const forbid = (name) => () => { throw new Error(name + " is not available in workflow scripts"); };
      Date = new Proxy(Date, {
        construct: (target, a) => a.length ? new target(...a) : forbid("new Date()")(),
        apply: forbid("Date()"),
      });
      Date.now = forbid("Date.now()");
      Math.random = forbid("Math.random()");
    }`,
    context,
  );
  const wrapper = `(async function (${HOOKS.join(", ")}) {\n${body}\n})`;
  const fn = new vm.Script(wrapper, {
    filename: pathToFileURL(file).href,
    lineOffset: -1,
  }).runInContext(context);
  const result = await fn(...HOOKS.map((name) => hooks[name]));
  // Values built in the script context: cloned into this realm for assertions.
  return {
    result: structuredClone(result),
    meta: structuredClone(__meta.value),
    calls,
    phases,
    logs,
  };
}
