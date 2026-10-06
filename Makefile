.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the git-prev-branch binary into the repository root
	go build -o git-prev-branch ./cmd/git-prev-branch

.PHONY: build-dist
build-dist: ## Build dist/git-prev-branch for the current GOOS/GOARCH with -trimpath
	@mkdir -p dist
	@out=dist/git-prev-branch; \
	if [ "$$GOOS" = "windows" ]; then out=$$out.exe; fi; \
	go build -trimpath -o "$$out" ./cmd/git-prev-branch

.PHONY: test
test: ## Run the Go test suite
	go test ./...

.PHONY: fmt
fmt: ## Rewrite Go sources in place with gofmt
	gofmt -w .

.PHONY: fmt-check
fmt-check: ## Fail if any Go source is not gofmt-clean
	@files=$$(gofmt -l .); \
	if [ -n "$$files" ]; then \
		echo "gofmt needed on:"; \
		echo "$$files"; \
		exit 1; \
	fi

.PHONY: vet
vet: ## Run go vet over every package
	go vet ./...

.PHONY: next-tag
next-tag: ## Print the vX.Y.Z tag to create after the next merge to main
	@python3 scripts/next_tag.py

.PHONY: lint
lint: ## Run golangci-lint (needs golangci-lint v2 installed)
	golangci-lint run ./...

.PHONY: check
check: fmt-check vet test check-readme test-scripts ## Run every check that needs only Go and Python

.PHONY: check-readme
check-readme: ## Verify README.pl.md is in sync with README.md
	python3 scripts/check_readme_sync.py

.PHONY: test-scripts
test-scripts: ## Run the self-tests for the scripts/ tooling
	python3 scripts/test_check_readme_sync.py && python3 scripts/test_next_tag.py
