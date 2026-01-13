unexport GOFLAGS

GOOS ?= linux
GOARCH ?= amd64
GOENV = GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=0 GOFLAGS=
GOPATH := $(shell go env GOPATH | awk -F: '{print $1}')
BIN_DIR := $(GOPATH)/bin
HOME ?= $(shell echo ~)

GOLANGCI_LINT_VERSION = v2.1.5
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
test: lint ## Run tests (after linting)
	@echo "Running tests..."
	go test ./... -v $(TESTOPTS)

.PHONY: coverage
coverage: ## Generate test coverage report
	@echo "Generating test coverage report..."
	hack/codecov.sh

.PHONY: getlint
getlint: ## Install golangci-lint if not already installed
	@echo "Checking for golangci-lint..."
	@which golangci-lint >/dev/null 2>&1 || (echo "Installing golangci-lint..." && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(BIN_DIR) $(GOLANGCI_LINT_VERSION))

.PHONY: lint
lint: getlint ## Run golangci-lint
	@echo "Running golangci-lint..."
	$(BIN_DIR)/golangci-lint --version
	$(BIN_DIR)/golangci-lint run --timeout 5m

.PHONY: vet
vet: ## Run go vet to catch common mistakes
	@echo "Running go vet..."
	go vet ./...

.PHONY: check
check: fmt lint vet ## Run all code checks (fmt, lint, vet)
	@echo "Running all code checks..."

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

.PHONY: fmt
fmt: ## Format the code
	@echo "Formatting the code..."
	gofmt -s -l -w cmd pkg

.PHONY: clean
clean: ## Clean up build artifacts
	@echo "Cleaning up build artifacts..."
	rm -rf build/* dist/*

.PHONY: run
run: ## Run the application locally
	@echo "Running the application..."
	go run main.go
