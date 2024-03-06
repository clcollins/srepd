unexport GOFLAGS

GOOS?=linux
TESTOPTS ?=
GOARCH?=amd64
GOENV=GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=0 GOFLAGS=
GOPATH := $(shell go env GOPATH)
HOME?=$(shell mktemp -d)

GOLANGCI_LINT_VERSION=v1.51.2
GORELEASER_VERSION=v1.14.1
export GORELEASER_TOKEN?=$(shell jq -r .goreleaser_token ~/.config/goreleaser/goreleaser_token)

# Ensure go modules are enabled:
export GO111MODULE=on
export GOPROXY=https://proxy.golang.org

# Disable CGO so that we always generate static binaries:
export CGO_ENABLED=0


.PHONY: build
build:
	go build -o build/srepd .

.PHONY: install
install:
	go build -o ${GOPATH}/bin/srepd .

.PHONY: install-local
install-local:
	go build -o ${HOME}/.local/bin/srepd .

.PHONY: mod
mod:
	go mod tidy

.PHONY: test
test:
	go test ./... -v $(TESTOPTS)

.PHONY: coverage
coverage:
	hack/codecov.sh

# Installed using instructions from: https://golangci-lint.run/usage/install/#linux-and-windows
getlint:
	@mkdir -p $(GOPATH)/bin
	@ls $(GOPATH)/bin/golangci-lint 1>/dev/null || (echo "Installing golangci-lint..." && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin $(GOLANGCI_LINT_VERSION))

.PHONY: lint
lint: getlint
	$(GOPATH)/bin/golangci-lint run --timeout 5m

ensure-goreleaser:
	@ls $(GOPATH)/bin/goreleaser 1>/dev/null || go install github.com/goreleaser/goreleaser@${GORELEASER_VERSION}

release: ensure-goreleaser
	goreleaser release --rm-dist

.PHONY: fmt
fmt:
	gofmt -s -l -w cmd pkg tests

.PHONY: clean
clean:
	rm -rf \
		build/*

	rm -rf \
		dist/*
