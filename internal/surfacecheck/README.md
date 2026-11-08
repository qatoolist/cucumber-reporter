# Public surface corpus check

This harness is adapted from reviewer X's round-14 public API harness. It checks
plain outcomes, enhanced outcomes, HTML, JSON/Markdown/CSV exports, rerun selectors
and gate results through the public library across three gate policies. Inputs
separated by commas exercise the documented refusal of repeated locations.

Run `make surface-check` for all unique repository fixtures, including cross-input refusal sets.
`python3 internal/surfacecheck/run.py <additional-fixture-root>` adds review fixtures.

For a selected subset, run `go run internal/surfacecheck/check.go <fixture.json> <fixture.ndjson> ...`.
Pass the captured and synthetic fixtures; malformed inputs are reported as refusals.
The command exits nonzero on a surface mismatch. This is supplementary to the normal
unit, real-runner truth, browser and v0.2.0 compatibility suites.

Corrections to the reviewer's original harness are explicit:

- Empty execution is undefined in the existing status API and informational in HTML.
- `run2` and `run3` use their own exit status in multi-command capture summaries.
- Rerun selectors split on newlines, preserving feature paths containing spaces.
- Legacy JSON does not encode strict-pending or strict-flaky runner policy. Those captures (including the `p1-profile-ci` strict-flaky profile) require corresponding gates to reproduce the runner's exit code. Their
  all-gates checks still apply; Messages also preserves the runner-failure signal.
- Multi-input repeated locations must be refused. Disjoint shard checks are separate.

No expected result or input fixture is modified by this harness.


Run `python3 internal/surfacecheck/mutations.py` to disable each outcome rule in an
isolated source copy and require the runtime safeguard to refuse. It covers Messages
and JSON retries, known-issue isolation, Background folding, gates, runner signals,
Around evidence, cross-input refusal and empty execution. It never mutates the working tree.

Round-16 acceptance is two-way for captured runner truth: counts, failing locations,
policy exit and tone must match the console summary, except the named format-loss
and conservative retry cases in runnercheck. Synthetic/configuration inputs with
no console truth are explicitly labelled SURFACE-ONLY; they provide parity and
adversarial coverage, not independent runner truth.

`expected-refusals.json` pins 246 refusal observations across 524 unique inputs
and 28 repeated-input sets, plus 1,612 monotonicity checks. These are disjoint-input
checks: each accepted input is prepared again with a separate probe input holding one
failed or ambiguous scenario, in both input orders, and must stay failed with a gate
failure. They do not inject cases into the same stream; see the same-stream tests
below. The observations include
options, truth and args JSON intentionally passed as report input; truncated
formatter output; unsupported behave durations; adversarial duplicate statuses or
stream events; and documented ambiguous/in-file or cross-input repeats. No new
refusal is accepted automatically. The real-capture gate separately pins its 16
supported-policy refusals, so configuration JSON is never counted as refused real
runner output. Additional fixture roots require a separately reviewed inventory.

`TestRound16SameStreamMonotonicity` adds pending, undefined and rescued-failure
attempts to the same raw stream in both formats, with and without known annotations.
It also removes a failed attempt while retaining its passed rescue. These Go tests
supplement, rather than replace, the large cross-input surface property checks.
`TestRound17SameStreamMonotonicityGatesOff` repeats the same-stream property with
every quality gate off over every k* capture (k01–k26) and every strict capture,
in both formats, so only outcome and runner-signal rules decide. Adding a pending,
undefined, failed or rescued-failure case must never make a run greener, and
removing it must restore the exact verdict label, exit code, counts, gate list and
runner signal (352 checks).

`outcome-rules.json` binds the reviewed mutation inventory to outcome source files,
including the evidence floor itself (every non-test file in `internal/evidence`,
which is checked for completeness, plus `report/evidence.go` and `report/model.go`).
A rule edit requires reviewing and updating that inventory. CLI-effective mutants
must fail Go tests and be refused by the runtime safeguard. Defensive model-only
branches U1–U5 have direct unit mutants; these redundant branches cannot change a
real CLI verdict alone, and the gate does not mislabel them as runtime kills. The
same applies to the reviewer r17 singles S4b, R1 and R2, to each floor rule alone
(F1–F6: CheckRunner causes, flaky cause, soft cause and coverage, CheckCases
incomplete execution, Messages.Check), and to the doubles E1, E2, E3, E7, R4 and R5
that disable a floor rule together with the production rule it guards: those are
killed by the Go regression tests only. K4 (an issue policy's not-applied decision
lost) is refused at runtime by the floor alone, which re-derives environment, expiry and error-signature exclusions from the raw
RunConfig issues, on k22–k25 in both formats. K1 (stepText check skipped) is refused at
runtime by the floor alone on k22–k25 with a non-matching annotation.
