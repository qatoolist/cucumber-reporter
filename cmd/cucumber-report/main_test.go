package main

import (
	"bytes"
	"errors"
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
	cmd := exec.CommandContext(t.Context(), os.Args[0], args...)
	cmd.Env = append(os.Environ(), "CUCUMBER_REPORT_MAIN=1")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	code = 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
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
	p := filepath.Join(t.TempDir(), "options.json")
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
	if !strings.Contains(string(doc), `"title":"Example","name":"Example results"`) || !strings.Contains(string(doc), "<title>Example results</title>") || !strings.Contains(string(doc), "<h1><bdi>Example results</bdi></h1>") {
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

func TestOptionsFile(t *testing.T) {
	showcase := filepath.Join("..", "..", "testdata", "synthetic", "showcase")
	results, err := os.ReadFile(filepath.Join(showcase, "cucumber-known.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := newRun(t, results)
	stdout, stderr, code := command(t, "-options", filepath.Join(showcase, "options-known.json"), "-name", "Overridden name", dir)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	doc, _ := os.ReadFile(reportPath(t, stdout))
	for _, want := range []string{
		`"verdict":{"label":"Passed with known failures","tone":"warn"}`,
		`"name":"Overridden name"`,
		`"title":"Example shop suite"`,
		`"role":"known"`,
		`"ref":"EX-330","title":"Expected failure: no push is sent to that customer"`,
		`<title>Overridden name</title>`,
	} {
		if !strings.Contains(string(doc), want) {
			t.Errorf("report lacks %s", want)
		}
	}
	opts := writeFile(t, `{"version":2,"defaults":{"theme":"dark","density":"compact","expand":"all","view":"failures","widgets":{"duration":false},"nav":{"run":false}}}`)
	stdout, stderr, code = command(t, "-options", opts, dir)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	doc, _ = os.ReadFile(reportPath(t, stdout))
	for _, want := range []string{`<html lang="en" data-theme="dark">`, `"defaults":{"theme":"dark","density":"compact","expand":"all","view":"failures","widgets":{`, `"duration":false`, `"run":false`} {
		if !strings.Contains(string(doc), want) {
			t.Errorf("report lacks %s", want)
		}
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
	optionsLink := filepath.Join(t.TempDir(), "options.json")
	if runtime.GOOS != "windows" {
		if err := os.Symlink(writeFile(t, `{"version":2}`), optionsLink); err != nil {
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
		{"options with version 1", []string{"-options", writeFile(t, `{"version":1}`), good}, 1, "cucumber-report: -options: unsupported version"},
		{"options without version", []string{"-options", writeFile(t, `{"title":"x"}`), good}, 1, "cucumber-report: -options: unsupported version"},
		{"options with unknown key", []string{"-options", writeFile(t, `{"version":2,"`+marker+`":1}`), good}, 1, "cucumber-report: -options: not a valid options file"},
		{"options with bad value", []string{"-options", writeFile(t, `{"version":2,"defaults":{"theme":"`+marker+`"}}`), good}, 1, "cucumber-report: report: invalid options: defaults.theme"},
		{"options with unknown scenario", []string{"-options", writeFile(t, `{"version":2,"annotations":[{"scenario":"`+marker+`","kind":"known"}]}`), good}, 1, "cucumber-report: report: invalid options: annotations[0].scenario names no scenario"},
		{"options with wrong type", []string{"-options", writeFile(t, `{"version":2,"runFacts":"`+marker+`"}`), good}, 1, "cucumber-report: -options: not a valid options file"},
		{"attachment over the embedding limit", []string{"-options", writeFile(t, `{"version":2,"limits":{"embedding":4}}`), good}, 1, "cucumber-report: cucumber: input exceeds a size limit"},
		{"options with trailing data", []string{"-options", writeFile(t, `{"version":2} {}`), good}, 1, "cucumber-report: -options: not a valid options file"},
		{"options missing", []string{"-options", filepath.Join(good, "absent.json"), good}, 1, "cucumber-report: -options: cannot read the options file"},
		{"options symlinked", []string{"-options", optionsLink, good}, 1, "cucumber-report: -options: the options file is a symbolic link or not a regular file"},
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

// TestRefusesSymlinkSpelledAsDirectory: a trailing "/" or "/." must not let a symbolic link through.
func TestRefusesSymlinkSpelledAsDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need privileges on Windows")
	}
	good := newRun(t, fixture(t))
	base := t.TempDir()
	runLink, outLink, outReal := filepath.Join(base, "run-link"), filepath.Join(base, "out-link"), t.TempDir()
	if err := os.Symlink(good, runLink); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outReal, outLink); err != nil {
		t.Fatal(err)
	}
	options := writeFile(t, `{"version":2}`)
	optionsLink := filepath.Join(base, "settings-link")
	if err := os.Symlink(options, optionsLink); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{runLink + "/"},
		{runLink + "/."},
		{runLink + "//"},
		{"-o", outLink + "/", good},
		{"-o", outLink + "/.", good},
		{"-options", optionsLink + "/.", good},
	} {
		stdout, stderr, code := command(t, args...)
		if code != 1 || stdout != "" || stderr == "" {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want a refusal", args, code, stdout, stderr)
		}
	}
	if entries, _ := os.ReadDir(good); len(entries) != 1 {
		t.Errorf("refused renders wrote into the run directory: %d entries", len(entries))
	}
	if entries, _ := os.ReadDir(outReal); len(entries) != 0 {
		t.Errorf("refused renders wrote into the -o link target: %d entries", len(entries))
	}
}
