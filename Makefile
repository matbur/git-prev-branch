.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

# Version stamped into the binary (see `version` in cmd/git-prev-branch):
# the newest vX.Y.Z tag plus distance, the bare commit when no tag is known
# (a fresh CI checkout), or "dev" outside a repository. Override per build:
# make build VERSION=v1.2.3.
VERSION ?= $(shell git describe --tags --match 'v[0-9]*' --always --dirty 2>/dev/null || echo dev)

.PHONY: build
build: ## Build the git-prev-branch binary into the repository root
	go build -ldflags "-X main.version=$(VERSION)" -o git-prev-branch ./cmd/git-prev-branch

.PHONY: build-dist
build-dist: ## Build dist/git-prev-branch for the current GOOS/GOARCH, trimmed and stripped
	@mkdir -p dist
	@out=dist/git-prev-branch; \
	if [ "$$GOOS" = "windows" ]; then out=$$out.exe; fi; \
	go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o "$$out" ./cmd/git-prev-branch

# The release axes, in one place so the CI cross-compile gate and the release
# packager cannot drift apart. RELEASE_TARGETS is their product; override any
# of the three on the command line (e.g. RELEASE_GOOSES=linux).
RELEASE_GOOSES ?= linux darwin windows
RELEASE_GOARCHES ?= amd64 arm64
RELEASE_TARGETS ?= $(foreach goos,$(RELEASE_GOOSES),$(foreach goarch,$(RELEASE_GOARCHES),$(goos)/$(goarch)))

.PHONY: release-target-matrix
release-target-matrix: ## Print RELEASE_TARGETS as a JSON matrix for the CI build job
	@python3 -c "import json; print(json.dumps([{'goos': t.split('/')[0], 'goarch': t.split('/')[1]} for t in '$(RELEASE_TARGETS)'.split()]))"

# Each asset keeps its os/arch in the name: gh release create 404s on
# duplicate file names.
.PHONY: release-assets
release-assets: ## Build and package every RELEASE_TARGETS binary into release-assets/ (needs VERSION)
	@mkdir -p release-assets
	@for target in $(RELEASE_TARGETS); do \
		goos="$${target%/*}"; \
		goarch="$${target#*/}"; \
		echo "building $$goos/$$goarch"; \
		GOOS="$$goos" GOARCH="$$goarch" $(MAKE) --no-print-directory build-dist VERSION="$(VERSION)"; \
		if [ "$$goos" = "windows" ]; then \
			mv dist/git-prev-branch.exe "release-assets/git-prev-branch-$$goos-$$goarch.exe"; \
		else \
			tar -C dist -czf "release-assets/git-prev-branch-$$goos-$$goarch.tar.gz" git-prev-branch; \
		fi; \
	done

.PHONY: check
check: fmt-check vet test check-readme test-scripts ## Run every check that needs only Go and Python

# Flags shared by `make test` and `make test-race`: -count=1 defeats the go
# test result cache (the e2e suite builds a binary and shells out to git),
# -parallel caps concurrent t.Parallel subtests, -shuffle=on catches tests
# that only pass in declaration order, -timeout stops a hung suite.
# Override for one run: make test TESTFLAGS="-count=1 -v".
TESTFLAGS ?= -count=1 -parallel=4 -shuffle=on -timeout=5m

.PHONY: test
test: ## Run the Go test suite (TESTFLAGS overrides the default flags)
	go test $(TESTFLAGS) ./...

.PHONY: test-race
test-race: ## Run the Go test suite under the race detector
	go test -race $(TESTFLAGS) ./...

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

.PHONY: lint
lint: ## Run golangci-lint (needs golangci-lint v2 installed)
	golangci-lint run ./...

.PHONY: check-readme
check-readme: ## Verify README.pl.md is in sync with README.md
	python3 scripts/check_readme_sync.py

.PHONY: test-scripts
test-scripts: ## Run the self-tests for the scripts/ tooling
	python3 scripts/test_check_readme_sync.py && python3 scripts/test_next_tag.py && python3 scripts/test_update_homebrew_formula.py

.PHONY: next-tag
next-tag: ## Print the vX.Y.Z tag to create after the next merge to main
	@python3 scripts/next_tag.py
