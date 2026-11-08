# Round-14 reviewer Q2: new real cucumber-ruby 9.2.1 captures (round-13 features)

Captured 2026-11-07, macOS arm64, rbenv cucumber 9.2.1, offline, CUCUMBER_PUBLISH_QUIET=true, no --publish.
Each case: features/ (step defs driven by env PLAN=K:fail|K:failN|K:pending), args.json, summary.txt (cucumber progress tail + exit=N = truth),
<tag>.json + <tag>.ndjson (same process: --format json --format message), <tag>.rerun.txt, <tag>.stderr.txt.
Single-run cases use tag `run`; multi-run (rerun workflow) cases use run1/run2/run3 (separate processes, run2 = `@run1.rerun.txt`).
Capture script: ../cap.sh ; evaluation: ../ev.py, ../evA.sh (rerun orders), ../evC.sh (known issues, opts in ../opts), results in ../ev*.out.
e1/e2 blobs/ are the generated random attachments (regenerable; see features/support/att.rb).

| case | cucumber invocation(s) | cucumber truth (scenarios line, exit) |
|---|---|---|
| a1-rerun-allpass | run1: PLAN=A:fail,B:fail  ; run2: PLAN= @run1.rerun.txt | 4 scenarios (2 failed, 2 passed) exit=1 2 scenarios (2 passed) exit=0 |
| a2-rerun-diff | run1: PLAN=A:fail,B:fail  ; run2: PLAN=B:fail @run1.rerun.txt | 4 scenarios (2 failed, 2 passed) exit=1 2 scenarios (1 failed, 1 passed) exit=1 |
| a3-rerun-two-rounds | run1: PLAN=A:fail,B:fail,C:fail  ; run2: PLAN=B:fail,C:fail @run1.rerun.txt ; run3: PLAN=C:fail @run2.rerun.txt | 4 scenarios (3 failed, 1 passed) exit=1 3 scenarios (2 failed, 1 passed) exit=1 2 scenarios (1 failed, 1 passed) exit=1 |
| a3b-rerun-two-rounds-green | run1: PLAN=A:fail,B:fail,C:fail  ; run2: PLAN=B:fail,C:fail @run1.rerun.txt ; run3: PLAN= @run2.rerun.txt | 4 scenarios (3 failed, 1 passed) exit=1 3 scenarios (2 failed, 1 passed) exit=1 2 scenarios (2 passed) exit=0 |
| a4-rerun-retry | run1: PLAN=A:fail,B:fail1,C:fail --retry 1 ; run2: PLAN=A:fail1,C:fail --retry 1 @run1.rerun.txt | 4 scenarios (2 failed, 1 flaky, 1 passed) exit=1 2 scenarios (1 failed, 1 flaky) exit=1 |
| a5-rerun-outline | run1: PLAN=R2:fail,S2:fail  ; run2: PLAN= @run1.rerun.txt | 5 scenarios (2 failed, 3 passed) exit=1 2 scenarios (2 passed) exit=0 |
| a5b-rerun-outline-stillfail | run1: PLAN=R2:fail,S2:fail  ; run2: PLAN=S2:fail @run1.rerun.txt | 5 scenarios (2 failed, 3 passed) exit=1 2 scenarios (1 failed, 1 passed) exit=1 |
| a6-rerun-background | run1: PLAN=X2:fail  ; run2: PLAN= @run1.rerun.txt | 2 scenarios (1 failed, 1 passed) exit=1 1 scenario (1 passed) exit=0 |
| a6b-rerun-background-bgfail | run1: PLAN=BG:fail  ; run2: PLAN=BG:fail1 @run1.rerun.txt | 2 scenarios (2 failed) exit=1 2 scenarios (1 failed, 1 passed) exit=1 |
| a7-unrelated | run: PLAN=  | 2 scenarios (2 passed) exit=0 |
| a8-fullrerun-newfail | run1: PLAN=A:fail  ; run2: PLAN=D:fail  | 2 scenarios (1 failed, 1 passed) exit=1 2 scenarios (1 failed, 1 passed) exit=1 |
| b1-around-pass | run: PLAN=  | 3 scenarios (3 passed) exit=0 |
| b2-around-after-fail | run: PLAN=  | 3 scenarios (1 failed, 2 passed) exit=1 |
| b3-around-failing-step | run: PLAN=W1:fail  | 3 scenarios (1 failed, 2 passed) exit=1 |
| b4-around-retry | run: PLAN=W1:fail1,W2:fail --retry 1 | 3 scenarios (1 failed, 1 flaky, 1 passed) exit=1 |
| b4b-around-after-fail-retry | run: PLAN= --retry 1 | 3 scenarios (1 flaky, 2 passed) exit=0 |
| b5-around-attach-log | run: PLAN=W2:fail  | 3 scenarios (1 failed, 2 passed) exit=1 |
| c1-known-after-hook | run: PLAN=K1:fail  | 2 scenarios (1 failed, 1 passed) exit=1 |
| c2-known-second-fails | run: PLAN=K1:fail,K2:fail  | 2 scenarios (2 failed) exit=1 |
| c3-known-only | run: PLAN=K1:fail  | 2 scenarios (1 failed, 1 passed) exit=1 |
| c4-known-afterstep | run: PLAN=K1:fail  | 2 scenarios (2 failed) exit=1 |
| c5-known-retry-flaky-then-known | run: PLAN=K1:fail --retry 1 | 2 scenarios (1 failed, 1 passed) exit=1 |
| d1-tags-exclude-all | run: PLAN=Z:fail --tags not @a and not @b | 0 scenarios exit=0 |
| d2-empty-feature-file | run: PLAN=  | 0 scenarios exit=0 |
| d3-comments-only | run: PLAN=  | exit=2 |
| d4-dry-run-zero | run: PLAN= --dry-run --tags @none | 0 scenarios exit=0 |
| d5-feature-header-only | run: PLAN=  | 0 scenarios exit=0 |
| d6-zero-plus-failing-after-all | run: PLAN= --tags @none | 0 scenarios exit=2 |
| e1-large-attachments | run: PLAN=L:fail  | 2 scenarios (1 failed, 1 passed) exit=1 |
| e2-budget-spill-3mb | run: PLAN=L:fail  | 2 scenarios (1 failed, 1 passed) exit=1 |
| e3-big-text-only | run: PLAN=L:fail  | 1 scenario (1 failed) exit=1 |

## cr13 observed vs truth (-fail-on-unexpected; -bundle unless noted)
| case | JSON enhanced | Messages enhanced | verdict vs cucumber |
|---|---|---|---|
| a1/a5/a6/a3b ordered (final rerun green) | Passed (N flaky) exit 0 | Failed: quality gate exit 3 (runnerFailed from run1) | Messages REDDER (Q2-H1) |
| a2/a4/a5b/a3/a8 ordered | Failed exit 3, counts = latest attempt | same | match |
| any case REVERSED / shuffled | Failed (older run treated as final) | Failed | never greener (doc: keep order) |
| + unrelated extra run (first or last) | unchanged, +2 passed | unchanged | match |
| a6b ordered/reversed (Background failed in earlier input) | exit 1 "evidence floor ... reporter bug" | Failed exit 3 | JSON REFUSES (Q2-H2) |
| b1 / b3 / b4 | match | match | match |
| b2 around-after fail | Failed {failed 1} | Failed but counts {passed 3} + diagnostic | documented Ruby limitation |
| b4b around-after fail + retry | Passed (1 flaky) | Passed, 0 flaky; gates.flaky exit 0 | Messages loses flaky (Q2-H3) |
| b5 around + attach/log | Failed exit 3 | exit 1 "feature without scenarios or a step without a status" | Messages REFUSES (Q2-H4) |
| c1/c2/c4 known + other failure | Failed (known scenario stays failed) | same | match |
| c3 known only | Passed with known failures exit 0 | Failed exit 3 (runnerFailed) | JSON/Messages disagree (Q2-M1) |
| c5 known + --retry | exit 1 both IDs refused (-L2 "annotate -L2-2"; -L2-2 "names no scenario") | accepted, stays failed | JSON REFUSES config (Q2-M2) |
| d1/d2/d4/d5 zero | No scenarios executed exit 0; gates.empty -> exit 3 | same | match |
| d3 comments-only (cucumber parse crash exit 2) | refused (empty file) | refused (unfinished) | correct |
| d6 zero + failing AfterAll (cucumber exit 2) | No scenarios executed exit 0 | same (stream says success:true) | no signal in artifacts (Low) |
| e1 20x5MB png + 30MB text | no-bundle: exit 3, 20 png "omitted", text sidecar; RSS 2.1 GB | no-bundle ok, RSS 0.9 GB | -bundle exit 1 (Q2-H5) |
| e2 20x3MB png | no-bundle: 8 inline + 12 sidecar, verdict Failed | same | -bundle exit 1 (Q2-H5) |
| e3 30MB text | sidecar ok | ok | -bundle exit 1 (Q2-H5) |
| gates.flaky on b4/a1/retry3-flaky-green | exit 3 flaky | exit 3 flaky (except b4b) | match |
