# Round-15 reviewer E2: real cucumber-ruby 9.2.1 captures (realistic suite, 16 new shapes)

Captured 2026-11-07, macOS arm64, rbenv cucumber 9.2.1, offline, CUCUMBER_PUBLISH_QUIET=true, no --publish.
Suite (../src, copied into each case): 4 features — checkout (Background, outline 10+5 examples w/ tagged Examples, Rule w/ Background,
data table, doc string, ANSI+HTML-ish 3 KB error, very long name, emoji/CJK names, markdown description), search (same scenario name as
checkout, nested `step` calls, deep multiline exception, pending w/ message, undefined, ambiguous, @skip_on_ci Before-hook
skip_this_scenario, png/jpg/svg/pdf/json/html/text attachments, outline 15 examples), francais (`# language: fr`, Règle), account (20 scenarios).
Env: CI=true -> skip_this_scenario; BROWSER=firefox -> gw-firefox fails; FLAKY=key:N -> fail first N attempts.
Each case: features/ blobs/ args.json summary.txt (cucumber tail + exit=N = truth) <tag>.json + <tag>.ndjson (same process) <tag>.rerun.txt <tag>.stderr.txt.
Capture: ../scripts/cap.sh ; evaluation: ../scripts/ev.py (CR=cr14) -> ../ev-single.out, ../ev-multi.out

| case | invocation | cucumber truth | cr14 JSON enh | cr14 Messages enh | verdict |
|---|---|---|---|---|---|
| s1-suite-full | (none) | 73 (10F,1U,1P,61 pass) exit=1 | Failed 10 exit3 | same | match |
| s2-suite-ci-skip | CI=true | 73 (10F,1S,1U,1P,60) exit=1 | Failed 10 | same | match |
| s3-retry2 | FLAKY=gw-retry:1 --retry 2 | 73 (10F,1 flaky,1U,1P,60) exit=1 | Failed 10 (1 flaky), attempts 3x10,2x1 | same | match |
| s4-retry-total1 | --retry 2 --retry-total 1 | 73 (11F,1U,1P,60) exit=1 | Failed 11, one 3-attempt case | same | match |
| s5-random-seed | --order random:4242 | 73 (10F,1U,1P,61) exit=1 | Failed 10 | same | match |
| s6-retry-strict | --retry 1 --strict | 73 (10F,1 flaky,1U,1P,60) exit=1 | Failed 10 (1 flaky) | same | match |
| p1-profile-ci | -p ci (cucumber.yml: --strict --retry 1 tags) CI=true | 23 (1 flaky,1S,21) exit=1 | Passed (1 flaky) exit0 | Runner reported failure exit3 | JSON greener (known JSON --strict class; gates.flaky fixes) |
| g1-nonfail-mix | --retry 1, tags | 23 (1 flaky,1S,21) exit=0 | Passed (1 flaky) | same | match |
| g2-pend-undef-skip-flaky | --retry 1, lines | 25 (1 flaky,1S,1U,1P,21) exit=0 | Undefined (1 flaky) exit0 | same | match |
| g3-same-strict | g2 + --strict | 25 (same) exit=1 | Undefined (1 flaky) exit0 | Undefined (1 flaky) exit0 (cr13: exit3) | Messages REGRESSION E2-H1 |
| h1-shards | Disjoint checkout / search+fr / account shards | 73 scenarios, 10 failed | merged, failed | same | supported in v0.3.0 |
| h2-shards-green | shard1 account / shard2 @flaky --retry 1 | 20 exit0, 2(1 flaky) exit0 | Passed (1 flaky) | same | match |
| m1-matrix-browser | BROWSER=chrome / firefox (same 22) | 22 pass exit0 / 22 (1F) exit1 | repeated inputs refused | repeated inputs refused | one report per input; merging deferred by user decision |
| m2-matrix-retry | chrome FLAKY --retry 1 / firefox --retry 1 | 22 (1 flaky) exit0 / 22 (1F) exit1 | repeated inputs refused | repeated inputs refused | one report per input; merging deferred by user decision |
| r1-rerun-suite | run1 full; run2 @run1.rerun.txt | 73 (11F..) exit1 ; 11 (10F,1 pass) exit1 | repeated inputs refused | repeated inputs refused | one report per input; merging deferred by user decision |
| r2-rerun-green | run1 tags; run2 @run1.rerun.txt | 22 (1F) exit1 ; 1 pass exit0 | repeated inputs refused | repeated inputs refused | one report per input; merging deferred by user decision |

All: counts/scenario totals/attempts/attachments (102..169) identical JSON vs Messages vs ndjson attachment envelopes; unicode, 3 KB error,
multiline backtrace, pending message, ambiguous text intact. ANSI SGR bytes are emitted raw into HTML <pre> and fingerprints (E2-M1, legacy v0.2.0).

All repeated-location multi-input examples above are now expected refusals. Run each artifact separately; disjoint shards remain supported. Historical “match” descriptions do not authorize merging reruns.
