# Real cucumber-ruby 9.2.1 captures (round 11 gate)

Captured 2026-11-07 on macOS arm64 with the local `~/.rbenv/shims/cucumber` 9.2.1, run offline with
`CUCUMBER_PUBLISH_QUIET=true` and no `--publish`. Each directory holds `features/` (the source),
`run.json` (`--format json`), `run.ndjson` (`--format message`), `summary.txt` (`--format progress`
plus `exit=N`, cucumber's own exit status) and `stderr.txt`. `out/` holds scratch reports from probe.py.
Command: `cucumber --no-color [EXTRA] --format progress --out summary.txt --format json --out run.json --format message --out run.ndjson`.

Truth = cucumber's own summary line and exit status (below). `Failing Scenarios:` lists the scenarios cucumber itself treats as failures.

| dir | extra args | cucumber scenarios line | steps line | exit | Failing Scenarios | testRunFinished.success |
| --- | --- | --- | --- | --- | --- | --- |
| attach |  | 2 scenarios (1 failed, 1 passed) | 2 steps (1 failed, 1 passed) | 1 | features/a.feature:6 | false |
| background |  | 2 scenarios (2 failed) | 4 steps (2 failed, 2 skipped) | 1 | features/bg.feature:6; features/bg.feature:9 | false |
| flakyonly-strict | --retry 1 --strict-flaky | 2 scenarios (1 flaky, 1 passed) | 3 steps (1 failed, 2 passed) | 1 | features/f.feature:6 | false |
| flakyonly | --retry 1 | 2 scenarios (1 flaky, 1 passed) | 3 steps (1 failed, 2 passed) | 0 |  | true |
| hooks |  | 4 scenarios (3 failed, 1 passed) | 5 steps (2 skipped, 3 passed) | 1 | features/hooks.feature:7; features/hooks.feature:11; features/hooks.feature:15 | false |
| pendonly-strict | --strict | 2 scenarios (1 pending, 1 passed) | 2 steps (1 pending, 1 passed) | 1 | features/p.feature:6 | false |
| pendonly |  | 2 scenarios (1 pending, 1 passed) | 2 steps (1 pending, 1 passed) | 0 |  | true |
| pendpass-default |  | 3 scenarios (1 undefined, 1 pending, 1 passed) | 3 steps (1 undefined, 1 pending, 1 passed) | 0 |  | true |
| pendpass-nostrict | --no-strict | 3 scenarios (1 undefined, 1 pending, 1 passed) | 3 steps (1 undefined, 1 pending, 1 passed) | 0 |  | true |
| pendpass-strict | --strict | 3 scenarios (1 undefined, 1 pending, 1 passed) | 3 steps (1 undefined, 1 pending, 1 passed) | 1 | features/p.feature:9; features/p.feature:6 | false |
| retryfinal | --retry 2 | 3 scenarios (1 failed, 2 flaky) | 6 steps (3 failed, 1 skipped, 1 pending, 1 passed) | 1 | features/r.feature:3; features/r.feature:6 | false |
| shapes |  | 6 scenarios (2 failed, 4 passed) | 12 steps (2 failed, 10 passed) | 1 | features/shapes.feature:17; features/shapes.feature:36 | false |
| skippass |  | 2 scenarios (1 skipped, 1 passed) | 2 steps (1 skipped, 1 passed) | 0 |  | true |
| statuses-nostrict | --no-strict | 5 scenarios (2 failed, 1 undefined, 1 pending, 1 passed) | 12 steps (2 failed, 4 skipped, 1 undefined, 1 pending, 4 passed) | 1 | features/st.feature:16; features/st.feature:21 | false |
| statuses-strict | --strict | 5 scenarios (2 failed, 1 undefined, 1 pending, 1 passed) | 12 steps (2 failed, 4 skipped, 1 undefined, 1 pending, 4 passed) | 1 | features/st.feature:16; features/st.feature:21; features/st.feature:11; features/st.feature:6 | false |
| statuses |  | 5 scenarios (2 failed, 1 undefined, 1 pending, 1 passed) | 12 steps (2 failed, 4 skipped, 1 undefined, 1 pending, 4 passed) | 1 | features/st.feature:16; features/st.feature:21 | false |

Per-scenario truth notes:
- **hooks**: L3 passed; L7 Before hook raises (failed, step skipped); L11 After hook raises (failed); L15 AfterStep raises after step 1 (failed, step 2 skipped).
- **background**: Background step raises; both scenarios failed, their own steps skipped.
- **statuses** (+ -strict/-nostrict; identical JSON/NDJSON shape, only exit/success differ): L3 passed, L6 pending, L11 undefined, L16 ambiguous (cucumber counts it as failed), L21 failed.
- **retryfinal** (`--retry 2`): L3 fails then `pending`; L6 fails then `skip_this_scenario`; L9 fails then passes (flaky). Cucumber exits 1 and lists L3 and L6 as Failing Scenarios. A truthful report is NOT green.
- **shapes**: outline name placeholders (`Login as <role> with <count> items`), two Examples tables (one tagged `@ex_tag`), feature tag `@feature_tag` inherited, doc string (`json` media type) and data table, a `Rule:` with its own Background. L17 (count 9) and L36 (Under rule fails) fail.
- **attach**: step attaches text, png, json and calls `log`; Before hook `log`s; After hook attaches a png when the scenario failed plus text always.
- **pendpass-***: passed + pending + undefined. Exit 0 by default/--no-strict, 1 with --strict.
- **pendonly / pendonly-strict**: passed + pending. Exit 0 default, 1 with --strict (success:false in NDJSON).
- **skippass**: passed + `skip_this_scenario`. Exit 0.
- **flakyonly / flakyonly-strict**: one flaky (fail then pass) + one passed. Exit 0, or 1 with `--strict-flaky` (success:false).

cr11 (round 11) observed, plain / -enhanced JSON / -enhanced -format messages (see ../capmatrix.txt):
- Matches cucumber: attach, background, hooks, shapes, statuses*, pendpass-*, skippass, pendonly, flakyonly.
- **retryfinal**: every mode says `Passed` (exit 0 with -fail-on-unexpected; rerun.txt empty). Cucumber: exit 1, L3+L6 failing. GREENER THAN CUCUMBER.
- **pendonly-strict**: every mode says `Passed`; the NDJSON carries `success:false`.
- **flakyonly-strict**: JSON modes say Passed (JSON has no strictness signal); `-format messages` is REJECTED (exit 1, "input contains a feature without scenarios or a step without a status") because success:false with all-passed final attempts trips ErrIncomplete (internal/cucumberinput/messages.go:552-557).
- **attach** plain: the After-hook failure screenshot (png) and After text are dropped (documented "passing hooks stay hidden"); the Before-hook `log` is shown as "Runner output" on step 1 after the step's own output.
- **shapes**: the Rule name is not represented in any mode; Messages keyword is "Scenario" where JSON says "Example".
