GO ?= go
GOLANGCI_LINT ?= golangci-lint
FUZZTIME ?= 30s

.PHONY: help fmt fmt-check vet lint test generate fuzz-smoke golden-update browser-check verify

help: ## List targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-14s %s\n", $$1, $$2}'

fmt: ## Format the Go sources (gofumpt and goimports through golangci-lint)
	$(GOLANGCI_LINT) fmt ./...

fmt-check: ## Fail when a Go file is not gofmt-formatted
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "not formatted:"; echo "$$out"; exit 1; fi

vet: ## Run go vet, including the browser check
	$(GO) vet ./...
	$(GO) vet -tags browser ./...

lint: ## Run golangci-lint v2 with the project's lint configuration (the binary must be built with Go 1.27 or newer)
	@$(GOLANGCI_LINT) version
	$(GOLANGCI_LINT) run ./...
	$(GOLANGCI_LINT) run --build-tags browser ./...

test: ## Run all tests under the race detector
	$(GO) test -race -count=1 ./...

generate: ## Re-minify assets/app.js and assets/app.css into the committed .min files
	$(GO) generate ./report

fuzz-smoke: ## Run each fuzz target for FUZZTIME (default 30s)
	$(GO) test ./cucumber -run '^$$' -fuzz FuzzParse -fuzztime $(FUZZTIME)
	$(GO) test ./report -run '^$$' -fuzz FuzzRender -fuzztime $(FUZZTIME)

golden-update: ## Rewrite testdata/golden from the current renderer (review the diff)
	$(GO) test ./report -run TestGolden -update

browser-check: ## Interaction, print and stress checks in local headless Chrome (CHROME selects one; PDF checks need Ghostscript, GS selects one)
	$(GO) test -tags browser -count=1 ./internal/cdp ./report

verify: fmt-check vet lint test ## The offline gate: formatting, vet, lint and tests
