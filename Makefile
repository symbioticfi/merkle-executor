SHELL := bash
.DEFAULT_GOAL := help
export GOTOOLCHAIN := go1.27.1
PKG := github.com/symbioticfi/merkle-executor
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X $(PKG)/internal/version.Version=$(VERSION) -X $(PKG)/internal/version.Commit=$(COMMIT) -X $(PKG)/internal/version.Date=$(DATE)
GOLANGCI_LINT_VERSION := v2.13.2
ABIGEN_VERSION := v1.17.5
# The abigen CLI's swiss dependency excludes Go 1.27; service builds use 1.27.1.
ABIGEN_TOOLCHAIN := go1.26.8
ABIGEN := $(CURDIR)/.tools/abigen

.PHONY: help build test integration vet lint format generate tidy check tools
help:
	@echo 'build test integration vet lint format generate tidy check'
build:
	go build -trimpath -ldflags '$(LDFLAGS)' -o bin/merkle-executor ./cmd/merkle-executor
test:
	go test -race -cover ./...
integration:
	$(MAKE) -C sdk integration
	MERKLE_ANVIL_TEST=1 go test -tags=integration ./cmd/merkle-executor ./internal/txengine -run TestAnvil -count=1 -v
vet:
	go vet ./...
lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run
format:
	gofmt -s -w cmd internal sdk
tools: $(ABIGEN)
$(ABIGEN):
	GOTOOLCHAIN=$(ABIGEN_TOOLCHAIN) GOBIN='$(CURDIR)/.tools' go install github.com/ethereum/go-ethereum/cmd/abigen@$(ABIGEN_VERSION)
generate: tools
	ABIGEN='$(ABIGEN)' bash hack/generate-bindings.sh
	ABIGEN='$(ABIGEN)' bash hack/generate-protocol-bindings.sh
	python3 hack/generate-protocol-types.py
tidy:
	go mod tidy
check: build test vet lint
	$(MAKE) -C sdk check
