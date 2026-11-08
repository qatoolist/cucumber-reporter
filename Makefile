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
	$(GO) test ./internal/cucumberinput -run '^$$' -fuzz '^FuzzParse$$' -fuzztime $(FUZZTIME) -fuzzminimizetime 1s
	$(GO) test ./internal/cucumberinput -run '^$$' -fuzz FuzzMessages -fuzztime $(FUZZTIME) -fuzzminimizetime 1s
	$(GO) test ./internal/cucumberinput -run '^$$' -fuzz FuzzParseDifferential -fuzztime $(FUZZTIME) -fuzzminimizetime 1s
	$(GO) test ./report -run '^$$' -fuzz FuzzRender -fuzztime $(FUZZTIME) -fuzzminimizetime 1s
	$(GO) test ./report -run '^$$' -fuzz FuzzRedactionJSON -fuzztime $(FUZZTIME) -fuzzminimizetime 1s
	$(GO) test ./internal/minify -run '^$$' -fuzz '^FuzzJS$$' -fuzztime $(FUZZTIME) -fuzzminimizetime 1s
	$(GO) test ./internal/minify -run '^$$' -fuzz FuzzJSParseOracle -parallel 2 -fuzztime $(FUZZTIME) -fuzzminimizetime 1s
	$(GO) test ./internal/minify -run '^$$' -fuzz FuzzCSS -fuzztime $(FUZZTIME) -fuzzminimizetime 1s

golden-update: ## Rewrite testdata/golden from the current renderer (review the diff)
	$(GO) test ./report -run TestGolden -update

browser-check: ## Interaction, print and stress checks in local headless Chrome (CHROME selects one; PDF checks need Ghostscript, GS selects one)
	$(GO) test -tags browser -count=1 ./internal/cdp ./report

compat-check: ## Compare every fixture with a locally available v0.2.0 tag
	@if git rev-parse --verify refs/tags/v0.2.0 >/dev/null 2>&1; then python3 internal/compatcheck/check.py; else echo "Compatibility check failed: local v0.2.0 tag unavailable; fetch the baseline tag before release verification"; exit 1; fi

verify: fmt-check vet lint test compat-check ## The offline gate: formatting, vet, lint and tests

.PHONY: surface-check
surface-check: ## Compare public API and artifacts over every unique repository fixture
	python3 internal/surfacecheck/run.py

.PHONY: ui-check
BROWSERS ?= chromium
ui-check: ## Playwright UI checks on generated reports: routes, print, reflow, retries and interactions (BROWSERS="chromium firefox webkit")
	@d=$$(mktemp -d) && mkdir -p $$d/showcase $$d/retry $$d/empty $$d/messages && echo '[]' > $$d/empty.json && \
	$(GO) run ./cmd/cucumber-report -enhanced -bundle -options testdata/synthetic/showcase/options-enhanced.json -o $$d/showcase testdata/synthetic/showcase >/dev/null && \
	$(GO) run ./cmd/cucumber-report -enhanced -o $$d/retry testdata/captured/cucumber-ruby-9.2.1-retry/retry.json >/dev/null && \
	$(GO) run ./cmd/cucumber-report -enhanced -o $$d/empty $$d/empty.json >/dev/null && \
	$(GO) run ./cmd/cucumber-report -enhanced -format messages -o $$d/messages testdata/round15/captured/s3-retry2/run.ndjson >/dev/null && \
	python3 internal/browsercheck/check.py $$d/showcase/html-*/index.html $(BROWSERS) && \
	ok=1 && for b in $(BROWSERS); do python3 internal/browsercheck/retries.py $$d/retry/html-*/index.html $$b || ok=0; done && [ $$ok = 1 ] && \
	python3 internal/browsercheck/interactions.py $$d/retry/html-*/index.html $$d/empty/html-*/index.html $$d/messages/html-*/index.html $(BROWSERS) && \
	python3 internal/browsercheck/interactions_mutants.py $$d/retry/html-*/index.html $$d/empty/html-*/index.html $$d/messages/html-*/index.html $(firstword $(BROWSERS)); \
	status=$$?; rm -rf "$$d"; exit $$status

.PHONY: release-gate corpus-check api-check dom-check
release-gate: ## Clean-commit release gate with full output in dist/release-gate.log (browser CI remains separate)
	python3 internal/releasecheck/run.py

corpus-check: ## All 121 real Ruby capture groups and runner fixtures, including regenerated large evidence
	python3 internal/runnercheck/check.py

api-check: ## Freeze exported API and kill type-leak/signature/constant mutants
	python3 internal/apicheck/check.py

dom-check: ## Reviewer DOM interactions and UI mutants, without launching a browser
	python3 internal/browsercheck/dom/check.py --mutants
