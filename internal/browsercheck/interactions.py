"""Real-browser interaction checks for the scenario page, settings, history, mobile drawer,
evidence reveal, command palette and empty report.

Usage: python interactions.py RETRY_REPORT EMPTY_REPORT MESSAGES_RETRY_REPORT [chromium|firefox|webkit ...]

RETRY_REPORT is the enhanced report of testdata/captured/cucumber-ruby-9.2.1-retry/retry.json
(one failed scenario with three attempts); EMPTY_REPORT is the enhanced report of `[]`;
MESSAGES_RETRY_REPORT is the enhanced report of testdata/round15/captured/s3-retry2/run.ndjson
(Cucumber Messages, so every attempt has runner start and finish timestamps).
Set CR_UI_SCREENSHOTS=<dir> to save a screenshot of every failure.
Every wait is on a condition, not a fixed delay, so slower engines and runners only take longer.
"""

import json
import os
import re
import sys
from pathlib import Path
from urllib.parse import quote
from playwright.sync_api import sync_playwright

retry_file, empty_file, messages_file = (Path(x).resolve() for x in sys.argv[1:4])
retry, empty, messages = retry_file.as_uri(), empty_file.as_uri(), messages_file.as_uri()
engines = sys.argv[4:] or ["chromium", "firefox", "webkit"]
shots = (
    Path(os.environ["CR_UI_SCREENSHOTS"])
    if os.environ.get("CR_UI_SCREENSHOTS")
    else None
)

data = json.loads(
    re.search(r'id="report-data">([\s\S]*?)</script>', retry_file.read_text()).group(1)
)
SCENARIO = next(
    s["id"] for f in data["features"] for s in f["elements"] if s["status"] == "failed"
)
# Same escaping as the app's encodeURIComponent, which leaves !'()* literal.
SCENARIO_HREF = "#/scenarios/" + quote(SCENARIO, safe="!'()*")

# Counts scroll events, and records whether the read-failure banner was ever visible.
PROBE = """
window.__cr = {scrolls: 0};
addEventListener('scroll', () => { __cr.scrolls++; }, {passive: true});
new MutationObserver(() => {
  if ((document.body && document.body.innerText || '').includes('could not be read')) window.__crBanner = true;
}).observe(document, {subtree: true, childList: true, characterData: true});
"""
DESC = """(() => { const e = document.activeElement; if (!e || e === document.body) return 'body';
  return e.tagName.toLowerCase() + (e.id ? '#' + e.id : '') + ' "' + (e.getAttribute('aria-label') || e.textContent || '').trim().slice(0, 50) + '"'; })()"""
IN_VIEW = "(e) => { const r = e.getBoundingClientRect(); return r.bottom > 0 && r.top < innerHeight && r.width > 0 && r.height > 0; }"
# Resolves once the scroll position has been unchanged for 10 consecutive animation frames.
SETTLE = """() => new Promise((done, fail) => { let last = NaN, same = 0; const deadline = performance.now() + 5000;
  setTimeout(() => fail(new Error('scrolling did not settle within 6 s (animation frames stopped?)')), 6000);
  const tick = () => { const y = Math.round(scrollY); same = y === last ? same + 1 : 0; last = y;
    if (same >= 10) done(y); else if (performance.now() > deadline) fail(new Error('scrolling did not settle within 5 s (scrollY ' + y + ')')); else requestAnimationFrame(tick); };
  requestAnimationFrame(tick); })"""
STEP_FOCUSED = "() => /^(attempt-\\d+-)?step-\\d+$/.test(document.activeElement.id)"
H1_FOCUSED = "() => document.activeElement === document.querySelector('h1.page-t')"


def until(page, expression, what):
    try:
        page.wait_for_function(expression, timeout=10000)
    except Exception:
        raise AssertionError(f"{what} (focus {page.evaluate(DESC)})") from None


def settle(page):
    try:
        return page.evaluate(SETTLE)
    except Exception as e:
        raise AssertionError(str(e).splitlines()[0]) from None


def open_scenario(page):
    page.goto(retry + SCENARIO_HREF)
    page.locator(".v-sc .page-t").wait_for()
    settle(page)


def link(page, label):
    return page.locator("a.btn", has_text=label).first


def attempt(page, index):
    return page.locator(f'.attempts [data-attempt-index="{index}"]')


def jump_to_step(page):
    jump = link(page, "Jump to step")
    expected = "step-" + jump.get_attribute("href").split("step=", 1)[1].split("&", 1)[0]
    jump.click()
    until(page, f"() => document.activeElement.id === {json.dumps(expected)}", f"Jump to step did not focus {expected}")
    settle(page)
    return expected


def click_in_place(page, selector):
    """Click like a person: at the element's on-screen position. Playwright's locator.click()
    first scrolls its target into view, which moves the page for sticky top-bar buttons."""
    target = page.locator(selector).first
    target.wait_for(state="visible")
    box = target.bounding_box()
    x, y = box["x"] + box["width"] / 2, box["y"] + box["height"] / 2
    hit = target.evaluate("(e, p) => e.contains(document.elementFromPoint(p[0], p[1]))", [x, y])
    assert hit, f"{selector} is covered or off screen at its centre"
    page.mouse.click(x, y)


ANCHOR_TOP = "id => Math.round(document.getElementById(id).getBoundingClientRect().top)"


def settings_keep_scenario_state(page):
    page.set_viewport_size({"width": 1280, "height": 500})  # leave room to scroll
    open_scenario(page)
    assert page.locator(".attempts [data-attempt-index]").count() == 3, "expected three attempts"
    step = jump_to_step(page)
    # Collapse all also closes "Attempt details", which holds the attempt switch, so pick the attempt first.
    attempt(page, 0).click()
    page.locator(".step-controls [role=radio]", has_text="Problems only").click()
    page.get_by_role("button", name="Collapse all", exact=True).click()
    settle(page)
    # Scroll anchoring differs by engine (WebKit has none); switch it off so the page may not move at all.
    # (Set through the style property: the report's CSP rightly refuses injected <style>.)
    page.evaluate("for (const e of [document.documentElement, document.body]) e.style.overflowAnchor = 'none'")
    # Start mid-page with the failed step 150 px below the top, so a jump in either direction shows.
    page.evaluate(f"scrollBy(0, {page.evaluate(ANCHOR_TOP, step)} - 150)")
    y0 = settle(page)
    assert y0 >= 100, f"page too short to test scroll preservation: scrollY={y0}"
    assert page.evaluate("document.documentElement.dataset.density") != "compact"
    page.evaluate("document.querySelector('.step-controls').__crMark = 1")
    route = page.evaluate("location.hash")
    click_in_place(page, '[aria-label="Settings"]')
    dialog = page.locator("dialog[open]")
    dialog.wait_for()
    dialog.get_by_text("Compact", exact=True).click()
    until(page, "() => document.documentElement.dataset.density === 'compact'", "Compact did not take effect")
    y1 = settle(page)
    st = page.evaluate("""({inDialog: !!document.querySelector('dialog[open]')?.contains(document.activeElement),
      text: (document.activeElement.textContent || '').trim()})""")
    assert abs(y1 - y0) <= 1, f"page behind the dialog moved: scrollY {y0}->{y1}"
    assert st["inDialog"] and st["text"] == "Compact", f"focus left Compact: {page.evaluate(DESC)}"
    page.keyboard.press("Escape")
    until(page, "() => !document.querySelector('dialog[open]')", "Settings did not close on Escape")
    y2 = settle(page)
    assert abs(y2 - y0) <= 1, f"page moved when Settings closed: scrollY {y0}->{y2}"
    kept = page.evaluate("""({
      same: document.querySelector('.step-controls')?.__crMark === 1,
      problems: document.querySelector('.step-controls [aria-checked=true]')?.textContent,
      attempt: document.querySelector('.attempts [aria-checked=true]')?.dataset.attemptIndex,
      collapsed: [...document.querySelectorAll('main details')].filter(x => !x.closest('.acc')).every(x => !x.open),
      density: document.documentElement.dataset.density, route: location.hash})""")
    assert kept == {"same": True, "problems": "Problems only", "attempt": "0", "collapsed": True,
                    "density": "compact", "route": route}, f"scenario state changed: {kept}"


def back_forward_focus(page):
    open_scenario(page)
    step = jump_to_step(page)
    page.go_back()
    until(page, H1_FOCUSED, "Back did not focus the heading")
    y = settle(page)
    assert y <= 2, f"Back did not return to the top: scrollY={y}"
    page.go_forward()
    until(page, f"() => document.activeElement.id === {json.dumps(step)}", f"Forward did not focus {step}")
    settle(page)
    assert page.evaluate(IN_VIEW, page.evaluate_handle("document.activeElement")), "focused step is out of view"
    st = page.evaluate("""(() => { const s = getComputedStyle(document.activeElement);
      return {fv: document.activeElement.matches(':focus-visible'), style: s.outlineStyle, width: parseFloat(s.outlineWidth) || 0}; })()""")
    assert st["fv"] or (st["style"] not in ("none", "") and st["width"] > 0), f"focused step has no visible focus: {st}"


def mobile_drawer_closes_on_back(page):
    page.set_viewport_size({"width": 375, "height": 800})
    open_scenario(page)
    step = jump_to_step(page)
    top0 = page.evaluate(ANCHOR_TOP, step)
    click_in_place(page, ".menu-btn")
    until(page, "() => !document.querySelector('#side').inert", "drawer did not open")
    settle(page)
    assert abs(page.evaluate(ANCHOR_TOP, step) - top0) <= 2, "page behind the drawer scrolled when it opened"
    page.go_back()
    until(page, "() => document.querySelector('#side').inert", "drawer stayed open after Back")
    until(page, H1_FOCUSED, "Back did not focus the heading")
    settle(page)
    st = page.evaluate("""({hidden: document.querySelector('#side').getAttribute('aria-hidden'),
      expanded: document.querySelector('.menu-btn').getAttribute('aria-expanded'),
      overflow: document.documentElement.scrollWidth > innerWidth + 1})""")
    assert st == {"hidden": "true", "expanded": "false", "overflow": False}, st


def evidence_reveal_after_collapsing_attempts(page):
    open_scenario(page)
    attempt(page, 2).click()
    page.evaluate(
        "for (const d of document.querySelectorAll('.attempts details')) d.open = false"
    )
    link(page, "View evidence").click()
    until(
        page,
        "() => document.activeElement.classList.contains('att-h') && document.activeElement.getAttribute('aria-expanded') === 'true'",
        "View evidence did not focus an expanded attachment",
    )
    settle(page)
    st = page.evaluate("""(() => { const p = document.getElementById(document.activeElement.getAttribute('aria-controls'));
      return {panelHidden: p ? p.hidden : null, image: !!(p && p.querySelector('img'))}; })()""")
    # The link targets the final attempt's own screenshot, in the main step list.
    assert st == {"panelHidden": False, "image": True}, st
    assert page.evaluate(IN_VIEW, page.evaluate_handle("document.activeElement")), (
        "evidence is out of view"
    )


def palette_focuses_picked_step(page):
    open_scenario(page)
    # Pick the failed step, not the first one, so landing on step 0 cannot pass.
    failed = page.locator(".v-sc .steps li.s-failed").first
    failed_id = failed.get_attribute("id")
    text = failed.locator(".txt").inner_text().strip()
    assert failed_id and failed_id != "step-0", f"expected a failed step after step 0, got {failed_id}"
    page.locator("body").press("/")
    box = page.locator(".pal-in")
    box.wait_for()
    box.fill(text)
    # The same step text exists in other scenarios; choose the open scenario's option.
    target = page.locator(f'.pal-o[href^="{SCENARIO_HREF}?"]').first
    target.wait_for()
    href = target.get_attribute("href")
    query = dict(part.split("=", 1) for part in href.split("?", 1)[1].split("&"))
    expected = (f"attempt-{query['attempt']}-" if "attempt" in query else "") + "step-" + query["step"]
    assert expected == failed_id, f"palette option {href} does not point at the failed step {failed_id}"
    for _ in range(page.locator(".pal-o").count()):
        if target.get_attribute("aria-selected") == "true":
            break
        page.keyboard.press("ArrowDown")
    assert target.get_attribute("aria-selected") == "true", "could not select the open scenario's step"
    page.keyboard.press("Enter")
    until(page, "() => !document.querySelector('dialog[open]')", "palette did not close")
    until(page, f"() => document.activeElement.id === {json.dumps(expected)}", f"palette did not focus {expected}")
    route = page.evaluate("location.hash")
    assert route == href, f"palette navigated to {route}, selected option was {href}"


def attempt_times_distinguish_attempts(page):
    # Runner timestamps carry microseconds; every engine must parse them (WebKit rejects >3 digits unaided).
    page.goto(messages + "#/scenarios/checkout-55a03ad5f369-L31")
    page.locator(".v-sc .page-t").wait_for()
    options = page.locator(".attempts [data-attempt-index]")
    labels = [options.nth(i).inner_text().strip() for i in range(options.count())]
    assert labels == ["Attempt 1 · failed · 03:10:51.148 UTC", "Attempt 2 · failed · 03:10:51.154 UTC",
                      "Attempt 3 · failed · 03:10:51.158 UTC"], labels
    options.nth(0).click()
    until(page, "() => document.querySelector('.attempt-body p')?.textContent.startsWith('Attempt 1:')", "Attempt 1 details did not show")
    body = page.locator(".attempt-body p").first.inner_text().strip()
    assert body == "Attempt 1: Failed · Started 03:10:51.148364 UTC · Finished 03:10:51.154157 UTC · 5.8 ms elapsed", body


EMPTY_ROUTES = {
    "#/dashboard": ("No scenarios executed", "No scenarios executed"),
    "#/scenarios": ("Scenarios", "0 scenarios in 0 features"),
    "#/failures": ("Failures", "No unexpected failures"),
}


def empty_report_is_informational(page):
    for route, (heading, message) in EMPTY_ROUTES.items():
        page.goto(
            "about:blank"
        )  # a fresh load per route, so no earlier view can satisfy the waits
        page.goto(empty + route)
        until(
            page,
            f"() => document.querySelector('.page-t')?.textContent.trim() === {json.dumps(heading)}",
            f"{route} heading did not render",
        )
        settle(page)
        text = page.locator("main").inner_text()
        assert message in text, f"{route} empty state missing: {message!r}"
        assert not page.evaluate("!!window.__crBanner"), (
            f'"could not be read" appeared on {route}'
        )
        assert "could not be shown" not in text, route
        assert not page.evaluate(
            "document.documentElement.scrollWidth > innerWidth + 1"
        ), f"horizontal overflow on {route}"


CHECKS = [
    settings_keep_scenario_state,
    back_forward_focus,
    mobile_drawer_closes_on_back,
    evidence_reveal_after_collapsing_attempts,
    palette_focuses_picked_step,
    attempt_times_distinguish_attempts,
    empty_report_is_informational,
]

failed = []
with sync_playwright() as playwright:
    for engine in engines:
        browser = getattr(playwright, engine).launch()
        context = browser.new_context(viewport={"width": 1280, "height": 900})
        context.add_init_script(PROBE)
        for check in CHECKS:
            page = context.new_page()
            errors = []
            page.on("pageerror", lambda e: errors.append(str(e)))
            page.on(
                "console",
                lambda m: errors.append(m.text) if m.type == "error" else None,
            )
            try:
                check(page)
                assert not errors, errors
                print(f"{engine}: PASS {check.__name__}")
            except Exception as e:  # report every check, then fail
                failed.append(f"{engine}: {check.__name__}")
                print(
                    f"{engine}: FAIL {check.__name__}: {str(e).splitlines()[0] if str(e) else type(e).__name__}"
                )
                if shots:
                    try:
                        shots.mkdir(parents=True, exist_ok=True)
                        page.screenshot(
                            path=str(shots / f"{engine}-{check.__name__}.png"),
                            full_page=True,
                        )
                    except Exception as shot_error:
                        print(f"{engine}: screenshot failed: {shot_error}")
            page.close()
        browser.close()

if failed:
    sys.exit("interaction checks failed: " + ", ".join(failed))
print(f"{', '.join(engines)}: {len(CHECKS)} interaction checks passed")
