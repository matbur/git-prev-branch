.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: check
check: check-readme test-scripts ## Run every check (README sync + script self-tests)

.PHONY: check-readme
check-readme: ## Verify README.pl.md is in sync with README.md
	python3 scripts/check_readme_sync.py

.PHONY: test-scripts
test-scripts: ## Run the self-tests for the scripts/ tooling
	python3 scripts/test_check_readme_sync.py