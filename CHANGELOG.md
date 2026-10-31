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
