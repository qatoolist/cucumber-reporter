package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/qatoolist/cucumber-reporter/internal/minify"
)

// TestMinifiedAssetsAreCurrent re-minifies the readable sources and compares the bytes with the
// committed .min files, so a stale or hand-edited embedded asset fails the build. Run go generate
// ./report after editing assets/app.js or assets/app.css.
func TestMinifiedAssetsAreCurrent(t *testing.T) {
	for src, minFn := range map[string]func(string) (string, error){"app.js": minify.JS, "app.css": minify.CSS} {
		readable, err := os.ReadFile(filepath.Join("assets", src))
		if err != nil {
			t.Fatal(err)
		}
		want, err := minFn(string(readable))
		if err != nil {
			t.Fatalf("minify %s: %v", src, err)
		}
		again, err := minFn(want)
		if err != nil || again != want {
			t.Errorf("minifying %s is not idempotent", src)
		}
		name := src[:len(src)-len(filepath.Ext(src))] + ".min" + filepath.Ext(src)
		committed, err := os.ReadFile(filepath.Join("assets", name))
		if err != nil {
			t.Fatal(err)
		}
		if string(committed) != want {
			t.Errorf("assets/%s is not the minified assets/%s; run go generate ./report", name, src)
		}
		embedded := scriptSource
		if filepath.Ext(src) == ".css" {
			embedded = styleSource
		}
		if embedded != want {
			t.Errorf("the embedded %s differs from the minified source", name)
		}
	}
}

// TestDataBlockIsEncodingJSON: the data block is exactly encoding/json's HTML-escaping output for the
// view model, not a hand-built string.
func TestDataBlockIsEncodingJSON(t *testing.T) {
	features, opt := loadShowcase(t, "")
	_, raw := dataBlock(t, mustRender(t, features, opt))
	nopt, err := normalize(opt)
	if err != nil {
		t.Fatal(err)
	}
	data, err := buildPage(features, nopt, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if raw != string(want) {
		t.Error("data block differs from json.Marshal of the view model")
	}
}

// TestScriptBuildsNoMarkupFromStrings: the embedded page script (comments already stripped) has no
// API that parses a string as markup or code, so data can reach the DOM only as text nodes and
// attribute values. TestBrowserHostileData checks the resulting DOM.
func TestScriptBuildsNoMarkupFromStrings(t *testing.T) {
	banned := regexp.MustCompile("innerHTML|outerHTML|insertAdjacentHTML|HTMLUnsafe|\\bsetHTML\\b|document\\.write|\\beval\\s*\\(|new\\s+Function|set(Timeout|Interval)\\s*\\(\\s*['\"`]|srcdoc|DOMParser|createContextualFragment|\\.style\\.cssText|setAttribute\\(\\s*['\"]style")
	if m := banned.FindAllString(scriptSource, -1); len(m) > 0 {
		t.Errorf("app.js uses %q", m)
	}
}
