package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The tests run the real command: the test binary re-executes itself with CUCUMBER_REPORT_MAIN set,
// which calls main and exits with its status.
func TestMain(m *testing.M) {
	if os.Getenv("CUCUMBER_REPORT_MAIN") == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}

func command(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "CUCUMBER_REPORT_MAIN=1")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	code = 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return out.String(), errOut.String(), code
}

func fixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "synthetic", "godog-v0.16.0.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func newRun(t *testing.T, content []byte) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "run 1")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cucumber.json"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func reportPath(t *testing.T, stdout string) string {
	t.Helper()
	line := strings.TrimSuffix(stdout, "\n")
	path, ok := strings.CutPrefix(line, "HTML report: ")
	if !ok || strings.Contains(line, "\n") || filepath.Base(path) != "index.html" {
		t.Fatalf("stdout = %q, want exactly one 'HTML report: <path>/index.html' line", stdout)
	}
	return path
}

func TestRendersIntoRunDirectory(t *testing.T) {
	dir := newRun(t, fixture(t))
	stdout, stderr, code := command(t, "-title", "Example", "-name", "Example results", dir)
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	index := reportPath(t, stdout)
	out := filepath.Dir(index)
	if filepath.Dir(out) != dir || !strings.HasPrefix(filepath.Base(out), "html-") {
		t.Errorf("report %q is not in <run>/html-*", index)
	}
	if runtime.GOOS != "windows" {
		for path, want := range map[string]os.FileMode{out: 0o700, index: 0o600} {
			info, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != want {
				t.Errorf("%s mode %o, want %o", path, info.Mode().Perm(), want)
			}
		}
	}
	doc, _ := os.ReadFile(index)
	if !strings.Contains(string(doc), "<title>Example</title>") || !strings.Contains(string(doc), "<h1>Example results</h1>") {
		t.Error("title or name flag not applied")
	}

	stdout2, _, code := command(t, dir)
	if code != 0 || reportPath(t, stdout2) == index {
		t.Error("second render did not create a new report")
	}
	if again, _ := os.ReadFile(index); !bytes.Equal(again, doc) {
		t.Error("second render changed the first report")
	}
}

func TestOutputParent(t *testing.T) {
	dir := newRun(t, fixture(t))
	parent := t.TempDir()
	stdout, stderr, code := command(t, "-o", parent, dir)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if got := filepath.Dir(filepath.Dir(reportPath(t, stdout))); got != parent {
		t.Errorf("report under %q, want %q", got, parent)
	}
}

func TestDefaultsFile(t *testing.T) {
	dir := newRun(t, fixture(t))
	settings := writeFile(t, `{"version":1,"charts":{"step-status":false},"theme":"dark","density":"compact","expand":"all","pageSize":"all"}`)
	stdout, stderr, code := command(t, "-defaults", settings, dir)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	doc, _ := os.ReadFile(reportPath(t, stdout))
	for _, want := range []string{`data-theme="dark" data-density="compact" data-expand="all" data-page-size="0"`, `data-hidden-charts="step-status"`} {
		if !strings.Contains(string(doc), want) {
			t.Errorf("report lacks %s", want)
		}
	}
	settings = writeFile(t, `{"version":1,"pageSize":25}`)
	stdout, _, code = command(t, "-defaults", settings, dir)
	if doc, _ := os.ReadFile(reportPath(t, stdout)); code != 0 || !strings.Contains(string(doc), `data-page-size="25"`) {
		t.Error("numeric page size not applied")
	}
}

func TestRefusals(t *testing.T) {
	const marker = "SECRET-MARKER-c0de"
	good := newRun(t, fixture(t))
	broken := newRun(t, []byte(`[{"name":"`+marker+`"}]`))
	notJSON := newRun(t, []byte(marker))

	linkDir := filepath.Join(t.TempDir(), "linked")
	if err := os.Mkdir(linkDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink(filepath.Join(good, "cucumber.json"), filepath.Join(linkDir, "cucumber.json")); err != nil {
			t.Fatal(err)
		}
	}
	settingsLink := filepath.Join(t.TempDir(), "settings.json")
	if runtime.GOOS != "windows" {
		if err := os.Symlink(writeFile(t, `{"version":1}`), settingsLink); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name string
		args []string
		code int
		err  string
	}{
		{"no arguments", nil, 2, "usage: cucumber-report"},
		{"two run directories", []string{good, good}, 2, "usage: cucumber-report"},
		{"unknown flag", []string{"-x", good}, 2, "usage: cucumber-report"},
		{"incomplete results", []string{broken}, 1, "cucumber-report: cucumber: input contains a feature without scenarios"},
		{"not JSON", []string{notJSON}, 1, "cucumber-report: cucumber: input is not valid cucumber JSON"},
		{"missing run directory", []string{filepath.Join(good, "absent")}, 1, "cucumber-report: report: run directory has no cucumber.json"},
		{"symlinked cucumber.json", []string{linkDir}, 1, "cucumber-report: report: input is a symbolic link"},
		{"defaults with unknown version", []string{"-defaults", writeFile(t, `{"version":2}`), good}, 1, "cucumber-report: -defaults: unsupported version"},
		{"defaults without version", []string{"-defaults", writeFile(t, `{"theme":"dark"}`), good}, 1, "cucumber-report: -defaults: unsupported version"},
		{"defaults with unknown key", []string{"-defaults", writeFile(t, `{"version":1,"`+marker+`":1}`), good}, 1, "cucumber-report: -defaults: not a valid settings file"},
		{"defaults with bad value", []string{"-defaults", writeFile(t, `{"version":1,"theme":"`+marker+`"}`), good}, 1, "cucumber-report: report: invalid options: defaults.theme"},
		{"defaults with bad page size", []string{"-defaults", writeFile(t, `{"version":1,"pageSize":"some"}`), good}, 1, "cucumber-report: -defaults: not a valid settings file"},
		{"defaults with trailing data", []string{"-defaults", writeFile(t, `{"version":1} {}`), good}, 1, "cucumber-report: -defaults: not a valid settings file"},
		{"defaults missing", []string{"-defaults", filepath.Join(good, "absent.json"), good}, 1, "cucumber-report: -defaults: cannot read the settings file"},
		{"defaults symlinked", []string{"-defaults", settingsLink, good}, 1, "cucumber-report: -defaults: the settings file is a symbolic link or not a regular file"},
	}
	for _, tt := range tests {
		if runtime.GOOS == "windows" && strings.Contains(tt.name, "symlink") {
			continue
		}
		stdout, stderr, code := command(t, tt.args...)
		if code != tt.code || !strings.HasPrefix(stderr, tt.err) {
			t.Errorf("%s: exit %d, stderr %q; want exit %d, stderr starting %q", tt.name, code, stderr, tt.code, tt.err)
		}
		if stdout != "" {
			t.Errorf("%s: stdout %q, want nothing", tt.name, stdout)
		}
		if strings.Contains(stderr, marker) {
			t.Errorf("%s: stderr contains input content", tt.name)
		}
	}
	for _, dir := range []string{broken, notJSON, linkDir} {
		entries, _ := os.ReadDir(dir)
		if len(entries) != 1 {
			t.Errorf("%s: refused render left %d entries", dir, len(entries))
		}
	}
}
