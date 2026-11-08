# Release checklist

Do not tag until every required check below has passed on the exact release commit.
v0.3.0 is currently unreleased. Offline checks do not substitute for browser results.

- [ ] Commit the reviewed tree, then run `make release-gate`. It refuses dirty trees, records HEAD and the Git tree digest, and runs in a fresh detached worktree. Attach the complete receipt. Its required stages include `api-check`, `corpus-check` and `dom-check`; no stage may be skipped.
- [ ] jsdom for `internal/browsercheck/dom`: the DOM suite resolves `jsdom` from `internal/browsercheck/node_modules`. Run `npm ci --prefix internal/browsercheck --ignore-scripts` once (the release gate does this), or point `NODE_PATH` at an existing install of jsdom 20.0.3, for example `NODE_PATH=/path/to/node_modules python3 internal/browsercheck/dom/check.py --mutants`.
- [ ] Run the round-16 ten-step manual Comet checklist on that commit.
- [ ] Review the final public API against the previous tag; use named struct fields.
- [ ] Confirm `developmentVersion` in `cmd/cucumber-report/version.go` matches the
      intended version. Installed builds report `debug.ReadBuildInfo().Main.Version`;
      ordinary development source uses `0.3.0-dev` with a revision when available;
      release source/archives must set the exact version before tagging.
- [ ] Confirm pinned tools exist and install with the selected Go/Python versions:
      [golangci-lint v2.15.0](https://github.com/golangci/golangci-lint/releases/tag/v2.15.0)
      and [Playwright 1.63.0](https://pypi.org/project/playwright/1.63.0/).
      Both release sources were checked on 2026-11-07; recheck if pins change.
- [ ] Run `make verify`, `make surface-check`, all eight fuzz smoke targets, `python3 internal/surfacecheck/mutations.py`, the Node model tests, minified JS
      syntax validation, generated-asset parity, goldens and the install/build smoke test.
- [ ] Run `python3 internal/compatcheck/check.py` and `go test -C testdata/consumer ./...`.
      Confirm the captured Ruby retry fixture provenance. Capture JS text embeddings
      before describing that remaining hand-built case as a captured golden.
- [ ] Require green CI for browser interactions, print/PDF, and Chromium/Firefox/WebKit.
      Check large-print confirmation, selected-scope node counts, Ctrl+P scope reset,
      keyboard navigation and forced-colors focus. Do not infer these results from unit tests.
- [ ] Run the browser, print and cross-browser suites on the exact final release tree. The user
      supplied successful earlier `TestRealChrome` and `make browser-check` output.
      The reviewers never launched a browser; those receipts are not review evidence
      or a pass on the final release tree. A local Chrome launch aborted before
      DevTools became available, so its interaction tests remain pending CI.

- [ ] Change the README's “Planned version 0.3.0 (unreleased)” heading to the release
      version. Change CHANGELOG `[Unreleased]` to `[0.3.0] - YYYY-MM-DD` and add a fresh
      unreleased section. Replace the guide's pre-tag wording with the completed status.
- [ ] Run `go test ./report -run '^$' -bench '^BenchmarkSidecarRelativeBudget$' -benchtime=1x`
      on the release filesystem; the median durable-publication budget is 2× write-only.
- [ ] Set `developmentVersion` to the exact release version in source before tagging.
      Source archives use it; `-ldflags '-X main.developmentVersion=0.3.0'` can override it.
- [ ] Review the release diff, commit it, and create/push the `v0.3.0` tag only with
      release authorization and the checks above complete.
- [ ] Smoke-test the published module from a clean temporary GOPATH/GOBIN:
      `GOWORK=off GOPATH=<fresh-dir> GOBIN=<fresh-bin> go install github.com/qatoolist/cucumber-reporter/cmd/cucumber-report@v0.3.0`.
      Check `cucumber-report -version` and render the small fixture with that installed binary.
- [ ] Publish release notes and record the CI run links, module version and checksums.

- [ ] After tagging, bump `developmentVersion` to the next planned release so untagged builds identify themselves correctly.
