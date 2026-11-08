# Real runner gate

`make corpus-check` runs all 122 real Ruby capture groups, plus the hand-built
runner fixtures, against a freshly built CLI. It checks exact exit policy, verdict tone, scenario counts and failing-scenario locations against the captured console summary. JSON also runs
in plain mode. The three omitted large artifacts are regenerated with real
cucumber-ruby 9.2.1 in temporary directories at their documented sizes. Missing
runner dependencies fail the gate; they do not skip it.

Documented refusals are asserted, not skipped:

- `round13/.../around-fail`: Ruby's JSON formatter crashed. JSON is empty and the
  Messages stream is unfinished. The complete Messages-only capture is tested.
- `round14/.../d3-comments-only`: Ruby aborted before writing a complete artifact.
- Runner `cucumber-jvm-7-truncated` and `cucumber-js-10-messages-crashed`: intentionally
  truncated evidence must be refused.
- Runner `behave-1.2.6*`: native behave JSON is outside the documented input format.
- Runner `dup-jvm-surefire-rerun`: unconfirmed duplicate shape is refused in enhanced
  mode and retained as separate executions in plain mode, per its truth file.
- Cross-input repetitions: chrome/firefox, Q2 `a*`, E2 `m1/m2`, `r1/r2` and A2/sp
  must be refused. Their CLI/PrepareRun tests and the cross-input runtime mutant
  enforce this v0.3.0 policy. No cross-input rerun feature ships.

Two limitations cannot be inferred from artifact contents:

- Legacy JSON does not record strict/strict-flaky runner policy. Enhanced runs
  supply the corresponding gates from the capture command; plain runs compare
  execution evidence without claiming to reproduce unavailable runner policy.
- `d6-zero-plus-failing-after-all`: Ruby writes JSON `[]` and Messages
  `success:true`, then its AfterAll hook crashes outside those files. The gate
  requires an empty informational report, never a green Passed verdict. The
  reporter cannot reconstruct the omitted failure from either input. This is
  documented in the v0.3.0 support matrix.

These exceptions are explicit fixture-specific assertions. The independent
surface and mutation gates run separately and fail on unexpected refusals or
surviving mutations in their supported cases.

Round 16 adds 25 capture groups, including k23/k24 with known-issue options in
both formats. `k02-beforeall` records exit 2 before either formatter creates a
file; the gate explicitly asserts this missing-artifact case. The refusal
inventory remains exactly 16 and any newly refused input fails.

Round 17 adds `k26-strict-pending-known-z` (strict pending plus a failing
scenario; cucumber exits 1).

Pinned differences (not broad permission for a redder/greener result). Each is keyed
by the capture's full relative path, never a bare directory name, and every applied
difference prints a `DOCUMENTED ...` line (count, tone, exit, missing evidence or
policy):

- `retryfinal`: report counts are exactly failed=2, passed=1. Ruby counts one
  failed-to-skipped execution as flaky, but it did not strictly pass and is named
  in the failing-scenario list. Both unrescued failures remain visible.
- `k14-retry-fail-skip`: failed=1, passed=3, failed location a.feature:2, exit 3.
  Ruby exits 0 after skip_this_scenario; this release's documented rescue contract
  remains conservative.
- Messages `around-after-only`, `retry-around-after`, `around-fail-msgonly`,
  `b2-around-after-fail`: exact available case counts and failure locations are
  pinned because the runner omits the Around-after result. The run stays failed.
- `d6-zero-plus-failing-after-all`, `k01-afterall-pass`, `k17-afterall-retry-flaky`:
  AfterAll fails outside the emitted artifacts. The CI command must preserve the
  runner's exit independently of report generation.
- JSON `k19-wip-pass` does not encode WIP policy. `wip-allfail` remains failed in
  both formats: the reporter does not invert failed evidence into a pass.
- Plain JSON does not infer strict policy. Its tone and counts are nevertheless
  checked against available execution evidence, including hard failures; no
  blanket “plain exit 0 means any tone is acceptable” exemption remains.

Cross-surface checks independently read scenario counts from summary.txt, compare
exact per-scenario states among every output/API, and pin their entire refusal
inventory plus property-test count. Synthetic files without console truth are
cross-surface probes; they are not counted as real-capture truth checks.
