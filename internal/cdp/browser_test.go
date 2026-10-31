//go:build browser

package cdp

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRealChrome(t *testing.T) {
	chrome, err := FindChrome()
	if err != nil {
		t.Fatalf("no Chrome available (the browser checks need one; set CHROME): %v", err)
	}
	dir := t.TempDir()
	page := filepath.Join(dir, "page.html")
	html := `<!doctype html><meta name="viewport" content="width=device-width"><title>t</title><body><script>
window.marker = 'ready'; window.keys = [];
addEventListener('keydown', e => window.keys.push(e.key));
throw new Error('boom from page');
</script></body>`
	if err = os.WriteFile(page, []byte(html), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	b, err := Launch(ctx, chrome, filepath.Join(dir, "profile"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	if err = b.Navigate(ctx, "file://"+page); err != nil {
		t.Fatal(err)
	}
	var marker string
	if err = b.Eval(ctx, "window.marker", &marker); err != nil || marker != "ready" {
		t.Fatalf("marker = %q, %v", marker, err)
	}
	var found bool
	for _, p := range b.Problems() {
		found = found || strings.Contains(p, "boom from page")
	}
	if !found {
		t.Fatalf("exception missing from problems: %q", b.Problems())
	}

	if err = b.SetViewport(ctx, 400, 700); err != nil {
		t.Fatal(err)
	}
	var w int
	if err = b.Eval(ctx, "innerWidth", &w); err != nil || w != 400 {
		t.Fatalf("innerWidth = %d, %v", w, err)
	}

	if err = b.Key(ctx, "a", 0); err != nil {
		t.Fatal(err)
	}
	var keys []string
	if err = b.Eval(ctx, "window.keys", &keys); err != nil || len(keys) != 1 || keys[0] != "a" {
		t.Fatalf("keys = %q, %v", keys, err)
	}

	pdf, err := b.PrintPDF(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("not a PDF: %q", pdf[:min(8, len(pdf))])
	}
	if err = b.Reload(ctx); err != nil {
		t.Fatal(err)
	}
}
