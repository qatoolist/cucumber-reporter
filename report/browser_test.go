//go:build browser

package report

// Headless browser checks of the rendered page (make browser-check). They drive a local Chrome,
// Chromium or Edge (found through $CHROME or the usual install locations) over the DevTools protocol
// with internal/cdp, against the unmodified page under its own Content-Security-Policy, and fail on
// any exception, console error or CSP violation. The PDF checks also need Ghostscript ($GS or gs on
// PATH). All data is synthetic.
//
// The interaction checks are a port of the approved prototype's 35 checks. Waits poll a page condition
// with a bounded deadline; no check asserts on elapsed time.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	pub "github.com/qatoolist/cucumber-reporter/cucumber"

	"github.com/qatoolist/cucumber-reporter/internal/cdp"
	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

type session struct {
	t     *testing.T
	ctx   context.Context
	b     *cdp.Browser
	file  string
	loads int
}

// openSession renders features with opt into a temporary file and opens it in a fresh browser at a
// 1440 × 900 viewport.
func openSession(t *testing.T, features []cucumber.Feature, opt renderOptions, reducedMotion bool) *session {
	t.Helper()
	return openPage(t, mustRender(t, features, opt), reducedMotion)
}

// openPage opens an already rendered page, as openSession does.
func openPage(t *testing.T, page string, reducedMotion bool) *session {
	t.Helper()
	chrome, err := cdp.FindChrome()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "index.html")
	if err = os.WriteFile(file, []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	t.Cleanup(cancel)
	b, err := cdp.Launch(ctx, chrome, filepath.Join(dir, "profile"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	s := &session{t: t, ctx: ctx, b: b, file: file}
	if reducedMotion {
		s.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{{"name": "prefers-reduced-motion", "value": "reduce"}}})
	}
	s.size(1440, 900)
	s.goTo("/")
	s.must(`localStorage.clear(), true`)
	return s
}

func (s *session) call(method string, params any) {
	s.t.Helper()
	if err := s.b.Call(s.ctx, method, params, nil); err != nil {
		s.t.Fatalf("%s: %v", method, err)
	}
}

// eval evaluates expr and decodes its value into out; an exception fails the test.
func (s *session) eval(expr string, out any) {
	s.t.Helper()
	if err := s.b.Eval(s.ctx, expr, out); err != nil {
		s.t.Fatalf("eval %.80q: %v", expr, err)
	}
}

func (s *session) str(expr string) string {
	s.t.Helper()
	var v string
	s.eval(expr, &v)
	return v
}

func (s *session) num(expr string) int {
	s.t.Helper()
	var v float64
	s.eval(expr, &v)
	return int(v)
}

func (s *session) is(expr string) bool {
	s.t.Helper()
	var v bool
	s.eval("!!("+expr+")", &v)
	return v
}

// must evaluates a boolean expression and fails when it is false.
func (s *session) must(expr string) {
	s.t.Helper()
	if !s.is(expr) {
		s.t.Fatalf("not true: %s", expr)
	}
}

// waitFor polls expr until it is true, for up to five seconds, and reports whether it became true.
func (s *session) waitFor(expr string) bool {
	s.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if s.is(expr) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// settle waits for two animation frames and a task, so that pending hashchange renders and frame
// callbacks finish.
func (s *session) settle() {
	s.t.Helper()
	s.eval(`new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(() => r(true), 0))))`, nil)
}

func (s *session) size(w, h int) {
	s.t.Helper()
	if err := s.b.SetViewport(s.ctx, w, h); err != nil {
		s.t.Fatal(err)
	}
}

// goTo loads the page afresh at route hash (a full load, so every check starts from a clean render).
func (s *session) goTo(hash string) {
	s.t.Helper()
	s.loads++
	if err := s.b.Navigate(s.ctx, "file://"+s.file+"?load="+strconv.Itoa(s.loads)+"#"+hash); err != nil {
		s.t.Fatal(err)
	}
	if !s.waitFor(`document.querySelector('.page-t')`) {
		s.t.Fatalf("route %s did not render", hash)
	}
	s.settle()
}

func (s *session) reload() {
	s.t.Helper()
	if err := s.b.Reload(s.ctx); err != nil {
		s.t.Fatal(err)
	}
	if !s.waitFor(`document.querySelector('.page-t')`) {
		s.t.Fatal("reload did not render")
	}
	s.settle()
}

func (s *session) key(k string, mods int) {
	s.t.Helper()
	if err := s.b.Key(s.ctx, k, mods); err != nil {
		s.t.Fatal(err)
	}
	s.settle()
}

func (s *session) hash() string { s.t.Helper(); return s.str(`decodeURIComponent(location.hash)`) }

// check runs one named check as a subtest and fails it on any new browser problem.
func (s *session) check(name string, fn func() error) {
	s.t.Run(name, func(t *testing.T) {
		outer := s.t
		s.t = t
		defer func() { s.t = outer }()
		before := len(s.b.Problems())
		if err := fn(); err != nil {
			t.Error(err)
		}
		if p := s.b.Problems(); len(p) > before {
			t.Errorf("browser problems: %v", p[before:])
		}
	})
}

func fail(format string, args ...any) error { return fmt.Errorf(format, args...) }

func stressRun() []cucumber.Feature { return syntheticRun(40, 25, 5) } // 1,000 scenarios, 5,000 steps

func TestBrowserInteractions(t *testing.T) {
	showcase, showcaseOpt := loadShowcase(t, "")
	known, knownOpt := loadShowcase(t, "-known")
	pages := []struct {
		name     string
		features []cucumber.Feature
		opt      renderOptions
	}{
		{"showcase", showcase, showcaseOpt},
		{"known", known, knownOpt},
		{"stress", stressRun(), renderOptions{Version: OptionsVersion, Title: "Stress", RunFacts: []Fact{{Label: "Environment", Value: "synthetic"}}}},
	}
	for _, p := range pages {
		t.Run(p.name, func(t *testing.T) { interactionChecks(t, openSession(t, p.features, p.opt, true)) })
	}
}

// interactionChecks is the port of the prototype's tools/verify.mjs, check for check.
func interactionChecks(t *testing.T, s *session) {
	t.Helper()
	firstScenario := s.str(`(() => { const a = document.querySelector('.attn a, .slow a'); return a ? a.getAttribute('href') : ''; })()`)

	s.check("loads with app shell", func() error {
		if !s.is(`document.querySelector('.top .verdict-pill') && document.querySelector('.hero, .kpis')`) {
			return fail("no app shell")
		}
		return nil
	})
	s.check("nav link routes to Features", func() error {
		s.eval(`document.querySelector('.nav-i[data-k=features]').click()`, nil)
		if !s.waitFor(`location.hash === '#/features' && document.querySelector('.page-t').textContent === 'Features'`) {
			return fail("hash %s", s.hash())
		}
		return nil
	})
	s.check("nav marks current page (aria-current)", func() error {
		if !s.is(`document.querySelector('.nav-i[aria-current=page]').dataset.k === 'features'`) {
			return fail("aria-current not on Features")
		}
		return nil
	})
	s.check("feature card drills into feature", func() error {
		s.eval(`document.querySelector('.fcard').click()`, nil)
		if !s.waitFor(`location.hash.startsWith('#/features/') && document.querySelector('.crumbs li:nth-child(2) a').textContent === 'Features'`) {
			return fail("hash %s", s.hash())
		}
		return nil
	})
	s.check("breadcrumb returns to Features", func() error {
		s.eval(`document.querySelector('.crumbs li:nth-child(2) a').click()`, nil)
		if !s.waitFor(`location.hash === '#/features'`) {
			return fail("hash %s", s.hash())
		}
		return nil
	})
	s.check("history back and forward", func() error {
		s.eval(`history.back()`, nil)
		if !s.waitFor(`location.hash.startsWith('#/features/')`) {
			return fail("back: %s", s.hash())
		}
		s.eval(`history.forward()`, nil)
		if !s.waitFor(`location.hash === '#/features'`) {
			return fail("forward: %s", s.hash())
		}
		return nil
	})
	s.check("deep link to scenario step highlights it", func() error {
		route := strings.TrimPrefix(firstScenario, "#")
		if !strings.Contains(route, "?") {
			route += "?step=0"
		}
		s.goTo(route)
		if !s.waitFor(`document.querySelector('.step.flash') && document.querySelector('.steps')`) {
			return fail("no highlighted step at %s", route)
		}
		return nil
	})
	s.check("scenario prev/next navigation", func() error {
		before := s.hash()
		s.eval(`(document.querySelector('.pn a:last-child:not(.off)') || document.querySelector('.pn a:first-child:not(.off)')).click()`, nil)
		if !s.waitFor(`location.hash.startsWith('#/scenarios/') && decodeURIComponent(location.hash) !== ` + jsString(before)) {
			return fail("still at %s", before)
		}
		return nil
	})
	s.check("keyboard ] and [ move between scenarios", func() error {
		a := s.hash()
		s.eval(`document.activeElement.blur()`, nil)
		s.key("]", 0)
		b := s.hash()
		s.key("[", 0)
		c := s.hash()
		if (a == b && a == c) || !strings.HasPrefix(c, "#/scenarios/") {
			return fail("%s → %s → %s", a, b, c)
		}
		return nil
	})
	s.check("Cmd/Ctrl+K opens search; typing filters; Enter opens result", func() error {
		s.goTo("/")
		s.key("k", 4)
		if !s.is(`document.querySelector('.palette').open`) {
			return fail("palette not open")
		}
		word := s.str(`(() => { const e = document.querySelector('.att-t, .slow-t bdi, .fb-n bdi'); return e.textContent.split(/\s+/).find((w) => w.length > 4) || e.textContent.slice(0, 5); })()`)
		for _, r := range strings.ToLower(word) {
			if err := s.b.InsertText(s.ctx, string(r)); err != nil {
				return err
			}
		}
		s.settle()
		n, marks := s.num(`document.querySelectorAll('.pal-o').length`), s.num(`document.querySelectorAll('.pal-o mark').length`)
		s.key("ArrowDown", 0)
		sel := s.str(`decodeURIComponent(document.querySelector('.pal-o[aria-selected=true]').getAttribute('href'))`)
		s.key("Enter", 0)
		if !s.waitFor(`decodeURIComponent(location.hash) === `+jsString(sel)+` && !document.querySelector('.palette').open`) || n == 0 || marks == 0 {
			return fail("word %q: %d options, %d marks, selected %s, hash %s", word, n, marks, sel, s.hash())
		}
		return nil
	})
	s.check("slash opens search, Escape closes", func() error {
		s.eval(`document.activeElement.blur()`, nil)
		s.key("/", 0)
		opened := s.is(`document.querySelector('.palette').open`)
		s.key("Escape", 0)
		if !opened || s.is(`document.querySelector('.palette').open`) {
			return fail("opened %v, still open after Escape", opened)
		}
		return nil
	})
	s.check("? opens shortcuts; g f goes to failures", func() error {
		s.eval(`document.activeElement.blur()`, nil)
		s.key("?", 0)
		opened := s.is(`document.querySelector('dialog.small').open`)
		s.key("Escape", 0)
		s.key("g", 0)
		s.key("f", 0)
		if !opened || !s.waitFor(`location.hash === '#/failures'`) {
			return fail("shortcuts dialog %v, hash %s", opened, s.hash())
		}
		return nil
	})
	s.check("status chip filters list and is saved in the URL", func() error {
		s.goTo("/scenarios")
		count := `parseInt(document.querySelector('.active-f .count').textContent.replace(/,/g, ''))`
		all := s.num(count)
		k := s.str(`(() => { const c = [...document.querySelectorAll('.fchip[class*="s-"]')].find((b) => !b.classList.contains('s-passed')) || document.querySelector('.fchip.s-passed'); c.click(); return c.className.match(/s-(\w+)/)[1]; })()`)
		s.settle()
		n, h := s.num(count), s.hash()
		s.reload()
		pressed := s.str(`[...document.querySelectorAll('.fchip[aria-pressed=true]')].map((b) => b.className).join(' ')`)
		if !strings.Contains(h, "status="+k) || n > all || !strings.Contains(pressed, "s-"+k) {
			return fail("all %d, n %d, hash %s, pressed %q, status %s", all, n, h, pressed, k)
		}
		return nil
	})
	s.check("text filter highlights matches and count updates", func() error {
		s.goTo("/scenarios")
		s.eval(`(() => { const i = document.querySelector('.tb-search input'); i.value = document.querySelector('.row-t bdi').textContent.split(/\s+/).find((x) => x.length > 3); i.dispatchEvent(new Event('input')); })()`, nil)
		if !s.waitFor(`document.querySelectorAll('.rows mark').length > 0 && /q=/.test(location.hash) && /scenario/.test(document.querySelector('.active-f .count').textContent)`) {
			return fail("no highlighted matches")
		}
		return nil
	})
	s.check("clear filters restores the list", func() error {
		s.eval(`document.querySelector('.active-f .linkbtn').click()`, nil)
		if !s.waitFor(`!/q=|status=/.test(location.hash)`) {
			return fail("hash %s", s.hash())
		}
		return nil
	})
	s.check("donut segment hover shows tooltip; click drills down", func() error {
		s.goTo("/")
		tip := s.is(`(() => { const e = document.querySelector('.donut .seg'); const r = e.getBoundingClientRect(); e.dispatchEvent(new PointerEvent('pointermove', { clientX: r.left + 5, clientY: r.top + 5, bubbles: true })); return !document.getElementById('tip').hidden && document.getElementById('tip').textContent.length > 0; })()`)
		s.eval(`document.querySelector('.donut .seg').dispatchEvent(new MouseEvent('click', { bubbles: true }))`, nil)
		if !tip || !s.waitFor(`location.hash.startsWith('#/scenarios?status=')`) {
			return fail("tooltip %v, hash %s", tip, s.hash())
		}
		return nil
	})
	s.check("donut segment reachable and activatable by keyboard", func() error {
		s.goTo("/")
		s.eval(`document.querySelector('.donut .seg').focus()`, nil)
		tip := s.is(`!document.getElementById('tip').hidden`)
		s.key("Enter", 0)
		if !tip || !s.waitFor(`location.hash.startsWith('#/scenarios?status=')`) {
			return fail("tooltip %v, hash %s", tip, s.hash())
		}
		return nil
	})
	s.check("legend toggle hides a segment (aria-pressed)", func() error {
		s.goTo("/")
		a := s.num(`document.querySelectorAll('.donut .seg').length`)
		s.eval(`document.querySelector('.legend .leg').click()`, nil)
		s.settle()
		b := s.num(`document.querySelectorAll('.donut .seg').length`)
		pressed := s.str(`document.querySelector('.legend .leg').getAttribute('aria-pressed')`)
		if (a > 1 && b != a-1) || b > a || pressed != "false" {
			return fail("segments %d → %d, aria-pressed %s", a, b, pressed)
		}
		return nil
	})
	s.check("Scenarios/Steps switch redraws the donut", func() error {
		s.goTo("/")
		s.eval(`document.querySelector('.c-status .seg button:last-child').click()`, nil)
		s.settle()
		if !s.is(`/step/.test(document.querySelector('.donut-n span').textContent) && document.querySelector('.c-status .seg button:last-child').getAttribute('aria-checked') === 'true'`) {
			return fail("donut not switched to steps")
		}
		return nil
	})
	s.check("feature bar segment links to filtered feature", func() error {
		h := s.str(`document.querySelector('.fbars .sbar a').getAttribute('href')`)
		if !regexp.MustCompile(`^#/features/[^?]+\?status=\w+$`).MatchString(h) {
			return fail("href %s", h)
		}
		return nil
	})
	s.check("KPI tile is a link that filters", func() error {
		href := s.str(`document.querySelectorAll('.kpi')[3].getAttribute('href')`)
		s.eval(`document.querySelectorAll('.kpi')[3].click()`, nil)
		if !regexp.MustCompile(`^#/scenarios\?status=passed(,known)?$`).MatchString(href) || !s.waitFor(`location.hash === `+jsString(href)) {
			return fail("href %s, hash %s", href, s.hash())
		}
		return nil
	})
	s.check("settings: hide a widget, persists across reload, reset restores", func() error {
		s.goTo("/")
		s.eval(`document.querySelector('[aria-label=Settings]').click()`, nil)
		s.eval(`[...document.querySelectorAll('.sw-r')].find((r) => r.textContent.includes('Duration profile')).querySelector('input').click()`, nil)
		s.settle()
		gone := s.is(`!document.querySelector('.c-dur')`)
		s.key("Escape", 0)
		s.reload()
		still := s.is(`!document.querySelector('.c-dur')`)
		s.eval(`document.querySelector('[aria-label=Settings]').click()`, nil)
		s.eval(`[...document.querySelectorAll('.sheet-f button')].pop().click()`, nil)
		s.settle()
		back := s.is(`document.querySelector('.c-dur')`)
		if !gone || !still || !back {
			return fail("hidden %v, after reload %v, restored %v", gone, still, back)
		}
		return nil
	})
	s.check("settings: hide a nav section", func() error {
		toggle := `[...document.querySelectorAll('.sw-r')].find((r) => r.textContent.includes('Run details')).querySelector('input').click()`
		s.eval(toggle, nil)
		s.settle()
		gone := s.is(`!document.querySelector('.nav-i[data-k=run]')`)
		s.eval(toggle, nil)
		s.key("Escape", 0)
		if !gone || !s.is(`document.querySelector('.nav-i[data-k=run]')`) {
			return fail("hidden %v", gone)
		}
		return nil
	})
	s.check("theme button cycles and persists", func() error {
		s.goTo("/")
		btn := `document.querySelector('.top .ibtn[aria-label^=Theme]').click()`
		s.eval(btn, nil)
		t1 := s.str(`document.documentElement.dataset.theme || ''`)
		s.reload()
		t2 := s.str(`document.documentElement.dataset.theme || ''`)
		s.eval(btn, nil)
		s.eval(btn, nil)
		t3 := s.str(`document.documentElement.dataset.theme || ''`)
		if t1 != "light" || t2 != "light" || t3 != "" {
			return fail("themes %q %q %q", t1, t2, t3)
		}
		return nil
	})
	s.check("blocked storage falls back without errors", func() error {
		r := s.str(`(() => { try { Object.defineProperty(window, 'localStorage', { get() { throw new Error('blocked'); } }); } catch (e) {} document.querySelector('[aria-label=Settings]').click(); const sw = document.querySelector('.sw-r input'); sw.click(); sw.click(); return document.querySelector('.sheet-f p').textContent; })()`)
		s.key("Escape", 0)
		if !regexp.MustCompile(`blocks storage|Saved`).MatchString(r) {
			return fail("footer %q", r)
		}
		return nil
	})
	s.check("attachment opens and JSON/text renders; copy shows toast", func() error {
		sid := s.str(`(() => { const D = JSON.parse(document.getElementById('report-data').textContent); for (const f of D.features) for (const e of f.elements) if (e.steps.some((x) => x.att && x.att.some((a) => a.data))) return e.id; return ''; })()`)
		if sid == "" {
			return nil
		}
		s.goTo("/scenarios/" + sid)
		opened := s.is(`(() => { const b = [...document.querySelectorAll('.att-h')].find((x) => !/image\//.test(x.textContent)); const was = b.getAttribute('aria-expanded'); b.click(); if (b.getAttribute('aria-expanded') === 'false') b.click(); return was !== null && b.getAttribute('aria-expanded') === 'true'; })()`)
		body := s.waitFor(`document.querySelector('[aria-expanded=true] + .acc .jt li, [aria-expanded=true] + .acc pre')`)
		s.eval(`document.querySelector('[aria-expanded=true] + .acc .att-tools .btn:not([role])').click()`, nil)
		if !opened || !body || !s.waitFor(`/copied|blocked/i.test(document.querySelector('.toast').textContent)`) {
			return fail("opened %v, body %v, toast %q", opened, body, s.str(`document.querySelector('.toast').textContent`))
		}
		return nil
	})
	s.check("timeline renders lanes and step definitions; header sort works", func() error {
		s.goTo("/timeline")
		lanes := s.is(`document.querySelectorAll('.gantt a').length > 0 && document.querySelectorAll('.prof tbody tr').length > 0`)
		s.eval(`document.querySelector('.prof th:nth-child(2) button').click()`, nil)
		if !lanes || s.str(`document.querySelector('.prof th:nth-child(2)').getAttribute('aria-sort')`) != "descending" {
			return fail("lanes %v", lanes)
		}
		return nil
	})
	s.check("every route renders", func() error {
		for _, r := range []string{"/", "/features", "/scenarios", "/failures", "/known", "/skipped", "/timeline", "/run", "/print", "/nope"} {
			s.goTo(r)
		}
		return nil
	})
	s.check("narrow width: menu opens navigation, Escape closes", func() error {
		s.size(390, 844)
		defer s.size(1440, 900)
		s.goTo("/")
		vis := s.is(`getComputedStyle(document.querySelector('.menu-btn')).display !== 'none'`)
		s.eval(`document.querySelector('.menu-btn').click()`, nil)
		s.settle()
		open := s.is(`document.documentElement.classList.contains('side-open') && document.querySelector('.menu-btn').getAttribute('aria-expanded') === 'true'`)
		s.key("Escape", 0)
		closed := s.is(`!document.documentElement.classList.contains('side-open')`)
		overflow := s.is(`document.documentElement.scrollWidth <= window.innerWidth`)
		if !vis || !open || !closed || !overflow {
			return fail("menu visible %v, opens %v, closes %v, no horizontal scroll %v", vis, open, closed, overflow)
		}
		return nil
	})
	s.check("print to PDF produces a multi-page document", func() error {
		s.goTo("/")
		s.eval(`window.dispatchEvent(new Event('beforeprint'))`, nil)
		pdf, err := s.b.PrintPDF(s.ctx)
		if err != nil {
			return err
		}
		if n := pdfPages(pdf); n < 2 {
			return fail("%d pages", n)
		}
		return nil
	})
	s.check("headline KPI agrees with the verdict", func() error {
		s.goTo("/")
		var r struct {
			Known                   bool
			Label, Val, Hero, HeroL string
			Sub                     string
		}
		s.eval(`(() => { const k = document.querySelectorAll('.kpi')[3];
			return { known: document.querySelectorAll('.nav-i[data-k=known] .nav-c').length > 0, label: k.querySelector('.kpi-l').textContent.trim(), val: k.querySelector('.kpi-v').textContent.trim(),
				hero: document.querySelector('.hero-rv').textContent.trim(), heroL: document.querySelector('.hero-rl').textContent, sub: k.querySelector('.kpi-s').textContent }; })()`, &r)
		if r.Known {
			if r.Label != "As expected" || r.Val != r.Hero || r.HeroL != "as expected" || !regexp.MustCompile(`passed · \d+ known · pass rate`).MatchString(r.Sub) {
				return fail("%+v", r)
			}
		} else if r.Label != "Pass rate" || r.Val != r.Hero {
			return fail("%+v", r)
		}
		return nil
	})
	s.check("known-failure marker sits only on the declared step", func() error {
		var scen []struct {
			ID, Status string
			Known      []int
		}
		s.eval(`(() => { const D = JSON.parse(document.getElementById('report-data').textContent); const ids = new Set((D.annotations || []).filter((a) => a.kind === 'known').map((a) => a.scenario)); const out = [];
			for (const f of D.features) for (const e of f.elements) if (ids.has(e.id)) out.push({ id: e.id, status: e.status, known: e.steps.map((x, i) => (x.known ? i : -1)).filter((i) => i >= 0) });
			return out; })()`, &scen)
		for _, sc := range scen {
			if (sc.Status == "known") != (len(sc.Known) == 1) {
				return fail("%s: status %s with %d marked steps in the data", sc.ID, sc.Status, len(sc.Known))
			}
			s.goTo("/scenarios/" + sc.ID)
			var marked []int
			s.eval(`[...document.querySelectorAll('.steps > .step')].map((li, i) => (li.classList.contains('s-known') ? i : -1)).filter((i) => i >= 0)`, &marked)
			if fmt.Sprint(marked) != fmt.Sprint(sc.Known) && (len(marked) != 0 || len(sc.Known) != 0) {
				return fail("%s: marked %v, want %v", sc.ID, marked, sc.Known)
			}
			if sc.Status == "failed" && !s.is(`document.querySelector('.banner.b-fail .banner-w')`) {
				return fail("%s: an uncovered failure shows no warning", sc.ID)
			}
		}
		return nil
	})
	s.check("print uses the declared known-issue outcome and the same headline", func() error {
		s.goTo("/print")
		var r struct {
			Bad []string
			KP  string
		}
		s.eval(`(() => { const D = JSON.parse(document.getElementById('report-data').textContent); const pr = document.getElementById('print-root').textContent; const bad = [];
			for (const a of (D.annotations || []).filter((a) => a.kind === 'known')) if (a.step && a.observed && !pr.includes((a.ref ? a.ref + ': ' : '') + a.title + ' (step ' + a.step + ')')) bad.push(a.scenario);
			if (/not applicable: /.test(pr)) bad.push('not applicable outcome printed');
			return { bad, kp: document.querySelector('.pr-kpis').textContent }; })()`, &r)
		known := s.is(`document.querySelector('.nav-i[data-k=known] .nav-c')`)
		ok := len(r.Bad) == 0 && ((known && strings.Contains(r.KP, "As expected") && !strings.Contains(r.KP, "Pass rate")) || (!known && strings.Contains(r.KP, "Pass rate")))
		if !ok {
			return fail("%+v (known failures %v)", r, known)
		}
		return nil
	})
	s.check("dashboard rows end together without large empty areas", func() error {
		defer s.size(1440, 900)
		for _, w := range []int{1440, 1366, 1280} {
			s.size(w, 900)
			s.goTo("/dashboard")
			var d []string
			s.eval(`(() => { const bad = [];
				for (const p of document.querySelectorAll('.v-dash .pair')) {
					const b = [...p.children].map((c) => c.getBoundingClientRect().bottom);
					if (Math.max(...b) - Math.min(...b) > 4) bad.push('columns differ by ' + Math.round(Math.max(...b) - Math.min(...b)));
					for (const c of p.children) {
						// Empty space between a card's last content and its bottom edge.
						const inner = c.classList.contains('c-feat') ? c.querySelector('.fbars').lastElementChild : c.lastElementChild;
						const parts = inner.children.length ? [...inner.children] : [inner];
						const gap = c.getBoundingClientRect().bottom - Math.max(...parts.map((x) => x.getBoundingClientRect().bottom));
						const atMinimum = c.classList.contains('c-attn') && !c.querySelector('.att-g + li + li:not(.att-g):not(.att-more)');
						if (gap > 72 && !atMinimum) bad.push(c.className + ' ends ' + Math.round(gap) + ' px above its edge');
					}
				}
				return bad; })()`, &d)
			if len(d) > 0 {
				return fail("width %d: %v", w, d)
			}
		}
		return nil
	})
	s.check("needs attention keeps every group and counts each row", func() error {
		s.goTo("/dashboard")
		var groups []struct {
			Label        string
			Count, Shown int
			More, Want   int
		}
		s.eval(`(() => { const D = JSON.parse(document.getElementById('report-data').textContent); const n = {};
			for (const f of D.features) for (const e of f.elements) n[e.status] = (n[e.status] || 0) + 1;
			const key = { 'Unexpected failures': 'failed', 'Ambiguous steps': 'ambiguous', 'Undefined steps': 'undefined', 'Known failures': 'known', 'Pending': 'pending' };
			const out = []; let cur = null;
			for (const li of document.querySelectorAll('.attn > li')) {
				if (li.classList.contains('att-g')) { cur = { label: li.firstChild.textContent, count: +li.lastChild.textContent.replace(/,/g, ''), shown: 0, more: 0 }; cur.want = n[key[cur.label]] || 0; out.push(cur); }
				else if (li.classList.contains('att-more')) cur.more = +li.textContent.replace(/[^0-9]/g, '');
				else cur.shown++;
			}
			const want = Object.entries(key).filter(([, k]) => n[k]).map(([l]) => l);
			if (out.map((g) => g.label).join('|') !== want.join('|')) out.push({ label: 'groups ' + out.map((g) => g.label).join(',') + ' want ' + want.join(','), count: -1 });
			return out; })()`, &groups)
		for _, g := range groups {
			if g.Count != g.Want || g.Shown+g.More != g.Count || g.Shown < 1 {
				return fail("%+v", g)
			}
		}
		return nil
	})
	s.check("known-issue records that cover nothing are not counted as known", func() error {
		var d struct {
			Issues int
			NA     []string
		}
		s.eval(`(() => { const D = JSON.parse(document.getElementById('report-data').textContent); const k = (D.annotations || []).filter((a) => a.kind === 'known');
			return { issues: new Set(k.filter((a) => a.state === 'applied' || a.state === 'passed' || a.state === 'not-applicable').map((a) => a.ref)).size, na: k.filter((a) => a.state === 'not-applied' || a.state === 'invalid').map((a) => a.ref) }; })()`, &d)
		s.goTo("/known")
		head := s.str(`document.querySelector('.page-s').textContent`)
		if want := fmt.Sprintf("across %d issue", d.Issues); !strings.Contains(head, want) {
			return fail("Known issues reads %q, want %q", head, want)
		}
		var na []string
		s.eval(`[...document.querySelectorAll('.na-c')].filter((c) => /Known issue, not applied/.test(c.textContent) && c.querySelector('.banner-w').textContent.length > 10 && c.querySelector('.glyph.s-known') === null).map((c) => c.querySelector('.ref').textContent)`, &na)
		if fmt.Sprint(na) != fmt.Sprint(d.NA) {
			return fail("not-applied cards %v, want %v", na, d.NA)
		}
		s.goTo("/dashboard")
		var refs []string
		s.eval(`[...document.querySelectorAll('.c-known .refs .ref')].map((r) => r.textContent)`, &refs)
		for _, r := range refs {
			for _, x := range d.NA {
				if r == x {
					return fail("dashboard lists not-applied record %s among known issues", r)
				}
			}
		}
		if len(d.NA) > 0 && !s.is(`document.querySelector('.c-known .note.na')`) {
			return fail("dashboard does not mention the records not applied")
		}
		s.goTo("/print")
		for _, x := range d.NA {
			if !s.is(`/Known issue, not applied: ` + regexp.QuoteMeta(x) + `/.test(document.getElementById('print-root').textContent)`) {
				return fail("print lacks the not-applied record %s", x)
			}
		}
		return nil
	})
	s.check("histogram colors follow its caption", func() error {
		s.goTo("/dashboard")
		var r struct{ Failed, Known, Red, Amber int }
		s.eval(`(() => { const D = JSON.parse(document.getElementById('report-data').textContent); let failed = 0, known = 0, slow = false;
			for (const f of D.features) for (const e of f.elements) { const d = e.steps.reduce((a, x) => a + (x.duration || 0), 0); if (d >= 1e6) slow = true; if (e.status === 'failed') failed++; if (e.status === 'known') known++; }
			if (!document.querySelector('.hist')) return { failed: 0, known: 0, red: 0, amber: 0 };
			const sum = (re) => [...document.querySelectorAll('.hist-c')].reduce((a, c) => a + (+((c.getAttribute('aria-label').match(re) || [0, 0])[1])), 0);
			return { failed, known, red: sum(/, (\d+) failed/), amber: sum(/, (\d+) known/) }; })()`, &r)
		if r.Red != r.Failed || r.Amber != r.Known {
			return fail("%+v", r)
		}
		return nil
	})
	s.check("print preview starts under its header; timeline axis starts at 0", func() error {
		s.goTo("/print")
		gap := s.num(`document.getElementById('print-root').getBoundingClientRect().top - document.querySelector('.page-h').getBoundingClientRect().bottom`)
		if gap < 0 || gap > 48 {
			return fail("print document starts %d px below the header", gap)
		}
		s.goTo("/timeline")
		if first := s.str(`document.querySelector('.gantt-x span').textContent`); first != "0" {
			return fail("axis starts with %q", first)
		}
		return nil
	})
	s.check("timeline axis labels never overlap", func() error {
		defer s.size(1440, 900)
		for _, w := range []int{390, 768, 1440} {
			s.size(w, 900)
			s.goTo("/timeline")
			var r struct {
				N       int
				Overlap []string
			}
			s.eval(`(() => { const xs = [...document.querySelectorAll('.gantt-x span')].map((e) => ({ t: e.textContent, b: e.getBoundingClientRect() })); const bad = [];
				for (let i = 1; i < xs.length; i++) if (xs[i].b.left < xs[i - 1].b.right + 2) bad.push(xs[i - 1].t + ' / ' + xs[i].t);
				return { n: xs.length, overlap: bad }; })()`, &r)
			if r.N < 1 || r.N > 11 || len(r.Overlap) > 0 {
				return fail("%d px: %d labels, overlapping %v", w, r.N, r.Overlap)
			}
		}
		return nil
	})
	s.check("status legend labels never overlap or clip", func() error {
		defer s.size(1440, 900)
		defer s.eval(`localStorage.clear(), true`, nil)
		for _, theme := range []string{"light", "dark"} {
			s.eval(`localStorage.setItem('cucumber-reporter:v2:prefs', JSON.stringify({ v: 2, theme: '`+theme+`' })), true`, nil)
			for _, w := range []int{1440, 1366, 1280, 1180, 1024, 900, 720, 390} {
				s.size(w, 900)
				s.goTo("/dashboard")
				var bad []string
				s.eval(`(() => { const bad = [];
					for (const leg of document.querySelectorAll('.legend .leg')) {
						const l = leg.querySelector('.leg-l'), n = leg.querySelector('.leg-n'), p = leg.querySelector('.leg-p');
						const lr = l.getBoundingClientRect(), nr = n.getBoundingClientRect(), pr = p.getBoundingClientRect();
						if (lr.right > nr.left + 0.5 || nr.right > pr.left + 0.5) bad.push(l.textContent + ' overlaps');
						if (l.scrollWidth > l.clientWidth + 1) bad.push(l.textContent + ' is clipped');
						if (pr.right > leg.getBoundingClientRect().right + 0.5) bad.push(l.textContent + ' overflows its row');
					}
					return bad; })()`, &bad)
				if len(bad) > 0 {
					return fail("%s, %d px: %v", theme, w, bad)
				}
			}
		}
		return nil
	})
	s.check("no CSP violations or console errors across the run", func() error {
		if p := s.b.Problems(); len(p) > 0 {
			return fail("%v", p)
		}
		return nil
	})
}

func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

var pdfPage = regexp.MustCompile(`/Type\s*/Page[^s]`)

func pdfPages(pdf []byte) int { return len(pdfPage.FindAll(pdf, -1)) }

// TestBrowserMotion loads the dashboard with motion enabled: the count-up reaches the final values and
// the draw-in animations raise no errors.
func TestBrowserMotion(t *testing.T) {
	features, opt := loadShowcase(t, "-known")
	s := openSession(t, features, opt, false)
	s.check("counters reach the headline value", func() error {
		// The expected figure, computed here from the data block: (passed + known) / executed.
		want := s.str(`(() => { const D = JSON.parse(document.getElementById('report-data').textContent); const n = {};
			for (const f of D.features) for (const e of f.elements) n[e.status] = (n[e.status] || 0) + 1;
			const v = ((n.passed || 0) + (n.known || 0)) * 100 / (Object.values(n).reduce((a, b) => a + b, 0) - (n.skipped || 0));
			return (v === 100 || v === 0 ? v.toFixed(0) : v.toFixed(1)) + '%'; })()`)
		if !s.waitFor(`document.querySelectorAll('.kpi')[3].querySelector('.kpi-v').textContent.trim() === ` + jsString(want) + ` && document.querySelector('.hero-rv').textContent.trim() === ` + jsString(want)) {
			return fail("hero %q, want %q", s.str(`document.querySelector('.hero-rv').textContent`), want)
		}
		if !s.waitFor(`[...document.querySelectorAll('.donut .seg')].every((c) => !c.getAttribute('stroke-dasharray').startsWith('0 '))`) {
			return fail("donut did not draw in")
		}
		return nil
	})
}

// TestBrowserDefaultsAndPreferences: generation-time defaults reach the page; saved preferences that
// are invalid fall back to them.
func TestBrowserDefaultsAndPreferences(t *testing.T) {
	features, opt := loadShowcase(t, "")
	opt.Defaults = ViewDefaults{Theme: ThemeDark, Density: DensityCompact, View: ViewFailures, Widgets: map[string]bool{WidgetDuration: false}, Nav: map[string]bool{NavRun: false}}
	s := openSession(t, features, opt, true)
	state := `[document.documentElement.dataset.theme, document.documentElement.dataset.density, document.querySelector('.page-t').textContent, !!document.querySelector('.nav-i[data-k=run]')].join('|')`
	s.check("defaults apply without saved preferences", func() error {
		s.goTo("")
		if got := s.str(state); got != "dark|compact|Failures|false" {
			return fail("state %s", got)
		}
		s.goTo("/dashboard") // "#/" opens the start page, here Failures
		if s.is(`document.querySelector('.c-dur')`) || !s.is(`document.querySelector('.c-status')`) {
			return fail("widget defaults not applied: %s", s.str(`[...document.querySelectorAll('.card')].map((c) => c.className).join(',') + ' | ' + location.hash`))
		}
		return nil
	})
	for _, prefs := range []string{`'{not json'`, `JSON.stringify({v: 2, theme: 'neon', density: 5, view: 'nope', widgets: [1], nav: {dashboard: false, run: 'yes'}})`, `'[]'`, `'null'`, `JSON.stringify({v: 1, theme: 'light'})`} {
		s.check("invalid saved preferences fall back: "+prefs, func() error {
			s.eval(`localStorage.setItem('cucumber-reporter:v2:prefs', `+prefs+`), true`, nil)
			s.goTo("")
			if got := s.str(state); got != "dark|compact|Failures|false" {
				return fail("state %s", got)
			}
			return nil
		})
	}
	s.check("valid saved preferences override the defaults", func() error {
		s.eval(`localStorage.setItem('cucumber-reporter:v2:prefs', JSON.stringify({v: 2, theme: 'light', density: 'comfortable', view: 'features', nav: {run: true}, bogus: 1})), true`, nil)
		s.goTo("")
		if got := s.str(state); got != "light|comfortable|Features|true" {
			return fail("state %s", got)
		}
		return nil
	})
}

// TestBrowserNoScript: with JavaScript disabled the fallback shows the run and prints.
func TestBrowserNoScript(t *testing.T) {
	features, opt := loadShowcase(t, "")
	s := openSession(t, features, opt, true)
	s.call("Emulation.setScriptExecutionDisabled", map[string]any{"value": true})
	s.loads++
	if err := s.b.Navigate(s.ctx, "file://"+s.file+"?load="+strconv.Itoa(s.loads)); err != nil {
		t.Fatal(err)
	}
	var text string
	s.eval(`document.querySelector('.nj') ? document.querySelector('.nj').textContent : ''`, &text)
	for _, want := range []string{"Failed: 3 unexpected failures", "Run facts", "Summary", "Order processing", "Known failure EX-330"} {
		if !strings.Contains(text, want) {
			t.Errorf("fallback lacks %q", want)
		}
	}
	if s.is(`document.querySelector('.top')`) {
		t.Error("the app shell was built although scripts were disabled")
	}
	pdf, err := s.b.PrintPDF(s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if pdfPages(pdf) < 2 {
		t.Errorf("no-JS print has %d pages", pdfPages(pdf))
	}
	if p := s.b.Problems(); len(p) > 0 {
		t.Errorf("browser problems: %v", p)
	}
}

// TestBrowserStress: the 5,000-step page loads with a small DOM and a virtualised scenario list.
// Timings are logged, not asserted.
func TestBrowserStress(t *testing.T) {
	s := openSession(t, stressRun(), renderOptions{Version: OptionsVersion}, true)
	var m struct {
		Nodes, Rows            int
		Route, Filter, Palette float64
	}
	s.goTo("/")
	m.Nodes = s.num(`document.getElementsByTagName('*').length`)
	s.eval(`(() => { const t0 = performance.now(); location.hash = '#/scenarios'; window.dispatchEvent(new HashChangeEvent('hashchange')); return performance.now() - t0; })()`, &m.Route)
	s.waitFor(`document.querySelector('.rows .vl')`)
	s.settle()
	m.Rows = s.num(`document.querySelectorAll('.rows .row').length`)
	s.eval(`(() => { const i = document.querySelector('.tb-search input'); i.value = 'case 07'; const t0 = performance.now(); i.dispatchEvent(new Event('input')); return new Promise((r) => setTimeout(() => r(performance.now() - t0 - 60), 61)); })()`, &m.Filter)
	s.eval(`(() => { document.dispatchEvent(new KeyboardEvent('keydown', { key: '/' })); const i = document.querySelector('.pal-in'); const t0 = performance.now(); i.value = 'request 12'; i.dispatchEvent(new Event('input')); return performance.now() - t0; })()`, &m.Palette)
	t.Logf("stress page (1,000 scenarios, 5,000 steps): %d DOM nodes at load, scenario route %.1f ms, filter keystroke about %.1f ms, palette search %.1f ms, %d list rows rendered", m.Nodes, m.Route, m.Filter, m.Palette, m.Rows)
	if m.Rows == 0 || m.Rows > 100 {
		t.Errorf("%d rows rendered for 1,000 scenarios; the list must be virtualised", m.Rows)
	}
	if m.Nodes > 5000 {
		t.Errorf("%d DOM nodes at load; the dashboard must not render every scenario", m.Nodes)
	}
	if p := s.b.Problems(); len(p) > 0 {
		t.Errorf("browser problems: %v", p)
	}
}

// TestBrowserPDF prints each page and checks the PDF the way the prototype's tools/pdfcheck.py did:
// no near-empty page except the last, and no forbidden text.
func TestBrowserPDF(t *testing.T) {
	gs := os.Getenv("GS")
	if gs == "" {
		var err error
		if gs, err = exec.LookPath("gs"); err != nil {
			t.Fatal("Ghostscript not found; set GS or install gs")
		}
	}
	showcase, showcaseOpt := loadShowcase(t, "")
	known, knownOpt := loadShowcase(t, "-known")
	for _, p := range []struct {
		name     string
		features []cucumber.Feature
		opt      renderOptions
		fillRule bool // a one-scenario run has a short cover page by design; its fill is logged only
	}{
		{"showcase", showcase, showcaseOpt, true},
		{"known", known, knownOpt, true},
		{"minimal", loadFixture(t, "minimal.json"), renderOptions{Version: OptionsVersion}, false},
		{"review", loadFixture(t, "review/input/cucumber.json"), loadOptions(t, "review/input/options.json"), false},
	} {
		t.Run(p.name, func(t *testing.T) {
			s := openSession(t, p.features, p.opt, true)
			s.eval(`window.dispatchEvent(new Event('beforeprint')), true`, nil)
			pdf, err := s.b.PrintPDF(s.ctx)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			file := filepath.Join(dir, p.name+".pdf")
			if err := os.WriteFile(file, pdf, 0o600); err != nil {
				t.Fatal(err)
			}
			if out := os.Getenv("PDF_OUT"); out != "" {
				_ = os.WriteFile(filepath.Join(out, p.name+".pdf"), pdf, 0o600)
			}
			fill := pageFill(t, gs, file, dir)
			t.Logf("%s: %d pages, fill %v%%", p.name, len(fill), fill)
			for i, f := range fill[:len(fill)-1] {
				if p.fillRule && f < 35 {
					t.Errorf("page %d of %d is %d%% filled; only the last page may be under 35%%", i+1, len(fill), f)
				}
			}
			text := strings.Join(strings.Fields(pdfText(t, gs, file)), " ")
			for _, banned := range []string{"not applicable: ", "Pass rate 0%", "file://", "Reset to report defaults"} {
				if strings.Contains(text, banned) {
					t.Errorf("PDF contains %q", banned)
				}
			}
			if !strings.Contains(text, "Page 1 of") || !strings.Contains(text, "End of report") {
				t.Error("PDF lacks the page footer or the end marker")
			}
			if p.name == "review" && !strings.Contains(text, "Coverage ROW_SENTINEL") {
				t.Error("PDF lacks the section row (Coverage: ROW_SENTINEL)")
			}
			if (p.name == "showcase" || p.name == "known") && !strings.Contains(text, "EX-330: Expected failure: no push is sent to that customer (step 3)") {
				t.Error("PDF lacks the declared known-issue outcome")
			}
		})
	}
}

// pageFill rasterises the PDF at 24 dpi and returns, per page, the percentage of rows between 9% and
// 91% of the height (inside the margin boxes) that contain a dark pixel.
func pageFill(t *testing.T, gs, pdf, dir string) []int {
	t.Helper()
	out := filepath.Join(dir, "p-%03d.pgm")
	if b, err := exec.CommandContext(t.Context(), gs, "-q", "-dNOPAUSE", "-dBATCH", "-dSAFER", "-sDEVICE=pgmraw", "-r24", "-sOutputFile="+out, pdf).CombinedOutput(); err != nil {
		t.Fatalf("gs: %v: %s", err, b)
	}
	names, _ := filepath.Glob(filepath.Join(dir, "p-*.pgm"))
	var fill []int
	for _, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		var vals []string
		pos := 0
		for len(vals) < 4 {
			end := pos + bytes.IndexByte(data[pos:], '\n')
			line := data[pos:end]
			pos = end + 1
			if !bytes.HasPrefix(line, []byte("#")) {
				vals = append(vals, strings.Fields(string(line))...)
			}
		}
		w, _ := strconv.Atoi(vals[1])
		h, _ := strconv.Atoi(vals[2])
		px := data[pos:]
		ink, rows := 0, 0
		for y := h * 9 / 100; y < h*91/100; y++ {
			rows++
			for _, v := range px[y*w : (y+1)*w] {
				if v < 200 {
					ink++
					break
				}
			}
		}
		fill = append(fill, ink*100/rows)
	}
	return fill
}

func pdfText(t *testing.T, gs, pdf string) string {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), gs, "-q", "-dNOPAUSE", "-dBATCH", "-dSAFER", "-sDEVICE=txtwrite", "-sOutputFile=-", pdf).Output()
	if err != nil {
		t.Fatalf("gs txtwrite: %v", err)
	}
	return string(out)
}

// TestBrowserHostileData renders each attack string of the escaping table into every field and visits
// every view: the page must build no element, attribute or link from the data, raise no dialog and
// report no exception or CSP violation.
func TestBrowserHostileData(t *testing.T) {
	features, opt := hostileRun("placeholder")
	s := openSession(t, features, opt, true)
	s.call("Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": `for (const k of ['alert', 'confirm', 'prompt', 'print']) window[k] = () => { document.documentElement.setAttribute('data-dialog', k); };`})
	s.check("routes named after object prototype properties", func() error {
		names := []string{"constructor", "__proto__", "toString", "hasOwnProperty", "valueOf", "isPrototypeOf", "feature", "missing"}
		for _, n := range names {
			for _, r := range []string{"/" + n, "/features/" + n, "/scenarios/" + n, "/scenarios?status=" + n, "/run?section=" + n} {
				s.goTo(r)
				if s.is(`/could not be shown|\[object/.test(document.querySelector('main').textContent)`) {
					return fail("route %s: %s", r, s.str(`document.querySelector('main').textContent.slice(0, 120)`))
				}
			}
		}
		return nil
	})
	for i, h := range hostile {
		features, opt := hostileRun(h)
		file := filepath.Join(filepath.Dir(s.file), "hostile-"+strconv.Itoa(i)+".html")
		if err := os.WriteFile(file, []byte(mustRender(t, features, opt)), 0o600); err != nil {
			t.Fatal(err)
		}
		s.file = file
		fid, sid := featureIDs(features)[0], scenarioIDs(features)[0][0]
		s.check(fmt.Sprintf("string %d", i), func() error {
			for _, r := range []string{"/", "/features", "/features/" + fid, "/scenarios", "/scenarios/" + sid, "/failures", "/known", "/skipped", "/timeline", "/run", "/print"} {
				s.goTo(r)
				s.eval(`(() => { for (const b of document.querySelectorAll('.att-h')) if (b.getAttribute('aria-expanded') !== 'true') b.click(); document.dispatchEvent(new KeyboardEvent('keydown', { key: '/' })); const i = document.querySelector('.pal-in'); i.value = 'a'; i.dispatchEvent(new Event('input')); return true; })()`, nil)
				s.settle()
				var bad []string
				s.eval(`(() => { const bad = [];
					const q = (sel) => [...document.querySelectorAll(sel)];
					for (const e of q('img, iframe, object, embed, math, foreignObject, base, form, link, meta[http-equiv=refresh]')) if (e.tagName !== 'META' || e.httpEquiv.toLowerCase() !== 'content-security-policy') bad.push('element ' + e.tagName);
					for (const e of q('*')) for (const a of e.attributes) {
						if (/^on/i.test(a.name)) bad.push('attribute ' + a.name + ' on ' + e.tagName);
						// CSSOM geometry (positions, sizes, delays) is computed from numbers; nothing else may appear.
						if (a.name === 'style' && (!/^[\w\s:;.%()-]*$/.test(a.value) || /url|expression|image/i.test(a.value))) bad.push('style ' + a.value.slice(0, 40));
					}
					for (const a of q('a[href], [src], [xlink\\:href]')) { const v = a.getAttribute('href') || a.getAttribute('src') || ''; if (!/^#\//.test(v) && !/^attachment-/.test(v) && v !== '#main') bad.push('link ' + v.slice(0, 40)); }
					if (q('script').length !== 2) bad.push(q('script').length + ' scripts');
					if (document.documentElement.hasAttribute('data-dialog')) bad.push('dialog ' + document.documentElement.getAttribute('data-dialog'));
					return bad; })()`, &bad)
				if len(bad) > 0 {
					return fail("route %s: %v", r, bad)
				}
			}
			// The run name and verdict reach the print header through one inserted @page rule, as CSS
			// strings; decoded, they must equal the data exactly (control characters become spaces).
			s.goTo("/print")
			var page struct{ Rules, Want []string }
			s.eval(`(() => { const D = JSON.parse(document.getElementById('report-data').textContent); const rules = [];
				for (const sh of document.styleSheets) for (const r of sh.cssRules) if (r.type === CSSRule.PAGE_RULE) rules.push(r.cssText);
				rules.splice(0, rules.length - 1); // the rule the script inserts comes last
				const dec = (q) => q.slice(1, -1).replace(/\\([0-9a-fA-F]{1,6}) ?|\\([^0-9a-fA-F])/g, (m, h, c) => (h ? String.fromCodePoint(parseInt(h, 16)) : c));
				const got = rules.flatMap((t) => [...t.matchAll(/content:\s*("(?:\\.|[^"\\])*")/g)].map((m) => dec(m[1])));
				const clean = (x) => x.replace(/[\u0000-\u001f\u007f]/g, ' ');
				return { rules: got, want: [clean(D.name), clean(D.verdict.label)] }; })()`, &page)
			if fmt.Sprint(page.Rules) != fmt.Sprint(page.Want) {
				return fail("print header strings %q, want %q", page.Rules, page.Want)
			}
			return nil
		})
	}
}

// TestBrowserCallerVerdict: the page shows the verdict tone the renderer sends, not one it computes.
func TestBrowserCallerVerdict(t *testing.T) {
	features, opt := loadShowcase(t, "")
	opt.Verdict = &Verdict{Label: "Accepted by the release board", Tone: ToneInfo}
	s := openSession(t, features, opt, true)
	s.check("pill, hero and print use the caller's tone", func() error {
		s.goTo("/dashboard")
		got := s.str(`[document.querySelector('.verdict-pill').className, document.querySelector('.hero').className, document.querySelector('.verdict-pill').textContent].join('|')`)
		if got != "verdict-pill t-info|hero t-info|Accepted by the release board" {
			return fail("%s", got)
		}
		s.goTo("/print")
		if !s.is(`document.querySelector('.pr-verdict.t-info')`) {
			return fail("print verdict tone")
		}
		return nil
	})
}

// TestBrowserAttachmentKinds: raster images render as images, SVG and HTML only as text, and the page
// links only to the renderer's own sidecar names even when the data block is altered.
func TestBrowserAttachmentKinds(t *testing.T) {
	png := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="
	svg := `<svg xmlns="http://www.w3.org/2000/svg"><text>svg-attachment-body</text></svg>`
	big := strings.Repeat("evidence line\n", 200)
	features := one(step("s", cucumber.StatusFailed, 1,
		cucumber.Embedding{Name: "pixel.png", MimeType: "image/png", Data: png},
		cucumber.Embedding{Name: "drawing.svg", MimeType: "image/svg+xml", Data: b64(svg)},
		cucumber.Embedding{Name: "page.html", MimeType: "text/html", Data: b64("<b>html-attachment-body</b>")},
		cucumber.Embedding{Name: "big.txt", MimeType: "text/plain", Data: b64(big)},
	))
	dir, err := renderFeaturesDirTo(features, t.TempDir(), renderOptions{Version: OptionsVersion, Limits: pub.Limits{InlineAttachment: 1024}, Defaults: ViewDefaults{Expand: ExpandAll}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(raw)
	const sidecar = `"href":"attachment-f1-s1-t1-a4.txt"`
	if !strings.Contains(page, sidecar) {
		t.Fatal("no sidecar link in the data block")
	}
	route := "/scenarios/" + scenarioIDs(features)[0][0]
	s := openPage(t, page, true)
	s.check("image, SVG, HTML and sidecar", func() error {
		s.goTo(route)
		got := s.str(`[document.querySelectorAll('img').length, document.querySelector('img') ? document.querySelector('img').getAttribute('src').slice(0, 21) : '',
			[...document.querySelectorAll('pre')].some((p) => p.textContent.includes('svg-attachment-body')), [...document.querySelectorAll('pre')].some((p) => p.textContent.includes('<b>html-attachment-body</b>')),
			[...document.querySelectorAll('a[href^="attachment-"]')].map((a) => a.getAttribute('href')).join(',')].join('|')`)
		if got != "1|data:image/png;base64|true|true|attachment-f1-s1-t1-a4.txt" {
			return fail("%s", got)
		}
		return nil
	})
	for _, href := range []string{"javascript:attachment-a.txt", "https://example.invalid/attachment-a.txt", "attachment-a.txt?x=1", "../attachment-a.txt", "attachment-a.txt/../../x.txt", "xattachment-a.txt", "attachment-a.html"} {
		s.check("altered sidecar link "+href, func() error {
			altered := strings.Replace(page, sidecar, `"href":`+jsString(href), 1)
			if err := os.WriteFile(s.file, []byte(altered), 0o600); err != nil {
				return err
			}
			s.goTo(route)
			if n := s.num(`document.querySelectorAll('.atts a[href]').length`); n != 0 {
				return fail("%d links built from an altered href", n)
			}
			return nil
		})
	}
}

// TestBrowserNotApplicable: a passed scenario whose known-issue record has only not-applicable
// outcomes reads "Not applicable in this run" and is not flagged as possibly fixed.
func TestBrowserNotApplicable(t *testing.T) {
	features, opt := loadShowcase(t, "")
	s := openSession(t, features, opt, true)
	s.check("scenario banner and Known issues page", func() error {
		s.goTo("/scenarios/accounts-L4")
		heading := s.str(`[...document.querySelectorAll('.banner-header')].filter((b) => /EX-501/.test(b.textContent)).map((b) => b.querySelector('.banner-t').textContent).join('|')`)
		if heading != "Not applicable in this run" || s.is(`document.querySelector('.banner .fixed')`) {
			return fail("banner heading %q", heading)
		}
		s.goTo("/known")
		if !s.is(`[...document.querySelectorAll('.grp')].some((g) => /EX-501/.test(g.textContent) && g.querySelector('.oos') && !g.querySelector('.fixed'))`) {
			return fail("Known issues does not show EX-501 as not applicable")
		}
		s.goTo("/print")
		pr := s.str(`document.getElementById('print-root').textContent`)
		if !strings.Contains(pr, "Known issue EX-501 · not applicable in this run: Captcha blocks registration from new regions") || strings.Contains(pr, "Known failure EX-501") {
			return fail("print does not show EX-501 as not applicable")
		}
		s.goTo("/dashboard")
		if note := s.str(`document.querySelector('.c-known .note.na').textContent`); note != "1 known-issue record not applied; its scenario keeps its own result." {
			return fail("dashboard note %q", note)
		}
		return nil
	})
}

// loadOptions reads an options file (the command's -options format) from testdata/synthetic.
func loadOptions(t testing.TB, name string) renderOptions {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "synthetic", name))
	if err != nil {
		t.Fatal(err)
	}
	var opt renderOptions
	if err := json.Unmarshal(raw, &opt); err != nil {
		t.Fatal(err)
	}
	return opt
}

// noNullText is a page expression that lists text nodes reading exactly "null" or "undefined".
const noNullText = `(() => { const w = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT); const bad = [];
	for (let n = w.nextNode(); n; n = w.nextNode()) if (/^(null|undefined)$/.test(n.data.trim())) bad.push(n.parentNode.className || n.parentNode.tagName);
	return bad; })()`

// TestBrowserReviewFixtures replays the v0.1.0 review's fixtures (testdata/synthetic/review).
func TestBrowserReviewFixtures(t *testing.T) {
	t.Run("maximum duration keeps the timeline axis short", func(t *testing.T) {
		s := openSession(t, loadFixture(t, "review/duration/cucumber.json"), renderOptions{Version: OptionsVersion}, true)
		s.check("axis labels", func() error {
			s.goTo("/timeline")
			n := s.num(`document.querySelectorAll('.gantt-x span').length`)
			if n < 2 || n > 12 {
				return fail("%d axis labels", n)
			}
			for _, w := range []int{390, 768} {
				s.size(w, 900)
				s.goTo("/timeline")
				if s.is(`(() => { const b = [...document.querySelectorAll('.gantt-x span')].map((e) => e.getBoundingClientRect()); return b.some((r, i) => i && r.left < b[i - 1].right + 2); })()`) {
					s.size(1440, 900)
					return fail("axis labels overlap at %d px", w)
				}
			}
			s.size(1440, 900)
			var bad []string
			s.eval(noNullText, &bad)
			if len(bad) > 0 {
				return fail("null text in %v", bad)
			}
			return nil
		})
	})
	t.Run("attachments keep their original text", func(t *testing.T) {
		s := openSession(t, loadFixture(t, "review/input/cucumber.json"), loadOptions(t, "review/input/options.json"), true)
		s.call("Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": `window.__copied = []; Object.defineProperty(navigator, 'clipboard', { value: { writeText: (t) => { window.__copied.push(t); return Promise.resolve(); } } });`})
		const original = `{"id":9007199254740993,"value":1e400,"repeat":1,"repeat":2}`
		s.check("raw view and copy of JSON", func() error {
			s.goTo("/scenarios/review-L1")
			s.eval(`(() => { const b = [...document.querySelectorAll('.att-h')].find((x) => /evidence\.json/.test(x.textContent)); if (b.getAttribute('aria-expanded') !== 'true') b.click(); return true; })()`, nil)
			acc := `[...document.querySelectorAll('.att-x')].find((x) => /evidence\.json/.test(x.textContent))`
			s.eval(acc+`.querySelector('[role=radio]:last-child').click()`, nil)
			s.settle()
			raw := s.str(acc + `.querySelector('.att-c pre').textContent`)
			s.eval(acc+`.querySelector('.att-tools .btn:not([role])').click()`, nil)
			s.settle()
			copied := s.str(`window.__copied.join('|')`)
			if raw != original || copied != original {
				return fail("raw %q, copied %q; want %q", raw, copied, original)
			}
			return nil
		})
		s.check("empty attachment", func() error {
			s.goTo("/scenarios/review-L1")
			acc := `[...document.querySelectorAll('.att-x')].find((x) => /empty\.txt/.test(x.textContent))`
			s.eval(acc+`.querySelector('.att-h').click()`, nil)
			s.settle()
			got := s.str(acc + `.textContent`)
			if strings.Contains(got, "could not be decoded") || !strings.Contains(got, "Empty attachment") || s.num(acc+`.querySelectorAll('.att-tools .btn').length`) != 2 {
				return fail("empty attachment shows %q", got)
			}
			return nil
		})
		s.check("print keeps section rows", func() error {
			s.goTo("/print")
			if !s.is(`/ROW_SENTINEL/.test(document.getElementById('print-root').textContent)`) {
				return fail("print lacks the section row")
			}
			return nil
		})
	})
	t.Run("raw text keeps a leading byte order mark", func(t *testing.T) {
		const text = "\ufeff{\"bom\": true}"
		features := one(step("s", cucumber.StatusPassed, 1, cucumber.Embedding{Name: "bom.json", MimeType: "application/json", Data: b64(text)}))
		s := openSession(t, features, renderOptions{Version: OptionsVersion}, true)
		s.call("Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": `window.__copied = []; Object.defineProperty(navigator, 'clipboard', { value: { writeText: (t) => { window.__copied.push(t); return Promise.resolve(); } } });`})
		s.check("raw and copy", func() error {
			s.goTo("/scenarios/" + scenarioIDs(features)[0][0])
			s.eval(`(() => { const b = document.querySelector('.att-h'); if (b.getAttribute('aria-expanded') !== 'true') b.click(); return true; })()`, nil)
			s.eval(`document.querySelector('.att-x [role=radio]:last-child') && document.querySelector('.att-x [role=radio]:last-child').click()`, nil)
			s.settle()
			raw := s.str(`document.querySelector('.att-x .att-c pre').textContent`)
			s.eval(`document.querySelector('.att-x .att-tools .btn:not([role])').click()`, nil)
			s.settle()
			if copied := s.str(`window.__copied.join('|')`); raw != text || copied != text {
				return fail("raw %q, copied %q; want %q", raw, copied, text)
			}
			return nil
		})
	})
	t.Run("the last virtualised row is fully visible", func(t *testing.T) {
		s := openSession(t, loadFixture(t, "review/stress/cucumber.json"), renderOptions{Version: OptionsVersion}, true)
		for _, density := range []string{"comfortable", "compact"} {
			for _, w := range []int{1440, 768, 390} {
				s.check(fmt.Sprintf("%s at %d px", density, w), func() error {
					s.size(w, 900)
					defer s.size(1440, 900)
					s.eval(`localStorage.setItem('cucumber-reporter:v2:prefs', JSON.stringify({ v: 2, density: '`+density+`' })), true`, nil)
					s.goTo("/scenarios")
					s.eval(`window.scrollTo(0, document.documentElement.scrollHeight), true`, nil)
					s.settle()
					s.eval(`window.scrollTo(0, document.documentElement.scrollHeight), true`, nil)
					s.settle()
					var r struct {
						Found                          bool
						Top, Bottom, ClipBottom, Inner float64
						Null                           []string
					}
					s.eval(`(() => { const row = [...document.querySelectorAll('.rows .row')].find((x) => /Case 1000/.test(x.textContent)); if (!row) return { found: false };
						const b = row.getBoundingClientRect(), pad = document.querySelector('.rows .vl') || document.querySelector('.rows');
						return { found: true, top: b.top, bottom: b.bottom, clipBottom: pad.getBoundingClientRect().bottom, inner: window.innerHeight, null: `+noNullText+` }; })()`, &r)
					if !r.Found || r.Top < 0 || r.Bottom > r.Inner+0.5 || r.Bottom > r.ClipBottom+0.5 || len(r.Null) > 0 {
						return fail("%+v", r)
					}
					return nil
				})
			}
		}
	})
}

func TestBrowserElapsedTimeLinksToMeasuredPhases(t *testing.T) {
	features, opt := loadShowcase(t, "")
	opt.WallClock = "19m26s"
	opt.TimingSection = "execution-time"
	opt.Sections = append(opt.Sections, Section{ID: "execution-time", Title: "Measured phases", Rows: []Fact{{Label: "Setup", Value: "2m"}}})
	s := openSession(t, features, opt, true)
	s.check("elapsed time, summed steps and phase navigation", func() error {
		s.goTo("/dashboard")
		if !s.is(`Array.from(document.querySelectorAll('a')).some(a=>a.getAttribute('href')==='#/run?section=execution-time' && a.textContent.includes('19m26s') && a.textContent.includes('Total elapsed time') && a.textContent.includes('summed steps'))`) {
			return fail("elapsed time card missing")
		}
		s.goTo("/run?section=execution-time")
		if !s.is(`document.body.textContent.includes('Measured phases') && document.body.textContent.includes('2m')`) {
			return fail("phase section missing")
		}
		s.goTo("/print")
		if !s.is(`document.querySelector('.pr-kpis').textContent.includes('19m26s') && document.querySelector('.pr-kpis').textContent.includes('Total elapsed time') && document.querySelector('.pr-kpis').textContent.includes('Summed step time:')`) {
			return fail("print elapsed time missing")
		}
		return nil
	})
}

func TestBrowserTimingWithoutElapsedTimeKeepsSlowestNavigation(t *testing.T) {
	features, opt := loadShowcase(t, "")
	opt.WallClock = " \t "
	s := openSession(t, features, opt, true)
	s.check("summed time without an invented elapsed duration", func() error {
		s.goTo("/dashboard")
		if !s.is(`(() => { const a = document.querySelector('.kpis a[href="#/scenarios?sort=slowest"]'); return a && a.textContent.includes('Summed step time') && !a.querySelector('.kpi-s').textContent.includes('summed steps') && !a.textContent.includes('across workers'); })()`) {
			return fail("summed time card or legacy navigation missing")
		}
		s.goTo("/print")
		if !s.is(`document.querySelector('.pr-kpis').textContent.includes('Summed step time') && !document.querySelector('.pr-kpis').textContent.includes('Total elapsed time')`) {
			return fail("print invented elapsed time")
		}
		return nil
	})
}
