# cucumber-reporter

cucumber-reporter turns the Cucumber JSON that [Godog](https://github.com/cucumber/godog) writes
(`--format cucumber`) into one self-contained HTML report. The report works offline from `file://`,
needs no network, has light and dark themes and stays readable with JavaScript turned off.

It is a Go module with no dependencies outside the standard library.

## Install

```sh
go install github.com/qatoolist/cucumber-reporter/cmd/cucumber-report@latest
```

As a library:

```sh
go get github.com/qatoolist/cucumber-reporter
```

Go 1.27 or newer is required.

## Command line

```
cucumber-report [-title T] [-name N] [-defaults settings.json] [-o parent-dir] <run-dir>
```

The command reads `<run-dir>/cucumber.json` and writes a new `<run-dir>/html-XXXXXX/index.html`
(or `<parent-dir>/html-XXXXXX/index.html` with `-o`). Every render creates a new directory; earlier
reports are never replaced. On success it prints one line:

```
HTML report: /path/to/run/html-a1B2c3/index.html
```

| Flag | Meaning |
| --- | --- |
| `-title` | Document title. Default `Cucumber report`. |
| `-name` | Heading shown on the page. Default: the title. |
| `-defaults` | JSON file with the report's view defaults (below). |
| `-o` | Directory in which to create `html-XXXXXX`. Default: the run directory. |

Exit status is 0 on success, 1 when the input or settings are refused and 2 for a usage error.

### View defaults file

```json
{
  "version": 1,
  "charts": {"slowest-scenarios": false},
  "theme": "auto",
  "density": "comfortable",
  "expand": "failed",
  "pageSize": 50
}
```

`version` is required and must be 1. Every other key is optional. Unknown keys are refused.

| Key | Values |
| --- | --- |
| `charts` | Chart ID to `true` (shown) or `false` (hidden). IDs are listed under Charts. |
| `theme` | `auto` (follows the system), `light`, `dark` |
| `density` | `comfortable`, `compact` |
| `expand` | Scenario cards open at load: `failed`, `none`, `all` |
| `pageSize` | `25`, `50`, `100` or `"all"` |

## Library

```go
import (
	"github.com/qatoolist/cucumber-reporter/cucumber"
	"github.com/qatoolist/cucumber-reporter/report"
)

// Render a run directory, as the command does.
dir, err := report.RenderDir(runDir, report.Options{
	Version:  report.OptionsVersion,
	Title:    "Checkout suite",
	RunFacts: []report.Fact{{Label: "Revision", Value: revision}},
	Sections: []report.Section{{
		ID: "known-issues", Title: "Known issues", Tone: report.ToneWarn,
		Summary: []report.Fact{{Label: "Known failures", Value: "2"}},
		Chart: &report.ChartSeries{Title: "Known failures compared with failures", Segments: []report.Segment{
			{Label: "Known failures", Value: 2, Tone: report.ToneWarn},
			{Label: "Unexpected failures", Value: 0, Tone: report.ToneFail},
		}},
	}},
	Defaults: report.ViewDefaults{Charts: map[report.ChartID]bool{report.ChartSlowestScenarios: false}},
})

// Or parse and render in memory.
features, err := cucumber.Parse(r, cucumber.Limits{})
html, err := report.Render(features, report.Options{Version: report.OptionsVersion})
```

| Function | Purpose |
| --- | --- |
| `cucumber.Parse(r, limits)` | Strict parse of Cucumber JSON into `[]cucumber.Feature`. |
| `cucumber.Validate(features)` | The completeness checks `Parse` applies, for models built in code. |
| `Embedding.Decode(max)` | Attachment bytes; canonical standard base64 only. |
| `cucumber.FormatDuration(d)` | `40ms`, `<1ms`, `1h 2m 3s`. |
| `report.Render(features, opt)` | One HTML document as bytes. Deterministic: same input, same bytes. |
| `report.RenderDir(runDir, opt)` | Reads `cucumber.json`, writes a new `html-XXXXXX/index.html` in `runDir`. |
| `report.RenderDirTo(runDir, parent, opt)` | The same, creating `html-XXXXXX` in `parent`. |

### Options

| Field | Meaning |
| --- | --- |
| `Version` | Must be `report.OptionsVersion` (1). Any other value is refused. |
| `Title`, `Name` | Document title and page heading. |
| `RunFacts` | Label and value pairs shown in the header, in order. |
| `Sections` | Your own panels, shown in order on the dashboard. `ID` (a-z, 0-9, `-`, starting with a letter, at most 48 characters) names the section in settings. `Tone` is `info`, `ok`, `warn` or `fail`. A section may add a stacked bar chart through `Chart`. |
| `Defaults` | Generation-time view settings (same meaning as the defaults file; `Sections` hides caller sections by ID; `PageSize` uses `report.PageSizeAll` for all). |
| `Limits` | `Input` (default 512 MiB), `Embedding` (base64 characters per attachment, default 64 MiB) and `InlineAttachment` (default 4 MiB). |

Errors are sentinel values for `errors.Is`: `cucumber.ErrNoResults`, `ErrInvalidJSON`,
`ErrIncomplete`, `ErrInvalid`, `ErrTooLarge`, `ErrInvalidBase64`, and `report.ErrOptionsVersion`,
`ErrInvalidOptions`, `ErrNoInput`, `ErrNotRegular`, `ErrEmptyOutput`. Parse errors give a location
such as `feature[3].elements[1].steps[7]` or a byte offset, never input content.

## What the report shows

- A header with the overall result and your run facts.
- Status tiles for features, scenarios and steps.
- Your sections.
- Charts: `scenario-status` and `step-status` (donuts), `feature-results` (scenario results per
  feature), `slowest-steps` and `slowest-scenarios` (top 10), `feature-duration`, and
  `section-<id>` for each section with a chart. Charts are inline SVG drawn by the Go code.
- A feature table you can sort, search (`/`) and filter by status (`f` toggles failed only).
- Each feature with its scenarios as expandable cards: tags, steps with status symbol and text,
  durations, errors, data tables, doc strings and attachments.

Attachments: `text/*` (including `text/html`) and JSON are shown as text, JSON pretty-printed, each
with a copy button. HTML is never rendered as markup. Other types show their name, type and size
only. Text attachments above the inline limit are written beside the report as `.txt` or `.json`
files by `RenderDir` and linked, or left out with a notice by `Render`.

Status is always given as a symbol and a word as well as a colour.

## Settings

The Settings button opens a dialog to show or hide each chart and section and to choose the theme,
density, which scenarios start open and the page size. Choices are saved in the browser's
`localStorage` under `cucumber-reporter:v1:prefs`, so they follow the viewer from report to report
in that browser. "Reset to report defaults" removes them. When storage is unavailable (private
browsing, or `file://` storage turned off) the page uses the report's defaults and says that
preferences are not saved. Saved values the page does not recognise are ignored.

## Security model

The report shows test output, which may contain anything. It is built so that this content can
never act as markup, style or script:

- All data is escaped by `html/template` for its context. The code never converts data to
  `template.HTML`, `template.JS`, `template.CSS` or `template.URL`.
- The page carries a Content-Security-Policy: `default-src 'none'`, with the one inline stylesheet
  and the one inline script allowed by their SHA-256 hashes, `img-src data:`, `base-uri 'none'`,
  `form-action 'none'` and `object-src 'none'`, plus `referrer` `no-referrer`. The stylesheet and
  script are fixed files embedded at build time; no data is placed in them, and the script reads
  only `data-*` attributes and page text.
- Links are built only for anchors within the page and for attachment files whose names the renderer
  chooses. Links in the data are shown as text.
- `RenderDir` writes directories with mode 0700 and files with mode 0600, writes into a temporary
  directory that is then renamed, never overwrites, refuses a `cucumber.json`, run directory or
  output directory that is a symbolic link, and writes nothing outside the output directory.
- Error messages never include input content.

The report adds no redaction. Anything in `cucumber.json`, including attachments, appears in the
report as text. Remove secrets before they are attached.

## Limits

- Input: Godog's legacy Cucumber JSON. Every feature needs a scenario, every scenario a step and
  every step one of `passed`, `failed`, `skipped`, `pending`, `undefined`, `ambiguous`. Duplicate
  keys and data after the array are refused. Unknown fields (such as hooks from other runners) are
  ignored, so hooks are not shown.
- Images and video are not shown in this release.
- One run per report; there is no merging of runs and no trend over time.
- Pretty-printed JSON is written with HTML entities for quotes, so a report of mostly JSON
  attachments is about 1.4 to 1.5 times the decoded attachment size.

## Development

```sh
make verify          # gofmt check, go vet, golangci-lint, go test -race
make fuzz-smoke      # each fuzz target for FUZZTIME (default 30s)
make browser-check   # page script in a local headless Chrome, Chromium or Edge
make golden-update   # rewrite expected HTML after an intended change; review the diff
```

`testdata/synthetic/godog-v0.16.0.json` is real Godog v0.16.0 output for invented features;
`testdata/synthetic/godog-gen` regenerates it.

## Licence

MIT; see [LICENSE](LICENSE). The report layout and status semantics follow
multiple-cucumber-html-reporter; see [NOTICE](NOTICE).
