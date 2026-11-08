"""Open every enhanced scenario after folding retries; run in the browser matrix."""
from pathlib import Path
import sys
from playwright.sync_api import sync_playwright

report = Path(sys.argv[1]).resolve()
with sync_playwright() as playwright:
    browser = getattr(playwright, sys.argv[2]).launch()
    page = browser.new_page(reduced_motion='reduce')
    errors = []
    page.on('pageerror', lambda e: errors.append(str(e)))
    page.goto(report.as_uri())
    ids = page.evaluate("JSON.parse(document.getElementById('report-data').textContent).features.flatMap(f=>f.elements.map(s=>s.id))")
    for scenario_id in ids:
        page.goto(report.as_uri() + '#/scenarios/' + scenario_id)
        page.locator('.v-sc .page-t').wait_for()
        page.get_by_role('button', name='Collapse all', exact=True).click()
        assert page.locator('.v-sc details[open]').count() == 0
        assert page.get_by_role('searchbox', name='Find in attachment').count() == 0
        assert page.get_by_role('searchbox', name='Find in error').count() == 0
        page.get_by_role('button', name='Expand all', exact=True).click()
        assert page.locator('.v-sc details:not([open])').count() == 0
        assert page.locator('.v-sc .att-h[aria-expanded=false]').count() == 0
    assert not errors, errors
    browser.close()
    print(f'{sys.argv[2]}: {len(ids)} enhanced scenario routes and attempt controls passed')
