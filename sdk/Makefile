SHELL := bash
export GOTOOLCHAIN := go1.27.1
export GOWORK := off
GOLANGCI_LINT_VERSION := v2.13.2
.PHONY: check test integration generate verify-lens
generate:
	python3 recipes/lens/generate.py
	python3 providers/aave/lens/generate.py
verify-lens:
	python3 recipes/lens/generate.py --check
	python3 providers/aave/lens/generate.py --check
check: verify-lens
	go build ./...
	go test -race -cover ./...
	go vet ./...
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --config .golangci.yml
test:
	go test -race -cover ./...
integration:
	MERKLE_ANVIL_TEST=1 go test -tags=integration ./recipes -run TestAnvil -count=1 -v
