# Changelog

All notable changes are recorded here. The format follows Keep a Changelog, and versions follow
Semantic Versioning.

## [Unreleased]

### v0.1.0 (not yet released)

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
