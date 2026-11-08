"""Proves interactions.py can fail: each mutant breaks one behaviour in a copy of the report,
and exactly the checks listed for it must fail.

Usage: python interactions_mutants.py RETRY_REPORT EMPTY_REPORT MESSAGES_RETRY_REPORT [engine]   (default chromium)

The report's Content-Security-Policy pins a hash of its inline script, so each mutant also
replaces that hash; otherwise the page would not run at all and every check would fail.
"""

import base64
import hashlib
import re
import subprocess
import sys
import tempfile
from pathlib import Path

reports = dict(zip(("retry", "empty", "messages"), (Path(x).resolve() for x in sys.argv[1:4])))
engine = sys.argv[4] if len(sys.argv) > 4 else "chromium"
checks = Path(__file__).with_name("interactions.py")
DENSITY = "P.density=v;applyPrefs();rerender();"

HEADING = "if(q.has('step')||q.has('attempt'))focusStepTarget(q);else main.querySelector('.page-t')?.focus()"
EXPAND = "if(button){if(button.getAttribute('aria-expanded')!=='true')button.click();target=button;}"
SETTINGS = "settings_keep_scenario_state"
HISTORY = "back_forward_focus"
DRAWER = "mobile_drawer_closes_on_back"
EVIDENCE = "evidence_reveal_after_collapsing_attempts"
PALETTE = "palette_focuses_picked_step"
EMPTY = "empty_report_is_informational"
TIMES = "attempt_times_distinguish_attempts"

# name: (report to mutate, original text, replacement, the checks that must fail and no others)
MUTANTS = {
    "density-no-op": ("retry", DENSITY, "applyPrefs();rerender();", {SETTINGS}),
    "density-scroll-to-top": ("retry", DENSITY, "P.density=v;applyPrefs();scrollTo(0,0);rerender();", {SETTINGS}),
    "density-scroll-drift": ("retry", DENSITY, "P.density=v;applyPrefs();scrollBy(0,-30);rerender();", {SETTINGS}),
    # Only the in-scenario route change; the other drawer close paths stay intact.
    "drawer-stays-open-on-back": ("retry", "toggleSide(false);\n" + HEADING, "\n" + HEADING, {DRAWER}),
    # Back to the bare scenario no longer focuses the heading (the mobile check requires it too).
    "back-loses-heading-focus": ("retry", HEADING, HEADING.replace("main.querySelector('.page-t')?.focus()", "void 0"), {HISTORY, DRAWER}),
    # Every in-scenario step link (Jump to step, Forward, View evidence, palette) lands on step 0.
    "step-link-wrong-target": ("retry", HEADING, HEADING.replace("focusStepTarget(q)", "focusStepTarget(new URLSearchParams('step=0'))"),
                               {SETTINGS, HISTORY, DRAWER, EVIDENCE, PALETTE}),
    "evidence-not-expanded": ("retry", EXPAND, "if(button){target=button;}", {EVIDENCE}),
    "palette-wrong-step": ("retry", "navigate(opts[sel].getAttribute('href'))",
                           "navigate(opts[sel].getAttribute('href').replace(/step=\\d+/,'step=0'))", {PALETTE}),
    "attempt-label-without-clock": ("messages", "+(clockOf(a.started,3)?' · '+clockOf(a.started,3):'')", "", {TIMES}),
    "empty-read-error": ("empty", '"label":"No scenarios executed"', '"label":"could not be read"', {EMPTY}),
}


def digest(text):
    return "sha256-" + base64.b64encode(hashlib.sha256(text.encode()).digest()).decode()


def mutate(source, old, new):
    html = source.read_text()
    assert old in html, f"mutation point not found: {old}"
    out = html.replace(old, new)
    for script in re.findall(r"<script(?: [^>]*)?>([\s\S]*?)</script>", html):
        if old in script:
            # The data block is not executable and has no hash; a missed script hash would
            # stop the page and fail every check, which the "only that check" rule reports.
            out = out.replace(digest(script), digest(script.replace(old, new)))
    return out


ALL = {SETTINGS, HISTORY, DRAWER, EVIDENCE, PALETTE, TIMES, EMPTY}
RESULT = re.compile(r"^(\w+): (PASS|FAIL) (\w+)", re.M)


def judge(stdout, returncode, expected):
    """None when the run is a complete, attributed kill; otherwise the reason it is not proof."""
    if returncode != 1:
        return f"exit code {returncode}, expected 1 (a failed check, not a crash or timeout)"
    results = [(e, verdict, check) for e, verdict, check in RESULT.findall(stdout)]
    if any(e != engine for e, _, _ in results):
        return "results for another engine"
    names = [check for _, _, check in results]
    if sorted(names) != sorted(ALL):
        return f"incomplete or duplicated results: {sorted(names)}"
    failed = {check for _, verdict, check in results if verdict == "FAIL"}
    if failed != expected:
        return f"expected exactly {sorted(expected)} to fail, got {sorted(failed) or 'no failure'}"
    return None


# The runner must not mistake a crash or a truncated run for a kill.
def lines(passed, failed):
    return "".join(f"{engine}: PASS {c}\n" for c in passed) + "".join(f"{engine}: FAIL {c}: x\n" for c in failed)


complete = lines(sorted(ALL - {DRAWER}), [DRAWER])
assert judge(complete, 1, {DRAWER}) is None
assert judge(lines([], [DRAWER]), -9, {DRAWER})  # expected failure, then killed
assert judge(lines([], [DRAWER]), 1, {DRAWER})  # expected failure, then output truncated
assert judge(complete + f"{engine}: PASS {SETTINGS}\n", 1, {DRAWER})  # duplicate result
assert judge(complete, 0, {DRAWER})  # suite claimed success
assert judge(lines(sorted(ALL - {SETTINGS}), [SETTINGS]), 1, {DRAWER})  # killed by the wrong check
assert judge(complete, "timeout after 600 s", {DRAWER})  # timed out

problems = []
with tempfile.TemporaryDirectory(prefix="cr-ui-mutants-") as work:
    for name, (which, old, new, expected) in MUTANTS.items():
        source = reports[which]
        mutant = Path(work) / f"{name}.html"
        mutant.write_text(mutate(source, old, new))
        args = [str(mutant if key == which else path) for key, path in reports.items()]
        try:
            run = subprocess.run([sys.executable, "-u", str(checks), *args, engine], capture_output=True, text=True, timeout=600)
            stdout, code = run.stdout, run.returncode
        except subprocess.TimeoutExpired as e:
            out = e.stdout or b""
            stdout, code = out.decode(errors="replace") if isinstance(out, bytes) else out, "timeout after 600 s"
        problem = judge(stdout, code, expected)
        if problem:
            problems.append(f"{name}: {problem}\n{stdout[-2000:]}")
        print(f"{engine}: mutant {name} -> {'not proven: ' + problem if problem else 'killed by ' + ', '.join(sorted(expected))}")

if problems:
    sys.exit("surviving or misattributed UI mutants:\n" + "\n".join(problems))
print(f"{engine}: {len(MUTANTS)} UI mutants each killed by exactly the expected checks")
