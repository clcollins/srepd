unexport GOFLAGS

GOOS ?= linux
GOARCH ?= amd64
GOENV = GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=0 GOFLAGS=
GOPATH := $(shell go env GOPATH | awk -F: '{print $1}')
BIN_DIR := $(GOPATH)/bin
HOME ?= $(shell echo ~)

GOLANGCI_LINT_VERSION = v2.12.2
GORELEASER_VERSION = v2.8.2

# Ensure go modules are enabled:
export GO111MODULE = on
export GOPROXY = https://proxy.golang.org

# Disable CGO so that we always generate static binaries:
export CGO_ENABLED = 0

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the application
	@echo "Building the application..."
	goreleaser build --snapshot --clean --single-target

.PHONY: install
install: ## Install the application to $(GOPATH)/bin
	@echo "Installing the application..."
	go build -ldflags "-X github.com/clcollins/srepd/pkg/tui.GitSHA=$$(git rev-parse --short HEAD)" -o ${BIN_DIR}/srepd .

.PHONY: install-local
install-local: build ## Install the application locally to ~/.local/bin
	@echo "Installing the application locally..."
	cp dist/*/srepd $(HOME)/.local/bin/srepd

.PHONY: tidy
tidy: ## Tidy up go modules
	@echo "Tidying up go modules..."
	go mod tidy

.PHONY: test
test: ## Run unit tests
	@echo "Running tests..."
	go test ./... -v -count=1 $(TESTOPTS)

.PHONY: coverage
coverage: ## Generate test coverage report
	@echo "Generating test coverage report..."
	go test ./... -coverprofile=coverage.out -covermode=atomic
	@if [ ! -f coverage.out ]; then echo "Error: Coverage file not generated."; exit 1; fi
	@echo "Coverage summary:"
	go tool cover -func=coverage.out
	@if command -v codecov >/dev/null 2>&1; then \
		echo "Uploading coverage report to Codecov..."; \
		codecov -f coverage.out; \
	else \
		echo "Codecov CLI not found. Skipping upload."; \
	fi
	@rm -f coverage.out
	@echo "Test coverage report generation complete."

.PHONY: getlint
getlint: ## Install golangci-lint if not already installed
	@echo "Checking for golangci-lint..."
	@which golangci-lint >/dev/null 2>&1 || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION))

.PHONY: lint
lint: getlint ## Run golangci-lint
	@echo "Running golangci-lint..."
	$(BIN_DIR)/golangci-lint --version
	$(BIN_DIR)/golangci-lint run --timeout 5m

.PHONY: vet
vet: ## Run go vet to catch common mistakes
	@echo "Running go vet..."
	go vet ./...

.PHONY: fmt
fmt: ## Format the code
	@echo "Formatting the code..."
	gofmt -s -l -w cmd pkg

.PHONY: fmt-check
fmt-check: ## Check code formatting (CI-friendly, exits non-zero if unformatted)
	@echo "Checking code formatting..."
	@test -z "$$(gofmt -s -l cmd pkg)" || (echo "The following files are not formatted:"; gofmt -s -l cmd pkg; exit 1)

.PHONY: plan-check
plan-check: ## Check that a plan document exists for this branch
	@echo "Checking for plan document..."
	@MERGE_BASE=$$(git merge-base HEAD origin/main 2>/dev/null || echo ""); \
	if [ -z "$$MERGE_BASE" ]; then \
		echo "WARN: could not determine merge base with origin/main; skipping plan doc check"; \
		exit 0; \
	fi; \
	PLAN_FILES=$$(git diff --name-only --diff-filter=ACMR "$$MERGE_BASE"...HEAD -- 'docs/plans/*.md' 2>/dev/null || echo ""); \
	if [ -z "$$PLAN_FILES" ]; then \
		echo "ERROR: no plan document found in docs/plans/"; \
		echo ""; \
		echo "Every PR must include a plan document in docs/plans/."; \
		echo "See CONVENTIONS.md for the required format."; \
		echo ""; \
		echo "Create a file like: docs/plans/NNN-your-change.md"; \
		exit 1; \
	fi; \
	echo "Plan document(s) found:"; \
	echo "$$PLAN_FILES" | sed 's/^/  /'

.PHONY: readme-check
readme-check: ## Ensure README is updated when config/keys/flags change
	@echo "Checking if README update is needed..."
	@MERGE_BASE=$$(git merge-base HEAD origin/main 2>/dev/null || echo ""); \
	if [ -z "$$MERGE_BASE" ]; then exit 0; fi; \
	CHANGED=$$(git diff --name-only "$$MERGE_BASE"...HEAD); \
	NEEDS_README=false; \
	for f in $$CHANGED; do \
		case "$$f" in \
			pkg/tui/keymap.go|cmd/root.go|pkg/tui/commands.go) NEEDS_README=true ;; \
		esac; \
	done; \
	if [ "$$NEEDS_README" = "true" ]; then \
		if ! echo "$$CHANGED" | grep -q "README.md"; then \
			echo "ERROR: Changes to keymap.go, root.go, or commands.go require a README.md update"; \
			echo "Changed files that trigger this check:"; \
			echo "$$CHANGED" | grep -E "keymap.go|root.go|commands.go"; \
			exit 1; \
		fi; \
		echo "README.md update found - OK"; \
	else \
		echo "No config/key/flag changes detected - README check skipped"; \
	fi

.PHONY: test-race
test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	CGO_ENABLED=1 go test -race ./... -count=1 $(TESTOPTS)

.PHONY: test-vuln
test-vuln: ## Check for known vulnerabilities in dependencies
	@echo "Checking for known vulnerabilities..."
	@which govulncheck >/dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

COVERAGE_THRESHOLD ?= 55

.PHONY: test-coverage-threshold
test-coverage-threshold: ## Enforce minimum coverage threshold
	@echo "Checking coverage threshold ($(COVERAGE_THRESHOLD)%)..."
	@go test ./... -coverprofile=coverage.out -covermode=atomic > /dev/null 2>&1
	@total=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | tr -d '%'); \
	if [ $$(echo "$$total < $(COVERAGE_THRESHOLD)" | bc) -eq 1 ]; then \
		echo "FAIL: Coverage $$total% is below threshold $(COVERAGE_THRESHOLD)%"; \
		rm -f coverage.out; \
		exit 1; \
	fi; \
	echo "PASS: Coverage $$total% meets threshold $(COVERAGE_THRESHOLD)%"; \
	rm -f coverage.out

PATCH_COVERAGE_TARGET ?= 70

.PHONY: test-coverage-patch
test-coverage-patch: ## Check coverage of changed files (approximates Codecov patch coverage)
	@echo "Checking patch coverage for changed files (target $(PATCH_COVERAGE_TARGET)%)..."
	@MERGE_BASE=$$(git merge-base HEAD origin/main 2>/dev/null || echo ""); \
	if [ -z "$$MERGE_BASE" ]; then \
		echo "WARN: could not determine merge base; skipping patch coverage"; \
		exit 0; \
	fi; \
	CHANGED=$$(git diff --name-only "$$MERGE_BASE"...HEAD -- '*.go' | grep -v '_test.go' || echo ""); \
	if [ -z "$$CHANGED" ]; then \
		echo "No changed Go files - patch coverage check skipped"; \
		exit 0; \
	fi; \
	go test ./... -coverprofile=coverage.out -covermode=atomic > /dev/null 2>&1; \
	FAIL=false; \
	for f in $$CHANGED; do \
		pkg=$$(dirname $$f | sed 's|^|github.com/clcollins/srepd/|'); \
		cov=$$(go tool cover -func=coverage.out 2>/dev/null | grep "^$$pkg" | awk '{sum+=$$3; n++} END {if(n>0) printf "%.0f", sum/n; else print "N/A"}'); \
		if [ "$$cov" != "N/A" ] && [ $$(echo "$$cov < $(PATCH_COVERAGE_TARGET)" | bc 2>/dev/null || echo 0) -eq 1 ]; then \
			echo "  WARN: $$f package coverage $$cov% < $(PATCH_COVERAGE_TARGET)%"; \
			FAIL=true; \
		else \
			echo "  OK: $$f (package coverage $$cov%)"; \
		fi; \
	done; \
	rm -f coverage.out; \
	if [ "$$FAIL" = "true" ]; then \
		echo "WARN: Some changed packages have coverage below $(PATCH_COVERAGE_TARGET)%"; \
	else \
		echo "PASS: All changed packages meet $(PATCH_COVERAGE_TARGET)% coverage target"; \
	fi

.PHONY: test-fixtures
test-fixtures: ## Check that fixture data contains no real UUIDs, domains, or org names
	@echo "Checking fixture data for real values..."
	@FAIL=false; \
	for f in testdata/fixtures/*.json; do \
		if grep -qP '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' "$$f" 2>/dev/null; then \
			if grep -P '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' "$$f" | grep -qvP 'fake'; then \
				echo "ERROR: $$f contains UUID without 'fake' marker"; \
				FAIL=true; \
			fi; \
		fi; \
		if grep -q 'openshiftapps\.com\|devshift\.net\|devshift\.org\|redhat\.pagerduty\.com\|console\.redhat\.com' "$$f" 2>/dev/null; then \
			echo "ERROR: $$f contains real domain"; \
			FAIL=true; \
		fi; \
	done; \
	if [ "$$FAIL" = "true" ]; then \
		echo "FAIL: fixture data contains real values"; \
		exit 1; \
	fi; \
	echo "All fixture data is properly sanitized."

.PHONY: test-all
test-all: fmt-check vet lint test test-race test-fixtures ## Run all checks
	@echo "All checks passed."

.PHONY: ensure-goreleaser
ensure-goreleaser: ## Ensure goreleaser is installed
	@echo "Checking for goreleaser..."
	@which goreleaser >/dev/null 2>&1 || go install github.com/goreleaser/goreleaser/v2@${GORELEASER_VERSION}

.PHONY: release
release: ensure-goreleaser ## Create a release using goreleaser (RELEASE_NOTES=/path/to/notes.md optional)
	@echo "Creating a release..."
	GITHUB_TOKEN=$$(jq -r .goreleaser_token ~/.config/goreleaser/goreleaser_token) && \
	export GITHUB_TOKEN && \
	goreleaser release --clean --parallelism 1 $(if $(RELEASE_NOTES),--release-notes $(RELEASE_NOTES),)

.PHONY: clean
clean: ## Clean up build artifacts
	@echo "Cleaning up build artifacts..."
	rm -rf build/* dist/*

.PHONY: run
run: ## Run the application locally
	@echo "Running the application..."
	go run main.go
