# Report of astonishment: an unexplained `reasoning_extraction` flag and a model switch

Date of the events: 2026-10-23, 22:34 to 22:41 UTC.
Session: `5d70359f-547d-44fd-9e0f-4d8f883010be`, project `scaleway-manager`, work on issues #62 then #47.
Evidence: `excerpt.jsonl`, next to this report, holds the eight log entries that matter (source lines 878, 879, 880, 1018, 1019, 1020, 1021 and 1047), each wrapped as `{"sourceLine": N, "entry": ...}`. The line numbers below are those of the full session log (1190 lines, taken on 2026-10-23 after the events), which is kept locally as `session-5d70359f-2026-10-23.jsonl` in this folder and is not committed (`.gitignore`)

## Summary

Twice in a few minutes the safeguards of `claude-opus-5-5` stopped a response of the assistant. The second stop carried the category `reasoning_extraction` and the explanation that the request "seems to violate Anthropic's Terms of Service restrictions on reverse engineering or duplicating model outputs". The session was then switched, without any action of the user, from `claude-opus-5-5` to `claude-opus-5`.

Nothing in the stopped responses relates to reverse engineering or to duplicating model outputs. The work in progress was routine development: Git branches, a Makefile, a Helm chart. We do not understand why the message was flagged.

## What happened, from the log

### 1. First stop, 22:34:51 (line 879)

The assistant had written one line of text and one `git switch main` command. The log holds this system line:

```json
{"type":"system","subtype":"informational","timestamp":"2026-10-23T22:34:51.174Z","content":"Opus 5.5's safeguards stopped the response above · continuing once with that noted"}
```

The log holds no refusal record with a category for this stop: the neighbouring lines (877, 878) have no `stop_details`. The reason of this first stop is unknown.

### 2. Second stop, 22:40:36 (lines 1018 and 1020)

Line 1018, the assistant message `msg_011CgKyRspHDhz6ksRTS49YC`, model `claude-opus-5-5`:

- `stop_reason`: `refusal`
- `stop_details.category`: `reasoning_extraction`
- `stop_details.explanation`: "This request was blocked as it seems to violate Anthropic's Terms of Service restrictions on reverse engineering or duplicating model outputs. To learn more, visit <https://www.anthropic.com/legal/commercial-terms>."
- `usage.output_tokens`: 50, `thinking_tokens`: 0
- request id: `req_011CgKyRsVCAKG8EjXfVV5eA`

The whole content of that message is one tool call, cut off in the middle of a path:

```json
[{"type":"tool_use","id":"toolu_01CuDiBxUFE6x8tLSZHwJha2","name":"Bash","input":{"command":"cd /home/user/dev/scaleway/github.com/sc"},"caller":{"type":"direct"}}]
```

It has no text and no thinking. The command was the start of a `cd` into the project directory (`.../scaleway-manager`) before reading a test file and a Makefile.

Line 1020, 0.5 s later:

```json
{"subtype":"informational","timestamp":"2026-10-23T22:40:36.666Z","content":"Opus 5.5's safeguards stopped the response above · continuing once with that noted"}
```

### 3. Model switch, 22:40:51 (line 1047)

```json
{"type":"system","subtype":"model_refusal_fallback","timestamp":"2026-10-23T22:40:51.982Z","level":"warning","trigger":"refusal","direction":"retry","scope":"session","originalModel":"claude-opus-5-5","fallbackModel":"claude-opus-5","requestId":"req_011CgKySof3ANW7humYkTNik","apiRefusalCategory":"reasoning_extraction","refusedUserMessageUuid":"8e31e3a9-20ff-4508-8470-1ce88bdb2b36"}
```

The message shown to the user (same line): "Opus 5.5's safeguards flagged this message. This sometimes happens with safe, normal conversations. Switched to Opus 5. You can learn more: <https://support.claude.com/en/articles/8106465> Details: `[reasoning_extraction]`".

The request id of the switch (`req_011CgKySof3ANW7humYkTNik`) differs from the request id of the refused response (`req_011CgKyRsVCAKG8EjXfVV5eA`). The log does not say how the two are related.

### 4. What the user then saw

The user then ran `/model` twice (Opus 5, then Sonnet 5.5, both saved as default for new sessions) and asked why the model had changed. The assistant first answered that it had not changed model, which was wrong: it did not know what had happened. The user's screenshot of the warning showed the cause, and the log lines above confirm it.

## What we do not understand

- The category `reasoning_extraction` and its explanation about duplicating model outputs do not match anything in the conversation: a Bash call that only started a `cd`, in a session about Git, a Makefile and a Helm chart.
- The flagged response was cut off after 50 output tokens with no text and no thinking, so there was almost nothing to classify. The classifier may have judged the context rather than the response; the log does not say.
- The log does not record the reason of the first stop (22:34:51).
- The support page "Our approach to user safety" (<https://support.claude.com/en/articles/8106465-our-approach-to-user-safety>), read on 2026-10-23, describes detection models and prompt filters, says that they can give false positives, and mentions neither a model switch after a flag nor the category `reasoning_extraction`.
- The switch changed the model of the whole session (`scope: session`) and was saved as a consequence of the flag, with no confirmation asked to the user.

## Consequences

- The response in progress was lost, and the assistant had to be asked again.
- The session ran on another model than the one chosen, until the user changed it.
- The assistant gave a wrong explanation to the user for lack of information, which cost trust and time.

## Note on the evidence files

The full log holds the whole conversation, including private instructions and extracts of other projects, so only `excerpt.jsonl` is committed. Both files are anonymized: the user's name, account email address, Git owner name and home directory are replaced by `user`, `example-owner` and `user@example.invalid`. Line numbers and the JSON structure are unchanged, and every line still parses as JSON.
