SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

MARKDOWNLINT ?= markdownlint-cli2
FROZEN_MANIFEST := frozen-tests.sha256

# Mutation testing on the code changed by the task: 1 (default) or 0.
MUTATION ?= 1
ifneq ($(filter-out 0 1,$(MUTATION)),)
$(error MUTATION must be 0 or 1, got '$(MUTATION)')
endif
export MUTATION

.PHONY: help lint-docs fix-docs freeze check-frozen check-markdownlint cost cost-report

help: ## List the targets
	@grep -hE '^[a-z-]+:.*## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*## "} {printf "  %-14s %s\n", $$1, $$2}'
	@echo ""
	@echo "Variables: MUTATION=$(MUTATION) (0 switches mutation testing off)"

check-markdownlint:
	@command -v $(MARKDOWNLINT) >/dev/null || { \
		echo "Error: $(MARKDOWNLINT) not found. Install with: npm install -g markdownlint-cli2"; \
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
	@[ -n "$(ISSUE)" ] || { echo "Error: ISSUE is empty. Usage: make cost ISSUE=<n>"; exit 1; }
	@scripts/cost.sh $(ISSUE)

cost-report: ## Token cost of every work, per route and step (Markdown)
	@scripts/cost-report.sh
