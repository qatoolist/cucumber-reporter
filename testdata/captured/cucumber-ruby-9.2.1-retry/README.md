# Captured cucumber-ruby 9.2.1 output (real, not hand-built)

- Captured: 2026-11-07, macOS arm64, ruby 3.4.7, cucumber 9.2.1, cucumber-messages 22.0.0.
- Command: `cucumber --retry 2 --no-strict-flaky --format json --out retry.json` and `--format message --out retry.ndjson`. The feature files and step definitions are in `features/`.
- Expected truth:
  - **Flaky payment (L6):** fails on attempt 1, passes on attempt 2. Final status passed (flaky).
  - **Always fails (L10):** fails on 3 attempts. Final status failed.
  - **Discounts rows (L20, L21):** pass.
  - **Overall:** 1 failure, 1 flaky, 2 passed.

Observed facts about the format:
- **JSON:** every retry attempt is a separate pair of `background` and `scenario` elements with the same line, in attempt order. There is no attempt field.
- **JSON:** `output` arrays appear on steps and hooks (`log()`). Step-level `after` (AfterStep) entries and `embeddings` on `after` hooks also appear.
- **Messages:** `testCaseStarted.attempt` starts at **1**, not 0.
- **Messages:** `testCaseFinished.willBeRetried` is **false on every attempt**, including retried ones.

Result with cr10 (round 10):
- plain: "Failed: 4 unexpected failures". Wrong: every attempt is counted.
- `-enhanced` JSON: exit 1, "duplicate scenario; supply retries as attempts".
- `-enhanced -format messages`: exit 1, "attempt numbers must be contiguous from zero".
