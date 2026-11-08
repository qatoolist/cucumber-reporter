"""Run B2-M1 UI mutants against the same independent DOM assertions.

Only temporary generated reports change. Production assets and the working tree
are never modified. Each mutant must fail an assertion (not a parser/load error).
"""
import json
import os
import re
import subprocess


MUTATIONS = {
    'attempt-timing-hidden': [('if (started || finished) {', 'if (false) {')],
    'attempt-step-time-hidden': [('return timed.length ? [', 'return false ? [')],
    'attempt-label-without-clock': [("+ (clockOf(a.started, 3) ? ' · ' + clockOf(a.started, 3) : '')", '')],
    'attempt-elapsed-milliseconds-only': [(' + subMsNs(a.finished) - subMsNs(a.started)', '')],
    'attempt-step-time-from-final-attempt': [('attemptSteps(scenario, a).filter', '(scenario.steps || []).filter')],
    'virtual-list-return-anchor': [("CTX.anchor = row ? {id:row.dataset.id, top:row.getBoundingClientRect().top} : null;", 'CTX.anchor = null;')],
    'modal-tab-containment': [("if (e.key !== 'Tab') return;", 'if (true) return;')],
    'B2-M1-class-only-collapse': [
        ('hidden: !open, inert: !open', 'class: "acc" + (open ? " open" : "")'),
        ('wrap.hidden = !o; wrap.inert = !o;', 'wrap.classList.toggle("open", o);'),
        ('wrap.hidden=!expanded;wrap.inert=!expanded;', 'wrap.classList.toggle("open", expanded);'),
    ],
    'B2-M1-bulk-without-errors': [
        ("for (const error of node.querySelectorAll('.error-lines')) error.setExpanded(open);", '/* mutant: omit error expansion */'),
    ],
    'B2-M1-bulk-without-attachments': [
        ('''for (const b of node.querySelectorAll('.att-h[aria-expanded="' + !open + '"]')) b.click();''', '/* mutant: omit attachment expansion */'),
    ],
    'B2-M1-expand-never-disabled': [
        ('''expand.disabled = !states.includes(false) && !node.querySelector('.error-lines.clip');''', 'expand.disabled = false;'),
    ],
    'D2-M2-focus-rebuild': [
        ('if(samePath)restoreFocus();', 'restoreFocus();'),
    ],
    'D2-L1-leaked-listeners': [
        ('disposeList(box);', '/* mutant: retain prior listeners */'),
    ],
}

MUTATIONS.update({
 'H3-ancestor-details': [("for (let parent = row.parentElement; parent; parent = parent.parentElement) if (parent.tagName === 'DETAILS') parent.open = true;", '')],
 'H3-attempt-selection': [("if (option && option.getAttribute('aria-checked') !== 'true') option.click();", '')],
 'H3-hidden-row': [("if(row.hidden) doc.querySelector('.step-controls [role=radio]')?.click();\n    row.hidden = false;", '')],
 'H3-eager-inert': [("wrap.hidden = !o; wrap.inert = !o;", "wrap.hidden = !o;")],
 'H3-eager-hidden': [("wrap.hidden = !o; wrap.inert = !o;", "wrap.inert = !o;")],
 'H3-drawer-aria': [("side.setAttribute('aria-hidden', String(closed));", '')],
 'H3-i-bulk-ignores-attempt-details': [("const closed=[...node.querySelectorAll('details')]", "const closed=[...node.querySelectorAll('.steps details')]")],
 'H4-3-skip-branch-on-list-views': [("samePath && name === 'scenarios' && parts.length > 1) {", "samePath) {")],
 'H4-4-skip-branch-keeps-drawer-open': [("      toggleSide(false);\n      if(q.has('step')||q.has('attempt'))", "      if(q.has('step')||q.has('attempt'))")],
 'H4-6-skip-branch-no-heading-focus': [("else main.querySelector('.page-t')?.focus();\n      return;", "\n      return;")],
 'H4-N3-settings-reveal-reruns': [("if (name === 'scenarios' && (event || !samePath)) focusStepTarget(q);", "if (name === 'scenarios') focusStepTarget(q);")],
 'H4-N3-settings-rebuild-scenario': [("if (!onScenario()) render(); };", "render(); };")],
 'H4-N4-skip-branch-ignores-attempt': [("if(q.has('step')||q.has('attempt')) focusStepTarget(q); else main", "if(q.has('step')) focusStepTarget(q); else main")],
 'H4-N3-open-attachments-not-applied': [("if (b && (b.getAttribute('aria-expanded') === 'true') !== want) b.click();", '')],
 'H4-N3-reset-not-applied': [("if (applyOpenAttachments) applyOpenAttachments(); else render(); buildSettings();", "if (!applyOpenAttachments) render(); buildSettings();")],
 'H4-N4-no-step-fallback': [(" || doc.getElementById('step-' + step);", ";")],
})


# Mutants that must be killed by a named test, not by any assertion anywhere.
MEASURED = 'attempts show measured start, finish and elapsed time'
FALLBACK = 'attempts without timestamps show their own step time'
KILLED_BY = {
    'attempt-timing-hidden': MEASURED, 'attempt-label-without-clock': MEASURED,
    'attempt-elapsed-milliseconds-only': MEASURED, 'attempt-step-time-hidden': FALLBACK,
    'attempt-step-time-from-final-attempt': FALLBACK,
}


def run(root, tmp, reports):
    source = (root / 'report/assets/app.js').read_text()
    script = re.compile(r'<script>.*?</script>', re.DOTALL)

    def check(name, changed, expect_pass):
        folder = tmp / name
        folder.mkdir()
        paths = {}
        for fixture, original in reports.items():
            html, count = script.subn(lambda _: '<script>' + changed + '</script>', open(original).read())
            if count != 1:
                raise RuntimeError(f'{fixture}: expected one application script, found {count}')
            output = folder / (fixture + '.html')
            output.write_text(html)
            paths[fixture] = str(output)
        result = subprocess.run(
            ['node', '--test', 'internal/browsercheck/dom/ui.test.cjs'], cwd=root,
            env=dict(os.environ, CR_DOM_REPORTS=json.dumps(paths)), capture_output=True, text=True, timeout=600,
        )
        (folder / 'test.log').write_text(result.stdout + result.stderr)
        if expect_pass:
            if result.returncode:
                raise RuntimeError('Unminified control failed:\n' + result.stdout + result.stderr)
        elif result.returncode == 0 or 'AssertionError' not in result.stdout or 'SyntaxError' in result.stdout:
            raise RuntimeError(name + ' survived or failed for an unrelated reason:\n' + result.stdout + result.stderr)
        elif name in KILLED_BY and not re.search(r'^\s*✖ ' + re.escape(KILLED_BY[name]), result.stdout, re.M):
            raise RuntimeError(name + ' was not killed by its own test (' + KILLED_BY[name] + '):\n' + result.stdout)
        print(name + (': PASS' if expect_pass else ': KILLED by DOM assertions'), flush=True)

    check('unminified-control', source, True)
    for name, changes in MUTATIONS.items():
        changed = source
        for before, after in changes:
            expected = 2 if name == 'D2-L1-leaked-listeners' else 1
            count = changed.count(before)
            if count != expected:
                raise RuntimeError(f'{name}: mutation drift ({count}, expected {expected}): {before}')
            changed = changed.replace(before, after)
        check(name, changed, False)
    print(f'DOM mutation gate: {len(MUTATIONS)}/{len(MUTATIONS)} killed; zero skips', flush=True)
