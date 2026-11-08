"""Cross-browser offline report checks. Usage: python check.py report/index.html [browser]."""
import pathlib
import re
import sys
from playwright.sync_api import sync_playwright

# The label names its target: "View evidence (1) · <attachment>".
EVIDENCE_LINK = re.compile(r"^View evidence \(1\)( · .+)?$")

report = pathlib.Path(sys.argv[1]).resolve()
engines = sys.argv[2:] or ["chromium", "firefox", "webkit"]

with sync_playwright() as playwright:
    for engine in engines:
        browser = getattr(playwright, engine).launch()
        context = browser.new_context(viewport={"width": 1440, "height": 900}, reduced_motion="reduce")
        context.add_init_script("window.cspViolations = []; document.addEventListener('securitypolicyviolation', e => window.cspViolations.push(e.violatedDirective));")
        page = context.new_page()
        errors, requests = [], []
        page.on("pageerror", lambda e: errors.append(str(e)))
        page.on("console", lambda e: errors.append(e.text) if e.type == "error" else None)
        page.on("request", lambda r: requests.append(r.url) if r.url.startswith(("http:", "https:")) else None)
        page.goto(report.as_uri())
        page.locator(".page-t").wait_for()
        for route in ["dashboard", "features", "scenarios", "failures", "known", "skipped", "timeline", "run", "print"]:
            page.goto(report.as_uri() + "#/" + route)
            page.locator(".page-t").wait_for()
            assert page.get_by_text("This view could not be shown", exact=True).count() == 0, route
            assert page.locator("#main").count() == 1
            # Every visible button has an accessible text label or aria-label.
            assert page.locator("button").evaluate_all("xs => xs.filter(x => x.getClientRects().length && !(x.textContent.trim() || x.getAttribute('aria-label'))).length") == 0
        page.goto(report.as_uri() + "#/scenarios?status=failed&layout=table")
        page.locator(".scenario-table").wait_for()
        assert page.locator(".scenario-table tbody tr").count() == 3
        page.locator(".scenario-table tbody a").first.click()
        page.locator(".failure-summary").wait_for()
        # Exercise effects, not merely hashes; repeating a fragment does not emit hashchange.
        from urllib.parse import parse_qs
        for link in [page.get_by_role("link", name="Jump to step", exact=True),
                     page.get_by_role("link", name=EVIDENCE_LINK),
                     page.locator(".clean-banner a[href*='step=']").first,
                     page.locator(".sc-aside a[href*='evidence=']").first]:
            query = parse_qs(link.get_attribute("href").split("?", 1)[1])
            target = "#step-" + query["step"][0]
            if "evidence" in query:
                target += " .att-h"
            for _ in range(2):
                page.evaluate("window.scrollTo(0,0)")
                link.click()
                page.wait_for_function("selector => document.activeElement.matches(selector)", arg=target)
                assert page.locator(target).first.is_visible()
                assert page.locator(target).first.evaluate("x => { const r=x.getBoundingClientRect(); return r.top>=0 && r.top<innerHeight; }")
                if "evidence" in query:
                    assert page.locator(target).first.get_attribute("aria-expanded") == "true"
        page.get_by_role("button", name="Collapse all", exact=True).click()
        assert page.locator(".v-sc details[open]").count() == 0
        assert page.locator(".v-sc .att-h[aria-expanded=true]").count() == 0
        assert page.get_by_role("searchbox", name="Find in attachment").count() == 0
        assert page.get_by_role("searchbox", name="Find in error").count() == 0
        page.get_by_role("link", name=EVIDENCE_LINK).click()
        header = page.locator("#step-3 .att-h").first
        header.click()
        header.focus()
        page.keyboard.press("Tab")
        assert page.evaluate("!document.activeElement.closest('.acc[hidden]')")
        page.get_by_role("button", name="Expand all", exact=True).click()
        assert page.locator(".v-sc details:not([open])").count() == 0
        assert page.locator(".v-sc .att-h[aria-expanded=false]").count() == 0
        assert page.get_by_role("button", name="Expand all", exact=True).is_disabled()
        page.get_by_role("button", name="Full screen", exact=True).first.click()
        page.get_by_role("dialog", name="Error details").wait_for()
        page.get_by_role("searchbox", name="Find in error").last.fill("invoice")
        assert page.locator("dialog .matched-line").count() > 0
        # Escape in a filled search field first clears it (native browser behaviour); the next Escape closes.
        page.keyboard.press("Escape")
        if page.locator("dialog[open]").count():
            assert page.get_by_role("searchbox", name="Find in error").last.input_value() == ""
            page.keyboard.press("Escape")
        assert page.locator("dialog[open]").count() == 0
        # Dialog closure restores focus to the invoking button.
        assert page.evaluate("document.activeElement.textContent.includes('Full screen')")
        page.goto(report.as_uri() + "#/scenarios?tags=%40smoke%20and%20not%20%40slow")
        page.locator(".rows").wait_for()
        assert page.locator(".active-f").inner_text().startswith("4 of 40")
        page.get_by_role("button", name="Print or save as PDF").click()
        page.get_by_text("Compress a saved PDF for sharing", exact=True).click()
        page.get_by_label("Saved PDF compression quality", exact=True).select_option("small")
        assert page.locator("dialog code").inner_text() == "cucumber-report compress-pdf -quality small -o report-shared.pdf report.pdf"
        page.get_by_label("Print scope", exact=True).select_option("filtered")
        page.get_by_role("button", name="Preview", exact=True).click()
        page.locator(".pr-scope").wait_for()
        assert page.locator(".pr-sc:visible").count() == 4
        # Both printing and cancelling move the print root back into the preview.
        for _ in range(2):
            page.evaluate("window.dispatchEvent(new Event('beforeprint'))")
            page.evaluate("window.dispatchEvent(new Event('afterprint'))")
            assert page.locator("#main .pr-sc:visible").count() == 4
            assert page.locator(".pr-scope").is_visible()
        page.goto(report.as_uri() + "#/failures?layout=split")
        page.locator(".failure-split .failure-summary").wait_for()
        known_id = page.evaluate("JSON.parse(document.getElementById('report-data').textContent).annotations.find(a => a.ref === 'EX-101').scenario")
        page.goto(report.as_uri() + "#/scenarios/" + known_id)
        page.locator(".clean-banner").wait_for()
        assert page.locator(".clean-banner .ref").inner_text() == "EX-101"
        for width in [390, 768, 1440]:
            page.set_viewport_size({"width": width, "height": 900})
            assert page.evaluate("document.documentElement.scrollWidth <= innerWidth + 2"), (engine, width)
            page.keyboard.press("Tab")
            assert page.evaluate("(() => { const r=document.activeElement.getBoundingClientRect(); return r.bottom > 0 && r.top < innerHeight; })()")
        assert not requests, requests
        assert not errors, errors
        assert page.evaluate("window.cspViolations") == []
        context.close()
        offline = browser.new_context(java_script_enabled=False)
        fallback = offline.new_page()
        fallback.goto(report.as_uri())
        assert fallback.locator(".nj h1").is_visible()
        assert fallback.locator(".nj details").count() == 40
        offline.close()
        browser.close()
        print(f"{engine}: routes, filters, table, errors, focus, print selection, banners, reflow and no-JS passed")
