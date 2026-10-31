package report

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/qatoolist/cucumber-reporter/cucumber"
)

func runDir(t *testing.T, cucumberJSON string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "run with spaces")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cucumber.json"), []byte(cucumberJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func fixtureText(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "testdata", "synthetic", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func perm(t *testing.T, path string) fs.FileMode {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

// entries lists a directory's names, sorted.
func entries(t *testing.T, dir string) []string {
	t.Helper()
	list, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range list {
		names = append(names, e.Name())
	}
	return names
}

func TestRenderDirWritesPrivateNewDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	dir := runDir(t, fixtureText(t, "minimal.json"))
	out, err := RenderDir(dir, Options{Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(out) != dir || !strings.HasPrefix(filepath.Base(out), "html-") || len(filepath.Base(out)) <= len("html-") {
		t.Fatalf("output directory %q is not <runDir>/html-XXXXXX", out)
	}
	if got := perm(t, out); got != 0o700 {
		t.Errorf("directory mode %o, want 700", got)
	}
	index := filepath.Join(out, "index.html")
	if got := perm(t, index); got != 0o600 {
		t.Errorf("index.html mode %o, want 600", got)
	}
	data, err := os.ReadFile(index)
	if err != nil || len(data) == 0 {
		t.Fatalf("index.html: %d bytes, %v", len(data), err)
	}
	want := mustRender(t, loadFixture(t, "minimal.json"), Options{Version: 1})
	if string(data) != want {
		t.Error("RenderDir output differs from Render output for the same input")
	}

	// A second render creates another directory and keeps the first unchanged.
	out2, err := RenderDir(dir, Options{Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if out2 == out {
		t.Fatal("second render reused the first directory")
	}
	if again, _ := os.ReadFile(index); string(again) != want {
		t.Error("first render changed")
	}
	names := entries(t, dir)
	if len(names) != 3 {
		t.Errorf("run directory holds %v; want cucumber.json and two html-* directories, no temporary directory", names)
	}
}

func TestRenderDirSidecarForLargeAttachment(t *testing.T) {
	big := strings.Repeat("evidence line\n", 200)
	doc := `[{"name":"F","elements":[{"name":"S","steps":[{"keyword":"Then ","name":"s","result":{"status":"passed"},"embeddings":[
		{"name":"big.json","mime_type":"application/json","data":"` + b64(`{"lines":"`+strings.ReplaceAll(big, "\n", `\n`)+`"}`) + `"},
		{"name":"big.txt","mime_type":"text/plain","data":"` + b64(big) + `"},
		{"name":"big.html","mime_type":"text/html","data":"` + b64("<p>"+big+"</p>") + `"},
		{"name":"big.bin","mime_type":"application/octet-stream","data":"` + b64(big) + `"}]}]}]}]`
	dir := runDir(t, doc)
	out, err := RenderDir(dir, Options{Version: 1, Limits: cucumber.Limits{InlineAttachment: 1024}})
	if err != nil {
		t.Fatal(err)
	}
	names := entries(t, out)
	want := []string{"attachment-f1-s1-t1-a1.json", "attachment-f1-s1-t1-a2.txt", "attachment-f1-s1-t1-a3.txt", "index.html"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("output holds %v, want %v", names, want)
	}
	for _, name := range want[:3] {
		if runtime.GOOS != "windows" {
			if got := perm(t, filepath.Join(out, name)); got != 0o600 {
				t.Errorf("%s mode %o, want 600", name, got)
			}
		}
	}
	if got, _ := os.ReadFile(filepath.Join(out, "attachment-f1-s1-t1-a2.txt")); string(got) != big {
		t.Error("text sidecar is not the exact attachment bytes")
	}
	if got, _ := os.ReadFile(filepath.Join(out, "attachment-f1-s1-t1-a3.txt")); string(got) != "<p>"+big+"</p>" {
		t.Error("HTML attachment sidecar is not the exact bytes, saved as text")
	}
	index, _ := os.ReadFile(filepath.Join(out, "index.html"))
	for _, link := range []string{`href="attachment-f1-s1-t1-a1.json"`, `href="attachment-f1-s1-t1-a2.txt"`, `href="attachment-f1-s1-t1-a3.txt"`} {
		if !strings.Contains(string(index), link) {
			t.Errorf("index.html has no link %s", link)
		}
	}
	if strings.Contains(string(index), "evidence line\nevidence line") {
		t.Error("oversize attachment was also inlined")
	}
}

func TestRenderDirRefusesSymlinksAndOtherKinds(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need privileges on Windows")
	}
	target := filepath.Join(t.TempDir(), "elsewhere.json")
	if err := os.WriteFile(target, []byte(fixtureText(t, "minimal.json")), 0o600); err != nil {
		t.Fatal(err)
	}

	linked := filepath.Join(t.TempDir(), "run")
	if err := os.Mkdir(linked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(linked, "cucumber.json")); err != nil {
		t.Fatal(err)
	}

	asDir := filepath.Join(t.TempDir(), "run")
	if err := os.MkdirAll(filepath.Join(asDir, "cucumber.json"), 0o700); err != nil {
		t.Fatal(err)
	}

	realRun := runDir(t, fixtureText(t, "minimal.json"))
	runLink := filepath.Join(t.TempDir(), "run-link")
	if err := os.Symlink(realRun, runLink); err != nil {
		t.Fatal(err)
	}

	missing := filepath.Join(t.TempDir(), "run")
	if err := os.Mkdir(missing, 0o700); err != nil {
		t.Fatal(err)
	}

	notDir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notDir, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		dir  string
		want error
	}{
		{"cucumber.json is a symlink", linked, ErrNotRegular},
		{"cucumber.json is a directory", asDir, ErrNotRegular},
		{"run directory is a symlink", runLink, ErrNotRegular},
		{"run directory is a file", notDir, ErrNotRegular},
		{"no cucumber.json", missing, ErrNoInput},
		{"run directory missing", filepath.Join(missing, "absent"), ErrNoInput},
	}
	for _, tt := range tests {
		_, err := RenderDir(tt.dir, Options{Version: 1})
		if !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
		}
	}
	for _, dir := range []string{linked, asDir, missing, realRun} {
		for _, name := range entries(t, dir) {
			if strings.HasPrefix(name, "html-") || strings.HasPrefix(name, ".html-report-") {
				t.Errorf("%s: refused render left %s behind", dir, name)
			}
		}
	}
}

func TestRenderDirFailureLeavesNothingAndWithholdsContent(t *testing.T) {
	const marker = "SECRET-MARKER-91c2"
	for _, doc := range []string{
		`[{"name":"` + marker + `"}]`,
		`[{"name":"F","elements":[{"steps":[{"result":{"status":"` + marker + `"}}]}]}]`,
		`not json ` + marker,
		``,
	} {
		dir := runDir(t, doc)
		_, err := RenderDir(dir, Options{Version: 1})
		if err == nil {
			t.Fatalf("no error for %q", doc)
		}
		if strings.Contains(err.Error(), marker) {
			t.Errorf("error %q contains input content", err)
		}
		if names := entries(t, dir); len(names) != 1 {
			t.Errorf("failed render left %v", names)
		}
	}
	dir := runDir(t, fixtureText(t, "minimal.json"))
	if _, err := RenderDir(dir, Options{}); !errors.Is(err, ErrOptionsVersion) {
		t.Errorf("zero options: %v", err)
	}
	if names := entries(t, dir); len(names) != 1 {
		t.Errorf("options failure left %v", names)
	}
}

func TestRenderDirInputLimit(t *testing.T) {
	dir := runDir(t, fixtureText(t, "minimal.json"))
	if _, err := RenderDir(dir, Options{Version: 1, Limits: cucumber.Limits{Input: 100}}); !errors.Is(err, cucumber.ErrTooLarge) {
		t.Errorf("err = %v, want ErrTooLarge", err)
	}
}

func TestRenderDirTo(t *testing.T) {
	dir := runDir(t, fixtureText(t, "minimal.json"))
	parent := t.TempDir()
	out, err := RenderDirTo(dir, parent, Options{Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(out) != parent {
		t.Errorf("output %q not under %q", out, parent)
	}
	if names := entries(t, dir); len(names) != 1 {
		t.Errorf("run directory changed: %v", names)
	}
	if runtime.GOOS == "windows" {
		return
	}
	link := filepath.Join(t.TempDir(), "parent-link")
	if err := os.Symlink(parent, link); err != nil {
		t.Fatal(err)
	}
	if _, err := RenderDirTo(dir, link, Options{Version: 1}); !errors.Is(err, ErrNotRegular) {
		t.Errorf("symlinked parent: %v", err)
	}
	if _, err := RenderDirTo(dir, filepath.Join(parent, "absent"), Options{Version: 1}); err == nil {
		t.Error("missing parent accepted")
	}
}
