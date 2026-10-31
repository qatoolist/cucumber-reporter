//go:build browser

package report

// Headless browser check of the page script (make browser-check). It needs a local Chrome, Chromium
// or Edge, found through $CHROME or the usual install locations; no module dependency is involved.
// Each case renders a report, removes the CSP meta tag so a test prelude and postlude script may run
// around the unchanged report script, and inspects the DOM that the browser serialises.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/qatoolist/cucumber-reporter/cucumber"
)

func findBrowser(t *testing.T) string {
	if c := os.Getenv("CHROME"); c != "" {
		return c
	}
	for _, c := range []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
	} {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	for _, name := range []string{"google-chrome", "chromium", "chromium-browser", "microsoft-edge"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	t.Fatal("no Chrome, Chromium or Edge found; set CHROME")
	return ""
}

// dumpDOM loads doc with prelude before and postlude after the report script and returns the DOM.
func dumpDOM(t *testing.T, doc, prelude, postlude string) string {
	t.Helper()
	doc = cspPattern.ReplaceAllString(doc, "")
	i := strings.LastIndex(doc, "<script>")
	j := i + strings.Index(doc[i:], "</script>") + len("</script>")
	doc = doc[:i] + "<script>" + prelude + "</script>" + doc[i:j] + "<script>" + postlude + "</script>" + doc[j:]
	dir := t.TempDir()
	page := filepath.Join(dir, "index.html")
	if err := os.WriteFile(page, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, findBrowser(t), "--headless=new", "--disable-gpu", "--no-first-run",
		"--user-data-dir="+filepath.Join(dir, "profile"), "--virtual-time-budget=3000", "--dump-dom", "file://"+page).Output()
	if err != nil {
		t.Fatalf("browser: %v", err)
	}
	return string(out)
}

func hiddenAttr(t *testing.T, dom, id string) bool {
	t.Helper()
	m := regexp.MustCompile(`<[a-z]+ [^>]*id="` + regexp.QuoteMeta(id) + `"[^>]*>`).FindString(dom)
	if m == "" {
		t.Fatalf("no element #%s", id)
	}
	return strings.Contains(m, ` hidden=""`)
}

func openCount(dom string) int {
	return len(regexp.MustCompile(`<details class="scenario pg-item"[^>]* open=""`).FindAllString(dom, -1))
}

const setPrefs = `localStorage.setItem('cucumber-reporter:v1:prefs', %s);`

func withPrefs(js string) string { return strings.Replace(setPrefs, "%s", js, 1) }

func TestBrowserDefaultsWithoutPreferences(t *testing.T) {
	dom := dumpDOM(t, mustRender(t, loadFixture(t, "godog-v0.16.0.json"), sampleOptions()), "localStorage.clear();", "")
	if !strings.Contains(dom, `<html lang="en" data-theme="auto">`) {
		t.Error("theme default not applied")
	}
	if !hiddenAttr(t, dom, "chart-slowest-scenarios") || hiddenAttr(t, dom, "chart-step-status") {
		t.Error("chart defaults not applied")
	}
	if hiddenAttr(t, dom, "open-settings") || hiddenAttr(t, dom, "open-help") {
		t.Error("script controls still hidden")
	}
	if !hiddenAttr(t, dom, "storage-note") {
		t.Error("storage notice shown although storage works")
	}
	if n := openCount(dom); n != 2 {
		t.Errorf("open scenarios = %d, want the 2 failed", n)
	}
	if !strings.Contains(dom, `2 of 2 features`) {
		t.Error("filter count not shown")
	}
}

func TestBrowserSavedPreferencesOverrideDefaults(t *testing.T) {
	prefs := `JSON.stringify({theme:'dark',density:'compact',expand:'all',pageSize:'25',bogus:1,
		show:{'chart:slowest-scenarios':true,'section:skipped':false,'chart:no-such-chart':false,'section:known-issues':'yes'}})`
	dom := dumpDOM(t, mustRender(t, loadFixture(t, "godog-v0.16.0.json"), sampleOptions()), withPrefs(prefs), "")
	if !strings.Contains(dom, `<html lang="en" data-theme="dark">`) || !regexp.MustCompile(`<body [^>]*class="compact"`).MatchString(dom) {
		t.Error("theme or density preference not applied")
	}
	if hiddenAttr(t, dom, "chart-slowest-scenarios") {
		t.Error("chart shown by preference is hidden")
	}
	if !hiddenAttr(t, dom, "sec-skipped") {
		t.Error("section hidden by preference is shown")
	}
	if hiddenAttr(t, dom, "sec-known-issues") {
		t.Error("a non-boolean preference was not ignored")
	}
	if n := openCount(dom); n != 6 {
		t.Errorf("open scenarios = %d, want all 6", n)
	}
}

func TestBrowserInvalidPreferencesFallBack(t *testing.T) {
	for _, prefs := range []string{`'{not json'`, `JSON.stringify({theme:'neon',pageSize:'7',expand:5,show:[1]})`, `'[]'`, `'null'`} {
		dom := dumpDOM(t, mustRender(t, loadFixture(t, "godog-v0.16.0.json"), sampleOptions()), withPrefs(prefs), "")
		if !strings.Contains(dom, `data-theme="auto">`) || !hiddenAttr(t, dom, "chart-slowest-scenarios") || openCount(dom) != 2 {
			t.Errorf("prefs %s: defaults not used", prefs)
		}
		if !hiddenAttr(t, dom, "storage-note") {
			t.Errorf("prefs %s: storage notice shown for unreadable content", prefs)
		}
	}
}

func TestBrowserBlockedStorage(t *testing.T) {
	block := `Object.defineProperty(window, 'localStorage', {get: function () { throw new DOMException('blocked', 'SecurityError'); }});`
	post := `document.querySelector('[data-pref=theme]').value = 'dark';
		document.querySelector('[data-pref=theme]').dispatchEvent(new Event('change'));`
	dom := dumpDOM(t, mustRender(t, loadFixture(t, "godog-v0.16.0.json"), sampleOptions()), block, post)
	if hiddenAttr(t, dom, "storage-note") {
		t.Error("no notice although storage is blocked")
	}
	if !strings.Contains(dom, `data-theme="dark">`) {
		t.Error("a change made while storage is blocked was not applied to the page")
	}
}

func TestBrowserSettingsSaveAndReset(t *testing.T) {
	post := `var s = document.querySelector('[data-pref=density]'); s.value = 'compact'; s.dispatchEvent(new Event('change'));
		var c = document.querySelector('[data-toggle="chart:step-status"]'); c.checked = false; c.dispatchEvent(new Event('change'));
		document.body.setAttribute('data-saved', localStorage.getItem('cucumber-reporter:v1:prefs'));
		document.getElementById('reset-prefs').click();
		document.body.setAttribute('data-after-reset', String(localStorage.getItem('cucumber-reporter:v1:prefs')));`
	dom := dumpDOM(t, mustRender(t, loadFixture(t, "godog-v0.16.0.json"), sampleOptions()), "localStorage.clear();", post)
	if !strings.Contains(dom, `data-saved="{&quot;show&quot;:{&quot;chart:step-status&quot;:false},&quot;density&quot;:&quot;compact&quot;}"`) {
		t.Error("preferences not saved as expected")
	}
	if !strings.Contains(dom, `data-after-reset="null"`) {
		t.Error("reset did not remove the saved preferences")
	}
	if regexp.MustCompile(`<body [^>]*class="compact"`).MatchString(dom) || hiddenAttr(t, dom, "chart-step-status") {
		t.Error("reset did not restore the report defaults")
	}
}

func TestBrowserPagingFilteringAndShortcuts(t *testing.T) {
	opt := Options{Version: 1, Defaults: ViewDefaults{PageSize: 25}}
	doc := mustRender(t, syntheticRun(30, 5, 6), opt)

	dom := dumpDOM(t, doc, "localStorage.clear();", "")
	rows := regexp.MustCompile(`<tr class="pg-item[^"]*"[^>]*>`).FindAllString(dom, -1)
	visible := 0
	for _, r := range rows {
		if !strings.Contains(r, `hidden=""`) {
			visible++
		}
	}
	if len(rows) != 30 || visible != 25 {
		t.Errorf("rows %d, visible %d; want 30 and 25", len(rows), visible)
	}
	if !strings.Contains(dom, `Show 5 more of 5</button>`) {
		t.Error("no Show more button for the feature table")
	}

	post := `var q = document.getElementById('search'); q.value = 'AREA 07'; q.dispatchEvent(new Event('input'));`
	dom = dumpDOM(t, doc, "localStorage.clear();", post)
	if !strings.Contains(dom, `1 of 30 features`) || hiddenAttr(t, dom, "f8") || !hiddenAttr(t, dom, "f9") {
		t.Error("search did not filter features case-insensitively")
	}

	post = `document.dispatchEvent(new KeyboardEvent('keydown', {key: 'f'}));`
	dom = dumpDOM(t, doc, "localStorage.clear();", post)
	if !strings.Contains(dom, `30 of 30 features`) {
		t.Error("f shortcut: every synthetic feature has a failed scenario and should match")
	}
	post = `document.dispatchEvent(new KeyboardEvent('keydown', {key: 'f'})); document.dispatchEvent(new KeyboardEvent('keydown', {key: 'f'}));`
	dom = dumpDOM(t, doc, "localStorage.clear();", post)
	if !regexp.MustCompile(`<select id="status-filter"[^>]*>`).MatchString(dom) || !strings.Contains(dom, `30 of 30 features`) {
		t.Error("f twice should return to all features")
	}

	// A link to a scenario that paging hid extends the page and opens the card.
	one := []cucumber.Feature{{Name: "F", Keyword: "Feature", Elements: syntheticRun(1, 30, 1)[0].Elements}}
	post = `history.replaceState(null, '', '#f1-s28'); window.dispatchEvent(new HashChangeEvent('hashchange'));`
	dom = dumpDOM(t, mustRender(t, one, opt), "localStorage.clear();", post)
	if hiddenAttr(t, dom, "f1-s28") || !regexp.MustCompile(`id="f1-s28"[^>]* open=""`).MatchString(dom) || !hiddenAttr(t, dom, "f1-s29") {
		t.Error("link target hidden by paging was not shown and opened")
	}

	post = `document.querySelector('th[data-sort=num]:nth-child(8) button').click(); document.querySelector('th[data-sort=num]:nth-child(8) button').click();`
	dom = dumpDOM(t, doc, "localStorage.clear();", post)
	if !strings.Contains(dom, `aria-sort="descending"`) {
		t.Error("sorting did not mark the column")
	}
}

// TestBrowserScriptRunsUnderCSP loads the unmodified report: the hashed script must run (it reveals
// the settings button) and the hashed stylesheet must apply, under the page's own policy.
func TestBrowserScriptRunsUnderCSP(t *testing.T) {
	doc := mustRender(t, loadFixture(t, "godog-v0.16.0.json"), sampleOptions())
	dir := t.TempDir()
	page := filepath.Join(dir, "index.html")
	if err := os.WriteFile(page, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, findBrowser(t), "--headless=new", "--disable-gpu", "--no-first-run",
		"--user-data-dir="+filepath.Join(dir, "profile"), "--virtual-time-budget=3000", "--dump-dom", "file://"+page).Output()
	if err != nil {
		t.Fatalf("browser: %v", err)
	}
	dom := string(out)
	if hiddenAttr(t, dom, "open-settings") || !strings.Contains(dom, "2 of 2 features") {
		t.Error("report script did not run under the page's Content-Security-Policy")
	}
}
