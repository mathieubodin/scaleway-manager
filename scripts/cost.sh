#!/usr/bin/env bash
# Compute the token cost of one piece of work from Claude Code transcripts.
# Usage: scripts/cost.sh <issue>
# Reads _bmad-output/work/issue-<n>-*/.memlog.md, writes cost.json next to it,
# prints the Tokens value (input + output + cache creation, attributed only).
# Format and rules: docs/token-cost-tracking.md.
set -euo pipefail

issue=${1:-}
[[ $issue =~ ^[1-9][0-9]*$ ]] || { echo "Error: issue must be a number, got '$issue'" >&2; exit 1; }
root=$(git rev-parse --show-toplevel)
cd "$root"

shopt -s nullglob
dirs=(_bmad-output/work/issue-"$issue"-*/)
[ ${#dirs[@]} -eq 1 ] || { echo "Error: expected one _bmad-output/work/issue-$issue-*/ folder, found ${#dirs[@]}" >&2; exit 1; }
work=${dirs[0]%/}
memlog=$work/.memlog.md
[ -f "$memlog" ] || { echo "Error: $memlog not found" >&2; exit 1; }
out=$work/cost.json

# Transcripts of this repository: ~/.claude/projects/<root path with / and . replaced by ->.
transcripts=${COST_TRANSCRIPTS_DIR:-$HOME/.claude/projects/$(printf '%s' "$root" | tr '/.' '--')}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# UTC ISO 8601 timestamp to epoch seconds, fractional part kept.
epoch_def='def epoch: . as $t
  | ($t | sub("\\.[0-9]+"; "") | fromdateiso8601)
    + ([$t | capture("\\.(?<f>[0-9]+)Z$")? | .f] | first | if . then tonumber / pow(10; length) else 0 end);'

# Attribution events of every memlog, as JSON: {memlog, line, kind, step, session, run, at, epoch,
# since, sinceEpoch, route}.
parse_events() {
  jq -R --arg memlog "$1" "$epoch_def"'
    [., input_line_number] as [$text, $line]
    | $text | capture("^- \\(event\\) (?<kind>route|step-start|step-end|workflow-run|session)(?<rest>( [a-z]+=[^ ]+)*)\\s*$")?
    | . as $e
    | ($e.rest | [scan(" ([a-z]+)=([^ ]+)") | {key: .[0], value: .[1]}] | from_entries
       | with_entries(select(.key | IN("route", "step", "session", "run", "at", "since")))) as $kv
    | $kv + {memlog: $memlog, line: $line, kind: $e.kind}
    | if .at then
        if (.at | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?Z$"))
        then .epoch = (.at | epoch)
        else {bad: "\($memlog):\($line): at must be UTC, like 2026-10-18T10:00:00Z, got \(.at)"} end
      else . end
    | if .since and (.bad | not) then
        if (.since | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?Z$"))
        then .sinceEpoch = (.since | epoch)
        else {bad: "\($memlog):\($line): since must be UTC, like 2026-10-18T10:00:00Z, got \(.since)"} end
      else . end
  ' "$1"
}
# A malformed line is returned as {bad}, not raised: jq 1.7 exits 0 on an error raised
# after a "?" operator, which would let a wrong Tokens value through.
for m in _bmad-output/work/*/.memlog.md; do parse_events "$m"; done | jq -s . > "$tmp/events.json"
bad=$(jq -r '.[] | select(.bad) | .bad' "$tmp/events.json")
[ -z "$bad" ] || { echo "Error: $bad" >&2; exit 1; }

# Sessions named by this memlog.
mapfile -t sessions < <(jq -r --arg m "$memlog" '[.[] | select(.memlog == $m and .session) | .session] | unique[]' "$tmp/events.json")
printf '%s\n' "${sessions[@]}" | jq -R . | jq -s 'map(select(. != ""))' > "$tmp/named.json"

[ -f "$out" ] && cp "$out" "$tmp/previous.json" || echo '{}' > "$tmp/previous.json"
echo '[]' > "$tmp/messages.json"
present=()
for s in "${sessions[@]}"; do
  main=$transcripts/$s.jsonl
  [ -f "$main" ] || continue
  present+=("$s")
  files=("$main" "$transcripts/$s"/subagents/agent-*.jsonl "$transcripts/$s"/subagents/workflows/*/agent-*.jsonl)
  # One record per API message: lines repeat per content block with the same id and usage.
  jq -n --arg s "$s" "$epoch_def"'
    [inputs
     | select(.type == "assistant" and .message.usage and .message.id)
     | {id: .message.id, session: $s, model: .message.model,
        epoch: (.timestamp | epoch),
        run: (input_filename | capture("/subagents/workflows/(?<r>[^/]+)/").r? // null),
        u: {input: (.message.usage.input_tokens // 0), output: (.message.usage.output_tokens // 0),
            cacheCreation: (.message.usage.cache_creation_input_tokens // 0),
            cacheRead: (.message.usage.cache_read_input_tokens // 0)}}]
    | unique_by(.id)' "${files[@]}" > "$tmp/m.json"
  jq -s 'add' "$tmp/messages.json" "$tmp/m.json" > "$tmp/m2.json" && mv "$tmp/m2.json" "$tmp/messages.json"
done
printf '%s\n' "${present[@]}" | jq -R . | jq -s 'map(select(. != ""))' > "$tmp/present.json"

jq -n --arg m "$memlog" --argjson issue "$issue" --arg now "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --slurpfile ev "$tmp/events.json" --slurpfile msgs "$tmp/messages.json" \
  --slurpfile prev "$tmp/previous.json" --slurpfile present "$tmp/present.json" \
  --slurpfile named "$tmp/named.json" '
  $ev[0] as $events | $prev[0] as $prev | $present[0] as $present | $named[0] as $named

  # Step windows of every memlog: a step-start ends at the next step-start or step-end
  # of the same session, in any memlog, so two works never share a moment.
  | ([$events | map(select(.kind == "step-start" or .kind == "step-end"))
      | group_by(.session)[] | sort_by(.epoch) | . as $l
      | range(length) as $i | select($l[$i].kind == "step-start")
      | {memlog: $l[$i].memlog, session: $l[$i].session, step: $l[$i].step,
         from: $l[$i].epoch, to: ($l[$i + 1].epoch // infinite),
         since: ($l[$i].sinceEpoch // -infinite)}]) as $windows
  | [$events[] | select(.kind == "session")] as $wholes
  | [$events[] | select(.kind == "workflow-run")] as $runs

  # Step of one message for this work: a step name, "unattributed", or "#other" (another work).
  | def mine_or_other: if . == null then null elif .memlog == $m then .step else "#other" end;
  def step_of($x):
      ($wholes | map(select(.session == $x.session)) | sort_by(.memlog != $m) | first | mine_or_other) //
      (if $x.run then $runs | map(select(.run == $x.run)) | sort_by(.memlog != $m) | first | mine_or_other else null end) //
      ($windows | map(select(.session == $x.session and .from <= $x.epoch and $x.epoch < .to)) | first | mine_or_other) //
      # Outside every window: before a triage, and not before its "since", it belongs to the
      # work that triage creates.
      ($windows | map(select(.session == $x.session and .from > $x.epoch)) | min_by(.from)
       | if . and .step == "triage" and .since <= $x.epoch then mine_or_other else null end) //
      "unattributed";

  def addu: reduce .[] as $x ({}; .[$x.model] |= (. // {input: 0, output: 0, cacheCreation: 0, cacheRead: 0}
      | .input += $x.u.input | .output += $x.u.output
      | .cacheCreation += $x.u.cacheCreation | .cacheRead += $x.u.cacheRead));
  def merge: reduce .[] as $o ({}; reduce ($o | to_entries[]) as $e (.;
      .[$e.key] |= (. // {input: 0, output: 0, cacheCreation: 0, cacheRead: 0}
        | .input += $e.value.input | .output += $e.value.output
        | .cacheCreation += $e.value.cacheCreation | .cacheRead += $e.value.cacheRead)));

  ($msgs[0] | map(. + {step: step_of(.)}) | map(select(.step != "#other"))) as $mm

  # Recomputed for sessions whose main transcript is present; kept as they were for the
  # other sessions still named by the memlog; dropped for sessions no longer named.
  | ([$present[] as $s
      | {key: $s, value: {computedAt: $now,
          steps: ($mm | map(select(.session == $s and .run == null and .step != "unattributed"))
                  | group_by(.step) | map({key: .[0].step, value: addu}) | from_entries),
          unattributed: ($mm | map(select(.session == $s and .step == "unattributed")) | addu)}}]
     | from_entries) as $sessions
  | ($mm | map(select(.run != null and .step != "unattributed")) | group_by(.run)
     | map({key: .[0].run, value: {session: .[0].session, step: .[0].step, computedAt: $now, models: addu}})
     | from_entries) as $newruns
  | def kept: with_entries(select((.value.session // .key) as $s
      | ($named | index($s)) and ($present | index($s) | not)));
  (($prev.sessions // {}) | with_entries(.value.session = .key) | kept | map_values(del(.session))) + $sessions
  | . as $S
  | (($prev.runs // {}) | kept) + $newruns
  | . as $R

  | ([($S[] | .steps | to_entries[] | {step: .key, models: .value}),
      ($R[] | {step, models})]
     | group_by(.step) | map({key: .[0].step, value: (map(.models) | merge)}) | from_entries) as $steps
  | {issue: $issue,
     route: ([$events[] | select(.memlog == $m and .kind == "route") | .route] | last),
     tokens: ([$steps[][] | .input + .output + .cacheCreation] | add // 0),
     totals: {steps: $steps,
              models: ([$steps[]] | merge),
              unattributed: ([$S[] | .unattributed] | merge)},
     sessions: $S, runs: $R}
' > "$tmp/cost.json"

mv "$tmp/cost.json" "$out"
jq -r '.tokens' "$out"
