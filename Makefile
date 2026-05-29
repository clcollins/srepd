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
			pkg/tui/keymap.go|cmd/config.go|cmd/root.go|pkg/tui/commands.go) NEEDS_README=true ;; \
		esac; \
	done; \
	if [ "$$NEEDS_README" = "true" ]; then \
		if ! echo "$$CHANGED" | grep -q "README.md"; then \
			echo "ERROR: Changes to keymap.go, config.go, root.go, or commands.go require a README.md update"; \
			echo "Changed files that trigger this check:"; \
			echo "$$CHANGED" | grep -E "keymap.go|config.go|root.go|commands.go"; \
			exit 1; \
		fi; \
		echo "README.md update found - OK"; \
	else \
		echo "No config/key/flag changes detected - README check skipped"; \
	fi

.PHONY: test-all
test-all: fmt-check vet lint test ## Run all checks (fmt, vet, lint, test)
	@echo "All checks passed."

.PHONY: ensure-goreleaser
ensure-goreleaser: ## Ensure goreleaser is installed
	@echo "Checking for goreleaser..."
	@which goreleaser >/dev/null 2>&1 || go install github.com/goreleaser/goreleaser/v2@${GORELEASER_VERSION}

.PHONY: release
release: ensure-goreleaser ## Create a release using goreleaser
	@echo "Creating a release..."
	GITHUB_TOKEN=$$(jq -r .goreleaser_token ~/.config/goreleaser/goreleaser_token) && \
	export GITHUB_TOKEN && \
	goreleaser release --clean

.PHONY: clean
clean: ## Clean up build artifacts
	@echo "Cleaning up build artifacts..."
	rm -rf build/* dist/*

.PHONY: run
run: ## Run the application locally
	@echo "Running the application..."
	go run main.go
