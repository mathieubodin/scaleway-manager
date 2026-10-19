#!/usr/bin/env bash
# Aggregate every _bmad-output/work/*/cost.json as Markdown tables.
# Usage: scripts/cost-report.sh. Format: docs/token-cost-tracking.md.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
shopt -s nullglob
files=(_bmad-output/work/*/cost.json)
[ ${#files[@]} -gt 0 ] || { echo "No cost.json under _bmad-output/work/"; exit 0; }

jq -rs '
  def sum4: reduce .[] as $x ({input: 0, output: 0, cacheCreation: 0, cacheRead: 0};
    .input += $x.input | .output += $x.output | .cacheCreation += $x.cacheCreation | .cacheRead += $x.cacheRead);
  def row($a): "| " + ($a | map(tostring) | join(" | ")) + " |";
  def cols($t): [$t.input, $t.output, $t.cacheCreation, $t.cacheRead, ($t.input + $t.output + $t.cacheCreation)];

  "## Per work", "",
  "| Issue | Route | Tokens | Unattributed tokens | Cache read (attributed) |",
  "| --- | --- | --- | --- | --- |",
  (sort_by(.issue)[] | row([.issue, (.route // "-"), .tokens,
     ([.totals.unattributed[] | .input + .output + .cacheCreation] | add // 0),
     ([.totals.steps[][] | .cacheRead] | add // 0)])),
  "",
  "## Per route and step", "",
  "| Route | Step | Works | Input | Output | Cache creation | Cache read | Tokens |",
  "| --- | --- | --- | --- | --- | --- | --- | --- |",
  ([.[] | (.route // "-") as $r | .issue as $i | .totals.steps | to_entries[]
    | {route: $r, step: .key, issue: $i, t: ([.value[]] | sum4)}]
   | group_by([.route, .step])[]
   | row([.[0].route, .[0].step, (map(.issue) | unique | length)] + cols(map(.t) | sum4)))
' "${files[@]}"
