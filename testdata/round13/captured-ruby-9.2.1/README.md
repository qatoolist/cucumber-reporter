# Round-13 reviewer W: real cucumber-ruby 9.2.1 captures

Captured 2026-11-07, macOS arm64, local rbenv cucumber 9.2.1, offline, CUCUMBER_PUBLISH_QUIET=true, no --publish.
Each dir: features/ (source), args.json (extra args), run.json (--format json), run.ndjson (--format message, same run), summary.txt (command + --format progress output + exit=N = truth), stderr.txt.

| case | extra args | cucumber scenarios line | exit | Failing Scenarios |
|---|---|---|---|---|
| after-flaky-retry | --retry 1 | 2 scenarios (1 failed, 1 flaky) | 1 | features/a.feature:6 |
| around-after-only |  | 2 scenarios (1 failed, 1 passed) | 1 | features/a.feature:4 |
| around-fail |  |  | 2 |  |
| around-fail-msgonly |  | 3 scenarios (2 failed, 1 passed) | 1 | features/a.feature:4; features/a.feature:8 |
| bg-only |  | 1 scenario (1 passed) | 0 |  |
| bg-only-alone |  | 0 scenarios | 0 |  |
| dry-run | --dry-run | 5 scenarios (4 skipped, 1 undefined) | 0 |  |
| fail-fast | --fail-fast | 2 scenarios (1 failed, 1 passed) | 2 | features/a.feature:6 |
| fail-fast-retry | --fail-fast --retry 1 | 2 scenarios (1 failed, 1 flaky) | 2 | features/a.feature:6 |
| hooks-fail |  | 5 scenarios (4 failed, 1 passed) | 1 | features/h.feature:4; features/h.feature:8; features/h.feature:12; features/h.feature:21 |
| multi-feature |  | 5 scenarios (2 failed, 3 passed) | 1 | features/b.feature:3; features/b.feature:6 |
| name-collide |  | 5 scenarios (1 failed, 4 passed) | 1 | features/n.feature:17 |
| nostrict-retry1 | --retry 1 | 4 scenarios (1 flaky, 1 undefined, 1 pending, 1 passed) | 0 |  |
| rerun-workflow |  | 6 scenarios (5 failed, 1 passed) / 5 scenarios (5 failed) | 1,1 | features/r.feature:3; features/r.feature:6; features/r.feature:9; features/r.feature:15; features/r.feature:18; features/r.feature:3; features/r.feature:6; features/r.feature:9; features/r.feature:15; features/r.feature:18 |
| retry-around-after | --retry 1 | 2 scenarios (1 failed, 1 passed) | 1 | features/a.feature:4 |
| retry-background | --retry 2 | 2 scenarios (1 failed, 1 flaky) | 1 | features/bg.feature:10 |
| retry-before-flaky | --retry 2 | 3 scenarios (1 failed, 1 flaky, 1 passed) | 1 | features/a.feature:6 |
| retry-hooks | --retry 2 | 5 scenarios (4 failed, 1 passed) | 1 | features/h.feature:4; features/h.feature:8; features/h.feature:12; features/h.feature:21 |
| retry-outline | --retry 2 | 5 scenarios (2 failed, 2 flaky, 1 passed) | 1 | features/o.feature:10; features/o.feature:15 |
| retry2-mixed | --retry 2 | 6 scenarios (4 failed, 1 flaky, 1 passed) | 1 | features/r.feature:6; features/r.feature:9; features/r.feature:15; features/r.feature:18 |
| retry3-flaky-green | --retry 3 | 2 scenarios (1 flaky, 1 passed) | 0 |  |
| retry3-mixed | --retry 3 | 6 scenarios (3 failed, 2 flaky, 1 passed) | 1 | features/r.feature:6; features/r.feature:9; features/r.feature:15 |
| rules-background |  | 4 scenarios (3 failed, 1 passed) | 1 | features/rules.feature:17; features/rules.feature:20; features/rules.feature:24 |
| strict-flaky-retry1 | --strict-flaky --retry 1 | 4 scenarios (1 flaky, 1 undefined, 1 pending, 1 passed) | 1 | features/s.feature:12 |
| strict-pending-retry1 | --strict-pending --retry 1 | 4 scenarios (1 flaky, 1 undefined, 1 pending, 1 passed) | 1 | features/s.feature:6 |
| strict-retry1 | --strict --retry 1 | 4 scenarios (1 flaky, 1 undefined, 1 pending, 1 passed) | 1 | features/s.feature:12; features/s.feature:9; features/s.feature:6 |
| strict-undefined-only | --strict-undefined | 2 scenarios (1 undefined, 1 passed) | 1 | features/s.feature:6 |
| strict-undefined-retry1 | --strict-undefined --retry 1 | 4 scenarios (1 flaky, 1 undefined, 1 pending, 1 passed) | 1 | features/s.feature:9 |
| undefined-only-default |  | 2 scenarios (1 undefined, 1 passed) | 0 |  |
| wip-allfail | --wip --tags @wip and not @x | 2 scenarios (2 failed) | 0 | features/w.feature:4; features/w.feature:8 |
| wip-mixed | --wip --tags @wip | 2 scenarios (1 failed, 1 passed) | 1 | features/w.feature:8 |
| zero-tags | --tags @does_not_exist | 0 scenarios | 0 |  |

Notes: around-fail = cucumber's JSON formatter crashes (NoMethodError json.rb:146 around_hooks) -> empty run.json, truncated ndjson, exit 2. around-fail-msgonly = same features with only --format message (exit 1). rerun-workflow = run1 (--format rerun --out rerun.txt) + run2 (@rerun.txt), two separate processes. Ruby `step 'undefined'` inside a step (fundef) is reported by cucumber as failed, not undefined.
