# Changelog

All notable changes are recorded here. The format follows Keep a Changelog, and versions follow
Semantic Versioning.

## [Unreleased]

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
