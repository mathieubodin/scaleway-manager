#!/usr/bin/env bash
# Update an issue's item in the Project v2 board of the process (owner and number below).
# Usage:
#   scripts/project-item.sh add <issue>             add the issue to the board (no-op if there)
#   scripts/project-item.sh status <issue> <status> set Status (Backlog, Triage, Define, In Progress, Review)
#   scripts/project-item.sh tokens <issue> <value>  set the Tokens number field
# Done is never set here: the Project built-in workflows set it (docs/development-process.md).
# Field and option ids are looked up by name at each run.
set -euo pipefail

OWNER=mathieubodin
REPO=scaleway-manager
PROJECT=4

cmd=${1:-}
issue=${2:-}
value=${3:-}
usage() { echo "Usage: scripts/project-item.sh add|status|tokens <issue> [value]" >&2; exit 2; }
[[ $issue =~ ^[1-9][0-9]*$ ]] || usage

project_id=$(gh project view "$PROJECT" --owner "$OWNER" --format json --jq .id)

# Prints the item id of the issue in the board, empty when absent.
item_id() {
  gh api graphql -F n="$issue" -f owner="$OWNER" -f repo="$REPO" -f query='
    query($owner: String!, $repo: String!, $n: Int!) {
      repository(owner: $owner, name: $repo) {
        issue(number: $n) { projectItems(first: 20) { nodes { id project { number } } } }
      }
    }' --jq ".data.repository.issue.projectItems.nodes[] | select(.project.number == $PROJECT) | .id"
}

ensure_item() {
  local id
  id=$(item_id)
  if [ -z "$id" ]; then
    id=$(gh project item-add "$PROJECT" --owner "$OWNER" \
      --url "https://github.com/$OWNER/$REPO/issues/$issue" --format json --jq .id)
  fi
  echo "$id"
}

field() { # name -> JSON of the field
  gh project field-list "$PROJECT" --owner "$OWNER" --format json \
    | jq -ce --arg n "$1" '.fields[] | select(.name == $n)' \
    || { echo "Error: no field '$1' in project $PROJECT" >&2; exit 1; }
}

case $cmd in
  add)
    [ -z "$value" ] || usage
    echo "item $(ensure_item)"
    ;;
  status)
    case $value in
      Backlog|Triage|Define|"In Progress"|Review) ;;
      *) echo "Error: status must be Backlog, Triage, Define, In Progress or Review, got '$value'" >&2; exit 2 ;;
    esac
    f=$(field Status)
    option=$(jq -re --arg v "$value" '.options[] | select(.name == $v) | .id' <<<"$f")
    id=$(ensure_item)
    gh project item-edit --project-id "$project_id" --id "$id" \
      --field-id "$(jq -r .id <<<"$f")" --single-select-option-id "$option" >/dev/null
    echo "#$issue Status: $value"
    ;;
  tokens)
    [[ $value =~ ^[0-9]+$ ]] || { echo "Error: tokens must be a whole number, got '$value'" >&2; exit 2; }
    f=$(field Tokens)
    id=$(ensure_item)
    gh project item-edit --project-id "$project_id" --id "$id" \
      --field-id "$(jq -r .id <<<"$f")" --number "$value" >/dev/null
    echo "#$issue Tokens: $value"
    ;;
  *) usage ;;
esac
