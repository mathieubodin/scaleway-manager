SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

MARKDOWNLINT ?= node_modules/.bin/markdownlint-cli2
FROZEN_MANIFEST := frozen-tests.sha256

# Mutation testing on the code changed by the task: 1 (default) or 0.
MUTATION ?= 1
ifneq ($(filter-out 0 1,$(MUTATION)),)
$(error MUTATION must be 0 or 1, got '$(MUTATION)')
endif
export MUTATION

# Reference branch for check-migration. ISSUE and BASE reach the scripts through the
# environment (make exports command-line variables), never pasted into the shell.
BASE ?= origin/main
export BASE

.PHONY: help lint-docs fix-docs freeze check-frozen check-markdownlint cost cost-report check-node lint-workflows fix-workflows test-workflows check-migration evidence

help: ## List the targets
	@grep -hE '^[a-z-]+:.*## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*## "} {printf "  %-14s %s\n", $$1, $$2}'
	@echo ""
	@echo "Variables: MUTATION=$(MUTATION) (0 switches mutation testing off)"

check-markdownlint:
	@command -v $(MARKDOWNLINT) >/dev/null || { \
		echo "Error: $(MARKDOWNLINT) not found. Install with: npm ci"; \
		exit 1; }

lint-docs: check-markdownlint ## Lint Markdown (.markdownlint-cli2.yaml)
	$(MARKDOWNLINT)

fix-docs: check-markdownlint ## Fix Markdown in place, then lint
	$(MARKDOWNLINT) --fix

freeze: ## Record the sha256 of FILES="..." in frozen-tests.sha256 (new tests, or a change decided by the human)
	@[ -n "$(strip $(FILES))" ] || { echo "Error: FILES is empty. Usage: make freeze FILES=\"path ...\""; exit 1; }
	@for f in $(FILES); do [ -f "$$f" ] || { echo "Error: $$f is not a file"; exit 1; }; done
	@touch $(FROZEN_MANIFEST)
	@{ awk 'NR == FNR { keep[$$0]; next } !(substr($$0, 67) in keep)' \
			<(printf '%s\n' $(FILES)) $(FROZEN_MANIFEST); \
		sha256sum $(FILES); } | sort -k2 > $(FROZEN_MANIFEST).tmp
	@mv $(FROZEN_MANIFEST).tmp $(FROZEN_MANIFEST)
	@echo "Frozen: $(FILES)"

check-frozen: ## Fail if a frozen test file changed or disappeared
	@if [ -s $(FROZEN_MANIFEST) ]; then \
		sha256sum --check --strict --quiet $(FROZEN_MANIFEST) && echo "Frozen tests unchanged"; \
	else echo "No frozen tests"; fi

cost: ## Token cost of ISSUE=<n> into its work folder cost.json; prints Tokens
	@scripts/cost.sh "$${ISSUE:-}"

cost-report: ## Token cost of every work, per route and step (Markdown)
	@scripts/cost-report.sh

# Workflow scripts (.claude/workflows, harness fixtures) use a top-level `return`,
# which Biome cannot parse: oxlint and oxfmt check them. Biome checks the harness,
# the tests and the JSON files. See docs/development-process.md, "Workflow tooling".
NODE_BIN := node_modules/.bin
WORKFLOW_SCRIPTS := $(wildcard .claude/workflows/*.js tests/workflows/fixtures/*.js)

check-node:
	@[ -x $(NODE_BIN)/biome ] && [ -x $(NODE_BIN)/oxlint ] && [ -x $(NODE_BIN)/oxfmt ] && [ -x $(NODE_BIN)/c8 ] || { \
		echo "Error: Node tooling missing. Install with: npm ci"; \
		exit 1; }

lint-workflows: check-node ## Lint and format check: workflow scripts (oxlint, oxfmt), harness and JSON (Biome)
	$(if $(WORKFLOW_SCRIPTS),$(NODE_BIN)/oxlint --deny-warnings $(WORKFLOW_SCRIPTS))
	$(if $(WORKFLOW_SCRIPTS),$(NODE_BIN)/oxfmt --check $(WORKFLOW_SCRIPTS))
	$(NODE_BIN)/biome ci .

fix-workflows: check-node ## Fix lint and format in place, then lint
	-$(if $(WORKFLOW_SCRIPTS),$(NODE_BIN)/oxlint --fix $(WORKFLOW_SCRIPTS))
	-$(if $(WORKFLOW_SCRIPTS),$(NODE_BIN)/oxfmt $(WORKFLOW_SCRIPTS))
	-$(NODE_BIN)/biome check --write .
	@$(MAKE) --no-print-directory lint-workflows

test-workflows: check-node ## Run the workflow tests under the harness, with coverage (c8)
	$(NODE_BIN)/c8 node --test 'tests/workflows/**/*.test.js'

check-migration: ## Check docs/migrations/next/ISSUE.md: changed by the branch since BASE (origin/main), valid format
	@scripts/check-migration.sh "$${ISSUE:-}" "$$BASE"

evidence: ## Replay the checks, rewrite the evidence record of ISSUE=<n> (work folder evidence.md)
	@scripts/evidence.sh "$${ISSUE:-}"
