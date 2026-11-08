# Changelog

All notable changes are recorded here. The format follows Keep a Changelog, and versions follow
Semantic Versioning.

## [Unreleased]

Planned v0.3.0; browser/print/cross-browser CI remains a release gate.

### Added

- Optional enhanced run processing: separate historical identities, shard merging,
  baseline comparison, scoped bounded history, retries, completeness expectations,
  explicit quality gates, versioned JSON/CSV/Markdown exports and rerun selectors.
- Optional redaction, issue lifecycle policies, configurable failure fingerprints,
  measured execution intervals, deduplicated binary sidecars and ZIP bundles.
- Cucumber Messages input and feature-at-a-time legacy parsing; safe validation
  diagnostics and a writer-based rendering API.
- Investigation UI: attention actions, split failure view, problem summaries,
  searchable error/evidence viewers, scenario table, advanced filters and reusable
  views, persistent navigation context and optional analysis/attempt panels.
- Structured annotation details and compact banners with expandable notes/evidence.
- Selectable print scope without the former 400-scenario feature cap.
- Local saved-PDF compression with high/balanced/small quality presets, page-count
  verification, before/after size reporting and a never-enlarge fallback. Ghostscript
  is optional and required only for this command; saved-PDF layout stays unchanged.
  The HTML layout, typography, filtering and navigation do change in 0.3.0.
- Cross-browser CI checks, runner-format fixtures, a v0.2.0 compatibility harness,
  allocation benchmarks and report-size budgets.
- Context-aware rendering, streamed directory/ZIP output and durable file publication.
- Enhanced processing of runner timestamps/git facts and final-attempt retry outcomes.
- Real-browser UI interaction checks, run in CI on Chromium, Firefox and WebKit (`make ui-check`
  locally, Chromium by default): settings keep scenario state, Back/Forward focus, mobile drawer,
  evidence reveal, command palette, attempt times and the empty report, with ten mutants proving
  every check fails on broken behaviour.
- Execution attempts show the runner's recorded start, finish and elapsed time (Cucumber
  Messages), and the attempt selector shows each start time; without timestamps each attempt
  shows its own summed step time, labelled as such.

### Fixed

- Redaction no longer changes known-issue decisions. Step-text matching and issue
  policies (environment, error signature) are decided on the unmasked evidence and
  carried through masking in both directions, and the evidence floor checks them
  against its own pre-redaction snapshot. A mismatched `stepText` that masking makes
  identical is published with "(differs before redaction)" and covers nothing.
- Redaction no longer makes an earlier retry attempt's different step look like the
  covered final step. A step that differed before masking is published as
  "… (differs before redaction)", so a masked run gives the same failed report as an
  unmasked one instead of being refused.
- A baseline or history comparison on scenario identities that masking changed
  (generated from masked names or paths, or metadata identities a pattern matches) adds
  an `identity` diagnostic: a scenario whose secret changes between runs is the same
  scenario to the new-failures gate. Use opaque metadata identities that no pattern
  matches to tell such scenarios apart.
- A secret split by ANSI color codes or zero-width characters is masked. Display
  output removes or hides them and previously rejoined the secret in the HTML, CSV,
  Markdown and `report.zip`. Matches on the visible text are added to the raw-text
  matches; text with nothing more to mask is unchanged.
- Step, scenario and feature keywords are redacted like other display text.
- A baseline or history entry whose environment or branch matches the current run,
  or another history scope, only after masking is refused, as an unmasked scope
  mismatch is. Comparing a stored, already-redacted scope adds a diagnostic.
- `jsonKeys` match keys as they display, so a zero-width character cannot split a key.
- Caller `report.Input.Diagnostics` pass through redaction before publication in
  `summary.json`, the HTML and `report.zip`.
- Duration-regression gate and evidence floor share one exact decimal predicate, so a
  run exactly on the threshold (for example 16.7%) is no longer refused.
- CLI input diagnostics (for example ignored malformed hook metadata) are published in
  `summary.json` and the HTML again, through `report.Input.Diagnostics`, and are covered
  by the prepared digest.
- The evidence floor re-derives known-issue coverage from annotations and raw step text,
  so a mismatched `stepText` cannot turn a failure into a known failure.
- `ErrEvidence` is attached by error type when floor errors are created, never by
  matching message text. Caller edits and shard metadata conflicts are reported as such,
  naming the URI.
- A Messages input without `testRunFinished.success` names that field.
- Shard labels fall back to the cleaned relative path, so `x/run.json ci/x/run.json
  run.json` is accepted with unique, non-absolute names. The library's missing-results
  error is neutral; the CLI suggests `-enhanced` with each shard listed.
- Collision refusals sort by numeric line, truncate only the URI, and name inputs with
  identical content. Every `-validate -strict` diagnostic refusal uses the same wording.
- Settings no longer rebuild an open scenario or re-run its step reveal, so focus stays
  in the dialog and Collapse all, Problems only and the attempt choice are kept. "Open
  attachments" and Reset apply in place. A `?attempt=` hashchange selects that attempt,
  and a missing attempt row falls back to the step.
- Removed two dead runner-failure guards.
- The evidence floor re-derives issue-policy exclusions (environment, expiry, error
  signature) from the run configuration. `Input.Diagnostics` are cleaned and bounded, and
  may not use the reserved `runner.outcome` path or absolute paths. Repeated identical
  inputs never print an absolute path. A Messages envelope value of the wrong type names
  its field.
- Refuse all repeated scenario locations across inputs, listing the colliding
  locations and inputs. Disjoint shards and retries within one input remain supported.
  Generate one report per rerun or matrix cell; cross-input merging is not shipped.
- Fix enhanced retry bookmarks and annotations, Messages retry evidence safeguards,
  runner-failure attribution, Around-hook evidence/flakiness, and bundled sidecars.
- Add Background-step annotation migration and retain known issues across retries
  only when every failure is covered. Restore strict diagnostic checks.
- Batch durable publication with bounded file flushes and a final directory/device-cache
  barrier; avoid per-sidecar full-drive flushes on macOS.
- Fix repeated step/evidence navigation, expand/collapse scope and hidden evidence
  keyboard focus. Browser regression tests cover the affected interactions.

- Centralize effective outcomes across rendering and library APIs; verify each
  scenario against independent execution evidence before publishing.
- Known issues no longer hide unrelated failed steps or hooks. Failed quality gates
  produce a failed artifact verdict, including optional pending/flaky/empty policies.
- Accept Ruby Around hooks, completed failed cases missing Messages results,
  retries within a single input, and valid zero-execution reports.
- Retain hook attachment placement/origin, cap total inline evidence at 32 MiB,
  and use public step types in exported attempt metadata.
- Old folded Background/retry bookmarks now resolve to their final scenario.
  **Compatibility note:** annotations on those old IDs are refused with the final
  target ID; use that scenario ID plus `backgroundStep:N` and `stepText` to target
  a folded Background step. Retry annotations target the final scenario and declared
  step number. This is an error,
  not an automatic annotation redirect.
- Malformed-input byte offsets may differ from v0.2.0; numeric paths and sentinel
  error categories remain the supported diagnostic interface.

- Plain reports include non-passing scenario and step hooks and their evidence;
  standalone Background results fold into the following scenario. Plain validation
  uses the same rendering rules. Plain reports have no implicit quality gate.
- Generate colliding navigation IDs in linear time while retaining their values.
- Accept UTF-8 BOM input and explain unsupported encodings and duration units.
- Retain JVM/Ruby log output, cucumber-js arguments and supported evidence aliases.
- Preserve native scenario-link semantics, dark skip-link contrast and keyboard focus.
- Report missing output directories and unsupported input shapes clearly.

### Changed

- `-h` writes help to stdout and exits 0. `-strict` requires `-validate`.
- Confirmed Ruby retries fold in plain mode as well as enhanced mode. This can change
  a v0.2.0 failed verdict to Passed (flaky) when the final attempt strictly passed.
  A final pending/skipped attempt cannot rescue a failure; earlier evidence is retained.
- Messages runner-reported failures remain non-green even when final scenarios passed.
- Passing hooks with logs or attachments retain that evidence without adding hook rows.
- Fail closed when raw failure evidence is missing from the final report.

- Print builds only selected details. Above 1,000 scenarios, Ctrl+P without an explicit
  confirmed selection prints a cover; choose Full report in the Print dialog for all pages.
  Preview selections survive printing/cancelling until leaving the preview.
- Large static no-JavaScript reports cap passed details after 1,000 scenarios, retaining
  all problem details and complete interactive data/exports. The CLI warns above 200 MiB.
- Development/pseudo-version builds identify as `0.3.0-dev (<revision>)` when available.

### Compatibility

- Existing CLI invocation, options version 2, plain scenario navigation IDs and known-issue
  rules remain supported. History, gates, redaction and enriched input are optional.
- Gate exit code 3 is used only when explicitly requested and only after output is
  written, except validation, which writes nothing. Existing report-generation success still exits 0.
- Valid empty legacy arrays and features without scenarios now parse successfully.
  `cucumber.Validate(nil)` and `report.Validate(nil, validOptions)` succeed;
  `report.Render(nil, validOptions)` renders an informational empty report.
  `cucumber.RunStatus(nil)` is now `StatusUndefined` rather than `StatusPassed`.
- `ScenarioOutcomes` returns `OutcomeFolded` for source Background and retry slots
  folded into a surviving scenario. Exclude these slots when counting outcomes.
- Parse diagnostics use the public `*cucumber.ParseError`, including `Path`, `Offset`
  and `Unwrap`; concrete `%T` values are not a compatibility contract. Use
  `errors.As` and `errors.Is` rather than matching implementation type names.
- Prepared runs contain explicit public provenance and analysis values. Equal copies
  render identically; mismatching prepared options/features return `ErrInvalidOptions`.
  Prepare changed features again before rendering or publication.
- HTML layout/bytes change. Use named fields in exported Go struct literals.
- See [0.3.0 configuration and behavior](docs/0.3.0.md) for contracts and limitations.

## [0.2.0] - 2026-11-05

### Added

- Optional `Options.TimingSection` links the dashboard time card to an existing
  caller-provided section. Without it, the card keeps its slowest-scenarios link.

### Changed

- A supplied `WallClock` is the primary dashboard and print duration, labelled
  "Total elapsed time". Summed step time remains visible separately in both views.
  Without `WallClock`, the primary metric is "Summed step time"; the reporter does
  not estimate elapsed time from steps. Surrounding whitespace is trimmed and a
  whitespace-only value is treated as absent.
- Timing labels no longer assume that a run has multiple workers.

This minor release adds an exported option; existing named-field callers and
options files remain compatible. `OptionsVersion` remains 2.

## [0.1.1] - 2026-10-31

Fixes from the independent review of v0.1.0. No API change.

- The timeline axis has at most eleven labels for any duration, fewer on narrow screens so labels
  never overlap; a step of the largest int64 duration used to make the page create millions of labels.
- An attachment's Raw view and Copy give the original text byte for byte, a leading byte order mark
  included; re-serialised JSON had rounded large integers, turned `1e400` into `null` and dropped
  repeated keys.
- The virtualised scenario list measures its rows once they are in the page, so the last row is fully
  visible.
- `Render` and the directory functions enforce `Limits.Embedding` before decoding, as `Parse` does;
  an attachment over the limit is refused with `cucumber.ErrTooLarge`.
- Print shows ordinary sections' rows.
- An empty attachment shows as empty, with Copy and Save, not as undecodable.
- Optional content no longer renders the text "null".
- The no-JS scenario duration saturates at the largest int64 instead of overflowing.
- A test now decides the canonical base64 re-encoding check in `Embedding.Decode`.

## [0.1.0] - 2026-10-31

- `cucumber` package: strict parser for Godog's Cucumber JSON (six statuses, optional durations,
  error messages, doc strings, data tables, outline rows, attachments), `Validate`, canonical base64
  `Embedding.Decode`, scenario and feature status rules (`Combine`, `RunStatus`, multiple-cucumber-
  html-reporter's feature rule) and `FormatDuration`. A scenario without steps is accepted and counted
  as undefined, as Godog counts it.
- `report` package: `Render`, `RenderDir`, `RenderDirTo` and `RenderFeaturesDirTo` produce one
  self-contained HTML file. The page is an application with a dashboard, feature, scenario, failure,
  known-issue, skipped, timeline and run-detail views, hash routes for every view, a search palette,
  keyboard shortcuts, interactive charts with drill-down, a settings drawer (theme, density, start
  page, attachments, widgets, navigation) saved per browser under `cucumber-reporter:v2:prefs`, a
  dedicated print document, and a no-JS fallback rendered by `html/template`.
- The report data travels in one `application/json` block written by `encoding/json` with HTML-safe
  escaping; the page script builds the DOM with `createElement` and `textContent` only.
- `Options` version 2: `TopFacts`, `WallClock`, `Verdict{Label, Tone}`, `Sections` with `Role`,
  `Annotations` (known issues, skip reasons, observation limits) with the known-issue rules applied by
  the renderer (a known issue covers only its declared step, checked against `StepText`), and `ViewDefaults{Theme, Density, Expand, View, Widgets, Nav}`. `FeatureIDs` and
  `ScenarioIDs` give the stable identifiers used in links and annotations.
- Hash-pinned inline CSS and JavaScript under a strict Content-Security-Policy. The embedded assets
  are the committed output of `go generate ./report` (`internal/minify`, comments and whitespace
  only); a test re-minifies the readable sources and compares bytes.
- Colours are defined once with CSS `light-dark()`; the browser floor is in the README.
- `cucumber-report` command with `-title`, `-name`, `-options` and `-o`.
- Run, output and `-options` paths are cleaned before the symbolic-link check, so `link/` and
  `link/.` are refused.
- Browser checks (`make browser-check`) drive headless Chrome over the DevTools protocol with a
  standard-library client (`internal/cdp`) and check print output with Ghostscript.
- Linting uses a strict golangci-lint v2 configuration.
- Route and id lookups in the page have no object prototype, so links such as `#/constructor`
  show "Not found".
- A known issue covers only its declared step (`Annotation.StepText` is checked against it); a
  failure anywhere else stays an unexpected failure.
- The status legend moves under the donut when its card is narrow; labels never run into counts.
- Needs attention keeps every group with a "+N more" link; the duration histogram colours only
  failed scenarios red; the print preview sits under its header; the timeline axis starts at 0.
- A record that covers nothing is never counted as a known issue; the Known issues page lists it
  under "Records not applied", and print lists it with its reason.
- The computed headline for a failed run reads "Failed: N unexpected failures";
  `report.ScenarioOutcomes` exposes the outcome classification.
- `Outcome.NotApplicable`: a passed scenario whose record has only not-applicable outcomes reads
  "not applicable in this run" instead of "the issue may be fixed".
- Dashboard rows are fitted so no card ends in a large empty area.


Round-16 release hardening (unreleased): raw input status audits independently
check runner signals and scenario rescue outcomes; cross-input rerun merging
remains removed. Prepared snapshots validate feature, summary and execution
metadata together and reject missing digests. Empty enhanced runs render normally;
mobile same-page navigation closes the drawer; Settings navigation toggles work;
query-only step/evidence navigation preserves the mounted view. Release verification
now requires a clean commit and a detached worktree, with two-way corpus checks
and pinned refusal inventories. Parser provenance gains status-only audit fields,
and `report.ErrEvidence` categorizes runtime evidence contradictions.
