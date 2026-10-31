# Changelog

All notable changes are recorded here. The format follows Keep a Changelog, and versions follow
Semantic Versioning.

## [Unreleased]

### v0.1.0 (not yet released)

- `cucumber` package: strict parser for Godog's Cucumber JSON (six statuses, optional durations,
  error messages, doc strings, data tables, outline rows, attachments), `Validate`, canonical base64
  `Embedding.Decode`, scenario and feature status rules and `FormatDuration`.
- `report` package: `Render`, `RenderDir` and `RenderDirTo` produce one self-contained HTML file
  with a dashboard, caller sections, six inline SVG charts plus section charts, a sortable and
  searchable feature table, scenario cards and attachments shown as text.
- Settings dialog with per-viewer preferences in `localStorage` over generation-time defaults.
- Hash-pinned inline CSS and JavaScript under a strict Content-Security-Policy.
- `cucumber-report` command with `-title`, `-name`, `-defaults` and `-o`.
- A scenario without steps, which Godog writes with no `steps` key, is accepted and counted as
  undefined, as Godog counts it.
- Feature and run status follow multiple-cucumber-html-reporter's feature rule (`cucumber.Combine`,
  `cucumber.RunStatus`): a skipped or pending scenario beside passed ones no longer makes the
  feature or the headline skipped or pending.
- `Options.Verdict` lets the caller set the headline (status, label, explanation); the computed
  result is still shown.
- `report.RenderFeaturesDirTo` writes already-parsed features to a new directory;
  `report.FeatureAnchor` and `report.ScenarioAnchor` give the stable card ids.
- Run, output and `-defaults` paths are cleaned before the symbolic-link check, so `link/` and
  `link/.` are refused.
- Chart totals use int64.
- Linting uses the project's golangci-lint v2 configuration.
