// Story 3 of #41 (Helm chart): the lint-chart make target, its guards, its prek hook,
// its CI job and the chart packaging. The chart content itself is asserted by the
// helm-unittest suites in charts/scaleway-manager/tests/.
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import {
  chmodSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  rmSync,
  statSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { delimiter, join } from "node:path";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";

const ROOT = fileURLToPath(new URL("../..", import.meta.url));
const CHART = join(ROOT, "charts/scaleway-manager");
const VERSION_HINT =
  "Error: helm 4.3.0 required. Install with: https://helm.sh/docs/intro/install/";

const dir = mkdtempSync(join(tmpdir(), "lint-chart-"));
after(() => rmSync(dir, { recursive: true, force: true }));

const read = (path) => readFileSync(join(ROOT, path), "utf8");

// A fake helm: answers `version` and `plugin list` from the environment, logs every call.
const fakeBin = join(dir, "fake-bin");
mkdirSync(fakeBin);
writeFileSync(
  join(fakeBin, "helm"),
  `#!/bin/sh
echo "$*" >> "$FAKE_HELM_LOG"
case "$1" in
  version) printf '%s' "$FAKE_HELM_VERSION" ;;
  plugin) printf 'NAME\\tVERSION\\tTYPE\\tAPIVERSION\\tPROVENANCE\\tSOURCE\\n'
          [ -n "$FAKE_UNITTEST_VERSION" ] && printf 'unittest\\t%s\\tcli/v1\\tlegacy\\tunknown\\tunknown\\n' "$FAKE_UNITTEST_VERSION"
          exit 0 ;;
  *) exit 0 ;;
esac
`,
);
chmodSync(join(fakeBin, "helm"), 0o755);

// A PATH holding every executable of the current PATH except helm.
const noHelmBin = join(dir, "no-helm-bin");
mkdirSync(noHelmBin);
for (const d of (process.env.PATH ?? "").split(delimiter)) {
  let names = [];
  try {
    names = readdirSync(d);
  } catch {
    continue;
  }
  for (const name of names) {
    if (name === "helm" || existsSync(join(noHelmBin, name))) continue;
    try {
      if (statSync(join(d, name)).isFile())
        symlinkSync(join(d, name), join(noHelmBin, name));
    } catch {
      // unreadable entry: skipped
    }
  }
}

let runCount = 0;
const runMake = (
  args,
  { path, helmVersion = "", unittestVersion = "" } = {},
) => {
  runCount += 1;
  const log = join(dir, `helm-${runCount}.log`);
  writeFileSync(log, "");
  const env = {
    ...process.env,
    PATH: path ?? process.env.PATH,
    FAKE_HELM_LOG: log,
    FAKE_HELM_VERSION: helmVersion,
    FAKE_UNITTEST_VERSION: unittestVersion,
  };
  delete env.MAKEFLAGS;
  delete env.MAKELEVEL;
  const r = spawnSync("make", ["--no-print-directory", ...args], {
    cwd: ROOT,
    env,
    encoding: "utf8",
  });
  return {
    status: r.status,
    out: `${r.stdout}${r.stderr}`,
    calls: readFileSync(log, "utf8"),
  };
};
const withFake = `${fakeBin}${delimiter}${process.env.PATH}`;

// AC2: wrong helm, missing helm, missing or wrong plugin → exit 1 with the matching hint.

test("lint-chart fails with the helm hint when helm is missing", () => {
  const r = runMake(["lint-chart"], { path: noHelmBin });
  assert.notEqual(r.status, 0, r.out);
  assert.ok(r.out.includes(VERSION_HINT), r.out);
});

test("lint-chart fails with the helm hint when helm is not exactly v4.3.0", () => {
  for (const v of ["v4.2.0", "v4.3.1", "v4.3.0-rc.1", "4.3.0", "v3.19.0"]) {
    const r = runMake(["lint-chart"], {
      path: withFake,
      helmVersion: v,
      unittestVersion: "1.2.1",
    });
    assert.notEqual(r.status, 0, `${v}: ${r.out}`);
    assert.ok(r.out.includes(VERSION_HINT), `${v}: ${r.out}`);
    assert.doesNotMatch(
      r.calls,
      /^(lint|unittest)\b/m,
      `${v}: chart checked anyway`,
    );
  }
});

const assertPluginHint = (r, label) => {
  assert.notEqual(r.status, 0, `${label}: ${r.out}`);
  assert.ok(
    !r.out.includes(VERSION_HINT),
    `${label}: helm hint instead of plugin hint`,
  );
  assert.match(r.out, /helm-unittest/, label);
  assert.match(r.out, /1\.2\.1/, label);
  assert.match(r.out, /helm plugin install /, label);
  assert.doesNotMatch(
    r.calls,
    /^(lint|unittest)\b/m,
    `${label}: chart checked anyway`,
  );
};

test("lint-chart fails with the helm-unittest hint when the plugin is missing", () => {
  const r = runMake(["lint-chart"], { path: withFake, helmVersion: "v4.3.0" });
  assertPluginHint(r, "no plugin");
});

test("lint-chart fails with the helm-unittest hint when the plugin is not exactly 1.2.1", () => {
  for (const v of ["1.2.0", "1.2.10", "1.3.1"]) {
    const r = runMake(["lint-chart"], {
      path: withFake,
      helmVersion: "v4.3.0",
      unittestVersion: v,
    });
    assertPluginHint(r, `plugin ${v}`);
  }
});

// AC1: lint-chart runs helm lint with ci/test-values.yaml, then helm unittest, and joins make check.

test("lint-chart runs helm lint with ci/test-values.yaml, then helm unittest", () => {
  const r = runMake(["lint-chart"], {
    path: withFake,
    helmVersion: "v4.3.0",
    unittestVersion: "1.2.1",
  });
  assert.equal(r.status, 0, r.out);
  const calls = r.calls.split("\n");
  const lint = calls.indexOf(
    "lint charts/scaleway-manager --values charts/scaleway-manager/ci/test-values.yaml",
  );
  const unit = calls.indexOf("unittest charts/scaleway-manager");
  assert.ok(lint >= 0, r.calls);
  assert.ok(unit > lint, r.calls);
});

test("print-helm-version prints 4.3.0 and stays out of make help", () => {
  const r = spawnSync("make", ["-s", "print-helm-version"], {
    cwd: ROOT,
    encoding: "utf8",
  });
  assert.equal(r.status, 0, r.stderr);
  assert.equal(r.stdout.trim(), "4.3.0");
  const makefile = read("Makefile");
  assert.match(makefile, /^HELM_VERSION := 4\.3\.0$/m);
  assert.match(makefile, /^HELM_UNITTEST_VERSION := 1\.2\.1$/m);
  assert.doesNotMatch(makefile, /^print-helm-version:.*##/m);
  const help = spawnSync("make", ["help"], { cwd: ROOT, encoding: "utf8" });
  assert.match(help.stdout, /^\s+lint-chart\s/m);
  assert.doesNotMatch(help.stdout, /print-helm-version/);
});

test("make check runs lint-chart", () => {
  const line = read("Makefile").match(/^check:([^#\n]*)/m);
  assert.ok(line, "no check target");
  assert.ok(line[1].split(/\s+/).includes("lint-chart"), line[0]);
});

test("CI job lint-chart installs helm from print-helm-version and the pinned plugin, then runs make lint-chart", () => {
  const ci = read(".github/workflows/pr-checks.yml");
  const m = ci.match(/^ {2}lint-chart:\n((?: {4}.*\n|\s*\n)+)/m);
  assert.ok(m, "no lint-chart job");
  const job = m[1];
  assert.match(
    job,
    /uses: actions\/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7\.0\.1\n\s+with:\n\s+persist-credentials: false/,
  );
  assert.match(job, /make -s print-helm-version/);
  assert.match(
    job,
    /uses: azure\/setup-helm@9bc31f4ebc9c6b171d7bfbaa5d006ae7abdb4310 # v5\.0\.1/,
  );
  assert.match(
    job,
    /version: v\$\{\{ steps\.[A-Za-z0-9_-]+\.outputs\.[A-Za-z0-9_-]+ \}\}/,
  );
  assert.match(job, /helm plugin install /);
  assert.match(job, /1\.2\.1|HELM_UNITTEST_VERSION/);
  assert.match(job, /run: make lint-chart\s*$/m);
  const hint = runMake(["lint-chart"], {
    path: withFake,
    helmVersion: "v4.3.0",
  }).out;
  const install = job.match(/(helm plugin install [^\n]+)/)[1].trim();
  const hintInstall = hint.match(/(helm plugin install [^\n]+)/);
  assert.ok(hintInstall, hint);
  // CI may reach the version through an expression; the hint prints it expanded.
  const normalize = (cmd) =>
    cmd
      .replace(/\$\{\{[^}]*\}\}|\$\([A-Z_]+\)/g, "1.2.1")
      .replace(/["'`.]+$/, "")
      .trim();
  assert.equal(
    normalize(hintInstall[1]),
    normalize(install),
    "CI and the hint install the plugin with the same command",
  );
});

// AC4: prek runs lint-chart on charts/ and check-yaml skips the templates.

const inlineTable = (toml, id) => {
  const start = toml.indexOf(`{ id = "${id}"`);
  assert.ok(start >= 0, `no prek hook ${id}`);
  return toml.slice(start, toml.indexOf("}", start) + 1);
};
const tomlString = (table, key) => {
  const m = table.match(new RegExp(`\\b${key} = "((?:[^"\\\\]|\\\\.)*)"`));
  return m ? m[1].replace(/\\\\/g, "\\") : undefined;
};

test("prek: check-yaml excludes chart templates only", () => {
  const hook = inlineTable(read("prek.toml"), "check-yaml");
  const exclude = tomlString(hook, "exclude");
  assert.equal(exclude, "^charts/[^/]+/templates/");
  const re = new RegExp(exclude);
  assert.ok(re.test("charts/scaleway-manager/templates/deployment.yaml"));
  assert.ok(!re.test("charts/scaleway-manager/values.yaml"));
  assert.ok(!re.test("charts/scaleway-manager/tests/deployment_test.yaml"));
  assert.ok(!re.test(".github/workflows/pr-checks.yml"));
  assert.match(hook, /"--allow-multiple-documents"/);
});

test("prek: local hook lint-chart runs make lint-chart on charts/", () => {
  const hook = inlineTable(read("prek.toml"), "lint-chart");
  assert.equal(tomlString(hook, "name"), "Chart lint");
  assert.equal(tomlString(hook, "entry"), "make lint-chart");
  assert.equal(tomlString(hook, "language"), "system");
  assert.equal(tomlString(hook, "files"), "^charts/");
  assert.match(hook, /pass_filenames = false/);
  const re = new RegExp(tomlString(hook, "files"));
  assert.ok(re.test("charts/scaleway-manager/templates/deployment.yaml"));
  assert.ok(re.test("charts/scaleway-manager/values.yaml"));
  assert.ok(!re.test("Makefile"));
});

// Chart shape fixed by the story: Chart.yaml, one template per object kind, no schema.

test("Chart.yaml holds the fixed identity", () => {
  const chart = read("charts/scaleway-manager/Chart.yaml");
  assert.match(chart, /^apiVersion: v2$/m);
  assert.match(chart, /^name: scaleway-manager$/m);
  assert.match(chart, /^type: application$/m);
  assert.match(chart, /^version: 0\.0\.0$/m);
  assert.match(chart, /^appVersion: "0\.0\.0"$/m);
});

test("the chart has exactly the five object templates and no values.schema.json", () => {
  const yamls = readdirSync(join(CHART, "templates"))
    .filter((f) => f.endsWith(".yaml") || f.endsWith(".yml"))
    .sort();
  assert.deepEqual(yamls, [
    "clusterroles.yaml",
    "deployment.yaml",
    "httproute.yaml",
    "service.yaml",
    "serviceaccount.yaml",
  ]);
  assert.ok(existsSync(join(CHART, "templates/_helpers.tpl")));
  assert.ok(!existsSync(join(CHART, "values.schema.json")));
  assert.ok(existsSync(join(CHART, "ci/test-values.yaml")));
});

// AC3: the packaged chart holds no tests/ or ci/ file.

test(".helmignore excludes tests/ and ci/", () => {
  const lines = read("charts/scaleway-manager/.helmignore")
    .split("\n")
    .map((l) => l.trim());
  assert.ok(
    lines.some((l) => /^\/?tests\/?(\*\*?)?$/.test(l)),
    "tests/ not ignored",
  );
  assert.ok(
    lines.some((l) => /^\/?ci\/?(\*\*?)?$/.test(l)),
    "ci/ not ignored",
  );
});

const realHelm = spawnSync("helm", ["version", "--template", "{{.Version}}"], {
  encoding: "utf8",
});
test("helm package holds no tests/ or ci/ file", {
  skip:
    realHelm.status === 0 && realHelm.stdout === "v4.3.0"
      ? false
      : "helm v4.3.0 not on PATH",
}, () => {
  const out = join(dir, "package");
  mkdirSync(out);
  const pkg = spawnSync("helm", ["package", CHART, "--destination", out], {
    encoding: "utf8",
  });
  assert.equal(pkg.status, 0, pkg.stderr);
  const archive = join(out, "scaleway-manager-0.0.0.tgz");
  const list = spawnSync("tar", ["-tzf", archive], { encoding: "utf8" });
  assert.equal(list.status, 0, list.stderr);
  const entries = list.stdout.split("\n").filter(Boolean);
  assert.ok(entries.includes("scaleway-manager/Chart.yaml"), list.stdout);
  assert.ok(
    entries.includes("scaleway-manager/templates/deployment.yaml"),
    list.stdout,
  );
  assert.deepEqual(
    entries.filter((e) => /^scaleway-manager\/(tests|ci)\//.test(e)),
    [],
  );
});
